"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { invalidateAdminReferenceDataCache, patchAdminBillingConfig, patchAdminSettings } from "@/features/admin/api";
import type { AdminBillingConfigDTO, AdminBillingMode } from "@/features/admin/api/billing-types";
import {
  flattenPaymentSettings,
  formatBillingAmountInput,
  normalizePaymentProviders,
  parseEPayTypesJSON,
  paymentPatchItems,
  type PaymentSettings,
} from "@/features/admin/model/billing-settings";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveApiBaseURL } from "@/shared/api/http-client";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { configuredSettingsMap } from "@/features/admin/utils/settings-meta";

export type BillingDisplayCurrency = "USD" | "CNY";

type UseAdminBillingConfigOptions = {
  billingConfig: AdminBillingConfigDTO | null;
  setBillingConfig: React.Dispatch<React.SetStateAction<AdminBillingConfigDTO | null>>;
  paymentSettings: PaymentSettings;
  setPaymentSettings: React.Dispatch<React.SetStateAction<PaymentSettings>>;
  setSavedPaymentSettings: React.Dispatch<React.SetStateAction<PaymentSettings>>;
  paymentConfiguredMap: Record<string, boolean>;
  setPaymentConfiguredMap: React.Dispatch<React.SetStateAction<Record<string, boolean>>>;
};

// The prepaid amount and exchange-rate drafts live here because both the
// billing-config sync effect and a successful save rewrite them.
export function useAdminBillingConfig({
  billingConfig,
  setBillingConfig,
  paymentSettings,
  setPaymentSettings,
  setSavedPaymentSettings,
  paymentConfiguredMap,
  setPaymentConfiguredMap,
}: UseAdminBillingConfigOptions) {
  const t = useTranslations("adminBilling");
  const [saving, setSaving] = React.useState(false);
  const [billingUsdToCnyRate, setBillingUsdToCnyRate] = React.useState("7.2");
  const [savedBillingUsdToCnyRate, setSavedBillingUsdToCnyRate] = React.useState("7.2");
  const [prepaidAmount, setPrepaidAmount] = React.useState("0");
  const [savedPrepaidAmount, setSavedPrepaidAmount] = React.useState("0");
  const stripeWebhookEndpoint = React.useMemo(() => `${resolveApiBaseURL()}/api/v1/billing/payments/stripe/webhook`, []);

  const billingMode = billingConfig?.mode ?? "self";
  const billingDisplayCurrency: BillingDisplayCurrency = billingConfig?.displayCurrency === "CNY" ? "CNY" : "USD";
  const billingPrepaidAmountUSD = billingConfig?.prepaidAmountUSD;
  const billingUsdToCNYRate = billingConfig?.usdToCNYRate;

  React.useEffect(() => {
    if (billingPrepaidAmountUSD == null || billingUsdToCNYRate == null) {
      return;
    }
    const nextPrepaidAmount = formatBillingAmountInput(billingPrepaidAmountUSD);
    const nextUsdToCnyRate = formatBillingAmountInput(billingUsdToCNYRate);
    setPrepaidAmount(nextPrepaidAmount);
    setSavedPrepaidAmount(nextPrepaidAmount);
    setBillingUsdToCnyRate(nextUsdToCnyRate);
    setSavedBillingUsdToCnyRate(nextUsdToCnyRate);
  }, [billingPrepaidAmountUSD, billingUsdToCNYRate]);

  const prepaidAmountChanged = prepaidAmount.trim() !== savedPrepaidAmount.trim();
  const billingRateChanged = billingUsdToCnyRate.trim() !== savedBillingUsdToCnyRate.trim();

  async function savePaymentSettings() {
    const providers = normalizePaymentProviders(paymentSettings.payment_providers);
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
      toast.error(t("toast.paymentSaveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }

  async function changeBillingMode(nextMode: AdminBillingMode) {
    if (nextMode === billingMode) {
      return;
    }
    const previous = billingMode;
    setBillingConfig((current) => current ? { ...current, mode: nextMode } : current);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        setBillingConfig((current) => current ? { ...current, mode: previous } : current);
        return;
      }
      await patchAdminBillingConfig(token, { mode: nextMode });
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.billingModeChanged", { mode: t(`billingConfig.modes.${nextMode}`) }));
    } catch (error) {
      setBillingConfig((current) => current ? { ...current, mode: previous } : current);
      toast.error(t("toast.billingModeFailed"), { description: resolveAdminErrorMessage(error) });
    }
  }

  async function changeBillingDisplayCurrency(nextCurrency: BillingDisplayCurrency) {
    if (nextCurrency === billingDisplayCurrency) {
      return;
    }
    const previous = billingDisplayCurrency;
    setBillingConfig((current) => current ? { ...current, displayCurrency: nextCurrency } : current);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        setBillingConfig((current) => current ? { ...current, displayCurrency: previous } : current);
        return;
      }
      const result = await patchAdminBillingConfig(token, {
        mode: billingMode,
        displayCurrency: nextCurrency,
      });
      setBillingConfig((current) => current ? { ...current, displayCurrency: result.config.displayCurrency } : result.config);
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.displayCurrencySaved"));
    } catch (error) {
      setBillingConfig((current) => current ? { ...current, displayCurrency: previous } : current);
      toast.error(t("toast.displayCurrencySaveFailed"), { description: resolveAdminErrorMessage(error) });
    }
  }

  async function saveBillingConfig() {
    const amount = Number(prepaidAmount);
    const usdToCnyRate = Number(billingUsdToCnyRate);
    if (billingMode !== "self" && (!Number.isFinite(amount) || amount < 0)) {
      toast.error(t("toast.prepaidInvalid"), { description: t("toast.prepaidInvalidDescription") });
      return;
    }
    if (!Number.isFinite(usdToCnyRate) || usdToCnyRate <= 0) {
      toast.error(t("toast.usdToCnyRateInvalid"), { description: t("toast.usdToCnyRateInvalidDescription") });
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
        prepaidAmountUSD: billingMode !== "self" ? amount : undefined,
        usdToCNYRate: usdToCnyRate,
      });
      const nextAmount = formatBillingAmountInput(result.config.prepaidAmountUSD);
      const nextUsdToCnyRate = formatBillingAmountInput(result.config.usdToCNYRate);
      setPrepaidAmount(nextAmount);
      setSavedPrepaidAmount(nextAmount);
      setBillingUsdToCnyRate(nextUsdToCnyRate);
      setSavedBillingUsdToCnyRate(nextUsdToCnyRate);
      setBillingConfig((current) => current ? {
        ...current,
        mode: result.config.mode,
        prepaidAmountUSD: result.config.prepaidAmountUSD,
        prepaidAmountNanousd: result.config.prepaidAmountNanousd,
        usdToCNYRate: result.config.usdToCNYRate,
        displayCurrency: result.config.displayCurrency,
      } : result.config);
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.billingConfigSaved"));
    } catch (error) {
      toast.error(t("toast.billingConfigSaveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }

  return {
    saving,
    billingMode,
    billingDisplayCurrency,
    billingUsdToCnyRate,
    setBillingUsdToCnyRate,
    prepaidAmount,
    setPrepaidAmount,
    prepaidAmountChanged,
    billingRateChanged,
    stripeWebhookEndpoint,
    savePaymentSettings,
    changeBillingMode,
    changeBillingDisplayCurrency,
    saveBillingConfig,
  };
}
