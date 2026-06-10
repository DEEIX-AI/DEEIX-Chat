"use client";

import * as React from "react";
import {
  ArrowDownToLine,
  ArrowUpFromLine,
  Brain,
  ClockArrowUp,
  ClockCheck,
  CircleDollarSign,
  DatabaseSearch,
  DatabaseZap,
  Cpu,
  FilePenLine,
  Forward,
} from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Brush } from "@/components/animate-ui/icons/brush";
import { ChevronLeft } from "@/components/animate-ui/icons/chevron-left";
import { ChevronRight } from "@/components/animate-ui/icons/chevron-right";
import { Check } from "@/components/animate-ui/icons/check";
import { Copy } from "@/components/animate-ui/icons/copy";
import { Heart } from "@/components/animate-ui/icons/heart";
import { RotateCcw } from "@/components/animate-ui/icons/rotate-ccw";
import { ThumbsDown } from "@/components/animate-ui/icons/thumbs-down";
import { ThumbsUp } from "@/components/animate-ui/icons/thumbs-up";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { upsertUserMemory } from "@/shared/api/memory";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { resolvePersistedPublicID } from "@/features/chat/model/message-submit";
import { billingRateMultiplierNote, cacheWriteBillingLabel, cacheWriteBillingNote } from "@/shared/lib/billing-display";
import type { BillingDisplayLabels } from "@/shared/lib/billing-display";
import type { ChatBillingCost, ChatMessageBranchNavigator } from "@/features/chat/types/messages";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import { cn } from "@/lib/utils";

export type ChatMetaMessage = {
  publicID: string;
  status?: string;
  createdAt?: string;
  updatedAt?: string;
  editedAt?: string | null;
  isPending?: boolean;
  isStreaming?: boolean;
  branchNavigator?: ChatMessageBranchNavigator;
  platformModelName?: string;
  // Token usage for assistant messages.
  inputTokens?: number;
  outputTokens?: number;
  cacheReadTokens?: number;
  cacheWriteTokens?: number;
  reasoningTokens?: number;
  latencyMS?: number;
  billingCost?: ChatBillingCost;
};

export type AssistantReaction = "up" | "down" | null;

function formatMessageDate(value: string | undefined, locale: string): string {
  if (!value) {
    return "";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }

  const year = date.getFullYear();
  const month = date.getMonth() + 1;
  const day = date.getDate();
  const isCurrentYear = year === new Date().getFullYear();
  const isChinese = locale.toLowerCase().startsWith("zh");

  if (isChinese) {
    return isCurrentYear ? `${month}月${day}日` : `${year}年${month}月${day}日`;
  }

  return isCurrentYear ? `${month}/${day}` : `${year}/${month}/${day}`;
}

function BranchSwitcher({
  item,
  onCycle,
}: {
  item: ChatMetaMessage;
  onCycle: (parentPublicID: string | null, direction: "previous" | "next") => void;
}) {
  const t = useTranslations("chat.messages");
  if (!item.branchNavigator) {
    return null;
  }

  return (
    <div className="inline-flex items-center">
      <button
        type="button"
        className="inline-flex size-5 items-center justify-center rounded-md text-muted-foreground transition-colors hover:text-foreground disabled:opacity-35"
        aria-label={t("previousBranch")}
        disabled={!item.branchNavigator.canPrevious}
        onClick={() => onCycle(item.branchNavigator?.parentPublicID ?? null, "previous")}
      >
        <ChevronLeft size={14} strokeWidth={1.8} animateOnHover="default" />
      </button>
      <span className="min-w-7 text-center tabular-nums text-xs font-medium tracking-[0.01em] text-muted-foreground">
        {item.branchNavigator.index}/{item.branchNavigator.total}
      </span>
      <button
        type="button"
        className="inline-flex size-5 items-center justify-center rounded-md text-muted-foreground transition-colors hover:text-foreground disabled:opacity-35"
        aria-label={t("nextBranch")}
        disabled={!item.branchNavigator.canNext}
        onClick={() => onCycle(item.branchNavigator?.parentPublicID ?? null, "next")}
      >
        <ChevronRight size={14} strokeWidth={1.8} animateOnHover="default" />
      </button>
    </div>
  );
}

function MetaContainer({
  align,
  mobileStack = false,
  alwaysVisible = false,
  children,
}: React.PropsWithChildren<{
  align: "start" | "end";
  mobileStack?: boolean;
  alwaysVisible?: boolean;
}>) {
  return (
    <div
      className={[
        "mt-1.5 flex gap-1 text-xs text-muted-foreground opacity-100 transition-opacity duration-150",
        alwaysVisible ? "md:pointer-events-auto md:opacity-100" : "md:pointer-events-none md:opacity-0",
        mobileStack ? "flex-col items-start md:flex-row md:items-center" : "items-center",
        align === "end" ? "justify-end" : "justify-start",
        !alwaysVisible && align === "end"
          ? "md:group-hover/user-message:pointer-events-auto md:group-hover/user-message:opacity-100 md:group-focus-within/user-message:pointer-events-auto md:group-focus-within/user-message:opacity-100"
          : "",
        !alwaysVisible && align === "start"
          ? "md:group-hover/assistant-message:pointer-events-auto md:group-hover/assistant-message:opacity-100 md:group-focus-within/assistant-message:pointer-events-auto md:group-focus-within/assistant-message:opacity-100"
          : "",
      ].join(" ")}
    >
      {children}
    </div>
  );
}

