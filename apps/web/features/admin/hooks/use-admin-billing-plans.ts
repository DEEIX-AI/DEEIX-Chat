"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  createAdminBillingPlan,
  deleteAdminBillingPlan,
  invalidateAdminReferenceDataCache,
  updateAdminBillingPlan,
} from "@/features/admin/api";
import type { AdminBillingPlanDTO } from "@/features/admin/api/billing-types";
import { buildPlanPayload, createPlanFormState, type PlanFormState } from "@/features/admin/model/billing-settings";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

type UseAdminBillingPlansOptions = {
  setPlans: React.Dispatch<React.SetStateAction<AdminBillingPlanDTO[]>>;
};

function sortPlans(plans: AdminBillingPlanDTO[]): AdminBillingPlanDTO[] {
  return [...plans].sort((left, right) => (left.sortOrder - right.sortOrder) || (left.id - right.id));
}

export function useAdminBillingPlans({ setPlans }: UseAdminBillingPlansOptions) {
  const t = useTranslations("adminBilling");
  const [saving, setSaving] = React.useState(false);

  async function withToken<T>(run: (token: string) => Promise<T>): Promise<T | null> {
    const token = await resolveAccessToken();
    if (!token) {
      toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
      return null;
    }
    return run(token);
  }

  function upsertPlan(plan: AdminBillingPlanDTO) {
    setPlans((current) => sortPlans([...current.filter((item) => item.id !== plan.id), plan]));
    invalidateAdminReferenceDataCache();
  }

  /** Resolves `true` once the plan is persisted so the caller can close its editor. `planID` null creates a plan. */
  async function savePlan(planID: number | null, planForm: PlanFormState): Promise<boolean> {
    setSaving(true);
    try {
      const payload = buildPlanPayload(planForm);
      const data = await withToken((token) => (
        planID === null ? createAdminBillingPlan(token, payload) : updateAdminBillingPlan(token, planID, payload)
      ));
      if (!data) return false;
      upsertPlan(data.plan);
      toast.success(t(planID === null ? "toast.planCreated" : "toast.planSaved"));
      return true;
    } catch (error) {
      toast.error(t("toast.planSaveFailed"), { description: resolveAdminErrorMessage(error) });
      return false;
    } finally {
      setSaving(false);
    }
  }

  async function setPlanActive(plan: AdminBillingPlanDTO, isActive: boolean): Promise<void> {
    setSaving(true);
    try {
      const payload = buildPlanPayload({ ...createPlanFormState(plan), isActive });
      const data = await withToken((token) => updateAdminBillingPlan(token, plan.id, payload));
      if (!data) return;
      upsertPlan(data.plan);
      toast.success(t(isActive ? "toast.planActivated" : "toast.planDeactivated"));
    } catch (error) {
      toast.error(t("toast.planSaveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }

  async function deletePlan(plan: AdminBillingPlanDTO): Promise<boolean> {
    setSaving(true);
    try {
      const done = await withToken((token) => deleteAdminBillingPlan(token, plan.id).then(() => true));
      if (!done) return false;
      setPlans((current) => current.filter((item) => item.id !== plan.id));
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.planDeleted"));
      return true;
    } catch (error) {
      toast.error(t("toast.planDeleteFailed"), { description: resolveAdminErrorMessage(error) });
      return false;
    } finally {
      setSaving(false);
    }
  }

  return { saving, savePlan, setPlanActive, deletePlan };
}
