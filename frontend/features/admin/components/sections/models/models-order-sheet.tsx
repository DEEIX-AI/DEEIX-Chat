"use client";

import * as React from "react";
import { GripVertical, ListOrdered } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
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
  listAdminLLMModels,
  reorderAdminLLMModels,
} from "@/features/admin/api";
import { invalidateAdminReferenceDataCache } from "@/features/admin/api/reference-data";
import { listAllAdminPages } from "@/features/admin/api/shared";
import type { AdminLLMModelDTO } from "@/features/admin/api/llm.types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { LobeHubIcon } from "@/shared/components/lobehub-icon";
import { resolveLobeHubIconURL, resolveModelIdentity, resolveVendorIdentity } from "@/shared/lib/model-identity";
import { parseKindsJSON } from "@/shared/model/llm-schema";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

type ModelOrderSheetProps = {
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
};

type ModelOrderGroup = {
  vendorKey: string;
  label: string;
  icon: string;
  items: AdminLLMModelDTO[];
};

function buildModelOrderGroups(models: AdminLLMModelDTO[]): ModelOrderGroup[] {
  const groups = new Map<string, ModelOrderGroup>();
  for (const model of models) {
    const identity = resolveVendorIdentity(model.vendor);
    const current = groups.get(identity.vendorKey);
    if (current) {
      current.items.push(model);
      continue;
    }
    groups.set(identity.vendorKey, {
      vendorKey: identity.vendorKey,
      label: identity.vendorLabel,
      icon: identity.vendorIcon,
      items: [model],
    });
  }
  return Array.from(groups.values());
}

function flattenGroups(groups: ModelOrderGroup[]): AdminLLMModelDTO[] {
  return groups.flatMap((group) => group.items);
}

function moveAt<T>(items: T[], index: number, targetIndex: number): T[] {
  if (index < 0 || targetIndex < 0 || index >= items.length || targetIndex >= items.length) {
    return items;
  }
  if (index === targetIndex) {
    return items;
  }
  const next = [...items];
  const [item] = next.splice(index, 1);
  next.splice(targetIndex, 0, item);
  return next;
}

function dragVendorKey(vendorKey: string): string {
  return `vendor:${vendorKey}`;
}

function dragModelKey(vendorKey: string, modelID: number): string {
  return `model:${vendorKey}:${modelID}`;
}

