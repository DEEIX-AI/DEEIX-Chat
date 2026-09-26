"use client";

import * as React from "react";
import { Save } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { invalidateAdminReferenceDataCache, patchAdminBillingConfig, patchAdminSettings } from "@/features/admin/api";
import type { AdminBillingConfigDTO, AdminBillingMode } from "@/features/admin/api/billing.types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import {
  flattenPaymentSettings,
  formatBillingAmountInput,
  normalizePaymentProviders,
  parseEPayTypesJSON,
  paymentPatchItems,
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
import { resolveApiBaseURL } from "@/shared/api/http-client";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { configuredSettingsMap } from "@/shared/lib/settings-meta";

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
  const [saving, setSaving] = React.useState(false);
  const [paymentTab, setPaymentTab] = React.useState<PaymentProvider>("stripe");
  const [billingUsdToCnyRate, setBillingUsdToCnyRate] = React.useState("7.2");
  const [savedBillingUsdToCnyRate, setSavedBillingUsdToCnyRate] = React.useState("7.2");
  const [prepaidAmount, setPrepaidAmount] = React.useState("0");
  const [savedPrepaidAmount, setSavedPrepaidAmount] = React.useState("0");
  const stripeWebhookEndpoint = React.useMemo(() => `${resolveApiBaseURL()}/api/v1/billing/payments/stripe/webhook`, []);

  const billingMode = billingConfig?.mode ?? "self";
  const billingDisplayCurrency = billingConfig?.displayCurrency === "CNY" ? "CNY" : "USD";
  const billingPrepaidAmountUSD = billingConfig?.prepaidAmountUSD;
  const billingUsdToCNYRate = billingConfig?.usdToCNYRate;

  React.useEffect(() => {
    if (billingPrepaidAmountUSD == null || billingUsdToCNYRate == null) {
      return;
    }
    const nextPrepaidAmount = formatBillingAmountInput(billingPrepaidAmountUSD);
    const nextUsdToCnyRate = formatBillingAmountInput(billingUsdToCNYRate);
    setPrepaidAmount(nextPrepaidAmount);
    setSavedPrepaidAmount(nextPrepaidAmount);
    setBillingUsdToCnyRate(nextUsdToCnyRate);
    setSavedBillingUsdToCnyRate(nextUsdToCnyRate);
  }, [billingPrepaidAmountUSD, billingUsdToCNYRate]);

  const paymentProviders = React.useMemo(() => normalizePaymentProviders(paymentSettings.payment_providers), [paymentSettings.payment_providers]);
  const stripeEnabled = paymentProviders.includes("stripe");
  const epayEnabled = paymentProviders.includes("epay");
  const isPaymentDirty = React.useMemo(
    () => paymentSettingsChanged(paymentSettings, savedPaymentSettings),
    [paymentSettings, savedPaymentSettings],
  );
  const prepaidAmountChanged = prepaidAmount.trim() !== savedPrepaidAmount.trim();
  const billingRateChanged = billingUsdToCnyRate.trim() !== savedBillingUsdToCnyRate.trim();
  const billingConfigActions = ((billingMode !== "self" && prepaidAmountChanged) || billingRateChanged) ? (
    <Button
      type="button"
      size="sm"
      disabled={loading || saving}
      onClick={() => void handleBillingConfigSave()}
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

  async function savePaymentSettings() {
    const providers = normalizePaymentProviders(paymentSettings.payment_providers);
    if (providers.includes("stripe") && ((!paymentSettings.stripe_secret_key.trim() && !paymentConfiguredMap["billing.stripe_secret_key"]) || (!paymentSettings.stripe_webhook_secret.trim() && !paymentConfiguredMap["billing.stripe_webhook_secret"]))) {
      toast.error(t("toast.paymentIncomplete"), { description: t("toast.stripeRequired") });
      return;
    }
    if (providers.includes("epay") && (!paymentSettings.epay_gateway_url.trim() || !paymentSettings.epay_types.trim() || !paymentSettings.epay_pid.trim() || (!paymentSettings.epay_key.trim() && !paymentConfiguredMap["billing.epay_key"]))) {
      toast.error(t("toast.paymentIncomplete"), { description: t("toast.epayRequired") });
      return;
    }
    if (providers.includes("epay") && !parseEPayTypesJSON(paymentSettings.epay_types)) {
      toast.error(t("toast.paymentIncomplete"), { description: t("toast.epayTypesInvalid") });
      return;
    }

    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const grouped = await patchAdminSettings(token, { items: paymentPatchItems(paymentSettings) });
      const next = flattenPaymentSettings(grouped.billing || []);
      setPaymentConfiguredMap(configuredSettingsMap(grouped));
      setPaymentSettings(next);
      setSavedPaymentSettings(next);
      toast.success(t("toast.paymentSaved"));
    } catch (error) {
      toast.error(t("toast.paymentSaveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }

  async function handleBillingModeChange(nextMode: AdminBillingMode) {
    if (nextMode === billingMode) {
      return;
    }
    const previous = billingMode;
    setBillingConfig((current) => current ? { ...current, mode: nextMode } : current);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        setBillingConfig((current) => current ? { ...current, mode: previous } : current);
        return;
      }
      await patchAdminBillingConfig(token, { mode: nextMode });
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.billingModeChanged", { mode: t(`billingConfig.modes.${nextMode}`) }));
    } catch (error) {
      setBillingConfig((current) => current ? { ...current, mode: previous } : current);
      toast.error(t("toast.billingModeFailed"), { description: resolveAdminErrorMessage(error) });
    }
  }

  async function handleBillingDisplayCurrencyChange(nextCurrency: "USD" | "CNY") {
    if (nextCurrency === billingDisplayCurrency) {
      return;
    }
    const previous = billingDisplayCurrency;
    setBillingConfig((current) => current ? { ...current, displayCurrency: nextCurrency } : current);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        setBillingConfig((current) => current ? { ...current, displayCurrency: previous } : current);
        return;
      }
      const result = await patchAdminBillingConfig(token, {
        mode: billingMode,
        displayCurrency: nextCurrency,
      });
      setBillingConfig((current) => current ? { ...current, displayCurrency: result.config.displayCurrency } : result.config);
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.displayCurrencySaved"));
    } catch (error) {
      setBillingConfig((current) => current ? { ...current, displayCurrency: previous } : current);
      toast.error(t("toast.displayCurrencySaveFailed"), { description: resolveAdminErrorMessage(error) });
    }
  }

  async function handleBillingConfigSave() {
    const amount = Number(prepaidAmount);
    const usdToCnyRate = Number(billingUsdToCnyRate);
    if (billingMode !== "self" && (!Number.isFinite(amount) || amount < 0)) {
      toast.error(t("toast.prepaidInvalid"), { description: t("toast.prepaidInvalidDescription") });
      return;
    }
    if (!Number.isFinite(usdToCnyRate) || usdToCnyRate <= 0) {
      toast.error(t("toast.usdToCnyRateInvalid"), { description: t("toast.usdToCnyRateInvalidDescription") });
      return;
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const result = await patchAdminBillingConfig(token, {
        mode: billingMode,
        prepaidAmountUSD: billingMode !== "self" ? amount : undefined,
        usdToCNYRate: usdToCnyRate,
      });
      const nextAmount = formatBillingAmountInput(result.config.prepaidAmountUSD);
      const nextUsdToCnyRate = formatBillingAmountInput(result.config.usdToCNYRate);
      setPrepaidAmount(nextAmount);
      setSavedPrepaidAmount(nextAmount);
      setBillingUsdToCnyRate(nextUsdToCnyRate);
      setSavedBillingUsdToCnyRate(nextUsdToCnyRate);
      setBillingConfig((current) => current ? {
        ...current,
        mode: result.config.mode,
        prepaidAmountUSD: result.config.prepaidAmountUSD,
        prepaidAmountNanousd: result.config.prepaidAmountNanousd,
        usdToCNYRate: result.config.usdToCNYRate,
        displayCurrency: result.config.displayCurrency,
      } : result.config);
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.billingConfigSaved"));
    } catch (error) {
      toast.error(t("toast.billingConfigSaveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }

  return (
    <>
      <SettingsSection title={t("billingConfig.title")} actions={billingConfigActions} className="px-1">
        <SettingsFieldList>
          <SettingsFieldItem>
            <SettingsFieldRow
              title={t("billingConfig.mode")}
              description={t("billingConfig.modeDescription")}
            >
              <div className="w-full">
                <Select value={billingMode} onValueChange={(value) => void handleBillingModeChange(value as AdminBillingMode)} disabled={loading || saving}>
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
          <SettingsFieldItem index={1}>
            <SettingsFieldRow
              title={t("billingConfig.displayCurrency")}
              description={t("billingConfig.displayCurrencyDescription")}
            >
              <div className="w-full">
                <Select
                  value={billingDisplayCurrency}
                  onValueChange={(value) => void handleBillingDisplayCurrencyChange(value as "USD" | "CNY")}
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
            <Tabs value={paymentTab} onValueChange={(value) => setPaymentTab(value as PaymentProvider)}>
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
    </>
  );
}
