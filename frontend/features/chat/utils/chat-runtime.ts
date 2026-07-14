import type { UpstreamDebugInfo } from "@/shared/api/conversation.types";
import { resolveLocalizedErrorMessage } from "@/i18n/resolve-error-message";

const DEFAULT_MAX_FILES_PER_MESSAGE = 10;
const ZH_DETAIL_PREFIX = "\u8be6\u60c5\uff1a";
const ZH_ERROR_PREFIX = "\u9519\u8bef\uff1a";

export function resolveMaxFilesPerMessage(): number {
  return DEFAULT_MAX_FILES_PER_MESSAGE;
}

export function resolveHourByTimeZone(timeZone: string): number {
  try {
    const hourText = new Intl.DateTimeFormat("en-US", {
      hour: "2-digit",
      hour12: false,
      hourCycle: "h23",
      timeZone,
    }).format(new Date());
    const hour = Number.parseInt(hourText, 10);
    if (!Number.isNaN(hour)) {
      return hour;
    }
  } catch {
    // Fallback to UTC hour.
  }
  return new Date().getUTCHours();
}

export type GreetingPeriod = "morning" | "afternoon" | "evening" | "default";

export function resolveGreetingPeriodByHour(hour: number): GreetingPeriod {
  if (hour >= 6 && hour < 12) {
    return "morning";
  }
  if (hour >= 12 && hour < 18) {
    return "afternoon";
  }
  if (hour >= 18 && hour < 24) {
    return "evening";
  }
  return "default";
}

export function resolveErrorMessage(error: unknown, fallback: string): string {
  return resolveLocalizedErrorMessage(error, fallback);
}

export function resolveErrorSummary(error: unknown, fallback: string): string {
  const message = resolveErrorMessage(error, fallback);
  const details = resolveErrorDetails(error);
  const { statusCode, reason } = summarizeUpstreamError(message, details, fallback);
  return [statusCode ? `HTTP ${statusCode}` : "", reason].filter(Boolean).join(": ");
}

export function summarizeUpstreamError(
  message: string,
  details?: UpstreamDebugInfo,
  fallback = "",
): { statusCode: number | null; reason: string } {
  const statusCode = details?.response?.statusCode || extractStatusCode(message);
  const reason =
    extractStructuredErrorReason(details?.response?.body || "") ||
    extractStructuredErrorReason(extractDetailBody(message)) ||
    extractMessageReason(message) ||
    fallback;
  return { statusCode, reason };
}

export function isUpstreamStreamingDebugBody(value: string | null | undefined): boolean {
  const raw = value?.trim() || "";
  return raw.startsWith("data:") || /(^|\n)\s*data:\s*/.test(raw) || /^HTTP\s*2\d\d\s*,?\s*data:/i.test(raw);
}

export function resolveErrorDetails(error: unknown): UpstreamDebugInfo | undefined {
  if (!error || typeof error !== "object" || !("details" in error)) {
    return undefined;
  }
  const details = (error as { details?: unknown }).details;
  return isUpstreamDebugInfo(details) ? details : undefined;
}

function extractStatusCode(message: string): number | null {
  const match = message.match(/HTTP\s*(\d{3})/i);
  if (!match) {
    return null;
  }
  const statusCode = Number.parseInt(match[1], 10);
  return Number.isFinite(statusCode) ? statusCode : null;
}

function extractDetailBody(message: string): string {
  const detailLine = message.match(/(?:^|\n)Details:\s*([\s\S]*)$/i)?.[1]?.trim();
  if (detailLine) {
    return detailLine;
  }
  const index = message.indexOf(ZH_DETAIL_PREFIX);
  return index >= 0 ? message.slice(index + ZH_DETAIL_PREFIX.length).trim() : "";
}

function extractMessageReason(message: string): string {
  const lines = message.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
  const errorLine = lines.find((line) => line.startsWith("Error:") || line.startsWith(ZH_ERROR_PREFIX));
  if (errorLine) {
    return normalizeInlineErrorReason(errorLine.replace(/^Error:\s*/i, "").replace(ZH_ERROR_PREFIX, "").trim());
  }
  return normalizeInlineErrorReason(
    lines.find((line) => !line.startsWith("Details:") && !line.startsWith(ZH_DETAIL_PREFIX)) || "",
  );
}

function normalizeInlineErrorReason(value: string): string {
  const raw = value.trim();
  if (!raw || isUpstreamStreamingDebugBody(raw) || /^HTTP\s*2\d\d\s*,?\s*data:/i.test(raw)) {
    return "";
  }
  return raw;
}

function extractStructuredErrorReason(body: string): string {
  const raw = body.trim();
  if (!raw) {
    return "";
  }
  if (isUpstreamStreamingDebugBody(raw)) {
    return extractSSEErrorReason(raw);
  }
  try {
    const parsed = JSON.parse(raw) as unknown;
    return extractJSONErrorReason(parsed);
  } catch {
    return "";
  }
}

function extractSSEErrorReason(body: string): string {
  const payloads = body
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line.startsWith("data:"))
    .map((line) => line.slice("data:".length).trim())
    .filter((line) => line && line !== "[DONE]");
  for (const payload of payloads) {
    try {
      const reason = extractJSONErrorReason(JSON.parse(payload) as unknown);
      if (reason) {
        return reason;
      }
    } catch {
      // Ignore malformed stream chunks; the full body remains available in the response tab.
    }
  }
  return "";
}

function extractJSONErrorReason(value: unknown): string {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return "";
  }
  const root = value as Record<string, unknown>;
  const error =
    root.error && typeof root.error === "object" && !Array.isArray(root.error)
      ? (root.error as Record<string, unknown>)
      : undefined;
  for (const candidate of [error?.message, root.message, error?.code, root.code]) {
    if (typeof candidate === "string" && candidate.trim()) {
      return candidate.trim();
    }
  }
  const response =
    root.response && typeof root.response === "object" && !Array.isArray(root.response)
      ? (root.response as Record<string, unknown>)
      : undefined;
  return response ? extractJSONErrorReason(response) : "";
}

function isUpstreamDebugInfo(value: unknown): value is UpstreamDebugInfo {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return false;
  }
  const candidate = value as UpstreamDebugInfo;
  return typeof candidate.request === "object" || typeof candidate.response === "object";
}
