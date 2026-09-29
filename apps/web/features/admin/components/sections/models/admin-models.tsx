"use client";

import { Building2, Cable, Check, ChevronDownIcon, Layers3, ListOrdered, Maximize2, Plus, Tags, ToggleLeft, Trash2 } from "lucide-react";
import dynamic from "next/dynamic";
import { useLocale, useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

import { TablePagination, TableToolbar } from "@/components/ui/table-tools";
import { AdminBulkConfirmDialog } from "@/features/admin/components/shared/bulk-confirm-dialog";
import { useAdminCircuitBreaker } from "@/features/admin/hooks/use-admin-circuit-breaker";
import { useAdminModelsPresentation } from "@/features/admin/hooks/use-admin-models-presentation";
import { useAdminModelsProbe } from "@/features/admin/hooks/use-admin-models-probe";
import { useAdminModels } from "@/features/admin/hooks/use-admin-models";
import {
  isValidModelContextWindow,
  MODEL_CONTEXT_WINDOW_PRESETS,
} from "@/features/admin/model/model-context-window";
import {
  ADAPTER_LABELS,
  MODEL_KIND_OPTIONS,
  MODEL_SORT_OPTIONS,
} from "@/features/admin/types/llm";
import { isAdminLLMAdapter, isAdminLLMStatus } from "@/features/admin/model/admin-unions";
import { cn } from "@/lib/utils";
import { AdminCircuitBreakerControl } from "../../shared/circuit-breaker-control";
import { BulkDeleteModelsDialog, DeleteModelDialog } from "./models-dialog";
import { ModelProbeDialog } from "../../shared/model-probe-dialog";
import { ModelsTable } from "./models-table";

const ModelSheet = dynamic(() => import("./models-sheet").then((module) => module.ModelSheet), {
  ssr: false,
});

const UpstreamSourcesSheet = dynamic(
  () => import("./models-sources-sheet").then((module) => module.UpstreamSourcesSheet),
  {
    ssr: false,
  },
);

const ModelOrderSheet = dynamic(
  () => import("./models-order-sheet").then((module) => module.ModelOrderSheet),
  {
    ssr: false,
  },
);

const ModelPresentationDialog = dynamic(
  () => import("./models-presentation-dialog").then((module) => module.ModelPresentationDialog),
  { ssr: false },
);

type ModelBulkAction = "kinds" | "protocol" | "vendor" | "displayGroup" | "contextWindow" | "status";

function BulkActionControlRow({
  icon,
  label,
  disabled,
  onApply,
  children,
}: {
  icon: React.ReactNode;
  label: string;
  disabled: boolean;
  onApply: () => void;
  children: React.ReactNode;
}) {
  return (
    <div className="flex h-7 w-full items-center gap-1.5">
      <Button
        type="button"
        variant="ghost"
        className="h-7 w-16 shrink-0 justify-start gap-2 px-2 text-[11px] text-foreground/70 shadow-none hover:bg-muted hover:text-foreground"
        onClick={onApply}
        disabled={disabled}
      >
        {icon}
        {label}
      </Button>
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}

function KindsDropdown({
  value,
  onChange,
  disabled,
}: {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}) {
  const t = useTranslations("adminModels");
  const selectedKinds = React.useMemo(
    () => value.split(",").map((item) => item.trim()).filter(Boolean),
    [value],
  );
  const selectedKindLabel = React.useMemo(
    () => selectedKinds.map((kind) => t(`kinds.${kind}`)).join(", "),
    [selectedKinds, t],
  );

  function toggle(kind: string) {
    const next = new Set(selectedKinds);
    if (next.has(kind)) {
      next.delete(kind);
    } else {
      next.add(kind);
    }
    if (next.size === 0) {
      next.add("chat");
    }
    onChange(Array.from(next).join(","));
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          size="sm"
          role="combobox"
          disabled={disabled}
          className="h-7 w-full justify-between gap-2 border-input/40 bg-transparent px-2 py-0 text-[11px] font-normal text-muted-foreground shadow-none hover:bg-transparent focus-visible:border-ring/60 focus-visible:ring-[1px] focus-visible:ring-ring/40 has-[>svg]:px-2"
        >
          <span className={cn("min-w-0 flex-1 truncate text-left", selectedKindLabel ? "text-foreground/75" : "")}>
            {selectedKindLabel || t("fields.kind")}
          </span>
          <ChevronDownIcon className="size-3 shrink-0 text-muted-foreground opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-48 p-1">
        {MODEL_KIND_OPTIONS.map(({ value: kind }) => (
          <button
            key={kind}
            type="button"
            onClick={() => toggle(kind)}
            className="relative flex w-full items-center rounded-sm py-1.5 pr-8 pl-2 text-xs font-normal hover:bg-accent"
          >
            <span className="min-w-0 flex-1 truncate text-left">{t(`kinds.${kind}`)}</span>
            <Check
              className={cn(
                "absolute right-2 size-4 shrink-0 text-muted-foreground",
                selectedKinds.includes(kind) ? "opacity-100" : "opacity-0",
              )}
            />
          </button>
        ))}
      </PopoverContent>
    </Popover>
  );
}

function ContextWindowBulkInput({
  value,
  onChange,
  disabled,
}: {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}) {
  const t = useTranslations("adminModels");
  const [presetsOpen, setPresetsOpen] = React.useState(false);
  const parsedValue = Number(value);
  const invalid = value !== "" && !isValidModelContextWindow(parsedValue);

  return (
    <div className="flex h-7 min-w-0 overflow-hidden rounded-md border border-input/40 bg-transparent focus-within:border-ring/60 focus-within:ring-[1px] focus-within:ring-ring/40">
      <Input
        inputMode="numeric"
        value={value}
        placeholder={t("fields.contextWindow")}
        disabled={disabled}
        aria-invalid={invalid}
        aria-label={t("fields.contextWindow")}
        className="h-full min-w-0 flex-1 rounded-none border-0 px-2 text-[11px] tabular-nums shadow-none focus-visible:ring-0"
        onChange={(event) => onChange(event.target.value.replace(/\D/g, "").slice(0, 8))}
        onBlur={() => {
          if (invalid) {
            toast.error(t("toast.bulkContextWindowInvalid"));
          }
        }}
      />
      <Popover open={presetsOpen} onOpenChange={setPresetsOpen}>
        <PopoverTrigger asChild>
          <button
            type="button"
            disabled={disabled}
            className="inline-flex h-full w-7 shrink-0 items-center justify-center border-l border-border/50 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground disabled:pointer-events-none disabled:opacity-50"
            aria-label={t("sheet.contextWindowPresets")}
          >
            <ChevronDownIcon className="size-3.5" aria-hidden="true" />
          </button>
        </PopoverTrigger>
        <PopoverContent align="end" className="z-[100] w-32 p-1">
          {MODEL_CONTEXT_WINDOW_PRESETS.map((preset) => (
            <button
              key={preset.value}
              type="button"
              className="relative flex w-full items-center rounded-sm py-1.5 pr-8 pl-2 text-xs font-normal hover:bg-accent"
              onClick={() => {
                onChange(String(preset.value));
                setPresetsOpen(false);
              }}
            >
              <span>{preset.label}</span>
              <Check
                className={cn(
                  "absolute right-2 size-4 shrink-0 text-muted-foreground",
                  parsedValue === preset.value ? "opacity-100" : "opacity-0",
                )}
              />
            </button>
          ))}
        </PopoverContent>
      </Popover>
    </div>
  );
}

export function AdminModelsPage() {
  const t = useTranslations("adminModels");
  const locale = useLocale();
  const models = useAdminModels();
  const circuitBreaker = useAdminCircuitBreaker();
  const presentation = useAdminModelsPresentation();
  const [createOpen, setCreateOpen] = React.useState(false);
  const [orderOpen, setOrderOpen] = React.useState(false);
  const [presentationOpen, setPresentationOpen] = React.useState(false);
  const [bulkConfirmAction, setBulkConfirmAction] = React.useState<ModelBulkAction | null>(null);
  const {
    probeOpen,
    setProbeOpen,
    probeLoading,
    probeTargetName,
    probeResults,
    testModel: handleTestModel,
    testSource: handleTestSource,
    deleteProbeRoute: handleDeleteProbeRoute,
  } = useAdminModelsProbe({
    onRouteDeleted: () => {
      void models.loadModels(models.page, models.pageSize);
    },
  });

  const bulkConfirmOpen = bulkConfirmAction !== null;

  function handleConfirmBulkAction() {
    switch (bulkConfirmAction) {
      case "kinds":
        void models.handleBulkApplyKinds().then(() => setBulkConfirmAction(null));
        break;
      case "protocol":
        void models.handleBulkApplyProtocol().then(() => setBulkConfirmAction(null));
        break;
      case "vendor":
        void models.handleBulkApplyVendor().then(() => setBulkConfirmAction(null));
        break;
      case "displayGroup":
        void models.handleBulkApplyDisplayGroup().then(() => setBulkConfirmAction(null));
        break;
      case "contextWindow":
        void models.handleBulkApplyContextWindow().then(() => setBulkConfirmAction(null));
        break;
      case "status":
        void models.handleBulkApplyStatus().then(() => setBulkConfirmAction(null));
        break;
    }
  }

  return (
    <div className="space-y-3 pb-10">
      <div className="space-y-3">
        <div className="flex h-10 items-center justify-between gap-3 px-1">
          <h3 className="text-sm font-semibold">{t("pageTitle")}</h3>
          <AdminCircuitBreakerControl
            available={circuitBreaker.available}
            enabled={circuitBreaker.enabled}
            loading={circuitBreaker.loading}
            saving={circuitBreaker.saving}
            onEnabledChange={(checked) => {
              void circuitBreaker.updateEnabled(checked).then((updated) => {
                if (updated) void models.loadModels(models.page, models.pageSize);
              });
            }}
          />
        </div>

        <TableToolbar
          query={models.query}
          onQueryChange={models.setQuery}
          queryPlaceholder={t("table.searchPlaceholder")}
          filters={[
            {
              key: "status",
              label: t("fields.status"),
              value: models.statusFilter,
              onValueChange: models.setStatusFilter,
              options: [
                { label: t("table.allStatus"), value: "" },
                { label: t("status.active"), value: "active" },
                { label: t("status.inactive"), value: "inactive" },
              ],
            },
            {
              key: "protocol",
              label: t("fields.protocol"),
              value: models.protocolFilter,
              onValueChange: models.setProtocolFilter,
              options: [
                { label: t("table.allProtocols"), value: "" },
                ...Object.entries(ADAPTER_LABELS).map(([value, label]) => ({ label, value })),
              ],
            },
            {
              key: "vendor",
              label: t("fields.vendor"),
              value: models.vendorFilter,
              onValueChange: models.setVendorFilter,
              options: [
                { label: t("table.allVendors"), value: "" },
                ...presentation.vendors.map((item) => ({ label: item.name, value: item.key })),
              ],
            },
          ]}
          sort={{
            value: models.sortValue,
            onValueChange: (v) => {
              const sortOption = MODEL_SORT_OPTIONS.find((item) => item.value === v);
              if (sortOption) models.setSortValue(sortOption.value);
            },
            options: MODEL_SORT_OPTIONS.map((item) => ({
              label: t(item.labelKey),
              value: item.value,
            })),
          }}
          selectedCount={models.selectedModels.length}
          bulkContent={
            <div className="space-y-1">
              <BulkActionControlRow
                icon={<Tags className="size-3 stroke-1" />}
                label={t("actions.apply")}
                onApply={() => setBulkConfirmAction("kinds")}
                disabled={models.loading || models.batchApplying || models.selectedModels.length === 0 || !models.batchKindsDisplay}
              >
                <KindsDropdown
                  value={models.batchKindsDisplay}
                  onChange={models.setBatchKindsDisplay}
                  disabled={models.loading || models.batchApplying || models.selectedModels.length === 0}
                />
              </BulkActionControlRow>

              <BulkActionControlRow
                icon={<Layers3 className="size-3 stroke-1" />}
                label={t("actions.apply")}
                onApply={() => setBulkConfirmAction("displayGroup")}
                disabled={models.loading || models.batchApplying || models.selectedModels.length === 0 || !models.batchDisplayGroupID}
              >
                <Select
                  value={models.batchDisplayGroupID || undefined}
                  onValueChange={models.setBatchDisplayGroupID}
                  disabled={models.loading || models.batchApplying || models.selectedModels.length === 0}
                >
                  <SelectTrigger size="xs" className="h-7 px-2 text-[11px] text-muted-foreground">
                    <SelectValue placeholder={t("fields.displayGroup")} />
                  </SelectTrigger>
                  <SelectContent position="popper" align="start" className="z-[100]" viewportClassName="max-h-[220px]">
                    <SelectItem value="0" className="text-[11px]">
                      {t("presentation.followVendor")}
                    </SelectItem>
                    {presentation.displayGroups.map((item) => (
                      <SelectItem key={item.id} value={String(item.id)} className="text-[11px]">
                        {item.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </BulkActionControlRow>

              <BulkActionControlRow
                icon={<Cable className="size-3 stroke-1" />}
                label={t("actions.apply")}
                onApply={() => setBulkConfirmAction("protocol")}
                disabled={models.loading || models.batchApplying || models.selectedModels.length === 0 || !models.batchProtocol}
              >
                <Select
                  value={models.batchProtocol || undefined}
                  onValueChange={(value) => {
                    if (isAdminLLMAdapter(value)) models.setBatchProtocol(value);
                  }}
                  disabled={models.loading || models.batchApplying || models.selectedModels.length === 0}
                >
                  <SelectTrigger size="xs" className="h-7 px-2 text-[11px] text-muted-foreground">
                    <SelectValue placeholder={t("fields.protocol")} />
                  </SelectTrigger>
                  <SelectContent position="popper" align="start" className="z-[100]" viewportClassName="max-h-[220px]">
                    {Object.entries(ADAPTER_LABELS).map(([value, label]) => (
                      <SelectItem key={value} value={value} className="text-[11px]">
                        {label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </BulkActionControlRow>

              <BulkActionControlRow
                icon={<Maximize2 className="size-3 stroke-1" />}
                label={t("actions.apply")}
                onApply={() => setBulkConfirmAction("contextWindow")}
                disabled={
                  models.loading
                  || models.batchApplying
                  || models.selectedModels.length === 0
                  || !isValidModelContextWindow(Number(models.batchContextWindow))
                }
              >
                <ContextWindowBulkInput
                  value={models.batchContextWindow}
                  onChange={models.setBatchContextWindow}
                  disabled={models.loading || models.batchApplying || models.selectedModels.length === 0}
                />
              </BulkActionControlRow>

              <BulkActionControlRow
                icon={<Building2 className="size-3 stroke-1" />}
                label={t("actions.apply")}
                onApply={() => setBulkConfirmAction("vendor")}
                disabled={models.loading || models.batchApplying || models.selectedModels.length === 0 || !models.batchVendor}
              >
                <Select
                  value={models.batchVendor || undefined}
                  onValueChange={models.setBatchVendor}
                  disabled={models.loading || models.batchApplying || models.selectedModels.length === 0}
                >
                  <SelectTrigger size="xs" className="h-7 px-2 text-[11px] text-muted-foreground">
                    <SelectValue placeholder={t("fields.vendor")} />
                  </SelectTrigger>
                  <SelectContent position="popper" align="start" className="z-[100]" viewportClassName="max-h-[220px]">
                    {presentation.vendors.map((item) => (
                      <SelectItem key={item.key} value={item.key} className="text-[11px]">
                        {item.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </BulkActionControlRow>

              <BulkActionControlRow
                icon={<ToggleLeft className="size-3 stroke-1" />}
                label={t("actions.apply")}
                onApply={() => setBulkConfirmAction("status")}
                disabled={models.loading || models.batchApplying || models.selectedModels.length === 0 || !models.batchStatus}
              >
                <Select
                  value={models.batchStatus || undefined}
                  onValueChange={(value) => {
                    if (isAdminLLMStatus(value)) models.setBatchStatus(value);
                  }}
                  disabled={models.loading || models.batchApplying || models.selectedModels.length === 0}
                >
                  <SelectTrigger size="xs" className="h-7 px-2 text-[11px] text-muted-foreground">
                    <SelectValue placeholder={t("fields.status")} />
                  </SelectTrigger>
                  <SelectContent position="popper" align="start" className="z-[100]">
                    {(["active", "inactive"] as const).map((status) => (
                      <SelectItem key={status} value={status} className="text-[11px]">
                        {t(`status.${status}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </BulkActionControlRow>
            </div>
          }
          bulkActions={[
            {
              key: "delete-models",
              label: t("actions.bulkDelete"),
              icon: <Trash2 className="size-3.5 stroke-1" />,
              onClick: models.handleRequestBulkDelete,
            },
          ]}
          loading={models.loading}
          onRefresh={() => void models.loadModels(models.page, models.pageSize)}
        >
          <Button
            type="button"
            size="sm"
            variant="outline"
            className="h-7 gap-1 text-xs"
            onClick={() => setPresentationOpen(true)}
            disabled={presentation.loading || presentation.vendors.length === 0}
          >
            <Layers3 className="size-3.5 stroke-1" />
            {t("actions.managePresentation")}
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            className="h-7 gap-1 text-xs"
            onClick={() => setOrderOpen(true)}
            disabled={models.loading}
          >
            <ListOrdered className="size-3.5 stroke-1" />
            {t("actions.displayOrder")}
          </Button>
          <Button
            type="button"
            size="sm"
            className="h-7 gap-1 text-xs"
            onClick={() => setCreateOpen(true)}
            disabled={models.loading || presentation.loading || presentation.vendors.length === 0}
          >
            <Plus className="size-3.5 stroke-1" />
            {t("actions.create")}
          </Button>
        </TableToolbar>

        <ModelsTable
          items={models.filteredItems}
          loading={models.loading}
          batchApplying={models.batchApplying}
          circuitBreakerEnabled={circuitBreaker.enabled}
          selectedModelIDs={models.selectedModelIDs}
          onSelectedModelIDsChange={models.setSelectedModelIDs}
          onEdit={models.setEditTarget}
          onViewSources={models.setSourcesModel}
          pendingModelIDs={models.pendingModelIDs}
          onToggleStatus={(item, status) => void models.handleToggleStatus(item, status)}
          onToggleAccessScope={(item, scope) => void models.handleToggleAccessScope(item, scope)}
          onDelete={models.setDeleteTarget}
          onTestModel={handleTestModel}
          onTestSource={handleTestSource}
          onRefreshModels={() => void models.loadModels(models.page, models.pageSize)}
          onSourceAvailabilityChange={models.handleSourceAvailabilityChange}
          onSourceDeleteChange={models.handleSourceDeleteChange}
        />

        <TablePagination
          total={models.total}
          page={models.page}
          pageCount={models.pageCount}
          pageSize={models.pageSize}
          onPageChange={(nextPage) => void models.loadModels(nextPage, models.pageSize)}
          onPageSizeChange={(nextPageSize) => void models.loadModels(1, nextPageSize)}
          loading={models.loading}
        />
      </div>

      {createOpen || models.editTarget ? (
        <ModelSheet
          open
          mode={createOpen ? "create" : "edit"}
          target={models.editTarget}
          models={models.items}
          vendors={presentation.vendors}
          displayGroups={presentation.displayGroups}
          onClose={() => {
            setCreateOpen(false);
            models.setEditTarget(null);
          }}
          onSuccess={() => void models.loadModels(models.page, models.pageSize)}
        />
      ) : null}

      {presentationOpen ? (
        <ModelPresentationDialog
          open
          vendors={presentation.vendors}
          displayGroups={presentation.displayGroups}
          onClose={() => setPresentationOpen(false)}
          onChanged={async () => {
            await presentation.reload();
            await models.loadModels(models.page, models.pageSize);
          }}
        />
      ) : null}

      {orderOpen ? (
        <ModelOrderSheet
          open
          onClose={() => setOrderOpen(false)}
          onSaved={() => {
            models.setSortValue("sortOrder_asc");
            if (models.sortValue === "sortOrder_asc") {
              void models.loadModels(models.page, models.pageSize);
            }
          }}
        />
      ) : null}

      {/* Delete Dialog */}
      <DeleteModelDialog
        target={models.deleteTarget}
        onClose={() => models.setDeleteTarget(null)}
        onDeleted={models.handleDeleted}
      />

      <BulkDeleteModelsDialog
        open={models.bulkDeleteTargets.length > 0}
        targets={models.bulkDeleteTargets}
        onClose={models.closeBulkDelete}
        onDeleted={models.handleBulkDeleted}
      />

      {models.sourcesModel ? (
        <UpstreamSourcesSheet
          model={models.sourcesModel}
          circuitBreakerEnabled={circuitBreaker.enabled}
          onClose={() => models.setSourcesModel(null)}
          onRefreshModel={() => void models.loadModels(models.page, models.pageSize)}
          onSourceAvailabilityChange={models.handleSourceAvailabilityChange}
        />
      ) : null}

      <AdminBulkConfirmDialog
        open={bulkConfirmOpen}
        onOpenChange={(open) => {
          if (!open && !models.batchApplying) {
            setBulkConfirmAction(null);
          }
        }}
        pending={models.batchApplying}
        title={t("bulkConfirm.title")}
        description={bulkConfirmAction === "contextWindow"
          ? t("bulkConfirm.contextWindowDescription", {
            count: models.selectedModels.length,
            value: new Intl.NumberFormat(locale).format(Number(models.batchContextWindow)),
          })
          : t("bulkConfirm.description", { count: models.selectedModels.length })}
        confirmLabel={t("bulkConfirm.confirm")}
        pendingLabel={t("bulkConfirm.pending")}
        onConfirm={handleConfirmBulkAction}
      />

      <ModelProbeDialog
        open={probeOpen}
        loading={probeLoading}
        targetName={probeTargetName}
        result={null}
        results={probeResults}
        onDeleteRoute={handleDeleteProbeRoute}
        onOpenChange={(open) => {
          if (!open && !probeLoading) {
            setProbeOpen(false);
          }
        }}
      />
    </div>
  );
}
