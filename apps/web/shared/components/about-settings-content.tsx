"use client";

import type { ComponentType, ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { ExternalLink, Globe, Mail, Newspaper } from "lucide-react";

import { useTranslations } from "next-intl";

import packageMeta from "@/package.json";
import { Badge } from "@/components/ui/badge";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { DeeixLogo } from "@/shared/components/app-logo";
import {
  SettingsPage,
  SettingsSection,
} from "@/shared/components/settings-layout";
import { cn } from "@/lib/utils";

type AboutLabels = {
  details: string;
  official: string;
  website: string;
  repository: string;
  social: string;
  blog: string;
  contact: string;
  copyright: string;
  license: string;
};

type AboutSettingsContentProps = {
  title: string;
  description: string;
  consoleLabel: string;
  labels: AboutLabels;
  versionBadgeContent?: ReactNode;
  versionBadgeTooltip?: ReactNode;
  versionActions?: ReactNode;
  // Extra lines under the version (e.g. how the desktop app was distributed).
  // Supplied by the caller so this panel stays free of platform concerns.
  versionDetails?: ReactNode;
  // Extra sections appended after the product details (e.g. admin-only settings).
  extraSections?: ReactNode;
  // Renders third-party brand marks (GitHub, X) of the official links. Injected by the
  // caller so this product panel stays free of the identity-provider entity.
  brandIcon?: AboutBrandIcon;
};

export type AboutBrandIcon = ComponentType<{
  name: string;
  slug: string;
  className?: string;
  iconClassName?: string;
}>;

type AboutLinkItem = {
  label: string;
  value: string;
  href: string;
  icon?: LucideIcon;
  brand?: {
    name: string;
    slug: string;
  };
};

function AboutLink({
  item,
  brandIcon: BrandIcon,
  className,
}: {
  item: AboutLinkItem;
  brandIcon?: AboutBrandIcon;
  className?: string;
}) {
  const Icon = item.icon;

  return (
    <a
      href={item.href}
      target={item.href.startsWith("mailto:") ? undefined : "_blank"}
      rel={item.href.startsWith("mailto:") ? undefined : "noreferrer"}
      className={cn(
        "group relative isolate flex min-w-0 items-center justify-between gap-4 border-b border-border/60 px-2.5 py-3 text-sm transition-colors outline-none hover:border-foreground/30 focus-visible:border-foreground/30 focus-visible:ring-0",
        "before:pointer-events-none before:absolute before:-inset-x-0.5 before:inset-y-1 before:-z-10 before:rounded-md before:bg-muted/60 before:opacity-0 before:transition-opacity focus-visible:before:opacity-100",
        className,
      )}
    >
      <span className="flex min-w-0 items-center gap-2.5 text-muted-foreground">
        {item.brand && BrandIcon ? (
          <BrandIcon
            name={item.brand.name}
            slug={item.brand.slug}
            className="size-3.5"
            iconClassName="size-3.5"
          />
        ) : Icon ? (
          <Icon className="size-3.5 shrink-0" />
        ) : null}
        <span className="truncate">{item.label}</span>
      </span>
      <span className="flex min-w-0 items-center gap-1.5 text-right font-medium text-foreground">
        <span className="truncate">{item.value}</span>
        <ExternalLink className="size-3 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100" />
      </span>
    </a>
  );
}

export function AboutSettingsContent({
  title,
  description,
  consoleLabel,
  labels,
  versionBadgeContent,
  versionBadgeTooltip,
  versionActions,
  versionDetails,
  extraSections,
  brandIcon,
}: AboutSettingsContentProps) {
  const tCommon = useTranslations("common.appVersion");
  // A "-" in the version means a pre-release build (e.g. 0.4.4-beta.1).
  const isPrerelease = packageMeta.version.includes("-");
  const versionBadge = (
    <>
      {versionBadgeContent ?? `v${packageMeta.version}`}
      <span className="text-muted-foreground">· {tCommon(isPrerelease ? "prerelease" : "stable")}</span>
    </>
  );
  const links: AboutLinkItem[] = [
    {
      label: labels.website,
      value: "deeix.com",
      href: "https://deeix.com",
      icon: Globe,
    },
    {
      label: labels.official,
      value: "DEEIX",
      href: "https://github.com/DEEIX-AI",
      brand: { name: "GitHub", slug: "github" },
    },
    {
      label: labels.social,
      value: "@DEEIX_AI",
      href: "https://x.com/DEEIX_AI",
      brand: { name: "X", slug: "x" },
    },
    {
      label: labels.repository,
      value: "DEEIX-Chat",
      href: "https://github.com/DEEIX-AI/DEEIX-Chat",
      brand: { name: "GitHub", slug: "github" },
    },
    {
      label: labels.blog,
      value: "blog.cheny.me",
      href: "https://blog.cheny.me/",
      icon: Newspaper,
    },
    {
      label: labels.contact,
      value: "support@deeix.com",
      href: "mailto:support@deeix.com",
      icon: Mail,
    },
  ];

  return (
    <SettingsPage>
      <SettingsSection title={title}>
        <div className="space-y-5 px-0.5">
          <div className="flex min-w-0 flex-col gap-2.5">
            <div className="flex h-14 w-40 shrink-0 items-center sm:w-48">
              <DeeixLogo width={180} height={56} className="h-auto w-36 sm:w-44" />
            </div>
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <span className="text-xs text-muted-foreground">{consoleLabel}</span>
              {versionBadgeTooltip ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Badge variant="secondary" className="cursor-default">
                      {versionBadge}
                    </Badge>
                  </TooltipTrigger>
                  <TooltipContent>{versionBadgeTooltip}</TooltipContent>
                </Tooltip>
              ) : (
                <Badge variant="secondary">{versionBadge}</Badge>
              )}
              {versionActions ? <span className="ml-1.5 flex min-w-0 items-center gap-2">{versionActions}</span> : null}
            </div>
            {versionDetails}
          </div>

          <p className="max-w-[760px] text-sm leading-6 text-muted-foreground">
            {description}
          </p>
        </div>
      </SettingsSection>

      <SettingsSection title={labels.details}>
        <div className="grid gap-x-8 px-0.5 md:grid-cols-2">
          {links.map((item) => (
            <AboutLink key={`${item.label}-${item.value}`} item={item} brandIcon={brandIcon} />
          ))}
        </div>
        <div className="space-y-1 px-0.5 pt-4 text-xs text-muted-foreground">
          <p>{labels.copyright}</p>
          <a
            href="https://www.apache.org/licenses/LICENSE-2.0"
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-1 font-medium text-foreground/80 transition-colors outline-none hover:text-foreground focus-visible:text-foreground focus-visible:ring-0"
          >
            <span>{labels.license}</span>
            <ExternalLink className="size-3 shrink-0 text-muted-foreground" />
          </a>
        </div>
      </SettingsSection>
      {extraSections}
    </SettingsPage>
  );
}
