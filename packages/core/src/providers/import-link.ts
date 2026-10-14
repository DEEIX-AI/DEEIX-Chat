// Provider import links.
//
//   v=1: /import#v=1&url=<base URL>&key=<API key>[&protocol=<protocol>][&name=<name>]
//   v=2: /import#v=2&url=…&key=…[&protocol=…][&name=…]&url=…&key=…
//
// Parameters live in the fragment, which is never sent to the server, so keys
// stay out of access logs, proxies and Referer headers. In v=2 each entry
// starts at `url` and runs to the next `url`.
//
// Parsing is strict: any unknown, repeated or invalid parameter rejects the
// whole link. The server validates again on save.

import { normalizeApiBaseUrl } from "../server/url.ts";

const PROVIDER_IMPORT_LINK_VERSION = "1";
const PROVIDER_IMPORT_MULTI_LINK_VERSION = "2";
const SLASH = "/".charCodeAt(0);

/** Route of the import page on the web client. */
const PROVIDER_IMPORT_PATH = "/import";

/** Protocols a link may preselect. The server owns the authoritative list. */
export const PROVIDER_IMPORT_PROTOCOLS = [
  "openai_chat_completions",
  "openai_responses",
  "anthropic_messages",
  "google_generate_content",
  "gemini_interactions",
  "xai_responses",
  "openrouter_chat_completions",
  "openrouter_responses",
] as const;

/** Matches the highest per-user provider limit. */
export const MAX_PROVIDER_IMPORT_ENTRIES = 50;

export type ProviderImportProtocol = (typeof PROVIDER_IMPORT_PROTOCOLS)[number];

export type ProviderImportLink = {
  baseURL: string;
  apiKey: string;
  /** Present only when the link names a supported protocol. */
  protocol: ProviderImportProtocol | null;
  /** Trimmed display name; empty when the link has none. */
  name: string;
  /** Lowercase host of baseURL, shown prominently before the user confirms. */
  host: string;
};

export type ProviderImportLinkError =
  | "empty"
  | "unsupported_version"
  | "unknown_parameter"
  | "duplicate_parameter"
  | "missing_url"
  | "invalid_url"
  | "insecure_url"
  | "missing_key"
  | "invalid_key"
  | "invalid_protocol"
  | "invalid_name"
  | "too_many_entries"
  | "duplicate_entry";

export type ProviderImportLinkResult =
  | { ok: true; links: ProviderImportLink[] }
  | {
      ok: false;
      error: ProviderImportLinkError;
      /** 1-based entry index; v=2 only. */
      entry?: number;
    };

type EntryResult = { ok: true; link: ProviderImportLink } | { ok: false; error: ProviderImportLinkError };

const ALLOWED_PARAMETERS = new Set(["v", "url", "key", "protocol", "name"]);
/** Bounds parsing work; fields have their own limits. */
const MAX_FRAGMENT_LENGTH = 64 * 1024;
const MAX_URL_LENGTH = 512;
const MAX_KEY_LENGTH = 512;
const MAX_NAME_LENGTH = 64;

