import { resolveApiBaseURL } from "@/shared/api/http-client";
import { isRecord, readString, type UnknownRecord } from "@/shared/lib/type-guards";

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
  const payload: unknown = await response.json();
  // The endpoint has answered both bare and enveloped over time; accept either.
  if (isRecord(payload) && isRecord(payload.data)) {
    return readAppVersion(payload.data);
  }
  return isRecord(payload) ? readAppVersion(payload) : {};
}

function readAppVersion(record: UnknownRecord): AppVersionDTO {
  return {
    product: readString(record, "product"),
    version: readString(record, "version"),
    commit: readString(record, "commit"),
    buildTime: readString(record, "buildTime"),
    buildID: readString(record, "buildID"),
  };
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
