"use client";

import * as React from "react";

import { getPublicBranding, type BrandingDTO } from "@/shared/api/branding";
import { DEFAULT_BRANDING, setBrandingSnapshot } from "@/shared/config/branding";
import { waitForApiServer } from "@/shared/platform/desktop-session";
import { onServerOriginChanged } from "@/shared/platform/server-address";

const BRANDING_FALLBACK_DELAY_MS = 3_000;
const BRANDING_RETRY_DELAY_MS = 1_000;

let brandingRequest: Promise<BrandingDTO> | null = null;

function requestBranding(): Promise<BrandingDTO> {
  brandingRequest ??= getPublicBranding().catch((error) => {
    brandingRequest = null;
    throw error;
  });
  return brandingRequest;
}

/**
 * Loads the public branding of the page's API server. `ready` flips when branding arrived,
 * the request failed (one silent retry follows), the fallback delay elapsed, or there is no
 * server yet (an unbound desktop tab keeps the built-in branding), so a slow server never
 * keeps the page hidden. Desktop tabs reload it when their server origin changes.
 */
export function useBrandingLoader(): { branding: BrandingDTO; ready: boolean } {
  const [branding, setBranding] = React.useState(DEFAULT_BRANDING);
  const [ready, setReady] = React.useState(false);

  React.useLayoutEffect(() => {
    let active = true;
    const fallbackTimer = window.setTimeout(() => {
      if (active) {
        setReady(true);
      }
    }, BRANDING_FALLBACK_DELAY_MS);
    let retryTimer: number | undefined;
    // Bumped whenever the server changes, so a response from the previous server that
    // settles late cannot overwrite the current one.
    let generation = 0;
    const applyBranding = (nextBranding: BrandingDTO) => {
      setBrandingSnapshot(nextBranding);
      if (!active) {
        return;
      }
      window.clearTimeout(fallbackTimer);
      setBranding(nextBranding);
      setReady(true);
    };
    const applyFrom = (requestGeneration: number) => (nextBranding: BrandingDTO) => {
      if (requestGeneration === generation) {
        applyBranding(nextBranding);
      }
    };
    const handleLoadFailure = (requestGeneration: number) => {
      if (!active || requestGeneration !== generation) {
        return;
      }
      window.clearTimeout(fallbackTimer);
      setReady(true);
      window.clearTimeout(retryTimer);
      retryTimer = window.setTimeout(() => {
        void requestBranding().then(applyFrom(requestGeneration)).catch((): undefined => undefined);
      }, BRANDING_RETRY_DELAY_MS);
    };

    const load = () => {
      const requestGeneration = generation;
      void requestBranding()
        .then(applyFrom(requestGeneration))
        .catch(() => handleLoadFailure(requestGeneration));
    };

    // A desktop tab learns its server asynchronously, and it changes when the tab is bound
    // from the setup screen or the local sidecar restarts on another port. A server that
    // cannot be resolved is dropped by DesktopBootstrap, which then shows the setup screen.
    void waitForApiServer()
      .catch(() => false)
      .then((hasServer) => {
        if (hasServer) {
          load();
        } else {
          applyBranding(DEFAULT_BRANDING);
        }
      });
    const unsubscribe = onServerOriginChanged((origin) => {
      generation += 1;
      brandingRequest = null;
      window.clearTimeout(retryTimer);
      if (origin) {
        load();
      } else {
        applyBranding(DEFAULT_BRANDING);
      }
    });

    return () => {
      active = false;
      unsubscribe();
      window.clearTimeout(fallbackTimer);
      window.clearTimeout(retryTimer);
    };
  }, []);

  return { branding, ready };
}
