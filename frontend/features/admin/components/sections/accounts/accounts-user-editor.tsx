"use client";

import * as React from "react";
import { motion } from "motion/react";
import { useLocale, useTranslations } from "next-intl";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from "@/components/ui/combobox";
import {
  DialogCollapsible,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Sheet,
  SheetContent,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import {
  DISPLAY_NAME_MAX_LENGTH,
  PASSWORD_MIN_LENGTH,
  USERNAME_MAX_LENGTH,
} from "@/shared/auth/account-policy";
import { resolveAvatarImageSrc } from "@/shared/lib/avatar";
import { TimeZoneSelect } from "@/shared/components/time-zone-select";
import { cn } from "@/lib/utils";
import { AdminDateTimePicker } from "@/features/admin/components/admin-date-time-picker";
import type { UserDTO } from "@/shared/api/auth.types";
import type { AdminUserRole, AdminUserStatus } from "@/features/admin/api/admin.types";
import {
  COMPACT_COMBOBOX_CLASSNAME,
  USER_STATUS_OPTIONS,
  type CreateUserPayload,
  type EditUserPayload,
  type UserTier,
} from "@/features/admin/types/accounts";
import type { AdminBillingMode, AdminBillingPlanDTO } from "@/features/admin/api/billing.types";
import { formatBillingBalance, resolveDetailValue } from "@/features/admin/utils/account-display";

const DIALOG_LAYOUT_TRANSITION = {
  layout: {
    duration: 0.22,
    ease: [0.16, 1, 0.3, 1] as const,
  },
};

function formatSheetDateTime(value: string | null | undefined, locale: string): string {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function UserAvatarButton({
  onClick,
  disabled,
  src,
  alt,
  fallback,
}: {
  onClick: () => void;
  disabled?: boolean;
  src?: string;
  alt: string;
  fallback: string;
}) {
  return (
    <button
      type="button"
      className="rounded-full transition-opacity hover:opacity-85 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      onClick={onClick}
      disabled={disabled}
    >
      <Avatar className="h-9 w-9 rounded-full">
        <AvatarImage src={src || undefined} alt={alt} />
        <AvatarFallback className="rounded-full bg-foreground text-xs font-medium text-background">
          {fallback}
        </AvatarFallback>
      </Avatar>
    </button>
  );
}

type CreateUserDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  pending: boolean;
  createDialogContentRef?: React.RefObject<HTMLDivElement | null>;
  createPayload: CreateUserPayload;
  setCreatePayload: React.Dispatch<React.SetStateAction<CreateUserPayload>>;
  billingMode: AdminBillingMode;
  billingPlans: AdminBillingPlanDTO[];
  createAvatarSource: {
    username: string;
    displayName: string;
  };
  onOpenCreateAvatarDialog: () => void;
  onCreateSubmit: React.FormEventHandler<HTMLFormElement>;
  resolveCreateUserInitial: (username: string, displayName: string) => string;
};

export function CreateUserDialog({
  open,
  onOpenChange,
  pending,
  createDialogContentRef,
  createPayload,
  setCreatePayload,
  billingMode,
  billingPlans,
  createAvatarSource,
  onOpenCreateAvatarDialog,
  onCreateSubmit,
  resolveCreateUserInitial,
}: CreateUserDialogProps) {
  const t = useTranslations("adminUsers");
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        ref={createDialogContentRef}
        className="flex max-h-[min(86vh,760px)] w-[calc(100vw-2rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[560px]"
      >
        <DialogHeader className="shrink-0 px-4 py-4">
          <DialogTitle>{t("editor.createTitle")}</DialogTitle>
          <DialogDescription>{t("editor.createDescription")}</DialogDescription>
        </DialogHeader>

        <motion.form layout transition={DIALOG_LAYOUT_TRANSITION} onSubmit={onCreateSubmit} className="flex min-h-0 flex-1 flex-col">
          <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-2">
            <div className="grid grid-cols-[auto_minmax(0,1fr)_minmax(0,1fr)] items-end gap-5">
              <div className="space-y-1">
                <UserAvatarButton
                  onClick={onOpenCreateAvatarDialog}
                  disabled={pending}
                  src={resolveAvatarImageSrc(createPayload.avatarURL, createAvatarSource)}
                  alt={t("avatar.preview")}
                  fallback={resolveCreateUserInitial(createPayload.username, createPayload.displayName)}
                />
              </div>
              <div className="space-y-1">
                <p className="text-xs font-normal text-muted-foreground">{t("editor.username")}</p>
                <Input
                  value={createPayload.username}
                  placeholder={t("editor.usernamePlaceholder")}
                  onChange={(event) => setCreatePayload((current) => ({ ...current, username: event.target.value }))}
                  maxLength={USERNAME_MAX_LENGTH}
                  required
                />
              </div>
              <div className="space-y-1">
                <p className="text-xs font-normal text-muted-foreground">{t("editor.displayName")}</p>
                <Input
                  value={createPayload.displayName}
                  placeholder={t("editor.displayNamePlaceholder")}
                  onChange={(event) => setCreatePayload((current) => ({ ...current, displayName: event.target.value }))}
                  maxLength={DISPLAY_NAME_MAX_LENGTH}
                />
              </div>
            </div>

            <div className="space-y-1">
              <p className="text-xs font-normal text-muted-foreground">{t("editor.password")}</p>
              <Input
                value={createPayload.password}
                placeholder={t("editor.passwordPlaceholder")}
                type="password"
                onChange={(event) => setCreatePayload((current) => ({ ...current, password: event.target.value }))}
                minLength={PASSWORD_MIN_LENGTH}
                required
              />
            </div>

            <div className="space-y-1">
              <p className="text-xs font-normal text-muted-foreground">{t("editor.email")}</p>
              <Input
                value={createPayload.email}
                placeholder={t("editor.emailPlaceholder")}
                onChange={(event) => setCreatePayload((current) => ({ ...current, email: event.target.value }))}
              />
            </div>

            {billingMode === "period" ? (
              <div className="space-y-3">
                <div className="space-y-1">
                  <p className="text-xs font-normal text-muted-foreground">{t("editor.subscriptionPlan")}</p>
                  <Combobox
                    items={billingPlans.map((plan) => plan.code)}
                    value={createPayload.subscriptionTier}
                    filter={null}
                    autoComplete="none"
                    onValueChange={(value) =>
                      setCreatePayload((current) => ({
                        ...current,
                        subscriptionTier: value as UserTier,
                        subscriptionExpiresAt: value === "free" ? "" : current.subscriptionExpiresAt,
                      }))
                    }
                    disabled={pending}
                  >
                    <ComboboxInput className="w-full min-w-0" placeholder={t("editor.selectSubscriptionPlan")} showClear={false} disabled={pending} />
                    <ComboboxContent portalContainer={createDialogContentRef}>
                      <ComboboxEmpty>{t("editor.noMatchingSubscriptionPlans")}</ComboboxEmpty>
                      <ComboboxList>
                        {(tier: UserTier) => (
                          <ComboboxItem key={tier} value={tier}>
                            {billingPlans.find((plan) => plan.code === tier)?.name ?? tier}
                          </ComboboxItem>
                        )}
                      </ComboboxList>
                    </ComboboxContent>
                  </Combobox>
                </div>

                <DialogCollapsible open={createPayload.subscriptionTier !== "free"}>
                  <AdminDateTimePicker
                    value={createPayload.subscriptionExpiresAt}
                    label={t("editor.expiryTime")}
                    placeholder={t("editor.selectExpiryDate")}
                    granularity="date"
                    disabled={createPayload.subscriptionTier === "free"}
                    disabledDate={{ before: new Date() }}
                    onChange={(value) =>
                      setCreatePayload((current) => ({
                        ...current,
                        subscriptionExpiresAt: value,
                      }))
                    }
                  />
                </DialogCollapsible>
              </div>
            ) : null}
          </div>

          <DialogFooter className="shrink-0 px-4 py-3">
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)} disabled={pending}>
              {t("actions.cancel")}
            </Button>
            <Button type="submit" disabled={pending}>
              {pending ? <SpinnerLabel>{t("editor.creating")}</SpinnerLabel> : t("editor.createUser")}
            </Button>
          </DialogFooter>
        </motion.form>
      </DialogContent>
    </Dialog>
  );
}

type EditUserSheetProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  pending: boolean;
  editDialogTarget: UserDTO | null;
  editPayload: EditUserPayload;
  setEditPayload: React.Dispatch<React.SetStateAction<EditUserPayload>>;
  billingMode: AdminBillingMode;
  billingPlans: AdminBillingPlanDTO[];
  statusChanged: boolean;
  timeZoneOptions: string[];
  roleOptions: AdminUserRole[];
  onSaveEdit: () => void;
  onOpenEditAvatarDialog: () => void;
  onOpenResetPasswordDialog: () => void;
  onOpenResetTwoFactorDialog: () => void;
  onOpenRevokeDialog: () => void;
  onOpenDeleteDialog: () => void;
  resetPasswordPending: boolean;
  resetTwoFactorPending: boolean;
  revokePending: boolean;
  deletePending: boolean;
  resolveUserInitial: (user: UserDTO) => string;
};

function SheetSection({
  title,
  children,
  divided = true,
}: {
  title: string;
  children: React.ReactNode;
  divided?: boolean;
}) {
  return (
    <section className={cn("space-y-3", divided && "border-t pt-5")}>
      <h3 className="text-xs font-medium text-foreground">{title}</h3>
      {children}
    </section>
  );
}

function ReadOnlyField({
  label,
  value,
  mono,
}: {
  label: string;
  value: React.ReactNode;
  mono?: boolean;
}) {
  return (
    <div className="space-y-1">
      <Label className="text-xs font-normal text-muted-foreground">{label}</Label>
      <div
        className={cn(
          "flex min-h-8 items-center rounded-md bg-muted/35 px-2.5 py-1.5 text-xs text-foreground",
          mono && "font-mono break-all",
        )}
      >
        {value}
      </div>
    </div>
  );
}

