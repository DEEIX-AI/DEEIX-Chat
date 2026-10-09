"use client";

import * as React from "react";

import { resolveApiBaseURL } from "@/shared/api/http-client";
import { resolvePublicWebOrigin } from "@/shared/platform";

const noopSubscribe = () => () => {};
const readOrigin = () => resolvePublicWebOrigin(resolveApiBaseURL());

/** Origin that import links point at; empty while prerendering. */
export function useSettingsProviderImportOrigin(): string {
  return React.useSyncExternalStore(noopSubscribe, readOrigin, () => "");
}
