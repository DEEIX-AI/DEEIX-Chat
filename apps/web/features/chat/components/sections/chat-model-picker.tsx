"use client";

import { AnimatePresence, useReducedMotion } from "motion/react";
import * as React from "react";
import { ArrowRight, AudioLines, Check, ChevronDown, ChevronLeft, ChevronRight, CircleDollarSign, FileText, Image as ImageIcon, KeyRound, TicketSlash, Type, Video } from "lucide-react";
import { useTranslations } from "next-intl";

import { Badge } from "@/components/ui/badge";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { InputGroupButton } from "@/components/ui/input-group";
import type { ChatModelOption, ModelModality } from "@/features/chat/types/chat-runtime";
import { useChatPopoverAlignOffset } from "@/features/chat/hooks/use-chat-popover-align-offset";
import {
  resolveDesktopMenuListMaxHeight,
  resolveDesktopModelMenuListMaxHeight,
  createPopoverLayoutRectReader,
  layoutOffsetTop,
} from "./chat-model-picker-layout";
import { MORPH_DURATION_S, MorphingCard, useSwitchDirection } from "./chat-model-picker-motion";
import { useIsMobile } from "@/shared/hooks/use-mobile";
import { SlideSwitch } from "@/shared/components/slide-switch";
import {
  ModelIcon,
  resolveModelIconURL,
  resolveModelOptionIconUrl,
  resolveModelOptionLabel,
  resolveModelPresentationGroup,
} from "@/entities/model";
import {
  type BillingDisplayCurrency,
  type BillingDisplayLabels,
  type BillingDisplayOptions,
  cacheWritePricingLabel,
  cacheWritePricingNote,
  formatBillingDisplayUnitPriceFromUSD,
  formatRateMultiplier,
  resolveCacheWritePricingUSD,
  resolveCurrentSchedulePeriod,
} from "@/entities/billing";
import { cn } from "@/lib/utils";

type ChatModelPickerProps = {
  modelOptions: ChatModelOption[];
  billingDisplayCurrency: BillingDisplayCurrency;
  billingDisplayUsdToCnyRate: number | null;
  /** False in self-hosted mode, where no model shows a price. */
  billingEnabled: boolean;
  selectedPlatformModelName: string;
  loading: boolean;
  disabled: boolean;
  onModelCatalogRefresh?: () => void | Promise<void>;
  onModelChange: (platformModelName: string) => void;
  /** Side the phone menu opens on: below the composer on the landing page, above it in a conversation. */
  placementPreference: "top" | "bottom";
};

const MODEL_MENU_COLLISION_PADDING = 24;
const DESKTOP_MODEL_MENU_WIDTH = 224;
const DESKTOP_MODEL_SUBMENU_GAP = 8;
const DESKTOP_MODEL_MENU_SIDE_OFFSET = 8;
const DESKTOP_MODEL_MENU_MIN_SCROLL_HEIGHT = 96;
const DESKTOP_DETAIL_PANE_WIDTH = 256;
const DESKTOP_DETAIL_PANE_MIN_WIDTH = 200;
const DESKTOP_DETAIL_PANE_MIN_HEIGHT = 120;
/** Gap to the submenu; must match the 0.5rem offsets in its placement classes. */
const DESKTOP_DETAIL_PANE_GAP = 8;
/** Group panel non-list chrome: p-1.5 × 2 + header h-7. */
const DESKTOP_GROUP_MENU_VERTICAL_CHROME = 40;
/** Model submenu non-list chrome: p-1.5 × 2. */
const DESKTOP_SUBMENU_VERTICAL_CHROME = 12;

/** Models on the user's own key are grouped by provider, after the platform groups. */
function resolveModelGroupPresentation(item: ChatModelOption) {
  if (item.personalProviderName !== null) {
    return { key: `personal:${item.personalProviderName}`, label: item.personalProviderName, icon: item.personalProviderIcon };
  }
  return resolveModelPresentationGroup(item);
}

function resolveModelGroups(modelOptions: ChatModelOption[]) {
  const groupMap = new Map<string, { label: string; icon: string; personal: boolean; items: ChatModelOption[] }>();
  for (const item of modelOptions) {
    const presentation = resolveModelGroupPresentation(item);
    const group = groupMap.get(presentation.key);
    if (group) {
      group.items.push(item);
      continue;
    }
    groupMap.set(presentation.key, {
      label: presentation.label,
      icon: presentation.icon,
      personal: item.personalProviderName !== null,
      items: [item],
    });
  }

  return Array.from(groupMap.entries()).map(([key, group]) => ({ key, ...group }));
}

/** Fallback mark for a group served by the user's own key when it has no icon. */
function PersonalGroupIcon() {
  return (
    <span className="flex size-4 shrink-0 items-center justify-center text-muted-foreground" aria-hidden="true">
      <KeyRound className="size-3.5" strokeWidth={1.7} />
    </span>
  );
}

/** Tells a user-key group apart from a platform group of the same vendor. */
function PersonalGroupBadge({ label }: { label: string }) {
  return (
    <Badge variant="secondary" className="h-4 shrink-0 px-1 font-normal">
      {label}
    </Badge>
  );
}

function ChatModelIdentity({ model }: { model: ChatModelOption }) {
  const platformModelName = resolveModelOptionLabel(model.platformModelName);
  const iconURL = React.useMemo(() => resolveModelOptionIconUrl(model), [model]);

  return (
    <div className="flex min-w-0 items-center gap-2">
      <ModelIcon iconUrl={iconURL} label={platformModelName} />
      <p className="min-w-0 flex-1 truncate text-[12.5px] font-medium leading-4 text-foreground">{platformModelName}</p>
    </div>
  );
}

function ChatModelTriggerSkeleton() {
  return (
    <div className="flex min-w-0 items-center gap-2.5">
      <Skeleton className="size-4 shrink-0 rounded-full bg-muted/55" />
      <Skeleton className="h-3.5 w-20 rounded-full bg-muted/50" />
    </div>
  );
}

function ModelMenuScrollContainer({
  children,
  maxHeight,
}: {
  children: React.ReactNode;
  maxHeight?: number;
}) {
  const viewportRef = React.useRef<HTMLDivElement | null>(null);
  const [hasMoreAbove, setHasMoreAbove] = React.useState(false);
  const [hasMoreBelow, setHasMoreBelow] = React.useState(false);
  const resolvedMaxHeight = Number.isFinite(maxHeight) ? Math.max(0, maxHeight ?? 0) : undefined;

  const updateScrollHints = React.useCallback(() => {
    const viewport = viewportRef.current;
    if (!viewport) {
      setHasMoreAbove(false);
      setHasMoreBelow(false);
      return;
    }
    const remaining = viewport.scrollHeight - viewport.clientHeight - viewport.scrollTop;
    setHasMoreAbove(viewport.scrollTop > 1);
    setHasMoreBelow(remaining > 1);
  }, []);

  React.useLayoutEffect(() => {
    updateScrollHints();
    const viewport = viewportRef.current;
    if (!viewport || typeof ResizeObserver === "undefined") {
      return;
    }

    const observer = new ResizeObserver(updateScrollHints);
    observer.observe(viewport);
    if (viewport.firstElementChild) {
      observer.observe(viewport.firstElementChild);
    }
    return () => observer.disconnect();
  }, [children, resolvedMaxHeight, updateScrollHints]);

  return (
    <div className="relative">
      <div
        ref={viewportRef}
        data-model-menu-viewport=""
        style={resolvedMaxHeight === undefined ? undefined : { maxHeight: resolvedMaxHeight }}
        className={cn(
          "overflow-y-auto overscroll-contain pr-0 [-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden",
          resolvedMaxHeight === undefined
            ? "max-h-[min(20rem,var(--model-menu-scroll-max-height,var(--radix-popover-content-available-height)))]"
            : null,
        )}
        onScroll={updateScrollHints}
      >
        {children}
      </div>
      {hasMoreAbove ? (
        <div className="pointer-events-none absolute inset-x-0 top-0 flex h-4 items-start justify-center rounded-t-lg bg-gradient-to-b from-popover via-popover/80 to-transparent pt-px">
          <ChevronDown className="size-3 rotate-180 text-muted-foreground/75" strokeWidth={1.8} />
        </div>
      ) : null}
      {hasMoreBelow ? (
        <div className="pointer-events-none absolute inset-x-0 bottom-0 flex h-4 items-end justify-center rounded-b-lg bg-gradient-to-t from-popover via-popover/80 to-transparent pb-px">
          <ChevronDown className="size-3 text-muted-foreground/75" strokeWidth={1.8} />
        </div>
      ) : null}
    </div>
  );
}

