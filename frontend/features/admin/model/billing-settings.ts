import type {
  AdminBillingMode,
  AdminBillingPlanDTO,
  AdminModelPricingDTO,
  UpsertAdminModelPricingRequest,
} from "@/features/admin/api/billing.types";
import type { AdminLLMModelDTO } from "@/features/admin/api/llm.types";
import type { PatchSettingItem, SettingItem } from "@/shared/api/settings.types";

export type BillingModelPricingRow = {
  platformModelName: string;
  vendor: string;
  icon: string;
  pricing: AdminModelPricingDTO | null;
  isFree: boolean;
};

export type PricingMode = "token" | "call" | "duration" | "tiered";

export type TieredPricingTierForm = {
  id: string;
  upToKTokens: string;
  input: string;
  cacheRead: string;
  cacheWrite: string;
  output: string;
};

export type PricingFormState = {
  platformModelName: string;
  pricingMode: PricingMode;
  input: string;
  cacheRead: string;
  cacheWrite: string;
  output: string;
  call: string;
  duration: string;
  tieredTiers: TieredPricingTierForm[];
  isFree: boolean;
};

export type PlanFormState = {
  name: string;
  description: string;
  amount: string;
  billingInterval: string;
  periodCredit: string;
  discountPercent: string;
};

export type ModelPricingExportEntry = {
  currency: string;
  isFree: boolean;
  pricingMode: PricingMode;
  inputUSDPerMTokens: number;
  cacheReadUSDPerMTokens: number;
  cacheWriteUSDPerMTokens: number;
  outputUSDPerMTokens: number;
  callUSDPerCall: number;
  durationUSDPerSecond: number;
  tieredPricing?: unknown;
};

export type ModelPricingImportParseResult = {
  items: UpsertAdminModelPricingRequest[];
  errors: string[];
  unknownModelNames: string[];
};

export type ModelPricingImportMessages = {
  invalidJSON: string;
  rootObject: string;
  emptyModelName: string;
  duplicateModel: (model: string) => string;
  pricingObject: (model: string) => string;
  invalidPricingMode: (model: string) => string;
  invalidNumber: (model: string, field: string) => string;
  invalidTieredPricing: (model: string, field: string) => string;
  invalidTieredPricingJSON: (model: string) => string;
};

export const DEFAULT_PAGE_SIZE = 25;
export const PAYMENT_SETTING_KEYS = [
  "usd_to_cny_rate",
  "payment_providers",
  "stripe_publishable_key",
  "stripe_secret_key",
  "stripe_webhook_secret",
  "epay_gateway_url",
  "epay_types",
  "epay_pid",
  "epay_key",
] as const;
export type PaymentProvider = "stripe" | "epay";
export type PaymentSettings = Record<(typeof PAYMENT_SETTING_KEYS)[number], string>;

export const PAYMENT_DEFAULTS: PaymentSettings = {
  usd_to_cny_rate: "7.2",
  payment_providers: "disabled",
  stripe_publishable_key: "",
  stripe_secret_key: "",
  stripe_webhook_secret: "",
  epay_gateway_url: "",
  epay_types: `[
  {"name":"Alipay","type":"alipay"},
  {"name":"WeChat Pay","type":"wxpay"}
]`,
  epay_pid: "",
  epay_key: "",
};

const DEFAULT_TIERED_TIERS: TieredPricingTierForm[] = [
  {
    id: "default-1",
    upToKTokens: "200",
    input: "0",
    cacheRead: "0",
    cacheWrite: "0",
    output: "0",
  },
  {
    id: "default-2",
    upToKTokens: "0",
    input: "0",
    cacheRead: "0",
    cacheWrite: "0",
    output: "0",
  },
];

export const DIALOG_LAYOUT_TRANSITION = {
  layout: {
    duration: 0.22,
    ease: [0.16, 1, 0.3, 1] as const,
  },
};

export function formatUSD(value: number): string {
  if (!Number.isFinite(value) || value <= 0) {
    return "$0";
  }
  return `$${value.toLocaleString("en-US", {
    minimumFractionDigits: 0,
    maximumFractionDigits: 6,
  })}`;
}

export function formatAmountCents(cents: number, currency: string): string {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: currency || "USD",
  }).format((cents || 0) / 100);
}

