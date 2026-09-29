"use client";

import type { Feature } from "@deeix/core";
import * as React from "react";

import { type CapabilitiesState, INITIAL_CAPABILITIES_STATE, useCapabilitiesLoader } from "./use-capabilities-loader";

// Server capabilities decide which features the UI offers. They are loaded once
// per server; nothing here looks at the platform (docs/ARCHITECTURE.md §4).

const CapabilitiesContext = React.createContext<CapabilitiesState>(INITIAL_CAPABILITIES_STATE);

export function CapabilitiesProvider({ children }: { children: React.ReactNode }) {
  const state = useCapabilitiesLoader();
  return <CapabilitiesContext.Provider value={state}>{children}</CapabilitiesContext.Provider>;
}

export function useCapabilities(): CapabilitiesState {
  return React.useContext(CapabilitiesContext);
}

export function useFeature(feature: Feature): boolean {
  return React.useContext(CapabilitiesContext).flags[feature];
}

/**
 * Render children only when the server offers `feature`. Until capabilities are
 * known the gate is open, so a slow server never hides what it does provide.
 */
export function FeatureGate({
  feature,
  fallback = null,
  children,
}: {
  feature: Feature;
  fallback?: React.ReactNode;
  children: React.ReactNode;
}) {
  const enabled = useFeature(feature);
  return <>{enabled ? children : fallback}</>;
}
