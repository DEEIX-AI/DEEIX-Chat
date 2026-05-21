"use client";

import * as React from "react";
import dynamic from "next/dynamic";
import { usePathname } from "next/navigation";

import { AuthGuard } from "@/shared/auth/auth-guard";
import { AuthSessionProvider } from "@/shared/auth/auth-session-context";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { readAccessToken, SESSION_SNAPSHOT_CHANGED_EVENT, type SessionSnapshot } from "@/shared/auth/session";

const AdminAccessGate = dynamic(
  () => import("@/features/admin/components/admin-access-gate").then((mod) => mod.AdminAccessGate),
  { ssr: false },
);

const ProjectLayout = dynamic(
  () => import("@/features/layouts/components/sections/project-layout").then((mod) => mod.ProjectLayout),
  { ssr: false },
);

function isPathWithin(pathname: string, basePath: string): boolean {
  return pathname === basePath || pathname.startsWith(`${basePath}/`);
}

function isPublicPath(pathname: string): boolean {
  return pathname === "/login" || isPathWithin(pathname, "/auth");
}

function OptionalShareWorkspace({ children }: { children: React.ReactNode }) {
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
    function handleSessionChanged(event: Event) {
      const snapshot = (event as CustomEvent<SessionSnapshot>).detail;
      const nextToken = snapshot?.accessToken ?? "";
      setAccessToken(nextToken || null);
    }

    window.addEventListener(SESSION_SNAPSHOT_CHANGED_EVENT, handleSessionChanged as EventListener);
    return () => {
      window.removeEventListener(SESSION_SNAPSHOT_CHANGED_EVENT, handleSessionChanged as EventListener);
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

export function WorkspaceShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();

  if (isPublicPath(pathname)) {
    return <>{children}</>;
  }

  if (isPathWithin(pathname, "/share")) {
    return <OptionalShareWorkspace>{children}</OptionalShareWorkspace>;
  }

  const content = isPathWithin(pathname, "/admin")
    ? <AdminAccessGate>{children}</AdminAccessGate>
    : children;

  return (
    <AuthGuard>
      <ProjectLayout>{content}</ProjectLayout>
    </AuthGuard>
  );
}
