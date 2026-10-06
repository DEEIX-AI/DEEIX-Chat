// CodeMirror-based runtime behind JsonCodeEditor. Imported lazily by the component so the
// editor code is only downloaded once an editor actually mounts.

import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import { json } from "@codemirror/lang-json";
import {
  bracketMatching,
  ensureSyntaxTree,
  foldGutter,
  foldKeymap,
  HighlightStyle,
  indentOnInput,
  indentUnit,
  syntaxHighlighting,
} from "@codemirror/language";
import { type Diagnostic, diagnosticCount, linter, lintKeymap } from "@codemirror/lint";
import { highlightSelectionMatches, search, searchKeymap } from "@codemirror/search";
import { Annotation, Compartment, EditorState, type Extension, RangeSetBuilder } from "@codemirror/state";
import {
  Decoration,
  type DecorationSet,
  drawSelection,
  dropCursor,
  EditorView,
  highlightActiveLine,
  highlightActiveLineGutter,
  highlightSpecialChars,
  keymap,
  lineNumbers,
  placeholder as placeholderExtension,
  tooltips,
  ViewPlugin,
  type ViewUpdate,
} from "@codemirror/view";
import { tags } from "@lezer/highlight";

export type JsonEditorTheme = "light" | "dark";

export type JsonEditorOptions = {
  parent: HTMLElement;
  value: string;
  placeholder?: string;
  readOnly: boolean;
  wordWrap: boolean;
  theme: JsonEditorTheme;
  fontSize: number;
  onChange: (value: string) => void;
  onBlur: () => void;
  onDiagnosticsChange: (count: number) => void;
};

export type JsonEditorController = {
  getValue: () => string;
  // Replaces the document without reporting it through onChange.
  setValue: (value: string) => void;
  setReadOnly: (readOnly: boolean) => void;
  setWordWrap: (wordWrap: boolean) => void;
  setTheme: (theme: JsonEditorTheme) => void;
  setFontSize: (fontSize: number) => void;
  setPlaceholder: (placeholder: string | undefined) => void;
  // Returns false when the document is empty or not valid JSON.
  format: () => boolean;
  focus: () => void;
  destroy: () => void;
};

const LINT_DELAY_MS = 300;
const LINT_PARSE_BUDGET_MS = 100;
const INDENT = "  ";
const MONO_FONT_FAMILY = "var(--font-mono), ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace";
const SVG_NAMESPACE = "http://www.w3.org/2000/svg";

// Marks programmatic document replacements so they are not echoed back as user edits.
const externalChange = Annotation.define<boolean>();

type EditorPalette = {
  key: string;
  string: string;
  number: string;
  literal: string;
  punctuation: string;
  error: string;
  indentGuide: string;
  activeLine: string;
  selection: string;
  subtleHighlight: string;
  searchMatch: string;
};

// Token colors are tuned for the app's warm neutral surfaces: values meet WCAG AA (4.5:1) on
// both themes, numbers reuse the brand's terracotta hue, and punctuation recedes to 3:1 so the
// data stands out.
// Overlays are plain rgba because color-mix() is unavailable in the older WebKit the
// desktop shell still supports.
const LIGHT_PALETTE: EditorPalette = {
  key: "#2b5f8f",
  string: "#467330",
  number: "#b4532a",
  literal: "#8046a8",
  punctuation: "#8a857b",
  error: "#d0382b",
  indentGuide: "rgba(0, 0, 0, 0.07)",
  activeLine: "rgba(0, 0, 0, 0.03)",
  selection: "rgba(59, 130, 246, 0.16)",
  subtleHighlight: "rgba(0, 0, 0, 0.07)",
  searchMatch: "rgba(234, 179, 8, 0.28)",
};

const DARK_PALETTE: EditorPalette = {
  key: "#8db9e4",
  string: "#a9cb8d",
  number: "#e9a07c",
  literal: "#c9a4e6",
  punctuation: "#7f7a71",
  error: "#f07468",
  indentGuide: "rgba(255, 255, 255, 0.07)",
  activeLine: "rgba(255, 255, 255, 0.035)",
  selection: "rgba(96, 165, 250, 0.24)",
  subtleHighlight: "rgba(255, 255, 255, 0.1)",
  searchMatch: "rgba(234, 179, 8, 0.24)",
};

