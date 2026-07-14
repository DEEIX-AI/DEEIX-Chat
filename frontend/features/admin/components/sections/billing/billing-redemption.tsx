"use client";

import * as React from "react";
import { Check, CircleAlert, Copy, Download, Pencil, Plus, Trash2, X } from "lucide-react";
import { motion } from "motion/react";
import { useLocale, useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableLoadingRow, TableRow } from "@/components/ui/table";
import { TablePagination, TableToolbar } from "@/components/ui/table-tools";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useVirtualTableRows, VirtualTablePaddingRow } from "@/components/ui/virtual-table";
import { AdminDateTimePicker, adminDateTimeFormValue, adminDateTimeValueToISOString } from "@/features/admin/components/admin-date-time-picker";
import { AdminBulkConfirmDialog } from "@/features/admin/components/bulk-confirm-dialog";
import {
  batchDeleteAdminRedemptionCodes,
  createAdminRedemptionCodes,
  deleteAdminRedemptionCode,
  listAdminRedemptionCodes,
  revealAdminRedemptionCode,
  updateAdminRedemptionCode,
} from "@/features/admin/api";
import type { AdminBillingMode, AdminBillingPlanDTO, AdminRedemptionCodeDTO } from "@/features/admin/api/billing.types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import {
  DEFAULT_PAGE_SIZE,
  DIALOG_LAYOUT_TRANSITION,
  downloadJSONFile,
  formatCreditUSD,
  formatDateTime,
} from "@/features/admin/model/billing-settings";
import { CopyActionButton, useCopyAction } from "@/shared/components/copy-action";
import { mergeBatchResultData, runBulkActionInChunks } from "@/shared/lib/bulk-action";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { cn } from "@/lib/utils";

type BillingRedemptionSectionProps = {
  plans: AdminBillingPlanDTO[];
  billingMode: AdminBillingMode;
  loading: boolean;
};

type RedemptionFormState = {
  id?: number;
  code: string;
  quantity: string;
  mode: "usage" | "period";
  creditUSD: string;
  planID: string;
  durationDays: string;
  maxRedemptions: string;
  perUserLimit: string;
  expiresAt: string;
  description: string;
  status: "active" | "inactive";
};

type RedemptionBulkAction = "activate" | "deactivate" | "delete";

function redemptionCodesExportFilename(): string {
  const date = new Date().toISOString().slice(0, 10);
  return `deeix-chat-redemption-codes-${date}.json`;
}

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

function datetimeLocalToISOString(value: string): string | null | undefined {
  return adminDateTimeValueToISOString(value);
}

function parseOptionalPositiveInt(value: string): number | null | undefined {
  const text = value.trim();
  if (!text) return null;
  const parsed = Number(text);
  if (!Number.isInteger(parsed) || parsed <= 0) return undefined;
  return parsed;
}

function parseRequiredPositiveInt(value: string): number | undefined {
  const parsed = parseOptionalPositiveInt(value);
  return parsed && parsed > 0 ? parsed : undefined;
}

