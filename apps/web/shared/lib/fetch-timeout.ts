// Response-header timeouts that work on every supported runtime.
//
// The macOS desktop shell supports WKWebView down to macOS 10.15 (Safari <= 15.6), which lacks
// AbortSignal.timeout and AbortSignal.any. This helper builds the same behavior from
// AbortController + setTimeout and a manual abort listener instead.

import { isRecord } from "@/shared/lib/type-guards";

const TIMEOUT_ERROR_NAME = "TimeoutError";

// Thrown when response headers did not arrive in time. The name matches the DOMException that
// AbortSignal.timeout would produce, so `error.name === "TimeoutError"` checks work either way.
export class HeaderTimeoutError extends Error {
  readonly timeoutMs: number;

  constructor(timeoutMs: number) {
    super(`Response headers did not arrive within ${timeoutMs}ms`);
    this.name = TIMEOUT_ERROR_NAME;
    this.timeoutMs = timeoutMs;
  }
}

export function isTimeoutError(error: unknown): boolean {
  return isRecord(error) && error.name === TIMEOUT_ERROR_NAME;
}

export type HeaderTimeoutOptions = {
  // Maximum wait for the request to resolve (fetch resolves once headers arrive).
  timeoutMs: number;
  // Optional caller signal; aborting it cancels the request and any later body read.
  signal?: AbortSignal | null;
};

function abortReason(signal: AbortSignal): unknown {
  // Safari < 15.4 has no AbortSignal.reason; fall back to the standard AbortError.
  return signal.reason ?? new DOMException("The operation was aborted", "AbortError");
}

// Runs `request` with a signal that aborts if it has not resolved within `timeoutMs`.
// Once it resolves the timer is cleared, so the response body can take as long as it needs;
// the caller signal stays linked and can still cancel the body download.
// Rejects with HeaderTimeoutError on timeout, or with the caller's abort reason on caller abort.
export async function withHeaderTimeout<T>(
  request: (signal: AbortSignal) => Promise<T>,
  { timeoutMs, signal: externalSignal }: HeaderTimeoutOptions,
): Promise<T> {
  if (externalSignal?.aborted) {
    throw abortReason(externalSignal);
  }
  const controller = new AbortController();
  let timedOut = false;
  const onExternalAbort = () => {
    if (externalSignal) {
      controller.abort(abortReason(externalSignal));
    }
  };
  externalSignal?.addEventListener("abort", onExternalAbort, { once: true });
  const timer = setTimeout(() => {
    timedOut = true;
    controller.abort(new HeaderTimeoutError(timeoutMs));
  }, timeoutMs);
  try {
    return await request(controller.signal);
  } catch (error) {
    // The request never produced a body to cancel, so the caller link is no longer needed.
    externalSignal?.removeEventListener("abort", onExternalAbort);
    // Older WebKit rejects with a generic AbortError instead of the abort reason.
    if (timedOut && !externalSignal?.aborted) {
      throw new HeaderTimeoutError(timeoutMs);
    }
    throw error;
  } finally {
    clearTimeout(timer);
  }
}

export type FetchWithHeaderTimeoutInit = Omit<RequestInit, "signal"> & HeaderTimeoutOptions;

// fetch() with a response-header timeout; see withHeaderTimeout.
export function fetchWithHeaderTimeout(input: RequestInfo | URL, init: FetchWithHeaderTimeoutInit): Promise<Response> {
  const { timeoutMs, signal, ...requestInit } = init;
  return withHeaderTimeout((timeoutSignal) => fetch(input, { ...requestInit, signal: timeoutSignal }), {
    timeoutMs,
    signal,
  });
}
