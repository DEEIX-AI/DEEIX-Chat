"use client";

import * as React from "react";

import { getPublicBranding, type BrandingDTO } from "@/shared/api/branding";
import { DEFAULT_BRANDING, setBrandingSnapshot } from "@/shared/config/branding";

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
 * Loads the public branding once per page. `ready` flips when branding arrived, the
 * request failed (one silent retry follows) or the fallback delay elapsed, so a slow
 * server never keeps the page hidden.
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
    const applyBranding = (nextBranding: BrandingDTO) => {
      setBrandingSnapshot(nextBranding);
      if (!active) {
        return;
      }
      window.clearTimeout(fallbackTimer);
      setBranding(nextBranding);
      setReady(true);
    };
    const handleLoadFailure = () => {
      if (!active) {
        return;
      }
      window.clearTimeout(fallbackTimer);
      setReady(true);
      retryTimer = window.setTimeout(() => {
        void requestBranding().then(applyBranding).catch((): undefined => undefined);
      }, BRANDING_RETRY_DELAY_MS);
    };

    void requestBranding().then(applyBranding).catch(handleLoadFailure);

    return () => {
      active = false;
      window.clearTimeout(fallbackTimer);
      if (retryTimer !== undefined) {
        window.clearTimeout(retryTimer);
      }
    };
  }, []);

  return { branding, ready };
}
