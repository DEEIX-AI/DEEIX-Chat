"use client";

import * as React from "react";
import { Check, CircleAlert, Copy, Download, Pencil, Plus, Save, Trash2, Upload, X } from "lucide-react";
import { motion } from "motion/react";
import { useLocale, useTranslations } from "next-intl";
import { toast } from "sonner";

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
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field";
import { AdminDateTimePicker, adminDateTimeFormValue, adminDateTimeValueToISOString } from "@/features/admin/components/admin-date-time-picker";
import { AdminBulkConfirmDialog } from "@/features/admin/components/bulk-confirm-dialog";
import { PlanBillingDialog, PricingBillingDialog } from "@/features/admin/components/sections/billing/billing-dialogs";
import { PeriodBillingTable, PricingUnitCell } from "@/features/admin/components/sections/billing/billing-tables";
import {
  Table,
  TableBody,
  TableCell,
  TableEmptyRow,
  TableHead,
  TableHeader,
  TableRow,
  TableSkeletonRows,
} from "@/components/ui/table";
import { TablePagination, TableToolbar } from "@/components/ui/table-tools";
import {
  SettingsFieldItem,
  SettingsFieldList,
  SettingsFieldRow,
  SettingsSection,
} from "@/shared/components/settings-layout";
import {
  getAdminReferenceData,
  batchDeleteAdminRedemptionCodes,
  createAdminRedemptionCodes,
  deleteAdminRedemptionCode,
  invalidateAdminReferenceDataCache,
  listAdminRedemptionCodes,
  listAdminModelPricing,
  listAdminSettingsByNamespace,
  patchAdminBillingConfig,
  patchAdminSettings,
  revealAdminRedemptionCode,
  updateAdminRedemptionCode,
  updateAdminBillingPlan,
  upsertAdminModelPricing,
} from "@/features/admin/api";
import { listAllAdminPages } from "@/features/admin/api/shared";
import type { AdminBillingMode, AdminBillingPlanDTO, AdminModelPricingDTO, AdminRedemptionCodeDTO, NativeToolPricingDTO } from "@/features/admin/api/billing.types";
import type { AdminLLMModelDTO } from "@/features/admin/api/llm.types";
import { resolveErrorMessage } from "@/features/admin/types/llm";
import { cn } from "@/lib/utils";
import {
  DEFAULT_PAGE_SIZE,
  PAGE_SIZE_OPTIONS,
  PAYMENT_DEFAULTS,
  buildModelPricingExportObject,
  buildPricingRows,
  createFormState,
  createOptimisticModelPricing,
  createPlanFormState,
  flattenPaymentSettings,
  formatCreditUSD,
  formatDateTime,
  mergeModelPricingItem,
  normalizePaymentProviders,
  normalizePricingMode,
  parseModelPricingImportJSON,
  parseEPayTypesJSON,
  parseIntValue,
  parsePrice,
  paymentPatchItems,
  paymentProviderSetting,
  paymentSettingsChanged,
  stringifyTieredPricing,
  type BillingModelPricingRow,
  type PaymentProvider,
  type PaymentSettings,
  type PlanFormState,
  type PricingFormState,
  type TieredPricingTierForm,
} from "@/features/admin/model/billing-page";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { resolveApiBaseURL } from "@/shared/api/http-client";
import { LobeHubIcon } from "@/shared/components/lobehub-icon";
import { configuredSettingsMap } from "@/shared/lib/settings-meta";
import { KNOWN_VENDOR_OPTIONS, resolveLobeHubIconURL, resolveModelIdentity } from "@/shared/lib/model-identity";

function formatBillingAmountInput(value: number | null | undefined): string {
  if (!Number.isFinite(value ?? NaN) || (value ?? 0) <= 0) {
    return "0";
  }
  return String(value);
}

function modelPricingExportFilename(): string {
  const date = new Date().toISOString().slice(0, 10);
  return `deeix-chat-model-pricing-${date}.json`;
}

function redemptionCodesExportFilename(): string {
  const date = new Date().toISOString().slice(0, 10);
  return `deeix-chat-redemption-codes-${date}.json`;
}

function formatNativeToolPriceInput(priceNanousd: number): string {
  if (!Number.isFinite(priceNanousd) || priceNanousd <= 0) {
    return "0";
  }
  return String(priceNanousd / 1_000_000_000);
}

function nativeToolPriceInputToNanousd(value: string): number | null {
  const parsed = Number(value.trim());
  if (!Number.isFinite(parsed) || parsed < 0) {
    return null;
  }
  return Math.round(parsed * 1_000_000_000);
}

function nativeToolPriceDraftsFrom(items: NativeToolPricingDTO[]): Record<string, string> {
  return Object.fromEntries(items.map((item) => [item.toolKey, formatNativeToolPriceInput(item.priceNanousd)]));
}

function nativeToolPricingSignature(items: NativeToolPricingDTO[]): string {
  return JSON.stringify(items.map((item) => ({
    toolKey: item.toolKey,
    priceNanousd: item.priceNanousd,
    unit: item.unit,
    priceLabel: item.priceLabel,
    billable: item.billable,
  })).sort((left, right) => left.toolKey.localeCompare(right.toolKey)));
}

function normalizeNativeToolPricingForSave(items: NativeToolPricingDTO[]): NativeToolPricingDTO[] {
  return items.map((item) => ({
    ...item,
    unit: "call",
    priceLabel: "",
    billable: item.priceNanousd > 0,
  }));
}

