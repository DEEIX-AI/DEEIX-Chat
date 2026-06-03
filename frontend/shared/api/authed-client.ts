import { clearSessionSnapshot, readAccessToken, readSessionRevision, writeSessionSnapshot } from "@/shared/auth/session";
import { apiRequest, ApiError, resolveApiBaseURL, type ApiRequestOptions } from "@/shared/api/http-client";
import type { ApiEnvelope } from "@/shared/api/common.types";
import type { LoginData } from "@/shared/api/auth.types";

type AuthedRequestOptions = Omit<ApiRequestOptions, "accessToken"> & {
  accessToken: string;
};

type AuthedFetchOptions = Omit<RequestInit, "headers"> & {
  accessToken: string;
  headers?: HeadersInit;
};

type NavigatorWithLocks = Navigator & {
  locks?: {
    request<T>(name: string, callback: () => Promise<T> | T): Promise<T>;
  };
};

const AUTH_REFRESH_LOCK_NAME = "deeix-chat:auth-refresh";

let refreshAccessTokenPromise: Promise<string> | null = null;

async function requestAccessTokenRefresh(): Promise<string> {
  const startedRevision = readSessionRevision();
  try {
    const data = await apiRequest<LoginData>("/api/v1/auth/refresh", {
      method: "POST",
    });
    if (!data.accessToken) {
      if (readSessionRevision() === startedRevision) {
        clearSessionSnapshot({ syncPeers: false });
      }
      return "";
    }

    writeSessionSnapshot({
      accessToken: data.accessToken,
      sessionID: data.sessionID,
    });
    return data.accessToken;
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      if (readSessionRevision() === startedRevision) {
        clearSessionSnapshot({ syncPeers: false });
      }
    }
    return "";
  }
}

async function runAccessTokenRefresh(failedToken: string): Promise<string> {
  const refresh = async () => {
    const currentToken = readAccessToken();
    if (currentToken && currentToken !== failedToken) {
      return currentToken;
    }
    return requestAccessTokenRefresh();
  };

  const locks = typeof navigator === "undefined" ? undefined : (navigator as NavigatorWithLocks).locks;
  if (!locks) {
    return refresh();
  }

  return locks.request(AUTH_REFRESH_LOCK_NAME, refresh);
}

export function refreshAccessToken(failedToken = ""): Promise<string> {
  if (!refreshAccessTokenPromise) {
    refreshAccessTokenPromise = runAccessTokenRefresh(failedToken).finally(() => {
      refreshAccessTokenPromise = null;
    });
  }
  return refreshAccessTokenPromise;
}

async function recoverAccessToken(failedToken: string): Promise<string> {
  const currentToken = readAccessToken();
  if (currentToken && currentToken !== failedToken) {
    return currentToken;
  }
  return refreshAccessToken(failedToken);
}

export async function authedRequest<T>(
  path: string,
  options: AuthedRequestOptions,
  allowRefresh = true,
): Promise<T> {
  try {
    return await apiRequest<T>(path, options);
  } catch (error) {
    const isUnauthorized = error instanceof ApiError && error.status === 401;
    if (!allowRefresh || !isUnauthorized) {
      throw error;
    }

    const refreshedToken = await recoverAccessToken(options.accessToken);
    if (!refreshedToken) {
      throw error;
    }

    try {
      return await apiRequest<T>(path, {
        ...options,
        accessToken: refreshedToken,
      });
    } catch (retryError) {
      if (retryError instanceof ApiError && retryError.status === 401) {
        clearSessionSnapshot({ syncPeers: false });
      }
      throw retryError;
    }
  }
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

async function toApiError(response: Response): Promise<ApiError> {
  const contentType = response.headers.get("content-type") || "";
  const requestId = response.headers.get("x-request-id") || undefined;
  if (contentType.includes("application/json")) {
    try {
      const payload = (await response.json()) as Partial<ApiEnvelope<unknown>>;
      return new ApiError(
        payload?.errorMsg || `request failed: ${response.status}`,
        response.status,
        payload?.details,
        payload?.errorCode,
        payload?.requestId || requestId,
      );
    } catch {
      return new ApiError(`request failed: ${response.status}`, response.status, undefined, undefined, requestId);
    }
  }

  try {
    const text = (await response.text()).trim();
    return new ApiError(text || `request failed: ${response.status}`, response.status, undefined, undefined, requestId);
  } catch {
    return new ApiError(`request failed: ${response.status}`, response.status, undefined, undefined, requestId);
  }
}

export async function authedFetch(
  path: string,
  options: AuthedFetchOptions,
  allowRefresh = true,
): Promise<Response> {
  const endpoint = `${resolveApiBaseURL()}${path}`;
  const response = await fetch(endpoint, buildAuthedFetchInit(options));
  if (response.ok) {
    return response;
  }

  const isUnauthorized = response.status === 401;
  if (!allowRefresh || !isUnauthorized) {
    throw await toApiError(response);
  }

  const refreshedToken = await recoverAccessToken(options.accessToken);
  if (!refreshedToken) {
    throw await toApiError(response);
  }

  const retryResponse = await fetch(
    endpoint,
    buildAuthedFetchInit({
      ...options,
      accessToken: refreshedToken,
    }),
  );
  if (retryResponse.status === 401) {
    clearSessionSnapshot({ syncPeers: false });
  }
  if (!retryResponse.ok) {
    throw await toApiError(retryResponse);
  }
  return retryResponse;
}
