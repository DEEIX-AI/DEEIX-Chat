"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { ServerSetup } from "@/features/platform/components/server-setup";
import { isDesktopApp } from "@/shared/platform";
import { leaveServer } from "@/shared/platform/desktop-shell";
import { ensureLocalSession, initializeDesktopSession } from "@/shared/platform/desktop-session";

// Desktop bootstrap: learn which server this install uses before any request is
// made, and gate the app behind the setup screen until one is chosen. Local
// mode also establishes the session here, so the app never shows a login form
// for the bundled server. Browsers render children immediately.

type State = "pending" | "setup" | "ready";

export function DesktopBootstrap({ children }: { children: React.ReactNode }) {
  // Start identical on server and client ("pending" → renders nothing) and decide
  // after mount. Reading `window.isTauri` in the state initialiser would make the
  // server-rendered HTML and the first client render disagree (hydration error).
  const [state, setState] = React.useState<State>("pending");
  const t = useTranslations("desktopSetup");
  const tRef = React.useRef(t);
  tRef.current = t;

  React.useEffect(() => {
    if (!isDesktopApp()) {
      setState("ready");
      return;
    }
    let cancelled = false;
    void (async () => {
      try {
        const server = await initializeDesktopSession();
        if (server?.mode === "local") {
          await ensureLocalSession().catch(() => false);
        }
        if (!cancelled) {
          setState(server ? "ready" : "setup");
        }
      } catch (error) {
        // A configured server that cannot be started (the local sidecar failed)
        // is dropped so the setup screen can bind this tab afresh.
        toast.error(tRef.current("toasts.bootstrapFailed"), { description: describe(error) });
        await leaveServer().catch(() => undefined);
        if (!cancelled) {
          setState("setup");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  if (state === "pending") {
    return null;
  }
  if (state === "setup") {
    return <ServerSetup onConfigured={() => setState("ready")} />;
  }
  return <>{children}</>;
}

function describe(error: unknown): string {
  if (typeof error === "object" && error !== null && "message" in error) {
    return String((error as { message: unknown }).message);
  }
  return String(error);
}
