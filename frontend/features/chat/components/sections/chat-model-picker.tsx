"use client";

import * as React from "react";
import { Check, ChevronDown, ChevronLeft, ChevronRight, CircleDollarSign, TicketSlash } from "lucide-react";
import { useTranslations } from "next-intl";

import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { InputGroupButton } from "@/components/ui/input-group";
import type { ChatModelOption } from "@/features/chat/types/chat-runtime";
import { useIsMobile } from "@/shared/hooks/use-mobile";
import { LobeHubIcon } from "@/shared/components/lobehub-icon";
import {
  cacheWritePricingLabel,
  cacheWritePricingNote,
  formatBillingDisplayUnitPriceFromUSD,
  resolveCacheWritePricingUSD,
} from "@/shared/lib/billing-display";
import type { BillingDisplayCurrency, BillingDisplayLabels, BillingDisplayOptions } from "@/shared/lib/billing-display";
import { resolveLobeHubIconURL, resolveModelIdentity, resolveVendorIdentity } from "@/shared/lib/model-identity";
import { cn } from "@/lib/utils";

type ChatModelPickerProps = {
  modelOptions: ChatModelOption[];
  billingDisplayCurrency: BillingDisplayCurrency;
  billingDisplayUsdToCnyRate: number | null;
  selectedPlatformModelName: string;
  loading: boolean;
  disabled: boolean;
  onModelCatalogRefresh?: () => void | Promise<void>;
  onModelChange: (platformModelName: string) => void;
};

