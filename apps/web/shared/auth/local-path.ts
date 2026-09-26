export const DEFAULT_AUTH_NEXT_PATH = "/chat";

export function normalizeAuthNextPath(value: string | null | undefined, fallback = DEFAULT_AUTH_NEXT_PATH): string {
  if (!value || !value.startsWith("/") || value.startsWith("//")) {
    return fallback;
  }

  return value;
}
