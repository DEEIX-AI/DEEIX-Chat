"use client";

import * as React from "react";
import { Banknote, Check, Ticket } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Separator } from "@/components/ui/separator";
import { SpinnerLabel } from "@/components/ui/spinner";
import type { UserDTO } from "@/shared/api/auth.types";
import type { BillingOverviewData, BillingSubscriptionEntitlementDTO } from "@/shared/api/billing.types";
import type { BillingPlanDTO, BillingPlanPriceDTO } from "@/shared/api/billing.types";
import {
  formatAccountBalance,
  formatMediumDate,
  formatPlanCredit,
  formatPlanPrice,
  formatShortDate,
  isCurrentBillingPlan,
  isFreePlan,
  planRank,
  resolveDefaultPrice,
  resolveEPayTypeLabel,
  resolvePaymentProviderLabel,
  resolvePlanActionKind,
  resolvePlanActionLabel,
  resolvePlanButtonVariant,
  resolvePlanFeatures,
} from "@/features/settings/model/subscription-format";

type BillingMode = "period" | "usage" | "self";
type PaymentProvider = "stripe" | "epay";
type BillingAccount = NonNullable<BillingOverviewData["overview"]>["account"];

type SubscriptionIntervalLabels = {
  lifetime: string;
  year: string;
  month: string;
};

type SubscriptionEntitlementQueueLabels = {
  title: string;
  count: (count: number) => string;
  current: string;
  upcoming: string;
  range: (start: string, end: string) => string;
  credit: (credit: string) => string;
};

type PlanFeatureLabels = {
  monthlyCredit: (credit: string) => string;
  freeModelsNotIncluded: string;
};

type PaymentLabels = {
  alipay: string;
  wxpay: string;
  qqpay: string;
  custom: (type: string) => string;
};

type PlanActionLabels = {
  current: string;
  unavailable: string;
  renew: string;
  subscribe: string;
  switch: string;
  upgrade: string;
  freeBlocked: string;
};

type EPayTypeOption = {
  name: string;
  type: string;
};

function ActionRow({
  title,
  value,
  action,
}: {
  title: string;
  value?: string;
  action: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
      <div className="flex min-w-0 items-baseline gap-2">
        <p className="shrink-0 text-xs font-medium">{title}</p>
        {value ? <p className="max-w-[min(60vw,24rem)] truncate text-xs text-muted-foreground">{value}</p> : null}
      </div>
      <div className="self-start sm:self-auto sm:justify-self-end">{action}</div>
    </div>
  );
}

function ValueRow({
  title,
  value,
  action,
}: {
  title: string;
  value: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <p className="text-xs font-medium">{title}</p>
      <div className="flex min-w-0 max-w-full items-center gap-2 self-start rounded-lg bg-muted/35 px-2 py-1 text-xs text-muted-foreground sm:self-auto">
        <span className="max-w-[min(75vw,26rem)] truncate">{value}</span>
        {action}
      </div>
    </div>
  );
}

function entitlementTimeMS(value: string | null | undefined): number | null {
  if (!value) return null;
  const time = new Date(value).getTime();
  return Number.isFinite(time) ? time : null;
}

function SubscriptionEntitlementQueue({
  items,
  labels,
  locale,
}: {
  items: BillingSubscriptionEntitlementDTO[];
  labels: SubscriptionEntitlementQueueLabels;
  locale: string;
}) {
  if (items.length === 0) {
    return null;
  }
  const orderedItems = [...items].sort((left, right) => {
    const leftStart = entitlementTimeMS(left.currentPeriodStartAt || left.startAt) ?? 0;
    const rightStart = entitlementTimeMS(right.currentPeriodStartAt || right.startAt) ?? 0;
    if (leftStart !== rightStart) return leftStart - rightStart;
    return left.id - right.id;
  });

  return (
    <div className="px-1 text-xs text-muted-foreground">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
        <span className="font-medium text-foreground">{labels.title}</span>
        <span>{labels.count(items.length)}</span>
        <span className="text-muted-foreground/50">/</span>
        {orderedItems.map((item, index) => {
          const start = formatMediumDate(item.currentPeriodStartAt || item.startAt, locale);
          const end = item.currentPeriodEndAt ? formatMediumDate(item.currentPeriodEndAt, locale) : "-";
          return (
            <React.Fragment key={`${item.id}-${item.currentPeriodStartAt}`}>
              {index > 0 ? <span className="text-muted-foreground/50">/</span> : null}
              <span
                className={item.isCurrent ? "font-medium text-foreground" : undefined}
                title={`${labels.range(start, end)} · ${labels.credit(formatPlanCredit(item.plan.periodCreditUSD))}`}
              >
                {item.plan.name || item.plan.code}
              </span>
              <span className="tabular-nums">{labels.range(start, end)}</span>
            </React.Fragment>
          );
        })}
      </div>
    </div>
  );
}

