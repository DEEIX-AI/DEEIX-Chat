"use client";

import { Clock, Layers, Pencil } from "lucide-react";
import { useTranslations } from "next-intl";
import type * as React from "react";

import { Button } from "@/components/ui/button";
import { HoverCard, HoverCardContent, HoverCardTrigger } from "@/components/ui/hover-card";
import {
  Table,
  TableBody,
  TableCell,
  TableEmptyRow,
  TableHead,
  TableHeader,
  TableLoadingRow,
  TableRow,
} from "@/components/ui/table";
import type { AdminBillingPlanDTO, AdminModelPricingDTO } from "@/features/admin/api/billing.types";
import { cn } from "@/lib/utils";
import { WEEKDAY_ORDER, formatRateMultiplier, parseSchedulePricing } from "@/shared/model/schedule-pricing";
import {
  formatAmountCents,
  parseTieredPricingJSON,
  formatCreditUSD,
  formatUSD,
  normalizePricingMode,
} from "@/features/admin/model/billing-settings";

export function PeriodBillingTable({
  plans,
  loading,
  onEdit,
}: {
  plans: AdminBillingPlanDTO[];
  loading: boolean;
  onEdit: (plan: AdminBillingPlanDTO) => void;
}) {
  const t = useTranslations("adminBilling");
  const initialLoading = loading && plans.length === 0;
  const showPlans = plans.length > 0;

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t("plans.tablePlan")}</TableHead>
          <TableHead>{t("plans.tableDescription")}</TableHead>
          <TableHead>{t("plans.tablePrice")}</TableHead>
          <TableHead>{t("plans.tableCredit")}</TableHead>
          <TableHead stickyEnd className="w-[56px]" />
        </TableRow>
      </TableHeader>
      <TableBody>
        {initialLoading ? <TableLoadingRow colSpan={5} /> : null}
        {!loading && plans.length === 0 ? <TableEmptyRow colSpan={5}>{t("plans.empty")}</TableEmptyRow> : null}
        {showPlans
          ? plans.map((plan) => {
              const defaultPrice = plan.prices.find((item) => item.isDefault) || plan.prices[0];
              return (
                <TableRow key={plan.id}>
                  <TableCell className="py-1.5">
                    <span className="font-medium text-foreground">{plan.name}</span>
                  </TableCell>
                  <TableCell className="max-w-[280px] py-1.5 text-muted-foreground">
                    <span className="block truncate" title={plan.description || "-"}>
                      {plan.description || "-"}
                    </span>
                  </TableCell>
                  <TableCell className="py-1.5">
                    {defaultPrice ? (
                      <span>
                        {formatAmountCents(defaultPrice.amountCents, defaultPrice.currency)} / {t(`plans.intervals.${defaultPrice.billingInterval}`)}
                      </span>
                    ) : (
                      <span className="text-muted-foreground">-</span>
                    )}
                  </TableCell>
                  <TableCell className="py-1.5">
                    <span>
                      {formatCreditUSD(plan.periodCreditUSD)}
                      <span className="ml-1 text-xs text-muted-foreground">{t("plans.perPeriod")}</span>
                    </span>
                  </TableCell>
                  <TableCell stickyEnd className="w-[56px] py-1.5 text-right">
                    <div className="flex h-7 items-center justify-end">
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon-xs"
                        className="h-7 w-7 text-muted-foreground shadow-none"
                        onClick={() => onEdit(plan)}
                        aria-label={t("actions.editPlan")}
                      >
                        <Pencil className="size-3.5 stroke-1" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              );
            })
          : null}
      </TableBody>
    </Table>
  );
}

// Prices per million tokens; zero is dimmed so it does not compete with real figures.
function PriceValue({ value, suffix }: { value: number; suffix?: string }) {
  const zero = !Number.isFinite(value) || value <= 0;
  return (
    <span className={cn("tabular-nums", zero ? "text-muted-foreground/60" : "text-foreground")}>
      {formatUSD(value)}
      {suffix ? <span className="ml-1 text-muted-foreground">{suffix}</span> : null}
    </span>
  );
}

export const PRICE_COLUMN_COUNT = 4;

