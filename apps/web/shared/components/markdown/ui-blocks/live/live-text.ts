// Readable Markdown of a live block, in the state the user left it: the same
// resolution as on screen, written out as text. Leaves that read as one line
// (stats, control values, badges) are gathered into a list; text, callouts and
// catalog components stay paragraphs. Buttons are left out: they do nothing
// outside the block.

import type { BlockIssue, BlockSource, ResolvedNode, UIValue } from "@deeix/core";
import { compileBlock, expandPointProps, formatTemplateNumber, formulaToLatex, mergeState, resolveBlock, takeWhere, WHERE_LIST_PROP } from "@deeix/core";

import { LIVE_COMPILE_OPTIONS, resolveLatestDefinition, type UIBlockRegistry } from "../block";
import { joinTextBlocks, textPair, textTitle, textValue } from "../block-text";
import type { UIBlockState } from "../ui-block-state";
import { optionIndex, readNumber, readOptions, readText } from "./live-context";

type TextPart = { kind: "line" | "block"; text: string };

export function liveBlockText(block: BlockSource, saved: UIBlockState | undefined, registry: UIBlockRegistry): string {
  const compiled = compileBlock(block, LIVE_COMPILE_OPTIONS);
  const state = mergeState(compiled.initialState, saved);
  const { root, values } = resolveBlock(compiled, state);
  return joinParts(nodeText(root, state, values, registry));
}

function nodeText(node: ResolvedNode, state: Readonly<Record<string, UIValue>>, values: Readonly<Record<string, UIValue>>, registry: UIBlockRegistry): TextPart[] {
  const children = () => node.children.flatMap((child) => nodeText(child, state, values, registry));
  const line = (text: string): TextPart[] => (text.trim() ? [{ kind: "line", text }] : []);
  const block = (text: string): TextPart[] => (text.trim() ? [{ kind: "block", text }] : []);
  const titled = (title: string | undefined, parts: TextPart[]): TextPart[] => [...textTitle(title).flatMap(block), ...parts];

  switch (node.component) {
    case "panel":
      return titled(readText(node.props, "title"), [...block(readText(node.props, "description") ?? ""), ...children()]);
    case "card":
    case "collapse":
      return titled(readText(node.props, "title"), children());
    case "stack":
    case "row":
    case "grid":
      return children();
    case "tabs": {
      const options = readOptions(node.props, "items");
      return node.children.flatMap((child, index) => titled(options[index]?.label, nodeText(child, state, values, registry)));
    }
    case "divider":
      return block("---");
    case "text":
      return block(textValue(node.props.text));
    case "callout": {
      const title = readText(node.props, "title");
      const text = textValue(node.props.text);
      return block([title ? `**${title}**` : "", text].filter(Boolean).map((part) => `> ${part}`).join("\n>\n"));
    }
    case "stat":
      return line(textPair(readText(node.props, "label"), node.props.value === undefined ? "—" : textValue(node.props.value), readText(node.props, "unit")));
    case "badge":
      return line(textValue(node.props.text ?? node.props.label));
    case "progress": {
      const max = readNumber(node.props, "max") ?? 100;
      const value = readNumber(node.props, "value") ?? 0;
      const percent = max > 0 ? Math.round(Math.min(1, Math.max(0, value / max)) * 100) : 0;
      return line(textPair(readText(node.props, "label"), `${percent}%`));
    }
    case "slider":
    case "switch":
    case "input":
      return node.bind ? line(textPair(readText(node.props, "label"), textValue(state[node.bind]), readText(node.props, "unit"))) : [];
    case "select":
    case "segmented": {
      if (!node.bind) {
        return [];
      }
      const options = readOptions(node.props, "options");
      const index = optionIndex(options, state[node.bind]);
      return line(textPair(readText(node.props, "label"), index >= 0 ? (options[index]?.label ?? "") : textValue(state[node.bind])));
    }
    case "formula":
      return line(formulaText(node, values));
    case "scene":
    case "button":
      // A diagram and a button have no text of their own.
      return [];
    default:
      return block(catalogText(node, registry));
  }
}

// `F_g = m * g = 88.2 N`: the formula as written, then its current result.
function formulaText(node: ResolvedNode, values: Readonly<Record<string, UIValue>>): string {
  const expr = readText(node.props, "expr");
  if (!expr) {
    return "";
  }
  const name = readText(node.props, "name");
  const unit = readText(node.props, "unit");
  let result = "";
  try {
    const value = formulaToLatex(expr, { values, with: node.props.with }).result;
    if (value !== undefined) {
      const digits = Math.min(6, Math.max(0, Math.round(readNumber(node.props, "precision") ?? 2)));
      result = ` = ${formatTemplateNumber(Number(value.toFixed(digits)))}${unit ? ` ${unit}` : ""}`;
    }
  } catch {
    // An invalid formula is copied as written.
  }
  return `${name ? `${name} = ` : ""}${expr}${result}`;
}

// A nested catalog component, with per-point data expanded and `where`
// applied; its own state is local to the screen, so it reads as declared.
function catalogText(node: ResolvedNode, registry: UIBlockRegistry): string {
  const definition = resolveLatestDefinition(registry, node.component);
  if (!definition) {
    return "";
  }
  const issues: BlockIssue[] = [];
  const { props, where } = takeWhere(node.component, expandPointProps(node.component, node.props, issues), issues);
  let finalProps = props;
  if (where) {
    const listProp = WHERE_LIST_PROP[node.component] ?? "";
    const list = props[listProp];
    finalProps = { ...props, [listProp]: Array.isArray(list) ? list.filter((row, index) => where(row, index)) : [] };
  }
  return definition.toText(finalProps, undefined) ?? "";
}

function joinParts(parts: TextPart[]): string {
  const blocks: string[] = [];
  let lines: string[] = [];
  const flush = () => {
    if (lines.length > 0) {
      blocks.push(lines.map((text) => `- ${text}`).join("\n"));
      lines = [];
    }
  };
  for (const part of parts) {
    if (part.kind === "line") {
      lines.push(part.text);
    } else {
      flush();
      blocks.push(part.text);
    }
  }
  flush();
  return joinTextBlocks(blocks);
}
