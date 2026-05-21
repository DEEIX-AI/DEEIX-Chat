import { authedRequest } from "@/shared/api/authed-client";

export type UserSettingsMap = Record<string, string>;

type UserSettingsResponse = {
  settings: UserSettingsMap;
};

export async function getUserSettings(accessToken: string): Promise<UserSettingsMap> {
  const data = await authedRequest<UserSettingsResponse>("/api/v1/user/settings", { accessToken }, true);
  return data.settings ?? {};
}

export async function patchUserSettings(
  accessToken: string,
  settings: UserSettingsMap,
): Promise<UserSettingsMap> {
  const data = await authedRequest<UserSettingsResponse>(
    "/api/v1/user/settings",
    {
      accessToken,
      method: "PATCH",
      body: JSON.stringify({ settings }),
    },
    true,
  );
  return data.settings ?? {};
}
