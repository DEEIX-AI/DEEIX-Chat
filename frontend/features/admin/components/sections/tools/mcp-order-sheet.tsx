"use client";

import * as React from "react";
import { GripVertical, ListOrdered } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Spinner } from "@/components/ui/spinner";
import { cn } from "@/lib/utils";
import {
  listAdminMCPServerTools,
  reorderAdminMCPServers,
} from "@/features/admin/api";
import type { AdminMCPOrderGroupDTO, AdminMCPServerDTO } from "@/features/admin/api/mcp.types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import type { MCPToolDTO } from "@/shared/api/mcp.types";

type MCPOrderSheetProps = {
  open: boolean;
  servers: AdminMCPServerDTO[];
  onClose: () => void;
  onSaved: (groups: AdminMCPOrderGroupDTO[]) => void;
};

type MCPOrderGroup = {
  server: AdminMCPServerDTO;
  tools: MCPToolDTO[];
};

function serverLabel(server: AdminMCPServerDTO): string {
  return server.name.trim() || `#${server.id}`;
}

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

function moveAt<T>(items: T[], index: number, targetIndex: number): T[] {
  if (index < 0 || targetIndex < 0 || index >= items.length || targetIndex >= items.length || index === targetIndex) {
    return items;
  }
  const next = [...items];
  const [item] = next.splice(index, 1);
  next.splice(targetIndex, 0, item);
  return next;
}

function dragServerKey(serverID: number): string {
  return `server:${serverID}`;
}

function dragToolKey(serverID: number, toolID: number): string {
  return `tool:${serverID}:${toolID}`;
}

function parseDraggedToolID(value: string, serverID: number): number | null {
  const prefix = `tool:${serverID}:`;
  if (!value.startsWith(prefix)) {
    return null;
  }
  const id = Number(value.slice(prefix.length));
  return Number.isFinite(id) ? id : null;
}

function DragHandle({
  label,
  disabled,
  onDragStart,
  onDragEnd,
  onKeyDown,
}: {
  label: string;
  disabled?: boolean;
  onDragStart: (event: React.DragEvent<HTMLButtonElement>) => void;
  onDragEnd: () => void;
  onKeyDown: (event: React.KeyboardEvent<HTMLButtonElement>) => void;
}) {
  return (
    <button
      type="button"
      draggable={!disabled}
      disabled={disabled}
      aria-label={label}
      className="flex size-5 shrink-0 cursor-grab items-center justify-center rounded-md text-muted-foreground/70 transition-colors hover:bg-accent hover:text-accent-foreground active:cursor-grabbing disabled:cursor-not-allowed disabled:opacity-50"
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      onKeyDown={onKeyDown}
    >
      <GripVertical className="size-3 stroke-1" />
    </button>
  );
}

