"use client";

import * as React from "react";
import { Check, CircleAlert, Copy, Download, History, Pencil, Plus, Trash2, X } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogHeightTransition, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableLoadingRow, TableRow } from "@/components/ui/table";
import { TablePagination, TableToolbar } from "@/components/ui/table-tools";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useVirtualTableRows, VirtualTablePaddingRow } from "@/components/ui/virtual-table";
import { AdminDateTimePicker, adminDateTimeFormValue, adminDateTimeValueToISOString } from "@/features/admin/components/shared/date-time-picker";
import { AdminBulkConfirmDialog } from "@/features/admin/components/shared/bulk-confirm-dialog";
import { RedemptionRecordsDialog } from "@/features/admin/components/sections/billing/billing-redemption-records-dialog";
import type { AdminBillingMode, AdminBillingPlanDTO, AdminRedemptionCodeDTO } from "@/features/admin/api/billing-types";
import { useAdminBillingRedemptionBulk } from "@/features/admin/hooks/use-admin-billing-redemption-bulk";
import {
  type RedemptionFormState,
  useAdminBillingRedemptionCodes,
} from "@/features/admin/hooks/use-admin-billing-redemption-codes";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import {
  formatCreditUSD,
  formatDateTime,
} from "@/features/admin/model/billing-settings";
import { CopyActionButton, useCopyAction } from "@/shared/components/copy-action";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { cn } from "@/lib/utils";

type BillingRedemptionSectionProps = {
  plans: AdminBillingPlanDTO[];
  billingMode: AdminBillingMode;
  loading: boolean;
};

type RedemptionBulkAction = "activate" | "deactivate" | "delete";

function createRedemptionFormState(mode: AdminBillingMode, planID = ""): RedemptionFormState {
  return {
    code: "",
    quantity: "1",
    mode: mode === "period" ? "period" : "usage",
    creditUSD: "20",
    planID,
    durationDays: "30",
    maxRedemptions: "1",
    perUserLimit: "1",
    expiresAt: "",
    description: "",
    status: "active",
  };
}

function redemptionExpiresFormValue(value: string | null | undefined): string {
  return adminDateTimeFormValue(value);
}

function redemptionFormFromCode(item: AdminRedemptionCodeDTO): RedemptionFormState {
  return {
    id: item.id,
    code: "",
    quantity: "1",
    mode: item.mode === "period" ? "period" : "usage",
    creditUSD: String(item.creditUSD || 0),
    planID: item.planID ? String(item.planID) : "",
    durationDays: String(item.durationDays || 0),
    maxRedemptions: item.maxRedemptions == null ? "" : String(item.maxRedemptions),
    perUserLimit: String(item.perUserLimit || 1),
    expiresAt: redemptionExpiresFormValue(item.expiresAt),
    description: item.description || "",
    status: item.status === "inactive" ? "inactive" : "active",
  };
}

