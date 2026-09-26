import type { User } from "@deeix/api-contract";
import { authedRequest } from "@/shared/api/authed-client";

export type UserSettingsMap = User.SettingsPartialUpdate.RequestBody["settings"];

type GetUserSettingsResponse = NonNullable<User.SettingsList.ResponseBody["data"]>;
type PatchUserSettingsResponse = NonNullable<User.SettingsPartialUpdate.ResponseBody["data"]>;

export async function getUserSettings(accessToken: string): Promise<UserSettingsMap> {
  const data = await authedRequest<GetUserSettingsResponse>("/api/v1/user/settings", { accessToken }, true);
  return data.settings ?? {};
}

export async function patchUserSettings(
  accessToken: string,
  settings: UserSettingsMap,
): Promise<UserSettingsMap> {
  const body: User.SettingsPartialUpdate.RequestBody = { settings };
  const data = await authedRequest<PatchUserSettingsResponse>(
    "/api/v1/user/settings",
    {
      accessToken,
      method: "PATCH",
      body: JSON.stringify(body),
    },
    true,
  );
  return data.settings ?? {};
}
