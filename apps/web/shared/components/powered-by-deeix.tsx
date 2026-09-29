"use client";

import Image from "next/image";
import { useTranslations } from "next-intl";

import { cn } from "@/lib/utils";
import { useBranding } from "@/shared/config/branding-provider";

export function PoweredByDeeix({ className }: { className?: string }) {
  const t = useTranslations("common.attribution");
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center gap-1.5 text-[11px] font-medium leading-none text-muted-foreground/70",
        className,
      )}
    >
      <span>{t("poweredBy")}</span>
      <a
        href="https://github.com/DEEIX-AI/DEEIX-Chat"
        target="_blank"
        rel="noopener noreferrer"
        aria-label={t("githubLink")}
        className="inline-flex shrink-0 rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:ring-offset-2"
      >
        <Image
          src="/logo.svg"
          alt=""
          aria-hidden="true"
          width={58}
          height={18}
          className="h-3.5 w-auto opacity-65 dark:hidden"
        />
        <Image
          src="/logo-white.svg"
          alt=""
          aria-hidden="true"
          width={58}
          height={18}
          className="hidden h-3.5 w-auto opacity-65 dark:block"
        />
      </a>
    </span>
  );
}

export function CustomBrandAttribution({ className }: { className?: string }) {
  const branding = useBranding();
  if (!branding.logoURL) {
    return null;
  }
  return (
    <div className={className}>
      <PoweredByDeeix />
    </div>
  );
}
