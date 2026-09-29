"use client";

import { useTranslations } from "next-intl";

import { useDesktopDistribution } from "@/features/desktop/hooks/use-desktop-distribution";
import { hasActivePolicy } from "@/shared/platform/desktop-distribution";

// About page line for the desktop app: how this copy was distributed and whether
// enterprise policy manages part of it. Renders nothing in a browser, in a dev
// build, or when the shell could not report its distribution.

export function DesktopDistributionDetails() {
  const t = useTranslations("desktopUpdate.distribution");
  const { status, distribution } = useDesktopDistribution();

  if (status !== "ready" || distribution.kind === "dev") {
    return null;
  }
  const managed = hasActivePolicy(distribution.policy);

  return (
    <div className="flex min-w-0 flex-col gap-1 text-xs text-muted-foreground">
      <p>{t("edition", { kind: t(`kinds.${distribution.kind}`) })}</p>
      {managed ? <p>{t("managed")}</p> : null}
    </div>
  );
}
