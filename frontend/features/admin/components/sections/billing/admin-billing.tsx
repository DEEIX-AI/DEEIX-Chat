"use client";

import { Separator } from "@/components/ui/separator";
import { BillingConfigSection } from "@/features/admin/components/sections/billing/billing-config";
import { BillingMCPToolsSection } from "@/features/admin/components/sections/billing/billing-mcp-tools";
import { BillingPlanSection } from "@/features/admin/components/sections/billing/billing-plan";
import { BillingPricesSection } from "@/features/admin/components/sections/billing/billing-prices";
import { BillingRedemptionSection } from "@/features/admin/components/sections/billing/billing-redemption";
import { BillingToolsSection } from "@/features/admin/components/sections/billing/billing-tools";
import { useAdminBillingReference } from "@/features/admin/hooks/use-admin-billing-reference";

export function AdminBillingPage() {
  const billing = useAdminBillingReference();
  const billingMode = billing.billingConfig?.mode ?? "self";

  return (
    <div className="space-y-8 pb-10">
      <BillingConfigSection
        billingConfig={billing.billingConfig}
        setBillingConfig={billing.setBillingConfig}
        paymentSettings={billing.paymentSettings}
        setPaymentSettings={billing.setPaymentSettings}
        savedPaymentSettings={billing.savedPaymentSettings}
        setSavedPaymentSettings={billing.setSavedPaymentSettings}
        paymentConfiguredMap={billing.paymentConfiguredMap}
        setPaymentConfiguredMap={billing.setPaymentConfiguredMap}
        loading={billing.loading}
      />

      <Separator className="mx-1 my-10" />

      <BillingRedemptionSection
        plans={billing.plans}
        billingMode={billingMode}
        loading={billing.loading}
      />

      <Separator className="mx-1 my-10" />

      <BillingPlanSection
        plans={billing.plans}
        setPlans={billing.setPlans}
        permissionGroups={billing.permissionGroups}
        loading={billing.loading}
      />

      <Separator className="mx-1 my-10" />

      <BillingPricesSection
        models={billing.models}
        pricingItems={billing.pricingItems}
        setPricingItems={billing.setPricingItems}
        loading={billing.loading}
      />

      <Separator className="mx-1 my-10" />

      <BillingToolsSection
        billingConfig={billing.billingConfig}
        setBillingConfig={billing.setBillingConfig}
        loading={billing.loading}
      />

      <Separator className="mx-1 my-10" />

      <BillingMCPToolsSection />
    </div>
  );
}
