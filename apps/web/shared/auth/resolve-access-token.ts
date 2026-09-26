import { refreshAccessToken } from "@/shared/api/authed-client";
import { readAccessToken } from "@/shared/auth/session";

export async function resolveAccessToken(): Promise<string> {
  const cached = readAccessToken();
  if (cached) {
    return cached;
  }

  const refreshed = await refreshAccessToken();
  if (refreshed) {
    return refreshed;
  }

  return "";
}