type SubscriptionSummaryProps = {
  billingMode: BillingMode;
  billingLoading: boolean;
  redemptionLoading: boolean;
  topUpLoading: boolean;
  paymentDisabled: boolean;
  billingPlans: BillingPlanDTO[];
  billingOverview: BillingOverviewData["overview"] | null;
  currentPlan: BillingPlanDTO | null;
  currentPrice: BillingPlanPriceDTO | null;
  viewer: UserDTO | null;
  billingAccount: BillingAccount | null;
  subscriptionEntitlements: BillingSubscriptionEntitlementDTO[];
  locale: string;
  intervalLabels: SubscriptionIntervalLabels;
  entitlementLabels: SubscriptionEntitlementQueueLabels;
  planActionLabels: PlanActionLabels;
  planFeatureLabels: PlanFeatureLabels;
  epayLabels: PaymentLabels;
  epayTypes: EPayTypeOption[];
  paymentProviders: string[];
  selectedPlan: BillingPlanDTO | null;
  selectedPrice: BillingPlanPriceDTO | null;
  selectedPaymentProvider: PaymentProvider;
  selectedEPayType: string;
  checkoutPriceID: number | null;
  pricingDialogOpen: boolean;
  paymentDialogOpen: boolean;
  protectedPaidPlanRank: number;
  periodCredit: number;
  periodUsed: number;
  periodPercent: number;
  onOpenRedemptionDialog: () => void;
  onOpenTopUpDialog: () => void;
  onPricingDialogOpenChange: (open: boolean) => void;
  onPaymentDialogOpenChange: (open: boolean) => void;
  onSelectPlan: (plan: BillingPlanDTO, price: BillingPlanPriceDTO | null, isCurrent: boolean) => void;
  onPaymentProviderChange: (provider: PaymentProvider) => void;
  onEPayTypeChange: (type: string) => void;
  onConfirmPayment: () => void;
};

