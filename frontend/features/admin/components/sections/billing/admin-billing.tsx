"use client";

import * as React from "react";
import { Copy, Download, Pencil, Save, Upload } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
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
  invalidateAdminReferenceDataCache,
  listAdminSettingsByNamespace,
  patchAdminBillingConfig,
  patchAdminSettings,
  updateAdminBillingPlan,
  upsertAdminModelPricing,
} from "@/features/admin/api";
import type { AdminBillingMode, AdminBillingPlanDTO, AdminModelPricingDTO } from "@/features/admin/api/billing.types";
import type { AdminLLMModelDTO } from "@/features/admin/api/llm.types";
import { resolveErrorMessage } from "@/features/admin/types/llm";
import {
  DEFAULT_PAGE_SIZE,
  PAGE_SIZE_OPTIONS,
  PAYMENT_DEFAULTS,
  buildModelPricingExportObject,
  buildPricingRows,
  createFormState,
  createPlanFormState,
  flattenPaymentSettings,
  formatDateTime,
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

function shortListDescription(items: string[], emptyText = "", moreLabel = "and"): string {
  if (items.length === 0) {
    return emptyText;
  }
  const visible = items.slice(0, 5).join(", ");
  return items.length > 5 ? `${visible} ${moreLabel} ${items.length}` : visible;
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
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [query, setQuery] = React.useState("");
  const [statusFilter, setStatusFilter] = React.useState("");
  const [freeFilter, setFreeFilter] = React.useState("");
  const [pricingModeFilter, setPricingModeFilter] = React.useState("");
  const [vendorFilter, setVendorFilter] = React.useState("");
  const [billingMode, setBillingMode] = React.useState<AdminBillingMode>("self");
  const [prepaidAmount, setPrepaidAmount] = React.useState("0");
  const [savedPrepaidAmount, setSavedPrepaidAmount] = React.useState("0");
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

  React.useEffect(() => {
    void loadData();
  }, [loadData]);

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

  React.useEffect(() => {
    setPage(1);
  }, [freeFilter, pricingModeFilter, query, statusFilter, vendorFilter]);

  const pageCount = Math.max(1, Math.ceil(filteredRows.length / pageSize));
  const pageRows = React.useMemo(() => {
    const start = (page - 1) * pageSize;
    return filteredRows.slice(start, start + pageSize);
  }, [filteredRows, page, pageSize]);
  const isPaymentDirty = React.useMemo(
    () => paymentSettingsChanged(paymentSettings, savedPaymentSettings),
    [paymentSettings, savedPaymentSettings],
  );
  const paymentProviders = React.useMemo(() => normalizePaymentProviders(paymentSettings.payment_providers), [paymentSettings.payment_providers]);
  const prepaidAmountChanged = prepaidAmount.trim() !== savedPrepaidAmount.trim();
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
    } catch (error) {
      setBillingMode(previous);
      toast.error(t("toast.billingModeFailed"), { description: resolveErrorMessage(error) });
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
      await upsertAdminModelPricing(token, {
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
      });
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.pricingSaved"));
      setEditRow(null);
      setForm(null);
      await loadData();
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
      for (const item of parsed.items) {
        await upsertAdminModelPricing(token, item);
      }
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.imported", { count: parsed.items.length }));
      await loadData();
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
    setFreeSwitchPendingModel(row.platformModelName);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const pricingMode = normalizePricingMode(row.pricing?.pricingMode);
      await upsertAdminModelPricing(token, {
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
      });
      invalidateAdminReferenceDataCache();
      toast.success(checked ? t("toast.freeEnabled") : t("toast.freeDisabled"));
      await loadData();
    } catch (error) {
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
      await updateAdminBillingPlan(token, editPlan.id, {
        name: planForm.name.trim(),
        description: planForm.description.trim(),
        amountUSD: parsePrice(planForm.amount),
        currency: "USD",
        billingInterval: planForm.billingInterval,
        periodCreditUSD: parsePrice(planForm.periodCredit),
        discountPercent: Math.min(100, parseIntValue(planForm.discountPercent)),
      });
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.planSaved"));
      setEditPlan(null);
      setPlanForm(null);
      await loadData();
    } catch (error) {
      toast.error(t("toast.planSaveFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setSaving(false);
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
        <Field>
          <div className="flex">
            <div className="min-w-0 flex-1">
              <FieldLabel htmlFor="billing.usd_to_cny_rate">{t("payment.usdToCnyRate")}</FieldLabel>
              <FieldDescription className="text-[11px]">{t("payment.usdToCnyRateDescription")}</FieldDescription>
            </div>
            <div className="min-w-52 shrink-0">
              <Input
                id="billing.usd_to_cny_rate"
                value={paymentSettings.usd_to_cny_rate}
                className="text-right"
                disabled={loading || saving}
                onChange={(event) => updatePaymentSetting("usd_to_cny_rate", event.target.value)}
              />
            </div>
          </div>
        </Field>

        <div className="pt-4">
          <Tabs value={paymentTab} onValueChange={(value) => setPaymentTab(value as PaymentProvider)}>
            <div className="flex justify-between gap-3">
              <div className="min-w-0 flex-1">
                <FieldLabel>{t("payment.channels")}</FieldLabel>
                <FieldDescription className="text-[11px]">{t("payment.channelsDescription")}</FieldDescription>
              </div>
              <TabsList className="h-8">
                <TabsTrigger value="stripe">Stripe</TabsTrigger>
                <TabsTrigger value="epay">EPay</TabsTrigger>
              </TabsList>
            </div>

            <TabsContent value="stripe" className="mt-4 space-y-4">
              <Field>
                <div className="flex">
                  <div className="min-w-0 flex-1">
                    <FieldLabel>{t("payment.enableStripe")}</FieldLabel>
                    <FieldDescription className="text-[11px]">{t("payment.enableStripeDescription")}</FieldDescription>
                  </div>
                  <Switch size="sm" checked={stripeEnabled} disabled={loading || saving} onCheckedChange={(checked) => setPaymentProviderEnabled("stripe", checked)} />
                </div>
              </Field>
              <Field>
                <div className="flex gap-3">
                  <div className="min-w-0 flex-1">
                    <FieldLabel>{t("payment.stripeWebhookEndpoint")}</FieldLabel>
                    <FieldDescription className="text-[11px]">{t("payment.stripeWebhookEndpointDescription")}</FieldDescription>
                  </div>
                  <div className="flex w-52 shrink-0 items-center gap-1.5">
                    <Input value={stripeWebhookEndpoint} className="h-8 min-w-0 truncate font-mono text-xs" readOnly />
                    <Button type="button" variant="ghost" size="icon" className="size-8 shrink-0 shadow-none" onClick={() => void copyStripeWebhookEndpoint()} aria-label={tActions("copy")} title={tActions("copy")}>
                      <Copy className="size-3.5" />
                    </Button>
                  </div>
                </div>
              </Field>
              <Field>
                <div className="flex">
                  <div className="min-w-0 flex-1">
                    <FieldLabel>{t("payment.stripePublishableKey")}</FieldLabel>
                    <FieldDescription className="text-[11px]">{t("payment.stripePublishableKeyDescription")}</FieldDescription>
                  </div>
                  <div className="min-w-52 shrink-0">
                    <Input value={paymentSettings.stripe_publishable_key} className="text-right" disabled={loading || saving} placeholder="pk_..." onChange={(event) => updatePaymentSetting("stripe_publishable_key", event.target.value)} />
                  </div>
                </div>
              </Field>
              <Field>
                <div className="flex">
                  <div className="min-w-0 flex-1">
                    <FieldLabel>{t("payment.stripeSecretKey")}</FieldLabel>
                    <FieldDescription className="text-[11px]">{t("payment.stripeSecretKeyDescription")}</FieldDescription>
                  </div>
                  <div className="min-w-52 shrink-0">
                    <Input value={paymentSettings.stripe_secret_key} className="text-right" type="password" disabled={loading || saving} placeholder={paymentConfiguredMap["billing.stripe_secret_key"] ? tInput("configuredPasswordPlaceholder") : "sk_..."} onChange={(event) => updatePaymentSetting("stripe_secret_key", event.target.value)} />
                  </div>
                </div>
              </Field>
              <Field>
                <div className="flex">
                  <div className="min-w-0 flex-1">
                    <FieldLabel>{t("payment.stripeWebhookSecret")}</FieldLabel>
                    <FieldDescription className="text-[11px]">{t("payment.stripeWebhookSecretDescription")}</FieldDescription>
                  </div>
                  <div className="min-w-52 shrink-0">
                    <Input value={paymentSettings.stripe_webhook_secret} className="text-right" type="password" disabled={loading || saving} placeholder={paymentConfiguredMap["billing.stripe_webhook_secret"] ? tInput("configuredPasswordPlaceholder") : "whsec_..."} onChange={(event) => updatePaymentSetting("stripe_webhook_secret", event.target.value)} />
                  </div>
                </div>
              </Field>
            </TabsContent>

            <TabsContent value="epay" className="mt-4 space-y-4">
              <Field>
                <div className="flex">
                  <div className="min-w-0 flex-1">
                    <FieldLabel>{t("payment.enableEPay")}</FieldLabel>
                    <FieldDescription className="text-[11px]">{t("payment.enableEPayDescription")}</FieldDescription>
                  </div>
                  <Switch size="sm" checked={epayEnabled} disabled={loading || saving} onCheckedChange={(checked) => setPaymentProviderEnabled("epay", checked)} />
                </div>
              </Field>
              <Field>
                <div className="flex">
                  <div className="min-w-0 flex-1">
                    <FieldLabel>{t("payment.epayGateway")}</FieldLabel>
                    <FieldDescription className="text-[11px]">{t("payment.epayGatewayDescription")}</FieldDescription>
                  </div>
                  <div className="min-w-52 shrink-0">
                    <Input value={paymentSettings.epay_gateway_url} className="text-right" disabled={loading || saving} placeholder="https://..." onChange={(event) => updatePaymentSetting("epay_gateway_url", event.target.value)} />
                  </div>
                </div>
              </Field>
              <Field>
                <div className="flex">
                  <div className="min-w-0 flex-1">
                    <FieldLabel>{t("payment.epayPid")}</FieldLabel>
                    <FieldDescription className="text-[11px]">{t("payment.epayPidDescription")}</FieldDescription>
                  </div>
                  <div className="min-w-52 shrink-0">
                    <Input value={paymentSettings.epay_pid} className="text-right" disabled={loading || saving} onChange={(event) => updatePaymentSetting("epay_pid", event.target.value)} />
                  </div>
                </div>
              </Field>
              <Field>
                <div className="flex">
                  <div className="min-w-0 flex-1">
                    <FieldLabel>{t("payment.epayKey")}</FieldLabel>
                    <FieldDescription className="text-[11px]">{t("payment.epayKeyDescription")}</FieldDescription>
                  </div>
                  <div className="min-w-52 shrink-0">
                    <Input value={paymentSettings.epay_key} className="text-right" type="password" disabled={loading || saving} placeholder={paymentConfiguredMap["billing.epay_key"] ? tInput("configuredPasswordPlaceholder") : ""} onChange={(event) => updatePaymentSetting("epay_key", event.target.value)} />
                  </div>
                </div>
              </Field>
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
            onRefresh={() => void loadData()}
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
                        <TableCell className="py-1">
                          <div className="flex min-w-0 items-center gap-2">
                            <LobeHubIcon iconUrl={iconURL} label={row.platformModelName} />
                            <div className="flex min-w-0 flex-1">
                              <span className="truncate text-xs font-medium leading-5 text-foreground">
                                {row.platformModelName}
                              </span>
                            </div>
                          </div>
                        </TableCell>
                        <TableCell className="py-1">
                          <Switch
                            size="sm"
                            checked={row.isFree}
                            disabled={loading || saving || Boolean(freeSwitchPendingModel)}
                            onCheckedChange={(checked) => void toggleModelFree(row, checked)}
                            aria-label={`${row.platformModelName} ${t("modelPricing.freeModel")}`}
                          />
                        </TableCell>
                        <TableCell className="py-1">
                          {row.pricing ? t(`pricingModes.${normalizePricingMode(row.pricing.pricingMode)}`) : <span className="text-muted-foreground">-</span>}
                        </TableCell>
                        <TableCell className="py-1">
                          <PricingUnitCell pricing={row.pricing} />
                        </TableCell>
                        <TableCell className="py-1 text-muted-foreground">
                          {formatDateTime(row.pricing?.updatedAt ?? "", locale)}
                        </TableCell>
                        <TableCell stickyEnd className="w-[56px] py-1 text-right">
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
    </div>
  );
}
