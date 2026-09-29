import {
  isFiniteNumber,
  isRecord,
  parseJSON,
  readBoolean,
  readFiniteNumber,
  readString,
  type UnknownRecord,
} from "@/shared/lib/type-guards";

export type BillingServiceItemSnapshot = {
  service_code?: string;
  service_name?: string;
  pricing_mode?: string;
  call_count?: number;
  call_nanousd_per_call?: number;
  billed_nanousd?: number;
};

/** Backend ledger pricing snapshot (`pricing_snapshot_json`); field names match the backend. */
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
  /** Reason for billing outside normal settlement, e.g. upstream usage still settled after a moderation block (see MODERATION_BLOCKED_BILLED_REASON). */
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

/** Matches backend `billing.BilledReasonModerationBlockedUpstreamUsage`: a block only retracts content, not upstream usage already incurred. */
export const MODERATION_BLOCKED_BILLED_REASON = "moderation_blocked_upstream_usage";

export function parseBillingSnapshot(value: string | undefined): BillingSnapshot {
  if (!value?.trim()) {
    return {};
  }
  const parsed = parseJSON(value);
  return isRecord(parsed) ? normalizeBillingSnapshot(parsed) : {};
}

const BILLING_SNAPSHOT_STRING_KEYS = [
  "pricing_mode",
  "provider_protocol",
  "cache_timeout",
  "billing_speed",
  "billing_service_tier",
  "billed_reason",
] as const satisfies readonly (keyof BillingSnapshot)[];

const BILLING_SNAPSHOT_NUMBER_KEYS = [
  "rate_multiplier",
  "cache_write_5m_tokens",
  "cache_write_1h_tokens",
  "input_nanousd_per_m_tokens",
  "cache_read_nanousd_per_m_tokens",
  "cache_write_nanousd_per_m_tokens",
  "output_nanousd_per_m_tokens",
  "call_nanousd_per_call",
  "duration_nanousd_per_second",
  "input_billed_nanousd",
  "cache_read_billed_nanousd",
  "cache_write_billed_nanousd",
  "output_billed_nanousd",
  "call_billed_nanousd",
  "duration_billed_nanousd",
  "tiered_from_tokens",
] as const satisfies readonly (keyof BillingSnapshot)[];

const BILLING_SERVICE_ITEM_STRING_KEYS = [
  "service_code",
  "service_name",
  "pricing_mode",
] as const satisfies readonly (keyof BillingServiceItemSnapshot)[];

const BILLING_SERVICE_ITEM_NUMBER_KEYS = [
  "call_count",
  "call_nanousd_per_call",
  "billed_nanousd",
] as const satisfies readonly (keyof BillingServiceItemSnapshot)[];

// Fields with an unexpected type are dropped, so display code can rely on the declared types.
function normalizeBillingServiceItem(record: UnknownRecord): BillingServiceItemSnapshot {
  const item: BillingServiceItemSnapshot = {};
  for (const key of BILLING_SERVICE_ITEM_STRING_KEYS) {
    const field = readString(record, key);
    if (field !== undefined) item[key] = field;
  }
  for (const key of BILLING_SERVICE_ITEM_NUMBER_KEYS) {
    const field = readFiniteNumber(record, key);
    if (field !== undefined) item[key] = field;
  }
  return item;
}

function normalizeBillingSnapshot(record: UnknownRecord): BillingSnapshot {
  const snapshot: BillingSnapshot = {};
  for (const key of BILLING_SNAPSHOT_STRING_KEYS) {
    const field = readString(record, key);
    if (field !== undefined) snapshot[key] = field;
  }
  for (const key of BILLING_SNAPSHOT_NUMBER_KEYS) {
    const field = readFiniteNumber(record, key);
    if (field !== undefined) snapshot[key] = field;
  }
  const fastMode = readBoolean(record, "fast_mode");
  if (fastMode !== undefined) snapshot.fast_mode = fastMode;
  const isFreeModel = readBoolean(record, "is_free_model");
  if (isFreeModel !== undefined) snapshot.is_free_model = isFreeModel;
  const tieredUpTo = record.tiered_up_to_tokens;
  if (tieredUpTo === null || isFiniteNumber(tieredUpTo)) snapshot.tiered_up_to_tokens = tieredUpTo;
  if (Array.isArray(record.service_items)) {
    snapshot.service_items = record.service_items.filter(isRecord).map(normalizeBillingServiceItem);
  }
  return snapshot;
}
