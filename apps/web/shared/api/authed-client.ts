import {
  ApiNetworkError,
  type ApiRequestOptions,
  apiRequest,
  resolveAbortError,
  resolveApiBaseURL,
  toApiError,
} from "@/shared/api/http-client";
import { authClient } from "@/shared/auth/auth-client";

// Authenticated request helpers. The 401 → refresh → retry policy lives in
// @deeix/core (`authClient.withAuthRetry`); this file only adapts fetch/apiRequest
// to it. Do not add refresh or logout decisions here.

type AuthedRequestOptions = Omit<ApiRequestOptions, "accessToken"> & {
  accessToken: string;
};

type AuthedFetchOptions = Omit<RequestInit, "headers" | "signal"> & {
  accessToken: string;
  headers?: HeadersInit;
  signal?: AbortSignal;
};

/** Refresh the access token (deduplicated). Resolves "" when the session is gone. */
export function refreshAccessToken(failedToken = ""): Promise<string> {
  return authClient.refreshAccessToken(failedToken);
}

export async function authedRequest<T>(
  path: string,
  options: AuthedRequestOptions,
  allowRefresh = true,
): Promise<T> {
  return authClient.withAuthRetry<T>({
    accessToken: options.accessToken,
    allowRefresh,
    resolveAbort: (error) => resolveAbortError(error, options.signal),
    execute: (accessToken) => apiRequest<T>(path, { ...options, accessToken }),
  });
}

function buildAuthedFetchInit(options: AuthedFetchOptions): RequestInit {
  const headers = new Headers(options.headers ?? {});
  if (options.accessToken) {
    headers.set("Authorization", `Bearer ${options.accessToken}`);
  }

  return {
    ...options,
    headers,
    credentials: "include",
  };
}

export async function authedFetch(
  path: string,
  options: AuthedFetchOptions,
  allowRefresh = true,
): Promise<Response> {
  const endpoint = `${resolveApiBaseURL()}${path}`;

  const execute = async (accessToken: string): Promise<Response> => {
    let response: Response;
    try {
      response = await fetch(endpoint, buildAuthedFetchInit({ ...options, accessToken }));
    } catch (error) {
      const abortError = resolveAbortError(error, options.signal);
      if (abortError) {
        throw abortError;
      }
      throw new ApiNetworkError(error);
    }
    if (!response.ok) {
      // Surface every failure as ApiError so the shared policy can classify it.
      throw await toApiError(response);
    }
    return response;
  };

  return authClient.withAuthRetry<Response>({
    accessToken: options.accessToken,
    allowRefresh,
    resolveAbort: (error) => resolveAbortError(error, options.signal),
    execute,
  });
}
