"use client";

import type * as React from "react";
import { CircleHelp, Save } from "lucide-react";
import { useMessages, useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useVirtualTableRows, VirtualTablePaddingRow } from "@/components/ui/virtual-table";
import type { AdminBillingConfigDTO } from "@/features/admin/api/billing-types";
import {
  formatNativeToolPriceInput,
  useAdminBillingNativeTools,
} from "@/features/admin/hooks/use-admin-billing-native-tools";
import { CollapsibleMotionContent } from "@/shared/components/collapsible-motion-content";
import {
  SettingsFieldItem,
  SettingsFieldList,
  SettingsFieldRow,
  SettingsSection,
} from "@/shared/components/settings-layout";
import { localizedNativeToolText } from "@/entities/model";

type BillingToolsSectionProps = {
  billingConfig: AdminBillingConfigDTO | null;
  setBillingConfig: React.Dispatch<React.SetStateAction<AdminBillingConfigDTO | null>>;
  loading: boolean;
};

export function BillingToolsSection({ billingConfig, setBillingConfig, loading }: BillingToolsSectionProps) {
  const messages = useMessages();
  const t = useTranslations("adminBilling");
  const tActions = useTranslations("common.actions");
  const {
    nativeToolBillingEnabled,
    setNativeToolBillingEnabled,
    nativeToolPricing,
    nativeToolPriceDrafts,
    nativeToolBillingSaving,
    nativeToolBillingChanged,
    nativeToolPricingChanged,
    updateNativeToolPriceDraft,
    saveNativeToolBilling,
  } = useAdminBillingNativeTools({ billingConfig, setBillingConfig });

  const nativeToolVirtualRows = useVirtualTableRows(nativeToolPricing, {
    enabled: nativeToolPricing.length > 100,
    estimateSize: 40,
  });
  const showNativeToolRows = nativeToolPricing.length > 0;

  const toolPricingActions = nativeToolBillingChanged || nativeToolPricingChanged ? (
    <Button
      type="button"
      size="sm"
      disabled={loading || nativeToolBillingSaving}
      onClick={() => void saveNativeToolBilling()}
    >
      {nativeToolBillingSaving ? <SpinnerLabel>{tActions("saving")}</SpinnerLabel> : (
        <>
          <Save className="size-3.5" />
          {tActions("save")}
        </>
      )}
    </Button>
  ) : null;

  const sectionTitle = (
    <span className="inline-flex items-center gap-1">
      {t("toolPricing.title")}
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            className="text-muted-foreground hover:bg-transparent hover:text-foreground"
            aria-label={t("toolPricing.nativeHelpLabel")}
          >
            <CircleHelp className="size-3.5" />
          </Button>
        </TooltipTrigger>
        <TooltipContent side="right" className="max-w-xs text-xs leading-5">
          <div className="space-y-1">
            <p>{t("toolPricing.defaultPriceDescription")}</p>
            <p>{t("toolPricing.note")}</p>
          </div>
        </TooltipContent>
      </Tooltip>
    </span>
  );

  return (
    <SettingsSection title={sectionTitle} actions={toolPricingActions} className="px-1">
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
      <CollapsibleMotionContent open={nativeToolBillingEnabled} contentClassName="space-y-3">
        <Table
          viewportRef={nativeToolVirtualRows.viewportRef}
          viewportClassName={nativeToolVirtualRows.viewportClassName}
          viewportStyle={nativeToolVirtualRows.viewportStyle}
        >
          <TableHeader>
            <TableRow>
              <TableHead>{t("toolPricing.provider")}</TableHead>
              <TableHead>{t("toolPricing.tool")}</TableHead>
              <TableHead>{t("toolPricing.type")}</TableHead>
              <TableHead className="text-right">{t("toolPricing.price")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {showNativeToolRows ? <VirtualTablePaddingRow colSpan={4} height={nativeToolVirtualRows.paddingTop} /> : null}
            {nativeToolVirtualRows.rows.map(({ item: row }) => {
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
                        onChange={(event) => updateNativeToolPriceDraft(row.toolKey, event.target.value)}
                      />
                      <span className="whitespace-nowrap text-muted-foreground">
                        / {t("toolPricing.units.call")}
                      </span>
                    </div>
                  </TableCell>
                </TableRow>
              );
            })}
            {showNativeToolRows ? <VirtualTablePaddingRow colSpan={4} height={nativeToolVirtualRows.paddingBottom} /> : null}
          </TableBody>
        </Table>
      </CollapsibleMotionContent>
    </SettingsSection>
  );
}
