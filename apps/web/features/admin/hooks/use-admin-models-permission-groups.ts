"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  listModelPermissionGroups,
  listPermissionGroups,
  setModelPermissionGroups,
  type PermissionGroup,
} from "@/features/admin/api/permission-groups";
import type { AdminLLMModelDTO } from "@/features/admin/api/llm-types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

type UseAdminModelsPermissionGroupsOptions = {
  open: boolean;
  mode: "create" | "edit";
  target: AdminLLMModelDTO | null;
  multiUser: boolean;
};

// Permission-group assignment of the model being edited. Manual IDs are the
// editable selection; matched/effective IDs come from the server.
export function useAdminModelsPermissionGroups({ open, mode, target, multiUser }: UseAdminModelsPermissionGroupsOptions) {
  const t = useTranslations("adminModels");
  const [permissionGroups, setPermissionGroups] = React.useState<PermissionGroup[]>([]);
  const [manualPermissionGroupIDs, setManualPermissionGroupIDs] = React.useState<number[]>([]);
  const [matchedPermissionGroupIDs, setMatchedPermissionGroupIDs] = React.useState<number[]>([]);
  const [effectivePermissionGroupIDs, setEffectivePermissionGroupIDs] = React.useState<number[]>([]);
  const [permissionGroupsUnassigned, setPermissionGroupsUnassigned] = React.useState(false);
  const [permissionGroupsLoading, setPermissionGroupsLoading] = React.useState(false);

  React.useEffect(() => {
    if (!open) {
      setPermissionGroups([]);
      setManualPermissionGroupIDs([]);
      setMatchedPermissionGroupIDs([]);
      setEffectivePermissionGroupIDs([]);
      setPermissionGroupsUnassigned(false);
      setPermissionGroupsLoading(false);
      return;
    }

    let cancelled = false;
    setPermissionGroupsLoading(true);
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) {
          return;
        }
        const noModelGroups = { manualGroupIDs: [], matchedGroupIDs: [], effectiveGroupIDs: [], unassigned: false };
        const [groups, modelGroups] = await Promise.all([
          multiUser ? listPermissionGroups(token) : Promise.resolve([]),
          multiUser && mode === "edit" && target ? listModelPermissionGroups(token, target.id) : Promise.resolve(noModelGroups),
        ]);
        if (cancelled) {
          return;
        }
        setPermissionGroups(groups);
        setManualPermissionGroupIDs(modelGroups.manualGroupIDs);
        setMatchedPermissionGroupIDs(modelGroups.matchedGroupIDs);
        setEffectivePermissionGroupIDs(modelGroups.effectiveGroupIDs);
        setPermissionGroupsUnassigned(modelGroups.unassigned);
      } catch (error) {
        if (!cancelled) {
          setPermissionGroups([]);
          setManualPermissionGroupIDs([]);
          setMatchedPermissionGroupIDs([]);
          setEffectivePermissionGroupIDs([]);
          setPermissionGroupsUnassigned(false);
          toast.error(t("toast.permissionGroupsLoadFailed"), { description: resolveAdminErrorMessage(error) });
        }
      } finally {
        if (!cancelled) {
          setPermissionGroupsLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [mode, multiUser, open, t, target]);

  /** Persists the manual selection; a no-op without multi-user support. Errors propagate to the submit flow. */
  async function saveModelPermissionGroups(accessToken: string, modelID: number) {
    if (!multiUser) {
      return;
    }
    const data = await setModelPermissionGroups(accessToken, modelID, manualPermissionGroupIDs);
    setManualPermissionGroupIDs(data.manualGroupIDs);
    setMatchedPermissionGroupIDs(data.matchedGroupIDs);
    setEffectivePermissionGroupIDs(data.effectiveGroupIDs);
    setPermissionGroupsUnassigned(data.unassigned);
  }

  return {
    permissionGroups,
    manualPermissionGroupIDs,
    setManualPermissionGroupIDs,
    matchedPermissionGroupIDs,
    effectivePermissionGroupIDs,
    permissionGroupsUnassigned,
    permissionGroupsLoading,
    saveModelPermissionGroups,
  };
}
