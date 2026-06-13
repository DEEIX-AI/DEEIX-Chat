"use client";

import * as React from "react";
import dynamic from "next/dynamic";
import { Building2, Cable, Check, ChevronDownIcon, ListOrdered, Plus, Tags, ToggleLeft, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
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
import { AdminBulkConfirmDialog } from "@/features/admin/components/bulk-confirm-dialog";
import {
  deleteAdminLLMUpstreamModel,
  testAdminLLMModelAll,
  testAdminLLMUpstreamModelRoute,
} from "@/features/admin/api";
import { useAdminModels } from "@/features/admin/hooks/use-admin-models";
import { BulkDeleteModelsDialog, DeleteModelDialog } from "./models-dialog";
import { ModelProbeDialog } from "./models-probe-dialog";
import { ModelsTable } from "./models-table";
import {
  ADAPTER_LABELS,
  MODEL_KIND_OPTIONS,
  MODEL_SORT_OPTIONS,
  type ModelSortValue,
} from "@/features/admin/types/llm";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import type {
  AdminLLMAdapter,
  AdminLLMModelDTO,
  AdminLLMModelProbeResult,
  AdminLLMModelUpstreamSourceDTO,
  AdminLLMStatus,
} from "@/features/admin/api/llm.types";
import { KNOWN_VENDOR_OPTIONS } from "@/shared/lib/model-identity";
import { cn } from "@/lib/utils";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

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

type ModelBulkAction = "kinds" | "protocol" | "vendor" | "status";

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

export function AdminModelsPage() {
  const t = useTranslations("adminModels");
  const models = useAdminModels();
  const [createOpen, setCreateOpen] = React.useState(false);
  const [orderOpen, setOrderOpen] = React.useState(false);
  const [bulkConfirmAction, setBulkConfirmAction] = React.useState<ModelBulkAction | null>(null);
  const [probeOpen, setProbeOpen] = React.useState(false);
  const [probeLoading, setProbeLoading] = React.useState(false);
  const [probeTargetName, setProbeTargetName] = React.useState("");
  const [probeResults, setProbeResults] = React.useState<AdminLLMModelProbeResult[]>([]);

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
      case "status":
        void models.handleBulkApplyStatus().then(() => setBulkConfirmAction(null));
        break;
    }
  }

  async function runProbe(
    targetName: string,
    loader: (token: string) => Promise<AdminLLMModelProbeResult | AdminLLMModelProbeResult[]>,
  ) {
    setProbeTargetName(targetName);
    setProbeResults([]);
    setProbeOpen(true);
    setProbeLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        setProbeOpen(false);
        return;
      }
      const data = await loader(token);
      setProbeResults(Array.isArray(data) ? data : [data]);
    } catch (error) {
      toast.error(t("toast.operationFailed"), { description: resolveAdminErrorMessage(error) });
      setProbeOpen(false);
    } finally {
      setProbeLoading(false);
    }
  }

  function handleTestModel(item: AdminLLMModelDTO) {
    void runProbe(item.platformModelName, async (token) => (await testAdminLLMModelAll(token, item.id)).results);
  }

  function handleTestSource(source: AdminLLMModelUpstreamSourceDTO) {
    const targetName = `${source.upstreamName} / ${source.upstreamModelName}`;
    void runProbe(targetName, (token) => testAdminLLMUpstreamModelRoute(token, source.upstreamID, source.id));
  }

  async function handleDeleteProbeRoute(result: AdminLLMModelProbeResult) {
    const token = await resolveAccessToken();
    if (!token) {
      toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
      throw new Error("session expired");
    }
    try {
      await deleteAdminLLMUpstreamModel(token, result.upstreamID, result.routeID);
      const nextResults = probeResults.filter((item) => item.routeID !== result.routeID);
      setProbeResults(nextResults);
      if (nextResults.length === 0) {
        setProbeOpen(false);
      }
      toast.success(t("toast.sourceDeleted"));
      void models.loadModels(models.page, models.pageSize);
    } catch (error) {
      toast.error(t("toast.sourceDeleteFailed"), { description: resolveAdminErrorMessage(error) });
      throw error;
    }
  }

  return (
    <div className="space-y-3 pb-10">
      <div className="space-y-3">
        <div className="flex h-10 items-center px-1">
          <h3 className="text-sm font-semibold">{t("pageTitle")}</h3>
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
                ...KNOWN_VENDOR_OPTIONS.map(({ label, value }) => ({ label, value })),
              ],
            },
          ]}
          sort={{
            value: models.sortValue,
            onValueChange: (v) => models.setSortValue(v as ModelSortValue),
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
                icon={<Cable className="size-3 stroke-1" />}
                label={t("actions.apply")}
                onApply={() => setBulkConfirmAction("protocol")}
                disabled={models.loading || models.batchApplying || models.selectedModels.length === 0 || !models.batchProtocol}
              >
                <Select
                  value={models.batchProtocol || undefined}
                  onValueChange={(value) => models.setBatchProtocol(value as AdminLLMAdapter)}
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
                    {KNOWN_VENDOR_OPTIONS.map(({ label, value }) => (
                      <SelectItem key={value} value={value} className="text-[11px]">
                        {label}
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
                  onValueChange={(value) => models.setBatchStatus(value as AdminLLMStatus)}
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
            disabled={models.loading}
          >
            <Plus className="size-3.5 stroke-1" />
            {t("actions.create")}
          </Button>
        </TableToolbar>

        <ModelsTable
          items={models.filteredItems}
          loading={models.loading}
          selectedModelIDs={models.selectedModelIDs}
          onSelectedModelIDsChange={models.setSelectedModelIDs}
          onEdit={models.setEditTarget}
          onViewSources={models.setSourcesModel}
          onToggleStatus={(item, status) => void models.handleToggleStatus(item, status)}
          onToggleAccessScope={(item, scope) => void models.handleToggleAccessScope(item, scope)}
          onDelete={models.setDeleteTarget}
          onTestModel={handleTestModel}
          onTestSource={handleTestSource}
          onSourceStatusChange={models.handleSourceStatusChange}
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
          onClose={() => {
            setCreateOpen(false);
            models.setEditTarget(null);
          }}
          onSuccess={() => void models.loadModels(models.page, models.pageSize)}
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
          onClose={() => models.setSourcesModel(null)}
          onRefreshModel={() => void models.loadModels(models.page, models.pageSize)}
          onSourceStatusChange={models.handleSourceStatusChange}
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
        description={t("bulkConfirm.description", { count: models.selectedModels.length })}
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
