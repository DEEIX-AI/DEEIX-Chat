"use client";

import * as React from "react";
import { Save } from "lucide-react";
import { useMessages, useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { invalidateAdminReferenceDataCache, patchAdminBillingConfig } from "@/features/admin/api";
import type {
  AdminBillingConfigDTO,
  AdminNativeToolPricingPayload,
  NativeToolPricingDTO,
} from "@/features/admin/api/billing.types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { CollapsibleMotionContent } from "@/shared/components/collapsible-motion-content";
import {
  SettingsFieldItem,
  SettingsFieldList,
  SettingsFieldRow,
  SettingsSection,
} from "@/shared/components/settings-layout";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { localizedNativeToolText } from "@/shared/lib/native-tool-i18n";

type BillingToolsSectionProps = {
  billingConfig: AdminBillingConfigDTO | null;
  setBillingConfig: React.Dispatch<React.SetStateAction<AdminBillingConfigDTO | null>>;
  loading: boolean;
};

function formatNativeToolPriceInput(priceNanousd: number): string {
  if (!Number.isFinite(priceNanousd) || priceNanousd <= 0) {
    return "0";
  }
  return String(priceNanousd / 1_000_000_000);
}

function nativeToolPriceInputToNanousd(value: string): number | null {
  const parsed = Number(value.trim());
  if (!Number.isFinite(parsed) || parsed < 0) {
    return null;
  }
  return Math.round(parsed * 1_000_000_000);
}

function nativeToolPriceDraftsFrom(items: NativeToolPricingDTO[]): Record<string, string> {
  return Object.fromEntries(items.map((item) => [item.toolKey, formatNativeToolPriceInput(item.priceNanousd)]));
}

function nativeToolPricingSignature(items: NativeToolPricingDTO[]): string {
  return JSON.stringify(items.map((item) => ({
    toolKey: item.toolKey,
    label: item.label,
    description: item.description,
    type: item.type,
    priceNanousd: item.priceNanousd,
    unit: item.unit,
    priceLabel: item.priceLabel,
    billable: item.billable,
  })).sort((left, right) => left.toolKey.localeCompare(right.toolKey)));
}

function normalizeNativeToolPricingForSave(items: NativeToolPricingDTO[]): AdminNativeToolPricingPayload[] {
  return items.map((item) => ({
    toolKey: item.toolKey,
    priceNanousd: item.priceNanousd,
    unit: "call",
    priceLabel: "",
    billable: item.priceNanousd > 0,
  }));
}

export function BillingToolsSection({ billingConfig, setBillingConfig, loading }: BillingToolsSectionProps) {
  const messages = useMessages();
  const t = useTranslations("adminBilling");
  const tActions = useTranslations("common.actions");
  const [nativeToolBillingEnabled, setNativeToolBillingEnabled] = React.useState(true);
  const [savedNativeToolBillingEnabled, setSavedNativeToolBillingEnabled] = React.useState(true);
  const [nativeToolPricing, setNativeToolPricing] = React.useState<NativeToolPricingDTO[]>([]);
  const [savedNativeToolPricing, setSavedNativeToolPricing] = React.useState<NativeToolPricingDTO[]>([]);
  const [nativeToolPriceDrafts, setNativeToolPriceDrafts] = React.useState<Record<string, string>>({});
  const [nativeToolBillingSaving, setNativeToolBillingSaving] = React.useState(false);
  const billingNativeToolBillingEnabled = billingConfig?.nativeToolBillingEnabled;
  const billingNativeToolPricing = billingConfig?.nativeToolPricing;

  React.useEffect(() => {
    if (billingNativeToolBillingEnabled == null) {
      return;
    }
    const nextEnabled = Boolean(billingNativeToolBillingEnabled);
    const nextPricing = billingNativeToolPricing ?? [];
    setNativeToolBillingEnabled(nextEnabled);
    setSavedNativeToolBillingEnabled(nextEnabled);
    setNativeToolPricing(nextPricing);
    setSavedNativeToolPricing(nextPricing);
    setNativeToolPriceDrafts(nativeToolPriceDraftsFrom(nextPricing));
  }, [billingNativeToolBillingEnabled, billingNativeToolPricing]);

  const nativeToolBillingChanged = nativeToolBillingEnabled !== savedNativeToolBillingEnabled;
  const nativeToolPricingChanged = React.useMemo(
    () => nativeToolPricingSignature(nativeToolPricing) !== nativeToolPricingSignature(savedNativeToolPricing),
    [nativeToolPricing, savedNativeToolPricing],
  );
  const toolPricingActions = nativeToolBillingChanged || nativeToolPricingChanged ? (
    <Button
      type="button"
      size="sm"
      disabled={loading || nativeToolBillingSaving}
      onClick={() => void handleNativeToolBillingSave()}
    >
      {nativeToolBillingSaving ? <SpinnerLabel>{tActions("saving")}</SpinnerLabel> : (
        <>
          <Save className="size-3.5" />
          {tActions("save")}
        </>
      )}
    </Button>
  ) : null;

  async function handleNativeToolBillingSave() {
    setNativeToolBillingSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const nextNativeToolPricing = normalizeNativeToolPricingForSave(nativeToolPricing);
      const result = await patchAdminBillingConfig(token, {
        mode: billingConfig?.mode ?? "self",
        nativeToolBillingEnabled,
        nativeToolPricing: nextNativeToolPricing,
      });
      const savedValue = Boolean(result.config.nativeToolBillingEnabled);
      const savedPricing = result.config.nativeToolPricing ?? nativeToolPricing;
      setNativeToolBillingEnabled(savedValue);
      setSavedNativeToolBillingEnabled(savedValue);
      setNativeToolPricing(savedPricing);
      setSavedNativeToolPricing(savedPricing);
      setNativeToolPriceDrafts(nativeToolPriceDraftsFrom(savedPricing));
      setBillingConfig((current) => current ? {
        ...current,
        nativeToolBillingEnabled: savedValue,
        nativeToolPricing: savedPricing,
      } : result.config);
      invalidateAdminReferenceDataCache();
      toast.success(t("toast.nativeToolBillingSaved"));
    } catch (error) {
      toast.error(t("toast.nativeToolBillingSaveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setNativeToolBillingSaving(false);
    }
  }

  return (
    <SettingsSection title={t("toolPricing.title")} actions={toolPricingActions} className="px-1">
      <SettingsFieldList>
        <SettingsFieldItem>
          <SettingsFieldRow
            title={t("toolPricing.nativeToolBilling")}
            description={t("toolPricing.nativeToolBillingDescription")}
          >
            <Switch
              checked={nativeToolBillingEnabled}
              disabled={loading || nativeToolBillingSaving}
              onCheckedChange={setNativeToolBillingEnabled}
              aria-label={t("toolPricing.nativeToolBilling")}
            />
          </SettingsFieldRow>
        </SettingsFieldItem>
      </SettingsFieldList>
      <CollapsibleMotionContent open={nativeToolBillingEnabled} contentClassName="mt-5 space-y-2">
        <p className="px-1 text-[11px] leading-5 text-muted-foreground">
          {t("toolPricing.nativeToolCount", { count: nativeToolPricing.length })}
        </p>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("toolPricing.provider")}</TableHead>
              <TableHead>{t("toolPricing.tool")}</TableHead>
              <TableHead>{t("toolPricing.type")}</TableHead>
              <TableHead className="text-right">{t("toolPricing.price")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {nativeToolPricing.map((row) => {
              const label = localizedNativeToolText(messages, "nativeToolLabels", row.toolKey) || row.label || row.type || row.toolKey;
              const description = localizedNativeToolText(messages, "nativeToolDescriptions", row.toolKey) || row.description || row.type || row.toolKey;
              return (
                <TableRow key={`${row.provider}-${row.toolKey}`}>
                  <TableCell className="py-1.5 text-xs text-muted-foreground">{row.provider}</TableCell>
                  <TableCell className="py-1.5 text-xs text-foreground">
                    <div className="flex min-w-0 flex-col">
                      <span className="truncate">{label}</span>
                      <span className="truncate text-[11px] text-muted-foreground">{description}</span>
                    </div>
                  </TableCell>
                  <TableCell className="py-1.5 font-mono text-xs text-muted-foreground">{row.type || row.toolKey}</TableCell>
                  <TableCell className="py-1.5 text-right font-mono text-xs text-muted-foreground">
                    <div className="flex items-center justify-end gap-1.5">
                      <span className="text-muted-foreground">$</span>
                      <Input
                        value={nativeToolPriceDrafts[row.toolKey] ?? formatNativeToolPriceInput(row.priceNanousd)}
                        inputMode="decimal"
                        className="h-7 w-24 text-right font-mono text-xs"
                        disabled={loading || nativeToolBillingSaving}
                        aria-label={`${label} ${t("toolPricing.price")}`}
                        onChange={(event) => {
                          const nextDraft = event.target.value;
                          const nextNanousd = nativeToolPriceInputToNanousd(nextDraft);
                          setNativeToolPriceDrafts((current) => ({
                            ...current,
                            [row.toolKey]: nextDraft,
                          }));
                          if (nextNanousd === null) {
                            return;
                          }
                          setNativeToolPricing((current) => current.map((item) => (
                            item.toolKey === row.toolKey
                              ? { ...item, priceNanousd: nextNanousd, unit: "call", priceLabel: "", billable: nextNanousd > 0 }
                              : item
                          )));
                        }}
                      />
                      <span className="whitespace-nowrap text-muted-foreground">
                        / {t("toolPricing.units.call")}
                      </span>
                    </div>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
        <p className="text-[11px] leading-5 text-muted-foreground">{t("toolPricing.defaultPriceDescription")}</p>
        <p className="text-[11px] leading-5 text-muted-foreground">{t("toolPricing.note")}</p>
      </CollapsibleMotionContent>
    </SettingsSection>
  );
}