function MetaIconButton({
  label,
  disabled,
  onClick,
  className,
  children,
}: {
  label: string;
  disabled?: boolean;
  onClick?: () => void;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          className={cn(
            "inline-flex size-6 items-center justify-center rounded-md text-muted-foreground transition-colors hover:text-foreground disabled:opacity-40",
            className,
          )}
          aria-label={label}
          disabled={disabled}
          onClick={onClick}
        >
          {children}
        </button>
      </TooltipTrigger>
      <TooltipContent side="top">{label}</TooltipContent>
    </Tooltip>
  );
}

export function UserMessageMeta({
  item,
  busy,
  showRetry,
  onCycleBranch,
  onRetry,
  onEdit,
  onCopy,
  copySucceeded = false,
  readOnly = false,
  alwaysVisible = false,
  showBranchNavigator = true,
}: {
  item: ChatMetaMessage;
  busy: boolean;
  showRetry: boolean;
  onCycleBranch: (parentPublicID: string | null, direction: "previous" | "next") => void;
  onRetry: () => void;
  onEdit: () => void;
  onCopy: () => void;
  copySucceeded?: boolean;
  readOnly?: boolean;
  alwaysVisible?: boolean;
  showBranchNavigator?: boolean;
}) {
  const t = useTranslations("chat.messages");
  const { locale } = useAppLocale();
  const dateLabel = formatMessageDate(item.createdAt, locale);
  const hasPersistedMessage = Boolean(resolvePersistedPublicID(item.publicID));
  const canShowBranchNavigator = Boolean(showBranchNavigator && item.branchNavigator && !busy && !item.isPending);

  return (
    <MetaContainer align="end" alwaysVisible={alwaysVisible}>
      {dateLabel ? <span className="mr-1 shrink-0 tabular-nums">{dateLabel}</span> : null}
      {!readOnly ? (
        <div className="flex items-center">
          {showRetry && hasPersistedMessage ? (
            <MetaIconButton
              label={t("retryMessage")}
              disabled={item.isPending}
              onClick={onRetry}
            >
              <RotateCcw size={14} strokeWidth={1.8} animateOnHover="default" />
            </MetaIconButton>
          ) : null}
          <MetaIconButton
            label={t("editMessage")}
            disabled={item.isPending || !hasPersistedMessage}
            onClick={onEdit}
          >
            <Brush size={14} strokeWidth={1.8} animateOnHover="default" />
          </MetaIconButton>
          <MetaIconButton
            label={t("copyMessage")}
            disabled={item.isPending}
            onClick={onCopy}
          >
            {copySucceeded ? (
              <Check size={14} strokeWidth={1.8} animate="default" />
            ) : (
              <Copy size={14} strokeWidth={1.8} animateOnHover="default" />
            )}
          </MetaIconButton>
        </div>
      ) : null}
      {canShowBranchNavigator ? <BranchSwitcher item={item} onCycle={onCycleBranch} /> : null}
    </MetaContainer>
  );
}

function TokenBadge({
  inputTokens,
  outputTokens,
  cacheReadTokens,
  cacheWriteTokens,
  reasoningTokens,
}: {
  inputTokens?: number;
  outputTokens?: number;
  cacheReadTokens?: number;
  cacheWriteTokens?: number;
  reasoningTokens?: number;
}) {
  const t = useTranslations("chat.meta");
  const inputValue = inputTokens ?? 0;
  const outputValue = outputTokens ?? 0;
  const cacheReadValue = cacheReadTokens ?? 0;
  const cacheWriteValue = cacheWriteTokens ?? 0;
  const reasoningValue = reasoningTokens ?? 0;
  const hasUsage = inputValue > 0 || outputValue > 0 || cacheReadValue > 0 || cacheWriteValue > 0 || reasoningValue > 0;
  if (!hasUsage) {
    return null;
  }

  return (
    <span className="ml-0.5 inline-flex items-center gap-1.5 rounded px-1.5 py-0.5 text-[10px] leading-3.5 font-mono text-muted-foreground/70 bg-muted/30 select-none whitespace-nowrap">
      <TokenMetric label={t("inputTokens")} value={inputValue} icon={<ArrowUpFromLine className="size-3" strokeWidth={1.4} />} />
      <TokenMetric label={t("cacheReadTokens")} value={cacheReadValue} icon={<DatabaseSearch className="size-3" strokeWidth={1.4} />} />
      <TokenMetric label={t("reasoningTokens")} value={reasoningValue} icon={<Brain className="size-3" strokeWidth={1.4} />} />
      <TokenMetric label={t("outputTokens")} value={outputValue} icon={<ArrowDownToLine className="size-3" strokeWidth={1.4} />} />
      <TokenMetric label={t("cacheWriteTokens")} value={cacheWriteValue} icon={<DatabaseZap className="size-3" strokeWidth={1.4} />} />
    </span>
  );
}