function parseDraggedModelID(value: string, vendorKey: string): number | null {
  const prefix = `model:${vendorKey}:`;
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

function KindBadges({ kindsJSON }: { kindsJSON: string }) {
  const t = useTranslations("adminModels");
  const kinds = parseKindsJSON(kindsJSON);
  if (kinds.length === 0) {
    return null;
  }
  return (
    <div className="flex min-w-0 shrink-0 items-center justify-end gap-1 overflow-hidden">
      {kinds.map((kind) => (
        <Badge key={kind} variant="secondary" className="h-5 max-w-24 shrink-0 truncate px-1.5 py-0">
          {["chat", "audio", "image_gen", "image_edit", "video_gen"].includes(kind)
            ? t(`kinds.${kind}`)
            : kind}
        </Badge>
      ))}
    </div>
  );
}

export function ModelOrderSheet({
  open,
  onClose,
  onSaved,
}: ModelOrderSheetProps) {
  const t = useTranslations("adminModels.order");
  const commonT = useTranslations("common.actions");
  const toastT = useTranslations("adminModels.toast");
  const [models, setModels] = React.useState<AdminLLMModelDTO[]>([]);
  const [selectedVendorKey, setSelectedVendorKey] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [dirty, setDirty] = React.useState(false);
  const [draggingKey, setDraggingKey] = React.useState<string | null>(null);
  const [dragOverKey, setDragOverKey] = React.useState<string | null>(null);
  const initialOrderRef = React.useRef<string>("");

  const groups = React.useMemo(() => buildModelOrderGroups(models), [models]);
  const selectedGroup = groups.find((group) => group.vendorKey === selectedVendorKey) ?? groups[0] ?? null;

  const loadModels = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(toastT("sessionExpired"), { description: toastT("signInAgain") });
        return;
      }
      const results = await listAllAdminPages((options) =>
        listAdminLLMModels(token, {
          ...options,
          onlyAvailable: true,
          sort: "sortOrder_asc",
        }),
      );
      setModels(results);
      initialOrderRef.current = results.map((item) => item.id).join(",");
      setDirty(false);
      const nextGroups = buildModelOrderGroups(results);
      setSelectedVendorKey((current) =>
        nextGroups.some((group) => group.vendorKey === current)
          ? current
          : nextGroups[0]?.vendorKey ?? "",
      );
    } catch (error) {
      toast.error(t("loadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setLoading(false);
    }
  }, [t, toastT]);

  React.useEffect(() => {
    if (!open) {
      return;
    }
    void loadModels();
  }, [loadModels, open]);

  const commitGroups = React.useCallback((nextGroups: ModelOrderGroup[]) => {
    const nextModels = flattenGroups(nextGroups);
    setModels(nextModels);
    setDirty(nextModels.map((item) => item.id).join(",") !== initialOrderRef.current);
  }, []);

  const moveVendor = React.useCallback(
    (vendorKey: string, direction: "up" | "down") => {
      const index = groups.findIndex((group) => group.vendorKey === vendorKey);
      const targetIndex = direction === "up" ? index - 1 : index + 1;
      commitGroups(moveAt(groups, index, targetIndex));
    },
    [commitGroups, groups],
  );

  const moveVendorTo = React.useCallback(
    (vendorKey: string, targetVendorKey: string) => {
      if (vendorKey === targetVendorKey) {
        return;
      }
      const index = groups.findIndex((group) => group.vendorKey === vendorKey);
      const targetIndex = groups.findIndex((group) => group.vendorKey === targetVendorKey);
      commitGroups(moveAt(groups, index, targetIndex));
    },
    [commitGroups, groups],
  );

  const moveModel = React.useCallback(
    (modelID: number, direction: "up" | "down") => {
      if (!selectedGroup) {
        return;
      }
      const groupIndex = groups.findIndex((group) => group.vendorKey === selectedGroup.vendorKey);
      const itemIndex = selectedGroup.items.findIndex((item) => item.id === modelID);
      const targetIndex = direction === "up" ? itemIndex - 1 : itemIndex + 1;
      if (groupIndex < 0) {
        return;
      }
      const nextGroups = groups.map((group, index) =>
        index === groupIndex
          ? {
              ...group,
              items: moveAt(group.items, itemIndex, targetIndex),
            }
          : group,
      );
      commitGroups(nextGroups);
    },
    [commitGroups, groups, selectedGroup],
  );

  const moveModelTo = React.useCallback(
    (modelID: number, targetModelID: number) => {
      if (!selectedGroup || modelID === targetModelID) {
        return;
      }
      const groupIndex = groups.findIndex((group) => group.vendorKey === selectedGroup.vendorKey);
      const itemIndex = selectedGroup.items.findIndex((item) => item.id === modelID);
      const targetIndex = selectedGroup.items.findIndex((item) => item.id === targetModelID);
      if (groupIndex < 0) {
        return;
      }
      const nextGroups = groups.map((group, index) =>
        index === groupIndex
          ? {
              ...group,
              items: moveAt(group.items, itemIndex, targetIndex),
            }
          : group,
      );
      commitGroups(nextGroups);
    },
    [commitGroups, groups, selectedGroup],
  );

  const clearDragState = React.useCallback(() => {
    setDraggingKey(null);
    setDragOverKey(null);
  }, []);

  const startDrag = React.useCallback((event: React.DragEvent<HTMLButtonElement>, key: string) => {
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("text/plain", key);
    setDraggingKey(key);
  }, []);

  const handleSave = React.useCallback(async () => {
    if (!dirty || saving || models.length === 0) {
      return;
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(toastT("sessionExpired"), { description: toastT("signInAgain") });
        return;
      }
      await reorderAdminLLMModels(token, { modelIDs: models.map((model) => model.id) });
      initialOrderRef.current = models.map((model) => model.id).join(",");
      setDirty(false);
      invalidateAdminReferenceDataCache();
      toast.success(t("saveSuccess"));
      onSaved();
      onClose();
    } catch (error) {
      toast.error(t("saveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }, [dirty, models, onClose, onSaved, saving, t, toastT]);

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
          ) : models.length === 0 ? (
            <div className="flex h-full min-h-[22rem] items-center justify-center text-xs text-muted-foreground">
              {t("empty")}
            </div>
          ) : (
            <div className="grid h-full min-h-0 grid-cols-1 gap-3 lg:grid-cols-[230px_minmax(0,1fr)]">
              <section className="flex min-h-0 flex-col overflow-hidden rounded-lg border bg-background">
                <div className="flex h-9 shrink-0 items-center justify-between gap-3 border-b px-3">
                  <span className="text-xs font-medium text-foreground">{t("vendorHeader")}</span>
                  <span className="text-[11px] text-muted-foreground">{t("itemCount", { count: groups.length })}</span>
                </div>
                <div className="min-h-0 flex-1 overflow-y-auto p-1">
                  <div className="space-y-0.5">
                    {groups.map((group) => {
                      const selected = group.vendorKey === selectedGroup?.vendorKey;
                      const iconURL = resolveLobeHubIconURL(group.icon);
                      const dragKey = dragVendorKey(group.vendorKey);
                      return (
                        <div
                          key={group.vendorKey}
                          className={cn(
                            "group flex min-h-8 w-full items-center gap-0.5 rounded-md px-1 py-1 transition-[background-color,box-shadow,opacity]",
                            selected
                              ? "bg-accent text-accent-foreground"
                              : "text-muted-foreground hover:bg-accent/70 hover:text-accent-foreground",
                            draggingKey === dragKey && "opacity-45",
                            dragOverKey === dragKey && draggingKey !== dragKey && "ring-1 ring-ring/40",
                          )}
                          onDragOver={(event) => {
                            if (!draggingKey?.startsWith("vendor:")) {
                              return;
                            }
                            event.preventDefault();
                            event.dataTransfer.dropEffect = "move";
                            setDragOverKey(dragKey);
                          }}
                          onDrop={(event) => {
                            event.preventDefault();
                            const raw = event.dataTransfer.getData("text/plain") || draggingKey || "";
                            if (raw.startsWith("vendor:")) {
                              moveVendorTo(raw.slice("vendor:".length), group.vendorKey);
                            }
                            clearDragState();
                          }}
                        >
                          <DragHandle
                            label={t("dragVendor", { name: group.label })}
                            disabled={saving}
                            onDragStart={(event) => startDrag(event, dragKey)}
                            onDragEnd={clearDragState}
                            onKeyDown={(event) => {
                              if (event.key === "ArrowUp") {
                                event.preventDefault();
                                moveVendor(group.vendorKey, "up");
                              } else if (event.key === "ArrowDown") {
                                event.preventDefault();
                                moveVendor(group.vendorKey, "down");
                              }
                            }}
                          />
                          <button
                            type="button"
                            className="flex min-w-0 flex-1 items-center gap-1.5 rounded-sm px-1 text-left"
                            onClick={() => setSelectedVendorKey(group.vendorKey)}
                          >
                            {iconURL ? <LobeHubIcon iconUrl={iconURL} label={group.label} size={14} /> : null}
                            <span className="min-w-0 flex-1 truncate text-xs font-medium">{group.label}</span>
                            <span className="shrink-0 text-[11px] text-muted-foreground">{group.items.length}</span>
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
                      {t("modelHeader")}
                    </span>
                    {selectedGroup ? (
                      <span className="truncate text-[11px] text-muted-foreground">
                        {selectedGroup.label}
                      </span>
                    ) : null}
                  </div>
                  {selectedGroup ? (
                    <span className="shrink-0 text-[11px] text-muted-foreground">
                      {t("itemCount", { count: selectedGroup.items.length })}
                    </span>
                  ) : null}
                </div>

                <div className="min-h-0 flex-1 overflow-y-auto p-1">
                  {selectedGroup ? (
                    <div className="space-y-0.5">
                      {selectedGroup.items.map((model) => {
                        const identity = resolveModelIdentity({
                          code: model.platformModelName,
                          vendor: model.vendor,
                          icon: model.icon,
                        });
                        const iconURL = resolveLobeHubIconURL(identity.modelIcon);
                        const dragKey = dragModelKey(selectedGroup.vendorKey, model.id);
                        return (
                          <div
                            key={model.id}
                            className={cn(
                              "flex min-h-8 items-center gap-1.5 rounded-md px-1.5 py-1 text-left transition-[background-color,box-shadow,opacity] hover:bg-accent/55",
                              draggingKey === dragKey && "opacity-45",
                              dragOverKey === dragKey && draggingKey !== dragKey && "ring-1 ring-ring/40",
                            )}
                            onDragOver={(event) => {
                              if (!draggingKey?.startsWith(`model:${selectedGroup.vendorKey}:`)) {
                                return;
                              }
                              event.preventDefault();
                              event.dataTransfer.dropEffect = "move";
                              setDragOverKey(dragKey);
                            }}
                            onDrop={(event) => {
                              event.preventDefault();
                              const raw = event.dataTransfer.getData("text/plain") || draggingKey || "";
                              const modelID = parseDraggedModelID(raw, selectedGroup.vendorKey);
                              if (modelID !== null) {
                                moveModelTo(modelID, model.id);
                              }
                              clearDragState();
                            }}
                          >
                            <DragHandle
                              label={t("dragModel", { name: model.platformModelName })}
                              disabled={saving}
                              onDragStart={(event) => startDrag(event, dragKey)}
                              onDragEnd={clearDragState}
                              onKeyDown={(event) => {
                                if (event.key === "ArrowUp") {
                                  event.preventDefault();
                                  moveModel(model.id, "up");
                                } else if (event.key === "ArrowDown") {
                                  event.preventDefault();
                                  moveModel(model.id, "down");
                                }
                              }}
                            />
                            <LobeHubIcon iconUrl={iconURL} label={model.platformModelName} size={14} />
                            <span className="min-w-0 flex-1 truncate text-xs font-medium text-foreground">
                              {model.platformModelName}
                            </span>
                            <div className="ml-auto flex max-w-[45%] shrink-0 items-center justify-end overflow-hidden">
                              <KindBadges kindsJSON={model.kindsJSON} />
                            </div>
                          </div>
                        );
                      })}
                    </div>
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
