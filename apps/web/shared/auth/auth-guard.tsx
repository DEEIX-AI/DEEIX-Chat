"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";

import { SpinnerLabel } from "@/components/ui/spinner";
import { AuthSessionProvider } from "@/shared/auth/auth-session-context";
import { normalizeAuthNextPath } from "@/shared/auth/local-path";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { readAccessToken, SESSION_SNAPSHOT_CHANGED_EVENT, type SessionSnapshot } from "@/shared/auth/session";

type AuthGuardStatus = "checking" | "ready";

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const common = useTranslations("common");
  const router = useRouter();
  const [accessToken, setAccessToken] = React.useState<string | null>(() => readAccessToken() || null);
  const [status, setStatus] = React.useState<AuthGuardStatus>(() => readAccessToken() ? "ready" : "checking");
  const mountedRef = React.useRef(false);

  const redirectToLogin = React.useCallback(() => {
    // The fragment is never carried into `next`: it may hold an import key, which belongs to the
    // page that stashed it (see features/settings/utils/provider-import.ts), not to a URL.
    const nextPath = normalizeAuthNextPath(`${window.location.pathname}${window.location.search}`);
    router.replace(`/login?next=${encodeURIComponent(nextPath)}`);
  }, [router]);

  const checkSession = React.useCallback(async () => {
    const cachedToken = readAccessToken();
    if (cachedToken) {
      setAccessToken(cachedToken);
      setStatus("ready");
      return;
    }

    setStatus("checking");
    let token = "";
    try {
      token = await resolveAccessToken();
    } catch {
      token = "";
    }

    if (!mountedRef.current) {
      return;
    }

    if (token) {
      setAccessToken(token);
      setStatus("ready");
      return;
    }

    redirectToLogin();
  }, [redirectToLogin]);

  React.useEffect(() => {
    mountedRef.current = true;
    void checkSession();
    return () => {
      mountedRef.current = false;
    };
  }, [checkSession]);

  React.useEffect(() => {
    function handleSessionChanged(event: CustomEvent<SessionSnapshot>) {
      const snapshot: SessionSnapshot | undefined = event.detail;
      const nextToken = snapshot?.accessToken ?? "";
      setAccessToken(nextToken || null);
      setStatus(nextToken ? "ready" : "checking");
      if (!nextToken) {
        redirectToLogin();
      }
    }

    window.addEventListener(SESSION_SNAPSHOT_CHANGED_EVENT, handleSessionChanged);
    return () => {
      window.removeEventListener(SESSION_SNAPSHOT_CHANGED_EVENT, handleSessionChanged);
    };
  }, [redirectToLogin]);

  if (!accessToken || status === "checking") {
    return (
      <main className="flex h-svh w-full items-center justify-center px-4 text-sm text-muted-foreground">
        <SpinnerLabel>{common("states.loading")}</SpinnerLabel>
      </main>
    );
  }

  return <AuthSessionProvider accessToken={accessToken}>{children}</AuthSessionProvider>;
}
