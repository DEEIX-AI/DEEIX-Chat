import type { AdminOfficialPricingCatalogItemDTO, UpsertAdminModelPricingRequest } from "@/features/admin/api/billing.types";
import {
  parseTieredPricingJSON,
  type BillingModelPricingRow,
  type PricingFormState,
} from "@/features/admin/model/billing-settings";

export type OfficialPricingCatalogItem = AdminOfficialPricingCatalogItemDTO;

export type OfficialModelPricingSuggestion = {
  item: OfficialPricingCatalogItem;
  score: number;
  reason: "exact" | "vendor" | "similar";
  payload: UpsertAdminModelPricingRequest | null;
  ignoredFields: string[];
  unsupportedFields: string[];
};

const VENDOR_ALIASES: Record<string, string[]> = {
  anthropic: ["anthropic"],
  cohere: ["cohere"],
  deepseek: ["deepseek"],
  google: ["google"],
  gemini: ["google"],
  meta: ["meta-llama", "meta"],
  "meta-llama": ["meta-llama", "meta"],
  microsoft: ["microsoft"],
  mistral: ["mistralai", "mistral"],
  mistralai: ["mistralai", "mistral"],
  moonshot: ["moonshotai", "moonshot"],
  moonshotai: ["moonshotai", "moonshot"],
  openai: ["openai"],
  qwen: ["qwen", "alibaba"],
  alibaba: ["qwen", "alibaba"],
  xai: ["x-ai", "xai"],
  "x-ai": ["x-ai", "xai"],
  zai: ["z-ai", "zai"],
  "z-ai": ["z-ai", "zai"],
};

function normalizeKey(value: string): string {
  return value
    .toLowerCase()
    .replace(/[:_/\\.\s]+/g, "-")
    .replace(/[^a-z0-9-]+/g, "")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");
}

function tokenize(value: string): string[] {
  return normalizeKey(value)
    .split("-")
    .filter((part) => part.length > 1 && part !== "model" && part !== "latest");
}

function vendorAliases(vendor: string): string[] {
  const normalized = normalizeKey(vendor);
  return VENDOR_ALIASES[normalized] ?? (normalized ? [normalized] : []);
}

function candidateModelIDs(row: Pick<BillingModelPricingRow, "platformModelName" | "vendor">): string[] {
  const rawName = row.platformModelName.trim().toLowerCase();
  const values = new Set<string>();
  if (!rawName) return [];
  values.add(rawName);
  values.add(normalizeKey(rawName));
  for (const alias of vendorAliases(row.vendor)) {
    if (!rawName.includes("/")) {
      values.add(`${alias}/${rawName}`);
      values.add(normalizeKey(`${alias}/${rawName}`));
    }
  }
  return Array.from(values).filter(Boolean);
}

function tokenSimilarity(left: string[], right: string[]): number {
  if (left.length === 0 || right.length === 0) return 0;
  const rightSet = new Set(right);
  const hits = left.filter((token) => rightSet.has(token)).length;
  return hits / left.length;
}

function compactCatalogID(id: string): string {
  const parts = id.split("/");
  return parts.length > 1 ? parts.slice(1).join("/") : id;
}

function queryFieldScore(normalizedQuery: string, field: string): number {
  const normalizedField = normalizeKey(field);
  if (!normalizedField) return 0;
  if (normalizedField === normalizedQuery) return 100;
  const closeness = Math.min(1, normalizedQuery.length / normalizedField.length);
  if (normalizedField.startsWith(`${normalizedQuery}-`)) {
    return Math.round(78 + closeness * 16);
  }
  if (normalizedField.endsWith(`-${normalizedQuery}`)) {
    return Math.round(76 + closeness * 14);
  }
  if (normalizedField.includes(normalizedQuery)) {
    return Math.round(68 + closeness * 18);
  }
  return 0;
}

function searchCatalogItemScore(query: string, item: OfficialPricingCatalogItem): number {
  const normalizedQuery = normalizeKey(query);
  const fields = Array.from(new Set([
    item.id,
    compactCatalogID(item.id),
    item.canonicalSlug,
    compactCatalogID(item.canonicalSlug),
    item.name,
  ].filter(Boolean)));
  const fieldScore = Math.max(0, ...fields.map((field) => queryFieldScore(normalizedQuery, field)));
  const tokenScore = Math.round(48 + tokenSimilarity(tokenize(query), tokenize(fields.join(" "))) * 34);
  return Math.max(fieldScore, tokenScore);
}

