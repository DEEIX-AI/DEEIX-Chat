"use client";

import * as React from "react";
import { Plus } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import type { AdminBillingPlanDTO } from "@/features/admin/api/billing-types";
import type { PermissionGroup } from "@/features/admin/api/permission-groups";
import { useAdminBillingPlans } from "@/features/admin/hooks/use-admin-billing-plans";
import { createPlanFormState, nextPlanSortOrder, type PlanFormState } from "@/features/admin/model/billing-settings";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { AdminBulkConfirmDialog } from "@/features/admin/components/shared/bulk-confirm-dialog";
import { PlanBillingDialog } from "@/features/admin/components/sections/billing/billing-dialogs";
import { PeriodBillingTable } from "@/features/admin/components/sections/billing/billing-tables";

type BillingPlanSectionProps = {
  plans: AdminBillingPlanDTO[];
  setPlans: React.Dispatch<React.SetStateAction<AdminBillingPlanDTO[]>>;
  permissionGroups: PermissionGroup[];
  loading: boolean;
};

type PlanEditorTarget = { mode: "create" } | { mode: "edit"; plan: AdminBillingPlanDTO };

export function BillingPlanSection({ plans, setPlans, permissionGroups, loading }: BillingPlanSectionProps) {
  const t = useTranslations("adminBilling");
  const tActions = useTranslations("common.actions");
  const billingPlans = useAdminBillingPlans({ setPlans });
  const saving = billingPlans.saving;
  const [editorTarget, setEditorTarget] = React.useState<PlanEditorTarget | null>(null);
  const [planForm, setPlanForm] = React.useState<PlanFormState | null>(null);
  const [deleteTarget, setDeleteTarget] = React.useState<AdminBillingPlanDTO | null>(null);
  const stablePlanForm = useDialogSnapshot(planForm);
  const stableEditorTarget = useDialogSnapshot(editorTarget);
  const stableDeleteTarget = useDialogSnapshot(deleteTarget);
  const defaultPermissionGroupID = permissionGroups.find((group) => group.isDefault)?.id ?? permissionGroups[0]?.id;

  function openPlanCreate() {
    setEditorTarget({ mode: "create" });
    setPlanForm(createPlanFormState(null, defaultPermissionGroupID, nextPlanSortOrder(plans)));
  }

  function openPlanEdit(plan: AdminBillingPlanDTO) {
    setEditorTarget({ mode: "edit", plan });
    setPlanForm(createPlanFormState(plan, defaultPermissionGroupID));
  }

  function closeEditor() {
    setEditorTarget(null);
    setPlanForm(null);
  }

  async function savePlan(event?: React.FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    if (!editorTarget || !planForm) return;
    const planID = editorTarget.mode === "edit" ? editorTarget.plan.id : null;
    if (await billingPlans.savePlan(planID, planForm)) {
      closeEditor();
    }
  }

  async function confirmDelete() {
    if (!deleteTarget) return;
    if (await billingPlans.deletePlan(deleteTarget)) {
      setDeleteTarget(null);
    }
  }

  return (
    <section className="space-y-6 px-1">
      <div className="flex h-10 items-center justify-between gap-3">
        <h3 className="text-sm font-semibold">{t("plans.title")}</h3>
        <Button type="button" size="sm" className="h-7 gap-1 px-2 text-xs" onClick={openPlanCreate} disabled={loading || saving}>
          <Plus className="size-3.5 stroke-1" />
          {t("plans.create")}
        </Button>
      </div>
      <PeriodBillingTable
        plans={plans}
        loading={loading}
        disabled={saving}
        onEdit={openPlanEdit}
        onToggleActive={(plan, isActive) => void billingPlans.setPlanActive(plan, isActive)}
        onDelete={setDeleteTarget}
      />

      <PlanBillingDialog
        open={!!editorTarget && !!planForm}
        mode={stableEditorTarget?.mode ?? "edit"}
        saving={saving}
        planForm={stablePlanForm}
        setPlanForm={setPlanForm}
        permissionGroups={permissionGroups}
        onOpenChange={(open) => {
          if (!open && !saving) closeEditor();
        }}
        onCancel={closeEditor}
        onSubmit={savePlan}
      />

      <AdminBulkConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open && !saving) setDeleteTarget(null);
        }}
        pending={saving}
        title={t("plans.deleteTitle", { name: stableDeleteTarget?.name ?? "" })}
        description={t("plans.deleteDescription")}
        confirmLabel={tActions("delete")}
        pendingLabel={t("plans.deleting")}
        onConfirm={() => void confirmDelete()}
      />
    </section>
  );
}
