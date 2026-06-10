"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Separator } from "@/components/ui/separator";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import {
  createBillingCheckout,
  getBillingConfig,
  getBillingOverview,
  listBillingDailyUsage,
  listBillingMonthlyUsage,
  listBillingPlans,
  listBillingUsage,
  redeemBillingCode,
  subscribeBillingPlan,
} from "@/shared/api/billing";
import type {
  BillingConfigData,
  BillingMode,
  BillingOverviewData,
  BillingUsageDailyDTO,
  BillingUsageLedgerDTO,
  BillingUsageMonthlyDTO,
} from "@/shared/api/billing.types";
import type { BillingPlanDTO, BillingPlanPriceDTO } from "@/shared/api/billing.types";
import type { UserDTO } from "@/shared/api/auth.types";
import { SettingsPage, SettingsSectionHeader } from "@/shared/components/settings-layout";
import {
  formatAccountBalance,
  isFreePlan,
  planRank,
  resolveDefaultPrice,
  resolvePlanActionKind,
} from "./subscription-format";
import { RedemptionDialog, TopUpDialog } from "./subscription-billing-dialogs";
import { SubscriptionSummary } from "./subscription-summary";
import { SubscriptionTrend } from "./subscription-trend";
import type { UsageTrendView } from "./subscription-trend";
import { SubscriptionUsageLog } from "./subscription-usage-log";

type BillingRuntimeConfig = BillingConfigData["config"];
type PaymentProvider = "stripe" | "epay";

