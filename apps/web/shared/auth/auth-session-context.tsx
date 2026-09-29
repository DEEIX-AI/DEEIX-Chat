"use client";

import * as React from "react";

import type { UserDTO } from "@/shared/api/auth-types";
import { type AuthSessionUserStatus, useAuthUserLoader } from "@/shared/auth/use-auth-user-loader";

type AuthSessionContextValue = {
  accessToken: string;
  user: UserDTO | null;
  userStatus: AuthSessionUserStatus;
  refreshUser: () => Promise<UserDTO | null>;
};

const AuthSessionContext = React.createContext<AuthSessionContextValue | null>(null);

export function AuthSessionProvider({
  accessToken,
  children,
}: {
  accessToken: string;
  children: React.ReactNode;
}) {
  const { user, userStatus, refreshUser } = useAuthUserLoader(accessToken);

  const value = React.useMemo<AuthSessionContextValue>(
    () => ({
      accessToken,
      user,
      userStatus,
      refreshUser,
    }),
    [accessToken, refreshUser, user, userStatus],
  );

  return <AuthSessionContext.Provider value={value}>{children}</AuthSessionContext.Provider>;
}

export function useAuthSession() {
  const context = React.useContext(AuthSessionContext);
  if (!context) {
    throw new Error("useAuthSession must be used within AuthSessionProvider");
  }
  return context;
}

export function useOptionalAuthSession() {
  return React.useContext(AuthSessionContext);
}
