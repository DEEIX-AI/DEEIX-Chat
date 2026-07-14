"use client";

import * as React from "react";
import dynamic from "next/dynamic";

import { AuthGuard } from "@/shared/auth/auth-guard";
import { AuthSessionProvider } from "@/shared/auth/auth-session-context";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { readAccessToken, SESSION_SNAPSHOT_CHANGED_EVENT } from "@/shared/auth/session";

const ProjectLayout = dynamic(
  () => import("@/features/layouts/components/sections/project-layout").then((mod) => mod.ProjectLayout),
  { ssr: false },
);

export function ProjectWorkspace({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <AuthGuard>
      <ProjectLayout>{children}</ProjectLayout>
    </AuthGuard>
  );
}

export function ShareWorkspace({
  children,
}: {
  children: React.ReactNode;
}) {
  const [accessToken, setAccessToken] = React.useState<string | null>(() => readAccessToken() || null);

  React.useEffect(() => {
    let cancelled = false;

    async function checkSession() {
      try {
        const token = await resolveAccessToken();
        if (!cancelled) {
          setAccessToken(token || null);
        }
      } catch {
        if (!cancelled) {
          setAccessToken(null);
        }
      }
    }

    void checkSession();
    return () => {
      cancelled = true;
    };
  }, []);

  React.useEffect(() => {
    function handleSessionChanged() {
      setAccessToken(readAccessToken() || null);
    }

    window.addEventListener(SESSION_SNAPSHOT_CHANGED_EVENT, handleSessionChanged);
    return () => {
      window.removeEventListener(SESSION_SNAPSHOT_CHANGED_EVENT, handleSessionChanged);
    };
  }, []);

  if (!accessToken) {
    return <>{children}</>;
  }

  return (
    <AuthSessionProvider accessToken={accessToken}>
      <ProjectLayout defaultSidebarOpen={false}>{children}</ProjectLayout>
    </AuthSessionProvider>
  );
}
