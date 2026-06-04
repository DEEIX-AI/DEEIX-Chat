"use client";

import * as React from "react";
import { CheckCircle2, FileBraces, Pencil, Plus, RefreshCw, Save, Trash2, Wrench, XCircle } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { toast } from "sonner";

import { SettingsFieldEditor } from "../shared/settings-runtime-panel";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import {
  createAdminMCPServer,
  deleteAdminMCPServer,
  listAdminMCPServerTools,
  listAdminMCPServers,
  listAdminSettings,
  patchAdminSettings,
  syncAdminMCPServerTools,
  updateAdminMCPServer,
  updateAdminMCPServerToolsStatus,
  updateAdminMCPTool,
} from "@/features/admin/api";
import type { AdminMCPServerDTO, AdminMCPServerPayload } from "@/features/admin/api/mcp.types";
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableRow, TableSkeletonRows } from "@/components/ui/table";
import { TablePagination, TableToolbar } from "@/components/ui/table-tools";
import { AdminBulkConfirmDialog } from "@/features/admin/components/bulk-confirm-dialog";
import {
  TOOL_SETTINGS_FIELDS,
  applyToolSettingsDefaults,
  flattenToolSettings,
  resolveToolSettingsErrorMessage,
  toToolEditorField,
  toolFieldID,
} from "@/features/admin/model/tool-settings";
import { cn } from "@/lib/utils";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  SettingsFieldItem,
  SettingsFieldList,
  SettingsPage,
  SettingsSection,
} from "@/shared/components/settings-layout";
import type { MCPToolDTO } from "@/shared/api/mcp.types";
import type { PatchSettingItem } from "@/shared/api/settings.types";

type ServerFormState = {
  id?: number;
  name: string;
  baseURL: string;
  authToken: string;
  headersJSON: string;
  status: "active" | "inactive";
};

type ToolBulkAction = "active" | "inactive";

type ToolFormState = {
  id: number;
  displayName: string;
  description: string;
};

const EMPTY_SERVER_FORM: ServerFormState = {
  name: "",
  baseURL: "",
  authToken: "",
  headersJSON: "{}",
  status: "active",
};

const DEFAULT_SERVER_PAGE_SIZE = 25;
const DEFAULT_TOOL_PAGE_SIZE = 25;
const TOOL_SORT_OPTIONS = [
  { labelKey: "sort.nameAsc", value: "name_asc" },
  { labelKey: "sort.nameDesc", value: "name_desc" },
  { labelKey: "sort.statusAsc", value: "status_asc" },
  { labelKey: "sort.updatedDesc", value: "updated_desc" },
  { labelKey: "sort.updatedAsc", value: "updated_asc" },
] as const;

function serverStatusLabel(status: string, translate: (key: string) => string): string {
  return status === "active" ? translate("status.active") : translate("status.inactive");
}

function toServerForm(server: AdminMCPServerDTO): ServerFormState {
  return {
    id: server.id,
    name: server.name,
    baseURL: server.baseURL,
    authToken: "",
    headersJSON: server.headersJSON || "{}",
    status: server.status === "active" ? "active" : "inactive",
  };
}

function toServerPayload(form: ServerFormState): AdminMCPServerPayload {
  return {
    name: form.name.trim(),
    baseURL: form.baseURL.trim(),
    authToken: form.authToken.trim() || undefined,
    headersJSON: form.headersJSON.trim() || "{}",
    status: form.status,
  };
}