function ModelPricingTooltipContent(props: ModelPricingTooltipContentProps) {
  const { pricing, labels } = props;
  const current = React.useMemo(
    () => resolveCurrentSchedulePeriod(pricing.schedulePeriods ?? [], pricing.scheduleUTCOffsetMinutes ?? 0),
    [pricing.schedulePeriods, pricing.scheduleUTCOffsetMinutes],
  );
  const periods = pricing.schedulePeriods ?? [];
  return (
    <>
      <ModelPricingBase {...props} />
      {!pricing.isFree && periods.length > 0 ? (
        <div className="mt-2 flex flex-col gap-1 border-t border-background/20 pt-2">
          <span className="font-sans text-xs font-medium leading-4 text-background">{labels.schedule.title}</span>
          {periods.map((period) => {
            const active = current === period;
            return (
              <div key={`${period.name}-${period.start}-${period.end}`} className={cn("grid grid-cols-[minmax(5.5rem,max-content)_1fr_auto] items-baseline gap-x-4 font-sans text-[11px] leading-4", active ? "text-background" : "text-background/70")}>
                <span className="whitespace-nowrap">
                  {period.name}
                  {active ? <span className="ml-1 rounded-sm bg-background/20 px-1 text-[10px]">{labels.schedule.now}</span> : null}
                </span>
                <span className="whitespace-nowrap text-right tabular-nums">
                  {formatScheduleWeekdays(period.weekdays, labels.schedule)}{" "}
                  {labels.schedule.timeRange(period.start, period.end)}
                </span>
                <span className="w-12 whitespace-nowrap text-right tabular-nums">{formatRateMultiplier(period.ratePercent)}</span>
              </div>
            );
          })}
        </div>
      ) : null}
    </>
  );
}

type ModelPricingTooltipContentProps = {
  platformModelName: string;
  protocols: readonly string[];
  pricing: NonNullable<ChatModelOption["pricing"]>;
  billingDisplay: BillingDisplayOptions;
  labels: {
    freeModel: string;
    freeModelDescription: string;
    tieredPricing: string;
    callPricing: string;
    durationPricing: string;
    tokenPricing: string;
    input: string;
    output: string;
    cacheRead: string;
    perCall: string;
    perSecond: string;
    callUnit: string;
    secondUnit: string;
    billingDisplay: BillingDisplayLabels;
    schedule: {
      title: string;
      now: string;
      weekdays: readonly string[];
      /** Joins the ends of a weekday range: "周一至周五" (Chinese text avoids the hyphen). */
      dayRange: (from: string, to: string) => string;
      /** Separates weekdays and ranges in a list: "周一至周三、周六", "Mon–Wed, Sat". */
      daySeparator: string;
      /** Joins the ends of a time range: "13:00-18:00". */
      timeRange: (from: string, to: string) => string;
    };
  };
};

type ScheduleLabels = ModelPricingTooltipContentProps["labels"]["schedule"];

// Week-day list → "周一至周五", "周六、周日", "每天"; ranges only for runs of 3+ days.
function formatScheduleWeekdays(
  weekdays: readonly number[],
  { weekdays: names, dayRange: range, daySeparator }: Pick<ScheduleLabels, "weekdays" | "dayRange" | "daySeparator">,
): string {
  const ordered = [1, 2, 3, 4, 5, 6, 0].filter((day) => weekdays.includes(day));
  if (ordered.length === 7) {
    return names[7] ?? "";
  }
  const parts: string[] = [];
  let index = 0;
  while (index < ordered.length) {
    let end = index;
    while (end + 1 < ordered.length && ((ordered[end] ?? 0) + 1) % 7 === ordered[end + 1]) {
      end += 1;
    }
    if (end - index >= 2) {
      parts.push(range(names[ordered[index] ?? 0] ?? "", names[ordered[end] ?? 0] ?? ""));
    } else {
      for (let cursor = index; cursor <= end; cursor += 1) {
        parts.push(names[ordered[cursor] ?? 0] ?? "");
      }
    }
    index = end + 1;
  }
  return parts.join(daySeparator);
}

function ModelPricingBase({ platformModelName, protocols, pricing, billingDisplay, labels }: ModelPricingTooltipContentProps) {
  const cacheWriteLabel = cacheWritePricingLabel(protocols, labels.billingDisplay);
  const cacheWriteNote = cacheWritePricingNote(protocols, pricing, labels.billingDisplay);
  if (pricing.isFree) {
    return (
      <div className="flex flex-col gap-1">
        <span className="font-sans text-xs font-medium leading-4 text-background">{labels.freeModel}</span>
        <span className="font-sans text-[11px] leading-4 text-background/80">{labels.freeModelDescription}</span>
      </div>
    );
  }

  if (pricing.mode === "tiered") {
    return (
      <PricingTable
        platformModelName={platformModelName}
        title={labels.tieredPricing}
        footerNote={cacheWriteNote}
        headerRow={["", ...pricing.tiers.map((tier) => formatTierRange(tier.fromTokens, tier.upToTokens))]}
        bodyRows={[
          [labels.input, ...pricing.tiers.map((tier) => formatPricingUnitUSD(tier.inputUSDPerMTokens, billingDisplay))],
          [labels.output, ...pricing.tiers.map((tier) => formatPricingUnitUSD(tier.outputUSDPerMTokens, billingDisplay))],
          [labels.cacheRead, ...pricing.tiers.map((tier) => formatPricingUnitUSD(tier.cacheReadUSDPerMTokens, billingDisplay))],
          [cacheWriteLabel, ...pricing.tiers.map((tier) => formatPricingUnitUSD(resolveCacheWritePricingUSD(protocols, tier.cacheWriteUSDPerMTokens, pricing), billingDisplay))],
        ]}
      />
    );
  }

  if (pricing.mode === "call") {
    return (
      <div className="flex flex-col gap-1">
        <span className="font-sans text-xs font-medium leading-4 text-background">{labels.callPricing}</span>
        <PricingTooltipRow label={labels.perCall} value={`${formatPricingUnitUSD(pricing.callUSDPerCall, billingDisplay)} / ${labels.callUnit}`} />
      </div>
    );
  }

  if (pricing.mode === "duration") {
    return (
      <div className="flex flex-col gap-1">
        <span className="font-sans text-xs font-medium leading-4 text-background">{labels.durationPricing}</span>
        <PricingTooltipRow label={labels.perSecond} value={`${formatPricingUnitUSD(pricing.durationUSDPerSecond, billingDisplay)} / ${labels.secondUnit}`} />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-1">
      <span className="font-sans text-xs font-medium leading-4 text-background">{labels.tokenPricing}</span>
      <PricingTooltipRow label={labels.input} value={`${formatPricingUnitUSD(pricing.inputUSDPerMTokens, billingDisplay)} / 1M tokens`} />
      <PricingTooltipRow label={labels.output} value={`${formatPricingUnitUSD(pricing.outputUSDPerMTokens, billingDisplay)} / 1M tokens`} />
      <PricingTooltipRow label={labels.cacheRead} value={`${formatPricingUnitUSD(pricing.cacheReadUSDPerMTokens, billingDisplay)} / 1M tokens`} />
      <PricingTooltipRow label={cacheWriteLabel} value={`${formatPricingUnitUSD(resolveCacheWritePricingUSD(protocols, pricing.cacheWriteUSDPerMTokens, pricing), billingDisplay)} / 1M tokens`} />
      {cacheWriteNote ? <span className="block max-w-72 font-sans text-[11px] leading-4 text-background/70">{cacheWriteNote}</span> : null}
    </div>
  );
}

function PricingTooltipRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid grid-cols-[minmax(5.5rem,max-content)_auto] items-baseline gap-5 font-sans text-[11px] leading-4 text-background/80">
      <span className="whitespace-nowrap text-left">{label}</span>
      <span className="whitespace-nowrap text-right tabular-nums">{value}</span>
    </div>
  );
}

