import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  createGeneratedGithubAvatarRef,
  generateAvatarVariant,
  resolveAvatarImageSrc,
} from "@/shared/lib/avatar";
import {
  createAdminUser,
  deleteAdminUser,
  getAdminReferenceData,
  patchAdminUser,
  resetAdminUserPassword,
  resetAdminUserTwoFactor,
  revokeAdminUserSessions,
  updateAdminBillingAccountBalance,
} from "@/features/admin/api";
import type { AdminUserRole, AdminUserStatus } from "@/features/admin/api/admin.types";
import type { AdminBillingMode, AdminBillingPlanDTO } from "@/features/admin/api/billing.types";
import type { UserDTO } from "@/shared/api/auth.types";
import {
  isDisplayNameLengthValid,
  isPasswordPolicyValid,
  isUsernamePolicyValid,
} from "@/shared/auth/account-policy";
import {
  DEFAULT_CREATE_USER_PAYLOAD,
  type AvatarDialogState,
  type CreateUserPayload,
  type EditUserPayload,
  type InlineEditableField,
  type PendingAction,
  type UserSortValue,
} from "@/features/admin/types/accounts";
import {
  resolveSubscriptionExpiryInputValue,
  resolveSubscriptionExpiryDate,
  resolveSubscriptionExpiryISO,
} from "@/features/admin/utils/account-display";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { patchByID, removeByID, removeManyByID, replaceByID, restoreAt, restoreManyAt } from "@/shared/lib/optimistic-list";
import { runBulkActionInChunks, runSettledBulkItems } from "@/shared/lib/bulk-action";
import { resolveTimeZoneOptions } from "@/shared/lib/time-zone";
import { useAdminUserFilters } from "./use-admin-user-filters";
import { useAdminUserSelection } from "./use-admin-user-selection";

type UseAdminUsersPageParams = {
  items: UserDTO[];
  total: number;
  page: number;
  pageSize: number;
  query: string;
  setQuery: (value: string) => void;
  viewerRole?: string;
  onLoadUsers: () => Promise<void>;
  onSetPage: (value: number) => void;
  onSetUsers: React.Dispatch<React.SetStateAction<UserDTO[]>>;
  onSetTotal: React.Dispatch<React.SetStateAction<number>>;
};

type UseAdminUsersPageState = {
  timeZoneOptions: string[];
  pendingAction: PendingAction;
  inlinePending: Record<string, boolean>;
  actionUserID: number | null;
  createDialogOpen: boolean;
  setCreateDialogOpen: (open: boolean) => void;
  avatarDialog: AvatarDialogState;
  setAvatarDialog: React.Dispatch<React.SetStateAction<AvatarDialogState>>;
  editDialogTarget: UserDTO | null;
  setEditDialogTarget: (target: UserDTO | null) => void;
  resetDialogTarget: UserDTO | null;
  setResetDialogTarget: (target: UserDTO | null) => void;
  revokeDialogTarget: UserDTO | null;
  setRevokeDialogTarget: (target: UserDTO | null) => void;
  deleteDialogTarget: UserDTO | null;
  setDeleteDialogTarget: (target: UserDTO | null) => void;
  resetTwoFactorDialogTarget: UserDTO | null;
  setResetTwoFactorDialogTarget: (target: UserDTO | null) => void;
  query: string;
  setQuery: (value: string) => void;
  roleFilter: string;
  setRoleFilter: (value: string) => void;
  statusFilter: string;
  setStatusFilter: (value: string) => void;
  tierFilter: string;
  setTierFilter: (value: string) => void;
  sortValue: UserSortValue;
  setSortValue: (value: UserSortValue) => void;
  selectedUserIDs: Set<number>;
  batchRole: AdminUserRole | "";
  setBatchRole: (value: AdminUserRole | "") => void;
  batchStatus: AdminUserStatus | "";
  setBatchStatus: (value: AdminUserStatus | "") => void;
  batchTimezone: string;
  setBatchTimezone: (value: string) => void;
  batchBalance: string;
  setBatchBalance: (value: string) => void;
  createPayload: CreateUserPayload;
  setCreatePayload: React.Dispatch<React.SetStateAction<CreateUserPayload>>;
  editPayload: EditUserPayload;
  setEditPayload: React.Dispatch<React.SetStateAction<EditUserPayload>>;
  resetPasswordDraft: string;
  setResetPasswordDraft: (value: string) => void;
  billingMode: AdminBillingMode;
  billingPlans: AdminBillingPlanDTO[];
  createAvatarSource: Pick<CreateUserPayload, "username" | "displayName">;
  avatarDialogPreviewSrc: string | undefined;
  editStatusChanged: boolean;
  batchTimezoneOptions: { label: string; value: string }[];
  filteredItems: UserDTO[];
  selectAllState: boolean | "indeterminate";
  canManageUser: (user: UserDTO) => boolean;
  resolveInlineKey: (userID: number, field: InlineEditableField) => string;
  refreshUsers: (nextPage?: number) => Promise<void>;
  handleOpenEditDialog: (user: UserDTO) => void;
  handleOpenAvatarDialog: (user: UserDTO) => void;
  handleOpenCreateAvatarDialog: () => void;
  handleInlineUserPatch: (
    item: UserDTO,
    field: InlineEditableField,
    payload: Partial<Pick<UserDTO, "role" | "status">>,
  ) => Promise<void>;
  onCreateUser: (event: React.FormEvent<HTMLFormElement>) => Promise<void>;
  handleSaveAvatarDialog: () => Promise<void>;
  handleSaveEditDialog: () => Promise<void>;
  onResetPassword: () => Promise<void>;
  onResetTwoFactor: (user: UserDTO) => Promise<void>;
  onRevokeSessions: (userID: number) => Promise<void>;
  onDeleteUser: (user: UserDTO) => Promise<void>;
  handleSelectAllVisible: (checked: boolean) => void;
  handleToggleSelectedUser: (userID: number, checked: boolean) => void;
  onBulkApplyRole: () => Promise<void>;
  onBulkApplyStatus: () => Promise<void>;
  onBulkDeleteUsers: () => Promise<void>;
  onBulkApplyTimezone: () => Promise<void>;
  onBulkApplyBalance: () => Promise<void>;
  handleRandomizeAvatarDialog: () => void;
};

