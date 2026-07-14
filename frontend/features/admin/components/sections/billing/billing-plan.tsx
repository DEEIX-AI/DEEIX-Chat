"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { updateAdminBillingPlan, invalidateAdminReferenceDataCache } from "@/features/admin/api";
import type { AdminBillingPlanDTO } from "@/features/admin/api/billing.types";
import type { PermissionGroup } from "@/features/admin/api/permission-groups";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { createPlanFormState, parseIntValue, parsePrice, type PlanFormState } from "@/features/admin/model/billing-settings";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { PlanBillingDialog } from "@/features/admin/components/sections/billing/billing-dialogs";
import { PeriodBillingTable } from "@/features/admin/components/sections/billing/billing-tables";

type BillingPlanSectionProps = {
  plans: AdminBillingPlanDTO[];
  setPlans: React.Dispatch<React.SetStateAction<AdminBillingPlanDTO[]>>;
  permissionGroups: PermissionGroup[];
  loading: boolean;
};

export function BillingPlanSection({ plans, setPlans, permissionGroups, loading }: BillingPlanSectionProps) {
  const t = useTranslations("adminBilling");
  const [saving, setSaving] = React.useState(false);
  const [editPlan, setEditPlan] = React.useState<AdminBillingPlanDTO | null>(null);
  const [planForm, setPlanForm] = React.useState<PlanFormState | null>(null);
  const stablePlanForm = useDialogSnapshot(planForm);

  function openPlanEdit(plan: AdminBillingPlanDTO) {
    setEditPlan(plan);
    setPlanForm(createPlanFormState(plan, permissionGroups.find((group) => group.isDefault)?.id ?? permissionGroups[0]?.id));
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
        permissionGroupID: Number(planForm.permissionGroupID) || undefined,
      });
      setPlans((current) => current.map((plan) => plan.id === data.plan.id ? data.plan : plan));
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.planSaved"));
      setEditPlan(null);
      setPlanForm(null);
    } catch (error) {
      toast.error(t("toast.planSaveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }

  return (
    <section className="space-y-6 px-1">
      <div className="flex h-10 items-center">
        <h3 className="text-sm font-semibold">{t("plans.title")}</h3>
      </div>
      <PeriodBillingTable plans={plans} loading={loading} onEdit={openPlanEdit} />

      <PlanBillingDialog
        open={!!editPlan && !!planForm}
        saving={saving}
        planForm={stablePlanForm}
        setPlanForm={setPlanForm}
        permissionGroups={permissionGroups}
        onOpenChange={(open) => {
          if (!open && !saving) {
            setEditPlan(null);
            setPlanForm(null);
          }
        }}
        onCancel={() => setEditPlan(null)}
        onSubmit={savePlan}
      />
    </section>
  );
}