function PricingTable({
  platformModelName,
  title,
  footerNote,
  headerRow,
  bodyRows,
}: {
  platformModelName: string;
  title: string;
  footerNote?: string | null;
  headerRow: string[];
  bodyRows: string[][];
}) {
  return (
    <div className="flex max-w-[560px] flex-col gap-2 overflow-x-auto">
      <span className="font-sans text-xs font-medium leading-4 text-background">{title}</span>
      <table className="border-collapse text-left font-sans text-[11px] leading-4 text-background/80 tabular-nums">
        <thead>
          <tr className="border-b border-background/20">
            {headerRow.map((cell, index) => (
              <th
                key={`${platformModelName}-pricing-head-${index}`}
                scope="col"
                className={cn(
                  "whitespace-nowrap px-2 pb-1 font-medium text-background/70 first:pl-0 last:pr-0",
                  index > 0 ? "text-right" : null,
                )}
              >
                {cell}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {bodyRows.map((row, rowIndex) => (
            <tr key={`${platformModelName}-pricing-row-${rowIndex}`} className="border-b border-background/10 last:border-0">
              {row.map((cell, cellIndex) => (
                <td
                  key={`${platformModelName}-pricing-cell-${rowIndex}-${cellIndex}`}
                  className={cn(
                    "whitespace-nowrap px-2 py-1 first:pl-0 last:pr-0",
                    cellIndex === 0 ? "font-medium text-background/90" : "text-right",
                  )}
                >
                  {cell}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      {footerNote ? <span className="block font-sans text-[11px] leading-4 text-background/70">{footerNote}</span> : null}
    </div>
  );
}

function formatPricingUnitUSD(value: number, billingDisplay: BillingDisplayOptions): string {
  return formatBillingDisplayUnitPriceFromUSD(value, billingDisplay);
}

// "≤ 200K", "200K – 1M", "> 1M": the token range a pricing tier applies to.
function formatTierRange(fromTokens: number, upToTokens: number | null): string {
  if (!upToTokens || upToTokens <= 0) {
    return `> ${formatTokenQuantity(fromTokens)}`;
  }
  if (fromTokens <= 0) {
    return `≤ ${formatTokenQuantity(upToTokens)}`;
  }
  return `${formatTokenQuantity(fromTokens)} – ${formatTokenQuantity(upToTokens)}`;
}

function formatTokenQuantity(value: number): string {
  if (!Number.isFinite(value) || value <= 0) {
    return "0";
  }
  if (value >= 1000000 && value % 1000000 === 0) {
    return `${value / 1000000}M`;
  }
  if (value >= 1000 && value % 1000 === 0) {
    return `${value / 1000}K`;
  }
  return String(value);
}

const MODALITY_ICONS: Record<ModelModality, React.ComponentType<{ className?: string; strokeWidth?: number }>> = {
  text: Type,
  image: ImageIcon,
  pdf: FileText,
  audio: AudioLines,
  video: Video,
};

// Text is implied for every chat model, so only the other modalities are worth pointing out.
function mediaModalities(modalities: readonly ModelModality[]): ModelModality[] {
  return modalities.filter((modality) => modality !== "text");
}

type InputModalityLabels = {
  title: string;
  separator: string;
  names: Record<ModelModality, string>;
};

// "Input: image, video" style label; null when the model accepts nothing beyond text.
function describeInputModalities(modalities: ModelModality[], labels: InputModalityLabels): string | null {
  const visible = mediaModalities(modalities);
  if (visible.length === 0) {
    return null;
  }
  return `${labels.title}${visible.map((modality) => labels.names[modality]).join(labels.separator)}`;
}

// The row's own modality icons, for layouts without the detail pane. The description doubles as the
// tooltip and the accessible name.
function InputModalityIcons({
  modalities,
  labels,
}: {
  modalities: ModelModality[];
  labels: InputModalityLabels;
}) {
  const description = describeInputModalities(modalities, labels);
  if (!description) {
    return null;
  }
  return (
    <span
      className="flex shrink-0 items-center gap-1 px-1 text-muted-foreground/60 group-hover:text-current group-data-[selected=true]:text-current"
      role="img"
      aria-label={description}
      title={description}
    >
      {mediaModalities(modalities).map((modality) => {
        const Icon = MODALITY_ICONS[modality];
        return <Icon key={modality} className="size-3" strokeWidth={1.8} />;
      })}
    </span>
  );
}

function ChatModelMenuItem({
  model,
  selected,
  onSelect,
  billingDisplay,
  pricingLabels,
  inputModalityLabels,
  viewPricingLabel,
  freeBadgeLabel,
  buttonRef,
  describedBy,
  withDetailPane = false,
  reservePricingSlot = false,
  onHover,
  rowRef,
}: {
  model: ChatModelOption;
  selected: boolean;
  onSelect: () => void;
  billingDisplay: BillingDisplayOptions;
  pricingLabels: React.ComponentProps<typeof ModelPricingTooltipContent>["labels"];
  inputModalityLabels: InputModalityLabels;
  viewPricingLabel: string;
  /** Badge for free models, which the pane only mentions in its price line. */
  freeBadgeLabel: string;
  buttonRef?: React.Ref<HTMLButtonElement>;
  /** Id of the detail pane while it describes this row, so screen readers announce it too. */
  describedBy?: string;
  /**
   * True when the detail pane describes the hovered model, so the row keeps only its icon, name and
   * selection mark. Without the pane the row also carries the modality icons and the pricing tooltip.
   */
  withDetailPane?: boolean;
  /** Keep the pricing slot without pricing, so the selection mark stays in one column. */
  reservePricingSlot?: boolean;
  /** Reports the row the pointer or keyboard focus is on, which drives the detail pane. */
  onHover?: () => void;
  /** The row element, so the detail pane can be placed level with the row it describes. */
  rowRef?: (node: HTMLDivElement | null) => void;
}) {
  const platformModelName = resolveModelOptionLabel(model.platformModelName);
  const iconURL = React.useMemo(() => resolveModelOptionIconUrl(model), [model]);
  const inputModalityDescription = describeInputModalities(model.inputModalities, inputModalityLabels);

  return (
    <div
      data-selected={selected}
      ref={rowRef}
      className="group flex h-7 items-center rounded-md text-[11px] font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground focus-within:bg-accent focus-within:text-accent-foreground data-[selected=true]:bg-accent data-[selected=true]:text-accent-foreground"
      onMouseEnter={onHover}
      // Keyboard navigation opens the detail too; pointer focus (a click) does not.
      onFocus={(event) => {
        if (event.target instanceof HTMLElement && event.target.matches(":focus-visible")) {
          onHover?.();
        }
      }}
    >
      <button
        ref={buttonRef}
        type="button"
        aria-describedby={describedBy}
        className="flex h-7 min-w-0 flex-1 items-center gap-2 rounded-md bg-transparent py-0 pl-2 pr-1 text-left text-[11px] font-medium leading-none text-inherit outline-none"
        onClick={onSelect}
      >
        <ModelIcon iconUrl={iconURL} label={platformModelName} />
        <span className="min-w-0 flex-1 truncate leading-4">{platformModelName}</span>
        {inputModalityDescription && withDetailPane ? <span className="sr-only">{inputModalityDescription}</span> : null}
        {/* One trailing mark, flush right: the selection, else the free badge (pane layout only). */}
        {selected ? (
          <span className="flex size-3 shrink-0 items-center justify-center">
            <Check className="size-3 text-current" strokeWidth={1.7} />
          </span>
        ) : withDetailPane && model.pricing?.isFree ? (
          <Badge variant="secondary" className="h-4 shrink-0 px-1 font-normal">
            {freeBadgeLabel}
          </Badge>
        ) : null}
      </button>
      {withDetailPane ? null : (
        <>
          <InputModalityIcons modalities={model.inputModalities} labels={inputModalityLabels} />
          {!model.pricing && reservePricingSlot ? <span aria-hidden="true" className="h-7 w-7 shrink-0" /> : null}
          {model.pricing ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <button
                  type="button"
                  className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-muted-foreground/70 transition-colors hover:text-current focus-visible:text-current focus-visible:outline-none group-hover:text-current group-focus-within:text-current group-data-[selected=true]:text-current"
                  aria-label={viewPricingLabel}
                >
                  {model.pricing.isFree ? (
                    <TicketSlash className="size-3.5" strokeWidth={1.8} />
                  ) : (
                    <CircleDollarSign className="size-3.5" strokeWidth={1.8} />
                  )}
                </button>
              </TooltipTrigger>
              <TooltipContent
                side="right"
                align="center"
                sideOffset={8}
                className="z-[80] max-w-[min(92vw,35rem)] text-left font-medium tabular-nums"
              >
                <ModelPricingTooltipContent
                  platformModelName={model.platformModelName}
                  protocols={model.protocols}
                  pricing={model.pricing}
                  billingDisplay={billingDisplay}
                  labels={pricingLabels}
                />
              </TooltipContent>
            </Tooltip>
          ) : null}
        </>
      )}
    </div>
  );
}

// 128000 → "128K", 65536 → "64K": catalogs mix decimal and binary windows, so a window that is a
// round decimal stays decimal and 1024 is used only for the windows that are not.
function formatContextWindow(tokens: number): string {
  const binary = tokens % 1_000 !== 0 && tokens % 1_024 === 0;
  const kilo = binary ? 1_024 : 1_000;
  const mega = kilo * kilo;
  if (tokens >= mega) {
    return `${Number((tokens / mega).toFixed(1))}M`;
  }
  return `${Math.round(tokens / kilo)}K`;
}

type DetailPriceTable = {
  /** One heading per tier; empty for a single price column. */
  tiers: string[];
  rows: { label: string; values: string[] }[];
};

// Every charge as one table: a column per tier (or one column), a row per charge. Cache rows that
// are zero in every tier are dropped, and the cache-write row uses the 5m price the note explains.
function resolveDetailPriceTable(
  pricing: NonNullable<ChatModelOption["pricing"]>,
  protocols: readonly string[],
  billingDisplay: BillingDisplayOptions,
  labels: React.ComponentProps<typeof ModelPricingTooltipContent>["labels"],
): DetailPriceTable {
  const price = (value: number) => formatPricingUnitUSD(value, billingDisplay);
  if (pricing.mode === "call") {
    return { tiers: [], rows: [{ label: labels.perCall, values: [price(pricing.callUSDPerCall)] }] };
  }
  if (pricing.mode === "duration") {
    return { tiers: [], rows: [{ label: labels.perSecond, values: [price(pricing.durationUSDPerSecond)] }] };
  }
  const tiered = pricing.mode === "tiered" && pricing.tiers.length > 0;
  const columns = tiered ? pricing.tiers : [pricing];
  const rows: DetailPriceTable["rows"] = [
    { label: labels.input, values: columns.map((tier) => price(tier.inputUSDPerMTokens)) },
    { label: labels.output, values: columns.map((tier) => price(tier.outputUSDPerMTokens)) },
  ];
  if (columns.some((tier) => tier.cacheReadUSDPerMTokens > 0)) {
    rows.push({ label: labels.cacheRead, values: columns.map((tier) => price(tier.cacheReadUSDPerMTokens)) });
  }
  if (columns.some((tier) => tier.cacheWriteUSDPerMTokens > 0)) {
    rows.push({
      label: cacheWritePricingLabel(protocols, labels.billingDisplay),
      values: columns.map((tier) => price(resolveCacheWritePricingUSD(protocols, tier.cacheWriteUSDPerMTokens, pricing))),
    });
  }
  return {
    tiers: tiered ? pricing.tiers.map((tier) => formatTierRange(tier.fromTokens, tier.upToTokens)) : [],
    rows,
  };
}

// One line of the detail pane, the same height as a menu row: muted label left, value right.
function ModelDetailLine({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex h-7 items-center justify-between gap-4 px-2">
      <dt className="shrink-0 text-muted-foreground">{label}</dt>
      <dd className="flex min-w-0 items-center justify-end gap-1.5 truncate tabular-nums text-foreground">{children}</dd>
    </div>
  );
}

// Modalities as plain icons; each keeps its name for hover and screen readers.
function ModalityIcons({ modalities, labels }: { modalities: ModelModality[]; labels: InputModalityLabels }) {
  return (
    <span className="flex items-center gap-1.5" role="list">
      {modalities.map((modality) => {
        const Icon = MODALITY_ICONS[modality];
        const name = labels.names[modality];
        return (
          <span key={modality} role="listitem" title={name} className="inline-flex">
            <Icon className="size-3.5" strokeWidth={1.6} aria-hidden="true" />
            <span className="sr-only">{name}</span>
          </span>
        );
      })}
    </span>
  );
}

// Third level of the picker, shown while a model row is hovered: input and output modalities,
// context window and price, in the menu's own rhythm.
function ChatModelDetailPanel({
  model,
  inputModalityLabels,
  pricingLabels,
  billingDisplay,
  billingEnabled,
}: {
  model: ChatModelOption;
  inputModalityLabels: InputModalityLabels;
  pricingLabels: React.ComponentProps<typeof ModelPricingTooltipContent>["labels"];
  billingDisplay: BillingDisplayOptions;
  billingEnabled: boolean;
}) {
  const t = useTranslations("chat.modelPicker.detail");
  const pricing = model.pricing;
  const paid = pricing && !pricing.isFree ? pricing : null;
  const table = paid ? resolveDetailPriceTable(paid, model.protocols, billingDisplay, pricingLabels) : null;
  const priceUnit = paid && (paid.mode === "token" || paid.mode === "tiered") ? t("perMillion") : null;
  const periods = paid?.schedulePeriods ?? [];
  const currentPeriod = paid ? resolveCurrentSchedulePeriod(periods, paid.scheduleUTCOffsetMinutes ?? 0) : null;
  const cacheWriteNote = paid ? cacheWritePricingNote(model.protocols, paid, pricingLabels.billingDisplay) : null;

  return (
    <div
      data-detail-model={model.platformModelName}
      className="flex max-h-full min-h-0 flex-col overflow-y-auto p-1.5 text-[11px] leading-4"
    >
      <div className="flex h-7 shrink-0 items-center px-2">
        <span className="min-w-0 truncate font-medium text-foreground">{resolveModelOptionLabel(model.platformModelName)}</span>
      </div>
      <dl className="shrink-0">
        <ModelDetailLine label={t("modalities")}>
          {model.inputModalities.length > 0 ? (
            <>
              <ModalityIcons modalities={model.inputModalities} labels={inputModalityLabels} />
              {model.outputModalities.length > 0 ? (
                <>
                  <ArrowRight className="size-3 text-muted-foreground" strokeWidth={1.6} aria-hidden="true" />
                  <ModalityIcons modalities={model.outputModalities} labels={inputModalityLabels} />
                </>
              ) : null}
            </>
          ) : (
            <span className="text-muted-foreground">{t("unknown")}</span>
          )}
        </ModelDetailLine>
        <ModelDetailLine label={t("context")}>
          {model.contextWindow !== null ? (
            formatContextWindow(model.contextWindow)
          ) : (
            <span className="text-muted-foreground">{t("unknown")}</span>
          )}
        </ModelDetailLine>
        {billingEnabled && model.personalProviderName !== null ? (
          // Takes the price row: the platform does not charge for a user-key model, the provider does.
          <ModelDetailLine label={t("price")}>{t("chargedByProvider")}</ModelDetailLine>
        ) : null}
      </dl>
      {!table && !pricing?.isFree ? null : (
        <>
          <div className="mx-2 my-1 h-px shrink-0 bg-border" />
          {!table ? (
            <dl className="shrink-0">
              <ModelDetailLine label={t("price")}>{pricingLabels.freeModel}</ModelDetailLine>
            </dl>
          ) : (
            <table className="w-full shrink-0 border-separate border-spacing-0 tabular-nums">
              <thead>
                <tr className="h-7 text-muted-foreground">
                  <th scope="col" className="px-2 text-left font-normal">{t("price")}</th>
                  {table.tiers.length > 0 ? (
                    table.tiers.map((tier) => (
                      <th key={tier} scope="col" className="whitespace-nowrap pr-2 text-right font-normal">{tier}</th>
                    ))
                  ) : (
                    <th scope="col" className="pr-2 text-right font-normal">{priceUnit}</th>
                  )}
                </tr>
              </thead>
              <tbody>
                {table.rows.map((row) => (
                  <tr key={row.label} className="h-7">
                    <th scope="row" className="whitespace-nowrap px-2 text-left font-normal text-muted-foreground">{row.label}</th>
                    {row.values.map((value, index) => (
                      <td key={`${row.label}-${table.tiers[index] ?? index}`} className="whitespace-nowrap pr-2 text-right text-foreground">
                        {value}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </>
      )}
      {table && table.tiers.length > 0 && priceUnit ? (
        <p className="shrink-0 px-2 pb-1 text-right text-[10px] text-muted-foreground">{priceUnit}</p>
      ) : null}
      {periods.length > 0 ? (
        <>
          <div className="mx-2 my-1 h-px shrink-0 bg-border" />
          <div className="flex h-7 shrink-0 items-center px-2 text-muted-foreground">{t("schedule")}</div>
          <dl className="shrink-0">
            {periods.map((period) => {
              const active = period === currentPeriod;
              return (
                <div
                  key={`${period.name}-${period.start}-${period.end}`}
                  className={cn(
                    "mx-1 flex h-7 items-center justify-between gap-3 rounded-md px-1 tabular-nums",
                    active ? "bg-accent text-accent-foreground" : "text-muted-foreground",
                  )}
                >
                  <dt className="min-w-0 truncate">
                    {period.name}
                    <span className="ml-1.5 text-[10px] text-muted-foreground">
                      {formatScheduleWeekdays(period.weekdays, pricingLabels.schedule)}{" "}
                      {pricingLabels.schedule.timeRange(period.start, period.end)}
                    </span>
                  </dt>
                  <dd className={cn("shrink-0", active ? "text-foreground" : null)}>
                    {active ? <span className="sr-only">{pricingLabels.schedule.now}</span> : null}
                    {formatRateMultiplier(period.ratePercent)}
                  </dd>
                </div>
              );
            })}
          </dl>
        </>
      ) : null}
      {cacheWriteNote ? (
        <p className="shrink-0 px-2 pb-1 pt-1.5 text-[10px] leading-[1.45] text-muted-foreground">{cacheWriteNote}</p>
      ) : null}
    </div>
  );
}

type DetailPaneLayout = {
  left: number;
  top: number;
  width: number;
  /** Height of the details inside the border; null until they have been measured. */
  bodyHeight: number | null;
};

function sameDetailLayout(a: DetailPaneLayout | null, b: DetailPaneLayout | null): boolean {
  return a === b || (a !== null && b !== null && a.left === b.left && a.top === b.top && a.width === b.width && a.bodyHeight === b.bodyHeight);
}

function findSubmenuContent(submenu: HTMLElement, groupKey: string): HTMLElement | null {
  return submenu.querySelector<HTMLElement>(`[data-model-submenu-group="${CSS.escape(groupKey)}"]`);
}

function findDetailContent(pane: HTMLElement, modelName: string): HTMLElement | null {
  return pane.querySelector<HTMLElement>(`[data-detail-model="${CSS.escape(modelName)}"]`);
}

export function ChatModelPicker({
  modelOptions,
  billingDisplayCurrency,
  billingDisplayUsdToCnyRate,
  billingEnabled,
  selectedPlatformModelName,
  loading,
  disabled,
  onModelCatalogRefresh,
  onModelChange,
  placementPreference,
}: ChatModelPickerProps) {
  const t = useTranslations("chat.modelPicker");
  const isMobile = useIsMobile();
  const mobileShift = useChatPopoverAlignOffset("end");
  const [open, setOpen] = React.useState(false);
  const [activeGroupKey, setActiveGroupKey] = React.useState("");
  const [mobileGroupKey, setMobileGroupKey] = React.useState<string | null>(null);
  const [desktopSubmenuSide, setDesktopSubmenuSide] = React.useState<"right" | "left">("right");
  const [desktopSubmenuTop, setDesktopSubmenuTop] = React.useState(0);
  const [desktopSubmenuWidth, setDesktopSubmenuWidth] = React.useState(DESKTOP_MODEL_MENU_WIDTH);
  // The submenu's content height, set explicitly so switching groups can animate it.
  const [desktopSubmenuBodyHeight, setDesktopSubmenuBodyHeight] = React.useState<number | null>(null);
  // Detail pane placement inside the menu root box; null when the window has no room for it.
  const [desktopDetailLayout, setDesktopDetailLayout] = React.useState<DetailPaneLayout | null>(null);
  // The model whose row is hovered (or keyboard-focused). Mirrored in a ref so the placement code
  // reads the current row without depending on render timing, and stays a stable callback.
  const [desktopDetailModelName, setDesktopDetailModelName] = React.useState<string | null>(null);
  const desktopDetailModelNameRef = React.useRef<string | null>(null);
  const setDesktopDetailModel = React.useCallback((name: string | null) => {
    desktopDetailModelNameRef.current = name;
    setDesktopDetailModelName(name);
  }, []);
  const desktopDetailPaneRef = React.useRef<HTMLDivElement | null>(null);
  const desktopDetailBodyRef = React.useRef<HTMLDivElement | null>(null);
  const [desktopGroupListMaxHeight, setDesktopGroupListMaxHeight] = React.useState(320);
  const [desktopSubmenuListMaxHeight, setDesktopSubmenuListMaxHeight] = React.useState(320);
  const desktopMenuRootRef = React.useRef<HTMLDivElement | null>(null);
  const desktopGroupMenuRef = React.useRef<HTMLDivElement | null>(null);
  // Radix mounts the popover body in a portal one commit after it opens, so the refs are still
  // empty when the open effect runs. This flips once the node exists and re-runs the metrics.
  const [desktopMenuMounted, setDesktopMenuMounted] = React.useState(false);
  const attachDesktopMenuRoot = React.useCallback((node: HTMLDivElement | null) => {
    desktopMenuRootRef.current = node;
    setDesktopMenuMounted(node !== null);
  }, []);
  const desktopSubmenuRef = React.useRef<HTMLDivElement | null>(null);
  const desktopSubmenuBodyRef = React.useRef<HTMLDivElement | null>(null);
  const selectedModelButtonRef = React.useRef<HTMLButtonElement | null>(null);
  const desktopGroupItemRefs = React.useRef(new Map<string, HTMLButtonElement>());
  const desktopModelRowRefs = React.useRef(new Map<string, HTMLDivElement>());
  const selectedModel = React.useMemo(
    () => modelOptions.find((item) => item.platformModelName === selectedPlatformModelName) ?? null,
    [modelOptions, selectedPlatformModelName],
  );
  // Same grouping as the list, so a model on the user's own key opens its provider group.
  const selectedPresentation = React.useMemo(
    () => (selectedModel ? resolveModelGroupPresentation(selectedModel) : null),
    [selectedModel],
  );
  const selectedGroupKey = selectedPresentation?.key ?? "";
  const selectedGroupLabel = selectedPresentation?.label ?? "";
  const modelGroups = React.useMemo(() => resolveModelGroups(modelOptions), [modelOptions]);
  // Counts share one width, so the badges in front of them line up whatever the digit count.
  const groupCountStyle = React.useMemo<React.CSSProperties>(
    () => ({ minWidth: `${String(Math.max(0, ...modelGroups.map((group) => group.items.length))).length}ch` }),
    [modelGroups],
  );
  const activeDesktopGroupKey = activeGroupKey || selectedGroupKey || modelGroups[0]?.key || "";
  const activeDesktopGroup = React.useMemo(
    () => modelGroups.find((group) => group.key === activeDesktopGroupKey) ?? modelGroups[0] ?? null,
    [activeDesktopGroupKey, modelGroups],
  );
  const hasDesktopModelSubmenu = Boolean(activeDesktopGroup?.items.length);
  const desktopGroupHasPricing = Boolean(activeDesktopGroup?.items.some((item) => item.pricing));
  const mobileGroup = React.useMemo(
    () => modelGroups.find((group) => group.key === mobileGroupKey) ?? null,
    [mobileGroupKey, modelGroups],
  );
  const desktopDetailPaneVisible = desktopDetailLayout !== null && !isMobile;
  // The third level opens only while a model row is hovered (or focused), like a submenu.
  const desktopDetailModel = React.useMemo(
    () => activeDesktopGroup?.items.find((item) => item.platformModelName === desktopDetailModelName) ?? null,
    [activeDesktopGroup, desktopDetailModelName],
  );
  const desktopDetailShown = desktopDetailPaneVisible && desktopDetailModel !== null;
  const desktopDetailPaneId = React.useId();
  const reduceMotion = useReducedMotion();
  // Which way the pointer moved, so the next group's models and the next model's details slide in
  // from that side.
  const desktopGroupDirection = useSwitchDirection(activeDesktopGroup ? modelGroups.indexOf(activeDesktopGroup) : -1);
  const desktopDetailDirection = useSwitchDirection(
    activeDesktopGroup && desktopDetailModel ? activeDesktopGroup.items.indexOf(desktopDetailModel) : -1,
  );
  const inputModalityLabels = React.useMemo<InputModalityLabels>(
    () => ({
      title: t("inputModalities.title"),
      separator: t("inputModalities.separator"),
      names: {
        text: t("inputModalities.text"),
        image: t("inputModalities.image"),
        pdf: t("inputModalities.pdf"),
        audio: t("inputModalities.audio"),
        video: t("inputModalities.video"),
      },
    }),
    [t],
  );
  const pricingLabels = React.useMemo(
    () => ({
      freeModel: t("freeModel"),
      freeModelDescription: t("freeModelDescription"),
      tieredPricing: t("tieredPricing"),
      callPricing: t("callPricing"),
      durationPricing: t("durationPricing"),
      tokenPricing: t("tokenPricing"),
      input: t("input"),
      output: t("output"),
      cacheRead: t("cacheRead"),
      perCall: t("perCall"),
      perSecond: t("perSecond"),
      callUnit: t("callUnit"),
      secondUnit: t("secondUnit"),
      schedule: {
        title: t("schedule.title"),
        now: t("schedule.now"),
        weekdays: [t("schedule.sun"), t("schedule.mon"), t("schedule.tue"), t("schedule.wed"), t("schedule.thu"), t("schedule.fri"), t("schedule.sat"), t("schedule.everyDay")],
        dayRange: (from: string, to: string) => t("schedule.dayRange", { from, to }),
        daySeparator: t("schedule.daySeparator"),
        timeRange: (from: string, to: string) => t("schedule.timeRange", { from, to }),
      },
      billingDisplay: {
        cacheWrite: t("cacheWrite"),
        cacheWrite5m: t("cacheWrite5m"),
        cacheWrite1h: t("cacheWrite1h"),
        cacheWrite5m1h: t("cacheWrite5m1h"),
        claudeCacheWriteMixedNote: (multiplier: string) => t("claudeCacheWriteMixedNote", { multiplier }),
        claudeCacheWriteNote: (timeout: "5m" | "1h", multiplier: string) => t("claudeCacheWriteNote", { timeout, multiplier }),
        claudeFastModeNote: (multiplier: string) => t("claudeFastModeNote", { multiplier }),
        openaiServiceTierNote: (tier: string, multiplier: string) => t("openaiServiceTierNote", { tier, multiplier }),
        scheduleRateNote: (period: string, multiplier: string) => t("scheduleRateNote", { period, multiplier }),
        cacheWritePricingLabel: t("cacheWritePricingLabel"),
        cacheWritePricingNote: t("cacheWritePricingNote"),
      },
    }),
    [t],
  );
  const billingDisplay = React.useMemo<BillingDisplayOptions>(
    () => ({
      currency: billingDisplayCurrency,
      usdToCnyRate: billingDisplayUsdToCnyRate,
    }),
    [billingDisplayCurrency, billingDisplayUsdToCnyRate],
  );

  React.useEffect(() => {
    if (!open || !isMobile) {
      setMobileGroupKey(null);
    }
  }, [isMobile, open]);

  // Keep the state object when nothing moved: metrics run on every resize-observer pass and would
  // otherwise re-render the whole menu.
  const setDetailLayout = React.useCallback((next: DetailPaneLayout | null) => {
    setDesktopDetailLayout((current) => (sameDetailLayout(current, next) ? current : next));
  }, []);

  const updateDesktopSubmenuMetrics = React.useCallback(() => {
    if (!open || isMobile) {
      // Keep the last placement: Radix keeps the content mounted through the
      // ~150ms close animation, and resetting side/top here would visibly
      // snap a left-side submenu to the right while it fades out. Reopening
      // recomputes every metric in the layout effect before paint.
      return;
    }

    const menuRoot = desktopMenuRootRef.current;
    const groupMenu = desktopGroupMenuRef.current;
    if (!menuRoot || !groupMenu) {
      return;
    }

    const layoutRect = createPopoverLayoutRectReader(menuRoot);
    const menuRootRect = layoutRect(menuRoot);
    const groupMenuRect = layoutRect(groupMenu);
    const submenu = desktopSubmenuRef.current;
    const submenuBody = desktopSubmenuBodyRef.current;
    // While groups switch, the outgoing list is still in the card; measure the incoming one.
    const submenuContent = submenu && activeDesktopGroup ? findSubmenuContent(submenu, activeDesktopGroup.key) : null;
    const submenuScrollViewport = submenuContent?.querySelector<HTMLElement>("[data-model-menu-viewport]");
    const activeGroupButton = activeDesktopGroup
      ? desktopGroupItemRefs.current.get(activeDesktopGroup.key)
      : null;
    const activeGroupRect = activeGroupButton ? layoutRect(activeGroupButton) : undefined;
    const triggerRect = document.getElementById("chat-model-menu-trigger")?.getBoundingClientRect();
    const viewportLeft = MODEL_MENU_COLLISION_PADDING;
    const viewportRight = window.innerWidth - MODEL_MENU_COLLISION_PADDING;
    const viewportTop = MODEL_MENU_COLLISION_PADDING;
    const viewportBottom = window.innerHeight - MODEL_MENU_COLLISION_PADDING;
    const viewportHeight = Math.max(0, viewportBottom - viewportTop);
    const rightAvailableWidth = Math.max(0, viewportRight - groupMenuRect.right - DESKTOP_MODEL_SUBMENU_GAP);
    const leftAvailableWidth = Math.max(0, groupMenuRect.left - viewportLeft - DESKTOP_MODEL_SUBMENU_GAP);
    const nextSubmenuSide =
      rightAvailableWidth >= DESKTOP_MODEL_MENU_WIDTH || rightAvailableWidth >= leftAvailableWidth
        ? "right"
        : "left";
    const nextSubmenuWidth = Math.max(
      DESKTOP_MODEL_MENU_MIN_SCROLL_HEIGHT,
      Math.min(
        DESKTOP_MODEL_MENU_WIDTH,
        nextSubmenuSide === "right" ? rightAvailableWidth : leftAvailableWidth,
      ),
    );
    const nextGroupListMaxHeight = resolveDesktopModelMenuListMaxHeight({
      viewportTop,
      viewportBottom,
      triggerTop: triggerRect?.top,
      triggerBottom: triggerRect?.bottom,
      sideOffset: DESKTOP_MODEL_MENU_SIDE_OFFSET,
      verticalChrome: DESKTOP_GROUP_MENU_VERTICAL_CHROME,
    });

    let nextSubmenuTop = 0;
    let nextSubmenuListMaxHeight = nextGroupListMaxHeight;
    let nextSubmenuBodyHeight: number | null = null;
    if (hasDesktopModelSubmenu && activeGroupRect) {
      // Derive the height from the unclamped scroll content instead of feeding
      // the rendered (clamped) height back in. The assumed chrome misses the
      // real border-included chrome by ~1px, so that feedback loop grows a
      // bottom-anchored submenu by the error once per ResizeObserver pass,
      // crawling it upward for seconds instead of placing it in one frame.
      // Padding and border around the body. Layout sizes ignore the opening scale and stay exact
      // while the body height animates, unlike rects.
      const submenuChrome = submenu && submenuBody
        ? Math.max(DESKTOP_SUBMENU_VERTICAL_CHROME, submenu.offsetHeight - submenuBody.offsetHeight)
        : DESKTOP_SUBMENU_VERTICAL_CHROME;
      const submenuContentHeight = submenuScrollViewport?.scrollHeight ?? nextGroupListMaxHeight;
      const submenuOuterHeight = Math.min(submenuContentHeight + submenuChrome, viewportHeight);
      const maxViewportTop = Math.max(viewportTop, viewportBottom - submenuOuterHeight);
      // Anchor in viewport coordinates, then convert to an offset within
      // menuRoot (which may sit above viewportTop for a frame before re-shift).
      const preferredSubmenuViewportTop = Math.min(
        Math.max(activeGroupRect.top, viewportTop),
        maxViewportTop,
      );
      nextSubmenuTop = preferredSubmenuViewportTop - menuRootRect.top;
      const actualSubmenuViewportTop = menuRootRect.top + nextSubmenuTop;
      nextSubmenuListMaxHeight = resolveDesktopMenuListMaxHeight(
        Math.min(viewportHeight, viewportBottom - actualSubmenuViewportTop),
        submenuChrome,
      );
      nextSubmenuBodyHeight = submenuScrollViewport ? Math.min(submenuContentHeight, nextSubmenuListMaxHeight) : null;
    }

    // The pane is placed level with the row it describes, like the submenu is with its group.
    const detailModelName = desktopDetailModelNameRef.current;
    const anchorRow = detailModelName ? desktopModelRowRefs.current.get(detailModelName) : null;
    // Where the row ends up, not where it is drawn now: the submenu may still be gliding to its new
    // top and the row's list sliding in, so take its layout offset from the submenu's target top.
    const anchorOffset = anchorRow && submenu ? layoutOffsetTop(anchorRow, submenu) : Number.NaN;
    const anchorTop = Number.isFinite(anchorOffset) ? nextSubmenuTop + anchorOffset : nextSubmenuTop;
    // Height comes from the natural content, since the rendered box is capped by the previous one.
    const pane = desktopDetailPaneRef.current;
    const paneBody = desktopDetailBodyRef.current;
    const paneBorder = pane && paneBody ? pane.offsetHeight - paneBody.offsetHeight : 1;
    const paneContent = pane && detailModelName ? findDetailContent(pane, detailModelName) : null;
    const paneContentHeight = paneContent?.scrollHeight ?? 0;
    const paneHeight = Math.max(paneContentHeight + paneBorder, DESKTOP_DETAIL_PANE_MIN_HEIGHT);
    const paneTopFloor = viewportTop - menuRootRect.top;
    const maxDetailTop = viewportBottom - menuRootRect.top - paneHeight;
    const nextDetailTop = Math.max(paneTopFloor, Math.min(anchorTop, maxDetailTop));
    const nextDetailMaxHeight = viewportBottom - (menuRootRect.top + nextDetailTop);
    const detailHeightFits = nextDetailMaxHeight >= DESKTOP_DETAIL_PANE_MIN_HEIGHT;
    const nextDetailBodyHeight = paneContent ? Math.min(paneContentHeight, nextDetailMaxHeight - paneBorder) : null;
    // Placed next to the submenu it needs its own width on top of what the submenu already takes.
    const submenuOccupiedWidth = nextSubmenuWidth + DESKTOP_MODEL_SUBMENU_GAP;
    const outerAvailableWidth =
      (nextSubmenuSide === "right" ? rightAvailableWidth : leftAvailableWidth) -
      submenuOccupiedWidth -
      DESKTOP_DETAIL_PANE_GAP;
    if (detailHeightFits && outerAvailableWidth >= DESKTOP_DETAIL_PANE_MIN_WIDTH) {
      const outerWidth = Math.min(DESKTOP_DETAIL_PANE_WIDTH, outerAvailableWidth);
      setDetailLayout({
        left:
          nextSubmenuSide === "right"
            ? menuRootRect.width + submenuOccupiedWidth + DESKTOP_DETAIL_PANE_GAP
            : -(submenuOccupiedWidth + DESKTOP_DETAIL_PANE_GAP + outerWidth),
        width: outerWidth,
        top: nextDetailTop,
        bodyHeight: nextDetailBodyHeight,
      });
    } else {
      // Not enough room beside the submenu: cover the group menu instead. The pane only shows while
      // a model row is hovered, so the group list stays reachable. Coordinates are menu-root relative
      // and may go negative, because they start at the group menu.
      const overlayOnLeft = nextSubmenuSide === "right";
      const overlayAvailableWidth = overlayOnLeft
        ? menuRootRect.left + menuRootRect.width - viewportLeft
        : viewportRight - menuRootRect.left;
      const overlayWidth = Math.min(DESKTOP_DETAIL_PANE_WIDTH, overlayAvailableWidth);
      // It has to hide the group menu completely, otherwise the peeking edge looks broken.
      const overlayFits =
        detailHeightFits &&
        overlayWidth >= DESKTOP_DETAIL_PANE_MIN_WIDTH &&
        overlayWidth >= groupMenuRect.width;
      setDetailLayout(
        overlayFits
          ? {
              // Aligned to the group menu's right edge, so it covers that menu exactly.
              left: overlayOnLeft ? groupMenuRect.width - overlayWidth : 0,
              width: overlayWidth,
              top: nextDetailTop,
              bodyHeight: nextDetailBodyHeight,
            }
          : null,
      );
    }

    setDesktopSubmenuSide(nextSubmenuSide);
    setDesktopSubmenuTop(nextSubmenuTop);
    setDesktopSubmenuBodyHeight(nextSubmenuBodyHeight);
    setDesktopSubmenuWidth(nextSubmenuWidth);
    setDesktopGroupListMaxHeight(nextGroupListMaxHeight);
    setDesktopSubmenuListMaxHeight(nextSubmenuListMaxHeight);
  }, [activeDesktopGroup, hasDesktopModelSubmenu, isMobile, open, setDetailLayout]);

  React.useLayoutEffect(() => {
    updateDesktopSubmenuMetrics();

    if (!open || isMobile) {
      return;
    }

    window.addEventListener("resize", updateDesktopSubmenuMetrics);
    window.addEventListener("scroll", updateDesktopSubmenuMetrics, true);
    // Radix places the popover by rewriting its wrapper's transform: on the first frame and again
    // whenever the trigger moves (the composer growing, for one). No resize observer sees a
    // transform, so watch the style itself; the viewport clamping depends on where the menu is.
    const popperWrapper = desktopMenuRootRef.current?.closest("[data-slot=popover-content]")?.parentElement;
    const moveObserver = new MutationObserver(updateDesktopSubmenuMetrics);
    if (popperWrapper) {
      moveObserver.observe(popperWrapper, { attributes: true, attributeFilter: ["style"] });
    }
    if (typeof ResizeObserver === "undefined") {
      return () => {
        window.removeEventListener("resize", updateDesktopSubmenuMetrics);
        window.removeEventListener("scroll", updateDesktopSubmenuMetrics, true);
        moveObserver.disconnect();
      };
    }

    const observer = new ResizeObserver(updateDesktopSubmenuMetrics);
    if (desktopMenuRootRef.current) {
      observer.observe(desktopMenuRootRef.current);
    }
    if (desktopGroupMenuRef.current) {
      observer.observe(desktopGroupMenuRef.current);
    }
    const submenuContent = desktopSubmenuRef.current && activeDesktopGroup
      ? findSubmenuContent(desktopSubmenuRef.current, activeDesktopGroup.key)
      : null;
    if (submenuContent) {
      observer.observe(submenuContent);
      const list = submenuContent.querySelector("[data-model-menu-viewport]")?.firstElementChild;
      if (list) {
        observer.observe(list);
      }
    }
    const activeGroupButton = activeDesktopGroup
      ? desktopGroupItemRefs.current.get(activeDesktopGroup.key)
      : null;
    if (activeGroupButton) {
      observer.observe(activeGroupButton);
    }

    return () => {
      window.removeEventListener("resize", updateDesktopSubmenuMetrics);
      window.removeEventListener("scroll", updateDesktopSubmenuMetrics, true);
      moveObserver.disconnect();
      observer.disconnect();
    };
  }, [activeDesktopGroup, desktopMenuMounted, hasDesktopModelSubmenu, isMobile, open, updateDesktopSubmenuMetrics]);

  // The pane's height depends on the model it describes, so place it once that content is in the
  // DOM; a layout effect runs before paint, so the pane never shows at a stale position.
  React.useLayoutEffect(() => {
    if (desktopDetailModelName) {
      updateDesktopSubmenuMetrics();
    }
  }, [desktopDetailModelName, updateDesktopSubmenuMetrics]);

  const handleOpenChange = React.useCallback(
    (nextOpen: boolean) => {
      if (nextOpen) {
        setActiveGroupKey(selectedGroupKey || modelGroups[0]?.key || "");
        if (onModelCatalogRefresh) {
          void Promise.resolve(onModelCatalogRefresh()).catch((): undefined => undefined);
        }
      }
      // Esc and outside clicks close the menu without a mouseleave, so drop the hovered row here.
      setDesktopDetailModel(null);
      setOpen(nextOpen);
    },
    [modelGroups, onModelCatalogRefresh, selectedGroupKey, setDesktopDetailModel],
  );

  const closeMenu = React.useCallback(() => {
    handleOpenChange(false);
  }, [handleOpenChange]);

  const selectDesktopGroup = React.useCallback((groupKey: string) => {
    if (groupKey === activeDesktopGroupKey) {
      return;
    }
    setActiveGroupKey(groupKey);
  }, [activeDesktopGroupKey]);

  return (
      <div className="min-w-0 max-w-[min(320px,100%)] shrink">
        <Popover open={open} onOpenChange={handleOpenChange}>
          <PopoverTrigger asChild>
            <InputGroupButton
              ref={mobileShift.triggerRef}
              id="chat-model-menu-trigger"
              type="button"
              variant="ghost"
              size="sm"
              className="w-full min-w-0 max-w-[min(320px,100%)] rounded-lg px-1.5 hover:bg-accent focus-visible:bg-accent data-[state=open]:bg-accent sm:px-2"
              disabled={disabled || loading}
              aria-label={t("selectModel")}
            >
              {loading ? (
                <ChatModelTriggerSkeleton />
              ) : selectedModel ? (
                <ChatModelIdentity model={selectedModel} />
              ) : selectedPlatformModelName.trim() ? (
                <span className="truncate text-[12px] font-medium text-foreground">
                  {resolveModelOptionLabel(selectedPlatformModelName)}
                </span>
              ) : (
                <span className="truncate text-[12px] font-medium text-muted-foreground">
                  {t("selectModel")}
                </span>
              )}
            </InputGroupButton>
          </PopoverTrigger>
          {/* On a phone the menu keeps the composer's side, as the other composer menus do, and its
              list shrinks to the room left there. Letting Radix flip it measured the unclamped list,
              which never fits below the landing composer, so it always opened upwards. */}
          <PopoverContent
            ref={isMobile ? mobileShift.contentRef : undefined}
            align="end"
            alignOffset={isMobile ? mobileShift.alignOffset : 0}
            side={isMobile ? placementPreference : "bottom"}
            avoidCollisions={!isMobile}
            sideOffset={DESKTOP_MODEL_MENU_SIDE_OFFSET}
            collisionPadding={24}
            onOpenAutoFocus={(event) => {
              if (!isMobile && selectedModelButtonRef.current) {
                event.preventDefault();
                selectedModelButtonRef.current.focus();
                // The menu places this focus, not the user, so it must not open the detail pane;
                // the row's focus handler runs in the same render, before this reset lands.
                setDesktopDetailModel(null);
              }
            }}
            className={cn(
              "relative overflow-visible rounded-xl",
              isMobile
                ? "w-[min(20rem,calc(100vw-3rem))] p-1.5 [--model-menu-scroll-max-height:max(7rem,calc(var(--radix-popover-content-available-height)-2.75rem))]"
                : "w-[min(14rem,calc(100vw-3rem))] border-0 bg-transparent p-0 shadow-none",
            )}
          >
            {isMobile ? (
              <>
                <div className="flex h-7 items-center justify-between gap-2 px-2">
                  {mobileGroup ? (
                    <button
                      type="button"
                      className="-ml-1.5 flex h-7 min-w-0 items-center gap-0.5 rounded-md px-0.5 text-[11px] font-medium text-muted-foreground outline-none transition-colors hover:bg-accent hover:text-foreground focus-visible:bg-accent focus-visible:text-foreground"
                      onClick={() => setMobileGroupKey(null)}
                    >
                      <ChevronLeft className="size-3.5" strokeWidth={1.8} />
                      <span>{t("group")}</span>
                    </button>
                  ) : (
                    <span className="text-[11px] font-medium text-foreground">{t("group")}</span>
                  )}
                  <span className="min-w-0 truncate text-right text-[10px] font-medium text-muted-foreground">
                    {mobileGroup ? mobileGroup.label : selectedGroupLabel}
                  </span>
                </div>
                {modelGroups.length === 0 ? (
                  <div className="px-2 py-3 text-[11px] leading-4 text-muted-foreground">
                    {t("empty")}
                  </div>
                ) : (
                  <ModelMenuScrollContainer>
                    {mobileGroup ? (
                      <div className="flex flex-col gap-0.5">
                        {mobileGroup.items.map((item) => (
                          <ChatModelMenuItem
                            key={item.platformModelName}
                            model={item}
                            selected={item.platformModelName === selectedPlatformModelName}
                            onSelect={() => {
                              onModelChange(item.platformModelName);
                              closeMenu();
                            }}
                            billingDisplay={billingDisplay}
                            pricingLabels={pricingLabels}
                            inputModalityLabels={inputModalityLabels}
                            viewPricingLabel={t("viewPricing")}
                            freeBadgeLabel={t("freeBadge")}
                            reservePricingSlot={mobileGroup.items.some((item) => item.pricing)}
                          />
                        ))}
                      </div>
                    ) : (
                      <div className="flex flex-col gap-0.5">
                        {modelGroups.map((group) => {
                          const selectedGroup = group.key === selectedGroupKey;
                          const groupIconURL = resolveModelIconURL(group.icon);
                          return (
                            <button
                              type="button"
                              key={group.key}
                              className={cn(
                                "flex h-7 w-full items-center justify-between gap-2 rounded-md px-2 py-0 text-left text-[11px] font-medium outline-none transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:bg-accent focus-visible:text-accent-foreground",
                                selectedGroup ? "bg-accent text-accent-foreground" : "text-muted-foreground",
                              )}
                              onClick={() => {
                                setMobileGroupKey(group.key);
                              }}
                            >
                              {group.personal && !group.icon ? <PersonalGroupIcon /> : <ModelIcon iconUrl={groupIconURL} label={group.label} />}
                              <span className="min-w-0 flex-1 truncate font-medium">{group.label}</span>
                              {group.personal ? <PersonalGroupBadge label={t("personalBadge")} /> : null}
                              <span className="shrink-0 text-right text-[10px] tabular-nums text-muted-foreground/80" style={groupCountStyle}>
                                {group.items.length}
                              </span>
                            </button>
                          );
                        })}
                      </div>
                    )}
                  </ModelMenuScrollContainer>
                )}
              </>
            ) : (
              <div ref={attachDesktopMenuRoot} className="relative min-w-0">
                {hasDesktopModelSubmenu && activeDesktopGroup ? (
                  <MorphingCard
                    ref={desktopSubmenuRef}
                    bodyRef={desktopSubmenuBodyRef}
                    top={desktopSubmenuTop}
                    bodyHeight={desktopSubmenuBodyHeight}
                    style={{ width: desktopSubmenuWidth }}
                    className={cn(
                      "absolute flex max-h-[calc(100dvh-3rem)] flex-col overflow-hidden rounded-xl border-[0.5px] border-border bg-popover p-1.5 shadow-xs",
                      desktopSubmenuSide === "right" ? "left-[calc(100%+0.5rem)]" : "right-[calc(100%+0.5rem)]",
                    )}
                  >
                    <SlideSwitch
                      itemKey={activeDesktopGroup.key}
                      direction={desktopGroupDirection}
                      axis="y"
                      duration={MORPH_DURATION_S}
                    >
                      <div data-model-submenu-group={activeDesktopGroup.key}>
                        <ModelMenuScrollContainer maxHeight={desktopSubmenuListMaxHeight}>
                          <div
                            className="flex flex-col gap-0.5"
                            onMouseLeave={() => setDesktopDetailModel(null)}
                            onBlur={(event) => {
                              if (!event.currentTarget.contains(event.relatedTarget)) {
                                setDesktopDetailModel(null);
                              }
                            }}
                          >
                            {activeDesktopGroup.items.map((item) => (
                              <ChatModelMenuItem
                                key={item.platformModelName}
                                model={item}
                                selected={item.platformModelName === selectedPlatformModelName}
                                buttonRef={item.platformModelName === selectedPlatformModelName ? selectedModelButtonRef : undefined}
                                onSelect={() => {
                                  onModelChange(item.platformModelName);
                                  closeMenu();
                                }}
                                billingDisplay={billingDisplay}
                                pricingLabels={pricingLabels}
                                inputModalityLabels={inputModalityLabels}
                                viewPricingLabel={t("viewPricing")}
                                reservePricingSlot={desktopGroupHasPricing}
                                withDetailPane={desktopDetailPaneVisible}
                                describedBy={
                                  desktopDetailShown && desktopDetailLayout && desktopDetailModel?.platformModelName === item.platformModelName
                                    ? desktopDetailPaneId
                                    : undefined
                                }
                                freeBadgeLabel={t("freeBadge")}
                                onHover={() => setDesktopDetailModel(item.platformModelName)}
                                rowRef={(node) => {
                                  if (node) {
                                    desktopModelRowRefs.current.set(item.platformModelName, node);
                                    return;
                                  }
                                  desktopModelRowRefs.current.delete(item.platformModelName);
                                }}
                              />
                            ))}
                          </div>
                        </ModelMenuScrollContainer>
                      </div>
                    </SlideSwitch>
                  </MorphingCard>
                ) : null}

                <AnimatePresence>
                  {desktopDetailShown && desktopDetailLayout ? (
                    <MorphingCard
                      key="detail"
                      ref={desktopDetailPaneRef}
                      bodyRef={desktopDetailBodyRef}
                      id={desktopDetailPaneId}
                      top={desktopDetailLayout.top}
                      bodyHeight={desktopDetailLayout.bodyHeight}
                      style={{
                        left: desktopDetailLayout.left,
                        width: desktopDetailLayout.width,
                        // Grow out of the submenu it belongs to.
                        transformOrigin: desktopDetailLayout.left > 0 ? "left center" : "right center",
                      }}
                      initial={{ opacity: 0, scale: reduceMotion ? 1 : 0.96 }}
                      animate={{ opacity: 1, scale: 1 }}
                      exit={{ opacity: 0, scale: reduceMotion ? 1 : 0.96 }}
                      transition={{ duration: 0.15, ease: "easeOut" }}
                      className="absolute z-10 flex flex-col overflow-hidden rounded-xl border-[0.5px] border-border bg-popover shadow-xs"
                    >
                      <SlideSwitch
                        itemKey={desktopDetailModel.platformModelName}
                        direction={desktopDetailDirection}
                        axis="y"
                        duration={MORPH_DURATION_S}
                        className="h-full"
                      >
                        <ChatModelDetailPanel
                          model={desktopDetailModel}
                          inputModalityLabels={inputModalityLabels}
                          pricingLabels={pricingLabels}
                          billingDisplay={billingDisplay}
                          billingEnabled={billingEnabled}
                        />
                      </SlideSwitch>
                    </MorphingCard>
                  ) : null}
                </AnimatePresence>

                <div
                  ref={desktopGroupMenuRef}
                  className="flex min-w-0 max-h-[calc(100dvh-3rem)] flex-col overflow-hidden rounded-xl border-[0.5px] border-border bg-popover p-1.5 shadow-xs"
                >
                  <div className="flex h-7 shrink-0 items-center justify-between gap-3 px-2">
                    <span className="text-[11px] font-medium text-foreground">{t("group")}</span>
                    <span className="truncate text-[10px] font-medium text-muted-foreground">
                      {selectedGroupLabel}
                    </span>
                  </div>
                  {modelGroups.length === 0 ? (
                    <div className="px-2 py-3 text-[11px] leading-4 text-muted-foreground">
                      {t("empty")}
                    </div>
                  ) : (
                    <div className="min-h-0 min-w-0">
                      <ModelMenuScrollContainer maxHeight={desktopGroupListMaxHeight}>
                        <div className="flex flex-col gap-0.5">
                          {modelGroups.map((group) => {
                            const selectedGroup = group.key === selectedGroupKey;
                            const activeGroup = group.key === activeDesktopGroup?.key;
                            const groupIconURL = resolveModelIconURL(group.icon);
                            return (
                              <button
                                type="button"
                                key={group.key}
                                ref={(node) => {
                                  if (node) {
                                    desktopGroupItemRefs.current.set(group.key, node);
                                    return;
                                  }
                                  desktopGroupItemRefs.current.delete(group.key);
                                }}
                                className={cn(
                                  "flex h-7 w-full items-center gap-2 rounded-md px-2 py-0 text-left text-[11px] font-medium outline-none transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:bg-accent focus-visible:text-accent-foreground",
                                  activeGroup ? "bg-accent text-accent-foreground" : "text-muted-foreground",
                                  selectedGroup && !activeGroup ? "text-foreground" : null,
                                )}
                                onMouseEnter={() => selectDesktopGroup(group.key)}
                                onFocus={() => selectDesktopGroup(group.key)}
                                onClick={() => selectDesktopGroup(group.key)}
                              >
                                {group.personal && !group.icon ? <PersonalGroupIcon /> : <ModelIcon iconUrl={groupIconURL} label={group.label} />}
                                <span className="min-w-0 flex-1 truncate font-medium">{group.label}</span>
                                {group.personal ? <PersonalGroupBadge label={t("personalBadge")} /> : null}
                                <span className="shrink-0 text-right text-[10px] tabular-nums text-muted-foreground/80" style={groupCountStyle}>
                                  {group.items.length}
                                </span>
                                <ChevronRight className="size-3.5 shrink-0 text-muted-foreground/65" strokeWidth={1.8} />
                              </button>
                            );
                          })}
                        </div>
                      </ModelMenuScrollContainer>
                    </div>
                  )}
                </div>
              </div>
            )}
        </PopoverContent>
      </Popover>
      </div>
  );
}