function formatTime(value: string | null | undefined, locale: string, fallback: string): string {
  if (!value) {
    return fallback;
  }
  return new Intl.DateTimeFormat(locale, {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}

function toolDisplayName(tool: MCPToolDTO): string {
  return tool.displayName?.trim() || tool.name;
}

function countActiveTools(items: MCPToolDTO[]): number {
  return items.filter((item) => item.status === "active").length;
}

export function AdminToolsPage() {
  const locale = useLocale();
  const t = useTranslations("adminTools");
  const tActions = useTranslations("common.actions");
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [settingsMap, setSettingsMap] = React.useState<Record<string, string>>(() => applyToolSettingsDefaults({}));
  const [savedMap, setSavedMap] = React.useState<Record<string, string>>(() => applyToolSettingsDefaults({}));
  const [servers, setServers] = React.useState<AdminMCPServerDTO[]>([]);
  const [serversLoading, setServersLoading] = React.useState(true);
  const [serverQuery, setServerQuery] = React.useState("");
  const [serverStatusFilter, setServerStatusFilter] = React.useState("");
  const [serverPage, setServerPage] = React.useState(1);
  const [serverPageSize, setServerPageSize] = React.useState(DEFAULT_SERVER_PAGE_SIZE);
  const [actionServerID, setActionServerID] = React.useState<number | null>(null);
  const [toolSheetServerID, setToolSheetServerID] = React.useState<number | null>(null);
  const [serverDialogOpen, setServerDialogOpen] = React.useState(false);
  const [serverForm, setServerForm] = React.useState<ServerFormState>(EMPTY_SERVER_FORM);
  const [serverSaving, setServerSaving] = React.useState(false);
  const [serverDeleteTarget, setServerDeleteTarget] = React.useState<AdminMCPServerDTO | null>(null);
  const [serverDeleting, setServerDeleting] = React.useState(false);
  const [tools, setTools] = React.useState<MCPToolDTO[]>([]);
  const [toolsLoading, setToolsLoading] = React.useState(false);
  const [toolQuery, setToolQuery] = React.useState("");
  const [toolStatusFilter, setToolStatusFilter] = React.useState("");
  const [toolPage, setToolPage] = React.useState(1);
  const [toolPageSize, setToolPageSize] = React.useState(DEFAULT_TOOL_PAGE_SIZE);
  const [toolSortValue, setToolSortValue] = React.useState<(typeof TOOL_SORT_OPTIONS)[number]["value"]>("name_asc");
  const [selectedToolIDs, setSelectedToolIDs] = React.useState<Set<number>>(new Set());
  const [toolBulkAction, setToolBulkAction] = React.useState<ToolBulkAction | null>(null);
  const [toolBulkApplying, setToolBulkApplying] = React.useState(false);
  const [syncingServerID, setSyncingServerID] = React.useState<number | null>(null);
  const [schemaTool, setSchemaTool] = React.useState<MCPToolDTO | null>(null);
  const [toolForm, setToolForm] = React.useState<ToolFormState | null>(null);
  const [toolSaving, setToolSaving] = React.useState(false);

  const toolSheetServer = React.useMemo(
    () => servers.find((item) => item.id === toolSheetServerID) ?? null,
    [servers, toolSheetServerID],
  );
  const activeToolCount = React.useMemo(() => countActiveTools(tools), [tools]);

  const filteredServers = React.useMemo(() => {
    const query = serverQuery.trim().toLowerCase();
    return servers.filter((server) => {
      if (serverStatusFilter && server.status !== serverStatusFilter) {
        return false;
      }
      if (!query) {
        return true;
      }
      return server.name.toLowerCase().includes(query) || server.baseURL.toLowerCase().includes(query);
    });
  }, [serverQuery, serverStatusFilter, servers]);

  const serverPageCount = Math.max(1, Math.ceil(filteredServers.length / serverPageSize));
  const safeServerPage = Math.min(serverPage, serverPageCount);
  const pagedServers = React.useMemo(() => {
    const start = (safeServerPage - 1) * serverPageSize;
    return filteredServers.slice(start, start + serverPageSize);
  }, [filteredServers, safeServerPage, serverPageSize]);

  const filteredTools = React.useMemo(() => {
    const query = toolQuery.trim().toLowerCase();
    const result = tools.filter((tool) => {
      if (toolStatusFilter && tool.status !== toolStatusFilter) {
        return false;
      }
      if (!query) {
        return true;
      }
      return (
        tool.name.toLowerCase().includes(query) ||
        tool.displayName.toLowerCase().includes(query) ||
        tool.description.toLowerCase().includes(query)
      );
    });
    const updatedTimestamps = new Map(result.map((tool) => [tool.id, new Date(tool.updatedAt || 0).getTime()]));
    result.sort((left, right) => {
      switch (toolSortValue) {
        case "name_desc":
          return toolDisplayName(right).localeCompare(toolDisplayName(left), locale);
        case "status_asc":
          return left.status.localeCompare(right.status, "en") || toolDisplayName(left).localeCompare(toolDisplayName(right), locale);
        case "updated_desc":
          return (updatedTimestamps.get(right.id) ?? 0) - (updatedTimestamps.get(left.id) ?? 0);
        case "updated_asc":
          return (updatedTimestamps.get(left.id) ?? 0) - (updatedTimestamps.get(right.id) ?? 0);
        case "name_asc":
        default:
          return toolDisplayName(left).localeCompare(toolDisplayName(right), locale);
      }
    });
    return result;
  }, [locale, toolQuery, toolSortValue, toolStatusFilter, tools]);

  const toolPageCount = Math.max(1, Math.ceil(filteredTools.length / toolPageSize));
  const safeToolPage = Math.min(toolPage, toolPageCount);
  const pagedTools = React.useMemo(() => {
    const start = (safeToolPage - 1) * toolPageSize;
    return filteredTools.slice(start, start + toolPageSize);
  }, [filteredTools, safeToolPage, toolPageSize]);
  const pagedToolIDs = React.useMemo(() => pagedTools.map((tool) => tool.id), [pagedTools]);
  const selectedToolCount = selectedToolIDs.size;
  const allPagedToolsSelected = pagedToolIDs.length > 0 && pagedToolIDs.every((id) => selectedToolIDs.has(id));
  const somePagedToolsSelected = pagedToolIDs.some((id) => selectedToolIDs.has(id));

  const loadSettings = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const grouped = await listAdminSettings(token);
      const flattened = flattenToolSettings(grouped);
      setSettingsMap(flattened);
      setSavedMap(flattened);
    } catch (error) {
      toast.error(t("toast.settingsLoadFailed"), { description: resolveToolSettingsErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setLoading(false);
    }
  }, [t]);

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
      toast.error(t("toast.serversLoadFailed"), { description: resolveToolSettingsErrorMessage(error, t("toast.unknownError")) });
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
      toast.error(t("toast.toolsLoadFailed"), { description: resolveToolSettingsErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setToolsLoading(false);
    }
  }, [t]);

  React.useEffect(() => {
    void loadSettings();
    void loadServers();
  }, [loadServers, loadSettings]);

  React.useEffect(() => {
    if (!toolSheetServerID) {
      setTools([]);
      setSelectedToolIDs(new Set());
      return;
    }
    void loadTools(toolSheetServerID);
  }, [loadTools, toolSheetServerID]);

  React.useEffect(() => {
    setServerPage((current) => Math.min(current, serverPageCount));
  }, [serverPageCount]);

  React.useEffect(() => {
    setServerPage(1);
  }, [serverQuery, serverStatusFilter, serverPageSize]);

  React.useEffect(() => {
    setToolPage((current) => Math.min(current, toolPageCount));
  }, [toolPageCount]);

  React.useEffect(() => {
    setToolPage(1);
  }, [toolQuery, toolSortValue, toolStatusFilter, toolPageSize, toolSheetServerID]);

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

  const dirtyFieldIDs = React.useMemo(() => {
    const result = new Set<string>();
    for (const field of TOOL_SETTINGS_FIELDS) {
      const id = toolFieldID(field);
      if ((settingsMap[id] ?? "") !== (savedMap[id] ?? "")) {
        result.add(id);
      }
    }
    return result;
  }, [savedMap, settingsMap]);
  const handleSaveMCPSettings = React.useCallback(async () => {
    const items: PatchSettingItem[] = TOOL_SETTINGS_FIELDS
      .filter((field) => dirtyFieldIDs.has(toolFieldID(field)))
      .map((field) => ({
        namespace: field.namespace,
        key: field.key,
        value: settingsMap[toolFieldID(field)] ?? "",
      }));
    if (items.length === 0) {
      return;
    }

    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const grouped = await patchAdminSettings(token, { items });
      const flattened = flattenToolSettings(grouped);
      setSettingsMap(flattened);
      setSavedMap(flattened);
      toast.success(t("toast.settingsUpdated"));
    } catch (error) {
      toast.error(t("toast.saveFailed"), { description: resolveToolSettingsErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setSaving(false);
    }
  }, [dirtyFieldIDs, settingsMap, t]);

  const openCreateServerDialog = React.useCallback(() => {
    setServerForm(EMPTY_SERVER_FORM);
    setServerDialogOpen(true);
  }, []);

  const openEditServerDialog = React.useCallback((server: AdminMCPServerDTO) => {
    setServerForm(toServerForm(server));
    setServerDialogOpen(true);
  }, []);

  const syncTools = React.useCallback(
    async (serverID: number) => {
      setSyncingServerID(serverID);
      try {
        const token = await resolveAccessToken();
        if (!token) {
          toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
          return;
        }
        const nextTools = await syncAdminMCPServerTools(token, serverID);
        setToolSheetServerID(serverID);
        setTools(nextTools);
        await loadServers();
        toast.success(t("toast.toolsSynced"));
      } catch (error) {
        toast.error(t("toast.toolsSyncFailed"), { description: resolveToolSettingsErrorMessage(error, t("toast.unknownError")) });
      } finally {
        setSyncingServerID(null);
      }
    },
    [loadServers, t],
  );

  const saveServer = React.useCallback(async () => {
    setServerSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      let createdServerID: number | null = null;
      if (serverForm.id) {
        await updateAdminMCPServer(token, serverForm.id, toServerPayload(serverForm));
        toast.success(t("toast.serverUpdated"));
      } else {
        const created = await createAdminMCPServer(token, toServerPayload(serverForm));
        createdServerID = created.id;
        toast.success(t("toast.serverCreated"));
      }
      setServerDialogOpen(false);
      if (createdServerID) {
        await syncTools(createdServerID);
      } else {
        await loadServers();
      }
    } catch (error) {
      toast.error(t("toast.serverSaveFailed"), { description: resolveToolSettingsErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setServerSaving(false);
    }
  }, [loadServers, serverForm, syncTools, t]);

  const confirmDeleteServer = React.useCallback(async () => {
      if (!serverDeleteTarget) {
        return;
      }
      setServerDeleting(true);
      try {
        const token = await resolveAccessToken();
        if (!token) {
          toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
          return;
        }
        await deleteAdminMCPServer(token, serverDeleteTarget.id);
        toast.success(t("toast.serverDeleted"));
        setServerDeleteTarget(null);
        await loadServers();
      } catch (error) {
        toast.error(t("toast.serverDeleteFailed"), { description: resolveToolSettingsErrorMessage(error, t("toast.unknownError")) });
      } finally {
        setServerDeleting(false);
      }
    }, [loadServers, serverDeleteTarget, t]);

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
      toast.error(t("toast.serverStatusFailed"), { description: resolveToolSettingsErrorMessage(error, t("toast.unknownError")) });
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
      toast.error(t("toast.toolStatusFailed"), { description: resolveToolSettingsErrorMessage(error, t("toast.unknownError")) });
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
      toast.error(t("toast.selectedToolsUpdateFailed"), { description: resolveToolSettingsErrorMessage(error, t("toast.unknownError")) });
    }
  }, [refreshServerToolCount, selectedToolIDs, t, toolSheetServer, tools]);

  const toggleSelectedTool = React.useCallback((toolID: number, selected: boolean) => {
    setSelectedToolIDs((current) => {
      const next = new Set(current);
      if (selected) {
        next.add(toolID);
      } else {
        next.delete(toolID);
      }
      return next;
    });
  }, []);

  const toggleSelectedPagedTools = React.useCallback((selected: boolean) => {
    setSelectedToolIDs((current) => {
      const next = new Set(current);
      for (const id of pagedToolIDs) {
        if (selected) {
          next.add(id);
        } else {
          next.delete(id);
        }
      }
      return next;
    });
  }, [pagedToolIDs]);

  const openEditToolDialog = React.useCallback((tool: MCPToolDTO) => {
    setToolForm({
      id: tool.id,
      displayName: tool.displayName?.trim() || tool.name,
      description: tool.description ?? "",
    });
  }, []);

  const saveTool = React.useCallback(async () => {
    if (!toolForm) {
      return;
    }
    setToolSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        throw new Error(t("toast.sessionExpired"));
      }
      const savedTool = await updateAdminMCPTool(token, toolForm.id, {
        displayName: toolForm.displayName,
        description: toolForm.description,
      });
      setTools((items) => items.map((item) => (item.id === savedTool.id ? savedTool : item)));
      setToolForm(null);
      toast.success(t("toast.toolUpdated"));
    } catch (error) {
      toast.error(t("toast.toolSaveFailed"), { description: resolveToolSettingsErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setToolSaving(false);
    }
  }, [t, toolForm]);

  const schemaText = React.useMemo(() => {
    const raw = schemaTool?.inputSchemaJSON?.trim();
    if (!raw) {
      return "{}";
    }
    try {
      return JSON.stringify(JSON.parse(raw), null, 2);
    } catch {
      return raw;
    }
  }, [schemaTool]);

  const copySchema = React.useCallback(async () => {
    if (!schemaTool) {
      return;
    }
    try {
      await navigator.clipboard.writeText(schemaText);
      toast.success(t("toast.schemaCopied"));
    } catch {
      toast.error(t("toast.copyFailed"));
    }
  }, [schemaText, schemaTool, t]);

  return (
    <SettingsPage>
      <SettingsSection
        title={t("sections.mcpTools")}
        actions={
          dirtyFieldIDs.size > 0 ? (
            <Button type="button" size="sm" disabled={loading || saving} onClick={() => void handleSaveMCPSettings()}>
              <Save className="size-3.5 stroke-1" />
              {tActions("save")}
            </Button>
          ) : null
        }
      >

        <SettingsFieldList>
          {TOOL_SETTINGS_FIELDS.map((field, index) => {
            const id = toolFieldID(field);
            return (
              <SettingsFieldItem key={id} index={index}>
                <SettingsFieldEditor
                  field={toToolEditorField(field, (key) => t(`fields.${key}`))}
                  value={settingsMap[id] ?? ""}
                  dirty={(settingsMap[id] ?? "") !== (savedMap[id] ?? "")}
                  disabled={loading || saving}
                  onChange={(value) => setSettingsMap((prev) => ({ ...prev, [id]: value }))}
                />
              </SettingsFieldItem>
            );
          })}
        </SettingsFieldList>

        <Field className="gap-2">
          <div className="flex items-center">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <FieldLabel>{t("sections.servers")}</FieldLabel>
              </div>
            </div>
          </div>

          <TableToolbar
            query={serverQuery}
            onQueryChange={setServerQuery}
            queryPlaceholder={t("toolbar.searchServers")}
            filters={[
              {
                key: "status",
                label: t("table.status"),
                value: serverStatusFilter,
                onValueChange: setServerStatusFilter,
                options: [
                  { label: t("status.all"), value: "" },
                  { label: t("status.active"), value: "active" },
                  { label: t("status.inactive"), value: "inactive" },
                ],
              },
            ]}
            loading={serversLoading || actionServerID !== null}
            onRefresh={() => void loadServers()}
            refreshDisabled={serversLoading || actionServerID !== null}
            refreshLoading={serversLoading}
          >
            <Button
              type="button"
              size="sm"
              className="h-7 gap-1 text-xs"
              onClick={openCreateServerDialog}
              disabled={serversLoading}
            >
              <Plus className="size-3.5 stroke-1" />
              {t("toolbar.add")}
            </Button>
          </TableToolbar>

          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead>{t("table.name")}</TableHead>
                <TableHead className="w-[360px]">{t("table.url")}</TableHead>
                <TableHead className="w-24 text-center">{t("table.status")}</TableHead>
                <TableHead className="w-24 text-center">{t("table.tools")}</TableHead>
                <TableHead className="w-32">{t("table.lastSynced")}</TableHead>
                <TableHead className="w-[92px]" stickyEnd />
              </TableRow>
            </TableHeader>
            <TableBody>
              {serversLoading ? (
                <TableSkeletonRows colSpan={6} rowCount={8} />
              ) : pagedServers.length === 0 ? (
                <TableEmptyRow colSpan={6}>{t("table.emptyServers")}</TableEmptyRow>
              ) : (
                pagedServers.map((server) => (
                  <TableRow key={server.id}>
                    <TableCell className="py-1.5">
                      <button
                        type="button"
                        className="inline-flex max-w-full min-w-0 items-center gap-1.5 text-left font-medium hover:underline"
                        title={server.name}
                        onClick={() => openEditServerDialog(server)}
                      >
                        <span className="min-w-0 truncate">{server.name}</span>
                      </button>
                    </TableCell>
                    <TableCell className="w-[360px] max-w-[360px] truncate py-1.5 font-mono text-xs text-muted-foreground" title={server.baseURL}>
                      {server.baseURL}
                    </TableCell>
                    <TableCell className="py-1.5 text-center">
                      <div className="flex h-7 items-center justify-center">
                        <Switch
                          size="sm"
                          checked={server.status === "active"}
                          disabled={actionServerID === server.id}
                          onCheckedChange={(checked) => void setServerStatus(server, checked)}
                          aria-label={t("toolbar.toggleServer", { name: server.name })}
                        />
                      </div>
                    </TableCell>
                    <TableCell className="py-1.5 text-center">
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        className="h-7 gap-1.5 rounded-md px-2 text-xs text-muted-foreground shadow-none hover:bg-muted/60 hover:text-foreground"
                        onClick={() => setToolSheetServerID(server.id)}
                        title={t("toolbar.viewTools", { name: server.name })}
                      >
                        <Wrench className="size-3.5 stroke-1" />
                        {server.activeToolCount ?? 0}/{server.toolCount ?? 0}
                      </Button>
                    </TableCell>
                    <TableCell className="py-1.5 text-xs text-muted-foreground">
                      {formatTime(server.lastSyncedAt, locale, t("table.unsynced"))}
                    </TableCell>
                    <TableCell className="w-[92px] whitespace-nowrap py-1.5" stickyEnd>
                      <div className="flex h-7 items-center justify-start gap-1 md:justify-end">
                        <Button
                          type="button"
                          size="icon-xs"
                          variant="ghost"
                          className="text-muted-foreground shadow-none"
                          disabled={syncingServerID === server.id}
                          onClick={() => void syncTools(server.id)}
                          title={t("toolbar.syncTools")}
                          aria-label={t("toolbar.syncTools")}
                        >
                          <RefreshCw className={cn("size-3.5 stroke-1", syncingServerID === server.id ? "animate-spin" : "")} />
                        </Button>
                        <Button type="button" size="icon-xs" variant="ghost" className="text-muted-foreground shadow-none" onClick={() => openEditServerDialog(server)} title={t("toolbar.editServer")} aria-label={t("toolbar.editServer")}>
                          <Pencil className="size-3.5 stroke-1" />
                        </Button>
                        <Button type="button" size="icon-xs" variant="ghost" className="text-muted-foreground shadow-none" onClick={() => setServerDeleteTarget(server)} title={t("toolbar.deleteServer")} aria-label={t("toolbar.deleteServer")}>
                          <Trash2 className="size-3.5 stroke-1" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>

          <TablePagination
            total={filteredServers.length}
            page={safeServerPage}
            pageCount={serverPageCount}
            pageSize={serverPageSize}
            onPageChange={setServerPage}
            onPageSizeChange={setServerPageSize}
            loading={serversLoading || actionServerID !== null}
          />
        </Field>
      </SettingsSection>

      <Sheet open={Boolean(toolSheetServer)} onOpenChange={(open) => !open && setToolSheetServerID(null)}>
        <SheetContent className="flex flex-col sm:max-w-[720px]">
          <SheetHeader className="px-4 pb-4">
            <SheetTitle>{t("sections.tools")}</SheetTitle>
            <SheetDescription>
              {toolSheetServer?.name ?? ""}
            </SheetDescription>
          </SheetHeader>

          <div className="flex min-h-0 flex-1 flex-col overflow-hidden px-4">
            <TableToolbar
              query={toolQuery}
              onQueryChange={setToolQuery}
              queryPlaceholder={t("toolbar.searchTools")}
              filters={[
                {
                  key: "status",
                  label: t("table.status"),
                  value: toolStatusFilter,
                  onValueChange: setToolStatusFilter,
                  options: [
                    { label: t("status.all"), value: "" },
                    { label: t("status.enabled"), value: "active" },
                    { label: t("status.disabled"), value: "inactive" },
                  ],
                },
              ]}
              sort={{
                value: toolSortValue,
                onValueChange: (value) => setToolSortValue(value as (typeof TOOL_SORT_OPTIONS)[number]["value"]),
                options: TOOL_SORT_OPTIONS.map((item) => ({ label: t(item.labelKey), value: item.value })),
              }}
              selectedCount={selectedToolCount}
              bulkActions={[
                {
                  key: "activate",
                  label: t("toolbar.activateSelected"),
                  icon: <CheckCircle2 className="size-3.5 stroke-1" />,
                  onClick: () => setToolBulkAction("active"),
                },
                {
                  key: "deactivate",
                  label: t("toolbar.deactivateSelected"),
                  icon: <XCircle className="size-3.5 stroke-1" />,
                  onClick: () => setToolBulkAction("inactive"),
                },
              ]}
              loading={toolsLoading || syncingServerID === toolSheetServer?.id}
              onRefresh={() => {
                if (toolSheetServer) {
                  void loadTools(toolSheetServer.id);
                }
              }}
              refreshDisabled={!toolSheetServer || toolsLoading || syncingServerID === toolSheetServer?.id}
              refreshLoading={toolsLoading}
              className="mb-2"
            >
              {toolSheetServer ? (
                <Button
                  type="button"
                  size="sm"
                  className="h-7 shrink-0 text-xs"
                  disabled={syncingServerID === toolSheetServer.id}
                  onClick={() => void syncTools(toolSheetServer.id)}
                >
                  <RefreshCw className={cn("size-3.5 stroke-1", syncingServerID === toolSheetServer.id ? "animate-spin" : "")} />
                  {t("toolbar.sync")}
                </Button>
              ) : null}
            </TableToolbar>
            {toolSheetServer?.lastError ? (
              <div className="mb-3 rounded-md bg-destructive/5 px-3 py-2 text-xs leading-5 text-destructive">
                {toolSheetServer.lastError}
              </div>
            ) : null}

            <div className="min-h-0 flex-1 overflow-y-auto">
                <Table className="min-w-[640px]">
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-[44px] py-1.5 text-center">
                        <div className="flex h-7 items-center justify-center">
                          <Checkbox
                            checked={allPagedToolsSelected ? true : somePagedToolsSelected ? "indeterminate" : false}
                            onCheckedChange={(checked) => toggleSelectedPagedTools(checked === true)}
                            aria-label={t("toolbar.selectPageTools")}
                          />
                        </div>
                      </TableHead>
                      <TableHead>{t("table.tool")}</TableHead>
                      <TableHead className="w-[320px]">{t("table.description")}</TableHead>
                      <TableHead className="w-20 text-center">{t("table.schema")}</TableHead>
                      <TableHead className="w-20 text-center">{t("table.enabled")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {toolsLoading ? <TableSkeletonRows colSpan={5} rowCount={8} /> : null}
                    {pagedTools.map((tool) => (
                      <TableRow key={tool.id} selected={selectedToolIDs.has(tool.id)}>
                        <TableCell className="w-[44px] whitespace-nowrap py-1.5">
                          <div className="flex h-7 items-center justify-center">
                            <Checkbox
                              checked={selectedToolIDs.has(tool.id)}
                              onCheckedChange={(checked) => toggleSelectedTool(tool.id, checked === true)}
                              aria-label={t("toolbar.selectTool", { name: tool.name })}
                            />
                          </div>
                        </TableCell>
                        <TableCell className="py-1.5">
                          <div className="flex min-h-7 min-w-0 max-w-[18rem] items-center gap-2">
                            <div className="min-w-0 flex-1">
                              <p className="truncate text-xs font-medium">{toolDisplayName(tool)}</p>
                              <p className="truncate text-xs leading-4 text-muted-foreground">{tool.name}</p>
                            </div>
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon-xs"
                              className="shrink-0 text-muted-foreground shadow-none"
                              onClick={() => openEditToolDialog(tool)}
                              aria-label={t("toolbar.editTool")}
                              title={t("toolbar.editTool")}
                            >
                              <Pencil className="size-3.5 stroke-1" />
                            </Button>
                          </div>
                        </TableCell>
                        <TableCell className="w-[320px] whitespace-normal py-1.5">
                          <div className="line-clamp-2 text-xs leading-5 text-muted-foreground" title={tool.description || undefined}>
                            {tool.description || t("table.noDescription")}
                          </div>
                        </TableCell>
                        <TableCell className="py-1.5 text-center">
                          <div className="flex h-7 items-center justify-center">
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon-xs"
                              className="text-muted-foreground shadow-none"
                              onClick={() => setSchemaTool(tool)}
                              aria-label={t("toolbar.viewToolSchema", { name: tool.name })}
                              title={t("toolbar.viewSchema")}
                            >
                              <FileBraces className="size-3.5 stroke-1" />
                            </Button>
                          </div>
                        </TableCell>
                        <TableCell className="py-1.5 text-center">
                          <div className="flex h-7 items-center justify-center">
                            <Switch
                              size="sm"
                              checked={tool.status === "active"}
                              onCheckedChange={(checked) => void setToolStatus(tool, checked)}
                              aria-label={t("toolbar.toggleTool", { name: tool.name })}
                            />
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                    {!toolsLoading && filteredTools.length === 0 ? (
                      <TableEmptyRow colSpan={5}>
                        {tools.length === 0 ? t("table.emptyTools") : t("table.emptyFilteredTools")}
                      </TableEmptyRow>
                    ) : null}
                  </TableBody>
                </Table>
            </div>
          </div>

          <SheetFooter className="block px-4 py-3">
            <TablePagination
              total={filteredTools.length}
              page={safeToolPage}
              pageCount={toolPageCount}
              pageSize={toolPageSize}
              onPageChange={setToolPage}
              onPageSizeChange={setToolPageSize}
              loading={toolsLoading || syncingServerID === toolSheetServer?.id}
            />
          </SheetFooter>
        </SheetContent>
      </Sheet>

      <Dialog open={serverDialogOpen} onOpenChange={setServerDialogOpen}>
        <DialogContent className="flex max-h-[min(86vh,760px)] w-[calc(100vw-2rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[560px]">
          <DialogHeader className="shrink-0 px-4 py-4">
            <DialogTitle>{serverForm.id ? t("serverDialog.editTitle") : t("serverDialog.createTitle")}</DialogTitle>
            <DialogDescription>{t("serverDialog.description")}</DialogDescription>
          </DialogHeader>

          <form
            className="flex min-h-0 flex-1 flex-col"
            onSubmit={(event) => {
              event.preventDefault();
              void saveServer();
            }}
          >
            <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-2">
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1">
                  <p className="text-xs text-muted-foreground">
                    {t("serverDialog.name")} <span className="text-destructive">*</span>
                  </p>
                  <Input
                    value={serverForm.name}
                    placeholder={t("serverDialog.namePlaceholder")}
                    onChange={(event) => setServerForm((prev) => ({ ...prev, name: event.target.value }))}
                    required
                  />
                </div>
                <div className="space-y-1">
                  <p className="text-xs text-muted-foreground">{t("serverDialog.status")}</p>
                  <Select
                    value={serverForm.status}
                    onValueChange={(status: "active" | "inactive") => setServerForm((prev) => ({ ...prev, status }))}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="active">{t("status.active")}</SelectItem>
                      <SelectItem value="inactive">{t("status.inactive")}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>

              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">
                  {t("serverDialog.url")} <span className="text-destructive">*</span>
                </p>
                <Input
                  value={serverForm.baseURL}
                  placeholder="https://example.com/mcp"
                  onChange={(event) => setServerForm((prev) => ({ ...prev, baseURL: event.target.value }))}
                  required
                />
              </div>

              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">{t("serverDialog.authToken")}</p>
                <Input
                  value={serverForm.authToken}
                  placeholder={serverForm.id ? t("serverDialog.authTokenEditPlaceholder") : t("serverDialog.authTokenCreatePlaceholder")}
                  onChange={(event) => setServerForm((prev) => ({ ...prev, authToken: event.target.value }))}
                />
              </div>

              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">{t("serverDialog.headers")}</p>
                <Textarea
                  value={serverForm.headersJSON}
                  className="h-24 resize-none font-mono text-xs"
                  placeholder={`{
  "X-API-Key": "..."
}`}
                  onChange={(event) => setServerForm((prev) => ({ ...prev, headersJSON: event.target.value }))}
                />
              </div>
            </div>

            <DialogFooter className="shrink-0 px-4 py-3">
              <Button type="button" variant="ghost" onClick={() => setServerDialogOpen(false)} disabled={serverSaving}>
                {tActions("cancel")}
              </Button>
              <Button type="submit" disabled={serverSaving}>
                {serverForm.id ? tActions("save") : tActions("create")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(toolForm)} onOpenChange={(open) => !open && setToolForm(null)}>
        <DialogContent className="flex max-h-[min(86vh,760px)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[520px]">
          <DialogHeader className="shrink-0 px-4 py-4">
            <DialogTitle>{t("toolDialog.title")}</DialogTitle>
            <DialogDescription>{t("toolDialog.description")}</DialogDescription>
          </DialogHeader>

          <form
            className="flex min-h-0 flex-1 flex-col"
            onSubmit={(event) => {
              event.preventDefault();
              void saveTool();
            }}
          >
            <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-2">
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">{t("toolDialog.displayName")}</p>
                <Input
                  value={toolForm?.displayName ?? ""}
                  placeholder={t("toolDialog.displayNamePlaceholder")}
                  maxLength={160}
                  onChange={(event) => setToolForm((prev) => (prev ? { ...prev, displayName: event.target.value } : prev))}
                />
              </div>
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">{t("toolDialog.toolDescription")}</p>
                <Textarea
                  value={toolForm?.description ?? ""}
                  className="h-28 resize-none text-xs leading-5"
                  placeholder={t("toolDialog.toolDescriptionPlaceholder")}
                  maxLength={4096}
                  onChange={(event) => setToolForm((prev) => (prev ? { ...prev, description: event.target.value } : prev))}
                />
              </div>
            </div>

            <DialogFooter className="shrink-0 px-4 py-3">
              <Button type="button" variant="ghost" onClick={() => setToolForm(null)} disabled={toolSaving}>
                {tActions("cancel")}
              </Button>
              <Button type="submit" disabled={toolSaving}>
                {tActions("save")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(schemaTool)} onOpenChange={(open) => !open && setSchemaTool(null)}>
        <DialogContent className="flex max-h-[min(86vh,760px)] flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl">
          <DialogHeader className="shrink-0 px-4 py-4">
            <DialogTitle>{schemaTool?.displayName || schemaTool?.name || t("schemaDialog.fallbackTitle")}</DialogTitle>
            <DialogDescription>
              {schemaTool?.name ? t("schemaDialog.description", { name: schemaTool.name }) : t("schemaDialog.fallbackDescription")}
            </DialogDescription>
          </DialogHeader>
          <pre className="mx-4 min-h-0 flex-1 overflow-auto rounded-md border border-border/60 bg-muted/35 p-3 text-xs leading-5 text-foreground/86">
            <code>{schemaText}</code>
          </pre>
          <DialogFooter className="shrink-0 px-4 py-3">
            <Button type="button" variant="ghost" onClick={() => setSchemaTool(null)}>
              {tActions("close")}
            </Button>
            <Button type="button" onClick={() => void copySchema()}>
              {tActions("copy")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AdminBulkConfirmDialog
        open={toolBulkAction !== null}
        onOpenChange={(open) => {
          if (!open && !toolBulkApplying) {
            setToolBulkAction(null);
          }
        }}
        pending={toolBulkApplying}
        title={t("bulkConfirm.title")}
        description={t("bulkConfirm.description", { count: selectedToolCount })}
        confirmLabel={t("bulkConfirm.confirm")}
        pendingLabel={t("bulkConfirm.pending")}
        onConfirm={() => {
          if (!toolBulkAction) {
            return;
          }
          setToolBulkApplying(true);
          void setSelectedToolsStatus(toolBulkAction).finally(() => {
            setToolBulkApplying(false);
            setToolBulkAction(null);
          });
        }}
      />

      <AlertDialog
        open={serverDeleteTarget !== null}
        onOpenChange={(open) => {
          if (!open && !serverDeleting) {
            setServerDeleteTarget(null);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("toolbar.deleteServer")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("confirm.deleteServer", { name: serverDeleteTarget?.name ?? "" })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={serverDeleting}>
              {tActions("cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={serverDeleting || !serverDeleteTarget}
              onClick={(event) => {
                event.preventDefault();
                void confirmDeleteServer();
              }}
            >
              {serverDeleting ? t("bulkConfirm.pending") : tActions("delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsPage>
  );
}
