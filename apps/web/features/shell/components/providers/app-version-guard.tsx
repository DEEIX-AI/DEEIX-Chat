"use client";

import { useShellAppVersionCheck } from "@/features/shell/hooks/use-shell-app-version-check";

export function AppVersionGuard(): null {
  useShellAppVersionCheck();
  return null;
}
