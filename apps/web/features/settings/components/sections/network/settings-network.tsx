"use client";

import { useRouter } from "next/navigation";
import * as React from "react";

import { DesktopNetworkSettings, useDesktopNetworkAvailable } from "@/features/desktop";

// The tab exists only in the desktop app. A browser that reaches the URL
// directly goes back to General, as for any section this client does not offer.
export function SettingsNetwork() {
  const router = useRouter();
  const available = useDesktopNetworkAvailable();
  const [hydrated, setHydrated] = React.useState(false);

  React.useEffect(() => setHydrated(true), []);
  React.useEffect(() => {
    if (hydrated && !available) router.replace("/settings/general");
  }, [available, hydrated, router]);

  return available ? <DesktopNetworkSettings /> : null;
}
