import { FEATURE_DISABLED_ERROR_CODE, resolveApiBaseUrl } from "@deeix/core";
import { CLIENT_PLATFORM_HEADER, resolveClientPlatform } from "@/shared/platform";
import type { ApiEnvelope } from "@/shared/api/common-types";
import { isRecord, readString } from "@/shared/lib/type-guards";

type HttpMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

export type ApiRequestOptions = {
  method?: HttpMethod;
  accessToken?: string;
  body?: unknown;
  headers?: Record<string, string>;
  signal?: AbortSignal;
};

export class ApiError extends Error {
  status: number;
  errorCode?: string;
  details?: unknown;
  requestId?: string;
  retryAfterSeconds?: number;
  rawMessage: string;

  constructor(
    message: string,
    status: number,
    details?: unknown,
    errorCode?: string,
    requestId?: string,
    retryAfterSeconds?: number,
  ) {
    super(normalizeApiErrorMessage(message, status));
    this.name = "ApiError";
    this.status = status;
    this.details = details;
    this.errorCode = errorCode;
    this.requestId = requestId;
    this.retryAfterSeconds = retryAfterSeconds;
    this.rawMessage = message;
  }
}

// parseRetryAfterSeconds parses Retry-After seconds; returns undefined if missing or invalid.
export function parseRetryAfterSeconds(response: Response): number | undefined {
  const raw = response.headers.get("retry-after")?.trim();
  if (!raw) {
    return undefined;
  }
  const seconds = Number.parseInt(raw, 10);
  return Number.isFinite(seconds) && seconds > 0 ? seconds : undefined;
}

export class ApiNetworkError extends Error {
  cause?: unknown;

  constructor(cause?: unknown) {
    super("errors.network.unavailable");
    this.name = "ApiNetworkError";
    this.cause = cause;
  }
}

export function resolveAbortError(error: unknown, signal?: AbortSignal): Error | null {
  if (error instanceof Error && error.name === "AbortError") {
    return error;
  }
  if (!signal?.aborted) {
    return null;
  }
  if (signal.reason instanceof Error) {
    return signal.reason;
  }
  const abortError = new Error("The operation was aborted");
  abortError.name = "AbortError";
  return abortError;
}

function normalizeApiErrorMessage(message: string, status: number): string {
  const normalized = message.trim();
  if (/^errors\.[a-zA-Z0-9_.]+$/.test(normalized)) {
    return normalized;
  }
  if (status === 401) {
    return "errors.auth.unauthorized";
  }
  if (status === 403) {
    return "errors.auth.forbidden";
  }
  return normalized;
}

// Notified when the server answers feature.disabled, so the capabilities cache
// can be refreshed; registered by the capabilities layer to keep this module
// free of React.
let featureDisabledListener: (() => void) | null = null;

export function registerFeatureDisabledListener(listener: () => void): void {
  featureDisabledListener = listener;
}

// Runtime API base URL source, registered by the platform layer. On desktop the
// server address is persisted by the Rust shell and read through shared/platform
// (server-address.ts). Read on every request so a change takes effect immediately;
// http-client itself stays free of platform concerns.
let runtimeApiBaseURLResolver: (() => string) | null = null;

export function registerRuntimeApiBaseURLResolver(resolver: () => string): void {
  runtimeApiBaseURLResolver = resolver;
}

// resolveConfiguredApiBaseURL returns the explicit config (runtime override first, then build-time variable), or empty if unset.
export function resolveConfiguredApiBaseURL(): string {
  return resolveApiBaseUrl({
    runtimeOverride: runtimeApiBaseURLResolver?.(),
    configured: process.env.NEXT_PUBLIC_API_BASE_URL,
  });
}

// resolveApiBaseURL falls back to the current page origin when not explicitly configured (:8080 in local development).
export function resolveApiBaseURL(): string {
  return resolveApiBaseUrl({
    runtimeOverride: runtimeApiBaseURLResolver?.(),
    configured: process.env.NEXT_PUBLIC_API_BASE_URL,
    location: typeof window === "undefined" ? null : window.location,
  });
}

export function pathParam(value: string | number): string {
  return encodeURIComponent(String(value));
}

// Client platform header. Native clients use it so the backend delivers the
// refresh token in the response body instead of a SameSite cookie.
function applyClientPlatformHeader(headers: Record<string, string>): void {
  const platform = resolveClientPlatform();
  if (platform) {
    headers[CLIENT_PLATFORM_HEADER] = platform;
  }
}

function buildRequestInit(options: ApiRequestOptions): RequestInit {
  const headers: Record<string, string> = { ...(options.headers || {}) };
  applyClientPlatformHeader(headers);
  if (options.accessToken) {
    headers.Authorization = `Bearer ${options.accessToken}`;
  }

  let body: BodyInit | undefined;
  if (typeof options.body === "string") {
    body = options.body;
  } else if (typeof FormData !== "undefined" && options.body instanceof FormData) {
    body = options.body;
  } else if (typeof options.body !== "undefined") {
    body = JSON.stringify(options.body);
  }

  if (typeof body === "string" && !headers["Content-Type"]) {
    headers["Content-Type"] = "application/json";
  }

  return {
    method: options.method ?? "GET",
    headers,
    body,
    signal: options.signal,
    credentials: "include",
    cache: "no-store",
  };
}

