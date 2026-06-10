"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import { Check, ChevronDown, ChevronLeft, ChevronRight, CircleDollarSign } from "lucide-react";
import { useTranslations } from "next-intl";

import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { InputGroupButton } from "@/components/ui/input-group";
import type { ChatModelOption } from "@/features/chat/types/chat-runtime";
import { useIsMobile } from "@/shared/hooks/use-mobile";
import { LobeHubIcon } from "@/shared/components/lobehub-icon";
import { cacheWritePricingLabel, cacheWritePricingNote, resolveCacheWritePricingUSD } from "@/shared/lib/billing-display";
import type { BillingDisplayLabels } from "@/shared/lib/billing-display";
import { resolveLobeHubIconURL, resolveModelIdentity } from "@/shared/lib/model-identity";
import { cn } from "@/lib/utils";

const MODEL_MENU_MAX_HEIGHT = 400;
const MODEL_MENU_MODEL_PANEL_MAX_HEIGHT = 280;
const MODEL_MENU_HEADER_HEIGHT = 28;
const MODEL_MENU_POPOVER_CHROME_HEIGHT = 12;
const MODEL_MENU_SAFE_INSET = 16;
const MODEL_MENU_VENDOR_ROW_HEIGHT = 28;
const MODEL_MENU_MODEL_ROW_HEIGHT = 28;
const MODEL_MENU_ROW_GAP = 2;
const MODEL_MENU_MODEL_PANEL_CHROME_HEIGHT = 12;
const MODEL_MENU_TEXT_WIDTH_UNIT = 7;
const MODEL_MENU_CONTENT_GAP_WIDTH = 56;
const MODEL_MENU_VIEWPORT_GUTTER = 24;
const MODEL_MENU_PANEL_GAP = 8;
const MODEL_MENU_COLLISION_GUTTER = 12;
const MODEL_MENU_SCROLL_MORE_THRESHOLD = 8;
const PRICING_TOOLTIP_TITLE_CLASS = "font-sans text-xs font-medium leading-4 text-background";
const PRICING_TOOLTIP_BODY_CLASS = "font-sans text-[11px] leading-4 text-background/80";

type FloatingModelPanelLayout = {
  key: number;
  x: number;
  y: number;
  width: number;
  listMaxHeight: number;
};

type ChatModelPickerProps = {
  modelOptions: ChatModelOption[];
  selectedPlatformModelName: string;
  loading: boolean;
  disabled: boolean;
  onModelChange: (platformModelName: string) => void;
};

function resolveModelMenuContentHeight(
  itemCount: number,
  rowHeight: number,
  maxContentHeight = MODEL_MENU_MAX_HEIGHT,
): number {
  const actualContentHeight = itemCount > 0
    ? itemCount * rowHeight + Math.max(0, itemCount - 1) * MODEL_MENU_ROW_GAP
    : 0;
  return Math.min(actualContentHeight, maxContentHeight);
}

function resolveModelMenuMaxHeight(
  itemCount: number,
  rowHeight: number,
  chromeHeight: number,
  availablePanelHeight?: number | null,
  maxContentHeight = MODEL_MENU_MAX_HEIGHT,
): string {
  const contentHeight = resolveModelMenuContentHeight(itemCount, rowHeight, maxContentHeight);
  if (availablePanelHeight && availablePanelHeight > 0) {
    const availableListHeight = Math.max(rowHeight, Math.floor(availablePanelHeight - chromeHeight));
    return `${Math.min(contentHeight, availableListHeight)}px`;
  }
  return `min(${contentHeight}px, max(${rowHeight}px, calc(var(--radix-popover-content-available-height, calc(100vh - 96px)) - ${chromeHeight}px)))`;
}

function resolveAdaptiveMenuWidthValue(labels: string[], minWidth: number, maxWidth: number, viewportWidth?: number): number {
  const longestLabelLength = labels.reduce((maxLength, label) => Math.max(maxLength, label.length), 0);
  const contentWidth = longestLabelLength * MODEL_MENU_TEXT_WIDTH_UNIT + MODEL_MENU_CONTENT_GAP_WIDTH;
  const preferredWidth = Math.min(Math.max(contentWidth, minWidth), maxWidth);
  if (viewportWidth && viewportWidth > 0) {
    return Math.min(preferredWidth, Math.max(0, viewportWidth - MODEL_MENU_VIEWPORT_GUTTER));
  }
  return preferredWidth;
}

