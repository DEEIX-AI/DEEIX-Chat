"use client";

import * as React from "react";

import { getMe } from "@/shared/api/auth";
import type { UserDTO } from "@/shared/api/auth-types";
import { USER_PROFILE_UPDATED_EVENT } from "@/shared/auth/user-profile-events";

export type AuthSessionUserStatus = "loading" | "ready" | "failed";

/** Loads the signed-in user for `accessToken` and follows profile updates dispatched elsewhere in the app. */
export function useAuthUserLoader(accessToken: string) {
  const [user, setUser] = React.useState<UserDTO | null>(null);
  const [userStatus, setUserStatus] = React.useState<AuthSessionUserStatus>("loading");

  const refreshUser = React.useCallback(async () => {
    setUserStatus("loading");
    try {
      const nextUser = await getMe(accessToken);
      setUser(nextUser);
      setUserStatus("ready");
      return nextUser;
    } catch {
      setUser(null);
      setUserStatus("failed");
      return null;
    }
  }, [accessToken]);

  React.useEffect(() => {
    let cancelled = false;

    async function loadUser() {
      setUserStatus("loading");
      try {
        const nextUser = await getMe(accessToken);
        if (!cancelled) {
          setUser(nextUser);
          setUserStatus("ready");
        }
      } catch {
        if (!cancelled) {
          setUser(null);
          setUserStatus("failed");
        }
      }
    }

    void loadUser();
    return () => {
      cancelled = true;
    };
  }, [accessToken]);

  React.useEffect(() => {
    function handleProfileUpdated(event: CustomEvent<UserDTO>) {
      const nextUser = event.detail;
      if (!nextUser) {
        return;
      }

      setUser(nextUser);
      setUserStatus("ready");
    }

    window.addEventListener(USER_PROFILE_UPDATED_EVENT, handleProfileUpdated);
    return () => {
      window.removeEventListener(USER_PROFILE_UPDATED_EVENT, handleProfileUpdated);
    };
  }, []);

  return { user, userStatus, refreshUser };
}
