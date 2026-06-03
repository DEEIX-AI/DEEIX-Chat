import { resolveApiBaseURL } from "@/shared/api/http-client";

export type AppVersionDTO = {
  product?: string;
  version?: string;
  commit?: string;
  buildTime?: string;
  buildID?: string;
};

export async function getAppVersion(): Promise<AppVersionDTO> {
  const response = await fetch(`${resolveApiBaseURL()}/api/v1/version`, {
    cache: "no-store",
    credentials: "include",
  });
  if (!response.ok) {
    throw new Error(`version request failed: ${response.status}`);
  }
  const payload = (await response.json()) as AppVersionDTO | { data?: AppVersionDTO };
  if ("data" in payload && payload.data) {
    return payload.data;
  }
  return payload as AppVersionDTO;
}

export function resolveAppBuildID(version: AppVersionDTO): string {
  const explicitBuildID = version.buildID?.trim();
  if (explicitBuildID) {
    return explicitBuildID;
  }
  return [version.version, version.commit, version.buildTime]
    .map((item) => item?.trim() ?? "")
    .filter((item) => item && item !== "unknown" && item !== "dev")
    .join("-");
}
