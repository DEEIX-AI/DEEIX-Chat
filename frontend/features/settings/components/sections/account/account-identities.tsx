"use client";

import * as React from "react";
import { Unlink } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableRow, TableSkeletonRows } from "@/components/ui/table";
import { formatDateTime } from "@/features/settings/model/account-page";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import type { IdentityProviderDTO, UserIdentityDTO } from "@/shared/api/auth.types";
import { IdentityProviderIcon } from "@/shared/components/identity-provider-icon";
import { SettingsSection } from "@/shared/components/settings-layout";

export function AccountIdentitiesSection({
  loading,
  identities,
  identityProviders,
  availableBindProviders,
  providerLogoBySlug,
  identityUnlinkDisabled,
  onBindIdentity,
  onDeleteIdentity,
}: {
  loading: boolean;
  identities: UserIdentityDTO[];
  identityProviders: IdentityProviderDTO[];
  availableBindProviders: IdentityProviderDTO[];
  providerLogoBySlug: Map<string, string>;
  identityUnlinkDisabled: boolean;
  onBindIdentity: (provider: IdentityProviderDTO) => void;
  onDeleteIdentity: (identity: UserIdentityDTO) => void;
}) {
  const t = useTranslations("settings.accountPage");
  const { locale } = useAppLocale();

  if (identityProviders.length === 0) {
    return null;
  }

  return (
    <SettingsSection
      title={t("identity.title")}
      actions={
        availableBindProviders.length > 0 ? (
          <DropdownMenu modal={false}>
            <DropdownMenuTrigger asChild>
              <Button type="button" variant="outline" disabled={loading}>
                {t("actions.bind")}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              {availableBindProviders.map((provider) => (
                <DropdownMenuItem key={provider.publicID} onClick={() => onBindIdentity(provider)}>
                  <IdentityProviderIcon name={provider.name} slug={provider.slug} logoURL={provider.logoURL} />
                  {provider.name}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        ) : null
      }
    >
      <Table className="table-fixed" style={{ minWidth: 800 }}>
        <colgroup>
          <col style={{ width: 180 }} />
          <col style={{ width: 160 }} />
          <col style={{ width: 240 }} />
          <col style={{ width: 164 }} />
          <col style={{ width: 56 }} />
        </colgroup>
        <TableHeader>
          <TableRow>
            <TableHead>{t("identity.provider")}</TableHead>
            <TableHead>{t("identity.username")}</TableHead>
            <TableHead>{t("identity.email")}</TableHead>
            <TableHead>{t("identity.linkedAt")}</TableHead>
            <TableHead className="w-[56px]" stickyEnd />
          </TableRow>
        </TableHeader>
        <TableBody>
          {loading && identities.length === 0 ? <TableSkeletonRows colSpan={5} rowCount={4} /> : null}
          {!loading && identities.length === 0 ? (
            <TableEmptyRow colSpan={5}>{t("identity.empty")}</TableEmptyRow>
          ) : null}
          {identities.map((identity) => (
            <TableRow key={identity.id}>
              <TableCell className="max-w-0 font-medium">
                <div className="flex min-w-0 items-center gap-2">
                  <IdentityProviderIcon
                    name={identity.providerName || identity.providerSlug || identity.providerType}
                    slug={identity.providerSlug}
                    logoURL={providerLogoBySlug.get(identity.providerSlug) || identity.providerLogoURL}
                  />
                  <div className="min-w-0">
                    <div className="truncate">{identity.providerName || identity.providerSlug || identity.providerType}</div>
                  </div>
                </div>
              </TableCell>
              <TableCell className="max-w-0 text-muted-foreground">
                <span className="block truncate" title={identity.providerDisplayName || undefined}>
                  {identity.providerDisplayName || "-"}
                </span>
              </TableCell>
              <TableCell className="max-w-0 text-muted-foreground">
                <span className="block truncate" title={identity.email || undefined}>
                  {identity.email || "-"}
                </span>
              </TableCell>
              <TableCell className="max-w-0 text-muted-foreground">
                <span className="block truncate" title={formatDateTime(identity.linkedAt, locale)}>
                  {formatDateTime(identity.linkedAt, locale)}
                </span>
              </TableCell>
              <TableCell className="w-[56px] whitespace-nowrap" stickyEnd>
                <div className="flex justify-end">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-8 text-muted-foreground shadow-none"
                    onClick={() => onDeleteIdentity(identity)}
                    disabled={identityUnlinkDisabled}
                    aria-label={t("identity.unlink")}
                    title={identityUnlinkDisabled ? t("identity.unlinkDisabled") : t("identity.unlink")}
                  >
                    <Unlink className="size-3.5" />
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </SettingsSection>
  );
}
