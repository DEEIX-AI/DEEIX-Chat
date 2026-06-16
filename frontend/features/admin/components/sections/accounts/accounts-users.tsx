"use client";

import * as React from "react";
import dynamic from "next/dynamic";
import { Database, DollarSign, Globe, Plus, Settings, ShieldCheck, ShieldX, Trash2, Upload, UserCheck } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { toast } from "sonner";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from "@/components/ui/combobox";
import { Spinner } from "@/components/ui/spinner";
import {
  Table,
  TableBody,
  TableCell,
  TableEmptyRow,
  TableHead,
  TableHeader,
  TableLoadingRow,
  TableRow,
} from "@/components/ui/table";
import { useVirtualTableRows, VirtualTablePaddingRow } from "@/components/ui/virtual-table";
import { importOpenWebUIUsers } from "@/features/admin/api";
import type { ImportOpenWebUIUsersData, ImportOpenWebUIUsersRequest } from "@/features/admin/api/admin.types";
import { resolveAvatarImageSrc } from "@/shared/lib/avatar";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { TimeZoneSelect } from "@/shared/components/time-zone-select";
import type { AdminUserRole, AdminUserStatus } from "@/features/admin/api/admin.types";
import type { AdminBillingMode } from "@/features/admin/api/billing.types";
import type { UserDTO } from "@/shared/api/auth.types";

import { AccountAvatarEditorDialog } from "./accounts-avatar-dialog";
import { AccountConfirmationDialog } from "./accounts-confirm-dialog";
import { AccountOpenWebUIImportDialog } from "./accounts-import-dialog";
import { AccountPasswordResetDialog } from "./accounts-password-dialog";
import { TablePagination, TableToolbar } from "@/components/ui/table-tools";
import { AdminBulkConfirmDialog } from "@/features/admin/components/bulk-confirm-dialog";
import { useAdminUsersPage } from "@/features/admin/hooks/use-admin-users-page";
import {
  COMPACT_COMBOBOX_CLASSNAME,
  DEFAULT_CREATE_USER_PAYLOAD,
  USER_ROLE_OPTIONS,
  USER_SORT_OPTIONS,
  USER_STATUS_OPTIONS,
  USER_TIER_OPTIONS,
  type UserSortValue,
} from "@/features/admin/types/accounts";
import {
  formatBillingBalance,
  formatDateTime,
  resolveCreateUserInitial,
  resolveUserInitial,
  resolveValue,
} from "@/features/admin/utils/account-display";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";

const CreateUserDialog = dynamic(
  () => import("./accounts-user-editor").then((module) => module.CreateUserDialog),
  {
    ssr: false,
  },
);

const EditUserSheet = dynamic(
  () => import("./accounts-user-editor").then((module) => module.EditUserSheet),
  {
    ssr: false,
  },
);

type AccountBulkAction = "role" | "status" | "timezone" | "balance" | "delete";

function useUserStatusLabel() {
  const t = useTranslations("adminUsers.status");
  return React.useCallback(
    (value: string | null | undefined) => {
      switch (value?.trim()) {
        case "pending_activation":
          return t("pendingActivation");
        case "active":
          return t("active");
        case "locked":
          return t("locked");
        case "suspended":
          return t("suspended");
        case "deactivated":
          return t("deactivated");
        default:
          return value?.trim() || "-";
      }
    },
    [t],
  );
}

function useSubscriptionStatusLabel() {
  const t = useTranslations("adminUsers.subscriptionStatus");
  return React.useCallback(
    (value: string | null | undefined) => {
      switch (value?.trim()) {
        case "active":
          return t("active");
        case "trialing":
          return t("trialing");
        case "past_due":
          return t("pastDue");
        case "canceled":
          return t("canceled");
        case "unpaid":
          return t("unpaid");
        case "incomplete":
          return t("incomplete");
        case "incomplete_expired":
          return t("incompleteExpired");
        case "paused":
          return t("paused");
        default:
          return value?.trim() || "-";
      }
    },
    [t],
  );
}

function useBillingAccountStatusLabel() {
  const t = useTranslations("adminUsers.billingAccountStatus");
  return React.useCallback(
    (value: string | null | undefined) => {
      switch (value?.trim()) {
        case "active":
          return t("active");
        case "frozen":
          return t("frozen");
        case "closed":
          return t("closed");
        case "suspended":
          return t("suspended");
        default:
          return value?.trim() || "-";
      }
    },
    [t],
  );
}