export function SettingsSubscription() {
  const t = useTranslations("settings.subscriptionPage");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const { locale } = useAppLocale();
  const { accessToken, user } = useAuthSession();
  const [viewer, setViewer] = React.useState<UserDTO | null>(null);
  const [billingPlans, setBillingPlans] = React.useState<BillingPlanDTO[]>([]);
  const [billingConfig, setBillingConfig] = React.useState<BillingRuntimeConfig | null>(null);
  const [billingOverview, setBillingOverview] = React.useState<BillingOverviewData["overview"] | null>(null);
  const [usageLedgers, setUsageLedgers] = React.useState<BillingUsageLedgerDTO[]>([]);
  const [dailyUsage, setDailyUsage] = React.useState<BillingUsageDailyDTO[]>([]);
  const [monthlyUsage, setMonthlyUsage] = React.useState<BillingUsageMonthlyDTO[]>([]);
  const [usageTotal, setUsageTotal] = React.useState(0);
  const [usagePage, setUsagePage] = React.useState(1);
  const [usagePageSize, setUsagePageSize] = React.useState(25);
  const [usageQuery, setUsageQuery] = React.useState("");
  const [usageStatus, setUsageStatus] = React.useState("");
  const [usageSort, setUsageSort] = React.useState("newest");
  const [usageView, setUsageView] = React.useState<UsageTrendView>("daily");
  const [billingLoading, setBillingLoading] = React.useState(true);
  const [usageLoading, setUsageLoading] = React.useState(true);
  const [checkoutPriceID, setCheckoutPriceID] = React.useState<number | null>(null);
  const [topUpAmount, setTopUpAmount] = React.useState("20");
  const [topUpLoading, setTopUpLoading] = React.useState(false);
  const [pricingDialogOpen, setPricingDialogOpen] = React.useState(false);
  const [paymentDialogOpen, setPaymentDialogOpen] = React.useState(false);
  const [selectedPlan, setSelectedPlan] = React.useState<BillingPlanDTO | null>(null);
  const [selectedPrice, setSelectedPrice] = React.useState<BillingPlanPriceDTO | null>(null);
  const [selectedPaymentProvider, setSelectedPaymentProvider] = React.useState<PaymentProvider>("stripe");
  const [selectedEPayType, setSelectedEPayType] = React.useState("alipay");
  const [topUpDialogOpen, setTopUpDialogOpen] = React.useState(false);
  const [redemptionDialogOpen, setRedemptionDialogOpen] = React.useState(false);
  const [redemptionCode, setRedemptionCode] = React.useState("");
  const [redemptionLoading, setRedemptionLoading] = React.useState(false);
  const billingMode: BillingMode = billingConfig?.mode ?? "self";

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

  React.useEffect(() => {
    let mounted = true;
    setBillingLoading(true);
    void Promise.all([
      getBillingConfig(accessToken),
      listBillingPlans(accessToken),
      getBillingOverview(accessToken),
      listBillingDailyUsage(accessToken),
      listBillingMonthlyUsage(accessToken, 12),
    ])
      .then(([configData, plans, overviewData, dailyUsageData, monthlyUsageData]) => ({
        viewer: user,
        config: configData.config,
        plans,
        overview: overviewData.overview,
        dailyUsage: dailyUsageData,
        monthlyUsage: monthlyUsageData,
      }))
      .then(({ viewer: nextViewer, config, plans, overview, dailyUsage: nextDailyUsage, monthlyUsage: nextMonthlyUsage }) => {
        if (!mounted) return;
        setViewer(nextViewer);
        setBillingConfig(config);
        setBillingPlans(plans);
        setBillingOverview(overview);
        setDailyUsage(nextDailyUsage ?? []);
        setMonthlyUsage(nextMonthlyUsage ?? []);
      })
      .catch((error) => {
        if (mounted) toast.error(t("toasts.subscriptionLoadFailed"), { description: resolveErrorMessage(error, t("toasts.retryLater")) });
      })
      .finally(() => {
        if (mounted) setBillingLoading(false);
      });
    return () => {
      mounted = false;
    };
  }, [accessToken, resolveErrorMessage, t, user]);

  const loadUsageLogs = React.useCallback(async (page: number, pageSize: number, query: string, status: string, sort: string) => {
    setUsageLoading(true);
    try {
      const usage = await listBillingUsage(accessToken, { page, pageSize, query, status, sort });
      setUsageLedgers(usage.results ?? []);
      setUsageTotal(usage.total ?? 0);
    } catch (error) {
      toast.error(t("toasts.usageLogLoadFailed"), { description: resolveErrorMessage(error, t("toasts.retryLater")) });
    } finally {
      setUsageLoading(false);
    }
  }, [accessToken, resolveErrorMessage, t]);

  React.useEffect(() => {
    void loadUsageLogs(usagePage, usagePageSize, usageQuery, usageStatus, usageSort);
  }, [loadUsageLogs, usagePage, usagePageSize, usageQuery, usageStatus, usageSort]);

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

  const handleCheckout = React.useCallback(async (price: BillingPlanPriceDTO, paymentProvider: PaymentProvider, epayType?: string) => {
    setCheckoutPriceID(price.id);
    try {
      const data = await createBillingCheckout(accessToken, {
        orderType: "subscription",
        priceID: price.id,
        cycles: 1,
        paymentProvider,
        epayType: paymentProvider === "epay" ? epayType : undefined,
        successURL: `${window.location.origin}/setting/subscription?payment=success`,
        cancelURL: `${window.location.origin}/setting/subscription?payment=cancel`,
      });
      if (!data.checkout.checkoutURL) {
        toast.error(t("toasts.checkoutCreateFailed"), { description: t("toasts.checkoutURLMissing") });
        return;
      }
      window.open(data.checkout.checkoutURL, "_blank", "noopener,noreferrer");
    } catch (error) {
      toast.error(t("toasts.checkoutCreateFailed"), { description: resolveErrorMessage(error, t("toasts.retryLater")) });
    } finally {
      setCheckoutPriceID(null);
    }
  }, [accessToken, resolveErrorMessage, t]);

  const handleSubscribeFreePlan = React.useCallback(async (price: BillingPlanPriceDTO) => {
    setCheckoutPriceID(price.id);
    try {
      await subscribeBillingPlan(accessToken, price.id);
      toast.success(t("toasts.planUpdated"));
      window.location.reload();
    } catch (error) {
      toast.error(t("toasts.subscribeFailed"), { description: resolveErrorMessage(error, t("toasts.retryLater")) });
    } finally {
      setCheckoutPriceID(null);
    }
  }, [accessToken, resolveErrorMessage, t]);

  const handleTopUp = React.useCallback(async () => {
    const amount = Number(topUpAmount);
    if (!Number.isFinite(amount) || amount <= 0) {
      toast.error(t("toasts.invalidTopUpAmount"), { description: t("toasts.invalidTopUpAmountDescription") });
      return;
    }
    setTopUpLoading(true);
    try {
      const data = await createBillingCheckout(accessToken, {
        orderType: "topup",
        amountUSD: amount,
        cycles: 1,
        paymentProvider: selectedPaymentProvider,
        epayType: selectedPaymentProvider === "epay" ? selectedEPayType : undefined,
        successURL: `${window.location.origin}/setting/subscription?payment=success`,
        cancelURL: `${window.location.origin}/setting/subscription?payment=cancel`,
      });
      if (!data.checkout.checkoutURL) {
        toast.error(t("toasts.checkoutCreateFailed"), { description: t("toasts.checkoutURLMissing") });
        return;
      }
      window.open(data.checkout.checkoutURL, "_blank", "noopener,noreferrer");
    } catch (error) {
      toast.error(t("toasts.checkoutCreateFailed"), { description: resolveErrorMessage(error, t("toasts.retryLater")) });
    } finally {
      setTopUpLoading(false);
    }
  }, [accessToken, resolveErrorMessage, selectedEPayType, selectedPaymentProvider, t, topUpAmount]);

  const handleRedeemCode = React.useCallback(async () => {
    const code = redemptionCode.trim();
    if (!code) {
      toast.error(t("toasts.invalidRedemptionCode"));
      return;
    }
    setRedemptionLoading(true);
    try {
      const data = await redeemBillingCode(accessToken, { code });
      setBillingOverview(data.overview);
      setRedemptionDialogOpen(false);
      setRedemptionCode("");
      toast.success(t("toasts.redemptionSucceeded"));
    } catch (error) {
      toast.error(t("toasts.redemptionFailed"), { description: resolveErrorMessage(error, t("toasts.retryLater")) });
    } finally {
      setRedemptionLoading(false);
    }
  }, [accessToken, redemptionCode, resolveErrorMessage, t]);

  const subscriptionEntitlements = React.useMemo(
    () => billingOverview?.subscriptionEntitlements ?? [],
    [billingOverview?.subscriptionEntitlements],
  );
  const paymentDisabled = paymentProviders.length === 0;
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
        <SubscriptionTrend
          dailyUsage={dailyUsage}
          monthlyUsage={monthlyUsage}
          loading={billingLoading}
          view={usageView}
          onViewChange={setUsageView}
        />
        <Separator />
        <SubscriptionUsageLog
          items={usageLedgers}
          total={usageTotal}
          loading={usageLoading}
          page={usagePage}
          pageSize={usagePageSize}
          query={usageQuery}
          status={usageStatus}
          sort={usageSort}
          onQueryChange={(value) => {
            setUsageQuery(value);
            setUsagePage(1);
          }}
          onStatusChange={(value) => {
            setUsageStatus(value);
            setUsagePage(1);
          }}
          onSortChange={(value) => {
            setUsageSort(value);
            setUsagePage(1);
          }}
          onRefresh={() => void loadUsageLogs(usagePage, usagePageSize, usageQuery, usageStatus, usageSort)}
          onPageChange={setUsagePage}
          onPageSizeChange={(nextPageSize) => {
            setUsagePageSize(nextPageSize);
            setUsagePage(1);
          }}
        />
      </section>

      <TopUpDialog
        open={topUpDialogOpen}
        onOpenChange={setTopUpDialogOpen}
        amount={topUpAmount}
        currentBalance={formatAccountBalance(billingAccount?.balanceUSD ?? 0)}
        billingLoading={billingLoading}
        topUpLoading={topUpLoading}
        paymentDisabled={paymentDisabled}
        paymentProviders={paymentProviders}
        selectedPaymentProvider={selectedPaymentProvider}
        selectedEPayType={selectedEPayType}
        epayTypes={epayTypes}
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