function highlightStyleFor(palette: EditorPalette): HighlightStyle {
  return HighlightStyle.define([
    { tag: tags.propertyName, color: palette.key },
    { tag: tags.string, color: palette.string },
    { tag: tags.number, color: palette.number },
    { tag: [tags.bool, tags.null], color: palette.literal },
    { tag: [tags.separator, tags.brace, tags.squareBracket], color: palette.punctuation },
    { tag: tags.invalid, color: palette.error },
  ]);
}

const lightHighlightStyle = highlightStyleFor(LIGHT_PALETTE);
const darkHighlightStyle = highlightStyleFor(DARK_PALETTE);

// Chevron in the style of the app's lucide icons; CSS rotates it for folded ranges.
function foldMarker(open: boolean): HTMLElement {
  const marker = document.createElement("span");
  marker.className = open ? "cm-fold-marker" : "cm-fold-marker cm-fold-marker-closed";
  const icon = document.createElementNS(SVG_NAMESPACE, "svg");
  for (const [name, value] of Object.entries({
    viewBox: "0 0 24 24",
    width: "12",
    height: "12",
    fill: "none",
    stroke: "currentColor",
    "stroke-width": "2",
    "stroke-linecap": "round",
    "stroke-linejoin": "round",
    "aria-hidden": "true",
  })) {
    icon.setAttribute(name, value);
  }
  const path = document.createElementNS(SVG_NAMESPACE, "path");
  path.setAttribute("d", "m6 9 6 6 6-6");
  icon.append(path);
  marker.append(icon);
  return marker;
}

function indentLevels(text: string): number {
  let columns = 0;
  for (const character of text) {
    if (character === " ") {
      columns += 1;
    } else if (character === "\t") {
      columns += INDENT.length;
    } else {
      break;
    }
  }
  return Math.floor(columns / INDENT.length);
}

const indentGuideDecorations = new Map<number, Decoration>();
function indentGuideDecoration(levels: number): Decoration {
  let decoration = indentGuideDecorations.get(levels);
  if (!decoration) {
    decoration = Decoration.line({
      attributes: { class: "cm-indent-guides", style: `--cm-indent-levels: ${levels}` },
    });
    indentGuideDecorations.set(levels, decoration);
  }
  return decoration;
}

// One vertical guide per indent level, painted as the line's background so it adds no DOM.
// Only lines in the visible ranges are decorated, which keeps large documents cheap.
function buildIndentGuides(view: EditorView): DecorationSet {
  const builder = new RangeSetBuilder<Decoration>();
  let lastLineFrom = -1;
  for (const { from, to } of view.visibleRanges) {
    for (let position = from; position <= to; ) {
      const line = view.state.doc.lineAt(position);
      const levels = indentLevels(line.text);
      if (levels > 0 && line.from > lastLineFrom) {
        builder.add(line.from, line.from, indentGuideDecoration(levels));
        lastLineFrom = line.from;
      }
      position = line.to + 1;
    }
  }
  return builder.finish();
}

const indentGuides = ViewPlugin.fromClass(
  class {
    decorations: DecorationSet;

    constructor(view: EditorView) {
      this.decorations = buildIndentGuides(view);
    }

    update(update: ViewUpdate) {
      if (update.docChanged || update.viewportChanged) {
        this.decorations = buildIndentGuides(update.view);
      }
    }
  },
  { decorations: (plugin) => plugin.decorations },
);

