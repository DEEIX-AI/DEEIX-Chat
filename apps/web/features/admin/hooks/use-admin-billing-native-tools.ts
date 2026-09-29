"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { invalidateAdminReferenceDataCache, patchAdminBillingConfig } from "@/features/admin/api";
import type {
  AdminBillingConfigDTO,
  AdminNativeToolPricingPayload,
  NativeToolPricingDTO,
} from "@/features/admin/api/billing-types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export function formatNativeToolPriceInput(priceNanousd: number): string {
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
    label: item.label,
    description: item.description,
    type: item.type,
    priceNanousd: item.priceNanousd,
    unit: item.unit,
    priceLabel: item.priceLabel,
    billable: item.billable,
  })).sort((left, right) => left.toolKey.localeCompare(right.toolKey)));
}

function normalizeNativeToolPricingForSave(items: NativeToolPricingDTO[]): AdminNativeToolPricingPayload[] {
  return items.map((item) => ({
    toolKey: item.toolKey,
    priceNanousd: item.priceNanousd,
    unit: "call",
    priceLabel: "",
    billable: item.priceNanousd > 0,
  }));
}

type UseAdminBillingNativeToolsOptions = {
  billingConfig: AdminBillingConfigDTO | null;
  setBillingConfig: React.Dispatch<React.SetStateAction<AdminBillingConfigDTO | null>>;
};

// Drafts live here rather than in the section because a successful save
// rewrites them from the server response.
export function useAdminBillingNativeTools({ billingConfig, setBillingConfig }: UseAdminBillingNativeToolsOptions) {
  const t = useTranslations("adminBilling");
  const [nativeToolBillingEnabled, setNativeToolBillingEnabled] = React.useState(true);
  const [savedNativeToolBillingEnabled, setSavedNativeToolBillingEnabled] = React.useState(true);
  const [nativeToolPricing, setNativeToolPricing] = React.useState<NativeToolPricingDTO[]>([]);
  const [savedNativeToolPricing, setSavedNativeToolPricing] = React.useState<NativeToolPricingDTO[]>([]);
  const [nativeToolPriceDrafts, setNativeToolPriceDrafts] = React.useState<Record<string, string>>({});
  const [nativeToolBillingSaving, setNativeToolBillingSaving] = React.useState(false);
  const billingNativeToolBillingEnabled = billingConfig?.nativeToolBillingEnabled;
  const billingNativeToolPricing = billingConfig?.nativeToolPricing;

  React.useEffect(() => {
    if (billingNativeToolBillingEnabled == null) {
      return;
    }
    const nextEnabled = Boolean(billingNativeToolBillingEnabled);
    const nextPricing = billingNativeToolPricing ?? [];
    setNativeToolBillingEnabled(nextEnabled);
    setSavedNativeToolBillingEnabled(nextEnabled);
    setNativeToolPricing(nextPricing);
    setSavedNativeToolPricing(nextPricing);
    setNativeToolPriceDrafts(nativeToolPriceDraftsFrom(nextPricing));
  }, [billingNativeToolBillingEnabled, billingNativeToolPricing]);

  const nativeToolBillingChanged = nativeToolBillingEnabled !== savedNativeToolBillingEnabled;
  const nativeToolPricingChanged = React.useMemo(
    () => nativeToolPricingSignature(nativeToolPricing) !== nativeToolPricingSignature(savedNativeToolPricing),
    [nativeToolPricing, savedNativeToolPricing],
  );

  function updateNativeToolPriceDraft(toolKey: string, nextDraft: string) {
    const nextNanousd = nativeToolPriceInputToNanousd(nextDraft);
    setNativeToolPriceDrafts((current) => ({
      ...current,
      [toolKey]: nextDraft,
    }));
    if (nextNanousd === null) {
      return;
    }
    setNativeToolPricing((current) => current.map((item) => (
      item.toolKey === toolKey
        ? { ...item, priceNanousd: nextNanousd, unit: "call", priceLabel: "", billable: nextNanousd > 0 }
        : item
    )));
  }

  async function saveNativeToolBilling() {
    setNativeToolBillingSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const nextNativeToolPricing = normalizeNativeToolPricingForSave(nativeToolPricing);
      const result = await patchAdminBillingConfig(token, {
        mode: billingConfig?.mode ?? "self",
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
      setBillingConfig((current) => current ? {
        ...current,
        nativeToolBillingEnabled: savedValue,
        nativeToolPricing: savedPricing,
      } : result.config);
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.nativeToolBillingSaved"));
    } catch (error) {
      toast.error(t("toast.nativeToolBillingSaveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setNativeToolBillingSaving(false);
    }
  }

  return {
    nativeToolBillingEnabled,
    setNativeToolBillingEnabled,
    nativeToolPricing,
    nativeToolPriceDrafts,
    nativeToolBillingSaving,
    nativeToolBillingChanged,
    nativeToolPricingChanged,
    updateNativeToolPriceDraft,
    saveNativeToolBilling,
  };
}
