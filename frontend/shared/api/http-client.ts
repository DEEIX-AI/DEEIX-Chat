import type { ApiEnvelope } from "@/shared/api/common.types";

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
  rawMessage: string;

  constructor(message: string, status: number, details?: unknown, errorCode?: string, requestId?: string) {
    super(normalizeApiErrorMessage(message, status));
    this.name = "ApiError";
    this.status = status;
    this.details = details;
    this.errorCode = errorCode;
    this.requestId = requestId;
    this.rawMessage = message;
  }
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

export function resolveConfiguredApiBaseURL(): string {
  const configured = process.env.NEXT_PUBLIC_API_BASE_URL?.trim();
  return configured ? configured.replace(/\/+$/, "") : "";
}

export function resolveApiBaseURL(): string {
  const configured = resolveConfiguredApiBaseURL();
  if (configured) {
    return configured;
  }

  if (typeof window === "undefined") {
    return "";
  }

  const { hostname, port, origin } = window.location;
  if ((hostname === "localhost" || hostname === "127.0.0.1" || hostname === "::1") && port !== "8080") {
    const host = hostname === "::1" ? "[::1]" : hostname;
    return `http://${host}:8080`;
  }

  return origin.replace(/\/+$/, "");
}

export function pathParam(value: string | number): string {
  return encodeURIComponent(String(value));
}

function buildRequestInit(options: ApiRequestOptions): RequestInit {
  const headers: Record<string, string> = { ...(options.headers || {}) };
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

// toApiError 从失败响应中解析统一错误信封，生成携带错误码与请求 ID 的 ApiError。
export async function toApiError(response: Response): Promise<ApiError> {
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

// apiFetch 发起无鉴权请求并返回原始 Response；失败响应按统一错误信封抛出 ApiError。
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
  const payload = contentType.includes("application/json")
    ? ((await response.json()) as ApiEnvelope<T>)
    : ({ errorMsg: response.ok ? "" : await response.text(), requestId: responseRequestId } as ApiEnvelope<T>);

  if (!response.ok) {
    throw new ApiError(
      payload.errorMsg?.trim() || `request failed: ${response.status}`,
      response.status,
      payload.details,
      payload.errorCode,
      payload.requestId || responseRequestId,
    );
  }
  if (payload.errorMsg) {
    throw new ApiError(payload.errorMsg, response.status, payload.details, payload.errorCode, payload.requestId || responseRequestId);
  }
  return payload.data;
}