function resolveAdaptiveMenuWidth(labels: string[], minWidth: number, maxWidth: number): string {
  const preferredWidth = resolveAdaptiveMenuWidthValue(labels, minWidth, maxWidth);
  return `min(${preferredWidth}px, calc(100vw - ${MODEL_MENU_VIEWPORT_GUTTER}px))`;
}

function resolveVendorGroups(modelOptions: ChatModelOption[]) {
  const groupMap = new Map<string, ChatModelOption[]>();
  for (const item of modelOptions) {
    const identity = resolveModelIdentity({
      code: item.platformModelName,
      vendor: item.vendor,
      icon: item.icon,
    });
    const group = groupMap.get(identity.vendorKey) ?? [];
    group.push(item);
    groupMap.set(identity.vendorKey, group);
  }

  return Array.from(groupMap.entries()).map(([vendor, items]) => ({
    vendor,
    label: resolveModelIdentity({ vendor }).vendorLabel,
    icon: resolveModelIdentity({ vendor }).vendorIcon,
    items,
  }));
}

function resolveDesktopModelPanelLayout({
  activeVendorRowRect,
  itemCount,
  key,
  menuRect,
  preferredWidth,
  viewportHeight,
  viewportWidth,
}: {
  activeVendorRowRect?: DOMRect;
  itemCount: number;
  key: number;
  menuRect: DOMRect;
  preferredWidth: number;
  viewportHeight: number;
  viewportWidth: number;
}): FloatingModelPanelLayout {
  const width = Math.min(preferredWidth, Math.max(0, viewportWidth - MODEL_MENU_COLLISION_GUTTER * 2));
  const panelChromeHeight = MODEL_MENU_MODEL_PANEL_CHROME_HEIGHT;
  const contentHeight = resolveModelMenuContentHeight(
    itemCount,
    MODEL_MENU_MODEL_ROW_HEIGHT,
    MODEL_MENU_MODEL_PANEL_MAX_HEIGHT,
  );
  const maxListHeight = Math.max(
    MODEL_MENU_MODEL_ROW_HEIGHT,
    viewportHeight - MODEL_MENU_SAFE_INSET - MODEL_MENU_COLLISION_GUTTER - panelChromeHeight,
  );
  const initialListHeight = Math.min(contentHeight, maxListHeight);
  const initialPanelHeight = panelChromeHeight + initialListHeight;
  const preferredY = activeVendorRowRect
    ? activeVendorRowRect.top
    : menuRect.top + initialPanelHeight <= viewportHeight - MODEL_MENU_COLLISION_GUTTER
      ? menuRect.top
      : menuRect.bottom - initialPanelHeight;
  const y = Math.min(
    Math.max(preferredY, MODEL_MENU_SAFE_INSET),
    Math.max(MODEL_MENU_SAFE_INSET, viewportHeight - initialPanelHeight - MODEL_MENU_COLLISION_GUTTER),
  );
  const listMaxHeight = Math.min(
    contentHeight,
    Math.max(
      MODEL_MENU_MODEL_ROW_HEIGHT,
      Math.min(maxListHeight, viewportHeight - y - MODEL_MENU_COLLISION_GUTTER - panelChromeHeight),
    ),
  );
  const rightX = menuRect.right + MODEL_MENU_PANEL_GAP;
  const leftX = menuRect.left - MODEL_MENU_PANEL_GAP - width;
  const rightFits = rightX + width <= viewportWidth - MODEL_MENU_COLLISION_GUTTER;
  const leftFits = leftX >= MODEL_MENU_COLLISION_GUTTER;
  const preferredX = rightFits || !leftFits ? rightX : leftX;
  const x = Math.min(
    Math.max(preferredX, MODEL_MENU_COLLISION_GUTTER),
    Math.max(MODEL_MENU_COLLISION_GUTTER, viewportWidth - width - MODEL_MENU_COLLISION_GUTTER),
  );

  return { key, x, y, width, listMaxHeight };
}