function downloadJSONFile(filename: string, value: unknown): void {
  const blob = new Blob([JSON.stringify(value, null, 2)], { type: "application/json;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

const DIALOG_LAYOUT_TRANSITION = {
  layout: {
    duration: 0.22,
    ease: [0.16, 1, 0.3, 1] as const,
  },
};

function shortListDescription(items: string[], emptyText = "", moreLabel = "and"): string {
  if (items.length === 0) {
    return emptyText;
  }
  const visible = items.slice(0, 5).join(", ");
  return items.length > 5 ? `${visible} ${moreLabel} ${items.length}` : visible;
}

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

function redemptionExpiresFormValue(value: string | null | undefined): string {
  return adminDateTimeFormValue(value);
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

export function AdminBillingPage() {
  const locale = useLocale();
  const t = useTranslations("adminBilling");
  const tActions = useTranslations("common.actions");
  const tCommonErrors = useTranslations("common.errors");
  const tInput = useTranslations("common.input");
  const importPricingInputRef = React.useRef<HTMLInputElement | null>(null);
  const [plans, setPlans] = React.useState<AdminBillingPlanDTO[]>([]);
  const [models, setModels] = React.useState<AdminLLMModelDTO[]>([]);
  const [pricingItems, setPricingItems] = React.useState<AdminModelPricingDTO[]>([]);
  const [redemptionCodes, setRedemptionCodes] = React.useState<AdminRedemptionCodeDTO[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [redemptionLoading, setRedemptionLoading] = React.useState(false);
  const [modelPricingRefreshing, setModelPricingRefreshing] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [query, setQuery] = React.useState("");
  const [statusFilter, setStatusFilter] = React.useState("");
  const [freeFilter, setFreeFilter] = React.useState("");
  const [pricingModeFilter, setPricingModeFilter] = React.useState("");
  const [vendorFilter, setVendorFilter] = React.useState("");
  const [redemptionQuery, setRedemptionQuery] = React.useState("");
  const [redemptionModeFilter, setRedemptionModeFilter] = React.useState("");
  const [redemptionStatusFilter, setRedemptionStatusFilter] = React.useState("");
  const [redemptionAvailabilityFilter, setRedemptionAvailabilityFilter] = React.useState("");
  const [redemptionPage, setRedemptionPage] = React.useState(1);
  const [redemptionPageSize, setRedemptionPageSize] = React.useState(DEFAULT_PAGE_SIZE);
  const [redemptionTotal, setRedemptionTotal] = React.useState(0);
  const [billingMode, setBillingMode] = React.useState<AdminBillingMode>("self");
  const [prepaidAmount, setPrepaidAmount] = React.useState("0");
  const [savedPrepaidAmount, setSavedPrepaidAmount] = React.useState("0");
  const [nativeToolBillingEnabled, setNativeToolBillingEnabled] = React.useState(true);
  const [savedNativeToolBillingEnabled, setSavedNativeToolBillingEnabled] = React.useState(true);
  const [nativeToolPricing, setNativeToolPricing] = React.useState<NativeToolPricingDTO[]>([]);
  const [savedNativeToolPricing, setSavedNativeToolPricing] = React.useState<NativeToolPricingDTO[]>([]);
  const [nativeToolPriceDrafts, setNativeToolPriceDrafts] = React.useState<Record<string, string>>({});
  const [nativeToolBillingSaving, setNativeToolBillingSaving] = React.useState(false);
  const [paymentSettings, setPaymentSettings] = React.useState<PaymentSettings>(PAYMENT_DEFAULTS);
  const [savedPaymentSettings, setSavedPaymentSettings] = React.useState<PaymentSettings>(PAYMENT_DEFAULTS);
  const [paymentConfiguredMap, setPaymentConfiguredMap] = React.useState<Record<string, boolean>>({});
  const [paymentTab, setPaymentTab] = React.useState<PaymentProvider>("stripe");
  const [freeSwitchPendingModel, setFreeSwitchPendingModel] = React.useState("");
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(DEFAULT_PAGE_SIZE);
  const [editRow, setEditRow] = React.useState<BillingModelPricingRow | null>(null);
  const [form, setForm] = React.useState<PricingFormState | null>(null);
  const [editPlan, setEditPlan] = React.useState<AdminBillingPlanDTO | null>(null);
  const [planForm, setPlanForm] = React.useState<PlanFormState | null>(null);
  const [redemptionForm, setRedemptionForm] = React.useState<RedemptionFormState | null>(null);
  const [redemptionSaving, setRedemptionSaving] = React.useState(false);
  const [selectedRedemptionIDs, setSelectedRedemptionIDs] = React.useState<Set<number>>(new Set());
  const [redemptionBulkAction, setRedemptionBulkAction] = React.useState<RedemptionBulkAction | null>(null);
  const [redemptionBulkPending, setRedemptionBulkPending] = React.useState(false);
  const [redemptionDeleteTarget, setRedemptionDeleteTarget] = React.useState<AdminRedemptionCodeDTO | null>(null);
  const [createdRedemptionCodes, setCreatedRedemptionCodes] = React.useState<string[]>([]);
  const [redemptionCopyingID, setRedemptionCopyingID] = React.useState<number | null>(null);
  const [redemptionStatusPendingID, setRedemptionStatusPendingID] = React.useState<number | null>(null);
  const stripeWebhookEndpoint = React.useMemo(() => `${resolveApiBaseURL()}/api/v1/billing/payments/stripe/webhook`, []);

  const copyStripeWebhookEndpoint = React.useCallback(async () => {
    try {
      await navigator.clipboard.writeText(stripeWebhookEndpoint);
      toast.success(tActions("copied"));
    } catch {
      toast.error(tCommonErrors("copyFailed"));
    }
  }, [stripeWebhookEndpoint, tActions, tCommonErrors]);

  const loadData = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const [referenceData, billingSettings] = await Promise.all([
        getAdminReferenceData(token),
        listAdminSettingsByNamespace(token, "billing"),
      ]);
      const nextPaymentSettings = flattenPaymentSettings(billingSettings);
      const nextPaymentConfiguredMap = configuredSettingsMap({ billing: billingSettings });
      const nextPrepaidAmount = formatBillingAmountInput(referenceData.billingConfig.config.prepaidAmountUSD);
      setBillingMode(referenceData.billingConfig.config.mode);
      setNativeToolBillingEnabled(Boolean(referenceData.billingConfig.config.nativeToolBillingEnabled));
      setSavedNativeToolBillingEnabled(Boolean(referenceData.billingConfig.config.nativeToolBillingEnabled));
      setNativeToolPricing(referenceData.billingConfig.config.nativeToolPricing ?? []);
      setSavedNativeToolPricing(referenceData.billingConfig.config.nativeToolPricing ?? []);
      setNativeToolPriceDrafts(nativeToolPriceDraftsFrom(referenceData.billingConfig.config.nativeToolPricing ?? []));
      setPrepaidAmount(nextPrepaidAmount);
      setSavedPrepaidAmount(nextPrepaidAmount);
      setPlans(referenceData.billingPlans);
      setModels(referenceData.models);
      setPricingItems(referenceData.modelPricing);
      setPaymentSettings(nextPaymentSettings);
      setSavedPaymentSettings(nextPaymentSettings);
      setPaymentConfiguredMap(nextPaymentConfiguredMap);
    } catch (error) {
      toast.error(t("toast.loadFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setLoading(false);
    }
  }, [t]);

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
        toast.error(t("toast.redemptionLoadFailed"), { description: resolveErrorMessage(error) });
      }
    } finally {
      if (showLoading) {
        setRedemptionLoading(false);
      }
    }
  }, [redemptionAvailabilityFilter, redemptionModeFilter, redemptionPage, redemptionPageSize, redemptionQuery, redemptionStatusFilter, t]);

  const loadModelPricing = React.useCallback(async () => {
    setModelPricingRefreshing(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const items = await listAllAdminPages((options) => listAdminModelPricing(token, options));
      setPricingItems(items);
      invalidateAdminReferenceDataCache();
    } catch (error) {
      toast.error(t("toast.loadFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setModelPricingRefreshing(false);
    }
  }, [t]);

  React.useEffect(() => {
    void loadData();
  }, [loadData]);

  React.useEffect(() => {
    void loadRedemptionCodes();
  }, [loadRedemptionCodes]);

  const rows = React.useMemo(() => buildPricingRows(models, pricingItems), [models, pricingItems]);
  const vendorFilterOptions = React.useMemo(() => {
    const options = new Map(KNOWN_VENDOR_OPTIONS.map((item) => [item.value, item.label]));
    for (const row of rows) {
      const value = row.vendor.trim();
      if (!value || options.has(value)) {
        continue;
      }
      const identity = resolveModelIdentity({
        code: row.platformModelName,
        vendor: value,
        icon: row.icon,
      });
      options.set(value, identity.vendorLabel);
    }
    return Array.from(options.entries()).map(([value, label]) => ({ value, label }));
  }, [rows]);
  const filteredRows = React.useMemo(() => {
    const keyword = query.trim().toLowerCase();
    return rows.filter((row) => {
      const matchesQuery =
        !keyword ||
        row.platformModelName.toLowerCase().includes(keyword) ||
        row.vendor.toLowerCase().includes(keyword);
      const matchesStatus =
        statusFilter === "" ||
        (statusFilter === "configured" && row.pricing) ||
        (statusFilter === "unconfigured" && !row.pricing);
      const matchesFree =
        freeFilter === "" ||
        (freeFilter === "free" && row.isFree) ||
        (freeFilter === "not_free" && !row.isFree);
      const matchesPricingMode =
        pricingModeFilter === "" ||
        Boolean(row.pricing && normalizePricingMode(row.pricing.pricingMode) === pricingModeFilter);
      const matchesVendor = vendorFilter === "" || row.vendor.trim() === vendorFilter;
      return matchesQuery && matchesStatus && matchesFree && matchesPricingMode && matchesVendor;
    });
  }, [freeFilter, pricingModeFilter, query, rows, statusFilter, vendorFilter]);
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

  React.useEffect(() => {
    setPage(1);
  }, [freeFilter, pricingModeFilter, query, statusFilter, vendorFilter]);

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

  const pageCount = Math.max(1, Math.ceil(filteredRows.length / pageSize));
  const pageRows = React.useMemo(() => {
    const start = (page - 1) * pageSize;
    return filteredRows.slice(start, start + pageSize);
  }, [filteredRows, page, pageSize]);
  const redemptionPageCount = Math.max(1, Math.ceil(redemptionTotal / redemptionPageSize));
  const redemptionTableLoading = loading || redemptionLoading;
  const isPaymentDirty = React.useMemo(
    () => paymentSettingsChanged(paymentSettings, savedPaymentSettings),
    [paymentSettings, savedPaymentSettings],
  );
  const paymentProviders = React.useMemo(() => normalizePaymentProviders(paymentSettings.payment_providers), [paymentSettings.payment_providers]);
  const prepaidAmountChanged = prepaidAmount.trim() !== savedPrepaidAmount.trim();
  const nativeToolBillingChanged = nativeToolBillingEnabled !== savedNativeToolBillingEnabled;
  const nativeToolPricingChanged = React.useMemo(
    () => nativeToolPricingSignature(nativeToolPricing) !== nativeToolPricingSignature(savedNativeToolPricing),
    [nativeToolPricing, savedNativeToolPricing],
  );
  const billingConfigActions = billingMode !== "self" && prepaidAmountChanged ? (
    <Button
      type="button"
      size="sm"
      disabled={loading || saving}
      onClick={() => void handlePrepaidAmountSave()}
    >
            {saving ? <SpinnerLabel>{tActions("saving")}</SpinnerLabel> : (
        <>
          <Save className="size-3.5" />
          {tActions("save")}
        </>
      )}
    </Button>
  ) : null;
  const toolPricingActions = nativeToolBillingChanged || nativeToolPricingChanged ? (
    <Button
      type="button"
      size="sm"
      disabled={loading || nativeToolBillingSaving}
      onClick={() => void handleNativeToolBillingSave()}
    >
      {nativeToolBillingSaving ? <SpinnerLabel>{tActions("saving")}</SpinnerLabel> : (
        <>
          <Save className="size-3.5" />
          {tActions("save")}
        </>
      )}
    </Button>
  ) : null;
  const stripeEnabled = paymentProviders.includes("stripe");
  const epayEnabled = paymentProviders.includes("epay");

  function openEdit(row: BillingModelPricingRow) {
    setEditRow(row);
    setForm(createFormState(row));
  }

  function updateTieredTier(index: number, patch: Partial<TieredPricingTierForm>) {
    setForm((current) => {
      if (!current) return current;
      return {
        ...current,
        tieredTiers: current.tieredTiers.map((tier, tierIndex) =>
          tierIndex === index ? { ...tier, ...patch } : tier,
        ),
      };
    });
  }

  function addTieredTier() {
    setForm((current) => {
      if (!current) return current;
      return {
        ...current,
        tieredTiers: [
          ...current.tieredTiers,
          {
            id: `new-${Date.now()}-${current.tieredTiers.length}`,
            upToKTokens: "0",
            input: "0",
            cacheRead: "0",
            cacheWrite: "0",
            output: "0",
          },
        ],
      };
    });
  }

  function removeTieredTier(index: number) {
    setForm((current) => {
      if (!current || current.tieredTiers.length <= 1) return current;
      return {
        ...current,
        tieredTiers: current.tieredTiers.filter((_, tierIndex) => tierIndex !== index),
      };
    });
  }

  function openPlanEdit(plan: AdminBillingPlanDTO) {
    setEditPlan(plan);
    setPlanForm(createPlanFormState(plan));
  }

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

  async function copyCreatedRedemptionCodes() {
    if (createdRedemptionCodes.length === 0) return;
    await copyRedemptionText(createdRedemptionCodes.join("\n"));
  }

  async function copyRedemptionText(value: string) {
    if (!value.trim()) return;
    try {
      await navigator.clipboard.writeText(value);
      toast.success(tActions("copied"));
    } catch {
      toast.error(tCommonErrors("copyFailed"));
    }
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

  async function copyStoredRedemptionCode(item: AdminRedemptionCodeDTO) {
    setRedemptionCopyingID(item.id);
    try {
      const code = await fetchRedemptionCodePlaintext(item);
      await copyRedemptionText(code);
    } catch (error) {
      toast.error(t("toast.redemptionCopyFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setRedemptionCopyingID(null);
    }
  }

  async function copySelectedRedemptionCodes() {
    setRedemptionBulkPending(true);
    try {
      const { results, failedCount } = await revealSelectedRedemptionCodes();
      if (results.length === 0) return;
      await navigator.clipboard.writeText(results.map((result) => result.code).join("\n"));
      toast.success(t("toast.redemptionBulkCopied", { count: results.length }), {
        description: failedCount > 0 ? t("toast.redemptionBulkRevealSkipped", { count: failedCount }) : undefined,
      });
    } catch (error) {
      toast.error(t("toast.redemptionBulkCopyFailed"), { description: resolveErrorMessage(error) });
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
      toast.error(t("toast.redemptionBulkExportFailed"), { description: resolveErrorMessage(error) });
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
      const updatedCodes = await Promise.all(ids.map((id) => updateAdminRedemptionCode(token, id, { status })));
      setRedemptionCodes((current) => current.map((item) => updatedCodes.find((data) => data.code.id === item.id)?.code ?? item));
      setSelectedRedemptionIDs(new Set());
      setRedemptionBulkAction(null);
      toast.success(status === "active" ? t("toast.redemptionBulkEnabled", { count: ids.length }) : t("toast.redemptionBulkDisabled", { count: ids.length }));
      void loadRedemptionCodes({}, { showLoading: false });
    } catch (error) {
      setRedemptionCodes(previousRedemptionCodes);
      toast.error(t("toast.redemptionBulkFailed"), { description: resolveErrorMessage(error) });
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
      toast.error(t("toast.redemptionUpdateFailed"), { description: resolveErrorMessage(error) });
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
      const result = await batchDeleteAdminRedemptionCodes(token, { ids });
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
      toast.error(t("toast.redemptionDeleteFailed"), { description: resolveErrorMessage(error) });
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
      toast.error(t("toast.redemptionDeleteFailed"), { description: resolveErrorMessage(error) });
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
      toast.error(redemptionForm.id ? t("toast.redemptionUpdateFailed") : t("toast.redemptionCreateFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setRedemptionSaving(false);
    }
  }

  function updatePaymentSetting(key: keyof PaymentSettings, value: string) {
    setPaymentSettings((current) => ({ ...current, [key]: value }));
  }

  function setPaymentProviderEnabled(provider: PaymentProvider, enabled: boolean) {
    setPaymentSettings((current) => {
      const providers = normalizePaymentProviders(current.payment_providers);
      const next = enabled
        ? Array.from(new Set([...providers, provider]))
        : providers.filter((item) => item !== provider);
      return { ...current, payment_providers: paymentProviderSetting(next) };
    });
  }

  async function savePaymentSettings() {
    const providers = normalizePaymentProviders(paymentSettings.payment_providers);
    const usdToCnyRate = Number(paymentSettings.usd_to_cny_rate);
    if (providers.length > 0 && (!Number.isFinite(usdToCnyRate) || usdToCnyRate <= 0)) {
      toast.error(t("toast.paymentIncomplete"), { description: t("toast.paymentRateRequired") });
      return;
    }
    if (providers.includes("stripe") && ((!paymentSettings.stripe_secret_key.trim() && !paymentConfiguredMap["billing.stripe_secret_key"]) || (!paymentSettings.stripe_webhook_secret.trim() && !paymentConfiguredMap["billing.stripe_webhook_secret"]))) {
      toast.error(t("toast.paymentIncomplete"), { description: t("toast.stripeRequired") });
      return;
    }
    if (providers.includes("epay") && (!paymentSettings.epay_gateway_url.trim() || !paymentSettings.epay_types.trim() || !paymentSettings.epay_pid.trim() || (!paymentSettings.epay_key.trim() && !paymentConfiguredMap["billing.epay_key"]))) {
      toast.error(t("toast.paymentIncomplete"), { description: t("toast.epayRequired") });
      return;
    }
    if (providers.includes("epay") && !parseEPayTypesJSON(paymentSettings.epay_types)) {
      toast.error(t("toast.paymentIncomplete"), { description: t("toast.epayTypesInvalid") });
      return;
    }

    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const grouped = await patchAdminSettings(token, { items: paymentPatchItems(paymentSettings) });
      const next = flattenPaymentSettings(grouped.billing || []);
      setPaymentConfiguredMap(configuredSettingsMap(grouped));
      setPaymentSettings(next);
      setSavedPaymentSettings(next);
      toast.success(t("toast.paymentSaved"));
    } catch (error) {
      toast.error(t("toast.paymentSaveFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }

  async function handleBillingModeChange(nextMode: AdminBillingMode) {
    if (nextMode === billingMode) {
      return;
    }
    const previous = billingMode;
    setBillingMode(nextMode);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        setBillingMode(previous);
        return;
      }
      await patchAdminBillingConfig(token, { mode: nextMode });
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.billingModeChanged", { mode: t(`billingConfig.modes.${nextMode}`) }));
      if (redemptionAvailabilityFilter === "available") {
        void loadRedemptionCodes({}, { showLoading: false });
      }
    } catch (error) {
      setBillingMode(previous);
      toast.error(t("toast.billingModeFailed"), { description: resolveErrorMessage(error) });
    }
  }

  async function handleNativeToolBillingSave() {
    setNativeToolBillingSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const nextNativeToolPricing = normalizeNativeToolPricingForSave(nativeToolPricing);
      const result = await patchAdminBillingConfig(token, {
        mode: billingMode,
        nativeToolBillingEnabled,
        nativeToolPricing: nextNativeToolPricing,
      });
      const savedValue = Boolean(result.config.nativeToolBillingEnabled);
      const savedPricing = result.config.nativeToolPricing ?? nativeToolPricing;
      setNativeToolBillingEnabled(savedValue);
      setSavedNativeToolBillingEnabled(savedValue);
      setNativeToolPricing(savedPricing);
      setSavedNativeToolPricing(savedPricing);
      setNativeToolPriceDrafts(nativeToolPriceDraftsFrom(savedPricing));
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.nativeToolBillingSaved"));
    } catch (error) {
      toast.error(t("toast.nativeToolBillingSaveFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setNativeToolBillingSaving(false);
    }
  }

  async function handlePrepaidAmountSave() {
    const amount = Number(prepaidAmount);
    if (!Number.isFinite(amount) || amount < 0) {
      toast.error(t("toast.prepaidInvalid"), { description: t("toast.prepaidInvalidDescription") });
      return;
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const result = await patchAdminBillingConfig(token, {
        mode: billingMode,
        prepaidAmountUSD: amount,
      });
      const nextAmount = formatBillingAmountInput(result.config.prepaidAmountUSD);
      setPrepaidAmount(nextAmount);
      setSavedPrepaidAmount(nextAmount);
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.prepaidSaved"));
    } catch (error) {
      toast.error(t("toast.prepaidSaveFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }

  async function savePricing(event?: React.FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    if (!form) return;
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const payload = {
        platformModelName: form.platformModelName,
        currency: "USD",
        pricingMode: form.pricingMode,
        inputUSDPerMTokens: form.pricingMode === "token" ? parsePrice(form.input) : 0,
        cacheReadUSDPerMTokens: form.pricingMode === "token" ? parsePrice(form.cacheRead) : 0,
        cacheWriteUSDPerMTokens: form.pricingMode === "token" ? parsePrice(form.cacheWrite) : 0,
        outputUSDPerMTokens: form.pricingMode === "token" ? parsePrice(form.output) : 0,
        callUSDPerCall: form.pricingMode === "call" ? parsePrice(form.call) : 0,
        durationUSDPerSecond: form.pricingMode === "duration" ? parsePrice(form.duration) : 0,
        tieredPricingJSON: form.pricingMode === "tiered" ? stringifyTieredPricing(form.tieredTiers) : undefined,
        isFree: form.isFree,
      };
      const data = await upsertAdminModelPricing(token, payload);
      setPricingItems((current) => mergeModelPricingItem(current, data.modelPricing));
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.pricingSaved"));
      setEditRow(null);
      setForm(null);
    } catch (error) {
      toast.error(t("toast.pricingSaveFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }

  function exportModelPricing() {
    const payload = buildModelPricingExportObject(pricingItems);
    const count = Object.keys(payload).length;
    if (count === 0) {
      toast.error(t("toast.exportEmpty"));
      return;
    }
    downloadJSONFile(modelPricingExportFilename(), payload);
    toast.success(t("toast.exported", { count }));
  }

  async function importModelPricingFile(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0] ?? null;
    event.target.value = "";
    if (!file) {
      return;
    }

    setSaving(true);
    try {
      const raw = await file.text();
      const validNames = new Set(rows.map((row) => row.platformModelName));
      const parsed = parseModelPricingImportJSON(raw, validNames, {
        invalidJSON: t("importErrors.invalidJSON"),
        rootObject: t("importErrors.rootObject"),
        emptyModelName: t("importErrors.emptyModelName"),
        duplicateModel: (model) => t("importErrors.duplicateModel", { model }),
        pricingObject: (model) => t("importErrors.pricingObject", { model }),
        invalidPricingMode: (model) => t("importErrors.invalidPricingMode", { model }),
        invalidNumber: (model, field) => t("importErrors.invalidNumber", { model, field }),
        invalidTieredPricing: (model, field) => t("importErrors.invalidTieredPricing", { model, field }),
        invalidTieredPricingJSON: (model) => t("importErrors.invalidTieredPricingJSON", { model }),
      });
      if (parsed.unknownModelNames.length > 0) {
        toast.error(t("toast.importUnknownModels"), {
          description: shortListDescription(parsed.unknownModelNames, "", t("toast.moreItems")),
        });
        return;
      }
      if (parsed.errors.length > 0) {
        toast.error(t("toast.importInvalidJSON"), {
          description: shortListDescription(parsed.errors, "", t("toast.moreItems")),
        });
        return;
      }
      if (parsed.items.length === 0) {
        toast.error(t("toast.importEmpty"));
        return;
      }

      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const savedItems: AdminModelPricingDTO[] = [];
      for (const item of parsed.items) {
        const data = await upsertAdminModelPricing(token, item);
        savedItems.push(data.modelPricing);
      }
      setPricingItems((current) => savedItems.reduce((items, item) => mergeModelPricingItem(items, item), current));
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.imported", { count: parsed.items.length }));
    } catch (error) {
      toast.error(t("toast.importFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }

  async function toggleModelFree(row: BillingModelPricingRow, checked: boolean) {
    if (freeSwitchPendingModel) {
      return;
    }
    const previousPricingItems = pricingItems;
    setFreeSwitchPendingModel(row.platformModelName);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const pricingMode = normalizePricingMode(row.pricing?.pricingMode);
      const payload = {
        platformModelName: row.platformModelName,
        currency: row.pricing?.currency || "USD",
        pricingMode,
        inputUSDPerMTokens: pricingMode === "token" ? row.pricing?.inputUSDPerMTokens ?? 0 : 0,
        cacheReadUSDPerMTokens: pricingMode === "token" ? row.pricing?.cacheReadUSDPerMTokens ?? 0 : 0,
        cacheWriteUSDPerMTokens: pricingMode === "token" ? row.pricing?.cacheWriteUSDPerMTokens ?? 0 : 0,
        outputUSDPerMTokens: pricingMode === "token" ? row.pricing?.outputUSDPerMTokens ?? 0 : 0,
        callUSDPerCall: pricingMode === "call" ? row.pricing?.callUSDPerCall ?? 0 : 0,
        durationUSDPerSecond: pricingMode === "duration" ? row.pricing?.durationUSDPerSecond ?? 0 : 0,
        tieredPricingJSON: pricingMode === "tiered" ? row.pricing?.tieredPricingJSON || stringifyTieredPricing(createFormState(row).tieredTiers) : undefined,
        isFree: checked,
      };
      setPricingItems((current) => mergeModelPricingItem(current, createOptimisticModelPricing(row, payload)));
      const data = await upsertAdminModelPricing(token, payload);
      setPricingItems((current) => mergeModelPricingItem(current, data.modelPricing));
      invalidateAdminReferenceDataCache();
      toast.success(checked ? t("toast.freeEnabled") : t("toast.freeDisabled"));
    } catch (error) {
      setPricingItems(previousPricingItems);
      toast.error(t("toast.freeSaveFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setFreeSwitchPendingModel("");
    }
  }

  async function savePlan(event?: React.FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    if (!editPlan || !planForm) return;
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const data = await updateAdminBillingPlan(token, editPlan.id, {
        name: planForm.name.trim(),
        description: planForm.description.trim(),
        amountUSD: parsePrice(planForm.amount),
        currency: "USD",
        billingInterval: planForm.billingInterval,
        periodCreditUSD: parsePrice(planForm.periodCredit),
        discountPercent: Math.min(100, parseIntValue(planForm.discountPercent)),
      });
      setPlans((current) => current.map((plan) => plan.id === data.plan.id ? data.plan : plan));
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.planSaved"));
      setEditPlan(null);
      setPlanForm(null);
    } catch (error) {
      toast.error(t("toast.planSaveFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setSaving(false);
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
    if (billingMode !== codeMode) {
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

  const paymentConfigSection = (
    <section className="space-y-6 px-1">
      <div className="flex h-10 items-center justify-between gap-3">
        <h3 className="text-sm font-semibold">{t("payment.title")}</h3>
        {isPaymentDirty ? (
          <Button type="button" size="sm" onClick={() => void savePaymentSettings()} disabled={loading || saving}>
            {saving ? <SpinnerLabel>{tActions("saving")}</SpinnerLabel> : (
              <>
                <Save className="size-3.5" />
                {tActions("save")}
              </>
            )}
          </Button>
        ) : null}
      </div>

      <FieldGroup className="gap-0">
        <SettingsFieldRow
          title={t("payment.usdToCnyRate")}
          description={t("payment.usdToCnyRateDescription")}
        >
          <Input
            id="billing.usd_to_cny_rate"
            value={paymentSettings.usd_to_cny_rate}
            className="text-right"
            disabled={loading || saving}
            onChange={(event) => updatePaymentSetting("usd_to_cny_rate", event.target.value)}
          />
        </SettingsFieldRow>

        <div className="pt-4">
          <Tabs value={paymentTab} onValueChange={(value) => setPaymentTab(value as PaymentProvider)}>
            <SettingsFieldRow
              title={t("payment.channels")}
              description={t("payment.channelsDescription")}
            >
              <TabsList className="h-8 w-full">
                <TabsTrigger value="stripe">Stripe</TabsTrigger>
                <TabsTrigger value="epay">EPay</TabsTrigger>
              </TabsList>
            </SettingsFieldRow>

            <TabsContent value="stripe" className="mt-4 space-y-4">
              <SettingsFieldRow
                title={t("payment.enableStripe")}
                description={t("payment.enableStripeDescription")}
              >
                <Switch size="sm" checked={stripeEnabled} disabled={loading || saving} onCheckedChange={(checked) => setPaymentProviderEnabled("stripe", checked)} />
              </SettingsFieldRow>
              <SettingsFieldRow
                title={t("payment.stripeWebhookEndpoint")}
                description={t("payment.stripeWebhookEndpointDescription")}
              >
                <div className="grid w-full min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-1">
                  <Input value={stripeWebhookEndpoint} className="min-w-0 truncate text-left text-xs md:text-right" readOnly />
                  <Button type="button" variant="secondary" size="icon" className="size-8 shrink-0 rounded-md shadow-none active:scale-90 transition-transform" onClick={() => void copyStripeWebhookEndpoint()} aria-label={tActions("copy")} title={tActions("copy")}>
                    <Copy className="size-3.5" />
                  </Button>
                </div>
              </SettingsFieldRow>
              <SettingsFieldRow
                title={t("payment.stripePublishableKey")}
                description={t("payment.stripePublishableKeyDescription")}
              >
                <Input value={paymentSettings.stripe_publishable_key} className="text-right" disabled={loading || saving} placeholder="pk_..." onChange={(event) => updatePaymentSetting("stripe_publishable_key", event.target.value)} />
              </SettingsFieldRow>
              <SettingsFieldRow
                title={t("payment.stripeSecretKey")}
                description={t("payment.stripeSecretKeyDescription")}
              >
                <Input value={paymentSettings.stripe_secret_key} className="text-right" type="password" disabled={loading || saving} placeholder={paymentConfiguredMap["billing.stripe_secret_key"] ? tInput("configuredPasswordPlaceholder") : "sk_..."} onChange={(event) => updatePaymentSetting("stripe_secret_key", event.target.value)} />
              </SettingsFieldRow>
              <SettingsFieldRow
                title={t("payment.stripeWebhookSecret")}
                description={t("payment.stripeWebhookSecretDescription")}
              >
                <Input value={paymentSettings.stripe_webhook_secret} className="text-right" type="password" disabled={loading || saving} placeholder={paymentConfiguredMap["billing.stripe_webhook_secret"] ? tInput("configuredPasswordPlaceholder") : "whsec_..."} onChange={(event) => updatePaymentSetting("stripe_webhook_secret", event.target.value)} />
              </SettingsFieldRow>
            </TabsContent>

            <TabsContent value="epay" className="mt-4 space-y-4">
              <SettingsFieldRow
                title={t("payment.enableEPay")}
                description={t("payment.enableEPayDescription")}
              >
                <Switch size="sm" checked={epayEnabled} disabled={loading || saving} onCheckedChange={(checked) => setPaymentProviderEnabled("epay", checked)} />
              </SettingsFieldRow>
              <SettingsFieldRow
                title={t("payment.epayGateway")}
                description={t("payment.epayGatewayDescription")}
              >
                <Input value={paymentSettings.epay_gateway_url} className="text-right" disabled={loading || saving} placeholder="https://..." onChange={(event) => updatePaymentSetting("epay_gateway_url", event.target.value)} />
              </SettingsFieldRow>
              <SettingsFieldRow
                title={t("payment.epayPid")}
                description={t("payment.epayPidDescription")}
              >
                <Input value={paymentSettings.epay_pid} className="text-right" disabled={loading || saving} onChange={(event) => updatePaymentSetting("epay_pid", event.target.value)} />
              </SettingsFieldRow>
              <SettingsFieldRow
                title={t("payment.epayKey")}
                description={t("payment.epayKeyDescription")}
              >
                <Input value={paymentSettings.epay_key} className="text-right" type="password" disabled={loading || saving} placeholder={paymentConfiguredMap["billing.epay_key"] ? tInput("configuredPasswordPlaceholder") : ""} onChange={(event) => updatePaymentSetting("epay_key", event.target.value)} />
              </SettingsFieldRow>
              <Field>
                <div className="space-y-2">
                  <div>
                    <FieldLabel>{t("payment.epayTypes")}</FieldLabel>
                    <FieldDescription className="text-[11px]">{t("payment.epayTypesDescription")}</FieldDescription>
                  </div>
                  <Textarea
                    value={paymentSettings.epay_types}
                    className="h-28 w-full resize-none overflow-y-auto font-mono [field-sizing:fixed]"
                    disabled={loading || saving}
                    spellCheck={false}
                    onChange={(event) => updatePaymentSetting("epay_types", event.target.value)}
                  />
                </div>
              </Field>
            </TabsContent>
          </Tabs>
        </div>
      </FieldGroup>
    </section>
  );

  return (
    <div className="space-y-8 pb-10">
      <SettingsSection title={t("billingConfig.title")} actions={billingConfigActions} className="px-1">
        <SettingsFieldList>
          <SettingsFieldItem>
            <SettingsFieldRow
              title={t("billingConfig.mode")}
              description={t("billingConfig.modeDescription")}
            >
              <div className="w-full">
                <Select value={billingMode} onValueChange={(value) => void handleBillingModeChange(value as AdminBillingMode)} disabled={loading || saving}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent align="end">
                    <SelectItem value="self">{t("billingConfig.modes.self")}</SelectItem>
                    <SelectItem value="period">{t("billingConfig.modes.period")}</SelectItem>
                    <SelectItem value="usage">{t("billingConfig.modes.usage")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </SettingsFieldRow>
          </SettingsFieldItem>
          {billingMode !== "self" ? (
            <SettingsFieldItem index={1}>
              <SettingsFieldRow
                title={t("billingConfig.prepaidAmount")}
                description={t("billingConfig.prepaidAmountDescription")}
              >
                <div className="w-full">
                  <Input
                    type="number"
                    min={0}
                    step="0.01"
                    value={prepaidAmount}
                    className="text-right"
                    disabled={loading || saving}
                    onChange={(event) => setPrepaidAmount(event.target.value)}
                  />
                </div>
              </SettingsFieldRow>
            </SettingsFieldItem>
          ) : null}
        </SettingsFieldList>
      </SettingsSection>

      <Separator className="mx-1 my-10" />

      {paymentConfigSection}

      <Separator className="mx-1 my-10" />

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
                icon: <Copy className="size-3.5 stroke-1" />,
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

          <Table>
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
              {redemptionTableLoading ? <TableSkeletonRows colSpan={8} rowCount={4} /> : null}
              {!redemptionTableLoading && redemptionCodes.length === 0 ? <TableEmptyRow colSpan={8}>{t("redemption.empty")}</TableEmptyRow> : null}
              {!redemptionTableLoading
                ? redemptionCodes.map((item) => {
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
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon-xs"
                            className="h-6 w-6 text-muted-foreground shadow-none"
                            disabled={redemptionCopyingID === item.id}
                            onClick={() => void copyStoredRedemptionCode(item)}
                            aria-label={tActions("copy")}
                          >
                            <Copy className="size-3.5 stroke-1.5" />
                          </Button>
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
            </TableBody>
          </Table>

          <TablePagination
            total={redemptionTotal}
            page={redemptionPage}
            pageCount={redemptionPageCount}
            pageSize={redemptionPageSize}
            pageSizeOptions={PAGE_SIZE_OPTIONS}
            onPageChange={setRedemptionPage}
            onPageSizeChange={(next) => {
              setRedemptionPageSize(next);
              setRedemptionPage(1);
            }}
            loading={redemptionTableLoading}
          />
        </div>
      </section>

      <Separator className="mx-1 my-10" />

      <section className="space-y-6 px-1">
        <div className="flex h-10 items-center">
          <h3 className="text-sm font-semibold">{t("plans.title")}</h3>
        </div>
        <PeriodBillingTable plans={plans} loading={loading} onEdit={openPlanEdit} />
      </section>

      <Separator className="mx-1 my-10" />

      <section className="space-y-6 px-1">
        <div className="flex h-10 items-center">
          <h3 className="text-sm font-semibold">{t("modelPricing.title")}</h3>
        </div>
        <div className="space-y-3">
          <TableToolbar
            query={query}
            onQueryChange={setQuery}
            queryPlaceholder={t("modelPricing.searchPlaceholder")}
            filters={[
              {
                key: "status",
                label: t("modelPricing.filterLabel"),
                value: statusFilter,
                onValueChange: setStatusFilter,
                options: [
                  { label: t("modelPricing.all"), value: "" },
                  { label: t("modelPricing.configured"), value: "configured" },
                  { label: t("modelPricing.unconfigured"), value: "unconfigured" },
                ],
              },
              {
                key: "free",
                label: t("modelPricing.freeFilterLabel"),
                value: freeFilter,
                onValueChange: setFreeFilter,
                options: [
                  { label: t("modelPricing.allFreeStatus"), value: "" },
                  { label: t("modelPricing.freeOnly"), value: "free" },
                  { label: t("modelPricing.notFree"), value: "not_free" },
                ],
              },
              {
                key: "pricingMode",
                label: t("modelPricing.pricingMode"),
                value: pricingModeFilter,
                onValueChange: setPricingModeFilter,
                options: [
                  { label: t("modelPricing.allPricingModes"), value: "" },
                  { label: t("pricingModes.token"), value: "token" },
                  { label: t("pricingModes.call"), value: "call" },
                  { label: t("pricingModes.duration"), value: "duration" },
                  { label: t("pricingModes.tiered"), value: "tiered" },
                ],
              },
              {
                key: "vendor",
                label: t("modelPricing.vendor"),
                value: vendorFilter,
                onValueChange: setVendorFilter,
                options: [
                  { label: t("modelPricing.allVendors"), value: "" },
                  ...vendorFilterOptions,
                ],
              },
            ]}
            loading={loading}
            onRefresh={() => void loadModelPricing()}
            refreshDisabled={loading || saving || modelPricingRefreshing}
            refreshLoading={modelPricingRefreshing}
          >
            <input
              ref={importPricingInputRef}
              type="file"
              accept="application/json,.json"
              className="hidden"
              onChange={(event) => void importModelPricingFile(event)}
            />
            <Button
              type="button"
              size="icon-sm"
              variant="ghost"
              className="size-8 text-muted-foreground shadow-none hover:bg-muted hover:text-foreground"
              disabled={loading || saving}
              onClick={exportModelPricing}
              aria-label={t("actions.exportPricing")}
              title={t("actions.exportPricing")}
            >
              <Download className="size-3.5 stroke-1" />
            </Button>
            <Button
              type="button"
              size="icon-sm"
              variant="ghost"
              className="size-8 text-muted-foreground shadow-none hover:bg-muted hover:text-foreground"
              disabled={loading || saving}
              onClick={() => importPricingInputRef.current?.click()}
              aria-label={t("actions.importPricing")}
              title={t("actions.importPricing")}
            >
              <Upload className="size-3.5 stroke-1" />
            </Button>
          </TableToolbar>

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="min-w-[210px]">{t("modelPricing.platformModel")}</TableHead>
                <TableHead>{t("modelPricing.free")}</TableHead>
                <TableHead>{t("modelPricing.pricingMode")}</TableHead>
                <TableHead className="min-w-[260px]">{t("modelPricing.basePrice")}</TableHead>
                <TableHead>{t("modelPricing.updatedAt")}</TableHead>
                <TableHead stickyEnd className="w-[56px]" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? <TableSkeletonRows colSpan={6} rowCount={10} /> : null}
              {!loading && pageRows.length === 0 ? <TableEmptyRow colSpan={6}>{t("modelPricing.empty")}</TableEmptyRow> : null}
              {!loading
                ? pageRows.map((row) => {
                    const identity = resolveModelIdentity({
                      code: row.platformModelName,
                      vendor: row.vendor,
                      icon: row.icon,
                    });
                    const iconURL = resolveLobeHubIconURL(identity.modelIcon);

                    return (
                      <TableRow key={row.platformModelName}>
                        <TableCell className="py-1.5">
                          <div className="flex h-7 min-w-0 items-center gap-2">
                            <LobeHubIcon iconUrl={iconURL} label={row.platformModelName} />
                            <div className="flex min-w-0 flex-1">
                              <span className="truncate text-xs font-medium leading-5 text-foreground">
                                {row.platformModelName}
                              </span>
                            </div>
                          </div>
                        </TableCell>
                        <TableCell className="py-1.5">
                          <div className="flex h-7 items-center">
                            <Switch
                              size="sm"
                              checked={row.isFree}
                              disabled={loading || saving || Boolean(freeSwitchPendingModel)}
                              onCheckedChange={(checked) => void toggleModelFree(row, checked)}
                              aria-label={`${row.platformModelName} ${t("modelPricing.freeModel")}`}
                            />
                          </div>
                        </TableCell>
                        <TableCell className="py-1.5">
                          {row.pricing ? t(`pricingModes.${normalizePricingMode(row.pricing.pricingMode)}`) : <span className="text-muted-foreground">-</span>}
                        </TableCell>
                        <TableCell className="py-1.5">
                          <PricingUnitCell pricing={row.pricing} />
                        </TableCell>
                        <TableCell className="py-1.5 text-muted-foreground">
                          {formatDateTime(row.pricing?.updatedAt ?? "", locale)}
                        </TableCell>
                        <TableCell stickyEnd className="w-[56px] py-1.5 text-right">
                          <div className="flex h-7 items-center justify-end">
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon-xs"
                              className="h-7 w-7 text-muted-foreground shadow-none"
                              onClick={() => openEdit(row)}
                              aria-label={t("actions.editPricing")}
                            >
                              <Pencil className="size-3.5 stroke-1" />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    );
                  })
                : null}
            </TableBody>
          </Table>

          <TablePagination
            total={filteredRows.length}
            page={page}
            pageCount={pageCount}
            pageSize={pageSize}
            pageSizeOptions={PAGE_SIZE_OPTIONS}
            onPageChange={setPage}
            onPageSizeChange={(next) => {
              setPageSize(next);
              setPage(1);
            }}
            loading={loading}
          />
        </div>
      </section>

      <Separator className="mx-1 my-10" />

      <SettingsSection title={t("toolPricing.title")} actions={toolPricingActions} className="px-1">
        <SettingsFieldList>
          <SettingsFieldItem>
            <SettingsFieldRow
              title={t("toolPricing.nativeToolBilling")}
              description={t("toolPricing.nativeToolBillingDescription")}
            >
              <Switch
                checked={nativeToolBillingEnabled}
                disabled={loading || nativeToolBillingSaving}
                onCheckedChange={setNativeToolBillingEnabled}
                aria-label={t("toolPricing.nativeToolBilling")}
              />
            </SettingsFieldRow>
          </SettingsFieldItem>
        </SettingsFieldList>
        <div className="mt-5 space-y-2">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("toolPricing.provider")}</TableHead>
                <TableHead>{t("toolPricing.tool")}</TableHead>
                <TableHead className="text-right">{t("toolPricing.price")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {nativeToolPricing.map((row) => (
                <TableRow key={`${row.provider}-${row.toolKey}`}>
                  <TableCell className="py-1.5 text-xs text-muted-foreground">{row.provider}</TableCell>
                  <TableCell className="py-1.5 text-xs text-foreground">{t(`toolPricing.tools.${row.toolKey}`)}</TableCell>
                  <TableCell className="py-1.5 text-right font-mono text-xs text-muted-foreground">
                    <div className="flex items-center justify-end gap-1.5">
                      <span className="text-muted-foreground">$</span>
                      <Input
                        value={nativeToolPriceDrafts[row.toolKey] ?? formatNativeToolPriceInput(row.priceNanousd)}
                        inputMode="decimal"
                        className="h-7 w-24 text-right font-mono text-xs"
                        disabled={loading || nativeToolBillingSaving}
                        aria-label={`${t(`toolPricing.tools.${row.toolKey}`)} ${t("toolPricing.price")}`}
                        onChange={(event) => {
                          const nextDraft = event.target.value;
                          const nextNanousd = nativeToolPriceInputToNanousd(nextDraft);
                          setNativeToolPriceDrafts((current) => ({
                            ...current,
                            [row.toolKey]: nextDraft,
                          }));
                          if (nextNanousd === null) {
                            return;
                          }
                          setNativeToolPricing((current) => current.map((item) => (
                            item.toolKey === row.toolKey
                              ? { ...item, priceNanousd: nextNanousd, unit: "call", priceLabel: "", billable: nextNanousd > 0 }
                              : item
                          )));
                        }}
                      />
                      <span className="whitespace-nowrap text-muted-foreground">
                        / {t("toolPricing.units.call")}
                      </span>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          <p className="text-[11px] leading-5 text-muted-foreground">{t("toolPricing.defaultPriceDescription")}</p>
          <p className="text-[11px] leading-5 text-muted-foreground">{t("toolPricing.note")}</p>
        </div>
      </SettingsSection>

      <PlanBillingDialog
        open={!!editPlan && !!planForm}
        saving={saving}
        planForm={planForm}
        setPlanForm={setPlanForm}
        onOpenChange={(open) => {
          if (!open && !saving) {
            setEditPlan(null);
            setPlanForm(null);
          }
        }}
        onCancel={() => setEditPlan(null)}
        onSubmit={savePlan}
      />

      <PricingBillingDialog
        open={!!editRow && !!form}
        saving={saving}
        form={form}
        setForm={setForm}
        onOpenChange={(open) => {
          if (!open && !saving) {
            setEditRow(null);
            setForm(null);
          }
        }}
        onCancel={() => setEditRow(null)}
        onSubmit={savePricing}
        onAddTier={addTieredTier}
        onRemoveTier={removeTieredTier}
        onUpdateTier={updateTieredTier}
      />

      <Dialog
        open={!!redemptionForm}
        onOpenChange={(open) => {
          if (!open && !redemptionSaving) {
            setRedemptionForm(null);
          }
        }}
      >
        {redemptionForm ? (
          <DialogContent className="flex max-h-[min(86vh,760px)] flex-col gap-0 overflow-hidden p-0">
            <DialogHeader className="shrink-0 px-4 py-4">
              <DialogTitle>{redemptionForm.id ? t("redemption.editTitle") : t("redemption.createTitle")}</DialogTitle>
              <DialogDescription>
                {redemptionForm.id ? t("redemption.editDescription") : t("redemption.createDescription")}
              </DialogDescription>
            </DialogHeader>

            <motion.form layout transition={DIALOG_LAYOUT_TRANSITION} onSubmit={(event) => void saveRedemptionCode(event)} className="flex min-h-0 flex-1 flex-col">
              <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-2">
                {!redemptionForm.id ? (
                  <div className="grid grid-cols-2 gap-5">
                    <div className="space-y-1">
                      <p className="text-xs text-muted-foreground">{t("redemption.code")}</p>
                      <Input
                        id="redemption-code"
                        value={redemptionForm.code}
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
                        value={redemptionForm.quantity}
                        disabled={redemptionSaving || Boolean(redemptionForm.code.trim())}
                        onChange={(event) => setRedemptionForm((current) => current ? { ...current, quantity: event.target.value } : current)}
                      />
                    </div>
                  </div>
                ) : null}

                <div className={cn("grid gap-5", redemptionForm.id && "grid-cols-2")}>
                  <div className="space-y-1">
                    <p className="text-xs text-muted-foreground">{t("redemption.mode")}</p>
                    <Select
                      value={redemptionForm.mode}
                      disabled={redemptionSaving || Boolean(redemptionForm.id)}
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

                  {redemptionForm.id ? (
                    <div className="space-y-1">
                      <p className="text-xs text-muted-foreground">{t("redemption.status")}</p>
                      <div className="flex h-8 items-center px-1">
                        <Switch
                          size="sm"
                          checked={redemptionForm.status === "active"}
                          disabled={redemptionSaving}
                          onCheckedChange={(checked) => setRedemptionForm((current) => current ? { ...current, status: checked ? "active" : "inactive" } : current)}
                          aria-label={redemptionForm.status === "active" ? t("redemption.disable") : t("redemption.enable")}
                        />
                      </div>
                    </div>
                  ) : null}
                </div>

                {redemptionForm.mode === "usage" ? (
                  <div className="space-y-1">
                    <p className="text-xs text-muted-foreground">{t("redemption.creditUSD")}</p>
                    <Input
                      id="redemption-credit"
                      type="number"
                      min="0"
                      step="0.01"
                      value={redemptionForm.creditUSD}
                      disabled={redemptionSaving || Boolean(redemptionForm.id)}
                      onChange={(event) => setRedemptionForm((current) => current ? { ...current, creditUSD: event.target.value } : current)}
                    />
                  </div>
                ) : (
                  <div className="grid grid-cols-2 gap-5">
                    <div className="space-y-1">
                      <p className="text-xs text-muted-foreground">{t("redemption.plan")}</p>
                      <Select
                        value={redemptionForm.planID}
                        disabled={redemptionSaving || Boolean(redemptionForm.id) || activePlanOptions.length === 0}
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
                        value={redemptionForm.durationDays}
                        disabled={redemptionSaving || Boolean(redemptionForm.id)}
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
                      value={redemptionForm.maxRedemptions}
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
                      max={redemptionForm.maxRedemptions.trim() || undefined}
                      value={redemptionForm.perUserLimit}
                      disabled={redemptionSaving}
                      onChange={(event) => setRedemptionForm((current) => current ? { ...current, perUserLimit: event.target.value } : current)}
                    />
                  </div>
                </div>

                <AdminDateTimePicker
                  value={redemptionForm.expiresAt}
                  disabled={redemptionSaving}
                  label={t("redemption.expiresAt")}
                  placeholder={t("redemption.never")}
                  onChange={(value) => setRedemptionForm((current) => current ? { ...current, expiresAt: value } : current)}
                />

                <div className="space-y-1">
                  <p className="text-xs text-muted-foreground">{t("redemption.description")}</p>
                  <Textarea
                    id="redemption-description"
                    value={redemptionForm.description}
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
              <Button type="button" variant="outline" size="sm" className="h-7 gap-1 px-2 text-xs shadow-none" onClick={() => void copyCreatedRedemptionCodes()}>
                <Copy className="size-3.5" />
                {t("redemption.copyAll")}
              </Button>
            </div>
            <div className="max-h-72 space-y-2 overflow-y-auto pr-1">
              {createdRedemptionCodes.map((code) => (
                <div key={code} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-2 rounded-md border border-border/60 bg-muted/25 px-3 py-2">
                  <span className="min-w-0 break-all font-mono text-xs">{code}</span>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    className="text-muted-foreground"
                    onClick={() => void copyRedemptionText(code)}
                    aria-label={tActions("copy")}
                  >
                    <Copy className="size-3.5" />
                  </Button>
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
        title={redemptionBulkConfirmTitle(redemptionBulkAction)}
        description={t("redemption.bulkConfirmDescription", { count: selectedRedemptionIDs.size })}
        confirmLabel={redemptionBulkConfirmLabel(redemptionBulkAction)}
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
    </div>
  );
}