function BulkActionControlRow({
  icon,
  label,
  disabled,
  onApply,
  children,
}: {
  icon: React.ReactNode;
  label: string;
  disabled: boolean;
  onApply: () => void;
  children: React.ReactNode;
}) {
  return (
    <div className="flex h-7 w-full items-center gap-1.5">
      <Button
        type="button"
        variant="ghost"
        className="h-7 w-16 shrink-0 justify-start gap-2 px-2 text-[11px] text-foreground/70 shadow-none hover:bg-muted hover:text-foreground"
        onClick={onApply}
        disabled={disabled}
      >
        {icon}
        {label}
      </Button>
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}

type AccountsUsersProps = {
  items: UserDTO[];
  total: number;
  page: number;
  pageSize: number;
  loading: boolean;
  onLoadUsers: (page: number, pageSize?: number) => Promise<void>;
  onSetUsers: React.Dispatch<React.SetStateAction<UserDTO[]>>;
  onSetTotal: React.Dispatch<React.SetStateAction<number>>;
};

type UserTableRowProps = {
  item: UserDTO;
  checked: boolean;
  billingMode: AdminBillingMode;
  inlineRolePending: boolean;
  inlineStatusPending: boolean;
  pendingAction: string;
  actionUserID: number | null;
  roleOptions: AdminUserRole[];
  canManage: boolean;
  onToggleSelectedUser: (userID: number, checked: boolean) => void;
  onInlinePatch: (
    item: UserDTO,
    field: "role" | "status",
    payload: Partial<Pick<UserDTO, "role" | "status">>,
  ) => Promise<void>;
  onOpenAvatar: (user: UserDTO) => void;
  onOpenEdit: (user: UserDTO) => void;
};

const UserTableRow = React.memo(function UserTableRow({
  item,
  checked,
  billingMode,
  inlineRolePending,
  inlineStatusPending,
  pendingAction,
  actionUserID,
  roleOptions,
  canManage,
  onToggleSelectedUser,
  onInlinePatch,
  onOpenAvatar,
  onOpenEdit,
}: UserTableRowProps) {
  const t = useTranslations("adminUsers");
  const locale = useLocale();
  const resolveUserStatusLabel = useUserStatusLabel();
  const resolveSubscriptionStatusLabel = useSubscriptionStatusLabel();
  const resolveBillingAccountStatusLabel = useBillingAccountStatusLabel();
  const avatarAlt = item.displayName.trim() || item.username.trim() || item.publicID.trim() || t("fallbackUser");
  const disabled = Boolean(pendingAction) || !canManage;
  const rowRoleOptions = React.useMemo(
    () => (roleOptions.includes(item.role as AdminUserRole) ? roleOptions : [item.role as AdminUserRole, ...roleOptions]),
    [item.role, roleOptions],
  );

  return (
    <TableRow>
      <TableCell className="w-[44px] py-1.5 whitespace-nowrap text-center">
        <div className="flex h-7 items-center justify-center">
          <Checkbox
            checked={checked}
            onCheckedChange={(value) => onToggleSelectedUser(item.id, value === true)}
            disabled={disabled}
          />
        </div>
      </TableCell>
      <TableCell className="py-1.5 whitespace-nowrap font-mono text-xs text-foreground">
        <span className="flex h-7 items-center">{item.id}</span>
      </TableCell>
      <TableCell className="py-1.5">
        <div className="flex h-7 items-center gap-3">
          <button
            type="button"
            className="rounded-full transition-opacity hover:opacity-85 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            onClick={() => onOpenAvatar(item)}
            disabled={disabled}
            aria-label={t("table.editAvatarAria", { name: avatarAlt })}
          >
            <Avatar className="h-7 w-7 rounded-full">
              <AvatarImage src={resolveAvatarImageSrc(item.avatarURL, item) || undefined} alt={avatarAlt} />
              <AvatarFallback className="rounded-full bg-foreground text-xs font-medium text-background">
                {resolveUserInitial(item)}
              </AvatarFallback>
            </Avatar>
          </button>
          <div className="min-w-0 max-w-[22rem] truncate whitespace-nowrap text-xs">
            <span className="font-medium text-foreground">{resolveValue(item.displayName)}</span>
            <span className="px-1.5 text-muted-foreground">/</span>
            <span className="text-muted-foreground">@{resolveValue(item.username)}</span>
          </div>
        </div>
      </TableCell>
      <TableCell className="py-1.5 whitespace-nowrap">
        <div className="flex h-7 items-center">
          <Combobox
            items={rowRoleOptions}
            value={item.role}
            onValueChange={(value) => void onInlinePatch(item, "role", { role: value as UserDTO["role"] })}
            disabled={disabled || inlineRolePending}
          >
            <ComboboxInput
              className={`w-[90px] ${COMPACT_COMBOBOX_CLASSNAME}`}
              placeholder={t("table.selectRole")}
              showClear={false}
              disabled={disabled || inlineRolePending}
            />
            <ComboboxContent>
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
      </TableCell>
      <TableCell className="py-1.5 whitespace-nowrap">
        <div className="flex h-7 items-center">
          <Combobox
            items={USER_STATUS_OPTIONS}
            value={item.status}
            itemToStringLabel={resolveUserStatusLabel}
            onValueChange={(value) => void onInlinePatch(item, "status", { status: value as UserDTO["status"] })}
            disabled={disabled || inlineStatusPending}
          >
            <ComboboxInput
              className={`w-[80px] ${COMPACT_COMBOBOX_CLASSNAME}`}
              placeholder={t("table.selectStatus")}
              showClear={false}
              disabled={disabled || inlineStatusPending}
            />
            <ComboboxContent>
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
      </TableCell>
      {billingMode === "period" ? (
        <TableCell className="text-foreground">
          <div
            className="max-w-[10rem] truncate"
            title={resolveValue(
              item.subscriptionPlanName.trim() || item.subscriptionTier.trim() || resolveSubscriptionStatusLabel(item.subscriptionStatus),
            )}
          >
            {resolveValue(
              item.subscriptionPlanName.trim() || item.subscriptionTier.trim() || resolveSubscriptionStatusLabel(item.subscriptionStatus),
            )}
          </div>
        </TableCell>
      ) : null}
      {billingMode !== "self" ? (
        <TableCell className="whitespace-nowrap text-foreground">
          <span title={resolveBillingAccountStatusLabel(item.billingAccountStatus || "active")}>
            {formatBillingBalance(item.billingBalanceUSD)}
          </span>
        </TableCell>
      ) : null}
      <TableCell className="text-muted-foreground">
        <div className="flex items-center justify-center">
          {item.twoFactorEnabled ? <ShieldCheck className="size-3.5" /> : <ShieldX className="size-3.5" />}
        </div>
      </TableCell>
      <TableCell className="text-muted-foreground">
        <div className="max-w-[11rem] truncate" title={resolveValue(item.timezone)}>
          {resolveValue(item.timezone)}
        </div>
      </TableCell>
      <TableCell className="whitespace-nowrap text-muted-foreground">{formatDateTime(item.lastLoginAt, locale)}</TableCell>
      <TableCell className="w-[56px] py-1.5 whitespace-nowrap" stickyEnd>
        <div className="flex h-7 items-center justify-end">
          <Button
            type="button"
            size="icon-sm"
            variant="ghost"
            className="text-muted-foreground shadow-none"
            onClick={() => onOpenEdit(item)}
            disabled={disabled}
            aria-label={t("table.editUserAria", { username: item.username })}
            title={t("table.editUser")}
          >
            {pendingAction === "edit" && actionUserID === item.id ? (
              <Spinner className="size-3.5" />
            ) : (
              <Settings className="size-3.5 stroke-1" />
            )}
          </Button>
        </div>
      </TableCell>
    </TableRow>
  );
});

export function AccountsUsers({
  items,
  total,
  page,
  pageSize,
  loading,
  onLoadUsers,
  onSetUsers,
  onSetTotal,
}: AccountsUsersProps) {
  const t = useTranslations("adminUsers");
  const { user: viewer } = useAuthSession();
  const resolveUserStatusLabel = useUserStatusLabel();
  const roleOptions = React.useMemo(
    () => (viewer?.role === "superadmin" ? USER_ROLE_OPTIONS : USER_ROLE_OPTIONS.filter((role) => role !== "superadmin")),
    [viewer?.role],
  );
  const createDialogContentRef = React.useRef<HTMLDivElement | null>(null);
  const {
    timeZoneOptions,
    pendingAction,
    inlinePending,
    actionUserID,
    createDialogOpen,
    setCreateDialogOpen,
    avatarDialog,
    setAvatarDialog,
    editDialogTarget,
    setEditDialogTarget,
    resetDialogTarget,
    setResetDialogTarget,
    revokeDialogTarget,
    setRevokeDialogTarget,
    deleteDialogTarget,
    setDeleteDialogTarget,
    resetTwoFactorDialogTarget,
    setResetTwoFactorDialogTarget,
    query,
    setQuery,
    roleFilter,
    setRoleFilter,
    statusFilter,
    setStatusFilter,
    tierFilter,
    setTierFilter,
    sortValue,
    setSortValue,
    selectedUserIDs,
    batchRole,
    setBatchRole,
    batchStatus,
    setBatchStatus,
    batchTimezone,
    setBatchTimezone,
    batchBalance,
    setBatchBalance,
    createPayload,
    setCreatePayload,
    editPayload,
    setEditPayload,
    resetPasswordDraft,
    setResetPasswordDraft,
    billingMode,
    billingPlans,
    createAvatarSource,
    avatarDialogPreviewSrc,
    editStatusChanged,
    pageCount,
    filteredItems,
    selectAllState,
    resolveInlineKey,
    refreshUsers,
    handleOpenEditDialog,
    handleOpenAvatarDialog,
    handleOpenCreateAvatarDialog,
    handleInlineUserPatch,
    canManageUser,
    onCreateUser,
    handleSaveAvatarDialog,
    handleSaveEditDialog,
    onResetPassword,
    onResetTwoFactor,
    onRevokeSessions,
    onDeleteUser,
    handleSelectAllVisible,
    handleToggleSelectedUser,
    onBulkApplyRole,
    onBulkApplyStatus,
    onBulkDeleteUsers,
    onBulkApplyTimezone,
    onBulkApplyBalance,
    handleRandomizeAvatarDialog,
  } = useAdminUsersPage({ items, total, page, pageSize, viewerRole: viewer?.role, onLoadUsers, onSetUsers, onSetTotal });
  const virtualRows = useVirtualTableRows(filteredItems, {
    enabled: filteredItems.length > 100,
    estimateSize: 40,
  });
  const initialLoading = loading && filteredItems.length === 0;
  const showRows = filteredItems.length > 0;
  const [bulkConfirmAction, setBulkConfirmAction] = React.useState<AccountBulkAction | null>(null);
  const [openWebUIImportOpen, setOpenWebUIImportOpen] = React.useState(false);
  const [openWebUIImportPending, setOpenWebUIImportPending] = React.useState(false);
  const [openWebUIImportResult, setOpenWebUIImportResult] = React.useState<ImportOpenWebUIUsersData | null>(null);
  const bulkConfirmOpen = bulkConfirmAction !== null;
  const hasSelectableFilteredItems = React.useMemo(
    () => filteredItems.some(canManageUser),
    [canManageUser, filteredItems],
  );

  function handleConfirmBulkAction() {
    switch (bulkConfirmAction) {
      case "role":
        void onBulkApplyRole().then(() => setBulkConfirmAction(null));
        break;
      case "status":
        void onBulkApplyStatus().then(() => setBulkConfirmAction(null));
        break;
      case "timezone":
        void onBulkApplyTimezone().then(() => setBulkConfirmAction(null));
        break;
      case "balance":
        void onBulkApplyBalance().then(() => setBulkConfirmAction(null));
        break;
      case "delete":
        void onBulkDeleteUsers().then(() => setBulkConfirmAction(null));
        break;
    }
  }

  async function handleImportOpenWebUI(payload: ImportOpenWebUIUsersRequest) {
    if (openWebUIImportPending) {
      return;
    }
    setOpenWebUIImportPending(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      const result = await importOpenWebUIUsers(token, payload);
      setOpenWebUIImportResult(result);
      if (payload.dryRun) {
        toast.success(t("importOpenWebUI.toastPreviewSucceeded", {
          imported: result.imported,
          skipped: result.skippedExistingEmail + result.skippedDuplicateSourceEmail,
        }));
        return;
      }
      toast.success(t("importOpenWebUI.toastSucceeded", {
        imported: result.imported,
        skipped: result.skippedExistingEmail + result.skippedDuplicateSourceEmail,
      }));
      setOpenWebUIImportOpen(false);
      await onLoadUsers(1, pageSize);
    } catch (error) {
      toast.error(t("importOpenWebUI.toastFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setOpenWebUIImportPending(false);
    }
  }

  const showBalanceColumn = billingMode !== "self";
  const tableColSpan = 9 + (billingMode === "period" ? 2 : showBalanceColumn ? 1 : 0);

  return (
    <>
      <div className="space-y-3">
        <div className="flex h-10 items-center px-1">
          <h3 className="text-sm font-semibold">{t("pageTitle")}</h3>
        </div>
        
        <TableToolbar
          query={query}
          onQueryChange={setQuery}
          queryPlaceholder={t("table.searchPlaceholder")}
          filters={[
            {
              key: "role",
              label: t("fields.role"),
              value: roleFilter,
              onValueChange: setRoleFilter,
              options: [
                { label: t("table.allRoles"), value: "" },
                ...USER_ROLE_OPTIONS.map((item) => ({ label: item, value: item })),
              ],
            },
            {
              key: "status",
              label: t("fields.status"),
              value: statusFilter,
              onValueChange: setStatusFilter,
              options: [
                { label: t("table.allStatus"), value: "" },
                ...USER_STATUS_OPTIONS.map((item) => ({ label: resolveUserStatusLabel(item), value: item })),
              ],
            },
            ...(billingMode === "period"
              ? [
                  {
                    key: "tier",
                    label: t("fields.subscription"),
                    value: tierFilter,
                    onValueChange: setTierFilter,
                    options: [
                      { label: t("table.allSubscriptions"), value: "" },
                      ...billingPlans.map((item) => ({ label: item.name || item.code, value: item.code })),
                      ...USER_TIER_OPTIONS.filter((tier) => !billingPlans.some((plan) => plan.code === tier)).map((item) => ({
                        label: item,
                        value: item,
                      })),
                    ],
                  },
                ]
              : []),
          ]}
          sort={{
            value: sortValue,
            onValueChange: (value) => setSortValue(value as UserSortValue),
            options: USER_SORT_OPTIONS.map((item) => ({ label: t(item.labelKey), value: item.value })),
          }}
          selectedCount={selectedUserIDs.size}
          bulkContent={
            <div className="space-y-1">
              <BulkActionControlRow
                icon={<ShieldCheck className="size-3 stroke-1" />}
                label={t("actions.apply")}
                onApply={() => setBulkConfirmAction("role")}
                disabled={loading || Boolean(pendingAction) || selectedUserIDs.size === 0 || !batchRole}
              >
                <Select
                  value={batchRole || undefined}
                  onValueChange={(value) => setBatchRole(value as AdminUserRole)}
                  disabled={loading || Boolean(pendingAction) || selectedUserIDs.size === 0}
                >
                  <SelectTrigger size="xs" className="h-7 px-2 text-[11px] text-muted-foreground">
                    <SelectValue placeholder={t("fields.role")} />
                  </SelectTrigger>
                  <SelectContent position="popper" align="start" className="z-[100]">
                    {roleOptions.map((role) => (
                      <SelectItem key={role} value={role} className="text-[11px]">
                        {role}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </BulkActionControlRow>

              <BulkActionControlRow
                icon={<UserCheck className="size-3 stroke-1" />}
                label={t("actions.apply")}
                onApply={() => setBulkConfirmAction("status")}
                disabled={loading || Boolean(pendingAction) || selectedUserIDs.size === 0 || !batchStatus}
              >
                <Select
                  value={batchStatus || undefined}
                  onValueChange={(value) => setBatchStatus(value as AdminUserStatus)}
                  disabled={loading || Boolean(pendingAction) || selectedUserIDs.size === 0}
                >
                  <SelectTrigger size="xs" className="h-7 px-2 text-[11px] text-muted-foreground">
                    <SelectValue placeholder={t("fields.status")} />
                  </SelectTrigger>
                  <SelectContent position="popper" align="start" className="z-[100]">
                    {USER_STATUS_OPTIONS.map((status) => (
                      <SelectItem key={status} value={status} className="text-[11px]">
                        {resolveUserStatusLabel(status)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </BulkActionControlRow>

              <BulkActionControlRow
                icon={<Globe className="size-3 stroke-1" />}
                label={t("actions.apply")}
                onApply={() => setBulkConfirmAction("timezone")}
                disabled={loading || Boolean(pendingAction) || selectedUserIDs.size === 0 || !batchTimezone}
              >
                <TimeZoneSelect
                  value={batchTimezone}
                  options={timeZoneOptions}
                  disabled={loading || Boolean(pendingAction) || selectedUserIDs.size === 0}
                  triggerClassName="h-7 min-w-0 px-2 text-[11px]"
                  contentClassName="min-w-[260px]"
                  onChange={setBatchTimezone}
                />
              </BulkActionControlRow>

              {showBalanceColumn ? (
                <BulkActionControlRow
                  icon={<DollarSign className="size-3 stroke-1" />}
                  label={t("actions.apply")}
                  onApply={() => setBulkConfirmAction("balance")}
                  disabled={loading || Boolean(pendingAction) || selectedUserIDs.size === 0 || !batchBalance.trim()}
                >
                  <Input
                    type="number"
                    min="0"
                    step="0.000001"
                    value={batchBalance}
                    placeholder={t("fields.balance")}
                    onChange={(event) => setBatchBalance(event.target.value)}
                    disabled={loading || Boolean(pendingAction) || selectedUserIDs.size === 0}
                    className="h-7 px-2 text-[11px]"
                  />
                </BulkActionControlRow>
              ) : null}
            </div>
          }
          bulkActions={[
            {
              key: "delete",
              label: t("actions.delete"),
              icon: <Trash2 className="size-3.5 stroke-1" />,
              onClick: () => setBulkConfirmAction("delete"),
            },
          ]}
          loading={loading || Boolean(pendingAction) || openWebUIImportPending}
          onRefresh={() => void refreshUsers(page)}
          refreshDisabled={loading || Boolean(pendingAction) || openWebUIImportPending}
          refreshLoading={pendingAction === "refresh"}
        >
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="h-7 gap-1 px-2 text-xs"
                disabled={Boolean(pendingAction) || openWebUIImportPending}
              >
                <Upload className="size-3.5 stroke-1" />
                {t("importOpenWebUI.import")}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem
                onClick={() => {
                  setOpenWebUIImportResult(null);
                  setOpenWebUIImportOpen(true);
                }}
              >
                <Database className="size-3.5 stroke-1" />
                OpenWebUI
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <Button
            type="button"
            size="sm"
            className="h-7 gap-1 px-2 text-xs"
            onClick={() => setCreateDialogOpen(true)}
            disabled={Boolean(pendingAction) || openWebUIImportPending}
          >
            <Plus className="size-3.5 stroke-1" />
            {t("create")}
          </Button>
        </TableToolbar>

        <Table
          viewportRef={virtualRows.viewportRef}
          viewportClassName={virtualRows.viewportClassName}
          viewportStyle={virtualRows.viewportStyle}
        >
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead className="w-[44px] py-1.5 text-center">
                  <div className="flex h-7 items-center justify-center">
                    <Checkbox
                      checked={selectAllState}
                      onCheckedChange={(checked) => handleSelectAllVisible(checked === true)}
                      disabled={loading || !hasSelectableFilteredItems}
                    />
                  </div>
                </TableHead>
                <TableHead>ID</TableHead>
                <TableHead>{t("fields.info")}</TableHead>
                <TableHead>{t("fields.role")}</TableHead>
                <TableHead>{t("fields.status")}</TableHead>
                {billingMode === "period" ? <TableHead>{t("fields.subscription")}</TableHead> : null}
                {showBalanceColumn ? <TableHead>{t("fields.balance")}</TableHead> : null}
                <TableHead className="text-center">2FA</TableHead>
                <TableHead>{t("fields.timezone")}</TableHead>
                <TableHead>{t("fields.lastLogin")}</TableHead>
                <TableHead className="w-[56px]" stickyEnd />
              </TableRow>
            </TableHeader>
            <TableBody>
              {initialLoading ? (
                <TableLoadingRow colSpan={tableColSpan} />
              ) : null}
              {showRows ? <VirtualTablePaddingRow colSpan={tableColSpan} height={virtualRows.paddingTop} /> : null}
              {showRows
                ? virtualRows.rows.map(({ item }) => (
                    <UserTableRow
                      key={item.id}
                      item={item}
                      checked={selectedUserIDs.has(item.id)}
                      billingMode={billingMode}
                      inlineRolePending={Boolean(inlinePending[resolveInlineKey(item.id, "role")])}
                      inlineStatusPending={Boolean(inlinePending[resolveInlineKey(item.id, "status")])}
                      pendingAction={pendingAction}
                      actionUserID={actionUserID}
                      roleOptions={roleOptions}
                      canManage={canManageUser(item)}
                      onToggleSelectedUser={handleToggleSelectedUser}
                      onInlinePatch={handleInlineUserPatch}
                      onOpenAvatar={handleOpenAvatarDialog}
                      onOpenEdit={handleOpenEditDialog}
                    />
                  ))
                : null}
              {showRows ? <VirtualTablePaddingRow colSpan={tableColSpan} height={virtualRows.paddingBottom} /> : null}
              {!loading && filteredItems.length === 0 ? (
                <TableEmptyRow colSpan={tableColSpan}>{t("table.empty")}</TableEmptyRow>
              ) : null}
            </TableBody>
        </Table>

        <TablePagination
          total={filteredItems.length}
          page={page}
          pageCount={pageCount}
          pageSize={pageSize}
          onPageChange={(nextPage) => void onLoadUsers(nextPage, pageSize)}
          onPageSizeChange={(nextPageSize) => void onLoadUsers(1, nextPageSize)}
          loading={loading || Boolean(pendingAction) || openWebUIImportPending}
        />
      </div>

      <AccountAvatarEditorDialog
        open={avatarDialog.mode !== "closed"}
        onOpenChange={(open) => {
          if (avatarDialog.mode === "edit") {
            if (!open && pendingAction !== "avatar") {
              setAvatarDialog({ mode: "closed" });
            }
            return;
          }

          if (!open && avatarDialog.mode === "create") {
            setAvatarDialog({ mode: "closed" });
          }
        }}
        title={avatarDialog.mode === "edit" ? t("avatar.editTitle") : t("avatar.createTitle")}
        description={t("avatar.description")}
        previewSrc={avatarDialogPreviewSrc}
        alt={avatarDialog.mode === "edit" ? avatarDialog.target.displayName || avatarDialog.target.username || t("avatar.userAvatar") : t("avatar.preview")}
        fallback={
          avatarDialog.mode === "edit"
            ? resolveUserInitial(avatarDialog.target)
            : resolveCreateUserInitial(createPayload.username, createPayload.displayName)
        }
        value={avatarDialog.mode === "closed" ? "" : avatarDialog.value}
        onValueChange={(value) => {
          if (avatarDialog.mode === "closed") {
            return;
          }
          setAvatarDialog((current) => (current.mode === "closed" ? current : { ...current, value }));
        }}
        onRandomize={handleRandomizeAvatarDialog}
        onApply={() => {
          if (avatarDialog.mode === "edit") {
            void handleSaveAvatarDialog();
            return;
          }
          setCreatePayload((current) => ({
            ...current,
            avatarURL: avatarDialog.mode === "create" ? avatarDialog.value.trim() : current.avatarURL,
          }));
          setAvatarDialog({ mode: "closed" });
        }}
        applyLabel={t("avatar.apply")}
        pending={avatarDialog.mode === "edit" ? pendingAction === "avatar" : pendingAction === "create"}
      />

      {createDialogOpen ? (
        <CreateUserDialog
          open
          onOpenChange={(open) => {
            setCreateDialogOpen(open);
            if (!open && pendingAction !== "create") {
              setCreatePayload(DEFAULT_CREATE_USER_PAYLOAD);
              if (avatarDialog.mode === "create") {
                setAvatarDialog({ mode: "closed" });
              }
            }
          }}
          pending={pendingAction === "create"}
          createDialogContentRef={createDialogContentRef}
          createPayload={createPayload}
          setCreatePayload={setCreatePayload}
          billingMode={billingMode}
          billingPlans={billingPlans}
          createAvatarSource={createAvatarSource}
          onOpenCreateAvatarDialog={handleOpenCreateAvatarDialog}
          onCreateSubmit={onCreateUser}
          resolveCreateUserInitial={resolveCreateUserInitial}
        />
      ) : null}

      <AccountOpenWebUIImportDialog
        open={openWebUIImportOpen}
        pending={openWebUIImportPending}
        result={openWebUIImportResult}
        onOpenChange={(open) => {
          if (!open && openWebUIImportPending) {
            return;
          }
          setOpenWebUIImportOpen(open);
          if (open) {
            setOpenWebUIImportResult(null);
          }
        }}
        onPreviewReset={() => setOpenWebUIImportResult(null)}
        onSubmit={handleImportOpenWebUI}
      />

      {editDialogTarget ? (
        <EditUserSheet
          open
          onOpenChange={(open) => {
            if (!open && pendingAction !== "edit") {
              setEditDialogTarget(null);
            }
          }}
          pending={pendingAction === "edit"}
          editDialogTarget={editDialogTarget}
          editPayload={editPayload}
          setEditPayload={setEditPayload}
          billingMode={billingMode}
          billingPlans={billingPlans}
          statusChanged={editStatusChanged}
          timeZoneOptions={timeZoneOptions}
          roleOptions={roleOptions}
          onSaveEdit={() => void handleSaveEditDialog()}
          onOpenEditAvatarDialog={() => {
            setAvatarDialog({
              mode: "edit",
              target: editDialogTarget,
              value: editPayload.avatarURL.trim() || editDialogTarget.avatarURL.trim(),
            });
          }}
          onOpenResetPasswordDialog={() => {
            setResetDialogTarget(editDialogTarget);
            setResetPasswordDraft("");
          }}
          onOpenResetTwoFactorDialog={() => {
            setResetTwoFactorDialogTarget(editDialogTarget);
          }}
          onOpenRevokeDialog={() => {
            setRevokeDialogTarget(editDialogTarget);
          }}
          onOpenDeleteDialog={() => {
            setDeleteDialogTarget(editDialogTarget);
          }}
          resetPasswordPending={pendingAction === "reset-password" && actionUserID === editDialogTarget.id}
          resetTwoFactorPending={pendingAction === "reset-2fa" && actionUserID === editDialogTarget.id}
          revokePending={pendingAction === "revoke-sessions" && actionUserID === editDialogTarget.id}
          deletePending={pendingAction === "delete" && actionUserID === editDialogTarget.id}
          resolveUserInitial={resolveUserInitial}
        />
      ) : null}

      <AccountPasswordResetDialog
        open={Boolean(resetDialogTarget)}
        onOpenChange={(open) => {
          if (!open && pendingAction !== "reset-password") {
            setResetDialogTarget(null);
            setResetPasswordDraft("");
          }
        }}
        pending={pendingAction === "reset-password"}
        password={resetPasswordDraft}
        onPasswordChange={setResetPasswordDraft}
        onConfirm={() => void onResetPassword()}
        onCancel={() => {
          setResetDialogTarget(null);
          setResetPasswordDraft("");
        }}
      />

      <AccountConfirmationDialog
        open={Boolean(resetTwoFactorDialogTarget)}
        onOpenChange={(open) => {
          if (!open && pendingAction !== "reset-2fa") {
            setResetTwoFactorDialogTarget(null);
          }
        }}
        pending={pendingAction === "reset-2fa"}
        title={t("confirm.reset2faTitle")}
        description={t("confirm.reset2faDescription")}
        confirmLabel={t("confirm.reset")}
        pendingLabel={t("confirm.resetting")}
        onConfirm={() => {
          if (resetTwoFactorDialogTarget) {
            void onResetTwoFactor(resetTwoFactorDialogTarget);
          }
        }}
      />

      <AccountConfirmationDialog
        open={Boolean(revokeDialogTarget)}
        onOpenChange={(open) => {
          if (!open && pendingAction !== "revoke-sessions") {
            setRevokeDialogTarget(null);
          }
        }}
        pending={pendingAction === "revoke-sessions"}
        title={t("confirm.revokeSessionsTitle")}
        description={t("confirm.revokeSessionsDescription")}
        confirmLabel={t("confirm.revoke")}
        pendingLabel={t("confirm.revoking")}
        onConfirm={() => {
          if (revokeDialogTarget) {
            void onRevokeSessions(revokeDialogTarget.id).then(() => setRevokeDialogTarget(null));
          }
        }}
      />

      <AccountConfirmationDialog
        open={Boolean(deleteDialogTarget)}
        onOpenChange={(open) => {
          if (!open && pendingAction !== "delete") {
            setDeleteDialogTarget(null);
          }
        }}
        pending={pendingAction === "delete"}
        title={t("confirm.deleteTitle")}
        description={t("confirm.deleteDescription")}
        confirmLabel={t("delete")}
        pendingLabel={t("confirm.deleting")}
        onConfirm={() => {
          if (deleteDialogTarget) {
            void onDeleteUser(deleteDialogTarget);
          }
        }}
      />

      <AdminBulkConfirmDialog
        open={bulkConfirmOpen}
        onOpenChange={(open) => {
          if (!open && !pendingAction) {
            setBulkConfirmAction(null);
          }
        }}
        pending={Boolean(pendingAction)}
        title={t("bulkConfirm.title")}
        description={t("bulkConfirm.description", { count: selectedUserIDs.size })}
        confirmLabel={t("bulkConfirm.confirm")}
        pendingLabel={t("bulkConfirm.pending")}
        onConfirm={handleConfirmBulkAction}
      />
    </>
  );
}
