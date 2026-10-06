import {
  LOBEHUB_COLOR_ICON_SLUGS,
  LOBEHUB_ICON_BASE_PATH,
  LOBEHUB_ICON_SLUGS,
  LOBEHUB_MIXED_ICON_SLUGS,
} from "@/shared/generated/lobehub-icons";

/**
 * How a bundled icon is painted, decided from its SVG source at build time:
 * mono icons use only currentColor, color icons only fixed colors, mixed icons both.
 */
export type LobehubIconTone = "mono" | "color" | "mixed";

let iconTones: Map<string, LobehubIconTone> | null = null;

function bundledIconTones(): Map<string, LobehubIconTone> {
  if (!iconTones) {
    iconTones = new Map(LOBEHUB_ICON_SLUGS.split(",").map((slug) => [slug, "mono"]));
    for (const slug of LOBEHUB_COLOR_ICON_SLUGS.split(",")) iconTones.set(slug, "color");
    for (const slug of LOBEHUB_MIXED_ICON_SLUGS.split(",")) iconTones.set(slug, "mixed");
  }
  return iconTones;
}

// Unversioned path used before icons moved into a per-version directory; still accepted so stored
// or hand-entered URLs keep resolving.
const LEGACY_ICON_PREFIX = "/vendor/lobehub-icons/";
const ICON_SLUG = /^[a-z0-9][a-z0-9.-]*$/u;
const VARIANT_SUFFIX = /-(?:brand|brand-color|color|text|text-cn)$/u;

/** URL of a bundled LobeHub icon. */
export function lobehubIconURL(slug: string): string {
  return `${LOBEHUB_ICON_BASE_PATH}/${slug}.svg`;
}

/** Slug of a bundled LobeHub icon URL (current or legacy path); null for any other URL. */
export function parseLobehubIconSlug(url: string): string | null {
  const prefix = url.startsWith(`${LOBEHUB_ICON_BASE_PATH}/`) ? `${LOBEHUB_ICON_BASE_PATH}/` : LEGACY_ICON_PREFIX;
  if (!url.startsWith(prefix) || !url.endsWith(".svg")) {
    return null;
  }
  const slug = url.slice(prefix.length, -4);
  return ICON_SLUG.test(slug) ? slug : null;
}

/** How a bundled icon is painted; null when the slug is not bundled. */
export function lobehubIconTone(slug: string): LobehubIconTone | null {
  return bundledIconTones().get(slug) ?? null;
}

/** Display name derived from the slug, e.g. "claude-color" → "Claude". */
export function lobehubIconLabel(slug: string): string {
  return slug
    .replace(VARIANT_SUFFIX, "")
    .split("-")
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

export type LobehubIconOption = { id: string; name: string };

let iconOptions: LobehubIconOption[] | null = null;

/** Base icons offered in the icon picker; brand, color and text variants are left out. */
export function listLobehubIconOptions(): readonly LobehubIconOption[] {
  iconOptions ??= [...bundledIconTones().keys()]
    .filter((slug) => !VARIANT_SUFFIX.test(slug))
    .map((slug) => ({ id: slug, name: lobehubIconLabel(slug) }));
  return iconOptions;
}
