export type PublicModelPricingTierDTO = {
  fromTokens: number;
  upToTokens: number | null;
  inputUSDPerMTokens: number;
  cacheReadUSDPerMTokens: number;
  cacheWriteUSDPerMTokens: number;
  outputUSDPerMTokens: number;
};

export type PublicModelPricingDTO = {
  currency: string;
  isFree: boolean;
  mode: "token" | "call" | "duration" | "tiered" | string;
  inputUSDPerMTokens: number;
  cacheReadUSDPerMTokens: number;
  cacheWriteUSDPerMTokens: number;
  outputUSDPerMTokens: number;
  callUSDPerCall: number;
  durationUSDPerSecond: number;
  tiers: PublicModelPricingTierDTO[];
};

export type PublicModelDTO = {
  platformModelName: string;
  vendor: string;
  kindsJSON: string;
  icon: string;
  protocolsJSON: string;
  capabilitiesJSON: string;
  description: string;
  sortOrder: number;
  pricing: PublicModelPricingDTO | null;
};
