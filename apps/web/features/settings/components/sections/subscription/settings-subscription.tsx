"use client";

import dynamic from "next/dynamic";
import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { Separator } from "@/components/ui/separator";
import { type PaymentProvider, useSettingsBillingActions } from "@/features/settings/hooks/use-settings-billing-actions";
import { useSettingsBillingOverview } from "@/features/settings/hooks/use-settings-billing-overview";
import { useSettingsBillingUsageLog } from "@/features/settings/hooks/use-settings-billing-usage-log";
import {
  formatAccountBalance,
  isFreePlan,
  planRank,
  resolveDefaultPrice,
  resolvePlanActionKind,
} from "@/features/settings/model/subscription-format";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import type { BillingMode, BillingPlanDTO, BillingPlanPriceDTO } from "@/shared/api/billing-types";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { SettingsPage, SettingsSectionHeader } from "@/shared/components/settings-layout";
import { type BillingDisplayOptions, normalizeBillingDisplayCurrency } from "@/entities/billing";
import { SubscriptionActivityHeatmapSkeleton } from "./subscription-activity-heatmap-skeleton";
import { RedemptionDialog, TopUpDialog } from "./subscription-billing-dialogs";
import { SubscriptionSummary } from "./subscription-summary";
import type { UsageTrendView } from "./subscription-trend";
import { SubscriptionUsageLog } from "./subscription-usage-log";

const SubscriptionTrend = dynamic(
  () => import("./subscription-trend").then((module) => module.SubscriptionTrend),
  {
    ssr: false,
    loading: () => <SubscriptionTrendSkeleton />,
  },
);

const SubscriptionActivityHeatmap = dynamic(
  () => import("./subscription-activity-heatmap").then((module) => module.SubscriptionActivityHeatmap),
  {
    ssr: false,
    loading: () => <SubscriptionActivityHeatmapSkeleton />,
  },
);

function SubscriptionTrendSkeleton() {
  return (
    <div className="space-y-4">
      <div className="flex h-9 items-center justify-between gap-3">
        <div className="h-4 w-28 rounded-full bg-muted/50" />
        <div className="h-7 w-24 rounded-full bg-muted/50" />
      </div>
      <div className="grid grid-cols-2 gap-2 text-xs md:grid-cols-4">
        {Array.from({ length: 4 }).map((_, index) => (
          <div key={`subscription-trend-skeleton-${index}`} className="rounded-md bg-muted/40 p-3">
            <div className="h-3 w-16 rounded-full bg-muted/60" />
            <div className="mt-2 h-4 w-20 rounded-full bg-muted/60" />
          </div>
        ))}
      </div>
      <div className="rounded-md bg-muted/35 p-3">
        <div className="h-[260px] rounded-md bg-muted/30" />
      </div>
    </div>
  );
}

