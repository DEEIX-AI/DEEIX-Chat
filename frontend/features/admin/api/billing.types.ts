import type {
  BatchDeleteRedemptionCodeDataResponse,
  BatchDeleteRedemptionCodeRequest,
  BatchDeleteRedemptionCodeResultResponse,
  BillingAccountDataResponse,
  BillingAccountResponse,
  BillingConfigDataResponse,
  BillingConfigRequest,
  BillingConfigResponse,
  BillingPlanDataResponse,
  BillingPlanResponse,
  BillingPriceResponse,
  CreateRedemptionCodeRequest,
  ModelPricingDataResponse,
  ModelPricingResponse,
  NativeToolPricingResponse,
  NativeToolPricingRequest,
  OpenRouterOfficialPricingDataResponse,
  OpenRouterOfficialPricingItemResponse,
  PatchRedemptionCodeRequestDoc,
  RedemptionCodeCreateDataResponse,
  RedemptionCodeDataResponse,
  RedemptionCodeDeleteDataResponse,
  RedemptionCodeResponse,
  UpdateBillingAccountBalanceRequest,
  UpdateBillingPlanRequest,
  UpsertModelPricingRequest,
} from "@deeix/api-contract";
import type { PagePayload } from "@/shared/api/common.types";

export type AdminBillingPlanPriceDTO = BillingPriceResponse;

export type AdminBillingPlanDTO = Omit<BillingPlanResponse, "prices"> & {
  prices: AdminBillingPlanPriceDTO[];
};

export type AdminModelPricingDTO = ModelPricingResponse;

export type UpsertAdminModelPricingRequest = UpsertModelPricingRequest;

export type AdminModelPricingData = Omit<ModelPricingDataResponse, "modelPricing"> & {
  modelPricing: AdminModelPricingDTO;
};

export type AdminOfficialPricingCatalogItemDTO = OpenRouterOfficialPricingItemResponse;

export type AdminOfficialPricingCatalogData = Omit<OpenRouterOfficialPricingDataResponse, "items"> & {
  items: AdminOfficialPricingCatalogItemDTO[];
};

export type UpdateAdminBillingPlanRequest = Omit<UpdateBillingPlanRequest, "billingInterval" | "permissionGroupID"> & {
  billingInterval: "month" | "year" | "lifetime" | string;
  permissionGroupID?: number | null;
};

export type AdminBillingPlanData = Omit<BillingPlanDataResponse, "plan"> & {
  plan: AdminBillingPlanDTO;
};

export type AdminBillingMode = "self" | "period" | "usage";

export type NativeToolPricingDTO = NativeToolPricingResponse;
export type AdminNativeToolPricingPayload = NativeToolPricingRequest;

export type AdminBillingConfigDTO = Omit<BillingConfigResponse, "epayTypes" | "mode" | "nativeToolPricing"> & {
  mode: AdminBillingMode;
  nativeToolPricing: NativeToolPricingDTO[];
  epayTypes: Array<{ name: string; type: string }>;
};

export type UpdateAdminBillingConfigRequest = Omit<BillingConfigRequest, "mode" | "nativeToolPricing"> & {
  mode: AdminBillingMode;
  nativeToolPricing?: AdminNativeToolPricingPayload[];
};

export type AdminBillingConfigData = Omit<BillingConfigDataResponse, "config"> & {
  config: AdminBillingConfigDTO;
};

export type AdminBillingAccountDTO = BillingAccountResponse;

export type AdminBillingAccountData = Omit<BillingAccountDataResponse, "account"> & {
  account: AdminBillingAccountDTO;
};

export type UpdateAdminBillingAccountBalanceRequest = UpdateBillingAccountBalanceRequest;

export type AdminRedemptionCodeDTO = Omit<
  RedemptionCodeResponse,
  "code"
> & {
  code?: string;
};

export type CreateAdminRedemptionCodeRequest = CreateRedemptionCodeRequest;

export type UpdateAdminRedemptionCodeRequest = PatchRedemptionCodeRequestDoc;

export type AdminRedemptionCodePage = PagePayload<AdminRedemptionCodeDTO>;

export type AdminRedemptionCodeCreateData = Omit<RedemptionCodeCreateDataResponse, "results"> & {
  results: AdminRedemptionCodeDTO[];
};

export type AdminRedemptionCodeData = Omit<RedemptionCodeDataResponse, "code"> & {
  code: AdminRedemptionCodeDTO;
};

export type AdminRedemptionCodeDeleteData = RedemptionCodeDeleteDataResponse;

export type AdminRedemptionCodeBatchDeleteRequest = BatchDeleteRedemptionCodeRequest;

export type AdminRedemptionCodeBatchDeleteResult = Omit<BatchDeleteRedemptionCodeResultResponse, "status"> & {
  status: "deleted" | "not_found" | "failed" | string;
  error?: string;
};

export type AdminRedemptionCodeBatchDeleteData = Omit<BatchDeleteRedemptionCodeDataResponse, "results"> & {
  results: AdminRedemptionCodeBatchDeleteResult[];
};

export type AdminModelPricingPage = PagePayload<AdminModelPricingDTO>;