function isRedemptionCodeFormatValid(value: string): boolean {
  const text = value.trim();
  return !text || /^[A-Za-z0-9_-]{3,64}$/.test(text);
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
  const [redemptionCodes, setRedemptionCodes] = React.useState<AdminRedemptionCodeDTO[]>([]);
  const [redemptionLoading, setRedemptionLoading] = React.useState(false);
  const [redemptionQuery, setRedemptionQuery] = React.useState("");
  const [redemptionModeFilter, setRedemptionModeFilter] = React.useState("");
  const [redemptionStatusFilter, setRedemptionStatusFilter] = React.useState("");
  const [redemptionAvailabilityFilter, setRedemptionAvailabilityFilter] = React.useState("");
  const [redemptionPage, setRedemptionPage] = React.useState(1);
  const [redemptionPageSize, setRedemptionPageSize] = React.useState(DEFAULT_PAGE_SIZE);
  const [redemptionTotal, setRedemptionTotal] = React.useState(0);
  const [redemptionForm, setRedemptionForm] = React.useState<RedemptionFormState | null>(null);
  const redemptionDialogForm = useDialogSnapshot(redemptionForm);
  const [redemptionSaving, setRedemptionSaving] = React.useState(false);
  const [selectedRedemptionIDs, setSelectedRedemptionIDs] = React.useState<Set<number>>(new Set());
  const [redemptionBulkAction, setRedemptionBulkAction] = React.useState<RedemptionBulkAction | null>(null);
  const stableRedemptionBulkAction = useDialogSnapshot(redemptionBulkAction);
  const [redemptionBulkPending, setRedemptionBulkPending] = React.useState(false);
  const [redemptionDeleteTarget, setRedemptionDeleteTarget] = React.useState<AdminRedemptionCodeDTO | null>(null);
  const [createdRedemptionCodes, setCreatedRedemptionCodes] = React.useState<string[]>([]);
  const [redemptionStatusPendingID, setRedemptionStatusPendingID] = React.useState<number | null>(null);

  const activePlanOptions = React.useMemo(() => plans.filter((plan) => plan.isActive && plan.code.trim() !== "free"), [plans]);
  const defaultRedemptionPlanID = activePlanOptions[0]?.id ? String(activePlanOptions[0].id) : "";
  const redemptionVisibleIDs = React.useMemo(() => redemptionCodes.map((item) => item.id), [redemptionCodes]);
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

  const loadRedemptionCodes = React.useCallback(async (overrides: {
    page?: number;
    pageSize?: number;
    query?: string;
    mode?: string;
    status?: string;
    availability?: string;
  } = {}, options: { showLoading?: boolean; showError?: boolean } = {}) => {
    const showLoading = options.showLoading ?? true;
    const showError = options.showError ?? showLoading;
    if (showLoading) {
      setRedemptionLoading(true);
    }
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const result = await listAdminRedemptionCodes(token, {
        page: overrides.page ?? redemptionPage,
        pageSize: overrides.pageSize ?? redemptionPageSize,
        query: overrides.query ?? redemptionQuery,
        mode: overrides.mode ?? redemptionModeFilter,
        status: overrides.status ?? redemptionStatusFilter,
        availability: overrides.availability ?? redemptionAvailabilityFilter,
      });
      setRedemptionCodes(result.results ?? []);
      setRedemptionTotal(result.total ?? 0);
    } catch (error) {
      if (showError) {
        toast.error(t("toast.redemptionLoadFailed"), { description: resolveAdminErrorMessage(error) });
      }
    } finally {
      if (showLoading) {
        setRedemptionLoading(false);
      }
    }
  }, [redemptionAvailabilityFilter, redemptionModeFilter, redemptionPage, redemptionPageSize, redemptionQuery, redemptionStatusFilter, t]);

  React.useEffect(() => {
    void loadRedemptionCodes();
  }, [loadRedemptionCodes]);

  React.useEffect(() => {
    if (redemptionAvailabilityFilter === "available") {
      void loadRedemptionCodes({}, { showLoading: false });
    }
  }, [billingMode, loadRedemptionCodes, redemptionAvailabilityFilter]);

  React.useEffect(() => {
    const visibleSet = new Set(redemptionVisibleIDs);
    setSelectedRedemptionIDs((current) => {
      const next = new Set<number>();
      current.forEach((id) => {
        if (visibleSet.has(id)) next.add(id);
      });
      return next.size === current.size ? current : next;
    });
  }, [redemptionVisibleIDs]);

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

  async function fetchRedemptionCodePlaintext(item: AdminRedemptionCodeDTO): Promise<string> {
    const token = await resolveAccessToken();
    if (!token) {
      throw new Error(t("toast.sessionExpired"));
    }
    return fetchRedemptionCodePlaintextWithToken(token, item);
  }

  async function fetchRedemptionCodePlaintextWithToken(accessToken: string, item: AdminRedemptionCodeDTO): Promise<string> {
    const data = await revealAdminRedemptionCode(accessToken, item.id);
    const code = data.code.code?.trim();
    if (!code) {
      throw new Error(t("toast.redemptionCodeRevealUnavailable"));
    }
    return code;
  }

  async function revealSelectedRedemptionCodes(): Promise<{
    results: Array<{ item: AdminRedemptionCodeDTO; code: string }>;
    failedCount: number;
  }> {
    const selectedItems = redemptionCodes.filter((item) => selectedRedemptionIDs.has(item.id));
    if (selectedItems.length === 0) {
      return { results: [], failedCount: 0 };
    }
    const token = await resolveAccessToken();
    if (!token) {
      throw new Error(t("toast.sessionExpired"));
    }
    const results: Array<{ item: AdminRedemptionCodeDTO; code: string }> = [];
    let failedCount = 0;
    for (const item of selectedItems) {
      try {
        const code = await fetchRedemptionCodePlaintextWithToken(token, item);
        results.push({ item, code });
      } catch {
        failedCount += 1;
      }
    }
    if (results.length === 0 && failedCount > 0) {
      throw new Error(t("toast.redemptionBulkRevealSkipped", { count: failedCount }));
    }
    return { results, failedCount };
  }

  async function copySelectedRedemptionCodes() {
    setRedemptionBulkPending(true);
    try {
      const { results, failedCount } = await revealSelectedRedemptionCodes();
      if (results.length === 0) return;
      const copied = await copy(results.map((result) => result.code).join("\n"), {
        key: "selected-redemption-codes",
        copied: t("toast.redemptionBulkCopied", { count: results.length }),
        copiedDescription: failedCount > 0 ? t("toast.redemptionBulkRevealSkipped", { count: failedCount }) : undefined,
        failed: t("toast.redemptionBulkCopyFailed"),
      });
      if (!copied) {
        return;
      }
    } catch (error) {
      toast.error(t("toast.redemptionBulkCopyFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setRedemptionBulkPending(false);
    }
  }

  async function exportSelectedRedemptionCodes() {
    setRedemptionBulkPending(true);
    try {
      const { results, failedCount } = await revealSelectedRedemptionCodes();
      if (results.length === 0) return;
      downloadJSONFile(redemptionCodesExportFilename(), {
        exportedAt: new Date().toISOString(),
        total: results.length,
        results: results.map(({ item, code }) => ({
          id: item.id,
          code,
          codeHint: item.codeHint,
          mode: item.mode,
          rewardType: item.rewardType,
          creditUSD: item.creditUSD,
          planID: item.planID,
          durationDays: item.durationDays,
          maxRedemptions: item.maxRedemptions,
          perUserLimit: item.perUserLimit,
          redeemedCount: item.redeemedCount,
          remainingRedemptions: item.remainingRedemptions,
          status: item.status,
          expiresAt: item.expiresAt,
          description: item.description,
          createdAt: item.createdAt,
          updatedAt: item.updatedAt,
        })),
      });
      toast.success(t("toast.redemptionBulkExported", { count: results.length }), {
        description: failedCount > 0 ? t("toast.redemptionBulkRevealSkipped", { count: failedCount }) : undefined,
      });
    } catch (error) {
      toast.error(t("toast.redemptionBulkExportFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setRedemptionBulkPending(false);
    }
  }

  async function applyRedemptionBulkStatus(status: "active" | "inactive") {
    const ids = Array.from(selectedRedemptionIDs);
    if (ids.length === 0) return;
    const previousRedemptionCodes = redemptionCodes;
    const idSet = new Set(ids);
    const updatedAt = new Date().toISOString();
    setRedemptionCodes((current) => current.map((item) => (
      idSet.has(item.id) ? { ...item, status, updatedAt } : item
    )));
    setRedemptionBulkPending(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        setRedemptionCodes(previousRedemptionCodes);
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const updatedCodes = (await runBulkActionInChunks({
        chunkSize: 10,
        items: ids,
        title: t("redemption.bulkPending"),
        runChunk: async (chunk) => {
          const codes: AdminRedemptionCodeDTO[] = [];
          for (const id of chunk) {
            const data = await updateAdminRedemptionCode(token, id, { status });
            codes.push(data.code);
          }
          return codes;
        },
      })).flat();
      setRedemptionCodes((current) => current.map((item) => updatedCodes.find((code) => code.id === item.id) ?? item));
      setSelectedRedemptionIDs(new Set());
      setRedemptionBulkAction(null);
      toast.success(status === "active" ? t("toast.redemptionBulkEnabled", { count: ids.length }) : t("toast.redemptionBulkDisabled", { count: ids.length }));
      void loadRedemptionCodes({}, { showLoading: false });
    } catch (error) {
      setRedemptionCodes(previousRedemptionCodes);
      toast.error(t("toast.redemptionBulkFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setRedemptionBulkPending(false);
    }
  }

  async function setRedemptionCodeStatus(item: AdminRedemptionCodeDTO, checked: boolean) {
    const status = checked ? "active" : "inactive";
    if (item.status === status) return;
    const previousRedemptionCodes = redemptionCodes;
    const updatedAt = new Date().toISOString();
    setRedemptionCodes((current) => current.map((code) => (
      code.id === item.id ? { ...code, status, updatedAt } : code
    )));
    setRedemptionStatusPendingID(item.id);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        setRedemptionCodes(previousRedemptionCodes);
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const data = await updateAdminRedemptionCode(token, item.id, { status });
      setRedemptionCodes((current) => current.map((code) => code.id === data.code.id ? data.code : code));
      toast.success(status === "active" ? t("toast.redemptionEnabled") : t("toast.redemptionDisabled"));
      void loadRedemptionCodes({}, { showLoading: false });
    } catch (error) {
      setRedemptionCodes(previousRedemptionCodes);
      toast.error(t("toast.redemptionUpdateFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setRedemptionStatusPendingID(null);
    }
  }

  async function deleteSelectedRedemptionCodes() {
    const ids = Array.from(selectedRedemptionIDs);
    if (ids.length === 0) return;
    const previousRedemptionCodes = redemptionCodes;
    const previousRedemptionTotal = redemptionTotal;
    const idSet = new Set(ids);
    const removedVisibleCount = redemptionCodes.filter((item) => idSet.has(item.id)).length;
    setRedemptionCodes((current) => current.filter((item) => !idSet.has(item.id)));
    setRedemptionTotal((current) => Math.max(0, current - removedVisibleCount));
    setRedemptionBulkPending(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        setRedemptionCodes(previousRedemptionCodes);
        setRedemptionTotal(previousRedemptionTotal);
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const result = mergeBatchResultData(await runBulkActionInChunks({
        items: ids,
        title: t("redemption.bulkDeleteTitle"),
        runChunk: (chunk) => batchDeleteAdminRedemptionCodes(token, { ids: chunk }),
      }));
      setSelectedRedemptionIDs(new Set());
      setRedemptionBulkAction(null);
      if (result.failedCount > 0) {
        toast.error(t("toast.redemptionDeletePartialFailed"), {
          description: t("toast.redemptionDeleteSummary", {
            successCount: result.successCount,
            notFoundCount: result.notFoundCount,
            failedCount: result.failedCount,
          }),
        });
      } else {
        toast.success(t("toast.redemptionDeleted", { count: result.successCount }), {
          description: result.notFoundCount > 0
            ? t("toast.redemptionDeleteSummary", {
              successCount: result.successCount,
              notFoundCount: result.notFoundCount,
              failedCount: result.failedCount,
            })
            : undefined,
        });
      }
      void loadRedemptionCodes({}, { showLoading: false });
    } catch (error) {
      setRedemptionCodes(previousRedemptionCodes);
      setRedemptionTotal(previousRedemptionTotal);
      toast.error(t("toast.redemptionDeleteFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setRedemptionBulkPending(false);
    }
  }

  async function deleteSingleRedemptionCode() {
    if (!redemptionDeleteTarget) return;
    const target = redemptionDeleteTarget;
    const previousRedemptionCodes = redemptionCodes;
    const previousRedemptionTotal = redemptionTotal;
    const removedVisibleCount = redemptionCodes.some((item) => item.id === target.id) ? 1 : 0;
    setRedemptionCodes((current) => current.filter((item) => item.id !== target.id));
    setRedemptionTotal((current) => Math.max(0, current - removedVisibleCount));
    setRedemptionBulkPending(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        setRedemptionCodes(previousRedemptionCodes);
        setRedemptionTotal(previousRedemptionTotal);
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      await deleteAdminRedemptionCode(token, target.id);
      setSelectedRedemptionIDs((current) => {
        const next = new Set(current);
        next.delete(target.id);
        return next;
      });
      setRedemptionDeleteTarget(null);
      toast.success(t("toast.redemptionDeleted", { count: 1 }));
      void loadRedemptionCodes({}, { showLoading: false });
    } catch (error) {
      setRedemptionCodes(previousRedemptionCodes);
      setRedemptionTotal(previousRedemptionTotal);
      toast.error(t("toast.redemptionDeleteFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setRedemptionBulkPending(false);
    }
  }

  function confirmRedemptionBulkAction() {
    switch (redemptionBulkAction) {
      case "activate":
        void applyRedemptionBulkStatus("active");
        break;
      case "deactivate":
        void applyRedemptionBulkStatus("inactive");
        break;
      case "delete":
        void deleteSelectedRedemptionCodes();
        break;
    }
  }

  async function saveRedemptionCode(event?: React.FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    if (!redemptionForm) return;

    const maxRedemptions = parseOptionalPositiveInt(redemptionForm.maxRedemptions);
    const perUserLimit = parseRequiredPositiveInt(redemptionForm.perUserLimit);
    const expiresAt = datetimeLocalToISOString(redemptionForm.expiresAt);
    if (!isRedemptionCodeFormatValid(redemptionForm.code)) {
      toast.error(t("toast.redemptionInvalid"), { description: t("toast.redemptionInvalidCodeFormat") });
      return;
    }
    if (maxRedemptions === undefined) {
      toast.error(t("toast.redemptionInvalid"), { description: t("toast.redemptionInvalidMaxRedemptions") });
      return;
    }
    if (!perUserLimit) {
      toast.error(t("toast.redemptionInvalid"), { description: t("toast.redemptionInvalidPerUserLimit") });
      return;
    }
    if (expiresAt === undefined) {
      toast.error(t("toast.redemptionInvalid"), { description: t("toast.redemptionInvalidExpiresAt") });
      return;
    }
    if (expiresAt !== null && new Date(expiresAt).getTime() <= Date.now()) {
      toast.error(t("toast.redemptionInvalid"), { description: t("toast.redemptionExpiredAtPast") });
      return;
    }
    if (maxRedemptions !== null && perUserLimit > maxRedemptions) {
      toast.error(t("toast.redemptionUserLimitExceedsTotal"));
      return;
    }

    setRedemptionSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }

      if (redemptionForm.id) {
        const data = await updateAdminRedemptionCode(token, redemptionForm.id, {
          status: redemptionForm.status,
          maxRedemptions,
          perUserLimit,
          expiresAt,
          description: redemptionForm.description.trim(),
        });
        setRedemptionCodes((current) => current.map((item) => item.id === data.code.id ? data.code : item));
        setRedemptionForm(null);
        toast.success(t("toast.redemptionUpdated"));
        void loadRedemptionCodes({}, { showLoading: false });
        return;
      }

      const quantity = parseRequiredPositiveInt(redemptionForm.quantity);
      if (!quantity) {
        toast.error(t("toast.redemptionInvalid"), { description: t("toast.redemptionInvalidQuantity") });
        return;
      }
      if (redemptionForm.code.trim() && quantity !== 1) {
        toast.error(t("toast.redemptionManualQuantityInvalid"));
        return;
      }
      const payload = {
        code: redemptionForm.code.trim() || undefined,
        quantity,
        mode: redemptionForm.mode,
        maxRedemptions,
        perUserLimit,
        expiresAt,
        description: redemptionForm.description.trim() || undefined,
      };

      const data = redemptionForm.mode === "usage"
        ? await (async () => {
          const creditUSD = Number(redemptionForm.creditUSD);
          if (!Number.isFinite(creditUSD) || creditUSD <= 0) {
            throw new Error(t("toast.redemptionInvalidCredit"));
          }
          return createAdminRedemptionCodes(token, {
            ...payload,
            creditUSD,
          });
        })()
        : await (async () => {
          const planID = parseRequiredPositiveInt(redemptionForm.planID);
          const durationDays = parseRequiredPositiveInt(redemptionForm.durationDays);
          if (!planID || !durationDays) {
            throw new Error(!planID ? t("toast.redemptionInvalidPlan") : t("toast.redemptionInvalidDuration"));
          }
          return createAdminRedemptionCodes(token, {
            ...payload,
            planID,
            durationDays,
          });
        })();
      const created = data.results ?? [];
      setRedemptionCodes((current) => [...created, ...current].slice(0, redemptionPageSize));
      setRedemptionTotal((current) => current + created.length);
      setCreatedRedemptionCodes(created.map((item) => item.code || "").filter(Boolean));
      setRedemptionForm(null);
      toast.success(t("toast.redemptionCreated", { count: created.length }));
      void loadRedemptionCodes({}, { showLoading: false });
    } catch (error) {
      toast.error(redemptionForm.id ? t("toast.redemptionUpdateFailed") : t("toast.redemptionCreateFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setRedemptionSaving(false);
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
              <TableHead stickyEnd className="w-[88px]" />
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
                          resolveValue={() => fetchRedemptionCodePlaintext(item)}
                          onResolveError={(error) => toast.error(t("toast.redemptionCopyFailed"), { description: resolveAdminErrorMessage(error) })}
                          iconClassName="size-3.5 stroke-1.5"
                          aria-label={tActions("copy")}
                        />
                        {unavailableReason ? (
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <span
                                tabIndex={0}
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
                    <TableCell stickyEnd className="w-[88px] py-1.5 text-right">
                      <div className="flex h-7 items-center justify-end">
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
          <DialogContent className="flex max-h-[min(86vh,760px)] flex-col gap-0 overflow-hidden p-0">
            <DialogHeader className="shrink-0 px-4 py-4">
              <DialogTitle>{redemptionDialogForm.id ? t("redemption.editTitle") : t("redemption.createTitle")}</DialogTitle>
              <DialogDescription>
                {redemptionDialogForm.id ? t("redemption.editDescription") : t("redemption.createDescription")}
              </DialogDescription>
            </DialogHeader>

            <motion.form layout transition={DIALOG_LAYOUT_TRANSITION} onSubmit={(event) => void saveRedemptionCode(event)} className="flex min-h-0 flex-1 flex-col">
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
            </motion.form>
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
        <DialogContent className="flex max-h-[min(86vh,760px)] flex-col gap-0 overflow-hidden p-0">
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
    </section>
  );
}
