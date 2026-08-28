export type ToolResultCategory = "web_search" | "code_execution" | "image_generation" | "shell" | "generic";

export type ToolResultDescriptor = {
  name?: string;
  type?: string;
  input?: unknown;
  output?: unknown;
};

export type ToolSearchSource = {
  url: string;
  title: string;
  snippet: string;
};

const TOOL_PAYLOAD_MAX_DEPTH = 24;
const TOOL_PAYLOAD_MAX_NODES = 512;
const TOOL_PAYLOAD_MAX_RESULTS = 64;
const TOOL_EMBEDDED_JSON_MAX_CHARS = 64 * 1024;

const SEARCH_CONTAINER_KEYS = new Set([
  "citations",
  "documents",
  "grounding_chunks",
  "grounding_supports",
  "matches",
  "organic",
  "results",
  "search_results",
  "sources",
]);

const SEARCH_URL_KEYS = ["url", "uri", "link", "href", "retrievedUrl", "retrieved_url", "sourceUrl", "source_url"];
const SEARCH_TITLE_KEYS = ["title", "name", "pageTitle", "page_title"];
const SEARCH_SNIPPET_KEYS = ["snippet", "description", "summary", "excerpt", "content", "text"];
const SEARCH_LABELED_SOURCE_PATTERN = /(?:^|\n|["'])\s*(?:title|标题)\s*[:：]\s*([^\r\n]+)\r?\n\s*(?:url|链接)\s*[:：]\s*(https?:\/\/[^\s]+)/gim;
const SEARCH_MARKDOWN_SOURCE_PATTERN = /\[([^\]\r\n]+)\]\((https?:\/\/[^\s)]+)\)/g;

export function isToolPayloadRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

export function readToolPayloadString(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

export function readToolPayloadNumber(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

export function readToolPayloadBoolean(value: unknown): boolean | null {
  return typeof value === "boolean" ? value : null;
}

export function firstToolPayloadString(record: Record<string, unknown>, keys: string[]): string {
  for (const key of keys) {
    const value = readToolPayloadString(record[key]);
    if (value) return value;
  }
  return "";
}

export function formatToolPayload(value: string | undefined): string {
  const text = value?.trim();
  if (!text) return "";
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return text;
  }
}

export function parseToolPayload(value: string | undefined): unknown {
  const text = value?.trim();
  if (!text) return null;
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return text;
  }
}

function parseEmbeddedJSON(value: string): unknown {
  const text = value.trim();
  if (
    text.length < 2
    || text.length > TOOL_EMBEDDED_JSON_MAX_CHARS
    || !((text.startsWith("{") && text.endsWith("}")) || (text.startsWith("[") && text.endsWith("]")))
  ) {
    return null;
  }
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return null;
  }
}

export function walkToolPayload(value: unknown, visit: (item: unknown, key?: string) => void) {
  const pending: Array<{ value: unknown; key?: string; depth: number }> = [{ value, depth: 0 }];
  const visited = new Set<object>();
  let traversed = 0;

  while (pending.length > 0 && traversed < TOOL_PAYLOAD_MAX_NODES) {
    const current = pending.pop();
    if (!current) break;
    traversed += 1;
    visit(current.value, current.key);

    if (current.depth >= TOOL_PAYLOAD_MAX_DEPTH) continue;

    if (typeof current.value === "string") {
      const embedded = parseEmbeddedJSON(current.value);
      if (embedded !== null) {
        pending.push({ value: embedded, key: current.key, depth: current.depth + 1 });
      }
      continue;
    }
    if (current.value === null || typeof current.value !== "object" || visited.has(current.value)) {
      continue;
    }
    visited.add(current.value);

    if (Array.isArray(current.value)) {
      for (let index = current.value.length - 1; index >= 0; index -= 1) {
        if (traversed + pending.length >= TOOL_PAYLOAD_MAX_NODES) break;
        pending.push({ value: current.value[index], depth: current.depth + 1 });
      }
      continue;
    }

    for (const [key, item] of Object.entries(current.value as Record<string, unknown>)) {
      if (traversed + pending.length >= TOOL_PAYLOAD_MAX_NODES) break;
      pending.push({ value: item, key, depth: current.depth + 1 });
    }
  }
}

export function collectToolStrings(value: unknown, keys: string[]): string[] {
  const acceptedKeys = new Set(keys.map(normalizeIdentifier));
  const result = new Set<string>();
  walkToolPayload(value, (item, key) => {
    if (result.size >= TOOL_PAYLOAD_MAX_RESULTS || !key || !acceptedKeys.has(normalizeIdentifier(key))) return;
    const text = readToolPayloadString(item);
    if (text) result.add(text);
  });
  return Array.from(result);
}

function normalizeImageSource(value: string): string {
  const text = value.trim();
  if (!text) return "";
  if (/^(https?:|blob:)/i.test(text)) return text;
  if (/^data:image\/(?:png|jpe?g|webp|gif);base64,/i.test(text)) return text;
  if (/^[A-Za-z0-9+/=\s]+$/.test(text) && text.replace(/\s/g, "").length > 80) {
    return `data:image/png;base64,${text.replace(/\s/g, "")}`;
  }
  return "";
}

export function collectToolImageSources(value: unknown): string[] {
  const result = new Set<string>();
  walkToolPayload(value, (item) => {
    if (result.size >= TOOL_PAYLOAD_MAX_RESULTS) return;
    const source = normalizeImageSource(readToolPayloadString(item));
    if (source) result.add(source);
  });
  return Array.from(result);
}

