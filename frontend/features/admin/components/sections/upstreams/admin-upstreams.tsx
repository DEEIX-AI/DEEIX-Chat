"use client";

import * as React from "react";
import dynamic from "next/dynamic";
import { Plus, ToggleLeft, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
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
  UPSTREAM_SORT_OPTIONS,
  useAdminUpstreams,
  type UpstreamSortValue,
} from "@/features/admin/hooks/use-admin-upstreams";
import type { AdminLLMStatus } from "@/features/admin/api/llm.types";
import {
  BulkDeleteUpstreamsDialog,
  CircuitActionDialog,
  DeleteUpstreamDialog,
} from "./upstream-dialog";
import { UpstreamsTable } from "./upstreams-table";
import { COMPATIBLE_OPTIONS } from "@/features/admin/utils/llm-display";

const UpstreamSheet = dynamic(() => import("./upstream-sheet").then((module) => module.UpstreamSheet), {
  ssr: false,
});

const UpstreamModelsDialog = dynamic(
  () => import("./upstream-models-dialog").then((module) => module.UpstreamModelsDialog),
  {
    ssr: false,
  },
);

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

export function AdminUpstreamsPage() {
  const t = useTranslations("adminChannels");
  const upstreams = useAdminUpstreams();
  const [syncOnOpenUpstreamID, setSyncOnOpenUpstreamID] = React.useState<number | null>(null);
  const [statusConfirmOpen, setStatusConfirmOpen] = React.useState(false);

  return (
    <div className="space-y-3 pb-10">
      <div className="flex h-10 items-center px-1">
        <h3 className="text-sm font-semibold">{t("pageTitle")}</h3>
      </div>

      <TableToolbar
        query={upstreams.query}
        onQueryChange={upstreams.setQuery}
        queryPlaceholder={t("table.searchPlaceholder")}
        filters={[
          {
            key: "status",
            label: t("fields.status"),
            value: upstreams.statusFilter,
            onValueChange: upstreams.setStatusFilter,
            options: [
              { label: t("table.allStatus"), value: "" },
              { label: t("status.active"), value: "active" },
              { label: t("status.inactive"), value: "inactive" },
              { label: t("status.circuitOpen"), value: "circuit" },
            ],
          },
          {
            key: "compatible",
            label: t("fields.compatibility"),
            value: upstreams.compatibleFilter,
            onValueChange: upstreams.setCompatibleFilter,
            options: [
              { label: t("table.allCompatibility"), value: "" },
              ...COMPATIBLE_OPTIONS.map((item) => ({
                label: item.value === "custom" ? t("compatible.custom") : item.label,
                value: item.value,
              })),
            ],
          },
        ]}
        sort={{
          value: upstreams.sortValue,
          onValueChange: (v) => upstreams.setSortValue(v as UpstreamSortValue),
          options: UPSTREAM_SORT_OPTIONS.map((o) => ({ label: t(o.labelKey), value: o.value })),
        }}
        selectedCount={upstreams.selected.size}
        bulkContent={
          <div className="space-y-1">
            <BulkActionControlRow
              icon={<ToggleLeft className="size-3 stroke-1" />}
              label={t("actions.apply")}
              onApply={() => setStatusConfirmOpen(true)}
              disabled={upstreams.loading || upstreams.batchApplying || upstreams.selected.size === 0 || !upstreams.batchStatus}
            >
              <Select
                value={upstreams.batchStatus || undefined}
                onValueChange={(value) => upstreams.setBatchStatus(value as AdminLLMStatus)}
                disabled={upstreams.loading || upstreams.batchApplying || upstreams.selected.size === 0}
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
            key: "delete-upstreams",
            label: t("actions.bulkDelete"),
            icon: <Trash2 className="size-3.5 stroke-1" />,
            onClick: upstreams.handleRequestBulkDelete,
          },
        ]}
        loading={upstreams.loading}
        onRefresh={() => void upstreams.load()}
      >
        <Button
          type="button"
          size="sm"
          className="h-7 gap-1 text-xs"
          onClick={upstreams.handleOpenCreate}
          disabled={upstreams.loading}
        >
          <Plus className="size-3.5 stroke-1" />
          {t("actions.create")}
        </Button>
      </TableToolbar>

      <UpstreamsTable
        items={upstreams.pagedItems}
        loading={upstreams.loading}
        selected={upstreams.selected}
        togglingStatusIDs={upstreams.togglingStatusIDs}
        onSelectAll={upstreams.handleSelectAll}
        onSelectOne={upstreams.handleSelectOne}
        onEdit={upstreams.handleEdit}
        onManageModels={upstreams.handleManageModels}
        onSyncModels={(item) => {
          setSyncOnOpenUpstreamID(item.id);
          upstreams.handleManageModels(item);
        }}
        onCircuitAction={upstreams.handleCircuitAction}
        onToggleStatus={(item) => void upstreams.handleToggleStatus(item)}
        onDelete={upstreams.handleDelete}
      />

      <TablePagination
        total={upstreams.total}
        page={upstreams.safePage}
        pageCount={upstreams.pageCount}
        pageSize={upstreams.pageSize}
        onPageChange={upstreams.setPage}
        onPageSizeChange={upstreams.setPageSize}
        loading={upstreams.loading}
      />

      {upstreams.sheetState.open ? (
        <UpstreamSheet
          open
          onOpenChange={(open) => {
            if (!open) upstreams.closeSheet();
          }}
          mode={upstreams.sheetState.mode}
          target={upstreams.sheetState.mode === "edit" ? upstreams.sheetState.target : null}
          onSuccess={upstreams.handleSheetSuccess}
          onManageModels={(item) => {
            upstreams.closeSheet();
            upstreams.handleManageModels(item);
          }}
        />
      ) : null}

      <DeleteUpstreamDialog
        upstream={upstreams.deleteTarget}
        onClose={upstreams.closeDelete}
        onDeleted={upstreams.handleDeleted}
      />

      <BulkDeleteUpstreamsDialog
        open={upstreams.bulkDeleteTargets.length > 0}
        targets={upstreams.bulkDeleteTargets}
        onClose={upstreams.closeBulkDelete}
        onDeleted={upstreams.handleBulkDeleted}
      />

      <CircuitActionDialog
        upstream={upstreams.circuitState.open ? upstreams.circuitState.target : null}
        action={upstreams.circuitState.open ? upstreams.circuitState.action : "open"}
        onClose={upstreams.closeCircuit}
        onDone={upstreams.handleCircuitDone}
      />

      {upstreams.modelsOpen ? (
        <UpstreamModelsDialog
          open
          onOpenChange={(open) => {
            upstreams.setModelsOpen(open);
            if (!open) upstreams.closeModels();
          }}
          upstream={upstreams.modelsTarget}
          openRemoteOnOpen={syncOnOpenUpstreamID === upstreams.modelsTarget?.id}
          onUpstreamUpdated={upstreams.handleUpstreamUpdated}
          onRemoteOpenHandled={() => setSyncOnOpenUpstreamID(null)}
        />
      ) : null}

      <AdminBulkConfirmDialog
        open={statusConfirmOpen}
        onOpenChange={(open) => {
          if (!open && !upstreams.batchApplying) {
            setStatusConfirmOpen(false);
          }
        }}
        pending={upstreams.batchApplying}
        title={t("bulkConfirm.title")}
        description={t("bulkConfirm.description", { count: upstreams.selected.size })}
        confirmLabel={t("bulkConfirm.confirm")}
        pendingLabel={t("bulkConfirm.pending")}
        onConfirm={() => {
          void upstreams.handleBulkApplyStatus().then(() => setStatusConfirmOpen(false));
        }}
      />
    </div>
  );
}
