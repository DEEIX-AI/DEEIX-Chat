import { authedRequest } from "@/shared/api/authed-client";
import type { PagePayload } from "@/shared/api/common.types";
import type {
  AdminBillingConfigData,
  AdminBillingAccountData,
  AdminBillingPlanDTO,
  AdminBillingPlanData,
  AdminModelPricingDTO,
  AdminModelPricingData,
  AdminModelPricingPage,
  UpdateAdminBillingConfigRequest,
  UpdateAdminBillingPlanRequest,
  UpdateAdminBillingAccountBalanceRequest,
  UpsertAdminModelPricingRequest,
} from "@/features/admin/api/billing.types";

import { normalizeAdminPagePayload, resolveAdminPage, type AdminPageOptions } from "./shared";

type ListAdminModelPricingOptions = AdminPageOptions & {
  query?: string;
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
