"use client";

import { CircleHelp, PencilLine, Plus, Search, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from "@/components/ui/input-group";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useVirtualTableRows } from "@/components/ui/virtual-table";
import { cn } from "@/lib/utils";
import type {
  AdminLLMModelDisplayGroupDTO,
  AdminLLMModelVendorDTO,
} from "@/features/admin/api/llm.types";
import { AdminBulkConfirmDialog } from "@/features/admin/components/bulk-confirm-dialog";
import { ModelIconField } from "@/features/admin/components/sections/models/model-icon-field";
import {
  type PresentationTab,
  useAdminPresentationEditor,
} from "@/features/admin/hooks/use-admin-presentation-editor";
import { ModelIcon } from "@/shared/components/model-icon";
import { resolveModelIconURL } from "@/shared/lib/model-identity";

function InputHelp({ help }: { help: string }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <InputGroupButton
          size="icon-xs"
          className="size-5 text-muted-foreground hover:bg-transparent hover:text-foreground"
          aria-label={help}
        >
          <CircleHelp className="size-3 stroke-1.5" />
        </InputGroupButton>
      </TooltipTrigger>
      <TooltipContent className="max-w-64">{help}</TooltipContent>
    </Tooltip>
  );
}

function PresentationIcon({ icon, label }: { icon: string; label: string }) {
  return (
    <span className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border/40 bg-muted/40">
      <ModelIcon iconUrl={resolveModelIconURL(icon)} label={label} size={18} />
    </span>
  );
}

function PresentationActionButton({
  label,
  destructive = false,
  disabled = false,
  onClick,
  children,
}: {
  label: string;
  destructive?: boolean;
  disabled?: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          disabled={disabled}
          className={cn(
            "text-muted-foreground/75 hover:bg-background/80 hover:text-foreground",
            destructive && "hover:bg-destructive/10 hover:text-destructive",
          )}
          aria-label={label}
          onClick={onClick}
        >
          {children}
        </Button>
      </TooltipTrigger>
      <TooltipContent side="top">{label}</TooltipContent>
    </Tooltip>
  );
}