function TokenMetric({ label, value, icon }: { label: string; value: number; icon: React.ReactNode }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="inline-flex items-center gap-0.5" aria-label={label}>
          {icon}
          {value.toLocaleString()}
        </span>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}

function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) {
    return "";
  }
  const wholeMS = Math.max(1, Math.floor(ms));
  if (wholeMS <= 9999) {
    return `${wholeMS}ms`;
  }
  return `${Math.floor(wholeMS / 1000)}s`;
}

function useLiveElapsedMS(enabled: boolean, createdAt?: string): number {
  const [elapsedMS, setElapsedMS] = React.useState(0);

  React.useEffect(() => {
    if (!enabled) {
      setElapsedMS(0);
      return;
    }
    const startedAt = new Date(createdAt ?? "").getTime();
    if (Number.isNaN(startedAt)) {
      setElapsedMS(0);
      return;
    }

    let frameID: number | null = null;
    let timerID: number | null = null;

    const tick = () => {
      const nextElapsedMS = Math.max(0, Date.now() - startedAt);
      setElapsedMS(nextElapsedMS);

      if (nextElapsedMS < 9999) {
        frameID = window.requestAnimationFrame(tick);
        return;
      }

      const delayToNextSecond = Math.max(1, 1000 - (nextElapsedMS % 1000));
      timerID = window.setTimeout(tick, delayToNextSecond);
    };

    tick();

    return () => {
      if (frameID !== null) {
        window.cancelAnimationFrame(frameID);
      }
      if (timerID !== null) {
        window.clearTimeout(timerID);
      }
    };
  }, [createdAt, enabled]);

  return enabled ? elapsedMS : 0;
}

function calculateElapsedMS(startedAt?: string, endedAt?: string): number {
  if (!startedAt || !endedAt) {
    return 0;
  }
  const startMS = new Date(startedAt).getTime();
  const endMS = new Date(endedAt).getTime();
  if (Number.isNaN(startMS) || Number.isNaN(endMS)) {
    return 0;
  }
  return Math.max(0, endMS - startMS);
}

function LatencyBadge({ item }: { item: ChatMetaMessage }) {
  const t = useTranslations("chat.meta");
  const isLive = Boolean(item.isPending || item.isStreaming);
  const liveLatencyMS = useLiveElapsedMS(isLive, item.createdAt);
  const storedLatencyMS = item.latencyMS && item.latencyMS > 0 ? item.latencyMS : 0;
  const calculatedLatencyMS = calculateElapsedMS(item.createdAt, item.updatedAt);
  const latencyMS = isLive
    ? liveLatencyMS || calculatedLatencyMS || storedLatencyMS
    : storedLatencyMS || calculatedLatencyMS;
  const label = formatDuration(latencyMS);
  if (!label) {
    return null;
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          className="ml-0.5 inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] leading-3.5 font-mono text-muted-foreground/70 bg-muted/30 select-none whitespace-nowrap"
          aria-label={isLive ? t("generationDuration") : t("totalDuration")}
        >
          {isLive ? (
            <ClockArrowUp className="size-3" strokeWidth={1.4} />
          ) : (
            <ClockCheck className="size-3" strokeWidth={1.4} />
          )}
          {label}
        </span>
      </TooltipTrigger>
      <TooltipContent>{isLive ? t("generationDuration") : t("totalDuration")}</TooltipContent>
    </Tooltip>
  );
}

function EditedBadge() {
  const t = useTranslations("chat.messages");
  const label = t("replyEditedDisclaimer");
  const tooltip = t("replyEditedTooltip");

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          className="ml-0.5 inline-flex items-center gap-1 rounded bg-muted/30 px-1.5 py-0.5 text-[10px] leading-3.5 text-muted-foreground/70 select-none whitespace-nowrap"
          aria-label={tooltip}
        >
          <FilePenLine className="size-3" strokeWidth={1.4} />
          {label}
        </span>
      </TooltipTrigger>
      <TooltipContent className="max-w-64">{tooltip}</TooltipContent>
    </Tooltip>
  );
}

function ModelBadge({ label }: { label: string }) {
  const t = useTranslations("chat.meta");
  const normalized = label.trim();
  if (!normalized) {
    return null;
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          className="ml-0.5 inline-flex max-w-48 items-center gap-1 rounded bg-muted/30 px-1.5 py-0.5 font-mono text-[10px] leading-3.5 text-muted-foreground/70 select-none whitespace-nowrap"
          aria-label={t("model")}
        >
          <Cpu className="size-3 shrink-0" strokeWidth={1.4} />
          <span className="truncate">{normalized}</span>
        </span>
      </TooltipTrigger>
      <TooltipContent>{normalized}</TooltipContent>
    </Tooltip>
  );
}