/** Parses an import-link fragment, with or without the leading `#`. */
export function parseProviderImportLink(fragment: string): ProviderImportLinkResult {
  const raw = fragment.startsWith("#") ? fragment.slice(1) : fragment;
  if (!raw.trim() || raw.length > MAX_FRAGMENT_LENGTH) {
    return { ok: false, error: "empty" };
  }

  const pairs: Array<[string, string]> = [];
  for (const pair of raw.split("&")) {
    if (!pair) continue;
    const separator = pair.indexOf("=");
    const rawName = separator < 0 ? pair : pair.slice(0, separator);
    const rawValue = separator < 0 ? "" : pair.slice(separator + 1);
    let name: string;
    let value: string;
    try {
      name = decodeURIComponent(rawName);
      value = decodeURIComponent(rawValue.replace(/\+/g, "%20"));
    } catch {
      return { ok: false, error: "invalid_url" };
    }
    if (!ALLOWED_PARAMETERS.has(name)) {
      return { ok: false, error: "unknown_parameter" };
    }
    pairs.push([name, value]);
  }

  const versions = pairs.filter(([name]) => name === "v");
  if (versions.length > 1) {
    return { ok: false, error: "duplicate_parameter" };
  }
  const fields = pairs.filter(([name]) => name !== "v");
  // A missing version means v1.
  const version = versions[0]?.[1] ?? PROVIDER_IMPORT_LINK_VERSION;
  if (version === PROVIDER_IMPORT_LINK_VERSION) {
    const values = new Map<string, string>();
    for (const [name, value] of fields) {
      if (values.has(name)) {
        return { ok: false, error: "duplicate_parameter" };
      }
      values.set(name, value);
    }
    const entry = parseEntry(values);
    return entry.ok ? { ok: true, links: [entry.link] } : entry;
  }
  if (version === PROVIDER_IMPORT_MULTI_LINK_VERSION) {
    return parseMultiEntryFields(fields);
  }
  return { ok: false, error: "unsupported_version" };
}

function parseMultiEntryFields(fields: ReadonlyArray<[string, string]>): ProviderImportLinkResult {
  const groups: Array<Map<string, string>> = [];
  for (const [name, value] of fields) {
    if (name === "url") {
      groups.push(new Map());
    }
    const current = groups[groups.length - 1];
    // Parameters before the first `url` belong to no entry.
    if (!current) {
      return { ok: false, error: "missing_url", entry: 1 };
    }
    if (current.has(name)) {
      return { ok: false, error: "duplicate_parameter", entry: groups.length };
    }
    current.set(name, value);
  }
  if (groups.length === 0) {
    return { ok: false, error: "missing_url" };
  }
  if (groups.length > MAX_PROVIDER_IMPORT_ENTRIES) {
    return { ok: false, error: "too_many_entries" };
  }

  const links: ProviderImportLink[] = [];
  const seen = new Set<string>();
  for (const [index, values] of groups.entries()) {
    const entry = parseEntry(values);
    if (!entry.ok) {
      return { ...entry, entry: index + 1 };
    }
    // The server rejects a repeated URL and key pair.
    const identity = `${entry.link.baseURL}\n${entry.link.apiKey}`;
    if (seen.has(identity)) {
      return { ok: false, error: "duplicate_entry", entry: index + 1 };
    }
    seen.add(identity);
    links.push(entry.link);
  }
  return { ok: true, links };
}

function parseEntry(values: ReadonlyMap<string, string>): EntryResult {
  const urlValue = values.get("url")?.trim() ?? "";
  if (!urlValue) {
    return { ok: false, error: "missing_url" };
  }
  const endpoint = normalizeImportBaseURL(urlValue);
  if (!endpoint.ok) {
    return endpoint;
  }

  const apiKey = values.get("key")?.trim() ?? "";
  if (!apiKey) {
    return { ok: false, error: "missing_key" };
  }
  if (apiKey.length > MAX_KEY_LENGTH || !/^[\x21-\x7e]+$/.test(apiKey)) {
    return { ok: false, error: "invalid_key" };
  }

  const protocolValue = values.get("protocol")?.trim() ?? "";
  let protocol: ProviderImportProtocol | null = null;
  if (protocolValue) {
    if (!isProviderImportProtocol(protocolValue)) {
      return { ok: false, error: "invalid_protocol" };
    }
    protocol = protocolValue;
  }

  const name = (values.get("name") ?? "").trim();
  if (name.length > MAX_NAME_LENGTH || hasControlCharacter(name)) {
    return { ok: false, error: "invalid_name" };
  }

  return { ok: true, link: { baseURL: endpoint.baseURL, apiKey, protocol, name, host: endpoint.host } };
}

export type ProviderImportLinkInput = {
  baseURL?: string;
  apiKey?: string;
  protocol?: ProviderImportProtocol | null;
  name?: string;
};

