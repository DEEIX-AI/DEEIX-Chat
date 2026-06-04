import type { ApiEnvelope } from "@/shared/api/common.types";

type HttpMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

export type ApiRequestOptions = {
  method?: HttpMethod;
  accessToken?: string;
  body?: unknown;
  headers?: Record<string, string>;
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

export function resolveApiBaseURL(): string {
  const configured = process.env.NEXT_PUBLIC_API_BASE_URL?.trim();
  if (configured) {
    return configured.replace(/\/+$/, "");
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
    credentials: "include",
    cache: "no-store",
  };
}

export async function apiRequest<T>(path: string, options: ApiRequestOptions = {}): Promise<T> {
  const endpoint = `${resolveApiBaseURL()}${path}`;
  const response = await fetch(endpoint, buildRequestInit(options));
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
