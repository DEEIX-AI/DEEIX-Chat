import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { deletePermissionGroup, listPermissionGroups, type PermissionGroup } from "@/features/admin/api/permission-groups";
import { invalidateAdminReferenceDataCache } from "@/features/admin/api/reference-data";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

/**
 * Permission group list for the groups page. The initial load is guarded
 * against unmount; later reloads can skip the loading state so the table does
 * not flash after a save or delete.
 */
export function useAdminGroups() {
  const t = useTranslations("adminGroups");
  const [groups, setGroups] = React.useState<PermissionGroup[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [deletePending, setDeletePending] = React.useState(false);

  const fetchGroups = React.useCallback(async () => {
    const token = await resolveAccessToken();
    return listPermissionGroups(token);
  }, []);

  const loadGroups = React.useCallback(async (options: { showLoading?: boolean } = {}) => {
    const showLoading = options.showLoading ?? true;
    if (showLoading) {
      setLoading(true);
    }
    try {
      setGroups(await fetchGroups());
    } catch (error) {
      toast.error(resolveAdminErrorMessage(error, t("loadFailed")));
    } finally {
      if (showLoading) {
        setLoading(false);
      }
    }
  }, [fetchGroups, t]);

  React.useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const list = await fetchGroups();
        if (cancelled) {
          return;
        }
        setGroups(list);
      } catch (error) {
        if (!cancelled) {
          toast.error(resolveAdminErrorMessage(error, t("loadFailed")));
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [fetchGroups, t]);

  // `onDeleted` runs where the page used to clear the edit sheet and the
  // confirmation dialog, i.e. after the optimistic removal and before the refetch.
  const deleteGroup = React.useCallback(async (deleting: PermissionGroup, onDeleted: () => void) => {
    if (deletePending) {
      return;
    }
    const deletedGroupID = deleting.id;
    setDeletePending(true);
    try {
      const token = await resolveAccessToken();
      const result = await deletePermissionGroup(token, deletedGroupID);
      toast.success(t("deletedWithSummary", {
        models: result.summary.manualModelCount ?? 0,
        rules: result.summary.ruleCount ?? 0,
        users: result.summary.manualUserCount ?? 0,
      }));
      invalidateAdminReferenceDataCache();
      setGroups((current) => current.filter((group) => group.id !== deletedGroupID));
      onDeleted();
      void loadGroups({ showLoading: false });
    } catch (error) {
      toast.error(resolveAdminErrorMessage(error, t("saveFailed")));
    } finally {
      setDeletePending(false);
    }
  }, [deletePending, loadGroups, t]);

  return { groups, loading, loadGroups, deletePending, deleteGroup };
}