/**
 * Builds an import link. A missing URL or key is written as the `{address}` /
 * `{key}` placeholder. A single entry stays v=1 so older clients can open it.
 */
export function formatProviderImportLink(
  origin: string,
  input: ProviderImportLinkInput | readonly ProviderImportLinkInput[] = {},
): string {
  const entries = isEntryList(input) ? input : [input];
  const version = entries.length > 1 ? PROVIDER_IMPORT_MULTI_LINK_VERSION : PROVIDER_IMPORT_LINK_VERSION;
  const params = [`v=${version}`, ...(entries.length > 0 ? entries : [{}]).flatMap(entryParams)];
  return `${trimOrigin(origin)}${PROVIDER_IMPORT_PATH}#${params.join("&")}`;
}

function entryParams(input: ProviderImportLinkInput): string[] {
  const baseURL = input.baseURL?.trim();
  const apiKey = input.apiKey?.trim();
  const name = input.name?.trim();
  const params = [
    `url=${baseURL ? encodeURIComponent(baseURL) : "{address}"}`,
    `key=${apiKey ? encodeURIComponent(apiKey) : "{key}"}`,
  ];
  if (input.protocol) params.push(`protocol=${encodeURIComponent(input.protocol)}`);
  if (name) params.push(`name=${encodeURIComponent(name)}`);
  return params;
}

function isEntryList(
  input: ProviderImportLinkInput | readonly ProviderImportLinkInput[],
): input is readonly ProviderImportLinkInput[] {
  return Array.isArray(input);
}

/** Drop trailing slashes in one pass. A regex like /\/+$/ backtracks quadratically on a long run of slashes. */
function trimOrigin(origin: string): string {
  const trimmed = origin.trim();
  let end = trimmed.length;
  while (end > 0 && trimmed.charCodeAt(end - 1) === SLASH) end -= 1;
  return trimmed.slice(0, end);
}

/** Show only enough of a key to recognise it; never enough to reuse it. */
export function maskProviderApiKey(apiKey: string): string {
  const key = apiKey.trim();
  if (!key) return "";
  // Long keys keep their prefix and tail so the user can tell them apart; short ones
  // keep only a short tail, the very short none: revealing 7 of 8 characters is no mask.
  const [head, tail] = key.length >= 12 ? [3, 4] : key.length >= 6 ? [0, 2] : [0, 0];
  const hidden = key.length - head - tail;
  return `${key.slice(0, head)}${"•".repeat(Math.min(Math.max(hidden, 4), 8))}${tail ? key.slice(-tail) : ""}`;
}

function hasControlCharacter(value: string): boolean {
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index);
    if (code < 0x20 || code === 0x7f) return true;
  }
  return false;
}

function isProviderImportProtocol(value: string): value is ProviderImportProtocol {
  return (PROVIDER_IMPORT_PROTOCOLS as readonly string[]).includes(value);
}

type NormalizedEndpoint = { ok: true; baseURL: string; host: string } | { ok: false; error: ProviderImportLinkError };

// Parsed by hand, like normalizeApiBaseUrl, so the result is identical on browsers, Node and Hermes.
// normalizeApiBaseUrl already rejects userinfo, query and fragment by construction.
function normalizeImportBaseURL(raw: string): NormalizedEndpoint {
  if (raw.length > MAX_URL_LENGTH) {
    return { ok: false, error: "invalid_url" };
  }
  const scheme = raw.slice(0, raw.indexOf(":")).toLowerCase();
  if (scheme === "http") {
    return { ok: false, error: "insecure_url" };
  }
  if (scheme !== "https") {
    return { ok: false, error: "invalid_url" };
  }
  const baseURL = normalizeApiBaseUrl(raw);
  if (!baseURL) {
    return { ok: false, error: "invalid_url" };
  }
  const authority = baseURL.slice("https://".length).split("/")[0] ?? "";
  const host = authority.startsWith("[") ? authority.slice(0, authority.indexOf("]") + 1) : authority.split(":")[0];
  return { ok: true, baseURL, host };
}
