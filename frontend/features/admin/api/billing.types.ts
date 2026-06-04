import type { PagePayload } from "@/shared/api/common.types";

export type AdminBillingPlanPriceDTO = {
  id: number;
  planID: number;
  code: string;
  billingInterval: "month" | "year" | "lifetime" | string;
  currency: string;
  amountCents: number;
  isDefault: boolean;
};

export type AdminBillingPlanDTO = {
  id: number;
  code: "free" | "pro" | "max" | "ultra" | string;
  name: string;
  description: string;
  featureJSON: string;
  periodCreditUSD: number;
  periodCreditNanousd: number;
  discountPercent: number;
  sortOrder: number;
  isActive: boolean;
  prices: AdminBillingPlanPriceDTO[];
};

export type AdminModelPricingDTO = {
  id: number;
  platformModelName: string;
  modelVendor: string;
  modelIcon: string;
  currency: string;
  isFree: boolean;
  pricingMode: "token" | "call" | "duration" | "tiered" | string;
  inputUSDPerMTokens: number;
  cacheReadUSDPerMTokens: number;
  cacheWriteUSDPerMTokens: number;
  outputUSDPerMTokens: number;
  callUSDPerCall: number;
  durationUSDPerSecond: number;
  tieredPricingJSON: string;
  inputNanousdPerMTokens: number;
  cacheReadNanousdPerMTokens: number;
  cacheWriteNanousdPerMTokens: number;
  outputNanousdPerMTokens: number;
  callNanousdPerCall: number;
  durationNanousdPerSecond: number;
  createdAt: string;
  updatedAt: string;
};

export type UpsertAdminModelPricingRequest = {
  platformModelName: string;
  currency?: string;
  isFree: boolean;
  pricingMode: "token" | "call" | "duration" | "tiered" | string;
  inputUSDPerMTokens: number;
  cacheReadUSDPerMTokens: number;
  cacheWriteUSDPerMTokens: number;
  outputUSDPerMTokens: number;
  callUSDPerCall: number;
  durationUSDPerSecond: number;
  tieredPricingJSON?: string;
};

export type AdminModelPricingData = {
  modelPricing: AdminModelPricingDTO;
};

export type UpdateAdminBillingPlanRequest = {
  name: string;
  description: string;
  periodCreditUSD: number;
  discountPercent: number;
  currency?: string;
  amountUSD: number;
  billingInterval: "month" | "year" | "lifetime" | string;
};

export type AdminBillingPlanData = {
  plan: AdminBillingPlanDTO;
};

export type AdminBillingMode = "self" | "period" | "usage";

export type NativeToolPricingDTO = {
  provider: string;
  toolKey: string;
  label: string;
  description: string;
  type: string;
  priceNanousd: number;
  unit: "call" | "search" | string;
  priceLabel: "included" | "notMetered" | string;
  billable: boolean;
};

export type AdminBillingConfigDTO = {
  mode: AdminBillingMode;
  prepaidAmountUSD: number;
  prepaidAmountNanousd: number;
  nativeToolBillingEnabled: boolean;
  nativeToolPricing: NativeToolPricingDTO[];
  paymentProviders: Array<"stripe" | "epay" | string>;
  usdToCNYRate: number;
  epayTypes: Array<{ name: string; type: string }>;
};

export type UpdateAdminBillingConfigRequest = {
  mode: AdminBillingMode;
  prepaidAmountUSD?: number;
  nativeToolBillingEnabled?: boolean;
  nativeToolPricing?: NativeToolPricingDTO[];
};

export type AdminBillingConfigData = {
  config: AdminBillingConfigDTO;
};

export type AdminBillingAccountDTO = {
  userID: number;
  currency: string;
  balanceNanousd: number;
  balanceUSD: number;
  status: string;
  updatedAt: string;
};

export type AdminBillingAccountData = {
  account: AdminBillingAccountDTO;
};

export type UpdateAdminBillingAccountBalanceRequest = {
  balanceUSD: number;
  description?: string;
};

export type AdminRedemptionCodeDTO = {
  id: number;
  code?: string;
  codeHint: string;
  mode: "usage" | "period" | string;
  rewardType: "balance" | "subscription" | string;
  creditUSD: number;
  creditNanousd: number;
  planID: number;
  durationDays: number;
  maxRedemptions: number | null;
  perUserLimit: number;
  redeemedCount: number;
  remainingRedemptions: number | null;
  status: "active" | "inactive" | "deleted" | string;
  expiresAt: string | null;
  description: string;
  createdByUserID: number;
  createdAt: string;
  updatedAt: string;
};

export type CreateAdminRedemptionCodeRequest = {
  code?: string;
  quantity?: number;
  mode: "usage" | "period";
  creditUSD?: number;
  planID?: number;
  durationDays?: number;
  maxRedemptions?: number | null;
  perUserLimit: number;
  expiresAt?: string | null;
  description?: string;
};

export type UpdateAdminRedemptionCodeRequest = {
  status?: "active" | "inactive";
  maxRedemptions?: number | null;
  perUserLimit?: number;
  expiresAt?: string | null;
  description?: string;
};

export type AdminRedemptionCodePage = PagePayload<AdminRedemptionCodeDTO>;

export type AdminRedemptionCodeCreateData = {
  results: AdminRedemptionCodeDTO[];
};

export type AdminRedemptionCodeData = {
  code: AdminRedemptionCodeDTO;
};

export type AdminRedemptionCodeDeleteData = {
  deleted: boolean;
};

export type AdminRedemptionCodeBatchDeleteRequest = {
  ids: number[];
};

export type AdminRedemptionCodeBatchDeleteResult = {
  id: number;
  status: "deleted" | "not_found" | "failed" | string;
  error?: string;
};

export type AdminRedemptionCodeBatchDeleteData = {
  total: number;
  successCount: number;
  notFoundCount: number;
  failedCount: number;
  results: AdminRedemptionCodeBatchDeleteResult[];
};

export type AdminModelPricingPage = PagePayload<AdminModelPricingDTO>;
