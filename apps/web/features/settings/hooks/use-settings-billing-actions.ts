import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import type { BillingOverview } from "@/features/settings/hooks/use-settings-billing-overview";
import { billingDisplayAmountToMinorUnits } from "@/features/settings/model/subscription-format";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { createBillingCheckout, redeemBillingCode, subscribeBillingPlan } from "@/shared/api/billing";
import type { BillingPlanPriceDTO } from "@/shared/api/billing-types";

export type PaymentProvider = "stripe" | "epay";

function checkoutReturnURLs() {
  return {
    successURL: `${window.location.origin}/settings/subscription?payment=success`,
    cancelURL: `${window.location.origin}/settings/subscription?payment=cancel`,
  };
}

/**
 * Billing mutations on the subscription page. Paid flows open the provider
 * checkout in a new tab; switching to a free plan reloads the page so every
 * section picks up the new entitlement.
 */
export function useSettingsBillingActions({
  accessToken,
  onOverviewChange,
}: {
  accessToken: string;
  onOverviewChange: (overview: BillingOverview) => void;
}) {
  const t = useTranslations("settings.subscriptionPage");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [checkoutPriceID, setCheckoutPriceID] = React.useState<number | null>(null);
  const [topUpLoading, setTopUpLoading] = React.useState(false);
  const [redemptionLoading, setRedemptionLoading] = React.useState(false);

  const checkout = React.useCallback(async (price: BillingPlanPriceDTO, paymentProvider: PaymentProvider, epayType?: string) => {
    setCheckoutPriceID(price.id);
    try {
      const data = await createBillingCheckout(accessToken, {
        orderType: "subscription",
        priceID: price.id,
        cycles: 1,
        paymentProvider,
        epayType: paymentProvider === "epay" ? epayType : undefined,
        ...checkoutReturnURLs(),
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

  const subscribeFreePlan = React.useCallback(async (price: BillingPlanPriceDTO) => {
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

  const topUp = React.useCallback(async (amount: string, paymentProvider: PaymentProvider, epayType: string) => {
    const displayAmount = Number(amount);
    const amountMinorUnits = billingDisplayAmountToMinorUnits(displayAmount);
    if (!Number.isFinite(displayAmount) || displayAmount <= 0 || amountMinorUnits <= 0) {
      toast.error(t("toasts.invalidTopUpAmount"), { description: t("toasts.invalidTopUpAmountDescription") });
      return;
    }
    setTopUpLoading(true);
    try {
      const data = await createBillingCheckout(accessToken, {
        orderType: "topup",
        amountMinorUnits,
        cycles: 1,
        paymentProvider,
        epayType: paymentProvider === "epay" ? epayType : undefined,
        ...checkoutReturnURLs(),
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
  }, [accessToken, resolveErrorMessage, t]);

  // `onRedeemed` runs before the success toast, matching the original dialog close + code reset timing.
  const redeemCode = React.useCallback(async (rawCode: string, onRedeemed: () => void) => {
    const code = rawCode.trim();
    if (!code) {
      toast.error(t("toasts.invalidRedemptionCode"));
      return;
    }
    setRedemptionLoading(true);
    try {
      const data = await redeemBillingCode(accessToken, { code });
      onOverviewChange(data.overview);
      onRedeemed();
      toast.success(t("toasts.redemptionSucceeded"));
    } catch (error) {
      toast.error(t("toasts.redemptionFailed"), { description: resolveErrorMessage(error, t("toasts.retryLater")) });
    } finally {
      setRedemptionLoading(false);
    }
  }, [accessToken, onOverviewChange, resolveErrorMessage, t]);

  return {
    checkoutPriceID,
    topUpLoading,
    redemptionLoading,
    checkout,
    subscribeFreePlan,
    topUp,
    redeemCode,
  };
}
