"use client";

import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { ExternalLink, Globe, Mail, Newspaper } from "lucide-react";

import packageMeta from "@/package.json";
import { Badge } from "@/components/ui/badge";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { IdentityProviderIcon } from "@/shared/components/identity-provider-icon";
import { AppLogo } from "@/shared/components/app-logo";
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
};

type AboutLinkItem = {
  label: string;
  value: string;
  href: string;
  icon?: LucideIcon;
  providerIcon?: {
    name: string;
    slug: string;
  };
};

function AboutLink({ item, className }: { item: AboutLinkItem; className?: string }) {
  const Icon = item.icon;

  return (
    <a
      href={item.href}
      target={item.href.startsWith("mailto:") ? undefined : "_blank"}
      rel={item.href.startsWith("mailto:") ? undefined : "noreferrer"}
      className={cn(
        "group flex min-w-0 items-center justify-between gap-4 border-b border-border/60 py-3 text-sm transition-colors hover:border-foreground/30",
        className,
      )}
    >
      <span className="flex min-w-0 items-center gap-2.5 text-muted-foreground">
        {item.providerIcon ? (
          <IdentityProviderIcon
            name={item.providerIcon.name}
            slug={item.providerIcon.slug}
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
        <ExternalLink className="size-3 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
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
}: AboutSettingsContentProps) {
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
      providerIcon: { name: "GitHub", slug: "github" },
    },
    {
      label: labels.social,
      value: "@DEEIX_AI",
      href: "https://x.com/DEEIX_AI",
      providerIcon: { name: "X", slug: "x" },
    },
    {
      label: labels.repository,
      value: "DEEIX-Chat",
      href: "https://github.com/DEEIX-AI/DEEIX-Chat",
      providerIcon: { name: "GitHub", slug: "github" },
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
              <AppLogo width={180} height={56} className="h-auto w-36 sm:w-44" />
            </div>
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <span className="text-xs text-muted-foreground">{consoleLabel}</span>
              {versionBadgeTooltip ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Badge variant="secondary" className="cursor-default">
                      {versionBadgeContent ?? `v${packageMeta.version}`}
                    </Badge>
                  </TooltipTrigger>
                  <TooltipContent>{versionBadgeTooltip}</TooltipContent>
                </Tooltip>
              ) : (
                <Badge variant="secondary">{versionBadgeContent ?? `v${packageMeta.version}`}</Badge>
              )}
              {versionActions ? <span className="ml-1.5 flex min-w-0 items-center gap-2">{versionActions}</span> : null}
            </div>
          </div>

          <p className="max-w-[760px] text-sm leading-6 text-muted-foreground">
            {description}
          </p>
        </div>
      </SettingsSection>

      <SettingsSection title={labels.details}>
        <div className="grid gap-x-8 px-0.5 md:grid-cols-2">
          {links.map((item) => (
            <AboutLink key={`${item.label}-${item.value}`} item={item} />
          ))}
        </div>
        <div className="space-y-1 px-0.5 pt-4 text-xs text-muted-foreground">
          <p>{labels.copyright}</p>
          <a
            href="https://www.apache.org/licenses/LICENSE-2.0"
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-1 font-medium text-foreground/80 transition-colors hover:text-foreground"
          >
            <span>{labels.license}</span>
            <ExternalLink className="size-3 shrink-0 text-muted-foreground" />
          </a>
        </div>
      </SettingsSection>
    </SettingsPage>
  );
}
