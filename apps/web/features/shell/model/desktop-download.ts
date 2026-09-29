/**
 * The download page URL when it is safe to open as a link: an absolute
 * http(s) URL with a host and no embedded credentials. The server validates the same
 * rules; this guards against older servers and hand-edited settings. Returns null
 * when the entry must stay hidden.
 */
export function resolveDesktopDownloadURL(enabled: boolean, value: string): string | null {
  if (!enabled) return null;
  const trimmed = value.trim();
  if (!trimmed || trimmed.length > 200) return null;
  let parsed: URL;
  try {
    parsed = new URL(trimmed);
  } catch {
    return null;
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") return null;
  if (!parsed.hostname || parsed.username || parsed.password) return null;
  return trimmed;
}
