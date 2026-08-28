import type { LoginData } from "@/shared/api/auth.types";
import {
  ApiError,
  ApiNetworkError,
  type ApiRequestOptions,
  apiRequest,
  resolveAbortError,
  resolveApiBaseURL,
  toApiError,
} from "@/shared/api/http-client";
import { clearSessionSnapshot, readAccessToken, readSessionRevision, writeSessionSnapshot } from "@/shared/auth/session";

type AuthedRequestOptions = Omit<ApiRequestOptions, "accessToken"> & {
  accessToken: string;
};

type AuthedFetchOptions = Omit<RequestInit, "headers" | "signal"> & {
  accessToken: string;
  headers?: HeadersInit;
  signal?: AbortSignal;
};

type NavigatorWithLocks = Navigator & {
  locks?: {
    request<T>(name: string, callback: () => Promise<T> | T): Promise<T>;
  };
};

const AUTH_REFRESH_LOCK_NAME = "deeix-chat:auth-refresh";
const SESSION_TERMINATING_ERROR_CODES = new Set([
  "auth.invalid_token",
  "auth.invalid_refresh_token",
  "auth.session_invalid",
]);

let refreshAccessTokenPromise: Promise<string> | null = null;

function isSessionTerminatingAuthError(error: unknown): boolean {
  return error instanceof ApiError &&
    error.status === 401 &&
    typeof error.errorCode === "string" &&
    SESSION_TERMINATING_ERROR_CODES.has(error.errorCode);
}

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
    if (isSessionTerminatingAuthError(error)) {
      if (readSessionRevision() === startedRevision) {
        clearSessionSnapshot({ syncPeers: false });
      }
      return "";
    }
    throw error;
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
    const abortError = resolveAbortError(error, options.signal);
    if (abortError) {
      throw abortError;
    }
    const isUnauthorized = error instanceof ApiError && error.status === 401;
    if (!allowRefresh || !isUnauthorized) {
      throw error;
    }

    const refreshedToken = await recoverAccessToken(options.accessToken);
    const refreshAbortError = resolveAbortError(undefined, options.signal);
    if (refreshAbortError) {
      throw refreshAbortError;
    }
    if (!refreshedToken) {
      throw error;
    }

    try {
      return await apiRequest<T>(path, {
        ...options,
        accessToken: refreshedToken,
      });
    } catch (retryError) {
      if (isSessionTerminatingAuthError(retryError)) {
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

export async function authedFetch(
  path: string,
  options: AuthedFetchOptions,
  allowRefresh = true,
): Promise<Response> {
  const endpoint = `${resolveApiBaseURL()}${path}`;
  let response: Response;
  try {
    response = await fetch(endpoint, buildAuthedFetchInit(options));
  } catch (error) {
    const abortError = resolveAbortError(error, options.signal);
    if (abortError) {
      throw abortError;
    }
    throw new ApiNetworkError(error);
  }
  if (response.ok) {
    return response;
  }

  const isUnauthorized = response.status === 401;
  if (!allowRefresh || !isUnauthorized) {
    throw await toApiError(response);
  }

  const responseAbortError = resolveAbortError(undefined, options.signal);
  if (responseAbortError) {
    throw responseAbortError;
  }

  const refreshedToken = await recoverAccessToken(options.accessToken);
  const refreshAbortError = resolveAbortError(undefined, options.signal);
  if (refreshAbortError) {
    throw refreshAbortError;
  }
  if (!refreshedToken) {
    throw await toApiError(response);
  }

  let retryResponse: Response;
  try {
    retryResponse = await fetch(
      endpoint,
      buildAuthedFetchInit({
        ...options,
        accessToken: refreshedToken,
      }),
    );
  } catch (error) {
    const abortError = resolveAbortError(error, options.signal);
    if (abortError) {
      throw abortError;
    }
    throw new ApiNetworkError(error);
  }
  if (!retryResponse.ok) {
    const retryError = await toApiError(retryResponse);
    if (isSessionTerminatingAuthError(retryError)) {
      clearSessionSnapshot({ syncPeers: false });
    }
    throw retryError;
  }
  return retryResponse;
}
