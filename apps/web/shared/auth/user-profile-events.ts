"use client";

import type { UserDTO } from "@/shared/api/auth-types";

export const USER_PROFILE_UPDATED_EVENT = "deeix-chat:user-profile-updated";

// Typing the event on the window lets listeners read `detail` without casting.
declare global {
  // biome-ignore lint/style/useConsistentTypeDefinitions: augmenting the global WindowEventMap requires interface declaration merging.
  interface WindowEventMap {
    [USER_PROFILE_UPDATED_EVENT]: CustomEvent<UserDTO>;
  }
}

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