export function SubscriptionSummary({
  billingMode,
  billingLoading,
  redemptionLoading,
  topUpLoading,
  paymentDisabled,
  billingPlans,
  billingOverview,
  currentPlan,
  currentPrice,
  viewer,
  billingAccount,
  subscriptionEntitlements,
  locale,
  intervalLabels,
  entitlementLabels,
  planActionLabels,
  planFeatureLabels,
  epayLabels,
  epayTypes,
  paymentProviders,
  selectedPlan,
  selectedPrice,
  selectedPaymentProvider,
  selectedEPayType,
  checkoutPriceID,
  pricingDialogOpen,
  paymentDialogOpen,
  protectedPaidPlanRank,
  periodCredit,
  periodUsed,
  periodPercent,
  onOpenRedemptionDialog,
  onOpenTopUpDialog,
  onPricingDialogOpenChange,
  onPaymentDialogOpenChange,
  onSelectPlan,
  onPaymentProviderChange,
  onEPayTypeChange,
  onConfirmPayment,
}: SubscriptionSummaryProps) {
  const t = useTranslations("settings.subscriptionPage");
  const selectedPlanActionKind = selectedPlan
    ? resolvePlanActionKind(
      selectedPlan,
      selectedPrice,
      isCurrentBillingPlan(selectedPlan, currentPlan, viewer),
      currentPlan,
      protectedPaidPlanRank,
    )
    : "subscribe";
  const selectedRenewStartsAfterHigher = Boolean(
    selectedPlan
      && selectedPlanActionKind === "renew"
      && protectedPaidPlanRank > planRank(selectedPlan),
  );
  const paymentTitle = selectedPlanActionKind === "renew"
    ? t("payment.renewTitle")
    : selectedPlanActionKind === "upgrade"
      ? t("payment.upgradeTitle")
      : t("payment.title");
  const paymentImpactDescription = selectedPlanActionKind === "renew"
    ? selectedRenewStartsAfterHigher
      ? t("payment.renewAfterHigherDescription")
      : t("payment.renewDescription")
    : selectedPlanActionKind === "upgrade"
      ? t("payment.upgradeDescription")
      : null;

  return (
    <>
      {billingMode === "period" ? (
        <section className="space-y-6 px-0.5 md:space-y-7 xl:space-y-8 xl:px-1">
          <div className="space-y-4 md:space-y-5">
            <div className="flex items-start justify-between gap-3 md:gap-4">
              <div className="min-w-0 space-y-1">
                <p className="text-xs font-medium">{t("currentSubscription.title")}</p>
                <p className="truncate text-sm font-semibold">{currentPlan?.name ?? t("currentSubscription.none")}</p>
                <p className="text-xs text-muted-foreground">
                  {currentPlan ? `${formatPlanPrice(currentPrice, intervalLabels)} · ${t("plans.features.monthlyCredit", { credit: formatPlanCredit(periodCredit) })}` : t("currentSubscription.empty")}
                </p>
              </div>
              <div className="flex shrink-0 items-center gap-2">
                <Button type="button" variant="outline" disabled={billingLoading || redemptionLoading} onClick={onOpenRedemptionDialog}>
                  <Ticket className="size-3.5" />
                  {t("redemption.open")}
                </Button>
                <Button type="button" variant="outline" disabled={billingLoading || billingPlans.length === 0} onClick={() => onPricingDialogOpenChange(true)}>
                  <Banknote className="size-3.5" />
                  {t("currentSubscription.subscribe")}
                </Button>
              </div>
            </div>
          </div>

          <SubscriptionEntitlementQueue
            items={subscriptionEntitlements}
            labels={entitlementLabels}
            locale={locale}
          />

          <Separator />

          <div className="space-y-3 rounded-md bg-muted/35 p-3 md:space-y-4">
            <div className="flex items-start justify-between gap-3 md:gap-4">
              <div className="space-y-1">
                <p className="text-xs font-medium">{t("periodUsage.title")}</p>
                <p className="text-xs text-muted-foreground">
                  {billingOverview?.periodStartAt && billingOverview?.periodEndAt
                    ? `${formatShortDate(billingOverview.periodStartAt, locale)} - ${formatShortDate(billingOverview.periodEndAt, locale)}`
                    : t("periodUsage.currentPeriod")}
                </p>
              </div>
              <p className="shrink-0 text-xs font-medium text-muted-foreground">{Math.round(periodPercent)}%</p>
            </div>
            <div className="space-y-2">
              <div className="flex items-center justify-between gap-4 text-xs">
                <span className="text-muted-foreground">{t("periodUsage.used", { value: formatPlanCredit(periodUsed) })}</span>
                <span className="text-muted-foreground">{t("periodUsage.total", { value: formatPlanCredit(periodCredit) })}</span>
              </div>
              <div className="h-2 overflow-hidden rounded-full bg-muted">
                <div className="h-full rounded-full bg-foreground/70" style={{ width: `${periodPercent}%` }} />
              </div>
            </div>
          </div>

          <ActionRow
            title={t("periodOverage.title")}
            value={t("periodOverage.balance", { value: formatAccountBalance(billingAccount?.balanceUSD ?? 0) })}
            action={
              <Button type="button" variant="outline" disabled={billingLoading || topUpLoading || paymentDisabled} onClick={onOpenTopUpDialog}>
                <Banknote className="size-3.5" />
                {t("usageBilling.topUp")}
              </Button>
            }
          />
        </section>
      ) : null}

      {billingMode === "usage" ? (
        <section className="space-y-6 px-0.5 md:space-y-7 xl:space-y-8 xl:px-1">
          <ActionRow
            title={t("usageBilling.title")}
            value={t("usageBilling.balance", { value: formatAccountBalance(billingAccount?.balanceUSD ?? 0) })}
            action={
              <div className="flex items-center gap-2">
                <Button type="button" variant="outline" disabled={billingLoading || redemptionLoading} onClick={onOpenRedemptionDialog}>
                  <Ticket className="size-3.5" />
                  {t("redemption.open")}
                </Button>
                <Button type="button" variant="outline" disabled={billingLoading || topUpLoading || paymentDisabled} onClick={onOpenTopUpDialog}>
                  <Banknote className="size-3.5" />
                  {t("usageBilling.topUp")}
                </Button>
              </div>
            }
          />
        </section>
      ) : null}

      {billingMode === "self" ? (
        <section className="space-y-6 px-0.5 md:space-y-7 xl:space-y-8 xl:px-1">
          <ValueRow title={t("selfMode.title")} value={t("selfMode.value")} />
        </section>
      ) : null}

      <Dialog open={pricingDialogOpen} onOpenChange={onPricingDialogOpenChange}>
        <DialogContent className="xl:max-w-[1040px] xl:p-6">
          <DialogHeader>
            <DialogTitle>{t("plans.title")}</DialogTitle>
          </DialogHeader>

          <div className="space-y-2 xl:hidden">
            {billingPlans.map((plan) => {
              const price = resolveDefaultPrice(plan);
              const isCurrent = isCurrentBillingPlan(plan, currentPlan, viewer);
              const actionKind = resolvePlanActionKind(plan, price, isCurrent, currentPlan, protectedPaidPlanRank);
              const actionLabel = resolvePlanActionLabel(actionKind, planActionLabels);
              const disabled = billingLoading || actionKind === "current" || actionKind === "freeBlocked" || actionKind === "unavailable" || checkoutPriceID === price?.id;
              const isSelected = selectedPlan?.id === plan.id;
              const isHighlighted = isCurrent || isSelected;
              const buttonVariant = resolvePlanButtonVariant(actionKind);
              const actionButton = (
                <Button
                  type="button"
                  size="sm"
                  className="h-8 shrink-0 px-3 shadow-none"
                  variant={buttonVariant}
                  disabled={disabled}
                  onClick={() => onSelectPlan(plan, price, isCurrent)}
                >
                  {checkoutPriceID === price?.id ? <SpinnerLabel>{t("actions.processing")}</SpinnerLabel> : actionLabel}
                </Button>
              );
              return (
                <div
                  key={plan.id}
                  className={[
                    "flex items-center justify-between gap-3 rounded-md bg-muted/30 px-3 py-3 transition-colors",
                    isHighlighted ? "ring-1 ring-foreground" : "hover:bg-muted/45",
                  ].join(" ")}
                >
                  <div className="min-w-0 space-y-1">
                    <div className="flex min-w-0 items-center gap-2">
                      <p className="truncate text-sm font-medium">{plan.name}</p>
                    </div>
                    <p className="text-xs text-muted-foreground">{formatPlanPrice(price, intervalLabels)}</p>
                  </div>
                  {actionButton}
                </div>
              );
            })}
          </div>

          <div className="hidden gap-4 pt-4 xl:grid xl:grid-cols-4">
            {billingPlans.map((plan) => {
              const price = resolveDefaultPrice(plan);
              const isCurrent = isCurrentBillingPlan(plan, currentPlan, viewer);
              const actionKind = resolvePlanActionKind(plan, price, isCurrent, currentPlan, protectedPaidPlanRank);
              const actionLabel = resolvePlanActionLabel(actionKind, planActionLabels);
              const disabled = billingLoading || actionKind === "current" || actionKind === "freeBlocked" || actionKind === "unavailable" || checkoutPriceID === price?.id;
              const features = resolvePlanFeatures(plan, planFeatureLabels).slice(0, 6);
              const isSelected = selectedPlan?.id === plan.id;
              const isHighlighted = isCurrent || isSelected;
              const buttonVariant = resolvePlanButtonVariant(actionKind);
              const actionButton = (
                <Button
                  type="button"
                  className="mt-6 w-full shadow-none"
                  variant={buttonVariant}
                  disabled={disabled}
                  onClick={() => onSelectPlan(plan, price, isCurrent)}
                >
                  {checkoutPriceID === price?.id ? <SpinnerLabel>{t("actions.processing")}</SpinnerLabel> : actionLabel}
                </Button>
              );
              return (
                <div
                  key={plan.id}
                  className={[
                    "flex min-h-[26rem] flex-col rounded-lg border border-transparent bg-muted/30 p-5 transition-colors",
                    isHighlighted ? "ring-2 ring-foreground" : "hover:bg-muted/45",
                  ].join(" ")}
                >
                  <div className="space-y-3">
                    <div className="flex items-start justify-between gap-3">
                      <h3 className="truncate text-lg font-semibold">{plan.name}</h3>
                    </div>
                    <div className="space-y-1">
                      <p className="text-2xl font-semibold">{formatPlanPrice(price, intervalLabels)}</p>
                    </div>
                  </div>

                  {actionButton}

                  <div className="mt-6 hidden space-y-3 sm:block">
                    {features.map((feature) => (
                      <div key={feature} className="flex items-start gap-3 text-xs text-muted-foreground">
                        <Check className="mt-0.5 size-3.5 shrink-0 text-foreground" />
                        <span className="leading-5">{feature}</span>
                      </div>
                    ))}
                  </div>
                </div>
              );
            })}
          </div>
        </DialogContent>
      </Dialog>

      <Dialog open={paymentDialogOpen} onOpenChange={onPaymentDialogOpenChange}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader>
            <DialogTitle>{paymentTitle}</DialogTitle>
            <DialogDescription>
              <span className="block">
                {selectedPlan && selectedPrice
                  ? `${selectedPlan.name} · ${formatPlanPrice(selectedPrice, intervalLabels)}`
                  : t("payment.description")}
              </span>
              {paymentImpactDescription ? <span className="mt-1 block">{paymentImpactDescription}</span> : null}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            {paymentProviders.includes("stripe") ? (
              <button
                type="button"
                className={`flex w-full items-center justify-between rounded-lg border px-3 py-3 text-left ${
                  selectedPaymentProvider === "stripe" ? "border-foreground bg-muted/25" : "border-border bg-transparent"
                }`}
                disabled={paymentDisabled}
                onClick={() => onPaymentProviderChange("stripe")}
              >
                <span className="space-y-1">
                  <span className="block text-xs font-medium">Stripe</span>
                  <span className="block text-xs text-muted-foreground">{t("payment.card")}</span>
                </span>
                {selectedPaymentProvider === "stripe" ? <Check className="size-4" /> : null}
              </button>
            ) : null}
            {paymentProviders.includes("epay")
              ? epayTypes.map((item) => {
                const selected = selectedPaymentProvider === "epay" && selectedEPayType === item.type;
                return (
                  <button
                    key={item.type}
                    type="button"
                    className={`flex w-full items-center justify-between rounded-lg border px-3 py-3 text-left ${
                      selected ? "border-foreground bg-muted/25" : "border-border bg-transparent"
                    }`}
                    disabled={paymentDisabled}
                    onClick={() => {
                      onPaymentProviderChange("epay");
                      onEPayTypeChange(item.type);
                    }}
                  >
                    <span className="space-y-1">
                      <span className="block text-xs font-medium">{item.name || resolveEPayTypeLabel(item.type, epayLabels)}</span>
                      <span className="block text-xs text-muted-foreground">{resolvePaymentProviderLabel("epay", t("payment.disabled"))}</span>
                    </span>
                    {selected ? <Check className="size-4" /> : null}
                  </button>
                );
              })
              : null}
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => onPaymentDialogOpenChange(false)} disabled={checkoutPriceID === selectedPrice?.id}>
              {t("actions.cancel")}
            </Button>
            <Button type="button" disabled={paymentDisabled || !selectedPrice || checkoutPriceID === selectedPrice.id} onClick={onConfirmPayment}>
              {checkoutPriceID === selectedPrice?.id ? <SpinnerLabel>{t("actions.processing")}</SpinnerLabel> : t("payment.continue")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
