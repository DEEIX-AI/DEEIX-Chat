// URL helpers shared by the block renderers that take model-supplied addresses.
// They live next to block-text.ts rather than inside a component file so the
// blocks that need them do not have to import from another block's renderer.

/**
 * Validates an image address before it reaches the DOM. Only absolute http(s)
 * URLs and this server's own `/api/v1/...` files (uploads, generated images)
 * are accepted; `data:` URLs, other schemes and relative paths are dropped, as
 * the latter would resolve against the page instead of the file the model meant.
 */
export function safeImageSource(value: string | undefined): string | undefined {
  const trimmed = value?.trim();
  if (!trimmed) {
    return undefined;
  }
  if (trimmed.startsWith("/api/v1/")) {
    return trimmed;
  }
  try {
    const url = new URL(trimmed.startsWith("//") ? `https:${trimmed}` : trimmed);
    return url.protocol === "https:" || url.protocol === "http:" ? url.toString() : undefined;
  } catch {
    return undefined;
  }
}

/** Validates a link shown next to an image or on a card; returns the href and a display host name. */
export function safeLinkURL(value: string | undefined): { href: string; host: string } | undefined {
  if (!value) {
    return undefined;
  }
  try {
    const url = new URL(value);
    if (url.protocol !== "http:" && url.protocol !== "https:") {
      return undefined;
    }
    return { href: url.toString(), host: url.hostname.replace(/^www\./, "") };
  } catch {
    return undefined;
  }
}