function ChatModelIdentity({
  model,
  density = "default",
}: {
  model: ChatModelOption;
  density?: "default" | "compact";
}) {
  const platformModelName = model.platformModelName.trim();
  const identity = React.useMemo(
    () =>
      resolveModelIdentity({
        code: model.platformModelName,
        vendor: model.vendor,
        icon: model.icon,
      }),
    [model.icon, model.platformModelName, model.vendor],
  );
  const iconURL = React.useMemo(() => resolveLobeHubIconURL(identity.modelIcon), [identity.modelIcon]);
  const compact = density === "compact";

  return (
    <div className={cn("flex min-w-0 items-center", compact ? "gap-2" : "gap-2.5")}>
      <LobeHubIcon iconUrl={iconURL} label={platformModelName} />
      <div className="min-w-0 flex-1 overflow-hidden">
        <div className={cn("flex items-center", compact ? "gap-1" : "gap-1.5")}>
          <p
            className={cn(
              "truncate font-medium text-foreground",
              compact ? "text-[12.5px] leading-4" : "text-[13px] leading-4.5",
            )}
          >
            {platformModelName}
          </p>
        </div>
      </div>
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
  maxHeight,
  children,
}: {
  maxHeight: string;
  children: React.ReactNode;
}) {
  const viewportRef = React.useRef<HTMLDivElement | null>(null);
  const [hasMoreAbove, setHasMoreAbove] = React.useState(false);
  const [hasMoreBelow, setHasMoreBelow] = React.useState(false);

  const updateScrollHints = React.useCallback(() => {
    const viewport = viewportRef.current;
    if (!viewport) {
      setHasMoreAbove(false);
      setHasMoreBelow(false);
      return;
    }
    const remaining = viewport.scrollHeight - viewport.clientHeight - viewport.scrollTop;
    setHasMoreAbove(viewport.scrollTop > MODEL_MENU_SCROLL_MORE_THRESHOLD);
    setHasMoreBelow(remaining > MODEL_MENU_SCROLL_MORE_THRESHOLD);
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
  }, [children, maxHeight, updateScrollHints]);

  return (
    <div className="relative">
      <div
        ref={viewportRef}
        className="overflow-y-auto overscroll-contain pr-0 [-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
        style={{ maxHeight }}
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

function ModelPricingTooltipContent({
  platformModelName,
  protocols,
  pricing,
  labels,
}: {
  platformModelName: string;
  protocols: readonly string[];
  pricing: NonNullable<ChatModelOption["pricing"]>;
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
  };
}) {
  const cacheWriteLabel = cacheWritePricingLabel(protocols, labels.billingDisplay);
  const cacheWriteNote = cacheWritePricingNote(protocols, labels.billingDisplay);
  if (pricing.isFree) {
    return (
      <div className="flex flex-col gap-1">
        <span className={PRICING_TOOLTIP_TITLE_CLASS}>{labels.freeModel}</span>
        <span className={PRICING_TOOLTIP_BODY_CLASS}>{labels.freeModelDescription}</span>
      </div>
    );
  }

  if (pricing.mode === "tiered") {
    return (
      <PricingTable
        platformModelName={platformModelName}
        title={labels.tieredPricing}
        footerNote={cacheWriteNote}
        headerRow={["", ...pricing.tiers.map((tier) => formatTokenRange(tier.fromTokens, tier.upToTokens))]}
        bodyRows={[
          [labels.input, ...pricing.tiers.map((tier) => formatPricingUnitUSD(tier.inputUSDPerMTokens))],
          [labels.output, ...pricing.tiers.map((tier) => formatPricingUnitUSD(tier.outputUSDPerMTokens))],
          [labels.cacheRead, ...pricing.tiers.map((tier) => formatPricingUnitUSD(tier.cacheReadUSDPerMTokens))],
          [cacheWriteLabel, ...pricing.tiers.map((tier) => formatPricingUnitUSD(resolveCacheWritePricingUSD(protocols, tier.cacheWriteUSDPerMTokens)))],
        ]}
      />
    );
  }

  if (pricing.mode === "call") {
    return (
      <div className="flex flex-col gap-1">
        <span className={PRICING_TOOLTIP_TITLE_CLASS}>{labels.callPricing}</span>
        <PricingTooltipRow label={labels.perCall} value={`${formatPricingUnitUSD(pricing.callUSDPerCall)} / ${labels.callUnit}`} />
      </div>
    );
  }

  if (pricing.mode === "duration") {
    return (
      <div className="flex flex-col gap-1">
        <span className={PRICING_TOOLTIP_TITLE_CLASS}>{labels.durationPricing}</span>
        <PricingTooltipRow label={labels.perSecond} value={`${formatPricingUnitUSD(pricing.durationUSDPerSecond)} / ${labels.secondUnit}`} />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-1">
      <span className={PRICING_TOOLTIP_TITLE_CLASS}>{labels.tokenPricing}</span>
      <PricingTooltipRow label={labels.input} value={`${formatPricingUnitUSD(pricing.inputUSDPerMTokens)} / 1M tokens`} />
      <PricingTooltipRow label={labels.output} value={`${formatPricingUnitUSD(pricing.outputUSDPerMTokens)} / 1M tokens`} />
      <PricingTooltipRow label={labels.cacheRead} value={`${formatPricingUnitUSD(pricing.cacheReadUSDPerMTokens)} / 1M tokens`} />
      <PricingTooltipRow label={cacheWriteLabel} value={`${formatPricingUnitUSD(resolveCacheWritePricingUSD(protocols, pricing.cacheWriteUSDPerMTokens))} / 1M tokens`} />
      {cacheWriteNote ? <span className={cn(PRICING_TOOLTIP_BODY_CLASS, "block max-w-72 text-background/70")}>{cacheWriteNote}</span> : null}
    </div>
  );
}

function PricingTooltipRow({ label, value }: { label: string; value: string }) {
  return (
    <div className={cn("grid grid-cols-[minmax(5.5rem,max-content)_auto] items-baseline gap-5", PRICING_TOOLTIP_BODY_CLASS)}>
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
      <span className={PRICING_TOOLTIP_TITLE_CLASS}>{title}</span>
      <table className={cn("border-collapse text-left tabular-nums", PRICING_TOOLTIP_BODY_CLASS)}>
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
      {footerNote ? <span className={cn(PRICING_TOOLTIP_BODY_CLASS, "block text-background/70")}>{footerNote}</span> : null}
    </div>
  );
}

function formatPricingUSD(value: number): string {
  if (!Number.isFinite(value) || value <= 0) {
    return "$0";
  }
  return `$${value.toLocaleString("en-US", {
    minimumFractionDigits: 0,
    maximumFractionDigits: 6,
  })}`;
}

function formatPricingUnitUSD(value: number): string {
  if (!Number.isFinite(value) || value <= 0) {
    return "$0.00";
  }
  return `$${value.toLocaleString("en-US", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}`;
}

function formatTokenRange(fromTokens: number, upToTokens: number | null): string {
  if (!upToTokens || upToTokens <= 0) {
    return `${formatTokenQuantity(fromTokens)}～∞`;
  }
  return `${formatTokenQuantity(fromTokens)}～${formatTokenQuantity(upToTokens)}`;
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

function ChatModelMenuItem({
  model,
  selected,
  onSelect,
  pricingLabels,
  viewPricingLabel,
  pricingTooltipSide,
}: {
  model: ChatModelOption;
  selected: boolean;
  onSelect: () => void;
  pricingLabels: React.ComponentProps<typeof ModelPricingTooltipContent>["labels"];
  viewPricingLabel: string;
  pricingTooltipSide: "right";
}) {
  const platformModelName = model.platformModelName.trim();
  const identity = React.useMemo(
    () =>
      resolveModelIdentity({
        code: model.platformModelName,
        vendor: model.vendor,
        icon: model.icon,
      }),
    [model.icon, model.platformModelName, model.vendor],
  );
  const iconURL = React.useMemo(() => resolveLobeHubIconURL(identity.modelIcon), [identity.modelIcon]);

  return (
    <div
      data-selected={selected}
      className="group flex h-7 items-center rounded-md text-[11px] font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground focus-within:bg-accent focus-within:text-accent-foreground data-[selected=true]:bg-accent data-[selected=true]:text-accent-foreground"
    >
      <button
        type="button"
        className="flex h-7 min-w-0 flex-1 items-center gap-2 rounded-md bg-transparent py-0 pl-2 pr-1 text-left text-[11px] font-medium leading-none text-inherit outline-none"
        onClick={onSelect}
      >
        <LobeHubIcon iconUrl={iconURL} label={platformModelName} />
        <span className="min-w-0 flex-1 truncate leading-4">
          {platformModelName}
        </span>
        <span className="flex size-3 shrink-0 items-center justify-center">
          {selected ? <Check className="size-3 text-current" strokeWidth={1.7} /> : null}
        </span>
      </button>
      {model.pricing ? (
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-muted-foreground/70 transition-colors hover:text-current focus-visible:text-current focus-visible:outline-none group-hover:text-current group-focus-within:text-current group-data-[selected=true]:text-current"
              aria-label={viewPricingLabel}
            >
              <CircleDollarSign className="size-3.5" strokeWidth={1.8} />
            </button>
          </TooltipTrigger>
          <TooltipContent
            side={pricingTooltipSide}
            align="center"
            sideOffset={8}
            className="z-[80] max-w-[min(92vw,35rem)] text-left font-medium tabular-nums"
          >
            <ModelPricingTooltipContent
              platformModelName={model.platformModelName}
              protocols={model.protocols}
              pricing={model.pricing}
              labels={pricingLabels}
            />
          </TooltipContent>
        </Tooltip>
      ) : null}
    </div>
  );
}

export function ChatModelPicker({
  modelOptions,
  selectedPlatformModelName,
  loading,
  disabled,
  onModelChange,
}: ChatModelPickerProps) {
  const t = useTranslations("chat.modelPicker");
  const isMobile = useIsMobile();
  const [open, setOpen] = React.useState(false);
  const [activeVendorKey, setActiveVendorKey] = React.useState("");
  const [mobileVendorKey, setMobileVendorKey] = React.useState<string | null>(null);
  const [desktopModelPanelLayout, setDesktopModelPanelLayout] = React.useState<FloatingModelPanelLayout | null>(null);
  const [desktopModelPanelKey, setDesktopModelPanelKey] = React.useState(0);
  const desktopModelPanelKeyRef = React.useRef(0);
  const desktopPopoverContentRef = React.useRef<HTMLDivElement | null>(null);
  const desktopModelPanelRef = React.useRef<HTMLDivElement | null>(null);
  const selectedModel = React.useMemo(
    () => modelOptions.find((item) => item.platformModelName === selectedPlatformModelName) ?? null,
    [modelOptions, selectedPlatformModelName],
  );
  const selectedVendorKey = React.useMemo(() => {
    if (!selectedModel) {
      return "";
    }
    return resolveModelIdentity({
      code: selectedModel.platformModelName,
      vendor: selectedModel.vendor,
      icon: selectedModel.icon,
    }).vendorKey;
  }, [selectedModel]);
  const selectedVendorLabel = React.useMemo(() => {
    if (!selectedModel) {
      return "none";
    }
    return resolveModelIdentity({
      code: selectedModel.platformModelName,
      vendor: selectedModel.vendor,
      icon: selectedModel.icon,
    }).vendorLabel;
  }, [selectedModel]);
  const vendorGroups = React.useMemo(() => resolveVendorGroups(modelOptions), [modelOptions]);
  const vendorMenuMaxHeight = React.useMemo(
    () =>
      resolveModelMenuMaxHeight(
        vendorGroups.length,
        MODEL_MENU_VENDOR_ROW_HEIGHT,
        MODEL_MENU_HEADER_HEIGHT + MODEL_MENU_POPOVER_CHROME_HEIGHT + MODEL_MENU_SAFE_INSET,
      ),
    [vendorGroups.length],
  );
  const vendorMenuWidth = React.useMemo(
    () => resolveAdaptiveMenuWidth(vendorGroups.map((group) => group.label), 190, 260),
    [vendorGroups],
  );
  const activeDesktopVendorKey = activeVendorKey || selectedVendorKey || vendorGroups[0]?.vendor || "";
  const activeDesktopVendorGroup = React.useMemo(
    () => vendorGroups.find((group) => group.vendor === activeDesktopVendorKey) ?? vendorGroups[0] ?? null,
    [activeDesktopVendorKey, vendorGroups],
  );
  const desktopModelMenuMaxHeight = React.useMemo(
    () => {
      if (desktopModelPanelLayout) {
        return `${desktopModelPanelLayout.listMaxHeight}px`;
      }
      return resolveModelMenuMaxHeight(
        activeDesktopVendorGroup?.items.length ?? 0,
        MODEL_MENU_MODEL_ROW_HEIGHT,
        MODEL_MENU_MODEL_PANEL_CHROME_HEIGHT,
        null,
        MODEL_MENU_MODEL_PANEL_MAX_HEIGHT,
      );
    },
    [activeDesktopVendorGroup, desktopModelPanelLayout],
  );
  const desktopModelMenuWidthValue = React.useMemo(
    () =>
      activeDesktopVendorGroup
        ? resolveAdaptiveMenuWidthValue(
            activeDesktopVendorGroup.items.map((item) => item.platformModelName),
            232,
            420,
            typeof window === "undefined" ? undefined : window.innerWidth,
          )
        : 232,
    [activeDesktopVendorGroup],
  );
  const mobileVendorGroup = React.useMemo(
    () => vendorGroups.find((group) => group.vendor === mobileVendorKey) ?? null,
    [mobileVendorKey, vendorGroups],
  );
  const mobileMenuWidth = React.useMemo(
    () =>
      mobileVendorGroup
        ? resolveAdaptiveMenuWidth(mobileVendorGroup.items.map((item) => item.platformModelName), 232, 420)
        : resolveAdaptiveMenuWidth(vendorGroups.map((group) => group.label), 190, 320),
    [mobileVendorGroup, vendorGroups],
  );
  const mobileVendorMenuMaxHeight = React.useMemo(
    () =>
      resolveModelMenuMaxHeight(
        mobileVendorGroup ? mobileVendorGroup.items.length : vendorGroups.length,
        MODEL_MENU_VENDOR_ROW_HEIGHT,
        MODEL_MENU_HEADER_HEIGHT + MODEL_MENU_POPOVER_CHROME_HEIGHT + MODEL_MENU_SAFE_INSET,
      ),
    [mobileVendorGroup, vendorGroups.length],
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
      billingDisplay: {
        cacheWrite: t("cacheWrite"),
        cacheWrite5m: t("cacheWrite5m"),
        cacheWrite1h: t("cacheWrite1h"),
        cacheWrite5m1h: t("cacheWrite5m1h"),
        claudeCacheWriteMixedNote: (multiplier: string) => t("claudeCacheWriteMixedNote", { multiplier }),
        claudeCacheWriteNote: (timeout: "5m" | "1h", multiplier: string) => t("claudeCacheWriteNote", { timeout, multiplier }),
        claudeFastModeNote: (multiplier: string) => t("claudeFastModeNote", { multiplier }),
        openaiServiceTierNote: (tier: string, multiplier: string) => t("openaiServiceTierNote", { tier, multiplier }),
        cacheWritePricingLabel: t("cacheWritePricingLabel"),
        cacheWritePricingNote: t("cacheWritePricingNote"),
      },
    }),
    [t],
  );

  React.useEffect(() => {
    if (!open || !isMobile) {
      setMobileVendorKey(null);
    }
  }, [isMobile, open]);

  const resetDesktopModelPanelLayout = React.useCallback(() => {
    const nextKey = desktopModelPanelKeyRef.current + 1;
    desktopModelPanelKeyRef.current = nextKey;
    setDesktopModelPanelKey(nextKey);
    setDesktopModelPanelLayout(null);
    return nextKey;
  }, []);

  const handleOpenChange = React.useCallback(
    (nextOpen: boolean) => {
      resetDesktopModelPanelLayout();
      if (nextOpen) {
        setActiveVendorKey(selectedVendorKey || vendorGroups[0]?.vendor || "");
      }
      setOpen(nextOpen);
    },
    [resetDesktopModelPanelLayout, selectedVendorKey, vendorGroups],
  );

  const updateDesktopModelPanelLayout = React.useCallback((layoutKey: number) => {
    if (!open || isMobile || typeof window === "undefined") {
      return;
    }
    if (desktopModelPanelKeyRef.current !== layoutKey) {
      return;
    }
    const menu = desktopPopoverContentRef.current;
    if (!menu || !activeDesktopVendorGroup) {
      return;
    }

    const menuRect = menu.getBoundingClientRect();
    if (menuRect.width <= 0 || menuRect.height <= 0) {
      return;
    }
    const activeVendorRow = menu.querySelector<HTMLElement>('[data-active-vendor="true"]');
    setDesktopModelPanelLayout(resolveDesktopModelPanelLayout({
      activeVendorRowRect: activeVendorRow?.getBoundingClientRect(),
      itemCount: activeDesktopVendorGroup.items.length,
      key: layoutKey,
      menuRect,
      preferredWidth: desktopModelMenuWidthValue,
      viewportHeight: window.innerHeight,
      viewportWidth: window.innerWidth,
    }));
  }, [activeDesktopVendorGroup, desktopModelMenuWidthValue, isMobile, open]);

  React.useLayoutEffect(() => {
    if (!open || isMobile) {
      setDesktopModelPanelLayout(null);
      return;
    }

    const layoutKey = desktopModelPanelKey;
    let frameID = window.requestAnimationFrame(() => {
      frameID = window.requestAnimationFrame(() => {
        updateDesktopModelPanelLayout(layoutKey);
      });
    });
    const update = () => updateDesktopModelPanelLayout(layoutKey);
    window.addEventListener("resize", update);
    window.addEventListener("scroll", update, true);
    return () => {
      window.cancelAnimationFrame(frameID);
      window.removeEventListener("resize", update);
      window.removeEventListener("scroll", update, true);
    };
  }, [
    activeDesktopVendorGroup,
    desktopModelMenuWidthValue,
    desktopModelPanelKey,
    isMobile,
    open,
    updateDesktopModelPanelLayout,
  ]);

  const closeMenu = React.useCallback(() => {
    handleOpenChange(false);
  }, [handleOpenChange]);

  const selectDesktopVendor = React.useCallback((vendor: string) => {
    if (vendor === activeDesktopVendorKey) {
      return;
    }
    resetDesktopModelPanelLayout();
    setActiveVendorKey(vendor);
  }, [activeDesktopVendorKey, resetDesktopModelPanelLayout]);

  return (
    <>
      <div className="min-w-0 max-w-[min(320px,100%)] shrink">
      <Popover open={open} onOpenChange={handleOpenChange}>
        <PopoverTrigger asChild>
          <InputGroupButton
            id="chat-model-menu-trigger"
            type="button"
            variant="ghost"
            size="sm"
            className="w-full min-w-0 max-w-[min(320px,100%)] rounded-lg px-1.5 hover:bg-accent focus-visible:bg-accent data-[state=open]:bg-accent sm:px-2"
            disabled={disabled || loading || modelOptions.length === 0}
            aria-label={t("selectModel")}
          >
            {loading ? (
              <ChatModelTriggerSkeleton />
            ) : selectedModel ? (
              <ChatModelIdentity model={selectedModel} density="compact" />
            ) : selectedPlatformModelName.trim() ? (
              <span className="truncate text-[12px] font-medium text-foreground">
                {selectedPlatformModelName}
              </span>
            ) : (
              <span className="truncate text-[12px] font-medium text-muted-foreground">
                {t("selectModel")}
              </span>
            )}
          </InputGroupButton>
        </PopoverTrigger>
        <PopoverContent
          align="end"
          sideOffset={8}
          className="relative overflow-visible rounded-xl p-1.5"
          ref={desktopPopoverContentRef}
          style={{
            width: isMobile ? mobileMenuWidth : vendorMenuWidth,
            maxHeight: "var(--radix-popover-content-available-height)",
          }}
          onInteractOutside={(event) => {
            const target = event.target;
            if (target instanceof Node && desktopModelPanelRef.current?.contains(target)) {
              event.preventDefault();
            }
          }}
        >
          {isMobile ? (
            <>
              <div className="flex h-7 items-center justify-between gap-2 px-2">
                {mobileVendorGroup ? (
                  <button
                    type="button"
                    className="-ml-1.5 flex h-7 min-w-0 items-center gap-0.5 rounded-md px-0.5 text-[11px] font-medium text-muted-foreground outline-none transition-colors hover:bg-accent hover:text-foreground focus-visible:bg-accent focus-visible:text-foreground"
                    onClick={() => setMobileVendorKey(null)}
                  >
                    <ChevronLeft className="size-3.5" strokeWidth={1.8} />
                    <span>{t("vendor")}</span>
                  </button>
                ) : (
                  <span className="text-[11px] font-medium text-foreground">{t("vendor")}</span>
                )}
                <span className="min-w-0 truncate text-right text-[10px] font-medium text-muted-foreground">
                  {mobileVendorGroup ? mobileVendorGroup.label : selectedVendorLabel}
                </span>
              </div>
              <ModelMenuScrollContainer maxHeight={mobileVendorMenuMaxHeight}>
                {mobileVendorGroup ? (
                  <div className="flex flex-col gap-0.5">
                    {mobileVendorGroup.items.map((item) => (
                      <ChatModelMenuItem
                        key={item.platformModelName}
                        model={item}
                        selected={item.platformModelName === selectedPlatformModelName}
                        onSelect={() => {
                          onModelChange(item.platformModelName);
                          closeMenu();
                        }}
                        pricingLabels={pricingLabels}
                        viewPricingLabel={t("viewPricing")}
                        pricingTooltipSide="right"
                      />
                    ))}
                  </div>
                ) : (
                  <div className="flex flex-col gap-0.5">
                    {vendorGroups.map((group) => {
                      const selectedVendor = group.vendor === selectedVendorKey;
                      const vendorIconURL = resolveLobeHubIconURL(group.icon);
                      return (
                        <button
                          type="button"
                          key={group.vendor}
                          className={cn(
                            "flex h-7 w-full items-center justify-between gap-2 rounded-md px-2 py-0 text-left text-[11px] font-medium outline-none transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:bg-accent focus-visible:text-accent-foreground",
                            selectedVendor ? "bg-accent text-accent-foreground" : "text-muted-foreground",
                          )}
                          onClick={() => {
                            setMobileVendorKey(group.vendor);
                          }}
                        >
                          <LobeHubIcon iconUrl={vendorIconURL} label={group.label} />
                          <span className="min-w-0 flex-1 truncate font-medium">{group.label}</span>
                          <span className="shrink-0 text-[10px] tabular-nums text-muted-foreground/80">
                            {group.items.length}
                          </span>
                        </button>
                      );
                    })}
                  </div>
                )}
              </ModelMenuScrollContainer>
            </>
          ) : (
            <div className="relative">
              <div className="flex h-7 items-center justify-between gap-3 px-2">
                <span className="text-[11px] font-medium text-foreground">{t("vendor")}</span>
                <span className="truncate text-[10px] font-medium text-muted-foreground">
                  {selectedVendorLabel}
                </span>
              </div>
              <ModelMenuScrollContainer maxHeight={vendorMenuMaxHeight}>
                <div className="flex flex-col gap-0.5">
                  {vendorGroups.map((group) => {
                    const selectedVendor = group.vendor === selectedVendorKey;
                    const activeVendor = group.vendor === activeDesktopVendorGroup?.vendor;
                    const vendorIconURL = resolveLobeHubIconURL(group.icon);
                    return (
                      <button
                        type="button"
                        key={group.vendor}
                        data-active-vendor={activeVendor ? "true" : undefined}
                        className={cn(
                          "flex h-7 w-full items-center gap-2 rounded-md px-2 py-0 text-left text-[11px] font-medium outline-none transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:bg-accent focus-visible:text-accent-foreground",
                          activeVendor ? "bg-accent text-accent-foreground" : "text-muted-foreground",
                          selectedVendor && !activeVendor ? "text-foreground" : null,
                        )}
                        onMouseEnter={() => selectDesktopVendor(group.vendor)}
                        onFocus={() => selectDesktopVendor(group.vendor)}
                        onClick={() => selectDesktopVendor(group.vendor)}
                      >
                        <LobeHubIcon iconUrl={vendorIconURL} label={group.label} />
                        <span className="min-w-0 flex-1 truncate font-medium">{group.label}</span>
                        <span className="shrink-0 text-[10px] tabular-nums text-muted-foreground/80">
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
        </PopoverContent>
      </Popover>
      </div>
      {open
      && !isMobile
      && activeDesktopVendorGroup
      && desktopModelPanelLayout
      && desktopModelPanelLayout.key === desktopModelPanelKey
      && typeof document !== "undefined"
        ? createPortal(
          <div
            ref={desktopModelPanelRef}
            className="fixed z-[60] rounded-xl border-[0.5px] border-border bg-popover p-1.5 text-popover-foreground shadow-xs"
            style={{
              left: desktopModelPanelLayout.x,
              top: desktopModelPanelLayout.y,
              width: desktopModelPanelLayout.width,
            }}
          >
            <ModelMenuScrollContainer maxHeight={desktopModelMenuMaxHeight}>
              <div className="flex flex-col gap-0.5">
                {activeDesktopVendorGroup.items.map((item) => (
                  <ChatModelMenuItem
                    key={item.platformModelName}
                    model={item}
                    selected={item.platformModelName === selectedPlatformModelName}
                    onSelect={() => {
                      onModelChange(item.platformModelName);
                      closeMenu();
                    }}
                    pricingLabels={pricingLabels}
                    viewPricingLabel={t("viewPricing")}
                    pricingTooltipSide="right"
                  />
                ))}
              </div>
            </ModelMenuScrollContainer>
          </div>,
          document.body,
        )
      : null}
    </>
  );
}
