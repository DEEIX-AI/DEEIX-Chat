"use client";

import * as React from "react";

import { isDesktopApp } from "@/shared/platform";
import {
  type DistributionInfo,
  FALLBACK_DISTRIBUTION,
  loadDistribution,
} from "@/shared/platform/desktop-distribution";

export type DesktopDistributionStatus =
  /** Not known yet (always the first render, so server and client HTML agree). */
  | "loading"
  | "ready"
  /** The shell could not report it; `distribution` is the no-policy, no-update fallback. */
  | "failed"
  /** Browser build: there is no distribution. */
  | "unsupported";

export type DesktopDistributionState = {
  status: DesktopDistributionStatus;
  /** The shell's answer once `ready`, otherwise the fallback. */
  distribution: DistributionInfo;
};

const INITIAL_STATE: DesktopDistributionState = { status: "loading", distribution: FALLBACK_DISTRIBUTION };

/**
 * The desktop build's distribution kind, update mode and enterprise policy.
 * Fetched once per page (the shell's answer is memoised in shared/platform).
 */
export function useDesktopDistribution(): DesktopDistributionState {
  const [state, setState] = React.useState<DesktopDistributionState>(INITIAL_STATE);

  React.useEffect(() => {
    if (!isDesktopApp()) {
      setState({ status: "unsupported", distribution: FALLBACK_DISTRIBUTION });
      return;
    }
    let cancelled = false;
    loadDistribution().then(
      (distribution) => {
        if (!cancelled) setState({ status: "ready", distribution });
      },
      () => {
        if (!cancelled) setState({ status: "failed", distribution: FALLBACK_DISTRIBUTION });
      },
    );
    return () => {
      cancelled = true;
    };
  }, []);

  return state;
}