type UntypedEnvelope = Omit<ApiEnvelope<unknown>, "errorMsg"> & { errorMsg?: string };

// readEnvelope keeps only the envelope fields whose runtime type matches the contract, so a
// malformed body degrades to the status-based fallback message instead of leaking odd values.
function readEnvelope(payload: unknown): UntypedEnvelope {
  if (!isRecord(payload)) {
    return { data: undefined };
  }
  return {
    errorMsg: readString(payload, "errorMsg"),
    errorCode: readString(payload, "errorCode"),
    requestId: readString(payload, "requestId"),
    details: payload.details,
    data: payload.data,
  };
}

// toApiError parses the unified error envelope from a failed response into an ApiError carrying the error code and request ID.
export async function toApiError(response: Response): Promise<ApiError> {
  const contentType = response.headers.get("content-type") || "";
  const requestId = response.headers.get("x-request-id") || undefined;
  if (contentType.includes("application/json")) {
    try {
      const payload = readEnvelope(await response.json());
      return new ApiError(
        payload.errorMsg || `request failed: ${response.status}`,
        response.status,
        payload.details,
        payload.errorCode,
        payload.requestId || requestId,
        parseRetryAfterSeconds(response),
      );
    } catch {
      return new ApiError(
        `request failed: ${response.status}`,
        response.status,
        undefined,
        undefined,
        requestId,
        parseRetryAfterSeconds(response),
      );
    }
  }

  try {
    const text = (await response.text()).trim();
    return new ApiError(
      text || `request failed: ${response.status}`,
      response.status,
      undefined,
      undefined,
      requestId,
      parseRetryAfterSeconds(response),
    );
  } catch {
    return new ApiError(
      `request failed: ${response.status}`,
      response.status,
      undefined,
      undefined,
      requestId,
      parseRetryAfterSeconds(response),
    );
  }
}

// apiFetch issues an unauthenticated request and returns the raw Response; failed responses throw ApiError per the unified error envelope.
export async function apiFetch(path: string, options: ApiRequestOptions = {}): Promise<Response> {
  const endpoint = `${resolveApiBaseURL()}${path}`;
  let response: Response;
  try {
    response = await fetch(endpoint, buildRequestInit(options));
  } catch (error) {
    const abortError = resolveAbortError(error, options.signal);
    if (abortError) {
      throw abortError;
    }
    throw new ApiNetworkError(error);
  }
  if (!response.ok) {
    throw await toApiError(response);
  }
  return response;
}

export async function apiRequest<T>(path: string, options: ApiRequestOptions = {}): Promise<T> {
  const endpoint = `${resolveApiBaseURL()}${path}`;
  let response: Response;
  try {
    response = await fetch(endpoint, buildRequestInit(options));
  } catch (error) {
    const abortError = resolveAbortError(error, options.signal);
    if (abortError) {
      throw abortError;
    }
    throw new ApiNetworkError(error);
  }
  const contentType = response.headers.get("content-type") || "";
  const responseRequestId = response.headers.get("x-request-id") || undefined;
  const isJSON = contentType.includes("application/json");
  const payload: UntypedEnvelope = isJSON
    ? readEnvelope(await response.json())
    : { errorMsg: response.ok ? "" : await response.text(), requestId: responseRequestId, data: undefined };

  if (!response.ok) {
    if (payload.errorCode === FEATURE_DISABLED_ERROR_CODE) {
      featureDisabledListener?.();
    }
    throw new ApiError(
      payload.errorMsg?.trim() || `request failed: ${response.status}`,
      response.status,
      payload.details,
      payload.errorCode,
      payload.requestId || responseRequestId,
      parseRetryAfterSeconds(response),
    );
  }
  if (payload.errorMsg) {
    throw new ApiError(
      payload.errorMsg,
      response.status,
      payload.details,
      payload.errorCode,
      payload.requestId || responseRequestId,
      parseRetryAfterSeconds(response),
    );
  }
  // Every endpoint answers with the JSON envelope. A 2xx without one did not come from the API
  // (typically a static host or SPA fallback serving index.html for the path), so resolving it
  // would hand callers `undefined` as `T`.
  if (!isJSON) {
    throw new ApiError(
      `request failed: unexpected non-JSON response (${response.status})`,
      response.status,
      undefined,
      undefined,
      responseRequestId,
    );
  }
  // Type assertion: the envelope shape is validated above, but `data` is typed by the endpoint's
  // server contract (@deeix/api-contract) that callers pick through `T`; it is not re-validated here.
  return payload.data as T;
}
