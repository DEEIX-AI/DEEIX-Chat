"use client";

import * as React from "react";

import { isDesktopApp } from "@/shared/platform";

function subscribeNothing(): () => void {
  return () => {};
}

// Server and first client render agree on "not available"; the real value is
// read after hydration, so the static export never mismatches.
function getServerAvailable(): boolean {
  return false;
}

/**
 * Whether this build can configure a network proxy at all. It is a property of
 * the desktop shell, not a server feature: in a browser the page's proxy is the
 * browser's business, so the Network settings tab does not exist there. The
 * platform check lives here, in the desktop feature, like the download entry.
 */
export function useDesktopNetworkAvailable(): boolean {
  return React.useSyncExternalStore(subscribeNothing, isDesktopApp, getServerAvailable);
}
