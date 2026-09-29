"use client";

import * as React from "react";
import { Save } from "lucide-react";
import { useTranslations } from "next-intl";

import { FeatureGate } from "@/shared/capabilities";

import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import type { AdminBillingConfigDTO, AdminBillingMode } from "@/features/admin/api/billing-types";
import { type BillingDisplayCurrency, useAdminBillingConfig } from "@/features/admin/hooks/use-admin-billing-config";
import {
  normalizePaymentProviders,
  paymentProviderSetting,
  paymentSettingsChanged,
  type PaymentProvider,
  type PaymentSettings,
} from "@/features/admin/model/billing-settings";
import { CopyActionButton } from "@/shared/components/copy-action";
import { CollapsibleMotionContent } from "@/shared/components/collapsible-motion-content";
import {
  SettingsFieldItem,
  SettingsFieldList,
  SettingsFieldRow,
  SettingsSection,
} from "@/shared/components/settings-layout";
import { isOneOf } from "@/shared/lib/type-guards";

const BILLING_MODES = ["self", "period", "usage"] as const satisfies readonly AdminBillingMode[];
const isBillingMode = isOneOf(BILLING_MODES);
const BILLING_DISPLAY_CURRENCIES = ["USD", "CNY"] as const satisfies readonly BillingDisplayCurrency[];
const isBillingDisplayCurrency = isOneOf(BILLING_DISPLAY_CURRENCIES);
const PAYMENT_PROVIDERS = ["stripe", "epay"] as const satisfies readonly PaymentProvider[];
const isPaymentProvider = isOneOf(PAYMENT_PROVIDERS);

type BillingConfigSectionProps = {
  billingConfig: AdminBillingConfigDTO | null;
  setBillingConfig: React.Dispatch<React.SetStateAction<AdminBillingConfigDTO | null>>;
  paymentSettings: PaymentSettings;
  setPaymentSettings: React.Dispatch<React.SetStateAction<PaymentSettings>>;
  savedPaymentSettings: PaymentSettings;
  setSavedPaymentSettings: React.Dispatch<React.SetStateAction<PaymentSettings>>;
  paymentConfiguredMap: Record<string, boolean>;
  setPaymentConfiguredMap: React.Dispatch<React.SetStateAction<Record<string, boolean>>>;
  loading: boolean;
};

