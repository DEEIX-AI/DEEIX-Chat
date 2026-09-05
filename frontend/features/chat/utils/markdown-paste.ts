import TurndownService from "turndown";
import { gfm } from "turndown-plugin-gfm";

import { CODE_BLOCK_PLAIN_TEXT_MIME } from "@/shared/lib/clipboard";

type ClipboardMarkdownPaste = {
  block: boolean;
  markdown: string;
};

const VSCODE_EDITOR_DATA_MIME = "vscode-editor-data";
const VSCODE_NON_CODE_MODES = new Set(["markdown", "mdx", "plaintext"]);
const VSCODE_MARKDOWN_LANGUAGE_ALIASES: Readonly<Record<string, string>> = {
  javascriptreact: "jsx",
  shellscript: "bash",
  typescriptreact: "tsx",
};
const HTML_BLOCK_SELECTOR = [
  "address",
  "article",
  "aside",
  "blockquote",
  "details",
  "div",
  "dl",
  "fieldset",
  "figure",
  "footer",
  "form",
  "h1",
  "h2",
  "h3",
  "h4",
  "h5",
  "h6",
  "header",
  "hr",
  "main",
  "nav",
  "ol",
  "p",
  "pre",
  "section",
  "summary",
  "table",
  "ul",
].join(",");

const turndownService = new TurndownService({
  bulletListMarker: "-",
  codeBlockStyle: "fenced",
  emDelimiter: "*",
  headingStyle: "atx",
  strongDelimiter: "**",
});
turndownService.use(gfm);
turndownService.remove(["script", "style", "noscript"]);

function formatFencedCode(code: string, language: string): string {
  const normalizedCode = code.replace(/\r\n?/g, "\n");
  const longestBacktickRun = Math.max(0, ...(normalizedCode.match(/`+/g)?.map((run) => run.length) ?? []));
  const fence = "`".repeat(Math.max(3, longestBacktickRun + 1));
  return `${fence}${language}\n${normalizedCode}${normalizedCode.endsWith("\n") ? "" : "\n"}${fence}`;
}

function resolveVSCodeCodePaste(clipboardData: DataTransfer): ClipboardMarkdownPaste | null {
  const rawMetadata = clipboardData.getData(VSCODE_EDITOR_DATA_MIME);
  if (!rawMetadata) {
    return null;
  }

  try {
    const metadata = JSON.parse(rawMetadata) as { mode?: unknown; version?: unknown };
    if (metadata.version !== 1 || typeof metadata.mode !== "string") {
      return null;
    }

    const mode = metadata.mode.trim().toLowerCase();
    const code = clipboardData.getData("text/plain");
    if (!mode || !code || VSCODE_NON_CODE_MODES.has(mode)) {
      return null;
    }

    const safeMode = /^[a-z0-9_+#.-]+$/.test(mode) ? mode : "";
    const language = VSCODE_MARKDOWN_LANGUAGE_ALIASES[safeMode] ?? safeMode;
    return { block: true, markdown: formatFencedCode(code, language) };
  } catch {
    return null;
  }
}

function containsBlockHTML(root: ParentNode): boolean {
  return Boolean(root.querySelector(HTML_BLOCK_SELECTOR));
}

function normalizeText(value: string): string {
  return value.replace(/\r\n?/g, "\n").trim();
}

function normalizeTextFlow(value: string): string {
  return normalizeText(value).replace(/\s+/g, " ");
}

function containsMarkdownFormatting(root: ParentNode): boolean {
  return Boolean(
    root.querySelector(
      "a[href], b, blockquote, code, del, em, h1, h2, h3, h4, h5, h6, hr, i, img[src], ol, pre, s, strike, strong, table, ul",
    ),
  );
}

function losesSourceLineBreaks(markdown: string, plainText: string): boolean {
  const markdownLines = normalizeText(markdown).split("\n");
  const sourceLines = normalizeText(plainText).split("\n");
  return (
    markdownLines.length < sourceLines.length ||
    markdownLines.filter((line) => line.trim()).length < sourceLines.filter((line) => line.trim()).length
  );
}

function resolveRichTextMarkdownPaste(clipboardData: DataTransfer): ClipboardMarkdownPaste | null {
  const html = clipboardData.getData("text/html");
  const plainText = clipboardData.getData("text/plain");
  if (!html.trim() || !plainText.trim()) {
    return null;
  }

  const body = new DOMParser().parseFromString(html, "text/html").body;
  if (!containsMarkdownFormatting(body)) {
    return null;
  }

  const markdown = turndownService.turndown(body).trim();
  if (!markdown) {
    return null;
  }

  const normalizedMarkdown = normalizeText(markdown);
  const normalizedPlainText = normalizeText(plainText);
  const normalizedEscapedPlainText = normalizeText(turndownService.escape(plainText));
  const normalizedMarkdownFlow = normalizeTextFlow(markdown);
  if (
    normalizedMarkdown === normalizedPlainText ||
    normalizedMarkdown === normalizedEscapedPlainText ||
    normalizedMarkdownFlow === normalizeTextFlow(plainText) ||
    normalizedMarkdownFlow === normalizeTextFlow(normalizedEscapedPlainText) ||
    losesSourceLineBreaks(markdown, plainText)
  ) {
    return null;
  }

  return {
    block: containsBlockHTML(body),
    markdown,
  };
}

export function resolveClipboardMarkdownPaste(clipboardData: DataTransfer): ClipboardMarkdownPaste | null {
  if (clipboardData.getData(CODE_BLOCK_PLAIN_TEXT_MIME)) {
    return null;
  }
  if (clipboardData.getData(VSCODE_EDITOR_DATA_MIME)) {
    return resolveVSCodeCodePaste(clipboardData);
  }
  return resolveRichTextMarkdownPaste(clipboardData);
}

export function formatClipboardMarkdownPaste(
  draft: string,
  selectionStart: number,
  selectionEnd: number,
  paste: ClipboardMarkdownPaste,
): { caretIndex: number; value: string } {
  const before = draft.slice(0, selectionStart);
  const after = draft.slice(selectionEnd);
  const leadingBoundary = paste.block && before && !before.endsWith("\n") ? "\n" : "";
  const trailingBoundary = paste.block && after && !after.startsWith("\n") ? "\n" : "";
  const replacement = `${leadingBoundary}${paste.markdown}${trailingBoundary}`;

  return {
    caretIndex: before.length + replacement.length,
    value: `${before}${replacement}${after}`,
  };
}
