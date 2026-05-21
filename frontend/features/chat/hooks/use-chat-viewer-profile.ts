"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import {
  resolveGreetingPeriodByHour,
  resolveHourByTimeZone,
} from "@/features/chat/utils/chat-runtime";
import type { ViewerProfile } from "@/features/chat/types/chat-runtime";
import { useOptionalAuthSession } from "@/shared/auth/auth-session-context";
import type { UserDTO } from "@/shared/api/auth.types";

const DEFAULT_VIEWER_PROFILE: ViewerProfile = {
  name: "",
  timeZone: "Etc/UTC",
};
const GREETING_VARIANT_KEYS = [
  "default",
  "start",
  "today",
  "help",
  "continue",
  "ready",
  "focus",
  "next",
  "draft",
  "quiet",
] as const;

function toViewerProfile(user: UserDTO): ViewerProfile {
  return {
    name: (user.displayName || user.username || "").trim(),
    timeZone: (user.timezone || "").trim() || "Etc/UTC",
  };
}

export function useChatViewerProfile() {
  const t = useTranslations("chat.greeting");
  const session = useOptionalAuthSession();
  const user = session?.user ?? null;
  const [greetingVariantKey] = React.useState<(typeof GREETING_VARIANT_KEYS)[number]>(
    () => GREETING_VARIANT_KEYS[Math.floor(Math.random() * GREETING_VARIANT_KEYS.length)] ?? "default",
  );
  const viewer = React.useMemo(
    () => (user ? toViewerProfile(user) : DEFAULT_VIEWER_PROFILE),
    [user],
  );

  const greetingTitle = React.useMemo(() => {
    const hour = resolveHourByTimeZone(viewer.timeZone);
    const period = resolveGreetingPeriodByHour(hour);
    const greeting = t(period);
    return t(`titles.${period}.${greetingVariantKey}`, { greeting, name: viewer.name || t("fallbackName") });
  }, [greetingVariantKey, t, viewer.name, viewer.timeZone]);

  return {
    viewer,
    greetingTitle,
  };
}
