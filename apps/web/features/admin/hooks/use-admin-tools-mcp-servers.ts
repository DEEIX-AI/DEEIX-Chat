import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  createAdminMCPServer,
  deleteAdminMCPServer,
  listAdminMCPServerTools,
  listAdminMCPServers,
  syncAdminMCPServerTools,
  updateAdminMCPServer,
  updateAdminMCPServerToolsStatus,
  updateAdminMCPTool,
} from "@/features/admin/api";
import type { AdminMCPOrderGroupDTO, AdminMCPServerDTO, AdminMCPServerPayload } from "@/features/admin/api/mcp-types";
import type { MCPToolEditFormState } from "@/features/admin/components/sections/tools/tools-mcp-tool-edit-dialog";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import type { MCPToolDTO } from "@/shared/api/mcp-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export type ToolSyncConfirmation = {
  serverID: number;
  serverName: string;
};

function serverStatusLabel(status: string, translate: (key: string) => string): string {
  return status === "active" ? translate("status.active") : translate("status.inactive");
}

function countActiveTools(items: MCPToolDTO[]): number {
  return items.filter((item) => item.status === "active").length;
}

/**
 * MCP servers and the tools of the server open in the tool sheet. Servers and
 * tools live together because tool edits update the server's tool counters and
 * a sync opens the synced server's sheet. Status toggles are optimistic with
 * rollback; syncing a server with customized tool metadata asks first.
 */
