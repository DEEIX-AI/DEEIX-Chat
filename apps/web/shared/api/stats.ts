import { authedRequest } from "@/shared/api/authed-client";
import type { UserActivityDailyDTO } from "@/shared/api/stats.types";

export async function getUserActivity(
  accessToken: string,
  options: { days?: number; signal?: AbortSignal } = {},
): Promise<UserActivityDailyDTO[]> {
  const params = new URLSearchParams();
  if (options.days && options.days > 0) {
    params.set("days", String(options.days));
  }
  const query = params.toString();
  return authedRequest<UserActivityDailyDTO[]>(
    `/api/v1/user/stats/activity${query ? `?${query}` : ""}`,
    { accessToken, signal: options.signal },
    true,
  );
}
