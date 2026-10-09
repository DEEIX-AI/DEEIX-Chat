"use client";

import { Ban, CircleHelp, Globe, Link2, MoreHorizontal, RotateCcw, Save, Trash2, UserRound } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useLocale, useTranslations } from "next-intl";
import * as React from "react";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldLabel } from "@/components/ui/field";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableLoadingRow, TableRow } from "@/components/ui/table";
import { TablePagination, TableToolbar } from "@/components/ui/table-tools";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ModelIcon, resolveModelIconURL, resolveModelProviderIcon } from "@/entities/model";
import { type UserKeysSettingKey, useAdminUserKeysPolicy } from "@/features/admin/hooks/use-admin-user-keys-policy";
import { useAdminUserKeysProviders } from "@/features/admin/hooks/use-admin-user-keys-providers";
import { ModelsImportLinkDialog } from "@/features/settings";
import type { AdminPersonalProviderDTO } from "@/shared/api/personal-providers-types";
import { SettingsFieldInset, SettingsFieldItem, SettingsFieldList, SettingsPage, SettingsSection } from "@/shared/components/settings-layout";
import { resolveAvatarImageSrc } from "@/shared/lib/avatar";
import { SettingsFieldEditor, type SettingsFieldDefinition } from "../../shared/settings-runtime-panel";

/**
 * Users' own API keys: the policy that allows them (off by default) and every
 * provider users have added. Administrators see hosts and key hints, never
 * keys, and can disable a provider, a whole domain, or delete it.
 */
