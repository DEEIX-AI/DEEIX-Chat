"use client";

import * as React from "react";
import { MoreHorizontal } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
  formatDateTime,
  resolveSessionIP,
  resolveSessionLocation,
  resolveSessionTitle,
} from "@/features/settings/model/account-page";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import type { ActiveSessionDTO } from "@/shared/api/auth.types";
import { SettingsSection } from "@/shared/components/settings-layout";

export function AccountActiveSessionsSection({
  sessions,
  loading,
  revokingSessionID,
  onLogoutSession,
}: {
  sessions: ActiveSessionDTO[];
  loading: boolean;
  revokingSessionID: string;
  onLogoutSession: (session: ActiveSessionDTO) => void;
}) {
  const t = useTranslations("settings.accountPage");
  const { locale } = useAppLocale();

  return (
    <SettingsSection title={t("session.title")}>
      <Table className="table-fixed" style={{ minWidth: 840 }}>
        <colgroup>
          <col style={{ width: 260 }} />
          <col style={{ width: 220 }} />
          <col style={{ width: 152 }} />
          <col style={{ width: 152 }} />
          <col style={{ width: 56 }} />
        </colgroup>
        <TableHeader>
          <TableRow>
            <TableHead>{t("session.device")}</TableHead>
            <TableHead>{t("session.location")}</TableHead>
            <TableHead>{t("session.createdAt")}</TableHead>
            <TableHead>{t("session.updatedAt")}</TableHead>
            <TableHead className="w-[56px]" stickyEnd />
          </TableRow>
        </TableHeader>
        <TableBody>
          {!loading && sessions.length === 0 ? (
            <TableEmptyRow colSpan={5}>{t("session.empty")}</TableEmptyRow>
          ) : null}

          {(loading ? Array.from({ length: 2 }) : sessions).map((item, index) => {
            if (loading) {
              return (
                <TableRow key={`session-skeleton-${index}`}>
                  <TableCell>
                    <div className="my-2 h-4 w-full max-w-[10rem] animate-pulse rounded-full bg-muted/60" />
                  </TableCell>
                  <TableCell>
                    <div className="my-2 h-4 w-full max-w-[12rem] animate-pulse rounded-full bg-muted/50" />
                  </TableCell>
                  <TableCell>
                    <div className="my-2 h-4 w-full max-w-[8rem] animate-pulse rounded-full bg-muted/50" />
                  </TableCell>
                  <TableCell>
                    <div className="my-2 h-4 w-full max-w-[8rem] animate-pulse rounded-full bg-muted/50" />
                  </TableCell>
                  <TableCell>
                    <div className="ml-auto my-2 h-4 w-4 animate-pulse rounded-full bg-muted/50" />
                  </TableCell>
                </TableRow>
              );
            }

            const session = item as ActiveSessionDTO;

            return (
              <TableRow key={session.sessionID}>
                <TableCell className="max-w-0">
                  <div className="flex min-w-0 items-center gap-2">
                    <span className="min-w-0 truncate font-medium" title={resolveSessionTitle(session, t)}>
                      {resolveSessionTitle(session, t)}
                    </span>
                    {session.current ? (
                      <span className="inline-flex shrink-0 items-center rounded-md bg-muted px-1.5 py-0.5 text-xs">
                        {t("session.current")}
                      </span>
                    ) : null}
                  </div>
                </TableCell>

                <TableCell className="max-w-0 text-muted-foreground">
                  <div className="flex min-w-0 flex-col gap-1">
                    <span className="truncate" title={resolveSessionLocation(session, t)}>{resolveSessionLocation(session, t)}</span>
                    <span className="truncate text-xs" title={resolveSessionIP(session, t)}>{resolveSessionIP(session, t)}</span>
                  </div>
                </TableCell>
                <TableCell className="max-w-0 text-muted-foreground">
                  <span className="block truncate" title={formatDateTime(session.createdAt, locale)}>
                    {formatDateTime(session.createdAt, locale)}
                  </span>
                </TableCell>
                <TableCell className="max-w-0 text-muted-foreground">
                  <span className="block truncate" title={formatDateTime(session.updatedAt, locale)}>
                    {formatDateTime(session.updatedAt, locale)}
                  </span>
                </TableCell>
                <TableCell className="w-[56px] whitespace-nowrap" stickyEnd>
                  <div className="flex justify-end">
                    <DropdownMenu modal={false}>
                      <DropdownMenuTrigger asChild>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="size-8"
                          disabled={revokingSessionID === session.sessionID}
                          aria-label={t("session.actions")}
                        >
                          <MoreHorizontal className="size-3.5 stroke-1" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem
                          disabled={revokingSessionID === session.sessionID}
                          onClick={() => onLogoutSession(session)}
                        >
                          {t("session.logoutThisSession")}
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </SettingsSection>
  );
}