export function formatCreditUSD(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "$0";
  return `$${value.toLocaleString("en-US", {
    minimumFractionDigits: 0,
    maximumFractionDigits: 2,
  })}`;
}

export function formatDateTime(value: string, locale = "en-US"): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function cloneDefaultTieredTiers(): TieredPricingTierForm[] {
  return DEFAULT_TIERED_TIERS.map((tier) => ({ ...tier }));
}

function parseTieredPricingJSON(raw: string | undefined): TieredPricingTierForm[] {
  if (!raw) return cloneDefaultTieredTiers();
  try {
    const parsed = JSON.parse(raw) as {
      tiers?: Array<{
        upToTokens?: number;
        inputUSDPerMTokens?: number;
        cacheReadUSDPerMTokens?: number;
        cacheWriteUSDPerMTokens?: number;
        outputUSDPerMTokens?: number;
      }>;
    };
    if (!Array.isArray(parsed.tiers) || parsed.tiers.length === 0) {
      return cloneDefaultTieredTiers();
    }
    return parsed.tiers.map((tier, index) => ({
      id: `saved-${index}-${tier.upToTokens ?? 0}`,
      upToKTokens: String(Math.ceil((tier.upToTokens ?? 0) / 1000)),
      input: String(tier.inputUSDPerMTokens ?? 0),
      cacheRead: String(tier.cacheReadUSDPerMTokens ?? 0),
      cacheWrite: String(tier.cacheWriteUSDPerMTokens ?? 0),
      output: String(tier.outputUSDPerMTokens ?? 0),
    }));
  } catch {
    return cloneDefaultTieredTiers();
  }
}

export function stringifyTieredPricing(tiers: TieredPricingTierForm[]): string {
  return JSON.stringify({
    tiers: tiers.map((tier) => ({
      upToTokens: parseIntValue(tier.upToKTokens) * 1000,
      inputUSDPerMTokens: parsePrice(tier.input),
      cacheReadUSDPerMTokens: parsePrice(tier.cacheRead),
      cacheWriteUSDPerMTokens: parsePrice(tier.cacheWrite),
      outputUSDPerMTokens: parsePrice(tier.output),
    })),
  });
}

export function createFormState(row: BillingModelPricingRow): PricingFormState {
  const pricing = row.pricing;
  return {
    platformModelName: row.platformModelName,
    pricingMode: normalizePricingMode(pricing?.pricingMode),
    input: String(pricing?.inputUSDPerMTokens ?? 0),
    cacheRead: String(pricing?.cacheReadUSDPerMTokens ?? 0),
    cacheWrite: String(pricing?.cacheWriteUSDPerMTokens ?? 0),
    output: String(pricing?.outputUSDPerMTokens ?? 0),
    call: String(pricing?.callUSDPerCall ?? 0),
    duration: String(pricing?.durationUSDPerSecond ?? 0),
    tieredTiers: parseTieredPricingJSON(pricing?.tieredPricingJSON),
    isFree: pricing?.isFree ?? row.isFree,
  };
}

export function createPlanFormState(plan: AdminBillingPlanDTO): PlanFormState {
  const defaultPrice = plan.prices.find((item) => item.isDefault) || plan.prices[0];
  return {
    name: plan.name || "",
    description: plan.description || "",
    amount: String((defaultPrice?.amountCents ?? 0) / 100),
    billingInterval: defaultPrice?.billingInterval || "month",
    periodCredit: String(plan.periodCreditUSD ?? 0),
    discountPercent: String(plan.discountPercent ?? 0),
  };
}

export function parsePrice(value: string): number {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed < 0) {
    return 0;
  }
  return parsed;
}

export function normalizePricingMode(value: string | null | undefined): PricingMode {
  if (value === "call" || value === "duration" || value === "tiered") return value;
  return "token";
}

