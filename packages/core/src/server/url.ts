// Server address handling. Every client connects to a user-operated server, so
// the API base URL is runtime data that must be validated before use.

const LOOPBACK_HOSTS = new Set(["localhost", "127.0.0.1", "::1", "[::1]"]);

/** Drop trailing slashes in linear time (a `/\/+$/` regex backtracks quadratically). */
function trimTrailingSlashes(value: string): string {
  let end = value.length;
  while (end > 0 && value.charCodeAt(end - 1) === 47) {
    end -= 1;
  }
  return value.slice(0, end);
}
const DEFAULT_DEV_API_PORT = "8080";

// Strict absolute http(s) URL: scheme, host (DNS name, IPv4 or bracketed IPv6),
// optional port, optional path. Userinfo, query and fragment are rejected by
// construction. Parsed by hand rather than via `URL` so this works identically
// on browsers, Node and Hermes (whose URL implementation is incomplete).
const API_BASE_URL_PATTERN = /^(https?):\/\/(\[[0-9a-fA-F:.]+\]|[A-Za-z0-9](?:[A-Za-z0-9.-]*[A-Za-z0-9])?)(?::(\d{1,5}))?(\/[^\s?#@]*)?$/;

/**
 * Normalise a user- or config-supplied API base URL.
 * Returns "" when the value is empty or not an absolute http(s) URL, so callers
 * can fall through to the next source without try/catch.
 */
export function normalizeApiBaseUrl(raw: string | null | undefined): string {
  const trimmed = raw?.trim() ?? "";
  if (!trimmed) {
    return "";
  }
  const match = API_BASE_URL_PATTERN.exec(trimmed);
  if (!match) {
    return "";
  }
  const [, scheme, host, port, path = ""] = match;
  if (port !== undefined && (Number(port) < 1 || Number(port) > 65535)) {
    return "";
  }
  const authority = port ? `${host.toLowerCase()}:${port}` : host.toLowerCase();
  return trimTrailingSlashes(`${scheme.toLowerCase()}://${authority}${path}`);
}

export type ResolveApiBaseUrlInput = {
  /** Highest priority: value chosen at runtime (desktop/mobile server picker, tests). */
  runtimeOverride?: string | null;
  /** Build-time configuration (e.g. NEXT_PUBLIC_API_BASE_URL). */
  configured?: string | null;
  /** Current page location for same-origin deployments. Absent outside a browser. */
  location?: { hostname: string; port: string; origin: string } | null;
};

/**
 * Pick the API base URL in priority order: runtime override → build config →
 * same origin (with the local-dev convention that a non-8080 loopback page talks
 * to the Go server on 8080).
 */
export function resolveApiBaseUrl(input: ResolveApiBaseUrlInput): string {
  const override = normalizeApiBaseUrl(input.runtimeOverride);
  if (override) {
    return override;
  }
  const configured = normalizeApiBaseUrl(input.configured);
  if (configured) {
    return configured;
  }
  const location = input.location;
  if (!location) {
    return "";
  }
  if (LOOPBACK_HOSTS.has(location.hostname) && location.port !== DEFAULT_DEV_API_PORT) {
    const host = location.hostname === "::1" ? "[::1]" : location.hostname;
    return `http://${host}:${DEFAULT_DEV_API_PORT}`;
  }
  return trimTrailingSlashes(location.origin);
}
