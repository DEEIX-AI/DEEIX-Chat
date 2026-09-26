import { authedRequest } from "@/shared/api/authed-client";
import type { PagePayload } from "@/shared/api/common.types";
import type {
  BillingAccountData,
  BillingConfigData,
  BillingOverviewData,
  BillingUsageDailyDTO,
  BillingPlanDTO,
  BillingUsageLedgerDTO,
  BillingUsageMonthlyDTO,
  CheckoutData,
  CreateCheckoutRequest,
  RedeemBillingCodeData,
  RedeemBillingCodeRequest,
  SubscribeData,
} from "@/shared/api/billing.types";

export async function getBillingConfig(accessToken: string): Promise<BillingConfigData> {
  return authedRequest<BillingConfigData>("/api/v1/billing/config", { accessToken }, true);
}

export async function listBillingPlans(accessToken: string): Promise<BillingPlanDTO[]> {
  return authedRequest<BillingPlanDTO[]>("/api/v1/billing/plans", { accessToken }, true);
}

export async function getBillingAccount(accessToken: string): Promise<BillingAccountData> {
  return authedRequest<BillingAccountData>("/api/v1/billing/account", { accessToken }, true);
}

export async function getBillingOverview(accessToken: string): Promise<BillingOverviewData> {
  return authedRequest<BillingOverviewData>("/api/v1/billing/overview", { accessToken }, true);
}

export async function listBillingUsage(
  accessToken: string,
  options: { page?: number; pageSize?: number; query?: string; status?: string; sort?: string } = {},
): Promise<PagePayload<BillingUsageLedgerDTO>> {
  const page = options.page && options.page > 0 ? options.page : 1;
  const pageSize = options.pageSize && options.pageSize > 0 ? options.pageSize : 10;
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  if (options.query?.trim()) params.set("query", options.query.trim());
  if (options.status?.trim()) params.set("status", options.status.trim());
  if (options.sort?.trim()) params.set("sort", options.sort.trim());
  return authedRequest<PagePayload<BillingUsageLedgerDTO>>(
    `/api/v1/billing/usage?${params.toString()}`,
    { accessToken },
    true,
  );
}

export async function listBillingMonthlyUsage(accessToken: string, months = 12): Promise<BillingUsageMonthlyDTO[]> {
  const params = new URLSearchParams({ months: String(months) });
  return authedRequest<BillingUsageMonthlyDTO[]>(
    `/api/v1/billing/usage/monthly?${params.toString()}`,
    { accessToken },
    true,
  );
}

export async function listBillingDailyUsage(
  accessToken: string,
  options: { days?: number; startDate?: string; endDate?: string } = {},
): Promise<BillingUsageDailyDTO[]> {
  const params = new URLSearchParams();
  if (options.startDate && options.endDate) {
    params.set("start_date", options.startDate);
    params.set("end_date", options.endDate);
  } else if (options.days && options.days > 0) {
    params.set("days", String(options.days && options.days > 0 ? options.days : 30));
  }
  const query = params.toString();
  return authedRequest<BillingUsageDailyDTO[]>(
    `/api/v1/billing/usage/daily${query ? `?${query}` : ""}`,
    { accessToken },
    true,
  );
}

export async function createBillingCheckout(accessToken: string, payload: CreateCheckoutRequest): Promise<CheckoutData> {
  return authedRequest<CheckoutData>(
    "/api/v1/billing/payments/checkout",
    { method: "POST", accessToken, body: payload },
    true,
  );
}

export async function redeemBillingCode(accessToken: string, payload: RedeemBillingCodeRequest): Promise<RedeemBillingCodeData> {
  return authedRequest<RedeemBillingCodeData>(
    "/api/v1/billing/redemptions",
    { method: "POST", accessToken, body: payload },
    true,
  );
}

export async function subscribeBillingPlan(accessToken: string, priceID: number): Promise<SubscribeData> {
  return authedRequest<SubscribeData>(
    "/api/v1/billing/subscriptions",
    { method: "POST", accessToken, body: { priceID: priceID, cycles: 1 } },
    true,
  );
}
