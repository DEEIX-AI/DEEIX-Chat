import {
  getAdminBillingConfig,
  listAdminBillingPlans,
  listAdminModelPricing,
} from "./billing";
import { listAdminLLMModels } from "./llm";
import type { AdminBillingConfigData, AdminBillingPlanDTO, AdminModelPricingDTO } from "@/features/admin/api/billing.types";
import type { AdminLLMModelDTO } from "@/features/admin/api/llm.types";
import { listAllAdminPages } from "./shared";

type AdminReferenceData = {
  billingConfig: AdminBillingConfigData;
  billingPlans: AdminBillingPlanDTO[];
  models: AdminLLMModelDTO[];
  modelPricing: AdminModelPricingDTO[];
};

const CACHE_TTL_MS = 30_000;
let cachedReferenceData: { value: AdminReferenceData; expiresAt: number } | null = null;
let pendingReferenceData: Promise<AdminReferenceData> | null = null;

export function invalidateAdminReferenceDataCache(): void {
  cachedReferenceData = null;
  pendingReferenceData = null;
}

export async function getAdminReferenceData(accessToken: string): Promise<AdminReferenceData> {
  const now = Date.now();
  if (cachedReferenceData && cachedReferenceData.expiresAt > now) {
    return cachedReferenceData.value;
  }
  if (pendingReferenceData) {
    return pendingReferenceData;
  }

  pendingReferenceData = Promise.all([
    getAdminBillingConfig(accessToken),
    listAdminBillingPlans(accessToken),
    listAllAdminPages((options) => listAdminLLMModels(accessToken, { ...options, onlyActive: false, sort: "sortOrder_asc" })),
    listAllAdminPages((options) => listAdminModelPricing(accessToken, options)),
  ])
    .then(([billingConfig, billingPlans, models, modelPricing]) => {
      const value = { billingConfig, billingPlans, models, modelPricing };
      cachedReferenceData = {
        value,
        expiresAt: Date.now() + CACHE_TTL_MS,
      };
      return value;
    })
    .finally(() => {
      pendingReferenceData = null;
    });

  return pendingReferenceData;
}