function isPricingMode(value: unknown): value is PricingMode {
  return value === "token" || value === "call" || value === "duration" || value === "tiered";
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

const DEFAULT_IMPORT_MESSAGES: ModelPricingImportMessages = {
  invalidJSON: "File content is not valid JSON",
  rootObject: "Model pricing JSON must be an object keyed by platform model name",
  emptyModelName: "Platform model name cannot be empty",
  duplicateModel: (model) => `${model} appears more than once`,
  pricingObject: (model) => `${model} pricing must be an object`,
  invalidPricingMode: (model) => `${model}.pricingMode must be token, call, duration, or tiered`,
  invalidNumber: (model, field) => `${model}.${field} must be a number greater than or equal to 0`,
  invalidTieredPricing: (model, field) => `${model}.${field} must contain a non-empty tiers array`,
  invalidTieredPricingJSON: (model) => `${model}.tieredPricingJSON is not valid JSON`,
};

function numberFromPricingField(
  entry: Record<string, unknown>,
  key: string,
  errors: string[],
  platformModelName: string,
  messages: ModelPricingImportMessages,
): number {
  const value = entry[key];
  if (value === undefined || value === null || value === "") {
    return 0;
  }
  const parsed = typeof value === "number" ? value : typeof value === "string" ? Number(value) : NaN;
  if (!Number.isFinite(parsed) || parsed < 0) {
    errors.push(messages.invalidNumber(platformModelName, key));
    return 0;
  }
  return parsed;
}

function parseTieredPricingImportValue(
  entry: Record<string, unknown>,
  platformModelName: string,
  errors: string[],
  messages: ModelPricingImportMessages,
): string {
  const rawJSON = entry.tieredPricingJSON;
  if (typeof rawJSON === "string" && rawJSON.trim()) {
    try {
      const parsed = JSON.parse(rawJSON) as unknown;
      if (!isValidTieredPricingConfig(parsed)) {
        errors.push(messages.invalidTieredPricing(platformModelName, "tieredPricingJSON"));
        return "";
      }
      return JSON.stringify(parsed);
    } catch {
      errors.push(messages.invalidTieredPricingJSON(platformModelName));
      return "";
    }
  }

  const raw = entry.tieredPricing;
  if (!isValidTieredPricingConfig(raw)) {
    errors.push(messages.invalidTieredPricing(platformModelName, "tieredPricing"));
    return "";
  }
  return JSON.stringify(raw);
}

function isValidTieredPricingConfig(value: unknown): boolean {
  if (!isRecord(value) || !Array.isArray(value.tiers) || value.tiers.length === 0) {
    return false;
  }
  return value.tiers.every((tier) => {
    if (!isRecord(tier)) {
      return false;
    }
    const upToTokens = tier.upToTokens;
    return upToTokens === undefined || (typeof upToTokens === "number" && Number.isFinite(upToTokens) && upToTokens >= 0);
  });
}

function parseTieredPricingExportValue(raw: string): unknown {
  try {
    const parsed = JSON.parse(raw || "{}") as unknown;
    return isRecord(parsed) ? parsed : {};
  } catch {
    return {};
  }
}

export function buildModelPricingExportObject(pricingItems: AdminModelPricingDTO[]): Record<string, ModelPricingExportEntry> {
  const result: Record<string, ModelPricingExportEntry> = {};
  const sorted = [...pricingItems].sort((left, right) => left.platformModelName.localeCompare(right.platformModelName));
  for (const item of sorted) {
    const platformModelName = item.platformModelName.trim();
    if (!platformModelName) {
      continue;
    }
    const pricingMode = normalizePricingMode(item.pricingMode);
    result[platformModelName] = {
      currency: item.currency || "USD",
      isFree: item.isFree,
      pricingMode,
      inputUSDPerMTokens: pricingMode === "token" ? item.inputUSDPerMTokens : 0,
      cacheReadUSDPerMTokens: pricingMode === "token" ? item.cacheReadUSDPerMTokens : 0,
      cacheWriteUSDPerMTokens: pricingMode === "token" ? item.cacheWriteUSDPerMTokens : 0,
      outputUSDPerMTokens: pricingMode === "token" ? item.outputUSDPerMTokens : 0,
      callUSDPerCall: pricingMode === "call" ? item.callUSDPerCall : 0,
      durationUSDPerSecond: pricingMode === "duration" ? item.durationUSDPerSecond : 0,
      ...(pricingMode === "tiered" ? { tieredPricing: parseTieredPricingExportValue(item.tieredPricingJSON) } : {}),
    };
  }
  return result;
}

function modelPricingNanousd(value: number): number {
  if (!Number.isFinite(value) || value <= 0) {
    return 0;
  }
  return Math.round(value * 1_000_000_000);
}

export function mergeModelPricingItem(items: AdminModelPricingDTO[], item: AdminModelPricingDTO): AdminModelPricingDTO[] {
  const index = items.findIndex((current) => current.platformModelName === item.platformModelName);
  if (index < 0) {
    return [...items, item];
  }
  const next = [...items];
  next[index] = item;
  return next;
}

export function createOptimisticModelPricing(row: BillingModelPricingRow, payload: UpsertAdminModelPricingRequest): AdminModelPricingDTO {
  const pricingMode = normalizePricingMode(payload.pricingMode);
  const now = new Date().toISOString();
  const inputUSDPerMTokens = pricingMode === "token" ? payload.inputUSDPerMTokens : 0;
  const cacheReadUSDPerMTokens = pricingMode === "token" ? payload.cacheReadUSDPerMTokens : 0;
  const cacheWriteUSDPerMTokens = pricingMode === "token" ? payload.cacheWriteUSDPerMTokens : 0;
  const outputUSDPerMTokens = pricingMode === "token" ? payload.outputUSDPerMTokens : 0;
  const callUSDPerCall = pricingMode === "call" ? payload.callUSDPerCall : 0;
  const durationUSDPerSecond = pricingMode === "duration" ? payload.durationUSDPerSecond : 0;
  return {
    id: row.pricing?.id ?? 0,
    platformModelName: payload.platformModelName,
    modelVendor: row.pricing?.modelVendor || row.vendor,
    modelIcon: row.pricing?.modelIcon || row.icon,
    currency: payload.currency || row.pricing?.currency || "USD",
    isFree: payload.isFree,
    pricingMode,
    inputUSDPerMTokens,
    cacheReadUSDPerMTokens,
    cacheWriteUSDPerMTokens,
    outputUSDPerMTokens,
    callUSDPerCall,
    durationUSDPerSecond,
    tieredPricingJSON: pricingMode === "tiered" ? payload.tieredPricingJSON || "" : "",
    inputNanousdPerMTokens: modelPricingNanousd(inputUSDPerMTokens),
    cacheReadNanousdPerMTokens: modelPricingNanousd(cacheReadUSDPerMTokens),
    cacheWriteNanousdPerMTokens: modelPricingNanousd(cacheWriteUSDPerMTokens),
    outputNanousdPerMTokens: modelPricingNanousd(outputUSDPerMTokens),
    callNanousdPerCall: modelPricingNanousd(callUSDPerCall),
    durationNanousdPerSecond: modelPricingNanousd(durationUSDPerSecond),
    createdAt: row.pricing?.createdAt || now,
    updatedAt: now,
  };
}

export function parseModelPricingImportJSON(
  raw: string,
  knownPlatformModelNames: Set<string>,
  messages: ModelPricingImportMessages = DEFAULT_IMPORT_MESSAGES,
): ModelPricingImportParseResult {
  const errors: string[] = [];
  const unknownModelNames: string[] = [];
  const items: UpsertAdminModelPricingRequest[] = [];
  let parsed: unknown;

  try {
    parsed = JSON.parse(raw);
  } catch {
    return {
      items: [],
      errors: [messages.invalidJSON],
      unknownModelNames: [],
    };
  }

  if (!isRecord(parsed)) {
    return {
      items: [],
      errors: [messages.rootObject],
      unknownModelNames: [],
    };
  }

  const seen = new Set<string>();
  for (const [rawName, rawEntry] of Object.entries(parsed)) {
    const platformModelName = rawName.trim();
    if (!platformModelName) {
      errors.push(messages.emptyModelName);
      continue;
    }
    if (seen.has(platformModelName)) {
      errors.push(messages.duplicateModel(platformModelName));
      continue;
    }
    seen.add(platformModelName);

    if (!knownPlatformModelNames.has(platformModelName)) {
      unknownModelNames.push(platformModelName);
      continue;
    }
    if (!isRecord(rawEntry)) {
      errors.push(messages.pricingObject(platformModelName));
      continue;
    }
    if (!isPricingMode(rawEntry.pricingMode)) {
      errors.push(messages.invalidPricingMode(platformModelName));
      continue;
    }
    const entryErrors: string[] = [];
    const pricingMode = rawEntry.pricingMode;
    const tieredPricingJSON = pricingMode === "tiered"
      ? parseTieredPricingImportValue(rawEntry, platformModelName, entryErrors, messages)
      : undefined;
    const request: UpsertAdminModelPricingRequest = {
      platformModelName,
      currency: typeof rawEntry.currency === "string" && rawEntry.currency.trim() ? rawEntry.currency.trim() : "USD",
      isFree: typeof rawEntry.isFree === "boolean" ? rawEntry.isFree : false,
      pricingMode,
      inputUSDPerMTokens: pricingMode === "token" ? numberFromPricingField(rawEntry, "inputUSDPerMTokens", entryErrors, platformModelName, messages) : 0,
      cacheReadUSDPerMTokens: pricingMode === "token" ? numberFromPricingField(rawEntry, "cacheReadUSDPerMTokens", entryErrors, platformModelName, messages) : 0,
      cacheWriteUSDPerMTokens: pricingMode === "token" ? numberFromPricingField(rawEntry, "cacheWriteUSDPerMTokens", entryErrors, platformModelName, messages) : 0,
      outputUSDPerMTokens: pricingMode === "token" ? numberFromPricingField(rawEntry, "outputUSDPerMTokens", entryErrors, platformModelName, messages) : 0,
      callUSDPerCall: pricingMode === "call" ? numberFromPricingField(rawEntry, "callUSDPerCall", entryErrors, platformModelName, messages) : 0,
      durationUSDPerSecond: pricingMode === "duration" ? numberFromPricingField(rawEntry, "durationUSDPerSecond", entryErrors, platformModelName, messages) : 0,
      tieredPricingJSON,
    };
    if (entryErrors.length > 0) {
      errors.push(...entryErrors);
      continue;
    }
    items.push(request);
  }

  return { items, errors, unknownModelNames };
}

export function buildPricingRows(models: AdminLLMModelDTO[], pricingItems: AdminModelPricingDTO[]): BillingModelPricingRow[] {
  const pricingMap = new Map(pricingItems.map((item) => [item.platformModelName, item]));
  const groupedModels = new Map<string, AdminLLMModelDTO>();

  for (const model of models) {
    if (model.status !== "active") continue;
    if (model.activeSourceCount <= 0) continue;
    const platformModelName = model.platformModelName.trim();
    if (!platformModelName) continue;
    if (!groupedModels.has(platformModelName)) {
      groupedModels.set(platformModelName, model);
    }
  }

  return Array.from(groupedModels.entries())
    .map(([platformModelName, model]) => {
      const pricing = pricingMap.get(platformModelName) || null;
      return {
        platformModelName,
        vendor: pricing?.modelVendor || model.vendor || "",
        icon: pricing?.modelIcon || model.icon || "",
        pricing,
        isFree: pricing?.isFree ?? false,
      };
    });
}

export function parseIntValue(value: string): number {
  const parsed = Number.parseInt(value, 10);
  if (!Number.isFinite(parsed) || parsed < 0) {
    return 0;
  }
  return parsed;
}

export function flattenPaymentSettings(items: SettingItem[]): PaymentSettings {
  const next = { ...PAYMENT_DEFAULTS };
  for (const item of items) {
    if ((PAYMENT_SETTING_KEYS as readonly string[]).includes(item.key)) {
      next[item.key as keyof PaymentSettings] = item.value;
    }
  }
  return next;
}

export function paymentSettingsChanged(current: PaymentSettings, saved: PaymentSettings): boolean {
  return PAYMENT_SETTING_KEYS.some((key) => current[key] !== saved[key]);
}

export function normalizePaymentProviders(value: string): PaymentProvider[] {
  const providers: PaymentProvider[] = [];
  for (const part of value.split(",")) {
    const provider = part.trim();
    if ((provider === "stripe" || provider === "epay") && !providers.includes(provider)) {
      providers.push(provider);
    }
  }
  return providers;
}

export function paymentProviderSetting(providers: PaymentProvider[]): string {
  return providers.length > 0 ? providers.join(",") : "disabled";
}

export function parseEPayTypesJSON(value: string): boolean {
  try {
    const parsed = JSON.parse(value) as Array<{ name?: unknown; type?: unknown }>;
    return (
      Array.isArray(parsed) &&
      parsed.length > 0 &&
      parsed.every((item) => typeof item.name === "string" && item.name.trim() && typeof item.type === "string" && item.type.trim())
    );
  } catch {
    return false;
  }
}

export function paymentPatchItems(settings: PaymentSettings): PatchSettingItem[] {
  return PAYMENT_SETTING_KEYS.map((key) => ({
    namespace: "billing",
    key,
    value: settings[key].trim(),
  }));
}
