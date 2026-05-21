export const USERNAME_MIN_LENGTH = 3;
export const USERNAME_MAX_LENGTH = 16;
export const DISPLAY_NAME_MIN_LENGTH = 3;
export const DISPLAY_NAME_MAX_LENGTH = 16;
export const PASSWORD_MIN_LENGTH = 8;
export const PASSWORD_MAX_LENGTH = 128;

export function normalizeUsernameInput(value: string) {
  return value.trim().toLowerCase();
}

export function isUsernameLengthValid(value: string) {
  const normalized = normalizeUsernameInput(value);
  return normalized.length >= USERNAME_MIN_LENGTH && normalized.length <= USERNAME_MAX_LENGTH;
}

const RESERVED_USERNAMES = new Set([
  "admin",
  "api",
  "auth",
  "billing",
  "chat",
  "deeix_chat",
  "deeix-chat",
  "files",
  "help",
  "me",
  "root",
  "settings",
  "support",
  "system",
  "user",
  "users",
]);

export function isUsernamePolicyValid(value: string) {
  const normalized = normalizeUsernameInput(value);
  return (
    isUsernameLengthValid(normalized) &&
    !normalized.includes("@") &&
    /^[a-z0-9][a-z0-9_-]*[a-z0-9]$/.test(normalized) &&
    !RESERVED_USERNAMES.has(normalized)
  );
}

export function isDisplayNameLengthValid(value: string) {
  const count = [...value.trim()].length;
  return count >= DISPLAY_NAME_MIN_LENGTH && count <= DISPLAY_NAME_MAX_LENGTH;
}

export function isPasswordPolicyValid(value: string) {
  const normalized = value.trim();
  const count = [...normalized].length;
  return count >= PASSWORD_MIN_LENGTH && count <= PASSWORD_MAX_LENGTH && !/^\d+$/.test(normalized);
}
