"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { invalidateAdminReferenceDataCache, listAdminModelPricing, upsertAdminModelPricing } from "@/features/admin/api";
import type { AdminModelPricingDTO } from "@/features/admin/api/billing-types";
import { listAllAdminPages } from "@/features/admin/api/shared";
import {
  createFormState,
  createOptimisticModelPricing,
  mergeModelPricingItem,
  normalizePricingMode,
  parseModelPricingImportJSON,
  parsePrice,
  shortListDescription,
  stringifyTieredPricing,
  type BillingModelPricingRow,
  type PricingFormState,
} from "@/features/admin/model/billing-settings";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { stringifySchedulePricing, type SchedulePeriodPayload } from "@/entities/billing";

type UseAdminBillingPricingOptions = {
  pricingItems: AdminModelPricingDTO[];
  setPricingItems: React.Dispatch<React.SetStateAction<AdminModelPricingDTO[]>>;
};

export function useAdminBillingPricing({ pricingItems, setPricingItems }: UseAdminBillingPricingOptions) {
  const t = useTranslations("adminBilling");
  const [saving, setSaving] = React.useState(false);
  const [modelPricingRefreshing, setModelPricingRefreshing] = React.useState(false);
  const [freeSwitchPendingModel, setFreeSwitchPendingModel] = React.useState("");

  async function loadModelPricing() {
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
      toast.error(t("toast.loadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setModelPricingRefreshing(false);
    }
  }

  /** Resolves `true` once the pricing is persisted so the caller can close its editor. */
  async function savePricing(form: PricingFormState, schedulePeriods: SchedulePeriodPayload[]): Promise<boolean> {
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return false;
      }
      const payload = {
        platformModelName: form.platformModelName,
        currency: "USD",
        pricingMode: form.pricingMode,
        inputUSDPerMTokens: form.pricingMode === "token" ? parsePrice(form.input) : 0,
        cacheReadUSDPerMTokens: form.pricingMode === "token" ? parsePrice(form.cacheRead) : 0,
        cacheWriteUSDPerMTokens: form.pricingMode === "token" ? parsePrice(form.cacheWrite) : 0,
        cacheWritePriceBasis: form.cacheWritePriceBasis,
        outputUSDPerMTokens: form.pricingMode === "token" ? parsePrice(form.output) : 0,
        callUSDPerCall: form.pricingMode === "call" ? parsePrice(form.call) : 0,
        durationUSDPerSecond: form.pricingMode === "duration" ? parsePrice(form.duration) : 0,
        tieredPricingJSON: form.pricingMode === "tiered" ? stringifyTieredPricing(form.tieredTiers) : undefined,
        schedulePricingJSON: stringifySchedulePricing(schedulePeriods),
        isFree: form.isFree,
      };
      const data = await upsertAdminModelPricing(token, payload);
      setPricingItems((current) => mergeModelPricingItem(current, data.modelPricing));
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.pricingSaved"));
      return true;
    } catch (error) {
      toast.error(t("toast.pricingSaveFailed"), { description: resolveAdminErrorMessage(error) });
      return false;
    } finally {
      setSaving(false);
    }
  }

  async function importModelPricingFile(file: File, rows: BillingModelPricingRow[]) {
    setSaving(true);
    try {
      const raw = await file.text();
      const validNames = new Set(rows.map((row) => row.platformModelName));
      const videoGenerationNames = new Set(
        rows.filter((row) => row.supportsVideoGeneration).map((row) => row.platformModelName),
      );
      const parsed = parseModelPricingImportJSON(raw, validNames, videoGenerationNames, {
        invalidJSON: t("importErrors.invalidJSON"),
        rootObject: t("importErrors.rootObject"),
        emptyModelName: t("importErrors.emptyModelName"),
        duplicateModel: (model) => t("importErrors.duplicateModel", { model }),
        pricingObject: (model) => t("importErrors.pricingObject", { model }),
        invalidPricingMode: (model) => t("importErrors.invalidPricingMode", { model }),
        durationVideoOnly: (model) => t("importErrors.durationVideoOnly", { model }),
        invalidNumber: (model, field) => t("importErrors.invalidNumber", { model, field }),
        invalidTieredPricing: (model, field) => t("importErrors.invalidTieredPricing", { model, field }),
        invalidTieredPricingJSON: (model) => t("importErrors.invalidTieredPricingJSON", { model }),
        invalidSchedulePricing: (model) => t("importErrors.invalidSchedulePricing", { model }),
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
      toast.error(t("toast.importFailed"), { description: resolveAdminErrorMessage(error) });
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
        cacheWritePriceBasis: row.pricing?.cacheWritePriceBasis,
        outputUSDPerMTokens: pricingMode === "token" ? row.pricing?.outputUSDPerMTokens ?? 0 : 0,
        callUSDPerCall: pricingMode === "call" ? row.pricing?.callUSDPerCall ?? 0 : 0,
        durationUSDPerSecond: pricingMode === "duration" ? row.pricing?.durationUSDPerSecond ?? 0 : 0,
        tieredPricingJSON: pricingMode === "tiered" ? row.pricing?.tieredPricingJSON || stringifyTieredPricing(createFormState(row).tieredTiers) : undefined,
        schedulePricingJSON: row.pricing?.schedulePricingJSON || undefined,
        isFree: checked,
      };
      setPricingItems((current) => mergeModelPricingItem(current, createOptimisticModelPricing(row, payload)));
      const data = await upsertAdminModelPricing(token, payload);
      setPricingItems((current) => mergeModelPricingItem(current, data.modelPricing));
      invalidateAdminReferenceDataCache();
      toast.success(checked ? t("toast.freeEnabled") : t("toast.freeDisabled"));
    } catch (error) {
      setPricingItems(previousPricingItems);
      toast.error(t("toast.freeSaveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setFreeSwitchPendingModel("");
    }
  }

  return {
    saving,
    modelPricingRefreshing,
    freeSwitchPendingModel,
    loadModelPricing,
    savePricing,
    importModelPricingFile,
    toggleModelFree,
  };
}