export function AdminUserKeysPage() {
  const t = useTranslations("adminUserKeys");
  const commonT = useTranslations("common");
  const policy = useAdminUserKeysPolicy();
  const providers = useAdminUserKeysProviders();
  const locale = useLocale();
  const dateFormatter = React.useMemo(() => new Intl.DateTimeFormat(locale, { dateStyle: "medium" }), [locale]);

  const enabledField: SettingsFieldDefinition = {
    id: "personal_provider.enabled",
    type: "bool",
    label: t("policy.enabled.label"),
    description: t("policy.enabled.description"),
  };
  // Only meaningful once the feature is on, so they sit under the switch.
  const limitFields: Array<SettingsFieldDefinition & { key: UserKeysSettingKey }> = [
    { key: "max_per_user", id: "personal_provider.max_per_user", type: "int", label: t("policy.maxPerUser.label"), description: t("policy.maxPerUser.description") },
    {
      key: "blocked_hosts",
      id: "personal_provider.blocked_hosts",
      type: "textarea",
      label: t("policy.blockedHosts.label"),
      description: t("policy.blockedHosts.description"),
      placeholder: "example.com\n*.relay.example.org",
    },
  ];
  const settingsDisabled = policy.loading || policy.saving;
  const [linkOpen, setLinkOpen] = React.useState(false);

  const selectedIDs = [...providers.selected];
  const allSelected = providers.items.length > 0 && selectedIDs.length === providers.items.length;
  const confirm = providers.confirm;

  return (
    <SettingsPage>
      <SettingsSection
        title={
          <span className="inline-flex items-center gap-1">
            {t("policy.title")}
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-xs"
                  className="text-muted-foreground hover:bg-transparent hover:text-foreground"
                  aria-label={t("policy.securityLabel")}
                >
                  <CircleHelp className="size-3.5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent side="right" className="max-w-xs text-xs leading-5">
                {t("policy.securityNote")}
              </TooltipContent>
            </Tooltip>
          </span>
        }
        actions={
          <>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-8 gap-1.5 px-2.5 text-xs font-normal text-muted-foreground hover:text-foreground"
              onClick={() => setLinkOpen(true)}
            >
              <Link2 className="size-3.5 stroke-1" />
              {t("policy.importLink")}
            </Button>
            {policy.dirty ? (
              <Button type="button" size="sm" disabled={policy.loading || policy.saving} onClick={() => void policy.save()}>
                <Save className="size-3.5" />
                {commonT("actions.save")}
              </Button>
            ) : null}
          </>
        }
      >
        <SettingsFieldList>
          <SettingsFieldItem>
            <SettingsFieldEditor
              field={enabledField}
              value={policy.values.enabled}
              dirty={policy.isDirty("enabled")}
              disabled={settingsDisabled}
              onChange={(value) => policy.setValue("enabled", value)}
            />
          </SettingsFieldItem>
          <AnimatePresence initial={false}>
            {policy.values.enabled === "true" ? (
              <motion.div
                key="personal-provider-limits"
                initial={{ opacity: 0, gridTemplateRows: "0fr" }}
                animate={{ opacity: 1, gridTemplateRows: "1fr" }}
                exit={{ opacity: 0, gridTemplateRows: "0fr" }}
                transition={{ duration: 0.2, ease: [0.22, 1, 0.36, 1] }}
                style={{ display: "grid" }}
              >
                <div className="-mx-px overflow-hidden px-px pb-px">
                  <SettingsFieldItem index={1}>
                    <SettingsFieldInset>
                      <SettingsFieldList className="gap-3 md:gap-4">
                        {limitFields.map((field) => (
                          <SettingsFieldEditor
                            key={field.id}
                            field={field}
                            value={policy.values[field.key]}
                            dirty={policy.isDirty(field.key)}
                            disabled={settingsDisabled}
                            onChange={(value) => policy.setValue(field.key, value)}
                          />
                        ))}
                      </SettingsFieldList>
                    </SettingsFieldInset>
                  </SettingsFieldItem>
                </div>
              </motion.div>
            ) : null}
          </AnimatePresence>
        </SettingsFieldList>
      </SettingsSection>

      <SettingsSection>
        <Field className="gap-2">
          <FieldLabel>{t("list.title")}</FieldLabel>
          <TableToolbar
            query={providers.query}
            onQueryChange={providers.setQuery}
            queryPlaceholder={t("list.searchPlaceholder")}
            filters={[
              {
                key: "status",
                label: t("list.columns.status"),
                value: providers.status,
                onValueChange: providers.setStatus,
                options: [
                  { label: t("list.allStatus"), value: "" },
                  { label: t("status.active"), value: "active" },
                  { label: t("status.disabled"), value: "disabled" },
                  { label: t("status.suspended"), value: "suspended" },
                ],
              },
            ]}
            selectedCount={selectedIDs.length}
            bulkActions={[
              {
                key: "suspend",
                label: t("actions.suspend"),
                icon: <Ban className="size-3.5 stroke-1" />,
                onClick: () => providers.setConfirm({ kind: "suspend", ids: selectedIDs, label: t("list.selectedCount", { count: selectedIDs.length }) }),
                disabled: providers.acting,
              },
              {
                key: "resume",
                label: t("actions.resume"),
                icon: <RotateCcw className="size-3.5 stroke-1" />,
                onClick: () => void providers.resume(selectedIDs),
                disabled: providers.acting,
              },
              {
                key: "delete",
                label: t("actions.delete"),
                icon: <Trash2 className="size-3.5 stroke-1" />,
                onClick: () => providers.setConfirm({ kind: "delete", ids: selectedIDs, label: t("list.selectedCount", { count: selectedIDs.length }) }),
                disabled: providers.acting,
              },
            ]}
            loading={providers.loading || providers.acting}
            onRefresh={() => void providers.reload()}
          />

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-[44px] py-1.5 text-center" stickyStart>
                  <div className="flex h-7 items-center justify-center">
                    <Checkbox
                      checked={allSelected ? true : selectedIDs.length > 0 ? "indeterminate" : false}
                      onCheckedChange={(checked) => providers.selectAll(checked === true)}
                      aria-label={t("list.selectAll")}
                    />
                  </div>
                </TableHead>
                <TableHead className="left-[44px] min-w-[112px]" stickyStart>{t("list.columns.owner")}</TableHead>
                <TableHead className="min-w-[120px]">{t("list.columns.name")}</TableHead>
                <TableHead>{t("list.columns.baseURL")}</TableHead>
                <TableHead className="w-[64px] text-center">{t("list.columns.models")}</TableHead>
                <TableHead className="w-[80px] text-center">{t("list.columns.status")}</TableHead>
                <TableHead className="w-[112px]">{t("list.columns.created")}</TableHead>
                <TableHead stickyEnd className="w-[56px]" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {providers.loading && providers.items.length === 0 ? <TableLoadingRow colSpan={8} /> : null}
              {!providers.loading && providers.items.length === 0 ? (
                <TableEmptyRow colSpan={8}>{providers.query.trim() || providers.status ? t("list.searchEmpty") : t("list.empty")}</TableEmptyRow>
              ) : null}
              {providers.items.map((item) => (
                <TableRow key={item.id} tone={item.status === "active" ? undefined : "muted"}>
                  <TableCell className="w-[44px] py-1.5 text-center" stickyStart>
                    <div className="flex h-7 items-center justify-center">
                      <Checkbox
                        checked={providers.selected.has(item.id)}
                        onCheckedChange={(checked) => providers.toggleSelected(item.id, checked === true)}
                        aria-label={t("list.selectOne", { name: item.name })}
                      />
                    </div>
                  </TableCell>
                  <TableCell className="left-[44px] max-w-[180px] py-1.5" stickyStart>
                    <ProviderOwner item={item} />
                  </TableCell>
                  <TableCell className="py-1.5">
                    <div className="flex max-w-[14rem] min-w-0 items-center gap-2">
                      <ProviderIcon icon={item.icon} baseURL={item.baseURL} name={item.name} />
                      <span className="truncate font-medium text-foreground">{item.name}</span>
                    </div>
                  </TableCell>
                  <TableCell className="py-1.5">
                    <div className="max-w-[11rem] truncate text-xs text-muted-foreground" title={item.baseURL}>
                      {item.baseURL}
                    </div>
                  </TableCell>
                  <TableCell className="py-1.5 text-center tabular-nums text-muted-foreground">{item.modelCount}</TableCell>
                  <TableCell className="py-1.5 text-center">
                    <ProviderStatusBadge item={item} />
                  </TableCell>
                  <TableCell className="py-1.5 text-muted-foreground">{dateFormatter.format(new Date(item.createdAt))}</TableCell>
                  <TableCell stickyEnd className="w-[56px] py-1.5 text-right">
                    <div className="flex h-7 items-center justify-end">
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button type="button" size="icon-xs" variant="ghost" className="text-muted-foreground shadow-none" aria-label={t("list.moreActions")} disabled={providers.acting}>
                            <MoreHorizontal className="size-3.5 stroke-1" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          {item.ownerUsername ? (
                            <>
                              <DropdownMenuItem onClick={() => providers.setQuery(item.ownerUsername)}>
                                <UserRound className="size-3.5 stroke-1" />
                                {t("actions.filterOwner")}
                              </DropdownMenuItem>
                              <DropdownMenuSeparator />
                            </>
                          ) : null}
                          {item.status === "suspended" ? (
                            <DropdownMenuItem onClick={() => void providers.resume([item.id])}>
                              <RotateCcw className="size-3.5 stroke-1" />
                              {t("actions.resume")}
                            </DropdownMenuItem>
                          ) : (
                            <DropdownMenuItem onClick={() => providers.setConfirm({ kind: "suspend", ids: [item.id], label: item.name })}>
                              <Ban className="size-3.5 stroke-1" />
                              {t("actions.suspend")}
                            </DropdownMenuItem>
                          )}
                          <DropdownMenuItem onClick={() => providers.setConfirm({ kind: "suspendHost", host: item.host })}>
                            <Globe className="size-3.5 stroke-1" />
                            {t("actions.suspendHost", { host: item.host })}
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem variant="destructive" onClick={() => providers.setConfirm({ kind: "delete", ids: [item.id], label: item.name })}>
                            <Trash2 className="size-3.5 stroke-1" />
                            {t("actions.delete")}
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>

          <TablePagination
            total={providers.total}
            page={providers.page}
            pageCount={providers.pageCount}
            pageSize={providers.pageSize}
            onPageChange={providers.setPage}
            onPageSizeChange={providers.setPageSize}
            loading={providers.loading}
          />
        </Field>
      </SettingsSection>

      <AlertDialog open={confirm !== null} onOpenChange={(open) => !open && providers.setConfirm(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {confirm?.kind === "delete" ? t("confirm.deleteTitle") : confirm?.kind === "suspendHost" ? t("confirm.suspendHostTitle") : t("confirm.suspendTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {confirm?.kind === "delete"
                ? t("confirm.deleteDescription", { name: confirm.label })
                : confirm?.kind === "suspendHost"
                  ? t("confirm.suspendHostDescription", { host: confirm.host })
                  : confirm
                    ? t("confirm.suspendDescription", { name: confirm.label })
                    : null}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{commonT("actions.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant={confirm?.kind === "delete" ? "destructive" : "default"}
              onClick={() => void providers.confirmPending()}
            >
              {confirm?.kind === "delete" ? t("actions.delete") : t("actions.suspend")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
      <ModelsImportLinkDialog open={linkOpen} onOpenChange={setLinkOpen} />
    </SettingsPage>
  );
}

function ProviderStatusBadge({ item }: { item: AdminPersonalProviderDTO }) {
  const t = useTranslations("adminUserKeys");
  if (item.status === "suspended") {
    return <Badge variant="secondary" className="border-0 font-normal text-destructive shadow-none">{t("status.suspended")}</Badge>;
  }
  if (item.status === "disabled") {
    return <Badge variant="secondary" className="border-0 font-normal text-muted-foreground shadow-none">{t("status.disabled")}</Badge>;
  }
  if (item.lastError) {
    return <Badge variant="secondary" className="border-0 font-normal text-amber-700 shadow-none dark:text-amber-300">{t("status.error")}</Badge>;
  }
  return <Badge variant="secondary" className="border-0 font-normal shadow-none">{t("status.active")}</Badge>;
}

/** Same icon the owner sees in their settings: their pick, else the address's brand icon. */
function ProviderIcon({ icon, baseURL, name }: { icon: string; baseURL: string; name: string }) {
  const slug = resolveModelProviderIcon(icon, baseURL);
  return <ModelIcon iconUrl={slug ? resolveModelIconURL(slug) : null} label={name} />;
}

/** Who added the provider: the username, with avatar, nickname and email in a tooltip. */
function ProviderOwner({ item }: { item: AdminPersonalProviderDTO }) {
  const t = useTranslations("adminUserKeys");
  const username = item.ownerUsername.trim();
  if (!username) {
    return <span className="text-muted-foreground">{t("list.unknownUser")}</span>;
  }
  const nickname = item.ownerDisplayName.trim();
  const email = item.ownerEmail.trim();
  const source = { publicID: item.ownerPublicID, username, displayName: nickname };
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="block max-w-full cursor-default truncate text-foreground">{username}</span>
      </TooltipTrigger>
      <TooltipContent side="top" className="px-2.5 py-2">
        <div className="flex max-w-64 items-center gap-2.5">
          <Avatar className="size-7 shrink-0 rounded-full">
            <AvatarImage src={resolveAvatarImageSrc(item.ownerAvatarURL, source) || undefined} alt={nickname || username} />
            <AvatarFallback className="rounded-full bg-background text-[11px] font-medium text-foreground">
              {(nickname || username).charAt(0).toUpperCase()}
            </AvatarFallback>
          </Avatar>
          <div className="min-w-0">
            <p className="truncate font-medium">{nickname || username}</p>
            {email ? <p className="truncate text-[11px] leading-4 opacity-70">{email}</p> : null}
          </div>
        </div>
      </TooltipContent>
    </Tooltip>
  );
}
