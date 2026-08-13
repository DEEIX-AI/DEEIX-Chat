import { resolveModelIconURL } from "@/shared/lib/model-identity";

const LOCAL_ICON_URLS: Record<string, string> = {
  discord: "/identity-providers/discord.svg",
  facebook: "/identity-providers/facebook.svg",
  linuxdo: "/identity-providers/linux-do.svg",
  x: "/identity-providers/x.svg",
};

const PROVIDER_ICON_ALIASES: Array<{ icon: string; patterns: RegExp[] }> = [
  { icon: "apple", patterns: [/\bapple\b/i] },
  { icon: "github", patterns: [/\bgithub\b/i] },
  { icon: "google", patterns: [/\bgoogle\b/i] },
  { icon: "discord", patterns: [/\bdiscord\b/i] },
  { icon: "facebook", patterns: [/\bfacebook\b/i, /\bmeta\b/i] },
  { icon: "huggingface", patterns: [/\bhugging[\s-]?face\b/i, /\bhuggingface\b/i] },
  { icon: "linuxdo", patterns: [/\blinux[\s.-]?do\b/i, /\blinuxdo\b/i] },
  { icon: "microsoft", patterns: [/\bmicrosoft\b/i, /\bazure\b/i, /\bentra\b/i] },
  { icon: "x", patterns: [/\btwitter\b/i, /\bx\b/i] },
  { icon: "vercel", patterns: [/\bvercel\b/i] },
];

export function resolveIdentityProviderIconKey(name: string, slug: string): string {
  const value = `${name} ${slug}`.trim();
  const match = PROVIDER_ICON_ALIASES.find((item) => item.patterns.some((pattern) => pattern.test(value)));
  return match?.icon ?? "";
}

export function resolveIdentityProviderIconURL(name: string, slug: string): string | null {
  const icon = resolveIdentityProviderIconKey(name, slug);
  if (!icon) return null;
  if (LOCAL_ICON_URLS[icon]) return LOCAL_ICON_URLS[icon];
  return resolveModelIconURL(icon);
}

export function resolveSafeIdentityProviderIconURL(value: string | null | undefined): string {
  const normalized = value?.trim() ?? "";
  if (!normalized) {
    return "";
  }
  for (const character of normalized) {
    const codePoint = character.codePointAt(0) ?? 0;
    if (codePoint <= 0x1f || codePoint === 0x7f) {
      return "";
    }
  }

  if (normalized.startsWith("/") && !normalized.startsWith("//") && !normalized.includes("\\")) {
    const parsed = new URL(normalized, "https://identity-provider.local");
    return `${parsed.pathname}${parsed.search}${parsed.hash}`;
  }

  try {
    const parsed = new URL(normalized);
    if ((parsed.protocol === "https:" || parsed.protocol === "http:") && parsed.hostname && !parsed.username && !parsed.password) {
      return parsed.href;
    }
  } catch {
    // Invalid and relative URLs are intentionally rejected.
  }
  return "";
}

export function resolveIdentityProviderIconScale(name: string, slug: string): number {
  const icon = resolveIdentityProviderIconKey(name, slug);
  if (icon === "x") return 0.78;
  if (icon === "linuxdo") return 0.86;
  return 1;
}

export function shouldInvertIdentityProviderIcon(name: string, slug: string): boolean {
  const icon = resolveIdentityProviderIconKey(name, slug);
  return Boolean(icon) && icon !== "linuxdo";
}
