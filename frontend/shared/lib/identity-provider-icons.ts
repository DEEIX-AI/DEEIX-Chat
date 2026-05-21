import { resolveLobeHubIconURL } from "@/shared/lib/model-identity";

const SIMPLE_ICON_SLUGS = new Set(["discord", "facebook", "x"]);
const LOCAL_ICON_URLS: Record<string, string> = {
  linuxdo: "/linux-do.svg",
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
  if (SIMPLE_ICON_SLUGS.has(icon)) {
    return `https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/${icon}.svg`;
  }
  return resolveLobeHubIconURL(icon);
}

export function resolveIdentityProviderIconScale(name: string, slug: string): number {
  const icon = resolveIdentityProviderIconKey(name, slug);
  if (icon === "x") return 0.78;
  if (icon === "linuxdo") return 0.86;
  return 1;
}
