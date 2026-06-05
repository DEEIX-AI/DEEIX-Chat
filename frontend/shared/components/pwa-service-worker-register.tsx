"use client";

import * as React from "react";

export function PWAServiceWorkerRegister() {
  React.useEffect(() => {
    if (process.env.NODE_ENV !== "production") {
      return undefined;
    }
    if (!("serviceWorker" in navigator)) {
      return undefined;
    }

    let cancelled = false;
    const register = () => {
      if (cancelled) {
        return;
      }
      void navigator.serviceWorker.register("/sw.js", { scope: "/" }).catch(() => {
        // PWA enhancement only; the app must keep working without a service worker.
      });
    };

    if (document.readyState === "complete") {
      register();
    } else {
      window.addEventListener("load", register, { once: true });
    }

    return () => {
      cancelled = true;
      window.removeEventListener("load", register);
    };
  }, []);

  return null;
}
