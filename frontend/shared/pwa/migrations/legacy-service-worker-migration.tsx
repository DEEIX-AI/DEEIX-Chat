"use client";

import * as React from "react";

const LEGACY_CACHE_PREFIX = "deeix-chat-";

export function LegacyPWAServiceWorkerMigration() {
  React.useEffect(() => {
    if (process.env.NODE_ENV !== "production" || !("serviceWorker" in navigator)) {
      return;
    }

    const migrate = async () => {
      const cacheNames = "caches" in window ? await window.caches.keys() : [];
      const legacyCacheNames = cacheNames.filter((name) =>
        name.startsWith(LEGACY_CACHE_PREFIX),
      );
      const registration = await navigator.serviceWorker.getRegistration("/");
      const worker = registration?.active ?? registration?.waiting ?? registration?.installing;
      const isLegacyRegistration = worker
        ? new URL(worker.scriptURL).pathname === "/sw.js"
        : false;

      await Promise.all(legacyCacheNames.map((name) => window.caches.delete(name)));

      if (!isLegacyRegistration) {
        return;
      }

      await navigator.serviceWorker.register("/sw.js", {
        scope: "/",
        updateViaCache: "none",
      });
    };

    void migrate().catch(() => {
      // Migration is best-effort and must not block the application.
    });
  }, []);

  return null;
}
