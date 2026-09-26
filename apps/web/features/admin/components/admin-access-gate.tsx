"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { CenteredEmptyState } from "@/components/ui/empty-state";
import { useAuthSession } from "@/shared/auth/auth-session-context";

export function AdminAccessGate({ children }: { children: React.ReactNode }) {
  const t = useTranslations("adminUsers");
  const { user, userStatus } = useAuthSession();

  if (userStatus === "loading") {
    return null;
  }

  if (user?.role !== "admin" && user?.role !== "superadmin") {
    return (
      <main className="h-full min-h-0 w-full flex-1 bg-background">
        <CenteredEmptyState title={t("accessDeniedTitle")} description={t("accessDeniedDescription")} />
      </main>
    );
  }

  return <>{children}</>;
}