function scoreCatalogItem(
  row: BillingModelPricingRow,
  item: OfficialPricingCatalogItem,
): Omit<OfficialModelPricingSuggestion, "payload" | "ignoredFields" | "unsupportedFields"> | null {
  const candidateIDs = candidateModelIDs(row);
  const itemIDs = [item.id, item.canonicalSlug].filter(Boolean);
  const normalizedItemIDs = itemIDs.map(normalizeKey);

  for (const candidate of candidateIDs) {
    if (itemIDs.includes(candidate)) {
      return { item, score: 100, reason: "exact" };
    }
    const normalized = normalizeKey(candidate);
    if (normalizedItemIDs.includes(normalized)) {
      return { item, score: 96, reason: "exact" };
    }
    if (itemIDs.some((id) => id.endsWith(`/${candidate}`)) || normalizedItemIDs.some((id) => id.endsWith(`-${normalized}`))) {
      return { item, score: 92, reason: "vendor" };
    }
  }

  const rowTokens = tokenize(`${row.vendor} ${row.platformModelName}`);
  const itemTokens = tokenize(`${item.id} ${item.canonicalSlug} ${item.name}`);
  const similarity = tokenSimilarity(rowTokens, itemTokens);
  const vendorBonus = vendorAliases(row.vendor).some((alias) => item.id.startsWith(`${alias}/`)) ? 8 : 0;
  const score = Math.min(89, Math.round(48 + similarity * 36 + vendorBonus));
  return score >= 64 ? { item, score, reason: "similar" } : null;
}

function pricePerMillion(raw: string | undefined): number | null {
  if (raw === undefined || raw === null || raw.trim() === "") return 0;
  const value = Number(raw.trim());
  if (!Number.isFinite(value) || value < 0) return null;
  return Number((value * 1_000_000).toFixed(6));
}

function requiredPricePerMillion(raw: string | undefined): number | null {
  if (raw === undefined || raw === null || raw.trim() === "") return null;
  return pricePerMillion(raw);
}

function requiredOfficialPricingFields(item: OfficialPricingCatalogItem): string[] {
  const missing: string[] = [];
  if (requiredPricePerMillion(item.pricing.prompt) == null) {
    missing.push("prompt");
  }
  if (requiredPricePerMillion(item.pricing.completion) == null) {
    missing.push("completion");
  }
  return missing;
}

function uniqueOfficialPricingFields(fields: string[]): string[] {
  return Array.from(new Set(fields.filter((field) => field.trim())));
}

export function formatOfficialPricingValue(value: number): string {
  if (!Number.isFinite(value) || value <= 0) {
    return "0.000";
  }
  return new Intl.NumberFormat("en-US", { minimumFractionDigits: 3, maximumFractionDigits: 3 }).format(value);
}

export function officialPricingPayload(
  row: Pick<BillingModelPricingRow, "platformModelName">,
  item: OfficialPricingCatalogItem,
  multiplier = 1,
): UpsertAdminModelPricingRequest | null {
  if (!Number.isFinite(multiplier) || multiplier <= 0) return null;
  const tokenPrices = (pricing: Pick<OfficialPricingCatalogItem["pricing"], "prompt" | "completion" | "inputCacheRead" | "inputCacheWrite">) => {
    const input = requiredPricePerMillion(pricing.prompt);
    const output = requiredPricePerMillion(pricing.completion);
    if (input == null || output == null) return null;
    const scale = (value: number) => Number((value * multiplier).toFixed(6));
    return {
      inputUSDPerMTokens: scale(input),
      outputUSDPerMTokens: scale(output),
      cacheReadUSDPerMTokens: scale(pricePerMillion(pricing.inputCacheRead) ?? 0),
      cacheWriteUSDPerMTokens: scale(pricePerMillion(pricing.inputCacheWrite) ?? 0),
    };
  };
  const base = tokenPrices(item.pricing);
  if (!base) return null;
  const tiers: Array<typeof base & { upToTokens: number }> = [];
  let current = base;
  let previousThreshold = 0;
  // The backend has already resolved source precedence and inherited prices.
  for (const override of item.pricing.overrides ?? []) {
    const next = tokenPrices(override);
    if (!next || !Number.isSafeInteger(override.minPromptTokens) || override.minPromptTokens <= previousThreshold) continue;
    tiers.push({ upToTokens: override.minPromptTokens, ...current });
    current = next;
    previousThreshold = override.minPromptTokens;
  }
  const isTiered = tiers.length > 0;
  if (isTiered) tiers.push({ upToTokens: 0, ...current });
  const isFree = (isTiered ? tiers : [base]).every((price) =>
    price.inputUSDPerMTokens === 0 && price.outputUSDPerMTokens === 0 &&
    price.cacheReadUSDPerMTokens === 0 && price.cacheWriteUSDPerMTokens === 0,
  );
  return {
    ...base,
    platformModelName: row.platformModelName,
    currency: "USD",
    isFree,
    pricingMode: isTiered ? "tiered" : "token",
    cacheWritePriceBasis: item.pricing.cacheWritePriceBasis,
    callUSDPerCall: 0,
    durationUSDPerSecond: 0,
    ...(isTiered ? { tieredPricingJSON: JSON.stringify({ tiers }) } : {}),
  };
}