function resolveVendorGroups(modelOptions: ChatModelOption[]) {
  const groupMap = new Map<string, ChatModelOption[]>();
  for (const item of modelOptions) {
    const identity = resolveVendorIdentity(item.vendor);
    const group = groupMap.get(identity.vendorKey) ?? [];
    group.push(item);
    groupMap.set(identity.vendorKey, group);
  }

  return Array.from(groupMap.entries()).map(([vendor, items]) => {
    const identity = resolveVendorIdentity(vendor);
    return {
      vendor,
      label: identity.vendorLabel,
      icon: identity.vendorIcon,
      items,
    };
  });
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
  children,
}: {
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
  }, [children, updateScrollHints]);

  return (
    <div className="relative">
      <div
        ref={viewportRef}
        className="max-h-[min(20rem,var(--radix-popover-content-available-height))] overflow-y-auto overscroll-contain pr-0 [-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
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
  billingDisplay,
  labels,
}: {
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
  };
}) {
  const cacheWriteLabel = cacheWritePricingLabel(protocols, labels.billingDisplay);
  const cacheWriteNote = cacheWritePricingNote(protocols, labels.billingDisplay);
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
        headerRow={["", ...pricing.tiers.map((tier) => formatTokenRange(tier.fromTokens, tier.upToTokens))]}
        bodyRows={[
          [labels.input, ...pricing.tiers.map((tier) => formatPricingUnitUSD(tier.inputUSDPerMTokens, billingDisplay))],
          [labels.output, ...pricing.tiers.map((tier) => formatPricingUnitUSD(tier.outputUSDPerMTokens, billingDisplay))],
          [labels.cacheRead, ...pricing.tiers.map((tier) => formatPricingUnitUSD(tier.cacheReadUSDPerMTokens, billingDisplay))],
          [cacheWriteLabel, ...pricing.tiers.map((tier) => formatPricingUnitUSD(resolveCacheWritePricingUSD(protocols, tier.cacheWriteUSDPerMTokens), billingDisplay))],
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
      <PricingTooltipRow label={cacheWriteLabel} value={`${formatPricingUnitUSD(resolveCacheWritePricingUSD(protocols, pricing.cacheWriteUSDPerMTokens), billingDisplay)} / 1M tokens`} />
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
  billingDisplay,
  pricingLabels,
  viewPricingLabel,
  pricingTooltipSide,
}: {
  model: ChatModelOption;
  selected: boolean;
  onSelect: () => void;
  billingDisplay: BillingDisplayOptions;
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
              {model.pricing.isFree ? (
                <TicketSlash className="size-3.5" strokeWidth={1.8} />
              ) : (
                <CircleDollarSign className="size-3.5" strokeWidth={1.8} />
              )}
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
              billingDisplay={billingDisplay}
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
  billingDisplayCurrency,
  billingDisplayUsdToCnyRate,
  selectedPlatformModelName,
  loading,
  disabled,
  onModelCatalogRefresh,
  onModelChange,
}: ChatModelPickerProps) {
  const t = useTranslations("chat.modelPicker");
  const isMobile = useIsMobile();
  const [open, setOpen] = React.useState(false);
  const [activeVendorKey, setActiveVendorKey] = React.useState("");
  const [mobileVendorKey, setMobileVendorKey] = React.useState<string | null>(null);
  const selectedModel = React.useMemo(
    () => modelOptions.find((item) => item.platformModelName === selectedPlatformModelName) ?? null,
    [modelOptions, selectedPlatformModelName],
  );
  const selectedVendorKey = React.useMemo(() => {
    if (!selectedModel) {
      return "";
    }
    return resolveVendorIdentity(selectedModel.vendor).vendorKey;
  }, [selectedModel]);
  const selectedVendorLabel = React.useMemo(() => {
    if (!selectedModel) {
      return "none";
    }
    return resolveVendorIdentity(selectedModel.vendor).vendorLabel;
  }, [selectedModel]);
  const vendorGroups = React.useMemo(() => resolveVendorGroups(modelOptions), [modelOptions]);
  const activeDesktopVendorKey = activeVendorKey || selectedVendorKey || vendorGroups[0]?.vendor || "";
  const activeDesktopVendorGroup = React.useMemo(
    () => vendorGroups.find((group) => group.vendor === activeDesktopVendorKey) ?? vendorGroups[0] ?? null,
    [activeDesktopVendorKey, vendorGroups],
  );
  const mobileVendorGroup = React.useMemo(
    () => vendorGroups.find((group) => group.vendor === mobileVendorKey) ?? null,
    [mobileVendorKey, vendorGroups],
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
  const billingDisplay = React.useMemo<BillingDisplayOptions>(
    () => ({
      currency: billingDisplayCurrency,
      usdToCnyRate: billingDisplayUsdToCnyRate,
    }),
    [billingDisplayCurrency, billingDisplayUsdToCnyRate],
  );

  React.useEffect(() => {
    if (!open || !isMobile) {
      setMobileVendorKey(null);
    }
  }, [isMobile, open]);

  const handleOpenChange = React.useCallback(
    (nextOpen: boolean) => {
      if (nextOpen) {
        setActiveVendorKey(selectedVendorKey || vendorGroups[0]?.vendor || "");
        if (onModelCatalogRefresh) {
          void Promise.resolve(onModelCatalogRefresh()).catch(() => undefined);
        }
      }
      setOpen(nextOpen);
    },
    [onModelCatalogRefresh, selectedVendorKey, vendorGroups],
  );

  const closeMenu = React.useCallback(() => {
    handleOpenChange(false);
  }, [handleOpenChange]);

  const selectDesktopVendor = React.useCallback((vendor: string) => {
    if (vendor === activeDesktopVendorKey) {
      return;
    }
    setActiveVendorKey(vendor);
  }, [activeDesktopVendorKey]);

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
              disabled={disabled || loading}
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
            side="bottom"
            sideOffset={8}
            collisionPadding={24}
            className={cn(
              "relative overflow-visible rounded-xl",
              isMobile
                ? "w-[min(20rem,calc(100vw-3rem))] p-1.5"
                : "w-56 border-0 bg-transparent p-0 shadow-none",
            )}
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
                {vendorGroups.length === 0 ? (
                  <div className="px-2 py-3 text-[11px] leading-4 text-muted-foreground">
                    {t("empty")}
                  </div>
                ) : (
                  <ModelMenuScrollContainer>
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
                            billingDisplay={billingDisplay}
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
                )}
              </>
            ) : (
              <div className="relative min-w-0">
                <div
                  className="absolute right-[calc(100%+0.5rem)] top-0 w-[min(20rem,calc(100vw-16rem))] rounded-xl border-[0.5px] border-border bg-popover p-1.5 shadow-xs"
                >
                  {activeDesktopVendorGroup ? (
                    <ModelMenuScrollContainer>
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
                            billingDisplay={billingDisplay}
                            pricingLabels={pricingLabels}
                            viewPricingLabel={t("viewPricing")}
                            pricingTooltipSide="right"
                          />
                        ))}
                      </div>
                    </ModelMenuScrollContainer>
                  ) : (
                    <div className="px-2 py-3 text-[11px] leading-4 text-muted-foreground">
                      {t("empty")}
                    </div>
                  )}
                </div>

                <div className="min-w-0 rounded-xl border-[0.5px] border-border bg-popover p-1.5 shadow-xs">
                  <div className="flex h-7 items-center justify-between gap-3 px-2">
                    <span className="text-[11px] font-medium text-foreground">{t("vendor")}</span>
                    <span className="truncate text-[10px] font-medium text-muted-foreground">
                      {selectedVendorLabel}
                    </span>
                  </div>
                  {vendorGroups.length === 0 ? (
                    <div className="px-2 py-3 text-[11px] leading-4 text-muted-foreground">
                      {t("empty")}
                    </div>
                  ) : (
                    <ModelMenuScrollContainer>
                      <div className="flex flex-col gap-0.5">
                        {vendorGroups.map((group) => {
                          const selectedVendor = group.vendor === selectedVendorKey;
                          const activeVendor = group.vendor === activeDesktopVendorGroup?.vendor;
                          const vendorIconURL = resolveLobeHubIconURL(group.icon);
                          return (
                            <button
                              type="button"
                              key={group.vendor}
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
                  )}
                </div>
              </div>
            )}
        </PopoverContent>
      </Popover>
      </div>
    </>
  );
}