function DialogLayerTransition({
  editorOpen,
  editorLayer,
  listLayer,
}: {
  editorOpen: boolean;
  editorLayer: React.ReactNode;
  listLayer: React.ReactNode;
}) {
  const editorRef = React.useRef<HTMLDivElement>(null);
  const listRef = React.useRef<HTMLDivElement>(null);
  const [height, setHeight] = React.useState<number | null>(null);

  const measureActiveLayer = React.useCallback(() => {
    const activeLayer = editorOpen ? editorRef.current : listRef.current;
    if (!activeLayer) {
      return;
    }
    // Dialog 首次打开带缩放动画；offsetHeight 使用布局高度，避免把 zoom-in-95
    // 的视觉缩放误当成真实高度，导致初次打开时底部操作栏被裁掉。
    const nextHeight = activeLayer.offsetHeight;
    setHeight((current) => current === nextHeight ? current : nextHeight);
  }, [editorOpen]);

  // 两层保持挂载并分别测量，外框才能在列表层与编辑层之间执行真实的高度插值。
  React.useLayoutEffect(() => {
    measureActiveLayer();
    if (typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(measureActiveLayer);
    if (editorRef.current) {
      observer.observe(editorRef.current);
    }
    if (listRef.current) {
      observer.observe(listRef.current);
    }
    return () => observer.disconnect();
  }, [measureActiveLayer]);

  return (
    <div
      className="relative min-h-0 overflow-hidden transition-[height] duration-200 ease-out motion-reduce:transition-none"
      style={height === null ? undefined : { height }}
    >
      <div
        ref={editorRef}
        aria-hidden={!editorOpen}
        inert={!editorOpen}
        className={cn(
          "flex max-h-[min(86vh,760px)] min-h-0 flex-col overflow-hidden transition-opacity duration-150",
          editorOpen ? "relative opacity-100" : "pointer-events-none absolute inset-x-0 top-0 opacity-0",
        )}
      >
        {editorLayer}
      </div>
      <div
        ref={listRef}
        aria-hidden={editorOpen}
        inert={editorOpen}
        className={cn(
          "flex max-h-[min(86vh,760px)] min-h-0 flex-col overflow-hidden transition-opacity duration-150",
          editorOpen ? "pointer-events-none absolute inset-x-0 top-0 opacity-0" : "relative opacity-100",
        )}
      >
        {listLayer}
      </div>
    </div>
  );
}

export function ModelPresentationDialog({
  open,
  vendors,
  displayGroups,
  onClose,
  onChanged,
}: {
  open: boolean;
  vendors: AdminLLMModelVendorDTO[];
  displayGroups: AdminLLMModelDisplayGroupDTO[];
  onClose: () => void;
  onChanged: () => Promise<void>;
}) {
  const t = useTranslations("adminModels.presentation");
  const commonT = useTranslations("common.actions");
  const [tab, setTab] = React.useState<PresentationTab>("vendors");
  const {
    editor,
    setEditor,
    stableEditor,
    pending,
    deleteTarget,
    setDeleteTarget,
    catalogModels,
    modelsLoading,
    modelQuery,
    setModelQuery,
    loadCatalogModels,
    closeDialog,
    openCreate,
    openVendorEdit,
    openGroupEdit,
    toggleEditorModel,
    saveEditor,
    confirmDelete,
  } = useAdminPresentationEditor({ onChanged, onClose });
  const keyInputID = React.useId();
  const nameInputID = React.useId();
  const iconInputID = React.useId();
  const modelQueryInputID = React.useId();
  const [iconUploading, setIconUploading] = React.useState(false);

  const items = tab === "vendors" ? vendors : displayGroups;
  const editorOpen = editor !== null;
  const editorTitle = stableEditor?.kind === "vendors"
    ? stableEditor.creating
      ? t("createVendorTitle")
      : t("editVendorTitle")
    : stableEditor?.creating
      ? t("createGroupTitle")
      : t("editGroupTitle");
  const editorDescription = stableEditor?.kind === "vendors"
    ? t("vendorFormDescription")
    : t("groupFormDescription");
  const deleteTitle = deleteTarget?.kind === "vendors" ? t("deleteVendorTitle") : t("deleteGroupTitle");
  const deleteDescription = deleteTarget?.kind === "vendors"
    ? t("deleteVendorDescription", { name: deleteTarget.name })
    : t("deleteGroupDescription", { name: deleteTarget?.name ?? "" });
  const normalizedModelQuery = modelQuery.trim().toLowerCase();
  const filteredCatalogModels = catalogModels?.filter((model) => {
    if (!normalizedModelQuery) {
      return true;
    }
    return [model.platformModelName, model.vendorName, model.displayGroupName]
      .some((value) => value?.toLowerCase().includes(normalizedModelQuery));
  }) ?? [];
  const selectedModelIDs = React.useMemo(
    () => new Set(stableEditor?.modelIDs ?? []),
    [stableEditor?.modelIDs],
  );
  const memberRows = useVirtualTableRows(filteredCatalogModels, {
    estimateSize: 48,
    maxHeight: 224,
  });

  return (
    <>
      <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && closeDialog()}>
        <DialogContent className="w-[calc(100vw-2rem)] gap-0 overflow-hidden p-0 sm:max-w-[560px]">
          <DialogLayerTransition
            editorOpen={editorOpen}
            editorLayer={stableEditor ? (
              <>
                <DialogHeader className="shrink-0 px-4 py-4">
                  <DialogTitle>{editorTitle}</DialogTitle>
                  <DialogDescription>{editorDescription}</DialogDescription>
                </DialogHeader>

                <form
                  className="flex min-h-0 flex-1 flex-col"
                  onSubmit={(event) => {
                    event.preventDefault();
                    if (iconUploading) {
                      return;
                    }
                    void saveEditor();
                  }}
                >
                  <div className="min-h-0 flex-1 overflow-y-auto px-4 py-2">
                    <div className="grid gap-4 sm:grid-cols-2">
                      {stableEditor.kind === "vendors" ? (
                        <div className="space-y-1">
                          <Label htmlFor={keyInputID} className="text-xs font-normal text-muted-foreground">
                            {t("key")}
                          </Label>
                          <InputGroup>
                            <InputGroupInput
                              id={keyInputID}
                              value={stableEditor.key}
                              disabled={!stableEditor.creating || pending}
                              placeholder={t("keyPlaceholder")}
                              required
                              onChange={(event) => setEditor((current) => current ? { ...current, key: event.target.value } : current)}
                            />
                            <InputGroupAddon align="inline-end">
                              <InputHelp help={t("keyHelp")} />
                            </InputGroupAddon>
                          </InputGroup>
                        </div>
                      ) : null}

                      <div className="space-y-1">
                        <Label htmlFor={nameInputID} className="text-xs font-normal text-muted-foreground">
                          {t("name")}
                        </Label>
                        <Input
                          id={nameInputID}
                          value={stableEditor.name}
                          disabled={pending}
                          placeholder={t("namePlaceholder")}
                          autoFocus
                          required
                          onChange={(event) => setEditor((current) => current ? { ...current, name: event.target.value } : current)}
                        />
                      </div>

                      <div className={stableEditor.kind === "vendors" ? "space-y-1 sm:col-span-2" : "space-y-1"}>
                        <Label htmlFor={iconInputID} className="text-xs font-normal text-muted-foreground">
                          {t("icon")}
                        </Label>
                        <ModelIconField
                          id={iconInputID}
                          value={stableEditor.icon}
                          disabled={pending}
                          placeholder={t("iconPlaceholder")}
                          help={t("iconHelp")}
                          onChange={(value) => setEditor((current) => current ? { ...current, icon: value } : current)}
                          onUploadingChange={setIconUploading}
                        />
                      </div>
                    </div>

                    {stableEditor.kind === "groups" ? (
                      <div className="mt-4 space-y-2">
                        <div className="flex items-center justify-between gap-3">
                          <Label htmlFor={modelQueryInputID} className="text-xs font-normal text-muted-foreground">
                            {t("members")}
                          </Label>
                          <span className="text-[11px] text-muted-foreground">
                            {t("membersSelected", { count: stableEditor.modelIDs.length })}
                          </span>
                        </div>
                        <div className="relative">
                          <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
                          <Input
                            id={modelQueryInputID}
                            value={modelQuery}
                            disabled={pending || modelsLoading}
                            placeholder={t("memberSearchPlaceholder")}
                            className="pl-8"
                            onChange={(event) => setModelQuery(event.target.value)}
                          />
                        </div>
                        <div
                          ref={memberRows.viewportRef}
                          className="max-h-56 overflow-y-auto rounded-md border"
                        >
                          {modelsLoading ? (
                            <div className="flex min-h-28 items-center justify-center text-muted-foreground">
                              <Spinner className="size-4" />
                            </div>
                          ) : filteredCatalogModels.length === 0 ? (
                            <div className="flex min-h-28 items-center justify-center px-3 py-6 text-center text-xs text-muted-foreground">
                              {t("memberEmpty")}
                            </div>
                          ) : (
                            <div style={{ paddingTop: memberRows.paddingTop, paddingBottom: memberRows.paddingBottom }}>
                              {memberRows.rows.map(({ item: model }) => (
                                <label
                                  key={model.id}
                                  className="flex h-12 cursor-pointer items-center gap-3 border-b px-3 py-2 last:border-b-0 hover:bg-muted/40"
                                >
                                  <Checkbox
                                    checked={selectedModelIDs.has(model.id)}
                                    disabled={pending}
                                    onCheckedChange={(value) => toggleEditorModel(model.id, value === true)}
                                  />
                                  <span className="min-w-0 flex-1">
                                    <span className="block truncate text-xs font-medium">{model.platformModelName}</span>
                                    <span className="block truncate text-[11px] text-muted-foreground">
                                      {model.displayGroupName
                                        ? t("memberCurrentGroup", { group: model.displayGroupName })
                                        : t("memberFollowVendor", { vendor: model.vendorName || model.vendor })}
                                    </span>
                                  </span>
                                </label>
                              ))}
                            </div>
                          )}
                        </div>
                        <p className="text-[11px] leading-4 text-muted-foreground">{t("membersHelp")}</p>
                      </div>
                    ) : null}
                  </div>

                  <DialogFooter className="shrink-0 px-4 py-3">
                    <Button type="button" variant="ghost" disabled={pending} onClick={() => setEditor(null)}>
                      {commonT("cancel")}
                    </Button>
                    <Button type="submit" disabled={pending || iconUploading}>
                      {pending || iconUploading ? commonT("saving") : commonT("save")}
                    </Button>
                  </DialogFooter>
                </form>
              </>
            ) : null}
            listLayer={(
              <>
                <DialogHeader className="shrink-0 px-4 py-4">
                  <DialogTitle>{t("title")}</DialogTitle>
                  <DialogDescription>{t("description")}</DialogDescription>
                </DialogHeader>

                <Tabs
                  value={tab}
                  onValueChange={(value) => {
                    const nextTab = value as PresentationTab;
                    setTab(nextTab);
                    if (nextTab === "groups" && catalogModels === null && !modelsLoading) {
                      void loadCatalogModels();
                    }
                  }}
                  className="flex min-h-0 flex-1 flex-col gap-0"
                >
                  <div className="flex shrink-0 items-center justify-between gap-3 px-4 pb-3">
                    <TabsList>
                      <TabsTrigger value="vendors">{t("vendors")}</TabsTrigger>
                      <TabsTrigger value="groups">{t("groups")}</TabsTrigger>
                    </TabsList>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      className="gap-1.5"
                      disabled={tab === "groups" && (modelsLoading || catalogModels === null)}
                      onClick={() => openCreate(tab)}
                    >
                      <Plus className="size-3.5 stroke-1.5" />
                      {commonT("create")}
                    </Button>
                  </div>

                  <TabsContent value={tab} className="min-h-0 flex-1 overflow-y-auto px-2 py-2">
                    {items.length === 0 ? (
                      <div className="flex min-h-40 items-center justify-center px-3 py-6 text-center text-xs text-muted-foreground">
                        {t("empty")}
                      </div>
                    ) : (
                      <div className="space-y-0.5">
                        {tab === "vendors"
                          ? vendors.map((vendor) => (
                              <div key={vendor.key} className="group flex min-h-11 items-center gap-3 rounded-md px-2 hover:bg-muted/45">
                                <PresentationIcon icon={vendor.icon} label={vendor.name} />
                                <div className="min-w-0 flex-1">
                                  <div className="flex items-center gap-2">
                                    <span className="truncate text-xs font-medium">{vendor.name}</span>
                                    {vendor.builtIn ? <Badge variant="secondary">{t("builtIn")}</Badge> : null}
                                  </div>
                                  <p className="truncate text-[11px] text-muted-foreground">{vendor.key}</p>
                                </div>
                                <div className="grid w-13 shrink-0 grid-cols-2 gap-0.5">
                                  <PresentationActionButton label={commonT("edit")} onClick={() => openVendorEdit(vendor)}>
                                    <PencilLine className="size-3.5 stroke-[1.75]" />
                                  </PresentationActionButton>
                                  {vendor.builtIn ? <span aria-hidden="true" className="size-6" /> : (
                                    <PresentationActionButton
                                      destructive
                                      label={commonT("delete")}
                                      onClick={() => setDeleteTarget({
                                        kind: "vendors", key: vendor.key, name: vendor.name,
                                      })}
                                    >
                                      <Trash2 className="size-3.5 stroke-[1.75]" />
                                    </PresentationActionButton>
                                  )}
                                </div>
                              </div>
                            ))
                          : displayGroups.map((group) => (
                              <div key={group.id} className="group flex min-h-11 items-center gap-3 rounded-md px-2 hover:bg-muted/45">
                                <PresentationIcon icon={group.icon} label={group.name} />
                                <span className="min-w-0 flex-1 truncate text-xs font-medium">{group.name}</span>
                                <div className="grid w-13 shrink-0 grid-cols-2 gap-0.5">
                                  <PresentationActionButton
                                    label={commonT("edit")}
                                    disabled={modelsLoading || catalogModels === null}
                                    onClick={() => openGroupEdit(group)}
                                  >
                                    <PencilLine className="size-3.5 stroke-[1.75]" />
                                  </PresentationActionButton>
                                  <PresentationActionButton
                                    destructive
                                    label={commonT("delete")}
                                    onClick={() => setDeleteTarget({ kind: "groups", id: group.id, name: group.name })}
                                  >
                                    <Trash2 className="size-3.5 stroke-[1.75]" />
                                  </PresentationActionButton>
                                </div>
                              </div>
                            ))}
                      </div>
                    )}
                  </TabsContent>
                </Tabs>

                <DialogFooter className="shrink-0 px-4 py-3">
                  <Button type="button" variant="ghost" disabled={pending} onClick={closeDialog}>
                    {commonT("close")}
                  </Button>
                </DialogFooter>
              </>
            )}
          />
        </DialogContent>
      </Dialog>

      <AdminBulkConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(nextOpen) => !nextOpen && !pending && setDeleteTarget(null)}
        pending={pending}
        title={deleteTitle}
        description={deleteDescription}
        confirmLabel={commonT("delete")}
        pendingLabel={t("deleting")}
        onConfirm={() => void confirmDelete()}
        size="compact"
      />
    </>
  );
}