export function EditUserSheet({
  open,
  onOpenChange,
  pending,
  editDialogTarget,
  editPayload,
  setEditPayload,
  billingMode,
  billingPlans,
  statusChanged,
  timeZoneOptions,
  roleOptions,
  onSaveEdit,
  onOpenEditAvatarDialog,
  onOpenResetPasswordDialog,
  onOpenResetTwoFactorDialog,
  onOpenRevokeDialog,
  onOpenDeleteDialog,
  resetPasswordPending,
  resetTwoFactorPending,
  revokePending,
  deletePending,
  resolveUserInitial,
}: EditUserSheetProps) {
  const t = useTranslations("adminUsers");
  const locale = useLocale();
  const editSheetContentRef = React.useRef<HTMLDivElement | null>(null);
  const resolveUserStatusLabel = React.useCallback(
    (value: string | null | undefined) => {
      switch (value?.trim()) {
        case "pending_activation":
          return t("status.pendingActivation");
        case "active":
          return t("status.active");
        case "locked":
          return t("status.locked");
        case "suspended":
          return t("status.suspended");
        case "deactivated":
          return t("status.deactivated");
        default:
          return value?.trim() || "-";
      }
    },
    [t],
  );
  const resolveBillingAccountStatusLabel = React.useCallback(
    (value: string | null | undefined) => {
      switch (value?.trim()) {
        case "active":
          return t("billingAccountStatus.active");
        case "frozen":
          return t("billingAccountStatus.frozen");
        case "closed":
          return t("billingAccountStatus.closed");
        case "suspended":
          return t("billingAccountStatus.suspended");
        default:
          return value?.trim() || "-";
      }
    },
    [t],
  );

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent ref={editSheetContentRef} side="right" className="flex flex-col gap-0 sm:max-w-[520px]">
        <SheetHeader className="px-4 pb-4">
          <SheetTitle>{t("editor.manageTitle")}</SheetTitle>
        </SheetHeader>

        <div className="min-h-0 flex-1 space-y-6 overflow-y-auto px-4 pb-4">
          <div className="flex items-start gap-4">
            <Button
              type="button"
              variant="ghost"
              className="size-10 rounded-full p-0 transition-opacity hover:opacity-85"
              disabled={pending || !editDialogTarget}
              onClick={onOpenEditAvatarDialog}
            >
              <Avatar className="size-10 rounded-full">
                <AvatarImage src={resolveAvatarImageSrc(editPayload.avatarURL, editDialogTarget ?? undefined)} alt={editDialogTarget?.username || t("avatar.userAvatar")} />
                <AvatarFallback className="bg-foreground text-lg font-medium text-background">
                  {editDialogTarget ? resolveUserInitial(editDialogTarget) : "U"}
                </AvatarFallback>
              </Avatar>  
            </Button>
            <div className="min-w-0 text-xs truncate">
              <p className="flex items-center gap-2 font-semibold">
                {resolveDetailValue(editPayload.displayName)}
                <span className="truncate text-xs font-normal text-muted-foreground">@{resolveDetailValue(editDialogTarget?.username)}</span>
              </p>

              <div className="flex items-center gap-2 mt-1">
                <Badge variant="outline" className="text-muted-foreground">ID: {resolveDetailValue(editDialogTarget?.id)}</Badge>
                <Badge variant="outline" className="text-muted-foreground">{resolveDetailValue(editDialogTarget?.role)}</Badge>
                <Badge variant="outline" className="text-muted-foreground">{resolveUserStatusLabel(editDialogTarget?.status)}</Badge>
                {billingMode === "period" ? (
                  <Badge variant="outline" className="text-muted-foreground">{resolveDetailValue(editDialogTarget?.subscriptionTier)}</Badge>
                ) : null}
                {billingMode === "usage" ? (
                  <Badge variant="outline" className="text-muted-foreground">
                    {formatBillingBalance(editDialogTarget?.billingBalanceUSD)}
                  </Badge>
                ) : null}
              </div>
            </div>
          </div>

          <SheetSection title={t("editor.profileSection")} divided={false}>
            <div className="grid gap-3 md:grid-cols-2">
              <ReadOnlyField label={t("editor.username")} value={resolveDetailValue(editDialogTarget?.username)} />
              <div className="space-y-1">
                <Label className="text-xs font-normal text-muted-foreground">{t("editor.displayName")}</Label>
                <Input
                  value={editPayload.displayName}
                  placeholder={t("editor.userDisplayNamePlaceholder")}
                  onChange={(event) => setEditPayload((current) => ({ ...current, displayName: event.target.value }))}
                  disabled={pending}
                  maxLength={DISPLAY_NAME_MAX_LENGTH}
                />
              </div>
              <div className="space-y-1">
                <Label className="text-xs font-normal text-muted-foreground">{t("editor.email")}</Label>
                <Input
                  value={editPayload.email}
                  placeholder={t("editor.userEmailPlaceholder")}
                  onChange={(event) => setEditPayload((current) => ({ ...current, email: event.target.value }))}
                  disabled={pending}
                />
              </div>
              <div className="space-y-1">
                <Label className="text-xs font-normal text-muted-foreground">{t("editor.phone")}</Label>
                <Input
                  value={editPayload.phone}
                  placeholder={t("editor.phonePlaceholder")}
                  onChange={(event) => setEditPayload((current) => ({ ...current, phone: event.target.value }))}
                  disabled={pending}
                />
              </div>
            </div>
            <div className="space-y-1">
                <Label className="text-xs font-normal text-muted-foreground">{t("editor.preferences")}</Label>
                <Textarea
                  value={editPayload.profilePreferences}
                  onChange={(event) =>
                    setEditPayload((current) => ({ ...current, profilePreferences: event.target.value }))
                  }
                  disabled={pending}
                  placeholder={t("editor.preferencesPlaceholder")}
                  className="h-24 resize-none overflow-y-auto [field-sizing:fixed]"
                />
              </div>
          </SheetSection>

          <SheetSection title={t("editor.accessSection")}>
            <div className="grid gap-3 md:grid-cols-2">
              <div className="space-y-1">
                <Label className="text-xs font-normal text-muted-foreground">{t("fields.status")}</Label>
                <Combobox
                  items={USER_STATUS_OPTIONS}
                  value={editPayload.status}
                  itemToStringLabel={resolveUserStatusLabel}
                  onValueChange={(value) => setEditPayload((current) => ({ ...current, status: value as AdminUserStatus }))}
                  disabled={pending}
                >
                  <ComboboxInput className="w-full" placeholder={t("table.selectStatus")} showClear={false} disabled={pending} />
                  <ComboboxContent portalContainer={editSheetContentRef}>
                    <ComboboxEmpty>{t("table.noMatchingStatus")}</ComboboxEmpty>
                    <ComboboxList>
                      {(status: AdminUserStatus) => (
                        <ComboboxItem key={status} value={status}>
                          {resolveUserStatusLabel(status)}
                        </ComboboxItem>
                      )}
                    </ComboboxList>
                  </ComboboxContent>
                </Combobox>
              </div>
              <div className="space-y-1">
                <Label className="text-xs font-normal text-muted-foreground">{t("fields.role")}</Label>
                <Combobox
                  items={roleOptions}
                  value={editPayload.role}
                  onValueChange={(value) => setEditPayload((current) => ({ ...current, role: value as AdminUserRole }))}
                  disabled={pending}
                >
                  <ComboboxInput className="w-full" placeholder={t("table.selectRole")} showClear={false} disabled={pending} />
                  <ComboboxContent portalContainer={editSheetContentRef}>
                    <ComboboxEmpty>{t("table.noMatchingRoles")}</ComboboxEmpty>
                    <ComboboxList>
                      {(role: AdminUserRole) => (
                        <ComboboxItem key={role} value={role}>
                          {role}
                        </ComboboxItem>
                      )}
                    </ComboboxList>
                  </ComboboxContent>
                </Combobox>
              </div>
              <div className="space-y-1">
                <Label className="text-xs font-normal text-muted-foreground">{t("fields.timezone")}</Label>
                <TimeZoneSelect
                  value={editPayload.timezone}
                  options={timeZoneOptions}
                  disabled={pending}
                  portalContainer={editSheetContentRef}
                  onChange={(value) => setEditPayload((current) => ({ ...current, timezone: value }))}
                />
              </div>
              <div className="space-y-1">
                <Label className="text-xs font-normal text-muted-foreground">{t("editor.language")}</Label>
                <Input
                  value={editPayload.locale}
                  onChange={(event) => setEditPayload((current) => ({ ...current, locale: event.target.value }))}
                  disabled={pending}
                />
              </div>
              {statusChanged ? (
                <div className="space-y-1 md:col-span-2">
                  <Label className="text-xs font-normal text-muted-foreground">{t("editor.reason")}</Label>
                  <Input
                    value={editPayload.reason}
                    placeholder={t("editor.reasonPlaceholder")}
                    onChange={(event) => setEditPayload((current) => ({ ...current, reason: event.target.value }))}
                    disabled={pending}
                  />
                </div>
              ) : null}
            </div>
          </SheetSection>

          {billingMode !== "self" ? (
            <SheetSection title={t("editor.billingSection")}>
              {billingMode === "period" ? (
                <div className="grid gap-3 md:grid-cols-2">
                  <div className="space-y-1">
                    <Label className="text-xs font-normal text-muted-foreground">{t("editor.subscriptionPlan")}</Label>
                    <Combobox
                      items={billingPlans.map((plan) => plan.code)}
                      value={editPayload.subscriptionTier}
                      filter={null}
                      autoComplete="none"
                      onValueChange={(value) =>
                        setEditPayload((current) => ({
                          ...current,
                          subscriptionTier: value as UserTier,
                          subscriptionExpiresAt: value === "free" ? "" : current.subscriptionExpiresAt,
                        }))
                      }
                      disabled={pending || billingPlans.length === 0}
                    >
                      <ComboboxInput className="w-full min-w-0" placeholder={t("editor.selectSubscriptionPlan")} showClear={false} disabled={pending || billingPlans.length === 0} />
                      <ComboboxContent portalContainer={editSheetContentRef}>
                        <ComboboxEmpty>{t("editor.noMatchingSubscriptionPlans")}</ComboboxEmpty>
                        <ComboboxList>
                          {(tier: UserTier) => (
                            <ComboboxItem key={tier} value={tier}>
                              {billingPlans.find((plan) => plan.code === tier)?.name ?? tier}
                            </ComboboxItem>
                          )}
                        </ComboboxList>
                      </ComboboxContent>
                    </Combobox>
                  </div>
                  <DialogCollapsible open={editPayload.subscriptionTier !== "free"}>
                    <AdminDateTimePicker
                      value={editPayload.subscriptionExpiresAt}
                      label={t("editor.expiryTime")}
                      placeholder={t("editor.selectExpiryDate")}
                      granularity="date"
                      disabled={pending || editPayload.subscriptionTier === "free"}
                      disabledDate={{ before: new Date() }}
                      onChange={(value) =>
                        setEditPayload((current) => ({
                          ...current,
                          subscriptionExpiresAt: value,
                        }))
                      }
                    />
                  </DialogCollapsible>
                </div>
              ) : null}
              {billingMode === "usage" ? (
                <div className="grid gap-3 md:grid-cols-2">
                  <div className="space-y-1">
                    <Label className="text-xs font-normal text-muted-foreground">{t("editor.accountBalance")}</Label>
                    <Input
                      type="number"
                      min="0"
                      step="0.000001"
                      value={editPayload.billingBalanceUSD}
                      onChange={(event) => setEditPayload((current) => ({ ...current, billingBalanceUSD: event.target.value }))}
                      disabled={pending}
                    />
                  </div>
                  <ReadOnlyField label={t("editor.billingStatus")} value={resolveBillingAccountStatusLabel(editDialogTarget?.billingAccountStatus || "active")} />
                </div>
              ) : null}
            </SheetSection>
          ) : null}

          <SheetSection title={t("editor.securitySection")}>
            <div className="grid gap-3 md:grid-cols-2">
              <ReadOnlyField
                label={t("editor.twoFactor")}
                value={editDialogTarget?.twoFactorEnabled ? t("editor.twoFactorEnabled", { count: editDialogTarget.twoFactorRecoveryCount }) : t("editor.twoFactorDisabled")}
              />
              <ReadOnlyField label={t("fields.lastLogin")} value={formatSheetDateTime(editDialogTarget?.lastLoginAt, locale)} />
            </div>
          </SheetSection>

          <SheetSection title={t("editor.systemSection")}>
            <div className="grid gap-3">
              <ReadOnlyField label="Public ID" value={resolveDetailValue(editDialogTarget?.publicID)} mono />
              <ReadOnlyField label={t("editor.createdAt")} value={formatSheetDateTime(editDialogTarget?.createdAt, locale)} />
              <ReadOnlyField label={t("editor.updatedAt")} value={formatSheetDateTime(editDialogTarget?.updatedAt, locale)} />
            </div>
          </SheetSection>
        </div>

        <SheetFooter className="flex flex-row items-center justify-between gap-2 px-4 py-3">
          <DropdownMenu modal={false}>
            <DropdownMenuTrigger asChild>
              <Button type="button" variant="ghost" className="shrink-0" disabled={pending || resetPasswordPending || resetTwoFactorPending || revokePending || deletePending}>
                {t("editor.moreActions")}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" side="top" className="w-40">
              <DropdownMenuItem onSelect={onOpenResetPasswordDialog} disabled={pending || resetPasswordPending || resetTwoFactorPending || revokePending || deletePending}>
                {resetPasswordPending ? t("confirm.resetting") : t("editor.resetPassword")}
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={onOpenResetTwoFactorDialog} disabled={pending || resetPasswordPending || resetTwoFactorPending || revokePending || deletePending || !editDialogTarget?.twoFactorEnabled}>
                {resetTwoFactorPending ? t("confirm.resetting") : t("editor.reset2fa")}
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={onOpenRevokeDialog} disabled={pending || resetPasswordPending || resetTwoFactorPending || revokePending || deletePending}>
                {revokePending ? t("confirm.revoking") : t("editor.revokeSessions")}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                variant="destructive"
                onSelect={onOpenDeleteDialog}
                disabled={pending || resetPasswordPending || resetTwoFactorPending || revokePending || deletePending}
              >
                {deletePending ? t("confirm.deleting") : t("delete")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <div className="flex shrink-0 items-center gap-2">
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)} disabled={pending || resetPasswordPending || resetTwoFactorPending || revokePending || deletePending}>
              {t("actions.close")}
            </Button>
            <Button type="button" onClick={onSaveEdit} disabled={pending || resetPasswordPending || resetTwoFactorPending || revokePending || deletePending}>
              {pending ? <SpinnerLabel>{t("actions.saving")}</SpinnerLabel> : t("actions.save")}
            </Button>
          </div>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