export function BillingConfigSection({
  billingConfig,
  setBillingConfig,
  paymentSettings,
  setPaymentSettings,
  savedPaymentSettings,
  setSavedPaymentSettings,
  paymentConfiguredMap,
  setPaymentConfiguredMap,
  loading,
}: BillingConfigSectionProps) {
  const t = useTranslations("adminBilling");
  const tActions = useTranslations("common.actions");
  const tCommonErrors = useTranslations("common.errors");
  const tInput = useTranslations("common.input");
  const [paymentTab, setPaymentTab] = React.useState<PaymentProvider>("stripe");
  const {
    saving,
    billingMode,
    billingDisplayCurrency,
    billingUsdToCnyRate,
    setBillingUsdToCnyRate,
    prepaidAmount,
    setPrepaidAmount,
    prepaidAmountChanged,
    billingRateChanged,
    stripeWebhookEndpoint,
    savePaymentSettings,
    changeBillingMode,
    changeBillingDisplayCurrency,
    saveBillingConfig,
  } = useAdminBillingConfig({
    billingConfig,
    setBillingConfig,
    paymentSettings,
    setPaymentSettings,
    setSavedPaymentSettings,
    paymentConfiguredMap,
    setPaymentConfiguredMap,
  });

  const paymentProviders = React.useMemo(() => normalizePaymentProviders(paymentSettings.payment_providers), [paymentSettings.payment_providers]);
  const stripeEnabled = paymentProviders.includes("stripe");
  const epayEnabled = paymentProviders.includes("epay");
  const isPaymentDirty = React.useMemo(
    () => paymentSettingsChanged(paymentSettings, savedPaymentSettings),
    [paymentSettings, savedPaymentSettings],
  );
  const billingConfigActions = ((billingMode !== "self" && prepaidAmountChanged) || billingRateChanged) ? (
    <Button
      type="button"
      size="sm"
      disabled={loading || saving}
      onClick={() => void saveBillingConfig()}
    >
      {saving ? <SpinnerLabel>{tActions("saving")}</SpinnerLabel> : (
        <>
          <Save className="size-3.5" />
          {tActions("save")}
        </>
      )}
    </Button>
  ) : null;

  function updatePaymentSetting(key: keyof PaymentSettings, value: string) {
    setPaymentSettings((current) => ({ ...current, [key]: value }));
  }

  function setPaymentProviderEnabled(provider: PaymentProvider, enabled: boolean) {
    setPaymentSettings((current) => {
      const providers = normalizePaymentProviders(current.payment_providers);
      const next = enabled
        ? Array.from(new Set([...providers, provider]))
        : providers.filter((item) => item !== provider);
      return { ...current, payment_providers: paymentProviderSetting(next) };
    });
  }

  return (
    <>
      <SettingsSection title={t("billingConfig.title")} actions={billingConfigActions} className="px-1">
        <SettingsFieldList>
          <FeatureGate feature="billingGating">
            <SettingsFieldItem>
              <SettingsFieldRow
                title={t("billingConfig.mode")}
                description={t("billingConfig.modeDescription")}
              >
                <div className="w-full">
                  <Select value={billingMode} onValueChange={(value) => {
                    if (isBillingMode(value)) void changeBillingMode(value);
                  }} disabled={loading || saving}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent align="end">
                      <SelectItem value="self">{t("billingConfig.modes.self")}</SelectItem>
                      <SelectItem value="period">{t("billingConfig.modes.period")}</SelectItem>
                      <SelectItem value="usage">{t("billingConfig.modes.usage")}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </SettingsFieldRow>
            </SettingsFieldItem>
          </FeatureGate>
          <SettingsFieldItem index={1}>
            <SettingsFieldRow
              title={t("billingConfig.displayCurrency")}
              description={t("billingConfig.displayCurrencyDescription")}
            >
              <div className="w-full">
                <Select
                  value={billingDisplayCurrency}
                  onValueChange={(value) => {
                    if (isBillingDisplayCurrency(value)) void changeBillingDisplayCurrency(value);
                  }}
                  disabled={loading || saving}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent align="end">
                    <SelectItem value="USD">{t("billingConfig.displayCurrencies.usd")}</SelectItem>
                    <SelectItem value="CNY">{t("billingConfig.displayCurrencies.cny")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </SettingsFieldRow>
          </SettingsFieldItem>
          <SettingsFieldItem index={2}>
            <SettingsFieldRow
              title={t("billingConfig.usdToCnyRate")}
              description={t("billingConfig.usdToCnyRateDescription")}
            >
              <div className="w-full">
                <Input
                  id="billing.usd_to_cny_rate"
                  type="number"
                  min={0.000001}
                  step="0.0001"
                  value={billingUsdToCnyRate}
                  className="text-right"
                  disabled={loading || saving}
                  onChange={(event) => setBillingUsdToCnyRate(event.target.value)}
                />
              </div>
            </SettingsFieldRow>
          </SettingsFieldItem>
          {billingMode !== "self" ? (
            <SettingsFieldItem index={3}>
              <SettingsFieldRow
                title={t("billingConfig.prepaidAmount")}
                description={t("billingConfig.prepaidAmountDescription")}
              >
                <div className="w-full">
                  <Input
                    type="number"
                    min={0}
                    step="0.01"
                    value={prepaidAmount}
                    className="text-right"
                    disabled={loading || saving}
                    onChange={(event) => setPrepaidAmount(event.target.value)}
                  />
                </div>
              </SettingsFieldRow>
            </SettingsFieldItem>
          ) : null}
        </SettingsFieldList>
      </SettingsSection>

      <FeatureGate feature="billingGating">
        <Separator className="mx-1 my-10" />

        <section className="space-y-6 px-1">
          <div className="flex h-10 items-center justify-between gap-3">
            <h3 className="text-sm font-semibold">{t("payment.title")}</h3>
            {isPaymentDirty ? (
              <Button type="button" size="sm" onClick={() => void savePaymentSettings()} disabled={loading || saving}>
                {saving ? <SpinnerLabel>{tActions("saving")}</SpinnerLabel> : (
                  <>
                    <Save className="size-3.5" />
                    {tActions("save")}
                  </>
                )}
              </Button>
            ) : null}
          </div>

          <FieldGroup className="gap-0">
            <div>
              <Tabs value={paymentTab} onValueChange={(value) => {
                if (isPaymentProvider(value)) setPaymentTab(value);
              }}>
                <SettingsFieldRow
                  title={t("payment.channels")}
                  description={t("payment.channelsDescription")}
                >
                  <TabsList className="h-8 w-full">
                    <TabsTrigger value="stripe">Stripe</TabsTrigger>
                    <TabsTrigger value="epay">EPay</TabsTrigger>
                  </TabsList>
                </SettingsFieldRow>

                <TabsContent value="stripe" className="mt-4 space-y-4">
                  <SettingsFieldRow
                    title={t("payment.enableStripe")}
                    description={t("payment.enableStripeDescription")}
                  >
                    <Switch size="sm" checked={stripeEnabled} disabled={loading || saving} onCheckedChange={(checked) => setPaymentProviderEnabled("stripe", checked)} />
                  </SettingsFieldRow>
                  <CollapsibleMotionContent open={stripeEnabled} contentClassName="space-y-4">
                    <SettingsFieldRow
                      title={t("payment.stripeWebhookEndpoint")}
                      description={t("payment.stripeWebhookEndpointDescription")}
                    >
                      <div className="grid w-full min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-1">
                        <Input value={stripeWebhookEndpoint} className="min-w-0 truncate text-left text-xs md:text-right" readOnly />
                        <CopyActionButton
                          type="button"
                          variant="secondary"
                          size="icon"
                          className="size-8 shrink-0 rounded-md shadow-none active:scale-90 transition-transform"
                          value={stripeWebhookEndpoint}
                          messages={{ copied: tActions("copied"), failed: tCommonErrors("copyFailed") }}
                          aria-label={tActions("copy")}
                          title={tActions("copy")}
                        />
                      </div>
                    </SettingsFieldRow>
                    <SettingsFieldRow
                      title={t("payment.stripePublishableKey")}
                      description={t("payment.stripePublishableKeyDescription")}
                    >
                      <Input value={paymentSettings.stripe_publishable_key} className="text-right" disabled={loading || saving} placeholder="pk_..." onChange={(event) => updatePaymentSetting("stripe_publishable_key", event.target.value)} />
                    </SettingsFieldRow>
                    <SettingsFieldRow
                      title={t("payment.stripeSecretKey")}
                      description={t("payment.stripeSecretKeyDescription")}
                    >
                      <Input value={paymentSettings.stripe_secret_key} className="text-right" type="password" disabled={loading || saving} placeholder={paymentConfiguredMap["billing.stripe_secret_key"] ? tInput("configuredPasswordPlaceholder") : "sk_..."} onChange={(event) => updatePaymentSetting("stripe_secret_key", event.target.value)} />
                    </SettingsFieldRow>
                    <SettingsFieldRow
                      title={t("payment.stripeWebhookSecret")}
                      description={t("payment.stripeWebhookSecretDescription")}
                    >
                      <Input value={paymentSettings.stripe_webhook_secret} className="text-right" type="password" disabled={loading || saving} placeholder={paymentConfiguredMap["billing.stripe_webhook_secret"] ? tInput("configuredPasswordPlaceholder") : "whsec_..."} onChange={(event) => updatePaymentSetting("stripe_webhook_secret", event.target.value)} />
                    </SettingsFieldRow>
                  </CollapsibleMotionContent>
                </TabsContent>

                <TabsContent value="epay" className="mt-4 space-y-4">
                  <SettingsFieldRow
                    title={t("payment.enableEPay")}
                    description={t("payment.enableEPayDescription")}
                  >
                    <Switch size="sm" checked={epayEnabled} disabled={loading || saving} onCheckedChange={(checked) => setPaymentProviderEnabled("epay", checked)} />
                  </SettingsFieldRow>
                  <CollapsibleMotionContent open={epayEnabled} contentClassName="space-y-4">
                    <SettingsFieldRow
                      title={t("payment.epayGateway")}
                      description={t("payment.epayGatewayDescription")}
                    >
                      <Input value={paymentSettings.epay_gateway_url} className="text-right" disabled={loading || saving} placeholder="https://..." onChange={(event) => updatePaymentSetting("epay_gateway_url", event.target.value)} />
                    </SettingsFieldRow>
                    <SettingsFieldRow
                      title={t("payment.epayPid")}
                      description={t("payment.epayPidDescription")}
                    >
                      <Input value={paymentSettings.epay_pid} className="text-right" disabled={loading || saving} onChange={(event) => updatePaymentSetting("epay_pid", event.target.value)} />
                    </SettingsFieldRow>
                    <SettingsFieldRow
                      title={t("payment.epayKey")}
                      description={t("payment.epayKeyDescription")}
                    >
                      <Input value={paymentSettings.epay_key} className="text-right" type="password" disabled={loading || saving} placeholder={paymentConfiguredMap["billing.epay_key"] ? tInput("configuredPasswordPlaceholder") : ""} onChange={(event) => updatePaymentSetting("epay_key", event.target.value)} />
                    </SettingsFieldRow>
                    <Field>
                      <div className="space-y-2">
                        <div>
                          <FieldLabel>{t("payment.epayTypes")}</FieldLabel>
                          <FieldDescription className="text-[11px]">{t("payment.epayTypesDescription")}</FieldDescription>
                        </div>
                        <Textarea
                          value={paymentSettings.epay_types}
                          className="h-28 w-full resize-none overflow-y-auto font-mono [field-sizing:fixed]"
                          disabled={loading || saving}
                          spellCheck={false}
                          onChange={(event) => updatePaymentSetting("epay_types", event.target.value)}
                        />
                      </div>
                    </Field>
                  </CollapsibleMotionContent>
                </TabsContent>
              </Tabs>
            </div>
          </FieldGroup>
        </section>
      </FeatureGate>
    </>
  );
}
