import type { ProfileDraft } from "@/features/settings/types/settings";
import type { UserDTO } from "@/shared/api/auth.types";

function normalizeString(value: unknown, fallback = ""): string {
  if (typeof value !== "string") {
    return fallback;
  }

  const normalizedValue = value.trim();
  return normalizedValue || fallback;
}

export function createDraftFromUser(user?: UserDTO | null): ProfileDraft {
  return {
    avatarUrl: normalizeString(user?.avatarURL),
    displayName: normalizeString(user?.displayName),
    timezone: normalizeString(user?.timezone, "Etc/UTC"),
    locale: normalizeString(user?.locale, "en-US"),
    profilePreferences: normalizeString(user?.profilePreferences),
  };
}

export function isProfileDraftEqual(left: ProfileDraft, right: ProfileDraft): boolean {
  return (
    left.avatarUrl === right.avatarUrl &&
    left.displayName === right.displayName &&
    left.timezone === right.timezone &&
    left.locale === right.locale &&
    left.profilePreferences === right.profilePreferences
  );
}