function suggestionForItem(
  row: BillingModelPricingRow,
  item: OfficialPricingCatalogItem,
  scored: Omit<OfficialModelPricingSuggestion, "payload" | "ignoredFields" | "unsupportedFields">,
): OfficialModelPricingSuggestion {
  const payload = officialPricingPayload(row, item);
  const fields = item.pricing.unsupportedFields ?? [];
  const unavailableFields = requiredOfficialPricingFields(item);
  const ignoredFields = fields.filter((field) => !unavailableFields.includes(field));
  const unavailableReasonFields = uniqueOfficialPricingFields([
    ...unavailableFields,
    ...fields.filter((field) => unavailableFields.includes(field)),
  ]);
  const unsupportedFields = payload
    ? []
    : unavailableReasonFields.length > 0
      ? unavailableReasonFields
      : ["pricing"];
  return { ...scored, payload, ignoredFields, unsupportedFields };
}

export function findOfficialPricingSuggestions(
  row: BillingModelPricingRow,
  catalog: OfficialPricingCatalogItem[],
  limit = 5,
): OfficialModelPricingSuggestion[] {
  const suggestions: OfficialModelPricingSuggestion[] = [];
  for (const item of catalog) {
    const scored = scoreCatalogItem(row, item);
    if (!scored) continue;
    suggestions.push(suggestionForItem(row, item, scored));
  }
  return suggestions
    .sort((left, right) => right.score - left.score || left.item.id.localeCompare(right.item.id))
    .slice(0, limit);
}

export function searchOfficialPricingCatalog(
  query: string,
  row: BillingModelPricingRow,
  catalog: OfficialPricingCatalogItem[],
  limit = 8,
): OfficialModelPricingSuggestion[] {
  const normalizedQuery = normalizeKey(query);
  if (!normalizedQuery) {
    return findOfficialPricingSuggestions(row, catalog, limit);
  }
  const suggestions: OfficialModelPricingSuggestion[] = [];
  for (const item of catalog) {
    const score = searchCatalogItemScore(query, item);
    if (score < 62) continue;
    suggestions.push(suggestionForItem(row, item, {
      item,
      score,
      reason: score >= 96 ? "exact" : "similar",
    }));
  }
  return suggestions
    .sort((left, right) => right.score - left.score || left.item.id.localeCompare(right.item.id))
    .slice(0, limit);
}

export function applyOfficialPricingToForm(form: PricingFormState, payload: UpsertAdminModelPricingRequest): PricingFormState {
  const tiered = payload.pricingMode === "tiered";
  return {
    ...form,
    pricingMode: tiered ? "tiered" : "token",
    cacheWritePriceBasis: payload.cacheWritePriceBasis,
    input: tiered ? "0" : String(payload.inputUSDPerMTokens),
    cacheRead: tiered ? "0" : String(payload.cacheReadUSDPerMTokens),
    cacheWrite: tiered ? "0" : String(payload.cacheWriteUSDPerMTokens),
    output: tiered ? "0" : String(payload.outputUSDPerMTokens),
    tieredTiers: tiered ? parseTieredPricingJSON(payload.tieredPricingJSON) ?? form.tieredTiers : form.tieredTiers,
    call: "0",
    duration: "0",
    isFree: payload.isFree,
  };
}
