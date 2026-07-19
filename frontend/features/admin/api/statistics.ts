import type {
  UsageStatisticsMetricsResponse,
  UsageStatisticsModelRankResponse,
  UsageStatisticsResponse,
  UsageStatisticsTrendResponse,
  UsageStatisticsUserRankResponse,
} from "@deeix/api-contract";
import { authedRequest } from "@/shared/api/authed-client";

export type AdminUsageStatisticsRankBy = "cost" | "tokens" | "calls";
export type AdminUsageStatisticsBillingScope = "all" | "free" | "billable";
export type AdminUsageStatisticsSection = "all" | "models" | "users";

export type AdminUsageStatisticsMetricsDTO = UsageStatisticsMetricsResponse;

export type AdminUsageStatisticsTrendDTO = UsageStatisticsTrendResponse;

export type AdminUsageStatisticsModelRankDTO = Omit<UsageStatisticsModelRankResponse, "trend"> & {
  trend: AdminUsageStatisticsTrendDTO[];
};

export type AdminUsageStatisticsUserRankDTO = Omit<UsageStatisticsUserRankResponse, "trend"> & {
  trend: AdminUsageStatisticsTrendDTO[];
};

export type AdminUsageStatisticsData = Omit<
  UsageStatisticsResponse,
  "range" | "section" | "topModels" | "topUsers" | "totals" | "trend"
> & {
  section: AdminUsageStatisticsSection;
  range: UsageStatisticsResponse["range"];
  totals: AdminUsageStatisticsMetricsDTO;
  trend: AdminUsageStatisticsTrendDTO[];
  topModels: AdminUsageStatisticsModelRankDTO[];
  topUsers: AdminUsageStatisticsUserRankDTO[];
};

type AdminUsageStatisticsSubjectOptions =
  | { userID?: never; permissionGroupID?: never }
  | { userID: number; permissionGroupID?: never }
  | { userID?: never; permissionGroupID: number };

export type GetAdminUsageStatisticsOptions = {
  startDate: string;
  endDate: string;
  platformModelName?: string;
  billingScope?: AdminUsageStatisticsBillingScope;
  section?: AdminUsageStatisticsSection;
  modelRankBy?: AdminUsageStatisticsRankBy;
  userRankBy?: AdminUsageStatisticsRankBy;
} & AdminUsageStatisticsSubjectOptions;

export async function getAdminUsageStatistics(
  accessToken: string,
  options: GetAdminUsageStatisticsOptions,
): Promise<AdminUsageStatisticsData> {
  const params = new URLSearchParams({
    start_date: options.startDate,
    end_date: options.endDate,
    billing_scope: options.billingScope ?? "all",
    section: options.section ?? "all",
    model_rank_by: options.modelRankBy ?? "cost",
    user_rank_by: options.userRankBy ?? "cost",
  });
  if (options.userID && options.userID > 0) {
    params.set("user_id", String(options.userID));
  }
  if (options.permissionGroupID && options.permissionGroupID > 0) {
    params.set("permission_group_id", String(options.permissionGroupID));
  }
  if (options.platformModelName?.trim()) {
    params.set("platform_model_name", options.platformModelName.trim());
  }
  return authedRequest<AdminUsageStatisticsData>(
    `/api/v1/admin/usage-statistics?${params.toString()}`,
    { accessToken },
    true,
  );
}
