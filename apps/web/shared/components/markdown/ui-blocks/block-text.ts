// Readable Markdown for interactive blocks: what a block shows, in the state the
// user left it, for copying a reply somewhere the block cannot render. Each
// catalog component describes itself (`toText` on its definition); these are
// the shared pieces. Output carries only the block's own words, so it needs no
// translation.

import { formatTemplateNumber } from "@deeix/core";

export function textTitle(title: string | undefined): string[] {
  const trimmed = title?.trim();
  return trimmed ? [`**${escapeInline(trimmed)}**`] : [];
}

export function textValue(value: unknown): string {
  if (typeof value === "number") {
    return formatTemplateNumber(value);
  }
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "boolean") {
    return value ? "✓" : "✗";
  }
  if (value === null || value === undefined) {
    return "";
  }
  return JSON.stringify(value);
}

// "label: value unit", leaving out whatever is missing.
export function textPair(label: string | undefined, value: string, unit?: string): string {
  const shown = [value, unit?.trim()].filter(Boolean).join(" ");
  return label?.trim() ? `${label.trim()}: ${shown}` : shown;
}

export function textTable(headers: readonly string[], rows: readonly (readonly string[])[], maxRows = MAX_TABLE_ROWS): string[] {
  if (headers.length === 0) {
    return [];
  }
  const cell = (value: string) => escapeCell(value) || " ";
  const lines = [`| ${headers.map(cell).join(" | ")} |`, `| ${headers.map(() => "---").join(" | ")} |`];
  for (const row of rows.slice(0, maxRows)) {
    lines.push(`| ${headers.map((_, index) => cell(row[index] ?? "")).join(" | ")} |`);
  }
  if (rows.length > maxRows) {
    lines.push(`| … ${rows.length - maxRows} |${" |".repeat(Math.max(0, headers.length - 1))}`);
  }
  return lines;
}

export function joinTextBlocks(parts: readonly string[]): string {
  return parts.map((part) => part.trim()).filter(Boolean).join("\n\n");
}

const MAX_TABLE_ROWS = 200;

// Backslashes are escaped too: otherwise a "\" in the text would escape the
// "\" added before a pipe or asterisk and break the table or title.
function escapeCell(value: string): string {
  return value.replace(/\r?\n/g, " ").replace(/[\\|]/g, "\\$&").trim();
}

function escapeInline(value: string): string {
  return value.replace(/[\\*]/g, "\\$&");
}
