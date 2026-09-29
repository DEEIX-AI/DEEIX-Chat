"use client";

import type { CapabilityFlags, Feature } from "@deeix/core";
import { useRouter } from "next/navigation";
import * as React from "react";

import { useCapabilities } from "./capabilities-context";

// A sidebar and its route guard must agree on which sections exist, so both
// consume one table whose entries may name the feature they depend on.
export type SectionEntry = { readonly href: string; readonly feature?: Feature };

export function isSectionAvailable(entry: SectionEntry, flags: CapabilityFlags): boolean {
  return entry.feature === undefined || flags[entry.feature];
}

/**
 * Redirect away from a section the server does not offer. Returns true while
 * the redirect is pending so the caller renders nothing instead of the page.
 */
export function useSectionGuard(entry: SectionEntry | null, fallbackHref: string): boolean {
  const router = useRouter();
  const { flags, loaded } = useCapabilities();
  const blocked = loaded && entry !== null && !isSectionAvailable(entry, flags);

  React.useEffect(() => {
    if (blocked) {
      router.replace(fallbackHref);
    }
  }, [blocked, fallbackHref, router]);

  return blocked;
}
