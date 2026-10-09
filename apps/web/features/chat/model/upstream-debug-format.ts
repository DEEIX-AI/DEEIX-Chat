import { isRecord } from "@/shared/lib/type-guards";

// Upstream debug bodies arrive as single-line JSON (the server re-serializes them after redaction),
// so prompts show up as one huge string full of `\n` and `\"`. The readable view below lays the
// payload out as an indented key/value tree and prints multi-line strings as real text blocks.

export type UpstreamDebugBody =
  | { kind: "empty" }
  | { kind: "json"; value: unknown }
  | { kind: "sse"; events: unknown[] }
  | { kind: "text"; text: string };

export type UpstreamDebugLine = {
  depth: number;
  label?: string;
  // tag is the role / type of an array item ("system", "user", "text"), shown next to its "#n" label.
  tag?: string;
  value?: string;
  // block marks one line of a multi-line string; it is rendered as plain text without a label.
  block?: boolean;
};

export function parseUpstreamDebugBody(raw: string): UpstreamDebugBody {
  const value = raw.trim();
  if (!value) {
    return { kind: "empty" };
  }
  if (/(^|\n)\s*data:\s*/.test(value)) {
    const events = value
      .split(/\r?\n/)
      .map((line) => line.trim())
      // Streamed bodies start with a status line such as "HTTP 200,"; only data lines carry payloads.
      .filter((line) => line.startsWith("data:"))
      .map((line) => line.slice("data:".length).trim())
      .filter((line) => line && line !== "[DONE]")
      .map(parseJSONOrText);
    if (events.length > 0) {
      return { kind: "sse", events };
    }
  }
  try {
    return { kind: "json", value: JSON.parse(value) };
  } catch {
    return { kind: "text", text: value };
  }
}

export function formatUpstreamDebugLines(body: UpstreamDebugBody): UpstreamDebugLine[] {
  const lines: UpstreamDebugLine[] = [];
  switch (body.kind) {
    case "empty":
      break;
    case "text":
      pushBlock(lines, body.text, 0);
      break;
    case "json":
      appendValue(lines, body.value, 0);
      break;
    case "sse":
      body.events.forEach((event, index) => {
        lines.push({ depth: 0, label: `event ${index + 1}` });
        appendValue(lines, event, 1);
      });
      break;
  }
  return lines;
}

// formatUpstreamDebugRaw is the exact body, pretty-printed when it is JSON, for copying and the raw view.
export function formatUpstreamDebugRaw(raw: string): string {
  const value = raw.trim();
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

export function formatUpstreamDebugSize(raw: string): string {
  const bytes = new TextEncoder().encode(raw).length;
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

function parseJSONOrText(line: string): unknown {
  try {
    return JSON.parse(line);
  } catch {
    return line;
  }
}

function appendValue(lines: UpstreamDebugLine[], value: unknown, depth: number) {
  if (Array.isArray(value)) {
    value.forEach((item, index) => {
      appendEntry(lines, `#${index + 1}`, item, depth, arrayItemTag(item));
    });
    return;
  }
  if (isRecord(value)) {
    for (const [key, item] of Object.entries(value)) {
      appendEntry(lines, key, item, depth);
    }
    return;
  }
  if (typeof value === "string" && value.includes("\n")) {
    pushBlock(lines, value, depth);
    return;
  }
  lines.push({ depth, value: formatScalar(value) });
}

function appendEntry(lines: UpstreamDebugLine[], label: string, value: unknown, depth: number, tag?: string) {
  if (Array.isArray(value)) {
    if (value.length === 0) {
      lines.push({ depth, label, tag, value: "[]" });
      return;
    }
    lines.push({ depth, label: `${label} (${value.length})`, tag });
    appendValue(lines, value, depth + 1);
    return;
  }
  if (isRecord(value)) {
    if (Object.keys(value).length === 0) {
      lines.push({ depth, label, tag, value: "{}" });
      return;
    }
    lines.push({ depth, label, tag });
    appendValue(lines, value, depth + 1);
    return;
  }
  if (typeof value === "string" && value.includes("\n")) {
    lines.push({ depth, label, tag });
    pushBlock(lines, value, depth + 1);
    return;
  }
  lines.push({ depth, label, tag, value: formatScalar(value) });
}

// Chat payloads are arrays of messages / parts; tagging items by their role or type makes them scannable.
function arrayItemTag(item: unknown): string | undefined {
  if (!isRecord(item)) {
    return undefined;
  }
  for (const key of ["role", "type"]) {
    const tag = item[key];
    if (typeof tag === "string" && tag.trim()) {
      return tag.trim();
    }
  }
  return undefined;
}

function pushBlock(lines: UpstreamDebugLine[], text: string, depth: number) {
  for (const line of text.replace(/\r\n/g, "\n").split("\n")) {
    lines.push({ depth, value: line, block: true });
  }
}

function formatScalar(value: unknown): string {
  if (typeof value === "string") {
    return value === "" ? '""' : value;
  }
  if (value === null || value === undefined) {
    return "null";
  }
  return typeof value === "object" ? JSON.stringify(value) : String(value);
}
