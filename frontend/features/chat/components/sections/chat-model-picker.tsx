"use client";

import * as React from "react";
import { Check, ChevronDown, ChevronLeft, CircleDollarSign } from "lucide-react";
import { useTranslations } from "next-intl";

import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { InputGroupButton } from "@/components/ui/input-group";
import type { ChatModelOption } from "@/features/chat/types/chat-runtime";
import { useIsMobile } from "@/shared/hooks/use-mobile";
import { LobeHubIcon } from "@/shared/components/lobehub-icon";
import { cacheWritePricingLabel, cacheWritePricingNote, resolveCacheWritePricingUSD } from "@/shared/lib/billing-display";
import type { BillingDisplayLabels } from "@/shared/lib/billing-display";
import { resolveLobeHubIconURL, resolveModelIdentity } from "@/shared/lib/model-identity";
import { cn } from "@/lib/utils";

const MODEL_MENU_MAX_HEIGHT = 320;
const MODEL_MENU_VENDOR_ROW_HEIGHT = 28;
const MODEL_MENU_MODEL_ROW_HEIGHT = 28;
const MODEL_MENU_MODEL_HEADER_HEIGHT = 28;
const MODEL_MENU_TEXT_WIDTH_UNIT = 7;
const MODEL_MENU_CONTENT_GAP_WIDTH = 56;
const MODEL_MENU_VIEWPORT_GUTTER = 24;
const PRICING_TOOLTIP_TITLE_CLASS = "font-sans text-xs font-medium leading-4 text-background";
const PRICING_TOOLTIP_BODY_CLASS = "font-sans text-[11px] leading-4 text-background/80";

type ChatModelPickerProps = {
  modelOptions: ChatModelOption[];
  selectedPlatformModelName: string;
  loading: boolean;
  disabled: boolean;
  onModelChange: (platformModelName: string) => void;
};

function resolveModelMenuMaxHeight(itemCount: number, rowHeight: number, paddingHeight: number): string {
  const contentHeight = itemCount * rowHeight + paddingHeight;
  return `min(${Math.min(contentHeight, MODEL_MENU_MAX_HEIGHT)}px, var(--radix-dropdown-menu-content-available-height))`;
}

