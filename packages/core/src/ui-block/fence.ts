// Finds interactive blocks in a message's Markdown: `deeix-ui` blocks (JSON,
// interpreted by this engine) and `deeix-widget` illustrations (HTML and script
// the model wrote, run by the host in a sandboxed frame).
//
// The renderer sees one fence at a time; the chat side needs the whole message:
// which blocks it has (to drop state of blocks an edit removed), their titles
// (the server does not store them) and which keys repeat. A repeated key cannot
// tell its blocks apart, so those blocks keep state locally and never store it.

import { hashText, parseBlockSource } from "./block.ts";

export const UI_BLOCK_FENCE_LANGUAGE = "deeix-ui";
export const UI_WIDGET_FENCE_LANGUAGE = "deeix-widget";

// An illustration larger than this is not rendered (shown as source instead).
export const UI_WIDGET_MAX_BYTES = 200_000;

// The block format this engine renders, sent with each turn as `uiBlockVersion`
// so the server describes live blocks (state, computed, primitives) to the model.
// Clients that send nothing get static components only.
export const UI_BLOCK_FORMAT_VERSION = 2;

export type UIBlockFence = {
  kind: "block" | "widget";
  body: string;
  // The closing fence was found; an open fence runs to the end of the text.
  closed: boolean;
  // Lines of the whole fence, opening and closing lines included: [start, end).
  startLine: number;
  endLine: number;
};

const OPEN_FENCE = /^ {0,3}(`{3,}|~{3,})(.*)$/;

// CommonMark fences: up to three spaces of indent, three or more backticks or
// tildes, closed by the same character at least as long. Fences of any other
// language are skipped as a whole, so an example inside a code block is not a block.
export function scanUIBlockFences(markdown: string): UIBlockFence[] {
  const fences: UIBlockFence[] = [];
  const lines = markdown.split(/\r?\n/);
  let index = 0;
  while (index < lines.length) {
    const startLine = index;
    const open = OPEN_FENCE.exec(lines[index] ?? "");
    index += 1;
    if (!open) {
      continue;
    }
    const marker = open[1] ?? "```";
    const info = open[2] ?? "";
    // A backtick in the info string means inline code, not a fence.
    if (marker.startsWith("`") && info.includes("`")) {
      continue;
    }
    const language = (info.trim().split(/\s/)[0] ?? "").toLowerCase();
    const close = new RegExp(`^ {0,3}${marker.startsWith("`") ? "`" : "~"}{${marker.length},}\\s*$`);
    const body: string[] = [];
    let closed = false;
    while (index < lines.length) {
      const line = lines[index] ?? "";
      index += 1;
      if (close.test(line)) {
        closed = true;
        break;
      }
      body.push(line);
    }
    if (language === UI_BLOCK_FENCE_LANGUAGE || language === UI_WIDGET_FENCE_LANGUAGE) {
      fences.push({ kind: language === UI_BLOCK_FENCE_LANGUAGE ? "block" : "widget", body: body.join("\n"), closed, startLine, endLine: index });
    }
  }
  return fences;
}

// Replaces each block fence with what `replace` returns; `null` keeps the fence.
// Everything else, including line breaks, is left as it was.
export function replaceUIBlockFences(markdown: string, replace: (fence: UIBlockFence) => string | null): string {
  const fences = scanUIBlockFences(markdown);
  if (fences.length === 0) {
    return markdown;
  }
  const lines = markdown.split(/\r?\n/);
  const out: string[] = [];
  let cursor = 0;
  for (const fence of fences) {
    const replacement = replace(fence);
    if (replacement === null) {
      continue;
    }
    out.push(...lines.slice(cursor, fence.startLine), replacement);
    cursor = fence.endLine;
  }
  out.push(...lines.slice(cursor));
  return out.join("\n");
}

export type MessageUIBlock = {
  key: string;
  title?: string;
};

export type MessageUIBlocks = {
  // Blocks with a key of their own, in message order.
  blocks: MessageUIBlock[];
  // Keys used by more than one block.
  duplicates: ReadonlySet<string>;
};

export function readMessageUIBlocks(markdown: string): MessageUIBlocks {
  const seen = new Map<string, MessageUIBlock>();
  const duplicates = new Set<string>();
  const add = (key: string, title: string) => {
    if (seen.has(key)) {
      duplicates.add(key);
      return;
    }
    seen.set(key, title ? { key, title } : { key });
  };
  for (const fence of scanUIBlockFences(markdown)) {
    if (fence.kind === "widget") {
      if (fence.closed) {
        add(widgetKey(fence.body), widgetTitle(fence.body));
      }
      continue;
    }
    const parsed = parseBlockSource(fence.body, !fence.closed);
    if (parsed.status !== "ok" || parsed.block.key === "") {
      continue;
    }
    const { key, root } = parsed.block;
    add(key, typeof root.props.title === "string" ? root.props.title.trim() : "");
  }
  return { blocks: Array.from(seen.values()).filter((block) => !duplicates.has(block.key)), duplicates };
}

// An illustration has no id of its own: it is keyed by its source, so the
// renderer, the side panel and the message scan agree without coordination.
// Surrounding whitespace does not count.
export function widgetKey(source: string): string {
  return `widget-${hashText(source.trim())}`;
}

// The illustration's own name, for the model and the side panel: its <title>,
// else its first heading, as plain text.
export function widgetTitle(source: string): string {
  const raw = elementText(source, ["title"]) ?? elementText(source, ["h1", "h2", "h3"]) ?? "";
  const text = stripTags(raw)
    .replace(/&(?:amp|lt|gt|quot|#39|nbsp);/g, (entity) => HTML_ENTITIES[entity] ?? entity)
    .replace(/\s+/g, " ")
    .trim();
  return Array.from(text).slice(0, 40).join("");
}

const HTML_ENTITIES: Readonly<Record<string, string>> = { "&amp;": "&", "&lt;": "<", "&gt;": ">", "&quot;": '"', "&#39;": "'", "&nbsp;": " " };

// The content of the first element named one of `names`, found with indexOf
// rather than a regex: the source is model output, and lazy patterns over it
// backtrack quadratically on inputs like "<title<title<title…".
function elementText(source: string, names: readonly string[]): string | undefined {
  const lower = source.toLowerCase();
  for (let open = lower.indexOf("<"); open !== -1; open = lower.indexOf("<", open + 1)) {
    const name = names.find((candidate) => lower.startsWith(candidate, open + 1) && /[\s>/]/.test(lower[open + 1 + candidate.length] ?? ""));
    if (!name) {
      continue;
    }
    const start = lower.indexOf(">", open);
    if (start === -1) {
      return undefined;
    }
    const end = lower.indexOf(`</${name}`, start + 1);
    return end === -1 ? undefined : source.slice(start + 1, end);
  }
  return undefined;
}

// Drops markup and keeps the text between tags. Angle brackets that survive
// (an unclosed "<b") go too: the result is shown as plain text and sent to the
// model, never parsed as HTML.
function stripTags(value: string): string {
  let text = "";
  let inTag = false;
  for (const char of value) {
    if (char === "<") {
      inTag = true;
    } else if (char === ">") {
      inTag = false;
    } else if (!inTag) {
      text += char;
    }
  }
  return text;
}
