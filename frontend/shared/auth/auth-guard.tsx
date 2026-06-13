"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";

import { SpinnerLabel } from "@/components/ui/spinner";
import { AuthSessionProvider } from "@/shared/auth/auth-session-context";
import { normalizeAuthNextPath } from "@/shared/auth/local-path";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { readAccessToken, SESSION_SNAPSHOT_CHANGED_EVENT, type SessionSnapshot } from "@/shared/auth/session";

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const common = useTranslations("common");
  const router = useRouter();
  const [accessToken, setAccessToken] = React.useState<string | null>(() => readAccessToken() || null);

  React.useEffect(() => {
    let cancelled = false;

    async function checkSession() {
      try {
        const token = await resolveAccessToken();
        if (cancelled) {
          return;
        }
        if (token) {
          setAccessToken(token);
          return;
        }
      } catch {
        if (cancelled) {
          return;
        }
      }

      if (!cancelled) {
        const nextPath = normalizeAuthNextPath(`${window.location.pathname}${window.location.search}`);
        router.replace(`/login?next=${encodeURIComponent(nextPath)}`);
      }
    }

    void checkSession();
    return () => {
      cancelled = true;
    };
  }, [router]);

  React.useEffect(() => {
    function handleSessionChanged(event: Event) {
      const snapshot = (event as CustomEvent<SessionSnapshot>).detail;
      const nextToken = snapshot?.accessToken ?? "";
      setAccessToken(nextToken || null);
      if (!nextToken) {
        const nextPath = normalizeAuthNextPath(`${window.location.pathname}${window.location.search}`);
        router.replace(`/login?next=${encodeURIComponent(nextPath)}`);
      }
    }

    window.addEventListener(SESSION_SNAPSHOT_CHANGED_EVENT, handleSessionChanged as EventListener);
    return () => {
      window.removeEventListener(SESSION_SNAPSHOT_CHANGED_EVENT, handleSessionChanged as EventListener);
    };
  }, [router]);

  if (!accessToken) {
    return (
      <main className="flex h-svh w-full items-center justify-center px-4 text-sm text-muted-foreground">
        <SpinnerLabel>{common("states.loading")}</SpinnerLabel>
      </main>
    );
  }

  return <AuthSessionProvider accessToken={accessToken}>{children}</AuthSessionProvider>;
}