type BillingSnapshot = {
  pricing_mode?: "token" | "call" | "duration" | "tiered" | string;
  provider_protocol?: string;
  cache_timeout?: string;
  fast_mode?: boolean;
  billing_speed?: string;
  billing_service_tier?: string;
  rate_multiplier?: number;
  cache_write_5m_tokens?: number;
  cache_write_1h_tokens?: number;
  is_free_model?: boolean;
  input_nanousd_per_m_tokens?: number;
  cache_read_nanousd_per_m_tokens?: number;
  cache_write_nanousd_per_m_tokens?: number;
  output_nanousd_per_m_tokens?: number;
  call_nanousd_per_call?: number;
  duration_nanousd_per_second?: number;
  input_billed_nanousd?: number;
  cache_read_billed_nanousd?: number;
  cache_write_billed_nanousd?: number;
  output_billed_nanousd?: number;
  call_billed_nanousd?: number;
  duration_billed_nanousd?: number;
  tiered_from_tokens?: number;
  tiered_up_to_tokens?: number | null;
};

function parseBillingSnapshot(value: string): BillingSnapshot {
  if (!value.trim()) {
    return {};
  }
  try {
    const parsed = JSON.parse(value) as unknown;
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? (parsed as BillingSnapshot) : {};
  } catch {
    return {};
  }
}