export function useAdminToolsMCPServers() {
  const t = useTranslations("adminTools");
  const [servers, setServers] = React.useState<AdminMCPServerDTO[]>([]);
  const [serversLoading, setServersLoading] = React.useState(true);
  const [actionServerID, setActionServerID] = React.useState<number | null>(null);
  const [toolSheetServerID, setToolSheetServerID] = React.useState<number | null>(null);
  const [serverSaving, setServerSaving] = React.useState(false);
  const [serverDeleting, setServerDeleting] = React.useState(false);
  const [tools, setTools] = React.useState<MCPToolDTO[]>([]);
  const [toolsLoading, setToolsLoading] = React.useState(false);
  const [selectedToolIDs, setSelectedToolIDs] = React.useState<Set<number>>(new Set());
  const [syncingServerID, setSyncingServerID] = React.useState<number | null>(null);
  const [toolSyncConfirmation, setToolSyncConfirmation] = React.useState<ToolSyncConfirmation | null>(null);
  const [toolSaving, setToolSaving] = React.useState(false);

  const toolSheetServer = React.useMemo(
    () => servers.find((item) => item.id === toolSheetServerID) ?? null,
    [servers, toolSheetServerID],
  );

  const loadServers = React.useCallback(async () => {
    setServersLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const items = await listAdminMCPServers(token);
      setServers(items);
      setToolSheetServerID((current) => (current && items.some((item) => item.id === current) ? current : null));
    } catch (error) {
      toast.error(t("toast.serversLoadFailed"), { description: resolveAdminErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setServersLoading(false);
    }
  }, [t]);

  const loadTools = React.useCallback(async (serverID: number) => {
    setToolsLoading(true);
    setTools([]);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      setTools(await listAdminMCPServerTools(token, serverID));
    } catch (error) {
      setTools([]);
      toast.error(t("toast.toolsLoadFailed"), { description: resolveAdminErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setToolsLoading(false);
    }
  }, [t]);

  React.useEffect(() => {
    void loadServers();
  }, [loadServers]);

  React.useEffect(() => {
    if (!toolSheetServerID) {
      setTools([]);
      setSelectedToolIDs(new Set());
      return;
    }
    void loadTools(toolSheetServerID);
  }, [loadTools, toolSheetServerID]);

  React.useEffect(() => {
    setSelectedToolIDs((current) => {
      if (current.size === 0) {
        return current;
      }
      const existingIDs = new Set(tools.map((tool) => tool.id));
      const next = new Set([...current].filter((id) => existingIDs.has(id)));
      return next.size === current.size ? current : next;
    });
  }, [tools]);

  const syncTools = React.useCallback(
    async (serverID: number, overwriteCustomizedMetadata = false) => {
      setSyncingServerID(serverID);
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        setSyncingServerID(null);
        return;
      }

      try {
        const nextTools = await syncAdminMCPServerTools(token, serverID, overwriteCustomizedMetadata);
        setToolSheetServerID(serverID);
        setTools(nextTools);
        toast.success(t("toast.toolsSynced"));
      } catch (error) {
        toast.error(t("toast.toolsSyncFailed"), { description: resolveAdminErrorMessage(error, t("toast.unknownError")) });
      } finally {
        await loadServers();
        setSyncingServerID(null);
      }
    },
    [loadServers, t],
  );

  const requestToolSync = React.useCallback(
    (serverID: number) => {
      const server = servers.find((item) => item.id === serverID);
      if (server?.requiresToolMetadataSyncConfirmation) {
        setToolSyncConfirmation({ serverID, serverName: server.name });
        return;
      }
      void syncTools(serverID);
    },
    [servers, syncTools],
  );

  const confirmToolSync = React.useCallback(
    (overwriteCustomizedMetadata: boolean) => {
      if (!toolSyncConfirmation) {
        return;
      }
      const serverID = toolSyncConfirmation.serverID;
      setToolSyncConfirmation(null);
      void syncTools(serverID, overwriteCustomizedMetadata);
    },
    [syncTools, toolSyncConfirmation],
  );

  // `onSaved` closes the server dialog at the point the page used to, i.e.
  // after the success toast and before the follow-up sync or reload.
  const saveServer = React.useCallback(async (
    { id, payload }: { id?: number; payload: AdminMCPServerPayload },
    onSaved: () => void,
  ) => {
    setServerSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      let createdServerID: number | null = null;
      if (id) {
        await updateAdminMCPServer(token, id, payload);
        toast.success(t("toast.serverUpdated"));
      } else {
        const created = await createAdminMCPServer(token, payload);
        createdServerID = created.id;
        toast.success(t("toast.serverCreated"));
      }
      onSaved();
      if (createdServerID) {
        await syncTools(createdServerID);
      } else {
        await loadServers();
      }
    } catch (error) {
      toast.error(t("toast.serverSaveFailed"), { description: resolveAdminErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setServerSaving(false);
    }
  }, [loadServers, syncTools, t]);

  const confirmDeleteServer = React.useCallback(async (serverDeleteTarget: AdminMCPServerDTO, onDeleted: () => void) => {
      setServerDeleting(true);
      try {
        const token = await resolveAccessToken();
        if (!token) {
          toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
          return;
        }
        await deleteAdminMCPServer(token, serverDeleteTarget.id);
        toast.success(t("toast.serverDeleted"));
        onDeleted();
        await loadServers();
      } catch (error) {
        toast.error(t("toast.serverDeleteFailed"), { description: resolveAdminErrorMessage(error, t("toast.unknownError")) });
      } finally {
        setServerDeleting(false);
      }
    }, [loadServers, t]);

  const setServerStatus = React.useCallback(async (server: AdminMCPServerDTO, active: boolean) => {
    const previous = servers;
    const nextStatus = active ? "active" : "inactive";
    setActionServerID(server.id);
    setServers((items) => items.map((item) => (item.id === server.id ? { ...item, status: nextStatus } : item)));
    try {
      const token = await resolveAccessToken();
      if (!token) {
        throw new Error(t("toast.sessionExpired"));
      }
      await updateAdminMCPServer(token, server.id, {
        name: server.name,
        baseURL: server.baseURL,
        headersJSON: server.headersJSON || "{}",
        status: nextStatus,
      });
      toast.success(t("toast.serverStatusUpdated", { status: serverStatusLabel(nextStatus, t) }));
    } catch (error) {
      setServers(previous);
      toast.error(t("toast.serverStatusFailed"), { description: resolveAdminErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setActionServerID(null);
    }
  }, [servers, t]);

  const refreshServerToolCount = React.useCallback((serverID: number, nextTools: MCPToolDTO[]) => {
    setServers((items) =>
      items.map((item) =>
        item.id === serverID
          ? { ...item, toolCount: nextTools.length, activeToolCount: countActiveTools(nextTools) }
          : item,
      ),
    );
  }, []);

  const setToolStatus = React.useCallback(async (tool: MCPToolDTO, active: boolean) => {
    const previous = tools;
    const nextStatus = active ? "active" : "inactive";
    const nextTools = tools.map((item) => (item.id === tool.id ? { ...item, status: nextStatus } : item));
    setTools(nextTools);
    refreshServerToolCount(tool.serverID, nextTools);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        throw new Error(t("toast.sessionExpired"));
      }
      await updateAdminMCPTool(token, tool.id, { status: nextStatus });
    } catch (error) {
      setTools(previous);
      refreshServerToolCount(tool.serverID, previous);
      toast.error(t("toast.toolStatusFailed"), { description: resolveAdminErrorMessage(error, t("toast.unknownError")) });
    }
  }, [refreshServerToolCount, t, tools]);

  const setSelectedToolsStatus = React.useCallback(async (status: "active" | "inactive") => {
    if (!toolSheetServer || selectedToolIDs.size === 0) {
      return;
    }
    const targetIDs = [...selectedToolIDs];
    const targetIDSet = new Set(targetIDs);
    const previous = tools;
    const nextTools = tools.map((item) => (targetIDSet.has(item.id) ? { ...item, status } : item));
    setTools(nextTools);
    refreshServerToolCount(toolSheetServer.id, nextTools);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        throw new Error(t("toast.sessionExpired"));
      }
      const savedTools = await updateAdminMCPServerToolsStatus(token, toolSheetServer.id, status, targetIDs);
      setTools(savedTools);
      refreshServerToolCount(toolSheetServer.id, savedTools);
      setSelectedToolIDs(new Set());
      toast.success(status === "active" ? t("toast.selectedToolsEnabled") : t("toast.selectedToolsDisabled"));
    } catch (error) {
      setTools(previous);
      refreshServerToolCount(toolSheetServer.id, previous);
      toast.error(t("toast.selectedToolsUpdateFailed"), { description: resolveAdminErrorMessage(error, t("toast.unknownError")) });
    }
  }, [refreshServerToolCount, selectedToolIDs, t, toolSheetServer, tools]);

  const saveTool = React.useCallback(async (toolForm: MCPToolEditFormState, onSaved: () => void) => {
    setToolSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        throw new Error(t("toast.sessionExpired"));
      }
      const attachmentConfig = toolForm.attachmentInputMode === "image" ? {
        attachmentInputMode: toolForm.attachmentInputMode,
        attachmentArgument: toolForm.attachmentArgument,
        attachmentEncoding: toolForm.attachmentEncoding,
        attachmentPromptArgument: toolForm.passUserPrompt ? toolForm.attachmentPromptArgument : "",
      } : {
        attachmentInputMode: toolForm.attachmentInputMode,
      };
      const savedTool = await updateAdminMCPTool(token, toolForm.id, {
        displayName: toolForm.displayName,
        description: toolForm.description,
        ...attachmentConfig,
      });
      setTools((items) => items.map((item) => (item.id === savedTool.id ? savedTool : item)));
      await loadServers();
      onSaved();
      toast.success(t("toast.toolUpdated"));
    } catch (error) {
      toast.error(t("toast.toolSaveFailed"), { description: resolveAdminErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setToolSaving(false);
    }
  }, [loadServers, t]);

  // Applies the order saved in the MCP order sheet to both lists at once.
  const applyMCPOrder = React.useCallback((groups: AdminMCPOrderGroupDTO[]) => {
    setServers(groups.map((group) => group.server));
    if (toolSheetServerID) {
      const currentGroup = groups.find((group) => group.server.id === toolSheetServerID);
      if (currentGroup) {
        setTools(currentGroup.tools);
      }
    }
  }, [toolSheetServerID]);

  return {
    servers,
    serversLoading,
    actionServerID,
    toolSheetServerID,
    setToolSheetServerID,
    toolSheetServer,
    serverSaving,
    serverDeleting,
    tools,
    toolsLoading,
    selectedToolIDs,
    setSelectedToolIDs,
    syncingServerID,
    toolSyncConfirmation,
    setToolSyncConfirmation,
    toolSaving,
    loadServers,
    loadTools,
    requestToolSync,
    confirmToolSync,
    saveServer,
    confirmDeleteServer,
    setServerStatus,
    setToolStatus,
    setSelectedToolsStatus,
    saveTool,
    applyMCPOrder,
  };
}