export function MCPOrderSheet({
  open,
  servers,
  onClose,
  onSaved,
}: MCPOrderSheetProps) {
  const t = useTranslations("adminTools.order");
  const commonT = useTranslations("common.actions");
  const toastT = useTranslations("adminTools.toast");
  const [groups, setGroups] = React.useState<MCPOrderGroup[]>([]);
  const [selectedServerID, setSelectedServerID] = React.useState<number | null>(null);
  const [loading, setLoading] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [dirty, setDirty] = React.useState(false);
  const [draggingKey, setDraggingKey] = React.useState<string | null>(null);
  const [dragOverKey, setDragOverKey] = React.useState<string | null>(null);
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

  const clearDragState = React.useCallback(() => {
    setDraggingKey(null);
    setDragOverKey(null);
  }, []);

  const startDrag = React.useCallback((event: React.DragEvent<HTMLButtonElement>, key: string) => {
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("text/plain", key);
    setDraggingKey(key);
  }, []);

  const moveServer = React.useCallback((serverID: number, direction: "up" | "down") => {
    const index = groups.findIndex((group) => group.server.id === serverID);
    const targetIndex = direction === "up" ? index - 1 : index + 1;
    commitGroups(moveAt(groups, index, targetIndex));
  }, [commitGroups, groups]);

  const moveServerTo = React.useCallback((serverID: number, targetServerID: number) => {
    if (serverID === targetServerID) {
      return;
    }
    const index = groups.findIndex((group) => group.server.id === serverID);
    const targetIndex = groups.findIndex((group) => group.server.id === targetServerID);
    commitGroups(moveAt(groups, index, targetIndex));
  }, [commitGroups, groups]);

  const moveTool = React.useCallback((toolID: number, direction: "up" | "down") => {
    if (!selectedGroup) {
      return;
    }
    const groupIndex = groups.findIndex((group) => group.server.id === selectedGroup.server.id);
    const toolIndex = selectedGroup.tools.findIndex((tool) => tool.id === toolID);
    const targetIndex = direction === "up" ? toolIndex - 1 : toolIndex + 1;
    if (groupIndex < 0) {
      return;
    }
    commitGroups(groups.map((group, index) =>
      index === groupIndex
        ? { ...group, tools: moveAt(group.tools, toolIndex, targetIndex) }
        : group,
    ));
  }, [commitGroups, groups, selectedGroup]);

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
        ? { ...group, tools: moveAt(group.tools, toolIndex, targetIndex) }
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

  return (
    <Sheet open={open} onOpenChange={(nextOpen) => {
      if (!nextOpen && !saving) {
        onClose();
      }
    }}>
      <SheetContent className="gap-0 sm:max-w-[760px]">
        <SheetHeader>
          <SheetTitle className="flex items-center gap-2 text-sm">
            <ListOrdered className="size-4 stroke-1.5" />
            {t("title")}
          </SheetTitle>
          <SheetDescription className="max-w-2xl text-xs">
            {t("description")}
          </SheetDescription>
        </SheetHeader>

        <div className="min-h-0 flex-1 overflow-hidden px-6 pb-4">
          {loading ? (
            <div className="flex h-full min-h-[22rem] items-center justify-center text-xs text-muted-foreground">
              <Spinner className="mr-2 size-4" />
              {t("loading")}
            </div>
          ) : groups.length === 0 ? (
            <div className="flex h-full min-h-[22rem] items-center justify-center text-xs text-muted-foreground">
              {t("empty")}
            </div>
          ) : (
            <div className="grid h-full min-h-0 grid-cols-1 gap-3 lg:grid-cols-[230px_minmax(0,1fr)]">
              <section className="flex min-h-0 flex-col overflow-hidden rounded-lg border bg-background">
                <div className="flex h-9 shrink-0 items-center justify-between gap-3 border-b px-3">
                  <span className="text-xs font-medium text-foreground">{t("serverHeader")}</span>
                  <span className="text-[11px] text-muted-foreground">{t("itemCount", { count: groups.length })}</span>
                </div>
                <div className="min-h-0 flex-1 overflow-y-auto p-1">
                  <div className="space-y-0.5">
                    {groups.map((group) => {
                      const selected = group.server.id === selectedGroup?.server.id;
                      const dragKey = dragServerKey(group.server.id);
                      return (
                        <div
                          key={group.server.id}
                          className={cn(
                            "group flex min-h-8 w-full items-center gap-0.5 rounded-md px-1 py-1 transition-[background-color,box-shadow,opacity]",
                            selected
                              ? "bg-accent text-accent-foreground"
                              : "text-muted-foreground hover:bg-accent/70 hover:text-accent-foreground",
                            draggingKey === dragKey && "opacity-45",
                            dragOverKey === dragKey && draggingKey !== dragKey && "ring-1 ring-ring/40",
                          )}
                          onDragOver={(event) => {
                            if (!draggingKey?.startsWith("server:")) {
                              return;
                            }
                            event.preventDefault();
                            event.dataTransfer.dropEffect = "move";
                            setDragOverKey(dragKey);
                          }}
                          onDrop={(event) => {
                            event.preventDefault();
                            const raw = event.dataTransfer.getData("text/plain") || draggingKey || "";
                            if (raw.startsWith("server:")) {
                              moveServerTo(Number(raw.slice("server:".length)), group.server.id);
                            }
                            clearDragState();
                          }}
                        >
                          <DragHandle
                            label={t("dragServer", { name: serverLabel(group.server) })}
                            disabled={saving}
                            onDragStart={(event) => startDrag(event, dragKey)}
                            onDragEnd={clearDragState}
                            onKeyDown={(event) => {
                              if (event.key === "ArrowUp") {
                                event.preventDefault();
                                moveServer(group.server.id, "up");
                              } else if (event.key === "ArrowDown") {
                                event.preventDefault();
                                moveServer(group.server.id, "down");
                              }
                            }}
                          />
                          <button
                            type="button"
                            className="flex min-w-0 flex-1 items-center gap-1.5 rounded-sm px-1 text-left"
                            onClick={() => setSelectedServerID(group.server.id)}
                          >
                            <span className="min-w-0 flex-1 truncate text-xs font-medium">{serverLabel(group.server)}</span>
                            <span className="shrink-0 text-[11px] text-muted-foreground">{group.tools.length}</span>
                          </button>
                        </div>
                      );
                    })}
                  </div>
                </div>
              </section>

              <section className="flex min-h-0 flex-col overflow-hidden rounded-lg border bg-background">
                <div className="flex h-9 shrink-0 items-center justify-between gap-3 border-b px-3">
                  <div className="flex min-w-0 flex-1 items-center gap-2">
                    <span className="truncate text-xs font-medium text-foreground">
                      {t("toolHeader")}
                    </span>
                    {selectedGroup ? (
                      <span className="truncate text-[11px] text-muted-foreground">
                        {serverLabel(selectedGroup.server)}
                      </span>
                    ) : null}
                  </div>
                  {selectedGroup ? (
                    <span className="shrink-0 text-[11px] text-muted-foreground">
                      {t("itemCount", { count: selectedGroup.tools.length })}
                    </span>
                  ) : null}
                </div>

                <div className="min-h-0 flex-1 overflow-y-auto p-1">
                  {selectedGroup ? (
                    selectedGroup.tools.length > 0 ? (
                      <div className="space-y-0.5">
                        {selectedGroup.tools.map((tool) => {
                          const dragKey = dragToolKey(selectedGroup.server.id, tool.id);
                          return (
                            <div
                              key={tool.id}
                              className={cn(
                                "flex min-h-8 items-center gap-1.5 rounded-md px-1.5 py-1 text-left transition-[background-color,box-shadow,opacity] hover:bg-accent/55",
                                draggingKey === dragKey && "opacity-45",
                                dragOverKey === dragKey && draggingKey !== dragKey && "ring-1 ring-ring/40",
                              )}
                              onDragOver={(event) => {
                                if (!draggingKey?.startsWith(`tool:${selectedGroup.server.id}:`)) {
                                  return;
                                }
                                event.preventDefault();
                                event.dataTransfer.dropEffect = "move";
                                setDragOverKey(dragKey);
                              }}
                              onDrop={(event) => {
                                event.preventDefault();
                                const raw = event.dataTransfer.getData("text/plain") || draggingKey || "";
                                const toolID = parseDraggedToolID(raw, selectedGroup.server.id);
                                if (toolID !== null) {
                                  moveToolTo(toolID, tool.id);
                                }
                                clearDragState();
                              }}
                            >
                              <DragHandle
                                label={t("dragTool", { name: toolLabel(tool) })}
                                disabled={saving}
                                onDragStart={(event) => startDrag(event, dragKey)}
                                onDragEnd={clearDragState}
                                onKeyDown={(event) => {
                                  if (event.key === "ArrowUp") {
                                    event.preventDefault();
                                    moveTool(tool.id, "up");
                                  } else if (event.key === "ArrowDown") {
                                    event.preventDefault();
                                    moveTool(tool.id, "down");
                                  }
                                }}
                              />
                              <span className="min-w-0 flex-1 truncate text-xs font-medium text-foreground">
                                {toolLabel(tool)}
                              </span>
                              <span className={cn(
                                "shrink-0 rounded-md px-1.5 py-0.5 text-[10px]",
                                tool.status === "active"
                                  ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                                  : "bg-muted text-muted-foreground",
                              )}>
                                {tool.status === "active" ? t("statusActive") : t("statusInactive")}
                              </span>
                            </div>
                          );
                        })}
                      </div>
                    ) : (
                      <div className="flex h-full min-h-[12rem] items-center justify-center text-xs text-muted-foreground">
                        {t("emptyTools")}
                      </div>
                    )
                  ) : null}
                </div>
              </section>
            </div>
          )}
        </div>

        <SheetFooter className="flex-row items-center justify-end gap-2">
          <Button type="button" variant="ghost" size="sm" onClick={onClose} disabled={saving}>
            {commonT("cancel")}
          </Button>
          <Button type="button" size="sm" onClick={() => void handleSave()} disabled={!dirty || saving || loading}>
            {saving ? commonT("saving") : commonT("save")}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