// Layout and neutral chrome follow the app's CSS variables, which already switch with the theme.
const baseTheme = EditorView.theme({
  "&": {
    height: "100%",
    backgroundColor: "transparent",
    color: "var(--foreground)",
  },
  "&.cm-focused": {
    outline: "none",
  },
  // The editor has a fixed height, so the scroller owns both scroll axes.
  ".cm-scroller": {
    overflow: "auto",
    fontFamily: MONO_FONT_FAMILY,
    lineHeight: "1.6",
  },
  ".cm-content": {
    padding: "10px 0",
    caretColor: "var(--primary)",
  },
  ".cm-line": {
    padding: "0 12px 0 4px",
  },
  ".cm-cursor, .cm-dropCursor": {
    borderLeft: "1.5px solid var(--primary)",
  },
  ".cm-gutters": {
    backgroundColor: "transparent",
    border: "none",
    color: "var(--muted-foreground)",
    userSelect: "none",
  },
  ".cm-lineNumbers .cm-gutterElement": {
    minWidth: "2ch",
    padding: "0 2px 0 12px",
    opacity: "0.55",
    fontVariantNumeric: "tabular-nums",
  },
  ".cm-activeLineGutter": {
    backgroundColor: "transparent",
  },
  "&.cm-focused .cm-lineNumbers .cm-activeLineGutter": {
    opacity: "1",
  },
  // Fold chevrons stay out of the way until the gutter is hovered; folded ranges keep theirs.
  ".cm-foldGutter .cm-gutterElement": {
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    width: "16px",
  },
  ".cm-fold-marker": {
    display: "inline-flex",
    padding: "1px",
    borderRadius: "3px",
    cursor: "pointer",
    opacity: "0",
    transition: "opacity 120ms ease, transform 120ms ease",
  },
  ".cm-gutters:hover .cm-fold-marker": {
    opacity: "0.7",
  },
  // Touch screens cannot hover, so their chevrons stay faintly visible.
  "@media (hover: none)": {
    ".cm-fold-marker": {
      opacity: "0.5",
    },
  },
  ".cm-gutters .cm-fold-marker:hover": {
    opacity: "1",
    backgroundColor: "var(--muted)",
  },
  ".cm-fold-marker-closed": {
    opacity: "0.8",
    transform: "rotate(-90deg)",
  },
  ".cm-foldPlaceholder": {
    margin: "0 3px",
    padding: "0 5px",
    border: "none",
    borderRadius: "4px",
    backgroundColor: "var(--muted)",
    color: "var(--muted-foreground)",
  },
  ".cm-indent-guides": {
    backgroundImage:
      "repeating-linear-gradient(to right, var(--cm-indent-guide) 0, var(--cm-indent-guide) 1px, transparent 1px, transparent 2ch)",
    backgroundSize: "calc(var(--cm-indent-levels) * 2ch) 100%",
    backgroundPosition: "4px 0",
    backgroundRepeat: "no-repeat",
  },
  ".cm-placeholder": {
    color: "var(--muted-foreground)",
    opacity: "0.7",
  },
  ".cm-tooltip": {
    backgroundColor: "var(--popover)",
    color: "var(--popover-foreground)",
    border: "1px solid var(--border)",
    borderRadius: "8px",
    boxShadow: "0 8px 24px rgba(0, 0, 0, 0.12)",
    overflow: "hidden",
  },
  ".cm-diagnostic": {
    padding: "6px 10px 6px 9px",
    fontFamily: MONO_FONT_FAMILY,
    fontSize: "11px",
  },
  ".cm-panels": {
    backgroundColor: "var(--popover)",
    color: "var(--popover-foreground)",
  },
  ".cm-panels.cm-panels-top": {
    borderBottom: "1px solid var(--border)",
  },
  ".cm-search": {
    padding: "6px 8px",
    fontSize: "11px",
  },
  ".cm-textfield": {
    backgroundColor: "transparent",
    border: "1px solid var(--input)",
    borderRadius: "6px",
    color: "inherit",
    padding: "2px 6px",
  },
  ".cm-button": {
    backgroundImage: "none",
    backgroundColor: "transparent",
    border: "1px solid var(--border)",
    borderRadius: "6px",
    color: "inherit",
  },
  ".cm-button:hover": {
    backgroundColor: "var(--muted)",
  },
});

