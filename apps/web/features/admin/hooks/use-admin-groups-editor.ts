import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import type { AdminUserDTO } from "@/features/admin/api/admin-types";
import { listAdminUsers } from "@/features/admin/api/users";
import { listAdminIdentityProviders } from "@/features/admin/api/auth";
import { listAdminLLMModels, listAdminLLMUpstreams } from "@/features/admin/api/llm";
import type { AdminLLMModelDTO, AdminLLMUpstreamView } from "@/features/admin/api/llm-types";
import {
  listGroupModels,
  listGroupUsers,
  type PermissionGroup,
  type PermissionGroupModelRule,
  setGroupModels,
  setGroupUsers,
  updatePermissionGroup,
} from "@/features/admin/api/permission-groups";
import { invalidateAdminReferenceDataCache } from "@/features/admin/api/reference-data";
import { listAllAdminPages } from "@/features/admin/api/shared";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import type { IdentityProviderDTO } from "@/shared/api/auth-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

const GROUP_PICKER_PAGE_SIZE_DEFAULT = 25;

/**
 * State and requests behind the permission group edit sheet: the group's
 * current model/user assignments, the paged model and user pickers (each with
 * their own filters and "select all results"), and the multi-step save.
 * Opening a different group resets every picker before loading.
 */
export function useAdminGroupsEditor({
  group,
  onOpenChange,
  onSaved,
}: {
  group: PermissionGroup | null;
  onOpenChange: (open: boolean) => void;
  onSaved: () => Promise<void>;
}) {
  const t = useTranslations("adminGroups");
  const [name, setName] = React.useState("");
  const [description, setDescription] = React.useState("");
  const [rateMultiplier, setRateMultiplier] = React.useState("1");
  const [modelIDs, setModelIDs] = React.useState<Set<number>>(new Set());
  const [modelRules, setModelRules] = React.useState<PermissionGroupModelRule[]>([]);
  const [userIDs, setUserIDs] = React.useState<Set<number>>(new Set());
  const [selectionLoading, setSelectionLoading] = React.useState(false);
  const [selectionLoaded, setSelectionLoaded] = React.useState(false);
  const [modelRows, setModelRows] = React.useState<AdminLLMModelDTO[]>([]);
  const [modelTotal, setModelTotal] = React.useState(0);
  const [modelPage, setModelPage] = React.useState(1);
  const [modelPageSize, setModelPageSizeState] = React.useState(GROUP_PICKER_PAGE_SIZE_DEFAULT);
  const [modelQuery, setModelQuery] = React.useState("");
  const [modelUpstreamOptions, setModelUpstreamOptions] = React.useState<AdminLLMUpstreamView[]>([]);
  const [modelUpstreamFilter, setModelUpstreamFilter] = React.useState("");
  const [modelVendorFilter, setModelVendorFilter] = React.useState("");
  const [modelProtocolFilter, setModelProtocolFilter] = React.useState("");
  const [modelReloadKey, setModelReloadKey] = React.useState(0);
  const [modelLoading, setModelLoading] = React.useState(false);
  const [modelBulkLoading, setModelBulkLoading] = React.useState(false);
  const [userRows, setUserRows] = React.useState<AdminUserDTO[]>([]);
  const [userTotal, setUserTotal] = React.useState(0);
  const [userPage, setUserPage] = React.useState(1);
  const [userPageSize, setUserPageSizeState] = React.useState(GROUP_PICKER_PAGE_SIZE_DEFAULT);
  const [userQuery, setUserQuery] = React.useState("");
  const [userSubscriptionFilter, setUserSubscriptionFilter] = React.useState("");
  const [userIdentityFilter, setUserIdentityFilter] = React.useState("");
  const [userIdentityProviderOptions, setUserIdentityProviderOptions] = React.useState<IdentityProviderDTO[]>([]);
  const [userReloadKey, setUserReloadKey] = React.useState(0);
  const [userLoading, setUserLoading] = React.useState(false);
  const [userBulkLoading, setUserBulkLoading] = React.useState(false);
  const [saving, setSaving] = React.useState(false);

  React.useEffect(() => {
    if (!group) {
      setSelectionLoading(false);
      setModelLoading(false);
      setModelBulkLoading(false);
      setUserLoading(false);
      setUserBulkLoading(false);
      return;
    }
    setName(group.name);
    setDescription(group.description);
    setRateMultiplier(String((group.rateMultiplierPercent || 100) / 100));
    setSelectionLoading(true);
    setSelectionLoaded(false);
    setModelRules([]);
    setModelRows([]);
    setModelTotal(0);
    setModelPage(1);
    setModelPageSizeState(GROUP_PICKER_PAGE_SIZE_DEFAULT);
    setModelQuery("");
    setModelUpstreamOptions([]);
    setModelUpstreamFilter("");
    setModelVendorFilter("");
    setModelProtocolFilter("");
    setModelReloadKey(0);
    setUserRows([]);
    setUserTotal(0);
    setUserPage(1);
    setUserPageSizeState(GROUP_PICKER_PAGE_SIZE_DEFAULT);
    setUserQuery("");
    setUserSubscriptionFilter("");
    setUserIdentityFilter("");
    setUserIdentityProviderOptions([]);
    setUserReloadKey(0);
    let cancelled = false;
    (async () => {
      try {
        const token = await resolveAccessToken();
        const [selectedModels, selectedUsers, upstreams, identityProviderPage] = await Promise.all([
          listGroupModels(token, group.id),
          group.isDefault ? Promise.resolve([]) : listGroupUsers(token, group.id),
          listAllAdminPages((options) =>
            listAdminLLMUpstreams(token, {
              ...options,
              status: "active",
              sort: "name_asc",
            }),
          ),
          listAdminIdentityProviders(token),
        ]);
        if (!cancelled) {
          setModelIDs(new Set(selectedModels.modelIDs));
          setModelRules(selectedModels.rules);
          setUserIDs(new Set(selectedUsers));
          setModelUpstreamOptions(upstreams);
          setUserIdentityProviderOptions(identityProviderPage.results);
          setSelectionLoaded(true);
        }
      } catch (error) {
        if (!cancelled) {
          toast.error(resolveAdminErrorMessage(error, t("loadFailed")));
        }
      } finally {
        if (!cancelled) {
          setSelectionLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [group, t]);

  React.useEffect(() => {
    if (!group) {
      return;
    }
    let cancelled = false;
    setModelLoading(true);
    (async () => {
      try {
        const token = await resolveAccessToken();
        const page = await listAdminLLMModels(token, {
          page: modelPage,
          pageSize: modelPageSize,
          query: modelQuery.trim(),
          upstream: modelUpstreamFilter,
          vendor: modelVendorFilter,
          protocol: modelProtocolFilter,
        });
        if (!cancelled) {
          setModelRows(page.results);
          setModelTotal(page.total);
        }
      } catch (error) {
        if (!cancelled) {
          toast.error(resolveAdminErrorMessage(error, t("loadFailed")));
        }
      } finally {
        if (!cancelled) {
          setModelLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [
    group,
    modelPage,
    modelPageSize,
    modelQuery,
    modelReloadKey,
    modelUpstreamFilter,
    modelVendorFilter,
    modelProtocolFilter,
    t,
  ]);

  React.useEffect(() => {
    if (!group) {
      return;
    }
    if (group.isDefault) {
      setUserRows([]);
      setUserTotal(group.userCount ?? 0);
      setUserLoading(false);
      return;
    }
    let cancelled = false;
    setUserLoading(true);
    (async () => {
      try {
        const token = await resolveAccessToken();
        const page = await listAdminUsers(token, {
          page: userPage,
          pageSize: userPageSize,
          query: userQuery.trim(),
          subscriptionStatus: userSubscriptionFilter,
          identityProvider: userIdentityFilter,
        });
        if (!cancelled) {
          setUserRows(page.results);
          setUserTotal(page.total);
        }
      } catch (error) {
        if (!cancelled) {
          toast.error(resolveAdminErrorMessage(error, t("loadFailed")));
        }
      } finally {
        if (!cancelled) {
          setUserLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [group, t, userIdentityFilter, userPage, userPageSize, userQuery, userReloadKey, userSubscriptionFilter]);

  const handleModelQueryChange = React.useCallback((value: string) => {
    setModelQuery(value);
    setModelPage(1);
  }, []);

  const handleModelUpstreamFilterChange = React.useCallback((value: string) => {
    setModelUpstreamFilter(value);
    setModelPage(1);
  }, []);

  const handleModelVendorFilterChange = React.useCallback((value: string) => {
    setModelVendorFilter(value);
    setModelPage(1);
  }, []);

  const handleModelProtocolFilterChange = React.useCallback((value: string) => {
    setModelProtocolFilter(value);
    setModelPage(1);
  }, []);

  const handleUserQueryChange = React.useCallback((value: string) => {
    setUserQuery(value);
    setUserPage(1);
  }, []);

  const handleUserSubscriptionFilterChange = React.useCallback((value: string) => {
    setUserSubscriptionFilter(value);
    setUserPage(1);
  }, []);

  const handleUserIdentityFilterChange = React.useCallback((value: string) => {
    setUserIdentityFilter(value);
    setUserPage(1);
  }, []);

  const handleModelPageSizeChange = React.useCallback((value: number) => {
    setModelPageSizeState(value);
    setModelPage(1);
  }, []);

  const handleUserPageSizeChange = React.useCallback((value: number) => {
    setUserPageSizeState(value);
    setUserPage(1);
  }, []);

  const refreshModels = React.useCallback(() => {
    setModelReloadKey((current) => current + 1);
  }, []);

  const refreshUsers = React.useCallback(() => {
    setUserReloadKey((current) => current + 1);
  }, []);

  const selectAllModels = React.useCallback(async () => {
    setModelBulkLoading(true);
    try {
      const token = await resolveAccessToken();
      const rows = await listAllAdminPages((options) =>
        listAdminLLMModels(token, {
          ...options,
          query: modelQuery.trim(),
          upstream: modelUpstreamFilter,
          vendor: modelVendorFilter,
          protocol: modelProtocolFilter,
        }),
      );
      setModelIDs((current) => {
        const next = new Set(current);
        rows.forEach((model) => {
          next.add(model.id);
        });
        return next;
      });
    } catch (error) {
      toast.error(resolveAdminErrorMessage(error, t("loadFailed")));
    } finally {
      setModelBulkLoading(false);
    }
  }, [modelProtocolFilter, modelQuery, modelUpstreamFilter, modelVendorFilter, t]);

  const selectAllUsers = React.useCallback(async () => {
    setUserBulkLoading(true);
    try {
      const token = await resolveAccessToken();
      const rows = await listAllAdminPages((options) =>
        listAdminUsers(token, {
          ...options,
          query: userQuery.trim(),
          subscriptionStatus: userSubscriptionFilter,
          identityProvider: userIdentityFilter,
        }),
      );
      setUserIDs((current) => {
        const next = new Set(current);
        rows.forEach((user) => {
          next.add(user.id);
        });
        return next;
      });
    } catch (error) {
      toast.error(resolveAdminErrorMessage(error, t("loadFailed")));
    } finally {
      setUserBulkLoading(false);
    }
  }, [t, userIdentityFilter, userQuery, userSubscriptionFilter]);

  const clearModelSelection = React.useCallback(() => {
    setModelIDs(new Set());
  }, []);

  const clearUserSelection = React.useCallback(() => {
    setUserIDs(new Set());
  }, []);

  const handleSave = React.useCallback(async () => {
    if (!group) {
      return;
    }
    setSaving(true);
    let shouldRefreshGroups = false;
    let shouldInvalidateReferenceData = false;
    try {
      const token = await resolveAccessToken();
      const parsed = parseFloat(rateMultiplier);
      const rateMultiplierPercent =
        Number.isFinite(parsed) && parsed > 0 ? Math.round(parsed * 100) : 100;
      await updatePermissionGroup(token, group.id, { name, description, rateMultiplierPercent });
      shouldRefreshGroups = true;
      await setGroupModels(token, group.id, Array.from(modelIDs), modelRules);
      shouldInvalidateReferenceData = true;
      if (!group.isDefault) {
        await setGroupUsers(token, group.id, Array.from(userIDs));
      }
      toast.success(t("saved"));
      invalidateAdminReferenceDataCache();
      onOpenChange(false);
      await onSaved();
    } catch (error) {
      if (shouldInvalidateReferenceData) {
        invalidateAdminReferenceDataCache();
      }
      if (shouldRefreshGroups) {
        void onSaved();
      }
      toast.error(resolveAdminErrorMessage(error, t("saveFailed")));
    } finally {
      setSaving(false);
    }
  }, [description, group, modelIDs, modelRules, name, rateMultiplier, onOpenChange, onSaved, t, userIDs]);

  return {
    name,
    setName,
    description,
    setDescription,
    rateMultiplier,
    setRateMultiplier,
    modelIDs,
    setModelIDs,
    modelRules,
    setModelRules,
    userIDs,
    setUserIDs,
    selectionLoading,
    selectionLoaded,
    modelRows,
    modelTotal,
    modelPage,
    setModelPage,
    modelPageSize,
    modelQuery,
    modelUpstreamOptions,
    modelUpstreamFilter,
    modelVendorFilter,
    modelProtocolFilter,
    modelLoading,
    modelBulkLoading,
    userRows,
    userTotal,
    userPage,
    setUserPage,
    userPageSize,
    userQuery,
    userSubscriptionFilter,
    userIdentityFilter,
    userIdentityProviderOptions,
    userLoading,
    userBulkLoading,
    saving,
    handleModelQueryChange,
    handleModelUpstreamFilterChange,
    handleModelVendorFilterChange,
    handleModelProtocolFilterChange,
    handleUserQueryChange,
    handleUserSubscriptionFilterChange,
    handleUserIdentityFilterChange,
    handleModelPageSizeChange,
    handleUserPageSizeChange,
    refreshModels,
    refreshUsers,
    selectAllModels,
    selectAllUsers,
    clearModelSelection,
    clearUserSelection,
    handleSave,
  };
}