// Tier boundaries as "≤ 128K" / "128K – 256K" / "> 256K". Token counts use the
// K/M convention in every locale (zh-CN compact would give 12.8万), so the
// formatter is pinned to en-US.
export function formatTierRange(fromTokens: number, upToTokens: number): string {
  const compact = new Intl.NumberFormat("en-US", { notation: "compact", maximumFractionDigits: 1 });
  if (fromTokens === 0 && upToTokens > 0) {
    return `≤ ${compact.format(upToTokens)}`;
  }
  if (upToTokens === 0) {
    return `> ${compact.format(fromTokens)}`;
  }
  return `${compact.format(fromTokens)} – ${compact.format(upToTokens)}`;
}

// Secondary content for the mode column: the per-call / per-second figure, or
// one range line per tier so the stacked tier prices on the right read as rows.
export function PricingModeDetail({ pricing }: { pricing: AdminModelPricingDTO }) {
  const t = useTranslations("adminBilling");
  const mode = normalizePricingMode(pricing.pricingMode);
  if (mode === "call") {
    return <PriceValue value={pricing.callUSDPerCall} suffix={t("modelPricing.units.call")} />;
  }
  if (mode === "duration") {
    return <PriceValue value={pricing.durationUSDPerSecond} suffix={t("modelPricing.units.second")} />;
  }
  return null;
}

const WEEKDAY_KEYS = ["sun", "mon", "tue", "wed", "thu", "fri", "sat"] as const;

// Weekday sets read as a range when contiguous in display order (Mon–Fri),
// otherwise as a list (Sat, Sun).
function formatWeekdays(weekdays: number[], label: (key: (typeof WEEKDAY_KEYS)[number]) => string): string {
  const order: readonly number[] = WEEKDAY_ORDER;
  const ordered = order.filter((day) => weekdays.includes(day));
  const first = ordered[0];
  const last = ordered[ordered.length - 1];
  if (first === undefined || last === undefined) {
    return "";
  }
  const key = (day: number) => WEEKDAY_KEYS[day] ?? "sun";
  const contiguous = ordered.every((day, index) => index === 0 || order.indexOf(day) === order.indexOf(ordered[index - 1] as number) + 1);
  if (contiguous && ordered.length > 2) {
    return `${label(key(first))}–${label(key(last))}`;
  }
  return ordered.map((day) => label(key(day))).join(" ");
}

// Detail surfaces open on hover as a light card, not the inverted tooltip:
// a small table needs the same contrast as the page.
function DetailCard({ trigger, title, children, align }: { trigger: React.ReactNode; title: string; children: React.ReactNode; align: "start" | "end" }) {
  return (
    <HoverCard openDelay={150} closeDelay={80}>
      <HoverCardTrigger asChild>{trigger}</HoverCardTrigger>
      <HoverCardContent side="bottom" align={align} sideOffset={6} className="w-auto min-w-56 p-0">
        <p className="border-b-[0.5px] border-border px-3 py-2 text-[11px] font-medium text-muted-foreground">{title}</p>
        <div className="px-3 py-2">{children}</div>
      </HoverCardContent>
    </HoverCard>
  );
}

const DETAIL_TABLE = "border-collapse text-xs leading-6";
const DETAIL_HEAD = "pb-1 text-[11px] font-normal text-muted-foreground";