export function SettingsSubscription() {
  const t = useTranslations("settings.subscriptionPage");
  const { locale } = useAppLocale();
  const { accessToken, user } = useAuthSession();
  const {
    viewer,
    billingPlans,
    billingConfig,
    billingOverview,
    setBillingOverview,
    dailyUsage,
    monthlyUsage,
    billingLoading,
  } = useSettingsBillingOverview(accessToken, user);
  const usageLog = useSettingsBillingUsageLog(accessToken);
  const {
    checkoutPriceID,
    topUpLoading,
    redemptionLoading,
    checkout: handleCheckout,
    subscribeFreePlan: handleSubscribeFreePlan,
    topUp,
    redeemCode,
  } = useSettingsBillingActions({ accessToken, onOverviewChange: setBillingOverview });
  const [usageView, setUsageView] = React.useState<UsageTrendView>("daily");
  const [topUpAmount, setTopUpAmount] = React.useState("20");
  const [pricingDialogOpen, setPricingDialogOpen] = React.useState(false);
  const [paymentDialogOpen, setPaymentDialogOpen] = React.useState(false);
  const [selectedPlan, setSelectedPlan] = React.useState<BillingPlanDTO | null>(null);
  const [selectedPrice, setSelectedPrice] = React.useState<BillingPlanPriceDTO | null>(null);
  const [selectedPaymentProvider, setSelectedPaymentProvider] = React.useState<PaymentProvider>("stripe");
  const [selectedEPayType, setSelectedEPayType] = React.useState("alipay");
  const [topUpDialogOpen, setTopUpDialogOpen] = React.useState(false);
  const [redemptionDialogOpen, setRedemptionDialogOpen] = React.useState(false);
  const [redemptionCode, setRedemptionCode] = React.useState("");
  const billingMode: BillingMode = billingConfig?.mode ?? "self";
  const billingDisplay = React.useMemo<BillingDisplayOptions>(
    () => ({
      currency: normalizeBillingDisplayCurrency(billingConfig?.displayCurrency),
      usdToCnyRate: billingConfig?.usdToCNYRate ?? null,
    }),
    [billingConfig?.displayCurrency, billingConfig?.usdToCNYRate],
  );

  const intervalLabels = React.useMemo(
    () => ({
      lifetime: t("interval.lifetime"),
      year: t("interval.year"),
      month: t("interval.month"),
    }),
    [t],
  );
  const planActionLabels = React.useMemo(
    () => ({
      current: t("plans.actions.current"),
      unavailable: t("plans.actions.unavailable"),
      renew: t("plans.actions.renew"),
      subscribe: t("plans.actions.subscribe"),
      switch: t("plans.actions.switch"),
      upgrade: t("plans.actions.upgrade"),
      freeBlocked: t("plans.actions.freeBlocked"),
    }),
    [t],
  );
  const planFeatureLabels = React.useMemo(
    () => ({
      monthlyCredit: (credit: string) => t("plans.features.monthlyCredit", { credit }),
      freeModelsNotIncluded: t("plans.features.freeModelsNotIncluded"),
    }),
    [t],
  );
  const entitlementLabels = React.useMemo(
    () => ({
      title: t("entitlements.title"),
      count: (count: number) => t("entitlements.count", { count }),
      current: t("entitlements.current"),
      upcoming: t("entitlements.upcoming"),
      range: (start: string, end: string) => t("entitlements.range", { start, end }),
      credit: (credit: string) => t("entitlements.credit", { credit }),
    }),
    [t],
  );
  const epayLabels = React.useMemo(
    () => ({
      alipay: t("payment.epay.alipay"),
      wxpay: t("payment.epay.wxpay"),
      qqpay: t("payment.epay.qqpay"),
      custom: (type: string) => t("payment.epay.custom", { type }),
    }),
    [t],
  );

  const epayTypes = React.useMemo(() => {
    const values = billingConfig?.epayTypes?.filter((item) => item.type.trim()) ?? [];
    return values.length > 0 ? values : [{ name: epayLabels.alipay, type: "alipay" }, { name: epayLabels.wxpay, type: "wxpay" }];
  }, [billingConfig?.epayTypes, epayLabels.alipay, epayLabels.wxpay]);
  const paymentProviders = React.useMemo(() => billingConfig?.paymentProviders?.filter((item) => item === "stripe" || item === "epay") ?? [], [billingConfig?.paymentProviders]);

  React.useEffect(() => {
    if (paymentProviders.length > 0 && !paymentProviders.includes(selectedPaymentProvider)) {
      setSelectedPaymentProvider(paymentProviders[0] ?? "stripe");
    }
  }, [paymentProviders, selectedPaymentProvider]);

  React.useEffect(() => {
    if (selectedPaymentProvider !== "epay") return;
    if (!epayTypes.some((item) => item.type === selectedEPayType)) {
      setSelectedEPayType(epayTypes[0]?.type ?? "alipay");
    }
  }, [epayTypes, selectedEPayType, selectedPaymentProvider]);

  const handleTopUp = React.useCallback(
    () => topUp(topUpAmount, selectedPaymentProvider, selectedEPayType),
    [selectedEPayType, selectedPaymentProvider, topUp, topUpAmount],
  );

  const handleRedeemCode = React.useCallback(
    () => redeemCode(redemptionCode, () => {
      setRedemptionDialogOpen(false);
      setRedemptionCode("");
    }),
    [redeemCode, redemptionCode],
  );

  const subscriptionEntitlements = React.useMemo(
    () => billingOverview?.subscriptionEntitlements ?? [],
    [billingOverview?.subscriptionEntitlements],
  );
  const paymentDisabled = paymentProviders.length === 0;
  // Don't render the top-up entry until config loads, avoiding a "disabled → gone" flash; keep it hidden once no channel is confirmed.
  const topUpVisible = billingConfig !== null && paymentProviders.length > 0;
  const currentPlan = React.useMemo(() => {
    if (billingOverview?.plan) return billingOverview.plan;
    return billingPlans.find((plan) => viewer?.subscriptionPlanID === plan.id || viewer?.subscriptionTier === plan.code) ?? null;
  }, [billingOverview?.plan, billingPlans, viewer?.subscriptionPlanID, viewer?.subscriptionTier]);
  const currentPrice = React.useMemo(() => resolveDefaultPrice(currentPlan), [currentPlan]);
  const protectedPaidPlanRank = React.useMemo(
    () => Math.max(
      currentPlan && !isFreePlan(currentPlan) ? planRank(currentPlan) : 0,
      ...subscriptionEntitlements.map((item) => isFreePlan(item.plan) ? 0 : planRank(item.plan)),
    ),
    [currentPlan, subscriptionEntitlements],
  );

  const handleSelectPlan = React.useCallback(
    async (plan: BillingPlanDTO, price: BillingPlanPriceDTO | null, isCurrent: boolean) => {
      if (isCurrent && isFreePlan(plan)) {
        return;
      }
      if (!price) {
        toast.error(t("toasts.planUnavailable"), { description: t("toasts.planUnavailableDescription") });
        return;
      }
      const actionKind = resolvePlanActionKind(
        plan,
        price,
        isCurrent,
        currentPlan,
        protectedPaidPlanRank,
      );
      if (actionKind === "freeBlocked") {
        toast.error(t("toasts.freeSwitchBlocked"), { description: t("toasts.freeSwitchBlockedDescription") });
        return;
      }
      if (price.amountCents > 0) {
        if (paymentDisabled) {
          toast.error(t("toasts.paymentDisabled"), { description: t("toasts.paymentDisabledDescription") });
          return;
        }
        setSelectedPlan(plan);
        setSelectedPrice(price);
        setPricingDialogOpen(false);
        setPaymentDialogOpen(true);
        return;
      }
      await handleSubscribeFreePlan(price);
    },
    [currentPlan, handleSubscribeFreePlan, paymentDisabled, protectedPaidPlanRank, t],
  );

  const handleConfirmPayment = React.useCallback(async () => {
    if (!selectedPrice) {
      toast.error(t("toasts.noPlanSelected"), { description: t("toasts.noPlanSelectedDescription") });
      return;
    }
    await handleCheckout(selectedPrice, selectedPaymentProvider, selectedEPayType);
  }, [handleCheckout, selectedEPayType, selectedPaymentProvider, selectedPrice, t]);

  const periodCredit = billingOverview?.periodCreditUSD ?? currentPlan?.periodCreditUSD ?? 0;
  const periodUsed = billingOverview?.periodUsedUSD ?? 0;
  const periodPercent = periodCredit > 0 ? Math.min(100, Math.max(0, (periodUsed / periodCredit) * 100)) : 0;
  const billingAccount = billingOverview?.account ?? null;

  return (
    <SettingsPage className="space-y-6">
      <SettingsSectionHeader title={t("title")} className="px-1" />

      <SubscriptionSummary
        billingMode={billingMode}
        billingLoading={billingLoading}
        redemptionLoading={redemptionLoading}
        topUpLoading={topUpLoading}
        paymentDisabled={paymentDisabled}
        topUpVisible={topUpVisible}
        billingPlans={billingPlans}
        billingOverview={billingOverview}
        currentPlan={currentPlan}
        currentPrice={currentPrice}
        viewer={viewer}
        billingAccount={billingAccount}
        subscriptionEntitlements={subscriptionEntitlements}
        locale={locale}
        intervalLabels={intervalLabels}
        entitlementLabels={entitlementLabels}
        planActionLabels={planActionLabels}
        planFeatureLabels={planFeatureLabels}
        epayLabels={epayLabels}
        epayTypes={epayTypes}
        paymentProviders={paymentProviders}
        selectedPlan={selectedPlan}
        selectedPrice={selectedPrice}
        selectedPaymentProvider={selectedPaymentProvider}
        selectedEPayType={selectedEPayType}
        checkoutPriceID={checkoutPriceID}
        pricingDialogOpen={pricingDialogOpen}
        paymentDialogOpen={paymentDialogOpen}
        protectedPaidPlanRank={protectedPaidPlanRank}
        periodCredit={periodCredit}
        periodUsed={periodUsed}
        periodPercent={periodPercent}
        billingDisplay={billingDisplay}
        onOpenRedemptionDialog={() => setRedemptionDialogOpen(true)}
        onOpenTopUpDialog={() => setTopUpDialogOpen(true)}
        onPricingDialogOpenChange={setPricingDialogOpen}
        onPaymentDialogOpenChange={setPaymentDialogOpen}
        onSelectPlan={(plan, price, isCurrent) => void handleSelectPlan(plan, price, isCurrent)}
        onPaymentProviderChange={setSelectedPaymentProvider}
        onEPayTypeChange={setSelectedEPayType}
        onConfirmPayment={() => void handleConfirmPayment()}
      />

      <section className="space-y-6 px-0.5 md:space-y-7 xl:space-y-8 xl:px-1">
        <Separator />
        <SubscriptionActivityHeatmap accessToken={accessToken} />
        <Separator />
        <SubscriptionTrend
          dailyUsage={dailyUsage}
          monthlyUsage={monthlyUsage}
          loading={billingLoading}
          view={usageView}
          billingDisplay={billingDisplay}
          onViewChange={setUsageView}
        />
        <Separator />
        <SubscriptionUsageLog
          items={usageLog.items}
          total={usageLog.total}
          loading={usageLog.loading}
          page={usageLog.page}
          pageSize={usageLog.pageSize}
          query={usageLog.query}
          status={usageLog.status}
          sort={usageLog.sort}
          billingDisplay={billingDisplay}
          onQueryChange={usageLog.changeQuery}
          onStatusChange={usageLog.changeStatus}
          onSortChange={usageLog.changeSort}
          onRefresh={usageLog.refresh}
          onPageChange={usageLog.setPage}
          onPageSizeChange={usageLog.changePageSize}
        />
      </section>

      <TopUpDialog
        open={topUpDialogOpen}
        onOpenChange={setTopUpDialogOpen}
        amount={topUpAmount}
        currentBalance={formatAccountBalance(billingAccount?.balanceUSD ?? 0, billingDisplay)}
        billingLoading={billingLoading}
        topUpLoading={topUpLoading}
        paymentDisabled={paymentDisabled}
        paymentProviders={paymentProviders}
        selectedPaymentProvider={selectedPaymentProvider}
        selectedEPayType={selectedEPayType}
        epayTypes={epayTypes}
        billingDisplay={billingDisplay}
        epayLabels={epayLabels}
        onAmountChange={setTopUpAmount}
        onPaymentProviderChange={setSelectedPaymentProvider}
        onEPayTypeChange={setSelectedEPayType}
        onSubmit={() => void handleTopUp()}
      />

      <RedemptionDialog
        open={redemptionDialogOpen}
        onOpenChange={setRedemptionDialogOpen}
        code={redemptionCode}
        billingLoading={billingLoading}
        redemptionLoading={redemptionLoading}
        onCodeChange={setRedemptionCode}
        onSubmit={() => void handleRedeemCode()}
      />
    </SettingsPage>
  );
}
