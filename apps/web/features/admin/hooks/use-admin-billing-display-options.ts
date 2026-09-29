"use client";

import * as React from "react";

import { getAdminBillingConfig } from "@/features/admin/api/billing";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { normalizeBillingDisplayCurrency, type BillingDisplayOptions } from "@/entities/billing";

// useAdminBillingDisplayOptions loads admin billing display config (currency and exchange rate), falling back to USD on failure.
export function useAdminBillingDisplayOptions(): BillingDisplayOptions {
  const [billingDisplay, setBillingDisplay] = React.useState<BillingDisplayOptions>({
    currency: "USD",
    usdToCnyRate: null,
  });

  React.useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) return;
        const result = await getAdminBillingConfig(token);
        if (cancelled) return;
        setBillingDisplay({
          currency: normalizeBillingDisplayCurrency(result.config.displayCurrency),
          usdToCnyRate: result.config.usdToCNYRate ?? null,
        });
      } catch {
        if (!cancelled) {
          setBillingDisplay({ currency: "USD", usdToCnyRate: null });
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  return billingDisplay;
}