function resolveAdaptiveMenuWidth(labels: string[], minWidth: number, maxWidth: number): string {
  const longestLabelLength = labels.reduce((maxLength, label) => Math.max(maxLength, label.length), 0);
  const contentWidth = longestLabelLength * MODEL_MENU_TEXT_WIDTH_UNIT + MODEL_MENU_CONTENT_GAP_WIDTH;
  const preferredWidth = Math.min(Math.max(contentWidth, minWidth), maxWidth);
  return `min(${preferredWidth}px, calc(100vw - ${MODEL_MENU_VIEWPORT_GUTTER}px))`;
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
  const [hasMoreBelow, setHasMoreBelow] = React.useState(false);

  const updateHasMoreBelow = React.useCallback(() => {
    const viewport = viewportRef.current;
    if (!viewport) {
      setHasMoreBelow(false);
      return;
    }
    const remaining = viewport.scrollHeight - viewport.clientHeight - viewport.scrollTop;
    setHasMoreBelow(remaining > 1);
  }, []);

  React.useLayoutEffect(() => {
    updateHasMoreBelow();
    const viewport = viewportRef.current;
    if (!viewport || typeof ResizeObserver === "undefined") {
      return;
    }

    const observer = new ResizeObserver(updateHasMoreBelow);
    observer.observe(viewport);
    if (viewport.firstElementChild) {
      observer.observe(viewport.firstElementChild);
    }
    return () => observer.disconnect();
  }, [children, maxHeight, updateHasMoreBelow]);

  return (
    <div className="relative">
      <div
        ref={viewportRef}
        className="overflow-y-auto overscroll-contain pr-0 [-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
        style={{ maxHeight }}
        onScroll={updateHasMoreBelow}
      >
        {children}
      </div>
      {hasMoreBelow ? (
        <div className="pointer-events-none absolute inset-x-0 bottom-0 flex h-5 items-end justify-center bg-gradient-to-t from-popover via-popover/85 to-transparent pb-0.5">
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
    <div className={cn("grid grid-cols-[3rem_auto] items-baseline gap-5", PRICING_TOOLTIP_BODY_CLASS)}>
      <span className="text-left">{label}</span>
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
}: {
  model: ChatModelOption;
  selected: boolean;
  onSelect: () => void;
  pricingLabels: React.ComponentProps<typeof ModelPricingTooltipContent>["labels"];
  viewPricingLabel: string;
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
    <DropdownMenuItem
      data-selected={selected}
      className="group h-7 justify-between gap-2 rounded-md px-2 py-0 text-[11px] font-medium text-muted-foreground focus:bg-accent focus:text-accent-foreground data-[selected=true]:bg-accent data-[selected=true]:text-accent-foreground"
      onSelect={onSelect}
    >
      <LobeHubIcon iconUrl={iconURL} label={platformModelName} />
      <span className="min-w-0 flex-1 truncate">
        {platformModelName}
      </span>
      <span className="flex size-3 shrink-0 items-center justify-center">
        {selected ? <Check className="size-3 text-current" strokeWidth={1.7} /> : null}
      </span>
      {model.pricing ? (
        <Tooltip>
          <TooltipTrigger asChild>
            <span
              className="flex size-4 shrink-0 items-center justify-center rounded-sm text-muted-foreground/70 transition-colors group-focus:text-current group-data-[selected=true]:text-current"
              aria-label={viewPricingLabel}
            >
              <CircleDollarSign className="size-3.5" strokeWidth={1.8} />
            </span>
          </TooltipTrigger>
          <TooltipContent
            side="right"
            align="center"
            sideOffset={8}
            className="max-w-[560px] text-left font-medium tabular-nums"
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
    </DropdownMenuItem>
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
  const [mobileVendorKey, setMobileVendorKey] = React.useState<string | null>(null);
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
  const vendorGroups = React.useMemo(() => {
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
  }, [modelOptions]);
  const vendorMenuMaxHeight = React.useMemo(
    () => resolveModelMenuMaxHeight(vendorGroups.length, MODEL_MENU_VENDOR_ROW_HEIGHT, 12),
    [vendorGroups.length],
  );
  const vendorMenuWidth = React.useMemo(
    () => resolveAdaptiveMenuWidth(vendorGroups.map((group) => group.label), 190, 260),
    [vendorGroups],
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
        mobileVendorGroup ? MODEL_MENU_MODEL_ROW_HEIGHT : MODEL_MENU_VENDOR_ROW_HEIGHT,
        56,
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

  return (
    <DropdownMenu modal={false} open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger asChild>
        <InputGroupButton
          id="chat-model-menu-trigger"
          type="button"
          variant="ghost"
          size="sm"
          className="min-w-0 max-w-[min(320px,100%)] rounded-lg px-2 hover:bg-accent focus-visible:bg-accent data-[state=open]:bg-accent"
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
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        collisionPadding={12}
        sideOffset={8}
        className="p-1.5"
        style={{ width: isMobile ? mobileMenuWidth : vendorMenuWidth }}
      >
        {isMobile ? (
          <>
            <div className="flex h-8 items-center justify-between gap-2 px-1.5">
              {mobileVendorGroup ? (
                <button
                  type="button"
                  className="flex min-w-0 items-center gap-1.5 rounded-md px-1.5 py-1 text-[11px] font-medium text-muted-foreground outline-none transition-colors hover:bg-accent hover:text-foreground focus-visible:bg-accent focus-visible:text-foreground"
                  onClick={() => setMobileVendorKey(null)}
                >
                  <ChevronLeft className="size-3.5" strokeWidth={1.8} />
                  <span>{t("vendor")}</span>
                </button>
              ) : (
                <span className="px-1.5 text-[11px] font-medium text-foreground">{t("vendor")}</span>
              )}
              <span className="min-w-0 truncate px-1.5 text-right text-[10px] font-medium text-muted-foreground">
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
                        setOpen(false);
                      }}
                      pricingLabels={pricingLabels}
                      viewPricingLabel={t("viewPricing")}
                    />
                  ))}
                </div>
              ) : (
                <div className="flex flex-col gap-0.5">
                  {vendorGroups.map((group) => {
                    const selectedVendor = group.vendor === selectedVendorKey;
                    const vendorIconURL = resolveLobeHubIconURL(group.icon);
                    return (
                      <DropdownMenuItem
                        key={group.vendor}
                        className={cn(
                          "h-8 justify-between gap-2 rounded-md px-2 py-0 text-[12px] font-medium",
                          selectedVendor ? "bg-accent text-accent-foreground" : "text-muted-foreground",
                        )}
                        onSelect={(event) => {
                          event.preventDefault();
                          setMobileVendorKey(group.vendor);
                        }}
                      >
                        <LobeHubIcon iconUrl={vendorIconURL} label={group.label} />
                        <span className="min-w-0 flex-1 truncate font-medium">{group.label}</span>
                        <span className="shrink-0 text-[10px] tabular-nums text-muted-foreground/80">
                          {group.items.length}
                        </span>
                      </DropdownMenuItem>
                    );
                  })}
                </div>
              )}
            </ModelMenuScrollContainer>
          </>
        ) : (
          <>
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
                  const vendorIconURL = resolveLobeHubIconURL(group.icon);
                  const modelMenuMaxHeight = resolveModelMenuMaxHeight(
                    group.items.length,
                    MODEL_MENU_MODEL_ROW_HEIGHT,
                    MODEL_MENU_MODEL_HEADER_HEIGHT + 12,
                  );
                  const modelMenuWidth = resolveAdaptiveMenuWidth(
                    group.items.map((item) => item.platformModelName),
                    232,
                    420,
                  );
                  return (
                    <DropdownMenuSub key={group.vendor}>
                      <DropdownMenuSubTrigger
                        className={cn(
                          "h-7 gap-2 rounded-md px-2 py-0 text-[11px] font-medium hover:bg-accent focus:bg-accent data-[highlighted]:bg-accent data-[state=open]:bg-accent",
                          "[&>svg:last-child]:!ml-1 [&>svg:last-child]:!size-3.5 [&>svg:last-child]:!text-muted-foreground/65",
                          "data-[highlighted]:[&>svg]:!translate-x-0 data-[highlighted]:[&>svg]:!scale-100 data-[state=open]:[&>svg]:!scale-100",
                          selectedVendor ? "bg-accent text-accent-foreground" : "text-muted-foreground",
                        )}
                        onClick={(event) => {
                          event.preventDefault();
                        }}
                        onSelect={(event) => {
                          event.preventDefault();
                        }}
                      >
                        <LobeHubIcon iconUrl={vendorIconURL} label={group.label} />
                        <span className="min-w-0 flex-1 truncate font-medium">{group.label}</span>
                        <span className="shrink-0 text-[10px] tabular-nums text-muted-foreground/80">
                          {group.items.length}
                        </span>
                      </DropdownMenuSubTrigger>
                      <DropdownMenuSubContent
                        collisionPadding={12}
                        sideOffset={6}
                        className="p-1.5"
                        style={{ width: modelMenuWidth }}
                      >
                        <div className="flex h-7 items-center justify-between gap-3 px-2">
                          <span className="flex min-w-0 items-center gap-2">
                            <LobeHubIcon iconUrl={vendorIconURL} label={group.label} />
                            <span className="truncate text-[11px] font-medium text-foreground">
                              {group.label}
                            </span>
                          </span>
                          <span className="text-[10px] font-medium text-muted-foreground">
                            {group.items.length}
                          </span>
                        </div>
                        <ModelMenuScrollContainer maxHeight={modelMenuMaxHeight}>
                          <div className="flex flex-col gap-0.5">
                            {group.items.map((item) => (
                              <ChatModelMenuItem
                                key={item.platformModelName}
                                model={item}
                                selected={item.platformModelName === selectedPlatformModelName}
                              onSelect={() => {
                                onModelChange(item.platformModelName);
                                setOpen(false);
                              }}
                              pricingLabels={pricingLabels}
                              viewPricingLabel={t("viewPricing")}
                            />
                            ))}
                          </div>
                        </ModelMenuScrollContainer>
                      </DropdownMenuSubContent>
                    </DropdownMenuSub>
                  );
                })}
              </div>
            </ModelMenuScrollContainer>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