export function BillingRedemptionSection({ plans, billingMode, loading }: BillingRedemptionSectionProps) {
  const locale = useLocale();
  const t = useTranslations("adminBilling");
  const tActions = useTranslations("common.actions");
  const tCommonErrors = useTranslations("common.errors");
  const { copy, isCopied } = useCopyAction({
    messages: {
      copied: tActions("copied"),
      failed: tCommonErrors("copyFailed"),
    },
  });
  const {
    redemptionCodes,
    setRedemptionCodes,
    redemptionLoading,
    redemptionQuery,
    setRedemptionQuery,
    redemptionModeFilter,
    setRedemptionModeFilter,
    redemptionStatusFilter,
    setRedemptionStatusFilter,
    redemptionAvailabilityFilter,
    setRedemptionAvailabilityFilter,
    redemptionPage,
    setRedemptionPage,
    redemptionPageSize,
    setRedemptionPageSize,
    redemptionTotal,
    setRedemptionTotal,
    redemptionSaving,
    selectedRedemptionIDs,
    setSelectedRedemptionIDs,
    redemptionVisibleIDs,
    createdRedemptionCodes,
    setCreatedRedemptionCodes,
    redemptionStatusPendingID,
    loadRedemptionCodes,
    setRedemptionCodeStatus,
    saveRedemptionCode: persistRedemptionCode,
  } = useAdminBillingRedemptionCodes({ billingMode });
  const {
    redemptionBulkPending,
    revealRedemptionCode,
    copySelectedRedemptionCodes,
    exportSelectedRedemptionCodes,
    applyRedemptionBulkStatus,
    deleteSelectedRedemptionCodes,
    deleteSingleRedemptionCode: deleteSingleRedemptionCodeRequest,
  } = useAdminBillingRedemptionBulk({
    redemptionCodes,
    setRedemptionCodes,
    redemptionTotal,
    setRedemptionTotal,
    selectedRedemptionIDs,
    setSelectedRedemptionIDs,
    loadRedemptionCodes,
    copy,
  });
  const [redemptionForm, setRedemptionForm] = React.useState<RedemptionFormState | null>(null);
  const redemptionDialogForm = useDialogSnapshot(redemptionForm);
  const [redemptionBulkAction, setRedemptionBulkAction] = React.useState<RedemptionBulkAction | null>(null);
  const stableRedemptionBulkAction = useDialogSnapshot(redemptionBulkAction);
  const [redemptionDeleteTarget, setRedemptionDeleteTarget] = React.useState<AdminRedemptionCodeDTO | null>(null);
  const [redemptionRecordsTarget, setRedemptionRecordsTarget] = React.useState<AdminRedemptionCodeDTO | null>(null);

  const activePlanOptions = React.useMemo(() => plans.filter((plan) => plan.isActive && plan.code.trim() !== "free"), [plans]);
  const defaultRedemptionPlanID = activePlanOptions[0]?.id ? String(activePlanOptions[0].id) : "";
  const redemptionVisibleSelectedCount = React.useMemo(
    () => redemptionVisibleIDs.filter((id) => selectedRedemptionIDs.has(id)).length,
    [redemptionVisibleIDs, selectedRedemptionIDs],
  );
  const redemptionSelectAllState: boolean | "indeterminate" =
    redemptionVisibleIDs.length === 0
      ? false
      : redemptionVisibleSelectedCount === redemptionVisibleIDs.length
        ? true
        : redemptionVisibleSelectedCount > 0
          ? "indeterminate"
          : false;
  const planNameByID = React.useMemo(() => {
    const values = new Map<number, string>();
    for (const plan of plans) {
      values.set(plan.id, plan.name || plan.code);
    }
    return values;
  }, [plans]);
  const redemptionPageCount = Math.max(1, Math.ceil(redemptionTotal / redemptionPageSize));
  const redemptionTableLoading = loading || redemptionLoading;
  const redemptionVirtualRows = useVirtualTableRows(redemptionCodes, {
    enabled: redemptionCodes.length > 100,
    estimateSize: 40,
  });
  const redemptionInitialLoading = redemptionTableLoading && redemptionCodes.length === 0;
  const showRedemptionRows = redemptionCodes.length > 0;

  function openRedemptionCreate() {
    setCreatedRedemptionCodes([]);
    setRedemptionForm(createRedemptionFormState(billingMode, defaultRedemptionPlanID));
  }

  function openRedemptionEdit(item: AdminRedemptionCodeDTO) {
    setCreatedRedemptionCodes([]);
    setRedemptionForm(redemptionFormFromCode(item));
  }

  function handleToggleRedemptionSelected(id: number, checked: boolean) {
    setSelectedRedemptionIDs((current) => {
      const next = new Set(current);
      if (checked) {
        next.add(id);
      } else {
        next.delete(id);
      }
      return next;
    });
  }

  function handleSelectAllRedemptions(checked: boolean) {
    setSelectedRedemptionIDs((current) => {
      const next = new Set(current);
      for (const id of redemptionVisibleIDs) {
        if (checked) {
          next.add(id);
        } else {
          next.delete(id);
        }
      }
      return next;
    });
  }

  async function deleteSingleRedemptionCode() {
    if (!redemptionDeleteTarget) return;
    if (await deleteSingleRedemptionCodeRequest(redemptionDeleteTarget)) {
      setRedemptionDeleteTarget(null);
    }
  }

  function closeBulkActionOnSuccess(succeeded: boolean) {
    if (succeeded) setRedemptionBulkAction(null);
  }

  function confirmRedemptionBulkAction() {
    switch (redemptionBulkAction) {
      case "activate":
        void applyRedemptionBulkStatus("active").then(closeBulkActionOnSuccess);
        break;
      case "deactivate":
        void applyRedemptionBulkStatus("inactive").then(closeBulkActionOnSuccess);
        break;
      case "delete":
        void deleteSelectedRedemptionCodes().then(closeBulkActionOnSuccess);
        break;
    }
  }

  async function saveRedemptionCode(event?: React.FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    if (!redemptionForm) return;

    if (await persistRedemptionCode(redemptionForm, adminDateTimeValueToISOString(redemptionForm.expiresAt))) {
      setRedemptionForm(null);
    }
  }

  function redemptionRewardLabel(item: AdminRedemptionCodeDTO): string {
    if (item.mode === "period") {
      const planLabel = planNameByID.get(item.planID) || t("redemption.unknownPlan");
      return t("redemption.periodReward", { plan: planLabel, days: item.durationDays || 0 });
    }
    return t("redemption.usageReward", { amount: formatCreditUSD(item.creditUSD) });
  }

  function redemptionModeLabel(mode: AdminBillingMode | string): string {
    return t(`billingConfig.modes.${mode === "period" ? "period" : mode === "usage" ? "usage" : "self"}`);
  }

  function redemptionUnavailableReason(item: AdminRedemptionCodeDTO): string | null {
    if (item.status !== "active") {
      return t("redemption.unavailableInactive");
    }
    if (item.expiresAt && new Date(item.expiresAt).getTime() <= Date.now()) {
      return t("redemption.unavailableExpired");
    }
    if (item.remainingRedemptions !== null && item.remainingRedemptions <= 0) {
      return t("redemption.unavailableExhausted");
    }
    if (billingMode === "self") {
      return t("redemption.unavailableSelf");
    }
    const codeMode = item.mode === "period" ? "period" : "usage";
    const modeAllowed = billingMode === "period"
      ? codeMode === "usage" || codeMode === "period"
      : billingMode === codeMode;
    if (!modeAllowed) {
      return t("redemption.unavailableModeMismatch", {
        currentMode: redemptionModeLabel(billingMode),
        codeMode: redemptionModeLabel(codeMode),
      });
    }
    return null;
  }

  function redemptionBulkConfirmTitle(action: RedemptionBulkAction | null): string {
    switch (action) {
      case "activate":
        return t("redemption.bulkEnableTitle");
      case "deactivate":
        return t("redemption.bulkDisableTitle");
      case "delete":
        return t("redemption.bulkDeleteTitle");
      default:
        return "";
    }
  }

  function redemptionBulkConfirmLabel(action: RedemptionBulkAction | null): string {
    switch (action) {
      case "activate":
        return t("redemption.enable");
      case "deactivate":
        return t("redemption.disable");
      case "delete":
        return tActions("delete");
      default:
        return tActions("confirm");
    }
  }

  return (
    <section className="space-y-6 px-1">
      <div className="flex h-10 items-center justify-between gap-3">
        <h3 className="text-sm font-semibold">{t("redemption.title")}</h3>
      </div>

      <div className="space-y-3">
        <TableToolbar
          query={redemptionQuery}
          onQueryChange={(value) => {
            setRedemptionQuery(value);
            setRedemptionPage(1);
          }}
          queryPlaceholder={t("redemption.searchPlaceholder")}
          filters={[
            {
              key: "mode",
              label: t("redemption.modeFilterLabel"),
              value: redemptionModeFilter,
              onValueChange: (value) => {
                setRedemptionModeFilter(value);
                setRedemptionPage(1);
              },
              options: [
                { label: t("redemption.allModes"), value: "" },
                { label: t("billingConfig.modes.usage"), value: "usage" },
                { label: t("billingConfig.modes.period"), value: "period" },
              ],
            },
            {
              key: "status",
              label: t("redemption.statusFilterLabel"),
              value: redemptionStatusFilter,
              onValueChange: (value) => {
                setRedemptionStatusFilter(value);
                setRedemptionPage(1);
              },
              options: [
                { label: t("redemption.allStatuses"), value: "" },
                { label: t("redemption.active"), value: "active" },
                { label: t("redemption.inactive"), value: "inactive" },
              ],
            },
            {
              key: "availability",
              label: t("redemption.availabilityFilterLabel"),
              value: redemptionAvailabilityFilter,
              onValueChange: (value) => {
                setRedemptionAvailabilityFilter(value);
                setRedemptionPage(1);
              },
              options: [
                { label: t("redemption.allAvailability"), value: "" },
                { label: t("redemption.available"), value: "available" },
                { label: t("redemption.expired"), value: "expired" },
                { label: t("redemption.exhausted"), value: "exhausted" },
              ],
            },
          ]}
          selectedCount={selectedRedemptionIDs.size}
          bulkActions={[
            {
              key: "copy-codes",
              label: t("redemption.copySelected"),
              icon: isCopied("selected-redemption-codes") ? <Check className="size-3.5 stroke-1" /> : <Copy className="size-3.5 stroke-1" />,
              onClick: () => void copySelectedRedemptionCodes(),
            },
            {
              key: "export-codes",
              label: t("redemption.exportSelected"),
              icon: <Download className="size-3.5 stroke-1" />,
              onClick: () => void exportSelectedRedemptionCodes(),
            },
            {
              key: "activate",
              label: t("redemption.enable"),
              icon: <Check className="size-3.5 stroke-1" />,
              onClick: () => setRedemptionBulkAction("activate"),
            },
            {
              key: "deactivate",
              label: t("redemption.disable"),
              icon: <X className="size-3.5 stroke-1" />,
              onClick: () => setRedemptionBulkAction("deactivate"),
            },
            {
              key: "delete",
              label: tActions("delete"),
              icon: <Trash2 className="size-3.5 stroke-1" />,
              onClick: () => setRedemptionBulkAction("delete"),
            },
          ]}
          loading={redemptionTableLoading || redemptionBulkPending}
          onRefresh={() => void loadRedemptionCodes()}
        >
          <Button type="button" size="sm" disabled={redemptionTableLoading || redemptionSaving || redemptionBulkPending} onClick={openRedemptionCreate}>
            <Plus className="size-3.5" />
            {t("redemption.create")}
          </Button>
        </TableToolbar>

        <Table
          viewportRef={redemptionVirtualRows.viewportRef}
          viewportClassName={redemptionVirtualRows.viewportClassName}
          viewportStyle={redemptionVirtualRows.viewportStyle}
        >
          <TableHeader>
            <TableRow>
              <TableHead className="w-[44px] py-1.5 text-center">
                <div className="flex h-7 items-center justify-center">
                  <Checkbox
                    checked={redemptionSelectAllState}
                    onCheckedChange={(checked) => handleSelectAllRedemptions(checked === true)}
                    disabled={redemptionTableLoading || redemptionCodes.length === 0}
                  />
                </div>
              </TableHead>
              <TableHead className="w-[168px]">{t("redemption.columns.code")}</TableHead>
              <TableHead className="w-[112px]">{t("redemption.columns.mode")}</TableHead>
              <TableHead className="w-[112px]">{t("redemption.columns.reward")}</TableHead>
              <TableHead className="w-[120px]">{t("redemption.columns.limit")}</TableHead>
              <TableHead className="w-[76px] text-center">{t("redemption.columns.status")}</TableHead>
              <TableHead className="w-[104px]">{t("redemption.columns.expiresAt")}</TableHead>
              <TableHead stickyEnd className="w-[116px]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {redemptionInitialLoading ? <TableLoadingRow colSpan={8} /> : null}
            {!redemptionTableLoading && redemptionCodes.length === 0 ? <TableEmptyRow colSpan={8}>{t("redemption.empty")}</TableEmptyRow> : null}
            {showRedemptionRows ? <VirtualTablePaddingRow colSpan={8} height={redemptionVirtualRows.paddingTop} /> : null}
            {showRedemptionRows
              ? redemptionVirtualRows.rows.map(({ item }) => {
                const unavailableReason = redemptionUnavailableReason(item);
                const displayCode = item.codeHint || "-";
                const redemptionLimitTotal = item.maxRedemptions == null ? t("redemption.unlimited") : String(item.maxRedemptions);
                return (
                  <TableRow key={item.id} tone={unavailableReason ? "muted" : undefined} className={cn(unavailableReason && "text-muted-foreground")}>
                    <TableCell className="w-[44px] py-1.5 text-center">
                      <div className="flex h-7 items-center justify-center">
                        <Checkbox
                          checked={selectedRedemptionIDs.has(item.id)}
                          onCheckedChange={(checked) => handleToggleRedemptionSelected(item.id, checked === true)}
                          disabled={redemptionBulkPending}
                        />
                      </div>
                    </TableCell>
                    <TableCell className="w-[168px] max-w-[168px] py-1.5 font-mono text-xs">
                      <div className="flex h-7 items-center gap-1.5">
                        <span className="min-w-0 max-w-[112px] truncate">{displayCode}</span>
                        <CopyActionButton
                          type="button"
                          variant="ghost"
                          size="icon-xs"
                          className="h-6 w-6 text-muted-foreground shadow-none"
                          messages={{ copied: tActions("copied"), failed: t("toast.redemptionCopyFailed") }}
                          resolveValue={() => revealRedemptionCode(item)}
                          onResolveError={(error) => toast.error(t("toast.redemptionCopyFailed"), { description: resolveAdminErrorMessage(error) })}
                          iconClassName="size-3.5 stroke-1.5"
                          aria-label={tActions("copy")}
                        />
                        {unavailableReason ? (
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <span
                                tabIndex={0}
                                role="img"
                                aria-label={t("redemption.unavailable")}
                                className="inline-flex size-4 items-center justify-center text-amber-600 outline-none focus-visible:ring-2 focus-visible:ring-ring dark:text-amber-400"
                              >
                                <CircleAlert className="size-3.5 stroke-1.5" />
                              </span>
                            </TooltipTrigger>
                            <TooltipContent side="top" className="max-w-64 text-left">
                              <div className="space-y-1">
                                <p className="font-medium">{t("redemption.unavailable")}</p>
                                <p className="text-background/80">{unavailableReason}</p>
                              </div>
                            </TooltipContent>
                          </Tooltip>
                        ) : null}
                      </div>
                    </TableCell>
                    <TableCell className="w-[112px] py-1.5 text-xs">{redemptionModeLabel(item.mode)}</TableCell>
                    <TableCell className="w-[112px] max-w-[112px] py-1.5 text-xs">
                      <span className="block truncate">{redemptionRewardLabel(item)}</span>
                    </TableCell>
                    <TableCell className="w-[120px] py-1.5 text-xs">
                      <div className="flex items-center gap-1.5 text-[11px] leading-none">
                        <span className="inline-flex h-5 min-w-11 items-center justify-center rounded-sm border border-border/60 bg-background/60 px-1.5 font-mono tabular-nums">
                          {item.redeemedCount}
                          <span className="px-0.5 text-muted-foreground">/</span>
                          {redemptionLimitTotal}
                        </span>
                        <span className="truncate text-muted-foreground">
                          {t("redemption.perUserShort", { count: item.perUserLimit })}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell className="w-[76px] py-1.5 text-center">
                      <div className="flex h-7 items-center justify-center">
                        <Switch
                          size="sm"
                          checked={item.status === "active"}
                          disabled={redemptionBulkPending || redemptionStatusPendingID === item.id}
                          onCheckedChange={(checked) => void setRedemptionCodeStatus(item, checked)}
                          aria-label={item.status === "active" ? t("redemption.disable") : t("redemption.enable")}
                        />
                      </div>
                    </TableCell>
                    <TableCell className="w-[104px] py-1.5 text-xs text-muted-foreground">{item.expiresAt ? formatDateTime(item.expiresAt, locale) : t("redemption.never")}</TableCell>
                    <TableCell stickyEnd className="w-[116px] py-1.5 text-right">
                      <div className="flex h-7 items-center justify-end">
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon-xs"
                          className="h-7 w-7 text-muted-foreground shadow-none"
                          onClick={() => setRedemptionRecordsTarget(item)}
                          aria-label={t("redemption.viewRecords")}
                          title={t("redemption.viewRecords")}
                        >
                          <History className="size-3.5 stroke-1" />
                        </Button>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon-xs"
                          className="h-7 w-7 text-muted-foreground shadow-none"
                          onClick={() => openRedemptionEdit(item)}
                          aria-label={t("redemption.edit")}
                        >
                          <Pencil className="size-3.5 stroke-1" />
                        </Button>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon-xs"
                          className="h-7 w-7 text-destructive shadow-none hover:bg-destructive/10 hover:text-destructive"
                          onClick={() => setRedemptionDeleteTarget(item)}
                          aria-label={tActions("delete")}
                        >
                          <Trash2 className="size-3.5 stroke-1" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                );
              })
              : null}
            {showRedemptionRows ? <VirtualTablePaddingRow colSpan={8} height={redemptionVirtualRows.paddingBottom} /> : null}
          </TableBody>
        </Table>

        <TablePagination
          total={redemptionTotal}
          page={redemptionPage}
          pageCount={redemptionPageCount}
          pageSize={redemptionPageSize}
          onPageChange={setRedemptionPage}
          onPageSizeChange={(next) => {
            setRedemptionPageSize(next);
            setRedemptionPage(1);
          }}
          loading={redemptionTableLoading}
        />
      </div>

      <Dialog
        open={!!redemptionForm}
        onOpenChange={(open) => {
          if (!open && !redemptionSaving) {
            setRedemptionForm(null);
          }
        }}
      >
        {redemptionDialogForm ? (
          <DialogContent className="gap-0 overflow-hidden p-0">
            <DialogHeightTransition contentClassName="max-h-[min(86vh,760px)]">
              <DialogHeader className="shrink-0 px-4 py-4">
                <DialogTitle>{redemptionDialogForm.id ? t("redemption.editTitle") : t("redemption.createTitle")}</DialogTitle>
                <DialogDescription>
                  {redemptionDialogForm.id ? t("redemption.editDescription") : t("redemption.createDescription")}
                </DialogDescription>
              </DialogHeader>

              <form onSubmit={(event) => void saveRedemptionCode(event)} className="flex min-h-0 flex-1 flex-col">
                <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-2">
                  {!redemptionDialogForm.id ? (
                    <div className="grid grid-cols-2 gap-5">
                      <div className="space-y-1">
                        <p className="text-xs text-muted-foreground">{t("redemption.code")}</p>
                        <Input
                          id="redemption-code"
                          value={redemptionDialogForm.code}
                          placeholder={t("redemption.codePlaceholder")}
                          disabled={redemptionSaving}
                          onChange={(event) => setRedemptionForm((current) => current ? { ...current, code: event.target.value } : current)}
                        />
                      </div>
                      <div className="space-y-1">
                        <p className="text-xs text-muted-foreground">{t("redemption.quantity")}</p>
                        <Input
                          id="redemption-quantity"
                          type="number"
                          min={1}
                          max={100}
                          value={redemptionDialogForm.quantity}
                          disabled={redemptionSaving || Boolean(redemptionDialogForm.code.trim())}
                          onChange={(event) => setRedemptionForm((current) => current ? { ...current, quantity: event.target.value } : current)}
                        />
                      </div>
                    </div>
                  ) : null}

                  <div className={cn("grid gap-5", redemptionDialogForm.id && "grid-cols-2")}>
                    <div className="space-y-1">
                      <p className="text-xs text-muted-foreground">{t("redemption.mode")}</p>
                      <Select
                        value={redemptionDialogForm.mode}
                        disabled={redemptionSaving || Boolean(redemptionDialogForm.id)}
                        onValueChange={(value) => {
                          const mode = value === "period" ? "period" : "usage";
                          setRedemptionForm((current) => current ? {
                            ...current,
                            mode,
                            planID: mode === "period" ? current.planID || defaultRedemptionPlanID : current.planID,
                          } : current);
                        }}
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent align="end">
                          <SelectItem value="usage">{t("billingConfig.modes.usage")}</SelectItem>
                          <SelectItem value="period">{t("billingConfig.modes.period")}</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>

                    {redemptionDialogForm.id ? (
                      <div className="space-y-1">
                        <p className="text-xs text-muted-foreground">{t("redemption.status")}</p>
                        <div className="flex h-8 items-center px-1">
                          <Switch
                            size="sm"
                            checked={redemptionDialogForm.status === "active"}
                            disabled={redemptionSaving}
                            onCheckedChange={(checked) => setRedemptionForm((current) => current ? { ...current, status: checked ? "active" : "inactive" } : current)}
                            aria-label={redemptionDialogForm.status === "active" ? t("redemption.disable") : t("redemption.enable")}
                          />
                        </div>
                      </div>
                    ) : null}
                  </div>

                  {redemptionDialogForm.mode === "usage" ? (
                    <div className="space-y-1">
                      <p className="text-xs text-muted-foreground">{t("redemption.creditUSD")}</p>
                      <Input
                        id="redemption-credit"
                        type="number"
                        min="0"
                        step="0.01"
                        value={redemptionDialogForm.creditUSD}
                        disabled={redemptionSaving || Boolean(redemptionDialogForm.id)}
                        onChange={(event) => setRedemptionForm((current) => current ? { ...current, creditUSD: event.target.value } : current)}
                      />
                    </div>
                  ) : (
                    <div className="grid grid-cols-2 gap-5">
                      <div className="space-y-1">
                        <p className="text-xs text-muted-foreground">{t("redemption.plan")}</p>
                        <Select
                          value={redemptionDialogForm.planID}
                          disabled={redemptionSaving || Boolean(redemptionDialogForm.id) || activePlanOptions.length === 0}
                          onValueChange={(value) => setRedemptionForm((current) => current ? { ...current, planID: value } : current)}
                        >
                          <SelectTrigger>
                            <SelectValue placeholder={t("redemption.planPlaceholder")} />
                          </SelectTrigger>
                          <SelectContent align="end">
                            {activePlanOptions.map((plan) => (
                              <SelectItem key={plan.id} value={String(plan.id)}>{plan.name || plan.code}</SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="space-y-1">
                        <p className="text-xs text-muted-foreground">{t("redemption.durationDays")}</p>
                        <Input
                          id="redemption-duration"
                          type="number"
                          min={1}
                          value={redemptionDialogForm.durationDays}
                          disabled={redemptionSaving || Boolean(redemptionDialogForm.id)}
                          onChange={(event) => setRedemptionForm((current) => current ? { ...current, durationDays: event.target.value } : current)}
                        />
                      </div>
                    </div>
                  )}

                  <div className="grid grid-cols-2 gap-5">
                    <div className="space-y-1">
                      <p className="text-xs text-muted-foreground">{t("redemption.maxRedemptions")}</p>
                      <Input
                        id="redemption-max"
                        type="number"
                        min={1}
                        value={redemptionDialogForm.maxRedemptions}
                        placeholder={t("redemption.unlimited")}
                        disabled={redemptionSaving}
                        onChange={(event) => setRedemptionForm((current) => current ? { ...current, maxRedemptions: event.target.value } : current)}
                      />
                    </div>
                    <div className="space-y-1">
                      <p className="text-xs text-muted-foreground">{t("redemption.perUserLimit")}</p>
                      <Input
                        id="redemption-per-user"
                        type="number"
                        min={1}
                        max={redemptionDialogForm.maxRedemptions.trim() || undefined}
                        value={redemptionDialogForm.perUserLimit}
                        disabled={redemptionSaving}
                        onChange={(event) => setRedemptionForm((current) => current ? { ...current, perUserLimit: event.target.value } : current)}
                      />
                    </div>
                  </div>

                  <AdminDateTimePicker
                    value={redemptionDialogForm.expiresAt}
                    disabled={redemptionSaving}
                    label={t("redemption.expiresAt")}
                    placeholder={t("redemption.never")}
                    onChange={(value) => setRedemptionForm((current) => current ? { ...current, expiresAt: value } : current)}
                  />

                  <div className="space-y-1">
                    <p className="text-xs text-muted-foreground">{t("redemption.description")}</p>
                    <Textarea
                      id="redemption-description"
                      value={redemptionDialogForm.description}
                      className="h-20 resize-none"
                      disabled={redemptionSaving}
                      onChange={(event) => setRedemptionForm((current) => current ? { ...current, description: event.target.value } : current)}
                    />
                  </div>
                </div>

                <DialogFooter className="shrink-0 px-4 py-3">
                  <Button type="button" variant="ghost" disabled={redemptionSaving} onClick={() => setRedemptionForm(null)}>
                    {tActions("cancel")}
                  </Button>
                  <Button type="submit" disabled={redemptionSaving}>
                    {redemptionSaving ? <SpinnerLabel>{tActions("saving")}</SpinnerLabel> : tActions("save")}
                  </Button>
                </DialogFooter>
              </form>
            </DialogHeightTransition>
          </DialogContent>
        ) : null}
      </Dialog>

      <Dialog
        open={createdRedemptionCodes.length > 0}
        onOpenChange={(open) => {
          if (!open) {
            setCreatedRedemptionCodes([]);
          }
        }}
      >
        <DialogContent className="gap-0 overflow-hidden p-0">
          <DialogHeightTransition contentClassName="max-h-[min(86vh,760px)]">
            <DialogHeader className="shrink-0 px-4 py-4">
              <DialogTitle>{t("redemption.createdCodesTitle")}</DialogTitle>
              <DialogDescription>{t("redemption.createdCodesDescription")}</DialogDescription>
            </DialogHeader>

            <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-4 py-2">
              <div className="flex items-center justify-between gap-3">
                <p className="text-xs font-medium">{t("redemption.createdCodes")}</p>
                <CopyActionButton
                  type="button"
                  variant="outline"
                  size="sm"
                  className="h-7 gap-1 px-2 text-xs shadow-none"
                  value={createdRedemptionCodes.join("\n")}
                  messages={{ copied: tActions("copied"), failed: tCommonErrors("copyFailed") }}
                  disabled={createdRedemptionCodes.length === 0}
                >
                  {t("redemption.copyAll")}
                </CopyActionButton>
              </div>
              <div className="max-h-72 space-y-2 overflow-y-auto pr-1">
                {createdRedemptionCodes.map((code) => (
                  <div key={code} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-2 rounded-md border border-border/60 bg-muted/25 px-3 py-2">
                    <span className="min-w-0 break-all font-mono text-xs">{code}</span>
                    <CopyActionButton
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      className="text-muted-foreground"
                      value={code}
                      messages={{ copied: tActions("copied"), failed: tCommonErrors("copyFailed") }}
                      aria-label={tActions("copy")}
                    />
                  </div>
                ))}
              </div>
            </div>

            <DialogFooter className="shrink-0 px-4 py-3">
              <Button type="button" onClick={() => setCreatedRedemptionCodes([])}>
                {tActions("close")}
              </Button>
            </DialogFooter>
          </DialogHeightTransition>
        </DialogContent>
      </Dialog>

      <AdminBulkConfirmDialog
        open={redemptionBulkAction !== null}
        onOpenChange={(open) => {
          if (!open && !redemptionBulkPending) setRedemptionBulkAction(null);
        }}
        pending={redemptionBulkPending}
        title={redemptionBulkConfirmTitle(stableRedemptionBulkAction)}
        description={t("redemption.bulkConfirmDescription", { count: selectedRedemptionIDs.size })}
        confirmLabel={redemptionBulkConfirmLabel(stableRedemptionBulkAction)}
        pendingLabel={t("redemption.bulkPending")}
        onConfirm={confirmRedemptionBulkAction}
      />

      <AdminBulkConfirmDialog
        open={redemptionDeleteTarget !== null}
        onOpenChange={(open) => {
          if (!open && !redemptionBulkPending) setRedemptionDeleteTarget(null);
        }}
        pending={redemptionBulkPending}
        title={t("redemption.deleteTitle")}
        description={t("redemption.deleteDescription")}
        confirmLabel={tActions("delete")}
        pendingLabel={t("redemption.deleting")}
        onConfirm={() => void deleteSingleRedemptionCode()}
      />

      <RedemptionRecordsDialog code={redemptionRecordsTarget} onClose={() => setRedemptionRecordsTarget(null)} />
    </section>
  );
}
