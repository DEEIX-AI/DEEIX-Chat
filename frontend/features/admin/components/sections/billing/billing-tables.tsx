"use client";

import { Pencil } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableEmptyRow,
  TableHead,
  TableHeader,
  TableLoadingRow,
  TableRow,
} from "@/components/ui/table";
import type { AdminBillingPlanDTO, AdminModelPricingDTO } from "@/features/admin/api/billing.types";
import {
  formatAmountCents,
  formatCreditUSD,
  formatUSD,
  normalizePricingMode,
} from "@/features/admin/model/billing-settings";

export function PeriodBillingTable({
  plans,
  loading,
  onEdit,
}: {
  plans: AdminBillingPlanDTO[];
  loading: boolean;
  onEdit: (plan: AdminBillingPlanDTO) => void;
}) {
  const t = useTranslations("adminBilling");
  const initialLoading = loading && plans.length === 0;
  const showPlans = plans.length > 0;

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t("plans.tablePlan")}</TableHead>
          <TableHead>{t("plans.tableDescription")}</TableHead>
          <TableHead>{t("plans.tablePrice")}</TableHead>
          <TableHead>{t("plans.tableCredit")}</TableHead>
          <TableHead>{t("plans.tableDiscount")}</TableHead>
          <TableHead stickyEnd className="w-[56px]" />
        </TableRow>
      </TableHeader>
      <TableBody>
        {initialLoading ? <TableLoadingRow colSpan={6} /> : null}
        {!loading && plans.length === 0 ? <TableEmptyRow colSpan={6}>{t("plans.empty")}</TableEmptyRow> : null}
        {showPlans
          ? plans.map((plan) => {
              const defaultPrice = plan.prices.find((item) => item.isDefault) || plan.prices[0];
              return (
                <TableRow key={plan.id}>
                  <TableCell className="py-1.5">
                    <span className="font-medium text-foreground">{plan.name}</span>
                  </TableCell>
                  <TableCell className="max-w-[280px] py-1.5 text-muted-foreground">
                    <span className="block truncate" title={plan.description || "-"}>
                      {plan.description || "-"}
                    </span>
                  </TableCell>
                  <TableCell className="py-1.5">
                    {defaultPrice ? (
                      <span>
                        {formatAmountCents(defaultPrice.amountCents, defaultPrice.currency)} / {t(`plans.intervals.${defaultPrice.billingInterval}`)}
                      </span>
                    ) : (
                      <span className="text-muted-foreground">-</span>
                    )}
                  </TableCell>
                  <TableCell className="py-1.5">
                    <span>
                      {formatCreditUSD(plan.periodCreditUSD)}
                      <span className="ml-1 text-xs text-muted-foreground">{t("plans.perPeriod")}</span>
                    </span>
                  </TableCell>
                  <TableCell className="py-1.5">{plan.discountPercent}%</TableCell>
                  <TableCell stickyEnd className="w-[56px] py-1.5 text-right">
                    <div className="flex h-7 items-center justify-end">
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon-xs"
                        className="h-7 w-7 text-muted-foreground shadow-none"
                        onClick={() => onEdit(plan)}
                        aria-label={t("actions.editPlan")}
                      >
                        <Pencil className="size-3.5 stroke-1" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              );
            })
          : null}
      </TableBody>
    </Table>
  );
}

function PricingCell({ value, suffix }: { value: number; suffix: string }) {
  return (
    <span className="text-xs tabular-nums text-foreground">
      {formatUSD(value)}
      <span className="ml-1 text-xs text-muted-foreground">{suffix}</span>
    </span>
  );
}

export function PricingUnitCell({ pricing }: { pricing: AdminModelPricingDTO | null }) {
  const t = useTranslations("adminBilling");
  if (!pricing) return <span className="text-muted-foreground">-</span>;
  if (pricing.isFree) return <span className="text-muted-foreground">{t("modelPricing.freeLabel")}</span>;
  const mode = normalizePricingMode(pricing.pricingMode);
  if (mode === "call") return <PricingCell value={pricing.callUSDPerCall} suffix={t("modelPricing.units.call")} />;
  if (mode === "duration") return <PricingCell value={pricing.durationUSDPerSecond} suffix={t("modelPricing.units.second")} />;
  if (mode === "tiered") return <span className="text-xs text-foreground">{t("modelPricing.tieredLabel")}</span>;
  return (
    <div className="flex min-w-0 flex-wrap items-center gap-x-2.5 gap-y-0.5">
      <PricingCell value={pricing.inputUSDPerMTokens} suffix={t("modelPricing.units.input")} />
      <PricingCell value={pricing.outputUSDPerMTokens} suffix={t("modelPricing.units.output")} />
      <PricingCell value={pricing.cacheReadUSDPerMTokens} suffix={t("modelPricing.units.cacheRead")} />
      <PricingCell value={pricing.cacheWriteUSDPerMTokens} suffix={t("modelPricing.units.cacheWrite")} />
    </div>
  );
}
