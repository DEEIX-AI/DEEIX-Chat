"use client";

import { ListOrdered } from "lucide-react";
import { useTranslations } from "next-intl";

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
  AdminSortableHandle,
  AdminSortableItem,
  AdminSortableList,
} from "@/features/admin/components/shared/sortable-list";
import type { AdminMCPOrderGroupDTO, AdminMCPServerDTO } from "@/features/admin/api/mcp-types";
import { useAdminToolsMCPOrder } from "@/features/admin/hooks/use-admin-tools-mcp-order";
import type { MCPToolDTO } from "@/shared/api/mcp-types";

type MCPOrderSheetProps = {
  open: boolean;
  servers: AdminMCPServerDTO[];
  onClose: () => void;
  onSaved: (groups: AdminMCPOrderGroupDTO[]) => void;
};

function serverLabel(server: AdminMCPServerDTO): string {
  return server.name.trim() || `#${server.id}`;
}

function toolLabel(tool: MCPToolDTO): string {
  return tool.displayName?.trim() || tool.name;
}

export function MCPOrderSheet({
  open,
  servers,
  onClose,
  onSaved,
}: MCPOrderSheetProps) {
  const t = useTranslations("adminTools.order");
  const commonT = useTranslations("common.actions");
  const {
    groups,
    selectedGroup,
    setSelectedServerID,
    loading,
    saving,
    dirty,
    moveServerTo,
    moveToolTo,
    handleSave,
  } = useAdminToolsMCPOrder({ open, servers, onClose, onSaved });

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
            <div className="flex h-full min-h-[22rem] items-center justify-center px-3 py-6 text-center text-xs text-muted-foreground">
              <Spinner className="mr-2 size-4" />
              {t("loading")}
            </div>
          ) : groups.length === 0 ? (
            <div className="flex h-full min-h-[22rem] items-center justify-center px-3 py-6 text-center text-xs text-muted-foreground">
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
                  <AdminSortableList
                    items={groups.map((group) => String(group.server.id))}
                    disabled={saving || groups.length < 2}
                    onMove={(serverID, targetServerID) => moveServerTo(Number(serverID), Number(targetServerID))}
                  >
                    <div className="space-y-0.5">
                      {groups.map((group) => {
                        const selected = group.server.id === selectedGroup?.server.id;
                        return (
                          <AdminSortableItem
                            key={group.server.id}
                            id={String(group.server.id)}
                            disabled={saving || groups.length < 2}
                          >
                            {({ attributes, isDragging, listeners }) => (
                              <div
                                className={cn(
                                  "group flex min-h-8 w-full items-center gap-0.5 rounded-md px-1 py-1 transition-[background-color,box-shadow,opacity]",
                                  selected
                                    ? "bg-accent text-accent-foreground"
                                    : "text-muted-foreground hover:bg-accent/70 hover:text-accent-foreground",
                                  isDragging && "opacity-45",
                                )}
                              >
                                <AdminSortableHandle
                                  attributes={attributes}
                                  disabled={saving}
                                  hidden={groups.length < 2}
                                  label={t("dragServer", { name: serverLabel(group.server) })}
                                  listeners={listeners}
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
                            )}
                          </AdminSortableItem>
                        );
                      })}
                    </div>
                  </AdminSortableList>
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
                      <AdminSortableList
                        items={selectedGroup.tools.map((tool) => String(tool.id))}
                        disabled={saving || selectedGroup.tools.length < 2}
                        onMove={(toolID, targetToolID) => moveToolTo(Number(toolID), Number(targetToolID))}
                      >
                        <div className="space-y-0.5">
                          {selectedGroup.tools.map((tool) => (
                            <AdminSortableItem
                              key={tool.id}
                              id={String(tool.id)}
                              disabled={saving || selectedGroup.tools.length < 2}
                            >
                              {({ attributes, isDragging, listeners }) => (
                                <div
                                  className={cn(
                                    "flex min-h-8 items-center gap-1.5 rounded-md px-1.5 py-1 text-left transition-[background-color,box-shadow,opacity] hover:bg-accent/55",
                                    isDragging && "opacity-45",
                                  )}
                                >
                                  <AdminSortableHandle
                                    attributes={attributes}
                                    disabled={saving}
                                    hidden={selectedGroup.tools.length < 2}
                                    label={t("dragTool", { name: toolLabel(tool) })}
                                    listeners={listeners}
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
                              )}
                            </AdminSortableItem>
                          ))}
                        </div>
                      </AdminSortableList>
                    ) : (
                      <div className="flex h-full min-h-[12rem] items-center justify-center px-3 py-6 text-center text-xs text-muted-foreground">
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