function readBillingNumber(snapshot: BillingSnapshot, key: keyof BillingSnapshot): number {
  const value = snapshot[key];
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function nanousdToUSD(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 0;
  return value / 1_000_000_000;
}

function formatBillingCost(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "$0";
  if (value < 0.000001) return "< $0.000001";
  return `$${value.toLocaleString("en-US", {
    minimumFractionDigits: 0,
    maximumFractionDigits: 6,
  })}`;
}

function formatTooltipBillingCost(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "$0.000000";
  return `$${value.toLocaleString("en-US", {
    minimumFractionDigits: 6,
    maximumFractionDigits: 6,
  })}`;
}

function formatTooltipUnitPrice(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "$0.00";
  return `$${value.toLocaleString("en-US", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}`;
}

function calcTokenBilledNanousd(tokens: number, rateNanousd: number): number {
  if (!Number.isFinite(tokens) || !Number.isFinite(rateNanousd) || tokens <= 0 || rateNanousd <= 0) {
    return 0;
  }
  return Math.round((tokens * rateNanousd) / 1_000_000);
}

type BillingTooltipLine =
  | { type: "row"; left: string; right: string }
  | { type: "divider" }
  | { type: "tiered-table"; rangeLabel: string; rows: BillingTieredTableRow[]; totalAmount: string };

type BillingTieredTableRow = {
  item: string;
  tokens: string;
  unitPrice: string;
  amount: string;
};

type BillingMetaLabels = {
  display: BillingDisplayLabels;
  input: string;
  output: string;
  cacheRead: string;
  rateNote: string;
  cacheNote: string;
  total: string;
  freeModelNoBilling: string;
  perCall: string;
  perSecond: string;
  callUnit: string;
  secondUnit: string;
  tieredRange: (from: string, upTo: string | null) => string;
};

function useBillingMetaLabels(): BillingMetaLabels {
  const t = useTranslations("chat.meta.billing");
  return React.useMemo(
    () => ({
      display: {
        cacheWrite: t("cacheWrite"),
        cacheWrite5m: t("cacheWrite5m"),
        cacheWrite1h: t("cacheWrite1h"),
        cacheWrite5m1h: t("cacheWrite5m1h"),
        claudeCacheWriteMixedNote: (multiplier) => t("claudeCacheWriteMixedNote", { multiplier }),
        claudeCacheWriteNote: (timeout, multiplier) => t("claudeCacheWriteNote", { timeout, multiplier }),
        claudeFastModeNote: (multiplier) => t("claudeFastModeNote", { multiplier }),
        openaiServiceTierNote: (tier, multiplier) => t("openaiServiceTierNote", { tier, multiplier }),
        cacheWritePricingLabel: t("cacheWritePricingLabel"),
        cacheWritePricingNote: t("cacheWritePricingNote"),
      },
      input: t("input"),
      output: t("output"),
      cacheRead: t("cacheRead"),
      rateNote: t("rateNote"),
      cacheNote: t("cacheNote"),
      total: t("total"),
      freeModelNoBilling: t("freeModelNoBilling"),
      perCall: t("perCall"),
      perSecond: t("perSecond"),
      callUnit: t("callUnit"),
      secondUnit: t("secondUnit"),
      tieredRange: (from, upTo) => upTo ? t("tieredRangeBounded", { from, upTo }) : t("tieredRangeOpen", { from }),
    }),
    [t],
  );
}

function formatBillingFormulaLine(label: string, tokens: number, rateNanousd: number, billedNanousd: number): BillingTooltipLine {
  return {
    type: "row",
    left: label,
    right: `${tokens.toLocaleString("en-US")} tokens * ${formatTooltipUnitPrice(nanousdToUSD(rateNanousd))} / 1M = ${formatTooltipBillingCost(nanousdToUSD(billedNanousd))}`,
  };
}

function formatTokenQuantity(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "0";
  return value.toLocaleString("en-US");
}

function formatTieredRangeLabel(fromTokens: number | null | undefined, upToTokens: number | null | undefined, labels: BillingMetaLabels): string {
  const from = Number.isFinite(fromTokens ?? NaN) && (fromTokens ?? 0) > 0 ? fromTokens ?? 0 : 0;
  const upTo = Number.isFinite(upToTokens ?? NaN) && (upToTokens ?? 0) > 0 ? upToTokens ?? 0 : null;
  return labels.tieredRange(formatTokenQuantity(from), upTo ? formatTokenQuantity(upTo) : null);
}

function formatTieredTableRow(item: string, tokens: number, rateNanousd: number, billedNanousd: number): BillingTieredTableRow {
  const safeTokens = Number.isFinite(tokens) && tokens > 0 ? tokens : 0;
  const safeBilled = Number.isFinite(billedNanousd) && billedNanousd > 0 ? billedNanousd : 0;
  return {
    item,
    tokens: formatTokenQuantity(safeTokens),
    unitPrice: `${formatTooltipUnitPrice(nanousdToUSD(rateNanousd))} / 1M`,
    amount: formatTooltipBillingCost(nanousdToUSD(safeBilled)),
  };
}

function formatCountLine(label: string, count: number, unit: string, rateNanousd: number, billedNanousd: number): BillingTooltipLine {
  const safeCount = Number.isFinite(count) && count > 0 ? count : 0;
  return {
    type: "row",
    left: label,
    right: `${safeCount.toLocaleString("en-US")} ${unit} * ${formatTooltipUnitPrice(nanousdToUSD(rateNanousd))} / ${unit} = ${formatTooltipBillingCost(nanousdToUSD(billedNanousd))}`,
  };
}

function formatTotalLine(amount: string, labels: BillingMetaLabels): BillingTooltipLine {
  return { type: "row", left: labels.total, right: amount };
}

function billingTooltipLines(item: ChatMetaMessage, labels: BillingMetaLabels): BillingTooltipLine[] {
  const cost = item.billingCost;
  if (!cost) {
    return [];
  }
  const snapshot = parseBillingSnapshot(cost.pricingSnapshotJSON);
  const pricingMode = snapshot.pricing_mode === "call" || snapshot.pricing_mode === "duration" || snapshot.pricing_mode === "tiered" ? snapshot.pricing_mode : "token";
  const totalLine = snapshot.is_free_model
    ? formatTotalLine(`$0.000000 (${labels.freeModelNoBilling})`, labels)
    : formatTotalLine(formatTooltipBillingCost(nanousdToUSD(cost.billedNanousd)), labels);

  if (pricingMode === "call") {
    const rate = readBillingNumber(snapshot, "call_nanousd_per_call");
    const billed = readBillingNumber(snapshot, "call_billed_nanousd") || rate;
    return [formatCountLine(labels.perCall, 1, labels.callUnit, rate, billed), { type: "divider" }, totalLine];
  }

  if (pricingMode === "duration") {
    const rate = readBillingNumber(snapshot, "duration_nanousd_per_second");
    const billed = readBillingNumber(snapshot, "duration_billed_nanousd");
    return [formatCountLine(labels.perSecond, 1, labels.secondUnit, rate, billed), { type: "divider" }, totalLine];
  }

  const inputRate = readBillingNumber(snapshot, "input_nanousd_per_m_tokens");
  const outputRate = readBillingNumber(snapshot, "output_nanousd_per_m_tokens");
  const cacheReadRate = readBillingNumber(snapshot, "cache_read_nanousd_per_m_tokens");
  const cacheWriteRate = readBillingNumber(snapshot, "cache_write_nanousd_per_m_tokens");
  const inputTokens = item.inputTokens ?? 0;
  const cacheReadTokens = item.cacheReadTokens ?? 0;
  const cacheWriteTokens = item.cacheWriteTokens ?? 0;
  const outputTokens = item.outputTokens ?? 0;
  const reasoningTokens = item.reasoningTokens ?? 0;
  const billedOutputTokens = outputTokens + reasoningTokens;
  const cacheWriteLabel = cacheWriteBillingLabel(snapshot, labels.display);
  const cacheWriteNote = cacheWriteBillingNote(snapshot, labels.display);
  const rateMultiplierNote = billingRateMultiplierNote(snapshot, labels.display);

  if (pricingMode === "tiered") {
    const tieredRows = [
      formatTieredTableRow(labels.input, inputTokens, inputRate, readBillingNumber(snapshot, "input_billed_nanousd")),
      formatTieredTableRow(labels.output, billedOutputTokens, outputRate, readBillingNumber(snapshot, "output_billed_nanousd")),
      formatTieredTableRow(labels.cacheRead, cacheReadTokens, cacheReadRate, readBillingNumber(snapshot, "cache_read_billed_nanousd")),
      formatTieredTableRow(cacheWriteLabel, cacheWriteTokens, cacheWriteRate, readBillingNumber(snapshot, "cache_write_billed_nanousd")),
    ];
    const lines: BillingTooltipLine[] = [];
    if (rateMultiplierNote || cacheWriteNote) {
      if (rateMultiplierNote) {
        lines.push({ type: "row", left: labels.rateNote, right: rateMultiplierNote });
      }
      if (cacheWriteNote) {
        lines.push({ type: "row", left: labels.cacheNote, right: cacheWriteNote });
      }
      lines.push({ type: "divider" });
    }
    if (tieredRows.length > 0) {
      lines.push({
        type: "tiered-table",
        rangeLabel: formatTieredRangeLabel(snapshot.tiered_from_tokens, snapshot.tiered_up_to_tokens, labels),
        rows: tieredRows,
        totalAmount: snapshot.is_free_model ? `$0.000000 (${labels.freeModelNoBilling})` : formatTooltipBillingCost(nanousdToUSD(cost.billedNanousd)),
      });
      return lines;
    }
  }

  const lines: BillingTooltipLine[] = [
    formatBillingFormulaLine(labels.input, inputTokens, inputRate, readBillingNumber(snapshot, "input_billed_nanousd") || calcTokenBilledNanousd(inputTokens, inputRate)),
    formatBillingFormulaLine(labels.output, billedOutputTokens, outputRate, readBillingNumber(snapshot, "output_billed_nanousd") || calcTokenBilledNanousd(billedOutputTokens, outputRate)),
    formatBillingFormulaLine(labels.cacheRead, cacheReadTokens, cacheReadRate, readBillingNumber(snapshot, "cache_read_billed_nanousd") || calcTokenBilledNanousd(cacheReadTokens, cacheReadRate)),
    formatBillingFormulaLine(cacheWriteLabel, cacheWriteTokens, cacheWriteRate, readBillingNumber(snapshot, "cache_write_billed_nanousd") || calcTokenBilledNanousd(cacheWriteTokens, cacheWriteRate)),
    { type: "divider" },
    totalLine,
  ];
  const noteLines: BillingTooltipLine[] = [];
  if (rateMultiplierNote) {
    noteLines.push({ type: "row", left: labels.rateNote, right: rateMultiplierNote });
  }
  if (cacheWriteNote) {
    noteLines.push({ type: "row", left: labels.cacheNote, right: cacheWriteNote });
  }
  if (noteLines.length > 0) {
    lines.splice(4, 0, ...noteLines);
  }
  return lines;
}

function BillingCostBadge({ item }: { item: ChatMetaMessage }) {
  const t = useTranslations("chat.meta");
  const labels = useBillingMetaLabels();
  const cost = item.billingCost;
  if (!cost || cost.billingMode === "self") {
    return null;
  }
  const lines = billingTooltipLines(item, labels);
  if (lines.length === 0) {
    return null;
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          tabIndex={0}
          aria-label={t("billingCost")}
          className="ml-0.5 inline-flex cursor-default items-center gap-1 rounded bg-muted/30 px-1.5 py-0.5 font-mono text-[10px] leading-3.5 text-muted-foreground/70 select-none whitespace-nowrap outline-none focus-visible:ring-[1px] focus-visible:ring-ring/40"
        >
          <CircleDollarSign className="size-3" strokeWidth={1.4} />
          {formatBillingCost(nanousdToUSD(cost.billedNanousd))}
        </span>
      </TooltipTrigger>
      <TooltipContent side="top" align="start" className="max-w-[min(92vw,44rem)]">
        <div className="min-w-72 space-y-1 text-left text-[11px] leading-relaxed">
          {lines.map((line, index) =>
            line.type === "divider" ? (
              <div key={`divider-${index}`} className="my-1 h-px bg-background/20" />
            ) : line.type === "tiered-table" ? (
              <TieredBillingTable key={`tiered-table-${index}`} line={line} />
            ) : (
              <div key={`${line.left}-${index}`} className="grid grid-cols-[minmax(0,1fr)_auto] items-baseline gap-8">
                <span className="min-w-0 text-left">{line.left}</span>
                <span className="whitespace-nowrap text-right tabular-nums">{line.right}</span>
              </div>
            ),
          )}
        </div>
      </TooltipContent>
    </Tooltip>
  );
}

function TieredBillingTable({ line }: { line: Extract<BillingTooltipLine, { type: "tiered-table" }> }) {
  const t = useTranslations("chat.meta.billing.table");
  return (
    <div className="max-w-[min(92vw,34rem)] overflow-x-auto">
      <div className="mb-1 text-[10px] font-medium text-background/80">{line.rangeLabel}</div>
      <table className="w-full border-collapse text-left tabular-nums">
        <thead>
          <tr className="border-b border-background/20 text-[10px] text-background/65">
            <th className="whitespace-nowrap px-2 pb-1 font-medium first:pl-0" aria-label={t("item")} />
            <th className="whitespace-nowrap px-2 pb-1 text-right font-medium">{t("usage")}</th>
            <th className="whitespace-nowrap px-2 pb-1 text-right font-medium">{t("unitPrice")}</th>
            <th className="whitespace-nowrap px-2 pb-1 text-right font-medium last:pr-0">{t("amount")}</th>
          </tr>
        </thead>
        <tbody>
          {line.rows.map((row, rowIndex) => (
            <tr key={`${row.item}-${rowIndex}`} className="border-b border-background/10 last:border-0">
              <td className="whitespace-nowrap px-2 py-1 first:pl-0">{row.item}</td>
              <td className="whitespace-nowrap px-2 py-1 text-right">{row.tokens}</td>
              <td className="whitespace-nowrap px-2 py-1 text-right">{row.unitPrice}</td>
              <td className="whitespace-nowrap px-2 py-1 text-right last:pr-0">{row.amount}</td>
            </tr>
          ))}
        </tbody>
        <tfoot>
          <tr className="border-t border-background/20">
            <td className="px-2 pt-1.5 font-medium first:pl-0" colSpan={3}>{t("total")}</td>
            <td className="whitespace-nowrap px-2 pt-1.5 text-right font-medium last:pr-0">{line.totalAmount}</td>
          </tr>
        </tfoot>
      </table>
    </div>
  );
}

function QuickMemoryPin({ disabled }: { disabled?: boolean }) {
  const t = useTranslations("chat.messages");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [open, setOpen] = React.useState(false);
  const [key, setKey] = React.useState("");
  const [value, setValue] = React.useState("");
  const [saving, setSaving] = React.useState(false);

  const handleSave = React.useCallback(async () => {
    const trimmedKey = key.trim();
    const trimmedValue = value.trim();
    if (!trimmedKey || !trimmedValue) return;
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("authTokenMissing"));
        return;
      }
      await upsertUserMemory(token, trimmedKey, trimmedValue, "preference");
      toast.success(t("memorySaved"), { description: t("memorySavedDescription") });
      setKey("");
      setValue("");
      setOpen(false);
    } catch (error) {
      toast.error(t("memorySaveFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }, [key, resolveErrorMessage, t, value]);

  const handleKeyDown = React.useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        void handleSave();
      }
    },
    [handleSave],
  );

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <Tooltip>
        <TooltipTrigger asChild>
          <PopoverTrigger asChild>
            <button
              type="button"
              className="inline-flex size-6 items-center justify-center rounded-md text-muted-foreground transition-colors hover:text-foreground disabled:opacity-40"
              aria-label={t("rememberPreference")}
              disabled={disabled}
            >
              <Heart size={14} strokeWidth={1.8} animateOnHover="default" />
            </button>
          </PopoverTrigger>
        </TooltipTrigger>
        <TooltipContent side="top">{t("rememberPreference")}</TooltipContent>
      </Tooltip>
      <PopoverContent align="start" className="w-64 p-3">
        <p className="mb-2 text-[12px] font-medium text-foreground">{t("rememberPreference")}</p>
        <div className="space-y-2">
          <Input
            placeholder={t("memoryNamePlaceholder")}
            value={key}
            onChange={(e) => setKey(e.target.value)}
            onKeyDown={handleKeyDown}
          />
          <Input
            placeholder={t("memoryValuePlaceholder")}
            value={value}
            onChange={(e) => setValue(e.target.value)}
            onKeyDown={handleKeyDown}
          />
          <Button
            size="sm"
            className="h-7 w-full text-[12px]"
            disabled={!key.trim() || !value.trim() || saving}
            onClick={() => void handleSave()}
          >
            {saving ? t("savingPreference") : t("savePreference")}
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  );
}

