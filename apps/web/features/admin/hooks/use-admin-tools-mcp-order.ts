import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { listAdminMCPServerTools, reorderAdminMCPServers } from "@/features/admin/api";
import type { AdminMCPOrderGroupDTO, AdminMCPServerDTO } from "@/features/admin/api/mcp-types";
import { moveSortableItem } from "@/features/admin/components/shared/sortable-list";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import type { MCPToolDTO } from "@/shared/api/mcp-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export type MCPOrderGroup = {
  server: AdminMCPServerDTO;
  tools: MCPToolDTO[];
};

function toolLabel(tool: MCPToolDTO): string {
  return tool.displayName?.trim() || tool.name;
}

function orderedServers(servers: AdminMCPServerDTO[]): AdminMCPServerDTO[] {
  return [...servers].sort((left, right) => left.sortOrder - right.sortOrder || left.id - right.id);
}

function orderedTools(tools: MCPToolDTO[]): MCPToolDTO[] {
  return [...tools].sort((left, right) => left.sortOrder - right.sortOrder || toolLabel(left).localeCompare(toolLabel(right)) || left.id - right.id);
}

function flattenGroups(groups: MCPOrderGroup[]): string {
  return groups.map((group) => `${group.server.id}:${group.tools.map((tool) => tool.id).join(".")}`).join(",");
}

/**
 * Drives the MCP server/tool ordering sheet: loads every server's tools when
 * the sheet opens, tracks the draft order against the loaded baseline and
 * saves the whole order in one request.
 */
export function useAdminToolsMCPOrder({
  open,
  servers,
  onClose,
  onSaved,
}: {
  open: boolean;
  servers: AdminMCPServerDTO[];
  onClose: () => void;
  onSaved: (groups: AdminMCPOrderGroupDTO[]) => void;
}) {
  const t = useTranslations("adminTools.order");
  const toastT = useTranslations("adminTools.toast");
  const [groups, setGroups] = React.useState<MCPOrderGroup[]>([]);
  const [selectedServerID, setSelectedServerID] = React.useState<number | null>(null);
  const [loading, setLoading] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [dirty, setDirty] = React.useState(false);
  const initialOrderRef = React.useRef("");

  const selectedGroup = groups.find((group) => group.server.id === selectedServerID) ?? groups[0] ?? null;

  const loadOrder = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(toastT("sessionExpired"), { description: toastT("sessionExpiredDescription") });
        return;
      }
      const nextGroups = await Promise.all(
        orderedServers(servers).map(async (server) => ({
          server,
          tools: orderedTools(await listAdminMCPServerTools(token, server.id)),
        })),
      );
      setGroups(nextGroups);
      initialOrderRef.current = flattenGroups(nextGroups);
      setDirty(false);
      setSelectedServerID((current) =>
        nextGroups.some((group) => group.server.id === current)
          ? current
          : nextGroups[0]?.server.id ?? null,
      );
    } catch (error) {
      toast.error(t("loadFailed"), { description: resolveAdminErrorMessage(error, toastT("unknownError")) });
    } finally {
      setLoading(false);
    }
  }, [servers, t, toastT]);

  React.useEffect(() => {
    if (!open) {
      return;
    }
    void loadOrder();
  }, [loadOrder, open]);

  const commitGroups = React.useCallback((nextGroups: MCPOrderGroup[]) => {
    setGroups(nextGroups);
    setDirty(flattenGroups(nextGroups) !== initialOrderRef.current);
  }, []);

  const moveServerTo = React.useCallback((serverID: number, targetServerID: number) => {
    if (serverID === targetServerID) {
      return;
    }
    const index = groups.findIndex((group) => group.server.id === serverID);
    const targetIndex = groups.findIndex((group) => group.server.id === targetServerID);
    commitGroups(moveSortableItem(groups, index, targetIndex));
  }, [commitGroups, groups]);

  const moveToolTo = React.useCallback((toolID: number, targetToolID: number) => {
    if (!selectedGroup || toolID === targetToolID) {
      return;
    }
    const groupIndex = groups.findIndex((group) => group.server.id === selectedGroup.server.id);
    const toolIndex = selectedGroup.tools.findIndex((tool) => tool.id === toolID);
    const targetIndex = selectedGroup.tools.findIndex((tool) => tool.id === targetToolID);
    if (groupIndex < 0) {
      return;
    }
    commitGroups(groups.map((group, index) =>
      index === groupIndex
        ? { ...group, tools: moveSortableItem(group.tools, toolIndex, targetIndex) }
        : group,
    ));
  }, [commitGroups, groups, selectedGroup]);

  const handleSave = React.useCallback(async () => {
    if (!dirty || saving || groups.length === 0) {
      return;
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(toastT("sessionExpired"), { description: toastT("sessionExpiredDescription") });
        return;
      }
      const savedGroups = await reorderAdminMCPServers(token, groups.map((group) => ({
        serverID: group.server.id,
        toolIDs: group.tools.map((tool) => tool.id),
      })));
      const nextGroups = savedGroups.map((group) => ({
        server: group.server,
        tools: orderedTools(group.tools),
      }));
      setGroups(nextGroups);
      initialOrderRef.current = flattenGroups(nextGroups);
      setDirty(false);
      toast.success(t("saveSuccess"));
      onSaved(savedGroups);
      onClose();
    } catch (error) {
      toast.error(t("saveFailed"), { description: resolveAdminErrorMessage(error, toastT("unknownError")) });
    } finally {
      setSaving(false);
    }
  }, [dirty, groups, onClose, onSaved, saving, t, toastT]);

  return {
    groups,
    selectedGroup,
    setSelectedServerID,
    loading,
    saving,
    dirty,
    moveServerTo,
    moveToolTo,
    handleSave,
  };
}
