import type { ProfileDraft } from "@/features/settings/types/settings";
import type { UserDTO } from "@/shared/api/auth.types";
import { normalizeTrimmedString } from "@/shared/lib/string";

export function createDraftFromUser(user?: UserDTO | null): ProfileDraft {
  return {
    avatarUrl: normalizeTrimmedString(user?.avatarURL),
    displayName: normalizeTrimmedString(user?.displayName),
    timezone: normalizeTrimmedString(user?.timezone, "Etc/UTC"),
    locale: normalizeTrimmedString(user?.locale, "en-US"),
    profilePreferences: normalizeTrimmedString(user?.profilePreferences),
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
