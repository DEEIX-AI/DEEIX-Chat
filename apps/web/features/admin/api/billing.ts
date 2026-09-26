import { authedRequest } from "@/shared/api/authed-client";
import type { PagePayload } from "@/shared/api/common.types";
import type {
  AdminBillingConfigData,
  AdminBillingAccountData,
  AdminBillingPlanDTO,
  AdminBillingPlanData,
  AdminRedemptionCodeDTO,
  AdminRedemptionCodeBatchDeleteData,
  AdminRedemptionCodeBatchDeleteRequest,
  AdminRedemptionCodeCreateData,
  AdminRedemptionCodeData,
  AdminRedemptionCodeDeleteData,
  AdminRedemptionCodePage,
  AdminModelPricingDTO,
  AdminModelPricingData,
  AdminModelPricingPage,
  AdminOfficialPricingCatalogData,
  CreateAdminRedemptionCodeRequest,
  UpdateAdminRedemptionCodeRequest,
  UpdateAdminBillingConfigRequest,
  UpdateAdminBillingPlanRequest,
  UpdateAdminBillingAccountBalanceRequest,
  UpsertAdminModelPricingRequest,
} from "@/features/admin/api/billing.types";

import { normalizeAdminPagePayload, resolveAdminPage, type AdminPageOptions } from "./shared";

type ListAdminModelPricingOptions = AdminPageOptions & {
  query?: string;
};

type ListAdminRedemptionCodeOptions = AdminPageOptions & {
  query?: string;
  mode?: string;
  status?: string;
  availability?: string;
};

export async function listAdminBillingPlans(accessToken: string): Promise<AdminBillingPlanDTO[]> {
  return authedRequest<AdminBillingPlanDTO[]>("/api/v1/admin/billing/plans", { accessToken }, true);
}

export async function updateAdminBillingPlan(
  accessToken: string,
  planID: number,
  payload: UpdateAdminBillingPlanRequest,
): Promise<AdminBillingPlanData> {
  return authedRequest<AdminBillingPlanData>(
    `/api/v1/admin/billing/plans/${planID}`,
    { method: "PATCH", accessToken, body: payload },
    true,
  );
}

export async function getAdminBillingConfig(accessToken: string): Promise<AdminBillingConfigData> {
  return authedRequest<AdminBillingConfigData>("/api/v1/admin/billing/config", { accessToken }, true);
}

export async function patchAdminBillingConfig(accessToken: string, payload: UpdateAdminBillingConfigRequest): Promise<AdminBillingConfigData> {
  return authedRequest<AdminBillingConfigData>(
    "/api/v1/admin/billing/config",
    { method: "PATCH", accessToken, body: payload },
    true,
  );
}

export async function updateAdminBillingAccountBalance(
  accessToken: string,
  userID: number,
  payload: UpdateAdminBillingAccountBalanceRequest,
): Promise<AdminBillingAccountData> {
  return authedRequest<AdminBillingAccountData>(
    `/api/v1/admin/billing/accounts/${userID}/balance`,
    { method: "PATCH", accessToken, body: payload },
    true,
  );
}

export async function listAdminRedemptionCodes(
  accessToken: string,
  options: ListAdminRedemptionCodeOptions = {},
): Promise<AdminRedemptionCodePage> {
  const { page, pageSize } = resolveAdminPage(options);
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  if (options.query?.trim()) params.set("q", options.query.trim());
  if (options.mode?.trim()) params.set("mode", options.mode.trim());
  if (options.status?.trim()) params.set("status", options.status.trim());
  if (options.availability?.trim()) params.set("availability", options.availability.trim());
  const data = await authedRequest<PagePayload<AdminRedemptionCodeDTO>>(
    `/api/v1/admin/billing/redemption-codes?${params.toString()}`,
    { accessToken },
    true,
  );
  return normalizeAdminPagePayload(data);
}

export async function createAdminRedemptionCodes(
  accessToken: string,
  payload: CreateAdminRedemptionCodeRequest,
): Promise<AdminRedemptionCodeCreateData> {
  return authedRequest<AdminRedemptionCodeCreateData>(
    "/api/v1/admin/billing/redemption-codes",
    { method: "POST", accessToken, body: payload },
    true,
  );
}

export async function updateAdminRedemptionCode(
  accessToken: string,
  codeID: number,
  payload: UpdateAdminRedemptionCodeRequest,
): Promise<AdminRedemptionCodeData> {
  return authedRequest<AdminRedemptionCodeData>(
    `/api/v1/admin/billing/redemption-codes/${codeID}`,
    { method: "PATCH", accessToken, body: payload },
    true,
  );
}

export async function revealAdminRedemptionCode(
  accessToken: string,
  codeID: number,
): Promise<AdminRedemptionCodeData> {
  return authedRequest<AdminRedemptionCodeData>(
    `/api/v1/admin/billing/redemption-codes/${codeID}/code`,
    { accessToken },
    true,
  );
}

export async function deleteAdminRedemptionCode(
  accessToken: string,
  codeID: number,
): Promise<AdminRedemptionCodeDeleteData> {
  return authedRequest<AdminRedemptionCodeDeleteData>(
    `/api/v1/admin/billing/redemption-codes/${codeID}`,
    { method: "DELETE", accessToken },
    true,
  );
}

export async function batchDeleteAdminRedemptionCodes(
  accessToken: string,
  payload: AdminRedemptionCodeBatchDeleteRequest,
): Promise<AdminRedemptionCodeBatchDeleteData> {
  return authedRequest<AdminRedemptionCodeBatchDeleteData>(
    "/api/v1/admin/billing/redemption-codes/batch-delete",
    { method: "POST", accessToken, body: payload },
    true,
  );
}

export async function listAdminModelPricing(
  accessToken: string,
  options: ListAdminModelPricingOptions = {},
): Promise<AdminModelPricingPage> {
  const { page, pageSize } = resolveAdminPage(options);
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  if (options.query?.trim()) {
    params.set("q", options.query.trim());
  }
  const data = await authedRequest<PagePayload<AdminModelPricingDTO>>(
    `/api/v1/admin/billing/model-prices?${params.toString()}`,
    { accessToken },
    true,
  );
  return normalizeAdminPagePayload(data);
}

export async function upsertAdminModelPricing(
  accessToken: string,
  payload: UpsertAdminModelPricingRequest,
): Promise<AdminModelPricingData> {
  return authedRequest<AdminModelPricingData>(
    "/api/v1/admin/billing/model-prices",
    { method: "PUT", accessToken, body: payload },
    true,
  );
}

export async function getAdminOpenRouterOfficialPricing(
  accessToken: string,
  options: { refresh?: boolean } = {},
): Promise<AdminOfficialPricingCatalogData> {
  const params = new URLSearchParams();
  if (options.refresh) {
    params.set("refresh", "true");
  }
  const suffix = params.size > 0 ? `?${params.toString()}` : "";
  return authedRequest<AdminOfficialPricingCatalogData>(
    `/api/v1/admin/billing/official-pricing/openrouter${suffix}`,
    { accessToken },
    true,
  );
}