type AdminUserPatchPayload = {
  avatarURL?: string;
  displayName?: string;
  email?: string;
  phone?: string;
  role?: AdminUserRole;
  status?: AdminUserStatus;
  timezone?: string;
  locale?: string;
  profilePreferences?: string;
  subscriptionTier?: string;
  subscriptionExpiresAt?: string;
  reason?: string;
};

function createEditPayload(user: UserDTO, fallbackSubscriptionTier = "free"): EditUserPayload {
  const subscriptionTier = user.subscriptionTier.trim() || fallbackSubscriptionTier;
  return {
    avatarURL: user.avatarURL.trim(),
    displayName: user.displayName,
    email: user.email,
    phone: user.phone,
    role: user.role as AdminUserRole,
    status: user.status as AdminUserStatus,
    timezone: user.timezone.trim() || "Etc/UTC",
    locale: user.locale.trim() || "en-US",
    subscriptionTier,
    subscriptionExpiresAt: resolveSubscriptionExpiryInputValue(user.subscriptionExpiresAt),
    billingBalanceUSD: String(user.billingBalanceUSD ?? 0),
    profilePreferences: user.profilePreferences,
    reason: "",
  };
}

function hasPatchChanges(payload: AdminUserPatchPayload): boolean {
  return Object.keys(payload).some((key) => key !== "reason");
}

function roundBillingBalance(value: number): number {
  return Math.round(Math.max(0, value) * 1_000_000) / 1_000_000;
}

function userFromUnknownResponse(response: unknown): UserDTO | null {
  if (!response || typeof response !== "object" || !("user" in response)) {
    return null;
  }
  const user = (response as { user?: unknown }).user;
  return user && typeof user === "object" ? (user as UserDTO) : null;
}

