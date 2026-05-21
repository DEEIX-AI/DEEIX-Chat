export type BillingCacheWriteSnapshot = {
  provider_protocol?: string;
  cache_timeout?: string;
  fast_mode?: boolean;
  billing_speed?: string;
  billing_service_tier?: string;
  rate_multiplier?: number;
  cache_write_5m_tokens?: number;
  cache_write_1h_tokens?: number;
};

export type BillingDisplayLabels = {
  cacheWrite: string;
  cacheWrite5m: string;
  cacheWrite1h: string;
  cacheWrite5m1h: string;
  claudeCacheWriteMixedNote: (multiplier: string) => string;
  claudeCacheWriteNote: (timeout: "5m" | "1h", multiplier: string) => string;
  claudeFastModeNote: (multiplier: string) => string;
  openaiServiceTierNote: (tier: string, multiplier: string) => string;
  cacheWritePricingLabel: string;
  cacheWritePricingNote: string;
};

const DEFAULT_BILLING_DISPLAY_LABELS: BillingDisplayLabels = {
  cacheWrite: "Cache write",
  cacheWrite5m: "Cache write 5m",
  cacheWrite1h: "Cache write 1h",
  cacheWrite5m1h: "Cache write 5m/1h",
  claudeCacheWriteMixedNote: (multiplier) => `Claude cache write uses configured pricing at ${multiplier}`,
  claudeCacheWriteNote: (timeout, multiplier) => `Claude ${timeout} cache write uses configured pricing at ${multiplier}`,
  claudeFastModeNote: (multiplier) => `Claude Fast Mode bills input, output, and cache usage at ${multiplier}`,
  openaiServiceTierNote: (tier, multiplier) => `OpenAI service_tier=${tier} bills at ${multiplier}`,
  cacheWritePricingLabel: "Cache write 5m",
  cacheWritePricingNote: "Claude cache read uses configured pricing; cache write 5m uses 1.25x, 1h uses 2x, and Fast Mode applies another 6x multiplier.",
};

export function isAnthropicBillingSnapshot(snapshot: BillingCacheWriteSnapshot): boolean {
  return String(snapshot.provider_protocol || "").trim() === "anthropic_messages";
}

export function anthropicCacheTimeoutLabel(snapshot: BillingCacheWriteSnapshot): "5m" | "1h" {
  return String(snapshot.cache_timeout || "").trim().toLowerCase() === "1h" ? "1h" : "5m";
}

export function hasAnthropicMessagesProtocol(protocols: readonly string[] | null | undefined): boolean {
  return Array.isArray(protocols) && protocols.some((protocol) => String(protocol || "").trim() === "anthropic_messages");
}

export function cacheWriteBillingLabel(snapshot: BillingCacheWriteSnapshot, labels: BillingDisplayLabels = DEFAULT_BILLING_DISPLAY_LABELS): string {
  if (!isAnthropicBillingSnapshot(snapshot)) {
    return labels.cacheWrite;
  }
  if ((snapshot.cache_write_5m_tokens || 0) > 0 && (snapshot.cache_write_1h_tokens || 0) > 0) {
    return labels.cacheWrite5m1h;
  }
  return anthropicCacheTimeoutLabel(snapshot) === "1h" ? labels.cacheWrite1h : labels.cacheWrite5m;
}

export function cacheWriteBillingNote(snapshot: BillingCacheWriteSnapshot, labels: BillingDisplayLabels = DEFAULT_BILLING_DISPLAY_LABELS): string | null {
  if (!isAnthropicBillingSnapshot(snapshot)) {
    return null;
  }
  const mixedCacheWrite = (snapshot.cache_write_5m_tokens || 0) > 0 && (snapshot.cache_write_1h_tokens || 0) > 0;
  const timeout = anthropicCacheTimeoutLabel(snapshot);
  const multiplier = mixedCacheWrite ? "5m 1.25x, 1h 2x" : timeout === "1h" ? "2x" : "1.25x";
  return mixedCacheWrite ? labels.claudeCacheWriteMixedNote(multiplier) : labels.claudeCacheWriteNote(timeout, multiplier);
}

export function billingRateMultiplierNote(snapshot: BillingCacheWriteSnapshot, labels: BillingDisplayLabels = DEFAULT_BILLING_DISPLAY_LABELS): string | null {
  const multiplier = Number(snapshot.rate_multiplier || 0);
  if (!Number.isFinite(multiplier) || multiplier <= 0 || Math.abs(multiplier - 1) < 0.000001) {
    return null;
  }
  if (isAnthropicBillingSnapshot(snapshot) && (snapshot.fast_mode || String(snapshot.billing_speed || "").trim() === "fast")) {
    return labels.claudeFastModeNote(formatRateMultiplier(multiplier));
  }
  if (String(snapshot.provider_protocol || "").trim() === "openai_responses" || String(snapshot.provider_protocol || "").trim() === "openai_chat_completions") {
    const tier = String(snapshot.billing_service_tier || "").trim();
    if (tier === "priority" || tier === "flex") {
      return labels.openaiServiceTierNote(tier, formatRateMultiplier(multiplier));
    }
  }
  return null;
}

function formatRateMultiplier(value: number): string {
  return Number.isInteger(value) ? `${value}x` : `${value.toFixed(2).replace(/0+$/, "").replace(/\.$/, "")}x`;
}

export function cacheWritePricingLabel(protocols: readonly string[] | null | undefined, labels: BillingDisplayLabels = DEFAULT_BILLING_DISPLAY_LABELS): string {
  return hasAnthropicMessagesProtocol(protocols) ? labels.cacheWritePricingLabel : labels.cacheWrite;
}

export function cacheWritePricingNote(protocols: readonly string[] | null | undefined, labels: BillingDisplayLabels = DEFAULT_BILLING_DISPLAY_LABELS): string | null {
  return hasAnthropicMessagesProtocol(protocols) ? labels.cacheWritePricingNote : null;
}

export function resolveCacheWritePricingUSD(
  protocols: readonly string[] | null | undefined,
  configuredCacheWriteUSDPerMTokens: number,
  cacheTimeout: "5m" | "1h" = "5m",
): number {
  if (!hasAnthropicMessagesProtocol(protocols)) {
    return configuredCacheWriteUSDPerMTokens;
  }
  if (!Number.isFinite(configuredCacheWriteUSDPerMTokens) || configuredCacheWriteUSDPerMTokens <= 0) {
    return 0;
  }
  return cacheTimeout === "1h" ? configuredCacheWriteUSDPerMTokens * 2 : configuredCacheWriteUSDPerMTokens * 1.25;
}
