export type BillingServiceItemSnapshot = {
  service_code?: string;
  service_name?: string;
  pricing_mode?: string;
  call_count?: number;
  call_nanousd_per_call?: number;
  billed_nanousd?: number;
};

/** 后端账本的计价快照（`pricing_snapshot_json`），字段命名与后端保持一致。 */
export type BillingSnapshot = {
  pricing_mode?: "token" | "call" | "duration" | "tiered" | string;
  service_items?: BillingServiceItemSnapshot[];
  provider_protocol?: string;
  cache_timeout?: string;
  fast_mode?: boolean;
  billing_speed?: string;
  billing_service_tier?: string;
  rate_multiplier?: number;
  cache_write_5m_tokens?: number;
  cache_write_1h_tokens?: number;
  is_free_model?: boolean;
  /** 正常结算之外仍计费的原因，例如审核拦截后照常结算的上游用量（见 MODERATION_BLOCKED_BILLED_REASON）。 */
  billed_reason?: string;
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

/** 与后端 `billing.BilledReasonModerationBlockedUpstreamUsage` 一致：拦截只撤回内容，不撤回上游已产生的用量。 */
export const MODERATION_BLOCKED_BILLED_REASON = "moderation_blocked_upstream_usage";

export function parseBillingSnapshot(value: string | undefined): BillingSnapshot {
  if (!value?.trim()) {
    return {};
  }
  try {
    const parsed = JSON.parse(value) as unknown;
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? (parsed as BillingSnapshot) : {};
  } catch {
    return {};
  }
}