function chromeTheme(theme: JsonEditorTheme): Extension {
  const dark = theme === "dark";
  const palette = dark ? DARK_PALETTE : LIGHT_PALETTE;
  return [
    EditorView.theme(
      {
        "&": {
          "--cm-indent-guide": palette.indentGuide,
        },
        // Row and bracket highlights only while editing, so an idle field reads as plain data.
        ".cm-activeLine": {
          backgroundColor: "transparent",
        },
        "&.cm-focused .cm-activeLine": {
          backgroundColor: palette.activeLine,
        },
        "&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection":
          {
            backgroundColor: palette.selection,
          },
        "& .cm-matchingBracket, & .cm-nonmatchingBracket": {
          backgroundColor: "transparent",
        },
        "&.cm-focused .cm-matchingBracket": {
          backgroundColor: palette.subtleHighlight,
          borderRadius: "2px",
          outline: "none",
        },
        "&.cm-focused .cm-nonmatchingBracket": {
          color: palette.error,
        },
        ".cm-selectionMatch": {
          backgroundColor: palette.subtleHighlight,
        },
        ".cm-searchMatch": {
          backgroundColor: palette.searchMatch,
          outline: "none",
        },
        ".cm-diagnostic-error": {
          borderLeft: `3px solid ${palette.error}`,
        },
        ".cm-lintPoint-error:after": {
          borderBottomColor: palette.error,
        },
      },
      { dark },
    ),
    syntaxHighlighting(dark ? darkHighlightStyle : lightHighlightStyle),
  ];
}

function fontSizeTheme(fontSize: number): Extension {
  return EditorView.theme({ "&": { fontSize: `${fontSize}px` } });
}

function readOnlyExtension(readOnly: boolean): Extension {
  // Read-only keeps the editor focusable so selection, search and keyboard navigation still work.
  return EditorState.readOnly.of(readOnly);
}

function placeholderFor(text: string | undefined): Extension {
  return text ? placeholderExtension(text) : [];
}

// Where the JSON grammar first fails. JSON.parse messages often carry no position (V8's
// "Unexpected token ... is not valid JSON", WebKit's "JSON Parse error: ..."), so the syntax
// tree locates the problem; null when it does not finish parsing within the budget.
function firstSyntaxErrorPosition(state: EditorState): number | null {
  const tree = ensureSyntaxTree(state, state.doc.length, LINT_PARSE_BUDGET_MS);
  if (!tree) {
    return null;
  }
  let position: number | null = null;
  tree.iterate({
    enter: (node) => {
      if (position !== null) {
        return false;
      }
      if (node.type.isError) {
        position = node.from;
        return false;
      }
      return undefined;
    },
  });
  return position;
}

function positionFromParseMessage(message: string, length: number): number | null {
  const match = /at position (\d+)/.exec(message);
  return match ? Math.min(Number(match[1]), length) : null;
}

// JSON.parse stays the source of truth for validity, matching how every caller reads the
// value. An empty field is a valid "not configured" state, so it is not an error.
function lintJson(view: EditorView): Diagnostic[] {
  const text = view.state.doc.toString();
  if (!text.trim()) {
    return [];
  }
  try {
    JSON.parse(text);
    return [];
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    const position = firstSyntaxErrorPosition(view.state) ?? positionFromParseMessage(message, text.length) ?? 0;
    return [{ from: position, to: position, severity: "error", message }];
  }
}

function nextNonWhitespaceIndex(text: string, start: number): number {
  let index = start;
  while (index < text.length && /\s/.test(text[index])) {
    index += 1;
  }
  return index;
}

// Re-indents JSON text the way JSON.stringify(value, null, 2) lays it out, but only moves
// whitespace: number literals keep their exact digits (no float rounding of large integers)
// and duplicate keys survive, unlike a parse/stringify round trip.
export function formatJsonText(text: string): string | null {
  if (!text.trim()) {
    return null;
  }
  try {
    JSON.parse(text);
  } catch {
    return null;
  }

  let output = "";
  let depth = 0;
  let inString = false;
  let escaped = false;
  for (let index = 0; index < text.length; index += 1) {
    const character = text[index];
    if (inString) {
      output += character;
      if (escaped) {
        escaped = false;
      } else if (character === "\\") {
        escaped = true;
      } else if (character === '"') {
        inString = false;
      }
      continue;
    }

    switch (character) {
      case '"':
        inString = true;
        output += character;
        break;
      case "{":
      case "[": {
        const closingIndex = nextNonWhitespaceIndex(text, index + 1);
        if (text[closingIndex] === (character === "{" ? "}" : "]")) {
          output += character + text[closingIndex];
          index = closingIndex;
          break;
        }
        depth += 1;
        output += `${character}\n${INDENT.repeat(depth)}`;
        break;
      }
      case "}":
      case "]":
        depth -= 1;
        output += `\n${INDENT.repeat(depth)}${character}`;
        break;
      case ",":
        output += `,\n${INDENT.repeat(depth)}`;
        break;
      case ":":
        output += ": ";
        break;
      case " ":
      case "\t":
      case "\n":
      case "\r":
        break;
      default:
        output += character;
    }
  }
  return output;
}

