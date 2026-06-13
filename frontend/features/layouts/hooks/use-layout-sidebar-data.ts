"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { resolveAvatarImageSrc } from "@/shared/lib/avatar";
import { useOptionalAuthSession } from "@/shared/auth/auth-session-context";
import type { UserDTO } from "@/shared/api/auth.types";
import type { SidebarData, SidebarUser } from "@/features/layouts/types/sidebar";

const defaultData: SidebarData = {
  user: undefined,
  recents: [],
};

function toSidebarUser(item: UserDTO, fallbackName: string): SidebarUser {
  const username = item.username.trim();
  return {
    name: item.displayName || username || fallbackName,
    email: item.email || username || fallbackName,
    avatar: resolveAvatarImageSrc(item.avatarURL, item),
    role: item.role,
  };
}

export function useLayoutSidebarData() {
  const t = useTranslations("common.navigation");
  const session = useOptionalAuthSession();
  const user = session?.user ?? null;

  return React.useMemo<SidebarData>(
    () => ({
      ...defaultData,
      user: user ? toSidebarUser(user, t("fallbackUser")) : undefined,
    }),
    [t, user],
  );
}
