"use client";

import type { UserDTO } from "@/shared/api/auth.types";

export const USER_PROFILE_UPDATED_EVENT = "deeix-chat:user-profile-updated";

export function dispatchUserProfileUpdated(user: UserDTO) {
  if (typeof window === "undefined") {
    return;
  }

  window.dispatchEvent(
    new CustomEvent<UserDTO>(USER_PROFILE_UPDATED_EVENT, {
      detail: user,
    }),
  );
}