export function useAdminUsersPage({
  items,
  total,
  page,
  pageSize,
  query,
  setQuery,
  viewerRole,
  onLoadUsers,
  onSetPage,
  onSetUsers,
  onSetTotal,
}: UseAdminUsersPageParams): UseAdminUsersPageState {
  const t = useTranslations("adminUsers");
  const timeZoneOptions = React.useMemo(() => resolveTimeZoneOptions(), []);

  const [pendingAction, setPendingAction] = React.useState<PendingAction>("");
  const [inlinePending, setInlinePending] = React.useState<Record<string, boolean>>({});
  const [actionUserID, setActionUserID] = React.useState<number | null>(null);

  const [createDialogOpen, setCreateDialogOpen] = React.useState(false);
  const [avatarDialog, setAvatarDialog] = React.useState<AvatarDialogState>({ mode: "closed" });
  const [editDialogTarget, setEditDialogTarget] = React.useState<UserDTO | null>(null);
  const [resetDialogTarget, setResetDialogTarget] = React.useState<UserDTO | null>(null);
  const [revokeDialogTarget, setRevokeDialogTarget] = React.useState<UserDTO | null>(null);
  const [deleteDialogTarget, setDeleteDialogTarget] = React.useState<UserDTO | null>(null);
  const [resetTwoFactorDialogTarget, setResetTwoFactorDialogTarget] = React.useState<UserDTO | null>(null);
  const {
    roleFilter,
    setRoleFilter,
    statusFilter,
    setStatusFilter,
    tierFilter,
    setTierFilter,
    sortValue,
    setSortValue,
    filteredItems,
  } = useAdminUserFilters(items);
  const canManageUser = React.useCallback(
    (user: UserDTO) => viewerRole === "superadmin" || user.role !== "superadmin",
    [viewerRole],
  );
  const selectableFilteredItems = React.useMemo(
    () => filteredItems.filter(canManageUser),
    [canManageUser, filteredItems],
  );
  const {
    selectedUserIDs,
    selectAllState,
    resolveSelectedUsers,
    handleSelectAllVisible,
    handleToggleSelectedUser,
    setSelectedUserIDs,
  } = useAdminUserSelection(items, selectableFilteredItems);
  const [batchRole, setBatchRole] = React.useState<AdminUserRole | "">("");
  const [batchStatus, setBatchStatus] = React.useState<AdminUserStatus | "">("");
  const [batchTimezone, setBatchTimezone] = React.useState("");
  const [batchBalance, setBatchBalance] = React.useState("");

  const [createPayload, setCreatePayload] = React.useState<CreateUserPayload>(DEFAULT_CREATE_USER_PAYLOAD);
  const [editPayload, setEditPayload] = React.useState<EditUserPayload>({
    avatarURL: "",
    displayName: "",
    email: "",
    phone: "",
    role: "user",
    status: "active",
    timezone: "Etc/UTC",
    locale: "en-US",
    subscriptionTier: "free",
    subscriptionExpiresAt: "",
    billingBalanceUSD: "0",
    profilePreferences: "",
    reason: "",
  });
  const [resetPasswordDraft, setResetPasswordDraft] = React.useState("");
  const [billingMode, setBillingMode] = React.useState<AdminBillingMode>("self");
  const [billingPlans, setBillingPlans] = React.useState<AdminBillingPlanDTO[]>([]);

  const createAvatarSource = React.useMemo(
    () => ({ username: createPayload.username, displayName: createPayload.displayName }),
    [createPayload.displayName, createPayload.username],
  );

  const avatarDialogPreviewSrc = React.useMemo(() => {
    if (avatarDialog.mode === "edit") {
      return resolveAvatarImageSrc(avatarDialog.value, avatarDialog.target);
    }
    if (avatarDialog.mode === "create") {
      return resolveAvatarImageSrc(avatarDialog.value, createAvatarSource);
    }
    return undefined;
  }, [avatarDialog, createAvatarSource]);

  const editStatusChanged = editDialogTarget ? editPayload.status !== editDialogTarget.status : false;
  const batchTimezoneOptions = React.useMemo(
    () => timeZoneOptions.map((timeZone) => ({ label: timeZone, value: timeZone })),
    [timeZoneOptions],
  );

  const resolveInlineKey = React.useCallback((userID: number, field: InlineEditableField) => `${userID}:${field}`, []);

  React.useEffect(() => {
    let cancelled = false;
    void resolveAccessToken()
      .then((token) => {
        if (!token) {
          return null;
        }
        return getAdminReferenceData(token);
      })
      .then((billing) => {
        if (!cancelled) {
          if (billing) {
            setBillingMode(billing.billingConfig.config.mode);
            setBillingPlans(billing.billingPlans);
          }
        }
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, []);

  React.useEffect(() => {
    if (billingMode !== "period") {
      setTierFilter("");
      return;
    }
    const firstPlan = billingPlans[0];
    if (!firstPlan) {
      return;
    }
    setCreatePayload((current) =>
      current.subscriptionTier && billingPlans.some((plan) => plan.code === current.subscriptionTier)
        ? current
        : { ...current, subscriptionTier: firstPlan.code },
    );
  }, [billingMode, billingPlans, setTierFilter]);

  const refreshUsers = React.useCallback(
    async (nextPage = page) => {
      setPendingAction("refresh");
      try {
        if (nextPage !== page) {
          onSetPage(nextPage);
          return;
        }
        await onLoadUsers();
      } finally {
        setPendingAction("");
      }
    },
    [onLoadUsers, onSetPage, page],
  );

  const handleOpenEditDialog = React.useCallback((user: UserDTO) => {
    if (!canManageUser(user)) {
      return;
    }
    const fallbackSubscriptionTier = billingPlans[0]?.code || "free";
    setEditDialogTarget(user);
    setEditPayload(createEditPayload(user, fallbackSubscriptionTier));
  }, [billingPlans, canManageUser]);

  const handleOpenAvatarDialog = React.useCallback((user: UserDTO) => {
    if (!canManageUser(user)) {
      return;
    }
    setAvatarDialog({ mode: "edit", target: user, value: user.avatarURL.trim() });
  }, [canManageUser]);

  const handleOpenCreateAvatarDialog = React.useCallback(() => {
    setAvatarDialog({ mode: "create", value: createPayload.avatarURL.trim() });
  }, [createPayload.avatarURL]);

  const handleInlineUserPatch = React.useCallback(
    async (
      item: UserDTO,
      field: InlineEditableField,
      payload: Partial<Pick<UserDTO, "role" | "status">>,
    ) => {
      if (!canManageUser(item)) {
        return;
      }
      const inlineKey = resolveInlineKey(item.id, field);
      const previousItem = items.find((user) => user.id === item.id) ?? item;
      setInlinePending((current) => ({ ...current, [inlineKey]: true }));

      try {
        const token = await resolveAccessToken();
        if (!token) {
          throw new Error(t("toast.sessionExpired"));
        }

        onSetUsers((current) => patchByID<UserDTO, number>(current, item.id, (user) => user.id, payload));
        const response = await patchAdminUser(token, item.id, {
          role: payload.role as AdminUserRole | undefined,
          status: payload.status as AdminUserStatus | undefined,
          reason: "inline_admin_table",
        });

        onSetUsers((current) => replaceByID(current, item.id, (user) => user.id, response.user));
        toast.success(t("toast.inlineUpdated", { field: field === "role" ? t("fields.role") : t("fields.status") }));
      } catch (error) {
        onSetUsers((current) => replaceByID(current, item.id, (user) => user.id, previousItem));
        toast.error(t("toast.inlineUpdateFailed", { field: field === "role" ? t("fields.role") : t("fields.status") }), {
          description: resolveAdminErrorMessage(error),
        });
      } finally {
        setInlinePending((current) => {
          const next = { ...current };
          delete next[inlineKey];
          return next;
        });
      }
    },
    [canManageUser, items, onSetUsers, resolveInlineKey, t],
  );

  const onCreateUser = React.useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (pendingAction) {
        return;
      }

      if (!isUsernamePolicyValid(createPayload.username)) {
        toast.error(t("toast.createFailed"), { description: t("validation.usernameLength") });
        return;
      }
      if (createPayload.displayName.trim() && !isDisplayNameLengthValid(createPayload.displayName)) {
        toast.error(t("toast.createFailed"), { description: t("validation.displayNameLength") });
        return;
      }
      if (!isPasswordPolicyValid(createPayload.password)) {
        toast.error(t("toast.createFailed"), { description: t("validation.passwordMinLength") });
        return;
      }

      if (billingMode === "period" && createPayload.subscriptionTier !== "free" && !createPayload.subscriptionExpiresAt.trim()) {
        toast.error(t("toast.createFailed"), { description: t("validation.nonFreeRequiresExpiry") });
        return;
      }

      if (billingMode === "period" && createPayload.subscriptionTier !== "free" && !resolveSubscriptionExpiryDate(createPayload.subscriptionExpiresAt)) {
        toast.error(t("toast.createFailed"), { description: t("validation.invalidSubscriptionExpiry") });
        return;
      }

      setPendingAction("create");
      try {
        const token = await resolveAccessToken();
        if (!token) {
          toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
          return;
        }

        await createAdminUser(token, {
          avatarURL: createPayload.avatarURL.trim(),
          username: createPayload.username.trim(),
          password: createPayload.password,
          displayName: createPayload.displayName.trim(),
          email: createPayload.email.trim(),
          timezone: createPayload.timezone.trim(),
          locale: createPayload.locale.trim(),
          subscriptionTier: billingMode === "period" ? createPayload.subscriptionTier : undefined,
          subscriptionExpiresAt:
            billingMode !== "period" || createPayload.subscriptionTier === "free" || !createPayload.subscriptionExpiresAt.trim()
              ? undefined
              : resolveSubscriptionExpiryISO(createPayload.subscriptionExpiresAt),
        });

        setCreatePayload(DEFAULT_CREATE_USER_PAYLOAD);
        setAvatarDialog({ mode: "closed" });
        setCreateDialogOpen(false);
        toast.success(t("toast.createSucceeded"));
        if (page === 1) {
          await onLoadUsers();
        } else {
          onSetPage(1);
        }
      } catch (error) {
        toast.error(t("toast.createFailed"), { description: resolveAdminErrorMessage(error) });
      } finally {
        setPendingAction("");
      }
    },
    [billingMode, createPayload, onLoadUsers, onSetPage, page, pendingAction, t],
  );

  const handleSaveAvatarDialog = React.useCallback(async () => {
    if (avatarDialog.mode !== "edit" || pendingAction) {
      return;
    }

    const { target, value } = avatarDialog;
    if (!canManageUser(target)) {
      setAvatarDialog({ mode: "closed" });
      return;
    }
    const nextAvatarURL = value.trim();
    if (nextAvatarURL === target.avatarURL.trim()) {
      setAvatarDialog({ mode: "closed" });
      return;
    }

    setPendingAction("avatar");
    setActionUserID(target.id);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }

      const response = await patchAdminUser(token, target.id, {
        avatarURL: nextAvatarURL,
        reason: "admin_update_avatar",
      });

      onSetUsers((current) => replaceByID(current, target.id, (user) => user.id, response.user));
      toast.success(t("toast.avatarUpdated"));
      setEditDialogTarget((current) => (current?.id === target.id ? response.user : current));
      setEditPayload((current) =>
        editDialogTarget?.id === target.id ? { ...current, avatarURL: response.user.avatarURL.trim() } : current,
      );
      setAvatarDialog({ mode: "closed" });
    } catch (error) {
      toast.error(t("toast.avatarUpdateFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setPendingAction("");
      setActionUserID(null);
    }
  }, [avatarDialog, canManageUser, editDialogTarget?.id, onSetUsers, pendingAction, t]);

  const handleSaveEditDialog = React.useCallback(async () => {
    if (!editDialogTarget || pendingAction) {
      return;
    }
    if (!canManageUser(editDialogTarget)) {
      setEditDialogTarget(null);
      return;
    }

    const nextTimezone = editPayload.timezone.trim() || "Etc/UTC";
    const nextDisplayName = editPayload.displayName.trim();
    const nextEmail = editPayload.email.trim();
    const nextPhone = editPayload.phone.trim();
    const nextLocale = editPayload.locale.trim() || "en-US";
    const nextSubscriptionTier = editPayload.subscriptionTier.trim();
    const nextBillingBalance = Number(editPayload.billingBalanceUSD);
    const nextProfilePreferences = editPayload.profilePreferences.trim();
    const patchPayload: AdminUserPatchPayload = {};

    if (!isDisplayNameLengthValid(nextDisplayName)) {
      toast.error(t("toast.editFailed"), { description: t("validation.displayNameLength") });
      return;
    }

    if (editPayload.avatarURL.trim() !== editDialogTarget.avatarURL.trim()) {
      patchPayload.avatarURL = editPayload.avatarURL.trim();
    }
    if (nextDisplayName !== editDialogTarget.displayName.trim()) {
      patchPayload.displayName = nextDisplayName;
    }
    if (nextEmail !== editDialogTarget.email.trim()) {
      patchPayload.email = nextEmail;
    }
    if (nextPhone !== editDialogTarget.phone.trim()) {
      patchPayload.phone = nextPhone;
    }
    if (editPayload.role !== editDialogTarget.role) {
      patchPayload.role = editPayload.role;
    }
    if (editPayload.status !== editDialogTarget.status) {
      patchPayload.status = editPayload.status;
    }
    if (nextTimezone !== (editDialogTarget.timezone.trim() || "Etc/UTC")) {
      patchPayload.timezone = nextTimezone;
    }
    if (nextLocale !== (editDialogTarget.locale.trim() || "en-US")) {
      patchPayload.locale = nextLocale;
    }
    if (nextProfilePreferences !== editDialogTarget.profilePreferences.trim()) {
      patchPayload.profilePreferences = nextProfilePreferences;
    }
    if (billingMode === "period") {
      if (!nextSubscriptionTier) {
        toast.error(t("toast.editFailed"), { description: t("validation.selectSubscriptionPlan") });
        return;
      }
      if (nextSubscriptionTier !== "free" && !editPayload.subscriptionExpiresAt.trim()) {
        toast.error(t("toast.editFailed"), { description: t("validation.nonFreeRequiresExpiry") });
        return;
      }
      if (nextSubscriptionTier !== "free" && !resolveSubscriptionExpiryDate(editPayload.subscriptionExpiresAt)) {
        toast.error(t("toast.editFailed"), { description: t("validation.invalidSubscriptionExpiry") });
        return;
      }

      const currentSubscriptionTier = editDialogTarget.subscriptionTier.trim() || "free";
      const currentSubscriptionExpiresAt = resolveSubscriptionExpiryInputValue(editDialogTarget.subscriptionExpiresAt);
      const nextSubscriptionExpiresAt = nextSubscriptionTier === "free" ? "" : editPayload.subscriptionExpiresAt.trim();
      if (nextSubscriptionTier !== currentSubscriptionTier || nextSubscriptionExpiresAt !== currentSubscriptionExpiresAt) {
        patchPayload.subscriptionTier = nextSubscriptionTier;
        if (nextSubscriptionTier !== "free") {
          patchPayload.subscriptionExpiresAt = resolveSubscriptionExpiryISO(nextSubscriptionExpiresAt);
        }
      }
    }
    if (editPayload.status !== editDialogTarget.status && editPayload.reason.trim()) {
      patchPayload.reason = editPayload.reason.trim();
    }

    const billingBalanceChanged =
      billingMode !== "self" &&
      Number.isFinite(nextBillingBalance) &&
      roundBillingBalance(nextBillingBalance) !== roundBillingBalance(editDialogTarget.billingBalanceUSD ?? 0);

    if (billingMode !== "self" && (!Number.isFinite(nextBillingBalance) || nextBillingBalance < 0)) {
      toast.error(t("toast.editFailed"), { description: t("validation.invalidUsageBalance") });
      return;
    }

    if (!hasPatchChanges(patchPayload) && !billingBalanceChanged) {
      setEditDialogTarget(null);
      return;
    }

    setPendingAction("edit");
    setActionUserID(editDialogTarget.id);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }

      let nextUser: UserDTO = editDialogTarget;
      if (hasPatchChanges(patchPayload)) {
        const response = await patchAdminUser(token, editDialogTarget.id, patchPayload);
        nextUser = response.user;
      }
      if (billingBalanceChanged) {
        const response = await updateAdminBillingAccountBalance(token, editDialogTarget.id, {
          balanceUSD: roundBillingBalance(nextBillingBalance),
          description: t("toast.balanceAdjustmentDescription"),
        });
        nextUser = {
          ...nextUser,
          billingBalanceUSD: response.account.balanceUSD,
          billingAccountStatus: response.account.status,
        };
      }
      onSetUsers((current) => replaceByID(current, editDialogTarget.id, (user) => user.id, nextUser));
      toast.success(t("toast.userUpdated"));
      setEditDialogTarget(null);
    } catch (error) {
      toast.error(t("toast.editFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setPendingAction("");
      setActionUserID(null);
    }
  }, [billingMode, canManageUser, editDialogTarget, editPayload, onSetUsers, pendingAction, t]);

  const onResetPassword = React.useCallback(async () => {
    if (pendingAction || !resetDialogTarget) {
      return;
    }
    if (!canManageUser(resetDialogTarget)) {
      setResetDialogTarget(null);
      return;
    }
    if (!isPasswordPolicyValid(resetPasswordDraft)) {
      toast.error(t("toast.resetPasswordFailed"), { description: t("validation.passwordMinLength") });
      return;
    }

    setPendingAction("reset-password");
    setActionUserID(resetDialogTarget.id);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }

      await resetAdminUserPassword(token, resetDialogTarget.id, {
        newPassword: resetPasswordDraft,
        mustResetPassword: true,
      });

      toast.success(t("toast.passwordReset"));
      setResetPasswordDraft("");
      setResetDialogTarget(null);
    } catch (error) {
      toast.error(t("toast.resetPasswordFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setPendingAction("");
      setActionUserID(null);
    }
  }, [canManageUser, pendingAction, resetDialogTarget, resetPasswordDraft, t]);

  const onResetTwoFactor = React.useCallback(async (user: UserDTO) => {
    if (pendingAction || !user) {
      return;
    }
    if (!canManageUser(user)) {
      setResetTwoFactorDialogTarget(null);
      return;
    }

    setPendingAction("reset-2fa");
    setActionUserID(user.id);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      const response = await resetAdminUserTwoFactor(token, user.id);
      const nextUser = userFromUnknownResponse(response) ?? { ...user, twoFactorEnabled: false };
      onSetUsers((current) => replaceByID(current, user.id, (item) => item.id, nextUser));
      toast.success(t("toast.twoFactorReset"));
      setResetTwoFactorDialogTarget(null);
    } catch (error) {
      toast.error(t("toast.twoFactorResetFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setPendingAction("");
      setActionUserID(null);
    }
  }, [canManageUser, onSetUsers, pendingAction, t]);

  const onRevokeSessions = React.useCallback(async (userID: number) => {
    if (pendingAction) {
      return;
    }
    const user = items.find((item) => item.id === userID);
    if (user && !canManageUser(user)) {
      setRevokeDialogTarget(null);
      return;
    }

    setPendingAction("revoke-sessions");
    setActionUserID(userID);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }

      await revokeAdminUserSessions(token, userID);
      toast.success(t("toast.sessionsRevoked"));
    } catch (error) {
      toast.error(t("toast.revokeSessionsFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setPendingAction("");
      setActionUserID(null);
    }
  }, [canManageUser, items, pendingAction, t]);

  const onDeleteUser = React.useCallback(async (user: UserDTO) => {
    if (pendingAction) {
      return;
    }
    if (!canManageUser(user)) {
      setDeleteDialogTarget(null);
      return;
    }

    setPendingAction("delete");
    setActionUserID(user.id);
    const removedIndex = items.findIndex((item) => item.id === user.id);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }

      onSetUsers((current) => removeByID(current, user.id, (item) => item.id));
      onSetTotal((current) => Math.max(0, current - 1));
      await deleteAdminUser(token, user.id);
      toast.success(t("toast.userDeleted"));
      setDeleteDialogTarget(null);
    } catch (error) {
      onSetUsers((current) => restoreAt(current, user, removedIndex, (item) => item.id));
      onSetTotal((current) => Math.max(current, total));
      toast.error(t("toast.deleteFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setPendingAction("");
      setActionUserID(null);
    }
  }, [canManageUser, items, onSetTotal, onSetUsers, pendingAction, t, total]);

  const onBulkApplyRole = React.useCallback(async () => {
    const selectedUsers = resolveSelectedUsers();
    const nextRole = batchRole;
    if (!selectedUsers.length || !nextRole || pendingAction) {
      return;
    }

    const targets = selectedUsers.filter((item) => item.role !== nextRole);
    if (!targets.length) {
      toast.info(t("toast.bulkRoleAlreadyApplied"));
      return;
    }

    setPendingAction("bulk-role");
    setActionUserID(null);
    const targetIDs = new Set(targets.map((item) => item.id));
    const rollbackUsers = targets.map((item) => ({ item, index: items.findIndex((current) => current.id === item.id) }));
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }

      onSetUsers((current) =>
        current.map((item) => (targetIDs.has(item.id) ? { ...item, role: nextRole } : item)),
      );
      const results = await runSettledBulkItems({
        items: targets,
        title: t("toast.bulkRoleUpdated", { count: targets.length }),
        runItem: (item) =>
          patchAdminUser(token, item.id, {
            role: nextRole,
            reason: "bulk_update_role",
          }),
      });
      const failedUsers = results.filter((result) => result.status === "rejected").map((result) => result.item);
      const successUsers = results.filter((result) => result.status === "fulfilled").map((result) => result.item);
      const successResponses = results
        .filter((result): result is Extract<typeof result, { status: "fulfilled" }> => result.status === "fulfilled")
        .map((result) => result.value.user);
      onSetUsers((current) => successResponses.reduce((next, user) => replaceByID(next, user.id, (item) => item.id, user), current));
      if (failedUsers.length > 0) {
        const failedRollbackUsers = failedUsers.map((item) => ({ item, index: items.findIndex((current) => current.id === item.id) }));
        onSetUsers((current) => restoreManyAt(current, failedRollbackUsers, (item) => item.id));
        setSelectedUserIDs(new Set(failedUsers.map((item) => item.id)));
        toast.error(t("toast.bulkRolePartialFailed"), { description: t("toast.bulkPartialDescription", { success: successUsers.length, failed: failedUsers.length }) });
        return;
      }

      toast.success(t("toast.bulkRoleUpdated", { count: targets.length }));
      setSelectedUserIDs(new Set());
      setBatchRole("");
    } catch (error) {
      onSetUsers((current) => restoreManyAt(current, rollbackUsers, (item) => item.id));
      toast.error(t("toast.bulkRoleFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setPendingAction("");
    }
  }, [batchRole, items, onSetUsers, pendingAction, resolveSelectedUsers, setSelectedUserIDs, t]);

  const onBulkApplyStatus = React.useCallback(async () => {
    const selectedUsers = resolveSelectedUsers();
    const nextStatus = batchStatus;
    if (!selectedUsers.length || !nextStatus || pendingAction) {
      return;
    }

    const targets = selectedUsers.filter((item) => item.status !== nextStatus);
    if (!targets.length) {
      toast.info(t("toast.bulkStatusAlreadyApplied"));
      return;
    }

    setPendingAction("bulk-status");
    setActionUserID(null);
    const targetIDs = new Set(targets.map((item) => item.id));
    const rollbackUsers = targets.map((item) => ({ item, index: items.findIndex((current) => current.id === item.id) }));
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }

      onSetUsers((current) =>
        current.map((item) => (targetIDs.has(item.id) ? { ...item, status: nextStatus } : item)),
      );
      const results = await runSettledBulkItems({
        items: targets,
        title: t("toast.bulkStatusUpdated", { count: targets.length }),
        runItem: (item) =>
          patchAdminUser(token, item.id, {
            status: nextStatus,
            reason: "bulk_update_status",
          }),
      });
      const failedUsers = results.filter((result) => result.status === "rejected").map((result) => result.item);
      const successUsers = results.filter((result) => result.status === "fulfilled").map((result) => result.item);
      const successResponses = results
        .filter((result): result is Extract<typeof result, { status: "fulfilled" }> => result.status === "fulfilled")
        .map((result) => result.value.user);
      onSetUsers((current) => successResponses.reduce((next, user) => replaceByID(next, user.id, (item) => item.id, user), current));
      if (failedUsers.length > 0) {
        const failedRollbackUsers = failedUsers.map((item) => ({ item, index: items.findIndex((current) => current.id === item.id) }));
        onSetUsers((current) => restoreManyAt(current, failedRollbackUsers, (item) => item.id));
        setSelectedUserIDs(new Set(failedUsers.map((item) => item.id)));
        toast.error(t("toast.bulkStatusPartialFailed"), { description: t("toast.bulkPartialDescription", { success: successUsers.length, failed: failedUsers.length }) });
        return;
      }

      toast.success(t("toast.bulkStatusUpdated", { count: targets.length }));
      setSelectedUserIDs(new Set());
      setBatchStatus("");
    } catch (error) {
      onSetUsers((current) => restoreManyAt(current, rollbackUsers, (item) => item.id));
      toast.error(t("toast.bulkStatusFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setPendingAction("");
    }
  }, [batchStatus, items, onSetUsers, pendingAction, resolveSelectedUsers, setSelectedUserIDs, t]);

  const onBulkDeleteUsers = React.useCallback(async () => {
    const selectedUsers = resolveSelectedUsers();
    if (!selectedUsers.length || pendingAction) {
      return;
    }

    setPendingAction("bulk-delete");
    setActionUserID(null);
    const selectedIDs = selectedUsers.map((item) => item.id);
    const rollbackUsers = selectedUsers.map((item) => ({ item, index: items.findIndex((current) => current.id === item.id) }));
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }

      onSetUsers((current) => removeManyByID(current, selectedIDs, (item) => item.id));
      onSetTotal((current) => Math.max(0, current - selectedIDs.length));
      const failedIDs = new Set<number>();
      await runBulkActionInChunks({
        chunkSize: 10,
        items: selectedUsers,
        title: t("toast.bulkDeleting"),
        runChunk: async (chunk) => {
          for (const item of chunk) {
            try {
              await deleteAdminUser(token, item.id);
            } catch {
              failedIDs.add(item.id);
            }
          }
        },
      });
      const failedUsers = selectedUsers.filter((item) => failedIDs.has(item.id));
      const successCount = selectedUsers.length - failedUsers.length;
      if (failedUsers.length > 0) {
        const failedRollbackUsers = failedUsers.map((item) => ({ item, index: items.findIndex((current) => current.id === item.id) }));
        onSetUsers((current) => restoreManyAt(current, failedRollbackUsers, (item) => item.id));
        onSetTotal((current) => Math.max(0, total - successCount));
        setSelectedUserIDs(new Set(failedUsers.map((item) => item.id)));
        toast.error(t("toast.bulkDeletePartialFailed"), { description: t("toast.bulkPartialDescription", { success: successCount, failed: failedUsers.length }) });
        return;
      }

      toast.success(t("toast.bulkDeleted", { count: selectedUsers.length }));
      setSelectedUserIDs(new Set());
    } catch (error) {
      onSetUsers((current) => restoreManyAt(current, rollbackUsers, (item) => item.id));
      onSetTotal((current) => Math.max(current, total));
      toast.error(t("toast.bulkDeleteFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setPendingAction("");
    }
  }, [items, onSetTotal, onSetUsers, pendingAction, resolveSelectedUsers, setSelectedUserIDs, t, total]);

  const onBulkApplyTimezone = React.useCallback(async () => {
    const selectedUsers = resolveSelectedUsers();
    const nextTimezone = batchTimezone.trim();
    if (!selectedUsers.length || !nextTimezone || pendingAction) {
      return;
    }
    const targets = selectedUsers.filter((item) => (item.timezone.trim() || "Etc/UTC") !== nextTimezone);
    if (!targets.length) {
      toast.info(t("toast.timezoneAlreadyApplied"));
      return;
    }

    setPendingAction("bulk-timezone");
    setActionUserID(null);
    const targetIDs = new Set(targets.map((item) => item.id));
    const rollbackUsers = targets.map((item) => ({ item, index: items.findIndex((current) => current.id === item.id) }));
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }

      onSetUsers((current) =>
        current.map((item) => (targetIDs.has(item.id) ? { ...item, timezone: nextTimezone } : item)),
      );
      const results = await runSettledBulkItems({
        items: targets,
        title: t("toast.bulkTimezoneUpdated", { count: targets.length }),
        runItem: (item) =>
          patchAdminUser(token, item.id, {
            timezone: nextTimezone,
          }),
      });
      const failedUsers = results.filter((result) => result.status === "rejected").map((result) => result.item);
      const successUsers = results.filter((result) => result.status === "fulfilled").map((result) => result.item);
      if (failedUsers.length > 0) {
        const failedRollbackUsers = failedUsers.map((item) => ({ item, index: items.findIndex((current) => current.id === item.id) }));
        onSetUsers((current) => restoreManyAt(current, failedRollbackUsers, (item) => item.id));
        setSelectedUserIDs(new Set(failedUsers.map((item) => item.id)));
        toast.error(t("toast.bulkTimezonePartialFailed"), { description: t("toast.bulkPartialDescription", { success: successUsers.length, failed: failedUsers.length }) });
        return;
      }

      toast.success(t("toast.bulkTimezoneUpdated", { count: targets.length }));
      setSelectedUserIDs(new Set());
      setBatchTimezone("");
    } catch (error) {
      onSetUsers((current) => restoreManyAt(current, rollbackUsers, (item) => item.id));
      toast.error(t("toast.bulkTimezoneFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setPendingAction("");
    }
  }, [batchTimezone, items, onSetUsers, pendingAction, resolveSelectedUsers, setSelectedUserIDs, t]);

  const onBulkApplyBalance = React.useCallback(async () => {
    const selectedUsers = resolveSelectedUsers();
    const nextBalance = Number(batchBalance);
    if (!selectedUsers.length || !batchBalance.trim() || pendingAction) {
      return;
    }
    if (billingMode === "self") {
      return;
    }
    if (!Number.isFinite(nextBalance) || nextBalance < 0) {
      toast.error(t("toast.bulkBalanceFailed"), { description: t("validation.invalidUsageBalance") });
      return;
    }

    const roundedBalance = roundBillingBalance(nextBalance);
    const targets = selectedUsers.filter((item) => roundBillingBalance(item.billingBalanceUSD ?? 0) !== roundedBalance);
    if (!targets.length) {
      toast.info(t("toast.bulkBalanceAlreadyApplied"));
      return;
    }

    setPendingAction("bulk-balance");
    setActionUserID(null);
    const targetIDs = new Set(targets.map((item) => item.id));
    const rollbackUsers = targets.map((item) => ({ item, index: items.findIndex((current) => current.id === item.id) }));
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }

      onSetUsers((current) =>
        current.map((item) => (targetIDs.has(item.id) ? { ...item, billingBalanceUSD: roundedBalance } : item)),
      );
      const results = await runSettledBulkItems({
        items: targets,
        title: t("toast.bulkBalanceUpdated", { count: targets.length }),
        runItem: (item) =>
          updateAdminBillingAccountBalance(token, item.id, {
            balanceUSD: roundedBalance,
            description: t("toast.bulkBalanceAdjustmentDescription"),
          }),
      });
      const failedUsers = results.filter((result) => result.status === "rejected").map((result) => result.item);
      const successUsers = results.filter((result) => result.status === "fulfilled").map((result) => result.item);
      const successResponses = results
        .filter((result): result is Extract<typeof result, { status: "fulfilled" }> => result.status === "fulfilled")
        .map((result) => result.value.account);
      onSetUsers((current) =>
        successResponses.reduce(
          (next, account) =>
            patchByID(next, account.userID, (item) => item.id, {
              billingBalanceUSD: account.balanceUSD,
              billingAccountStatus: account.status,
            }),
          current,
        ),
      );
      if (failedUsers.length > 0) {
        const failedRollbackUsers = failedUsers.map((item) => ({ item, index: items.findIndex((current) => current.id === item.id) }));
        onSetUsers((current) => restoreManyAt(current, failedRollbackUsers, (item) => item.id));
        setSelectedUserIDs(new Set(failedUsers.map((item) => item.id)));
        toast.error(t("toast.bulkBalancePartialFailed"), { description: t("toast.bulkPartialDescription", { success: successUsers.length, failed: failedUsers.length }) });
        return;
      }

      toast.success(t("toast.bulkBalanceUpdated", { count: targets.length }));
      setSelectedUserIDs(new Set());
      setBatchBalance("");
    } catch (error) {
      onSetUsers((current) => restoreManyAt(current, rollbackUsers, (item) => item.id));
      toast.error(t("toast.bulkBalanceFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setPendingAction("");
    }
  }, [batchBalance, billingMode, items, onSetUsers, pendingAction, resolveSelectedUsers, setSelectedUserIDs, t]);

  const handleRandomizeAvatarDialog = React.useCallback(() => {
    const nextValue = createGeneratedGithubAvatarRef(generateAvatarVariant());
    setAvatarDialog((current) => (current.mode === "closed" ? current : { ...current, value: nextValue }));
  }, []);

  return {
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
    batchTimezoneOptions,
    filteredItems,
    selectAllState,
    canManageUser,
    resolveInlineKey,
    refreshUsers,
    handleOpenEditDialog,
    handleOpenAvatarDialog,
    handleOpenCreateAvatarDialog,
    handleInlineUserPatch,
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
  };
}
