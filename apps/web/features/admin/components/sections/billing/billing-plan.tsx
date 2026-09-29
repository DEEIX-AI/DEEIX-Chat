"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import type { AdminBillingPlanDTO } from "@/features/admin/api/billing-types";
import type { PermissionGroup } from "@/features/admin/api/permission-groups";
import { useAdminBillingPlans } from "@/features/admin/hooks/use-admin-billing-plans";
import { createPlanFormState, type PlanFormState } from "@/features/admin/model/billing-settings";
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
  const billingPlans = useAdminBillingPlans({ setPlans });
  const saving = billingPlans.saving;
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
    if (await billingPlans.savePlan(editPlan.id, planForm)) {
      setEditPlan(null);
      setPlanForm(null);
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
