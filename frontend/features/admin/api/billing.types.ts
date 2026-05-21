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

export type AdminBillingConfigDTO = {
  mode: AdminBillingMode;
  prepaidAmountUSD: number;
  prepaidAmountNanousd: number;
  paymentProviders: Array<"stripe" | "epay" | string>;
  usdToCNYRate: number;
  epayTypes: Array<{ name: string; type: string }>;
};

export type UpdateAdminBillingConfigRequest = {
  mode: AdminBillingMode;
  prepaidAmountUSD?: number;
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

export type AdminModelPricingPage = PagePayload<AdminModelPricingDTO>;