// Schedule badge: the count inline, every period on hover.
export function SchedulePricingBadge({ pricing }: { pricing: AdminModelPricingDTO }) {
  const t = useTranslations("adminBilling");
  const periods = parseSchedulePricing(pricing.schedulePricingJSON) ?? [];
  if (periods.length === 0) {
    return null;
  }
  const weekday = (key: (typeof WEEKDAY_KEYS)[number]) => t(`modelPricing.schedule.weekdays.${key}`);
  return (
    <DetailCard
      align="start"
      title={t("modelPricing.schedule.title")}
      trigger={
        <span className="inline-flex cursor-default items-center gap-0.5 rounded-sm bg-muted px-1 text-[10px] text-muted-foreground">
          <Clock className="size-3" strokeWidth={1.8} />
          {periods.length}
        </span>
      }
    >
      <table className={DETAIL_TABLE}>
        <thead>
          <tr>
            <th className={cn(DETAIL_HEAD, "pr-4 text-left")}>{t("modelPricing.schedule.name")}</th>
            <th className={cn(DETAIL_HEAD, "pr-4 text-left")}>{t("modelPricing.schedule.weekdaysLabel")}</th>
            <th className={cn(DETAIL_HEAD, "pr-4 text-left")}>{t("modelPricing.schedule.timeLabel")}</th>
            <th className={cn(DETAIL_HEAD, "text-right")}>{t("modelPricing.schedule.ratePercent")}</th>
          </tr>
        </thead>
        <tbody>
          {periods.map((period, index) => (
            <tr key={`${period.name}-${index}`}>
              <td className="whitespace-nowrap pr-4 font-medium">{period.name}</td>
              <td className="whitespace-nowrap pr-4 text-muted-foreground">{formatWeekdays(period.weekdays, weekday)}</td>
              <td className="whitespace-nowrap pr-4 tabular-nums text-muted-foreground">
                {period.start} – {period.end}
                {period.end <= period.start ? <span className="ml-1 text-[10px]">{t("modelPricing.schedule.nextDay")}</span> : null}
              </td>
              <td className="text-right tabular-nums">{formatRateMultiplier(period.ratePercent)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </DetailCard>
  );
}

// Tiered pricing keeps the row to one line: a tier count in the price span,
// with the full tier table on hover.
function TieredPricingSummary({ pricing }: { pricing: AdminModelPricingDTO }) {
  const t = useTranslations("adminBilling");
  const tiers = parseTieredPricingJSON(pricing.tieredPricingJSON) ?? [];
  const columns = [
    { key: "input", label: t("modelPricing.priceInput") },
    { key: "output", label: t("modelPricing.priceOutput") },
    { key: "cacheRead", label: t("modelPricing.priceCacheRead") },
    { key: "cacheWrite", label: t("modelPricing.priceCacheWrite") },
  ] as const;
  return (
    <DetailCard
      align="end"
      title={t("modelPricing.tieredTitle")}
      trigger={
        <span className="inline-flex cursor-default items-center gap-1 text-muted-foreground">
          <Layers className="size-3" strokeWidth={1.8} />
          {t("modelPricing.tierCount", { count: tiers.length })}
        </span>
      }
    >
      <table className={DETAIL_TABLE}>
        <thead>
          <tr>
            <th className={cn(DETAIL_HEAD, "pr-4 text-left")}>{t("modelPricing.tierRange")}</th>
            {columns.map((column) => (
              <th key={column.key} className={cn(DETAIL_HEAD, "pl-4 text-right")}>
                {column.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {tiers.map((tier, index) => (
            <tr key={tier.id}>
              <td className="whitespace-nowrap pr-4 tabular-nums text-muted-foreground">{formatTierRange(Number(tiers[index - 1]?.upToTokens ?? 0), Number(tier.upToTokens))}</td>
              {columns.map((column) => (
                <td key={column.key} className="whitespace-nowrap pl-4 text-right tabular-nums">
                  {formatUSD(Number(tier[column.key]))}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </DetailCard>
  );
}

// The four token-price columns of the pricing table. They only ever hold
// numbers; other modes describe their price in the mode column and leave
// these cells empty so the right-aligned figures never mix with text.
export function PricingColumns({ pricing, cellClassName }: { pricing: AdminModelPricingDTO | null; cellClassName?: string }) {
  const mode = pricing ? normalizePricingMode(pricing.pricingMode) : null;
  const numeric = cn("whitespace-nowrap text-right text-xs", cellClassName);
  if (pricing && mode === "tiered") {
    return (
      <TableCell colSpan={PRICE_COLUMN_COUNT} className={numeric}>
        <TieredPricingSummary pricing={pricing} />
      </TableCell>
    );
  }
  if (!pricing || mode !== "token") {
    return <TableCell colSpan={PRICE_COLUMN_COUNT} className={cellClassName} />;
  }
  return (
    <>
      <TableCell className={numeric}>
        <PriceValue value={pricing.inputUSDPerMTokens} />
      </TableCell>
      <TableCell className={numeric}>
        <PriceValue value={pricing.outputUSDPerMTokens} />
      </TableCell>
      <TableCell className={numeric}>
        <PriceValue value={pricing.cacheReadUSDPerMTokens} />
      </TableCell>
      <TableCell className={numeric}>
        <PriceValue value={pricing.cacheWriteUSDPerMTokens} />
      </TableCell>
    </>
  );
}
