"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { invalidateAdminReferenceDataCache, updateAdminBillingPlan } from "@/features/admin/api";
import type { AdminBillingPlanDTO } from "@/features/admin/api/billing-types";
import { parsePrice, type PlanFormState } from "@/features/admin/model/billing-settings";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

type UseAdminBillingPlansOptions = {
  setPlans: React.Dispatch<React.SetStateAction<AdminBillingPlanDTO[]>>;
};

export function useAdminBillingPlans({ setPlans }: UseAdminBillingPlansOptions) {
  const t = useTranslations("adminBilling");
  const [saving, setSaving] = React.useState(false);

  /** Resolves `true` once the plan is persisted so the caller can close its editor. */
  async function savePlan(planID: number, planForm: PlanFormState): Promise<boolean> {
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return false;
      }
      const data = await updateAdminBillingPlan(token, planID, {
        name: planForm.name.trim(),
        description: planForm.description.trim(),
        amountUSD: parsePrice(planForm.amount),
        currency: "USD",
        billingInterval: planForm.billingInterval,
        periodCreditUSD: parsePrice(planForm.periodCredit),
        permissionGroupID: Number(planForm.permissionGroupID) || undefined,
      });
      setPlans((current) => current.map((plan) => plan.id === data.plan.id ? data.plan : plan));
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.planSaved"));
      return true;
    } catch (error) {
      toast.error(t("toast.planSaveFailed"), { description: resolveAdminErrorMessage(error) });
      return false;
    } finally {
      setSaving(false);
    }
  }

  return { saving, savePlan };
}