function formatDocument(view: EditorView): boolean {
  if (view.state.readOnly) {
    return false;
  }
  const current = view.state.doc.toString();
  const formatted = formatJsonText(current);
  if (formatted === null) {
    return false;
  }
  if (formatted !== current) {
    view.dispatch({
      changes: { from: 0, to: view.state.doc.length, insert: formatted },
      userEvent: "input.format",
      scrollIntoView: true,
    });
  }
  return true;
}

export function createJsonEditor(options: JsonEditorOptions): JsonEditorController {
  const readOnlyCompartment = new Compartment();
  const wordWrapCompartment = new Compartment();
  const themeCompartment = new Compartment();
  const fontSizeCompartment = new Compartment();
  const placeholderCompartment = new Compartment();
  let lastDiagnosticCount = 0;

  const view = new EditorView({
    parent: options.parent,
    state: EditorState.create({
      doc: options.value,
      extensions: [
        lineNumbers(),
        foldGutter({ markerDOM: foldMarker }),
        highlightActiveLineGutter(),
        highlightSpecialChars(),
        history(),
        drawSelection(),
        dropCursor(),
        EditorState.allowMultipleSelections.of(true),
        EditorState.tabSize.of(INDENT.length),
        indentUnit.of(INDENT),
        indentOnInput(),
        bracketMatching(),
        indentGuides,
        highlightActiveLine(),
        highlightSelectionMatches(),
        search({ top: true }),
        json(),
        linter(lintJson, { delay: LINT_DELAY_MS }),
        // Tooltips render into <body> so dialogs and sheets with overflow clipping or
        // transforms cannot cut them off.
        tooltips({ parent: document.body }),
        keymap.of([
          ...defaultKeymap,
          ...historyKeymap,
          ...foldKeymap,
          ...searchKeymap,
          ...lintKeymap,
          indentWithTab,
        ]),
        baseTheme,
        readOnlyCompartment.of(readOnlyExtension(options.readOnly)),
        wordWrapCompartment.of(options.wordWrap ? EditorView.lineWrapping : []),
        themeCompartment.of(chromeTheme(options.theme)),
        fontSizeCompartment.of(fontSizeTheme(options.fontSize)),
        placeholderCompartment.of(placeholderFor(options.placeholder)),
        EditorView.updateListener.of((update) => {
          if (update.docChanged && !update.transactions.some((transaction) => transaction.annotation(externalChange))) {
            options.onChange(update.state.doc.toString());
          }
          if (update.focusChanged && !update.view.hasFocus) {
            options.onBlur();
          }
          const count = diagnosticCount(update.state);
          if (count !== lastDiagnosticCount) {
            lastDiagnosticCount = count;
            options.onDiagnosticsChange(count);
          }
        }),
      ],
    }),
  });

  return {
    getValue: () => view.state.doc.toString(),
    setValue: (value) => {
      if (value === view.state.doc.toString()) {
        return;
      }
      view.dispatch({
        changes: { from: 0, to: view.state.doc.length, insert: value },
        annotations: externalChange.of(true),
      });
    },
    setReadOnly: (readOnly) => {
      view.dispatch({ effects: readOnlyCompartment.reconfigure(readOnlyExtension(readOnly)) });
    },
    setWordWrap: (wordWrap) => {
      view.dispatch({ effects: wordWrapCompartment.reconfigure(wordWrap ? EditorView.lineWrapping : []) });
    },
    setTheme: (theme) => {
      view.dispatch({ effects: themeCompartment.reconfigure(chromeTheme(theme)) });
    },
    setFontSize: (fontSize) => {
      view.dispatch({ effects: fontSizeCompartment.reconfigure(fontSizeTheme(fontSize)) });
    },
    setPlaceholder: (placeholder) => {
      view.dispatch({ effects: placeholderCompartment.reconfigure(placeholderFor(placeholder)) });
    },
    format: () => formatDocument(view),
    focus: () => view.focus(),
    destroy: () => view.destroy(),
  };
}
