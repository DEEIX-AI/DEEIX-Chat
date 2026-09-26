"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  getAdminReferenceData,
  listAdminSettingsByNamespace,
  listPermissionGroups,
} from "@/features/admin/api";
import type { PermissionGroup } from "@/features/admin/api/permission-groups";
import type { AdminBillingConfigDTO, AdminBillingPlanDTO, AdminModelPricingDTO } from "@/features/admin/api/billing.types";
import type { AdminLLMModelDTO } from "@/features/admin/api/llm.types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import {
  flattenPaymentSettings,
  PAYMENT_DEFAULTS,
  type PaymentSettings,
} from "@/features/admin/model/billing-settings";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { configuredSettingsMap } from "@/shared/lib/settings-meta";

type UseAdminBillingReferenceState = {
  plans: AdminBillingPlanDTO[];
  setPlans: React.Dispatch<React.SetStateAction<AdminBillingPlanDTO[]>>;
  models: AdminLLMModelDTO[];
  pricingItems: AdminModelPricingDTO[];
  setPricingItems: React.Dispatch<React.SetStateAction<AdminModelPricingDTO[]>>;
  billingConfig: AdminBillingConfigDTO | null;
  setBillingConfig: React.Dispatch<React.SetStateAction<AdminBillingConfigDTO | null>>;
  paymentSettings: PaymentSettings;
  setPaymentSettings: React.Dispatch<React.SetStateAction<PaymentSettings>>;
  savedPaymentSettings: PaymentSettings;
  setSavedPaymentSettings: React.Dispatch<React.SetStateAction<PaymentSettings>>;
  paymentConfiguredMap: Record<string, boolean>;
  setPaymentConfiguredMap: React.Dispatch<React.SetStateAction<Record<string, boolean>>>;
  permissionGroups: PermissionGroup[];
  loading: boolean;
};

const DEFAULT_BILLING_CONFIG: AdminBillingConfigDTO = {
  mode: "self",
  prepaidAmountUSD: 0,
  prepaidAmountNanousd: 0,
  nativeToolBillingEnabled: true,
  nativeToolPricing: [],
  paymentProviders: [],
  usdToCNYRate: 7.2,
  displayCurrency: "USD",
  epayTypes: [],
};

export function useAdminBillingReference(): UseAdminBillingReferenceState {
  const t = useTranslations("adminBilling.toast");
  const [plans, setPlans] = React.useState<AdminBillingPlanDTO[]>([]);
  const [models, setModels] = React.useState<AdminLLMModelDTO[]>([]);
  const [pricingItems, setPricingItems] = React.useState<AdminModelPricingDTO[]>([]);
  const [billingConfig, setBillingConfig] = React.useState<AdminBillingConfigDTO | null>(DEFAULT_BILLING_CONFIG);
  const [paymentSettings, setPaymentSettings] = React.useState<PaymentSettings>(PAYMENT_DEFAULTS);
  const [savedPaymentSettings, setSavedPaymentSettings] = React.useState<PaymentSettings>(PAYMENT_DEFAULTS);
  const [paymentConfiguredMap, setPaymentConfiguredMap] = React.useState<Record<string, boolean>>({});
  const [permissionGroups, setPermissionGroups] = React.useState<PermissionGroup[]>([]);
  const [loading, setLoading] = React.useState(true);

  const reload = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("sessionExpired"), { description: t("sessionExpiredDescription") });
        return;
      }
      const [referenceData, billingSettings, groups] = await Promise.all([
        getAdminReferenceData(token),
        listAdminSettingsByNamespace(token, "billing"),
        listPermissionGroups(token),
      ]);
      const nextPaymentSettings = flattenPaymentSettings(billingSettings);
      const nextPaymentConfiguredMap = configuredSettingsMap({ billing: billingSettings });
      setPermissionGroups(groups);
      setBillingConfig(referenceData.billingConfig.config);
      setPlans(referenceData.billingPlans);
      setModels(referenceData.models);
      setPricingItems(referenceData.modelPricing);
      setPaymentSettings(nextPaymentSettings);
      setSavedPaymentSettings(nextPaymentSettings);
      setPaymentConfiguredMap(nextPaymentConfiguredMap);
    } catch (error) {
      toast.error(t("loadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setLoading(false);
    }
  }, [t]);

  React.useEffect(() => {
    void reload();
  }, [reload]);

  return {
    plans,
    setPlans,
    models,
    pricingItems,
    setPricingItems,
    billingConfig,
    setBillingConfig,
    paymentSettings,
    setPaymentSettings,
    savedPaymentSettings,
    setSavedPaymentSettings,
    paymentConfiguredMap,
    setPaymentConfiguredMap,
    permissionGroups,
    loading,
  };
}