export function AssistantMessageMeta({
  item,
  busy,
  reaction,
  onCycleBranch,
  onRetry,
  onContinue,
  onEdit,
  onCopy,
  copySucceeded = false,
  onReact,
  showModelInfo = true,
  showLatency = true,
  showTokenUsage = true,
  showBillingCost = false,
  readOnly = false,
  alwaysVisible = false,
  showBranchNavigator = true,
}: {
  item: ChatMetaMessage;
  busy: boolean;
  reaction: AssistantReaction;
  onCycleBranch: (parentPublicID: string | null, direction: "previous" | "next") => void;
  onRetry: () => void;
  onContinue?: () => void;
  onEdit?: () => void;
  onCopy: () => void;
  copySucceeded?: boolean;
  onReact: (value: AssistantReaction) => void;
  showModelInfo?: boolean;
  showLatency?: boolean;
  showTokenUsage?: boolean;
  showBillingCost?: boolean;
  readOnly?: boolean;
  alwaysVisible?: boolean;
  showBranchNavigator?: boolean;
}) {
  const t = useTranslations("chat.messages");
  const isLive = Boolean(item.isPending || item.isStreaming);
  const canRetry = !readOnly && !busy && !isLive;
  const canEdit = Boolean(canRetry && onEdit && resolvePersistedPublicID(item.publicID));
  const canContinue = Boolean(canRetry && resolvePersistedPublicID(item.publicID) && item.status === "interrupted");
  const canShowBranchNavigator = Boolean(showBranchNavigator && item.branchNavigator && !busy && !isLive);
  const hasTokenUsage = Boolean(
    (item.inputTokens ?? 0) > 0 ||
    (item.outputTokens ?? 0) > 0 ||
    (item.cacheReadTokens ?? 0) > 0 ||
    (item.cacheWriteTokens ?? 0) > 0 ||
    (item.reasoningTokens ?? 0) > 0,
  );
  const hasLatencyBadge = Boolean(
    showLatency &&
    (
      isLive ||
      (item.latencyMS && item.latencyMS > 0) ||
      calculateElapsedMS(item.createdAt, item.updatedAt) > 0
    ),
  );
  const hasDetailBadges = Boolean(
    (showModelInfo && item.platformModelName?.trim()) ||
    (showTokenUsage && hasTokenUsage) ||
    hasLatencyBadge ||
    item.editedAt ||
    (showBillingCost && item.billingCost && item.billingCost.billingMode !== "self"),
  );
  const hasActionRow = Boolean(!readOnly || canShowBranchNavigator);

  return (
    <MetaContainer align="start" alwaysVisible={alwaysVisible}>
      <div className="flex min-w-0 max-w-full flex-col items-start gap-1.5 pt-0.5">
        {hasDetailBadges ? (
          <div className="flex min-w-0 max-w-full flex-wrap items-center gap-1">
            {showModelInfo ? <ModelBadge label={item.platformModelName?.trim() || ""} /> : null}
            {showTokenUsage ? (
              <TokenBadge
                inputTokens={item.inputTokens}
                outputTokens={item.outputTokens}
                cacheReadTokens={item.cacheReadTokens}
                cacheWriteTokens={item.cacheWriteTokens}
                reasoningTokens={item.reasoningTokens}
              />
            ) : null}
            {hasLatencyBadge ? <LatencyBadge item={item} /> : null}
            {item.editedAt ? <EditedBadge /> : null}
            {showBillingCost ? <BillingCostBadge item={item} /> : null}
          </div>
        ) : null}
        {hasActionRow ? (
          <div className="flex min-w-0 max-w-full flex-wrap items-center gap-1">
            {!readOnly ? (
              <>
                <MetaIconButton
                  label={t("copyReply")}
                  disabled={!item.publicID}
                  onClick={onCopy}
                >
                  {copySucceeded ? (
                    <Check size={14} strokeWidth={1.8} animate="default" />
                  ) : (
                    <Copy size={14} strokeWidth={1.8} animateOnHover="default" />
                  )}
                </MetaIconButton>
                {canEdit ? (
                  <MetaIconButton
                    label={t("editReply")}
                    onClick={onEdit}
                  >
                    <Brush size={14} strokeWidth={1.8} animateOnHover="default" />
                  </MetaIconButton>
                ) : null}
                <MetaIconButton
                  label={t("likeReply")}
                  className={reaction === "up" ? "text-foreground" : undefined}
                  disabled={isLive}
                  onClick={() => onReact(reaction === "up" ? null : "up")}
                >
                  <ThumbsUp size={14} strokeWidth={1.8} animateOnHover="default" />
                </MetaIconButton>
                <MetaIconButton
                  label={t("dislikeReply")}
                  className={reaction === "down" ? "text-foreground" : undefined}
                  disabled={isLive}
                  onClick={() => onReact(reaction === "down" ? null : "down")}
                >
                  <ThumbsDown size={14} strokeWidth={1.8} animateOnHover="default" />
                </MetaIconButton>
                {canRetry ? (
                  <MetaIconButton
                    label={t("retryReply")}
                    onClick={onRetry}
                  >
                    <RotateCcw size={14} strokeWidth={1.8} animateOnHover="default" />
                  </MetaIconButton>
                ) : null}
                {canContinue && onContinue ? (
                  <MetaIconButton
                    label={t("continueReply")}
                    onClick={onContinue}
                  >
                    <Forward className="size-3.5" strokeWidth={1.8} />
                  </MetaIconButton>
                ) : null}
                <QuickMemoryPin disabled={isLive} />
              </>
            ) : null}
            {canShowBranchNavigator ? <BranchSwitcher item={item} onCycle={onCycleBranch} /> : null}
          </div>
        ) : null}
      </div>
    </MetaContainer>
  );
}
