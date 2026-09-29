import type { CapabilitiesResponse } from "@deeix/api-contract";
import { type CapabilityFlags, FEATURE_DISABLED_ERROR_CODE, resolveCapabilities } from "@deeix/core";
import { ApiError, apiRequest } from "@/shared/api/http-client";

/**
 * Fetch the server's capability flags. Never rejects: a server that predates the
 * endpoint, or a network failure, yields every feature enabled — the client must
 * not hide a feature the server actually has (docs/ARCHITECTURE.md §4).
 */
export async function getCapabilities(): Promise<CapabilityFlags> {
  try {
    const response = await apiRequest<CapabilitiesResponse>("/api/v1/capabilities");
    return resolveCapabilities(response);
  } catch {
    return resolveCapabilities(undefined);
  }
}

/**
 * Substitute `fallback` when the server answers feature.disabled. Only for code
 * that cannot read capabilities up front (module-level caches outside React);
 * components and hooks should consult `useCapabilities` and skip the request.
 */
export async function whenFeatureAvailable<T>(request: Promise<T>, fallback: T): Promise<T> {
  try {
    return await request;
  } catch (error) {
    if (error instanceof ApiError && error.errorCode === FEATURE_DISABLED_ERROR_CODE) {
      return fallback;
    }
    throw error;
  }
}