function normalizeHTTPLink(value: string): string {
  const text = value.trim();
  try {
    const parsed = new URL(text);
    return parsed.protocol === "http:" || parsed.protocol === "https:" ? text : "";
  } catch {
    return "";
  }
}

export function collectToolSearchSources(value: unknown): ToolSearchSource[] {
  const sources = new Map<string, ToolSearchSource>();
  walkToolPayload(value, (item) => {
    if (sources.size >= TOOL_PAYLOAD_MAX_RESULTS) return;
    if (typeof item === "string") {
      collectTextSearchSources(item, sources);
      return;
    }
    if (isToolPayloadRecord(item)) {
      addSearchSource(sources, {
        url: firstToolPayloadString(item, SEARCH_URL_KEYS),
        title: firstToolPayloadString(item, SEARCH_TITLE_KEYS),
        snippet: firstToolPayloadString(item, SEARCH_SNIPPET_KEYS),
      });
    }
  });
  return Array.from(sources.values());
}

function addSearchSource(sources: Map<string, ToolSearchSource>, source: ToolSearchSource) {
  if (sources.size >= TOOL_PAYLOAD_MAX_RESULTS) return;
  const url = normalizeHTTPLink(source.url.replace(/[.,;:!?，。；：！？\])}'"]+$/u, ""));
  if (!url) return;
  const next = { url, title: source.title.trim(), snippet: source.snippet.trim() };
  const current = sources.get(url);
  if (!current || (!current.title && next.title) || (!current.snippet && next.snippet)) {
    sources.set(url, {
      url,
      title: current?.title || next.title,
      snippet: current?.snippet || next.snippet,
    });
  }
}

function collectTextSearchSources(value: string, sources: Map<string, ToolSearchSource>) {
  const text = value.replaceAll("\\r\\n", "\n").replaceAll("\\n", "\n");
  for (const match of text.matchAll(SEARCH_LABELED_SOURCE_PATTERN)) {
    addSearchSource(sources, { title: match[1] || "", url: match[2] || "", snippet: "" });
  }
  for (const match of text.matchAll(SEARCH_MARKDOWN_SOURCE_PATTERN)) {
    addSearchSource(sources, { title: match[1] || "", url: match[2] || "", snippet: "" });
  }
}

export function collectToolNarrativeText(value: unknown): string {
  if (!isToolPayloadRecord(value) || !Array.isArray(value.content)) return "";
  return value.content
    .flatMap((item) => {
      if (!isToolPayloadRecord(item)) return [];
      const text = readToolPayloadString(item.text);
      return text && parseEmbeddedJSON(text) === null ? [text] : [];
    })
    .join("\n\n");
}

export function countToolPayloadItems(value: unknown, keys: string[]): number {
  const acceptedKeys = new Set(keys.map(normalizeIdentifier));
  let count = 0;
  walkToolPayload(value, (item, key) => {
    if (!key || !acceptedKeys.has(normalizeIdentifier(key))) return;
    if (Array.isArray(item)) {
      count = Math.max(count, item.length);
      return;
    }
    const numeric = readToolPayloadNumber(item);
    if (numeric !== null) count = Math.max(count, numeric);
  });
  return count;
}

function normalizeIdentifier(value: string): string {
  return value
    .replace(/([a-z0-9])([A-Z])/g, "$1_$2")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
}

function identifierTokens(...values: Array<string | undefined>): Set<string> {
  return new Set(
    values
      .flatMap((value) => normalizeIdentifier(value || "").split("_"))
      .filter(Boolean),
  );
}

function includesNormalizedIdentifier(values: Array<string | undefined>, candidates: string[]): boolean {
  const normalized = values.map((value) => normalizeIdentifier(value || ""));
  return candidates.some((candidate) => normalized.some((value) => value.includes(candidate)));
}

function hasSearchSemantics(name: string | undefined, type: string | undefined): boolean {
  const values = [name, type];
  const tokens = identifierTokens(...values);
  if (tokens.has("search") || tokens.has("browse") || tokens.has("crawler") || tokens.has("crawling")) {
    return true;
  }
  if (includesNormalizedIdentifier(values, ["url_context", "web_fetch", "open_page", "read_url", "read_page"])) {
    return true;
  }
  return tokens.has("reader") && (tokens.has("url") || tokens.has("web") || tokens.has("page"));
}

function hasSearchPayloadShape(value: unknown): boolean {
  let found = false;
  walkToolPayload(value, (_item, key) => {
    if (key && SEARCH_CONTAINER_KEYS.has(normalizeIdentifier(key))) found = true;
  });
  return found;
}

function hasQueryInput(value: unknown): boolean {
  let found = false;
  walkToolPayload(value, (item, key) => {
    if (!key || !["q", "query", "queries", "search_query", "search_term"].includes(normalizeIdentifier(key))) return;
    found = Boolean(readToolPayloadString(item) || (Array.isArray(item) && item.length > 0));
  });
  return found;
}

export function resolveToolResultCategory(descriptor: ToolResultDescriptor): ToolResultCategory {
  const { name, type, input, output } = descriptor;
  const values = [name, type];

  if (
    hasSearchSemantics(name, type)
    || (collectToolSearchSources(output).length > 0 && (hasSearchPayloadShape(output) || hasQueryInput(input)))
  ) {
    return "web_search";
  }
  if (includesNormalizedIdentifier(values, ["code_interpreter", "code_execution", "execute_code", "python_execution"])) {
    return "code_execution";
  }
  if (includesNormalizedIdentifier(values, ["image_generation", "generate_image", "image_generator"])) {
    return "image_generation";
  }
  if (identifierTokens(...values).has("shell") || includesNormalizedIdentifier(values, ["terminal_execution"])) {
    return "shell";
  }
  return "generic";
}
