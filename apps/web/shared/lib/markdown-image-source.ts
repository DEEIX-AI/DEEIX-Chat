import { authedFetch } from "@/shared/api/authed-client";
import { resolveApiBaseURL } from "@/shared/api/http-client";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { withHeaderTimeout } from "@/shared/lib/fetch-timeout";

// Restore protected absolute image URLs to API-relative paths so authedFetch uniformly attaches credentials and handles 401 refresh.
function resolveProtectedMarkdownImagePath(src: string): string | null {
  const protectedSrc = resolveProtectedMarkdownImageSource(src);
  if (!protectedSrc) {
    return null;
  }
  const url = new URL(protectedSrc, window.location.origin);
  return `${url.pathname}${url.search}`;
}

async function fetchProtectedMarkdownImage(path: string, signal?: AbortSignal): Promise<Response> {
  const accessToken = await resolveAccessToken();
  if (!accessToken) {
    throw new Error("Missing access token");
  }
  return authedFetch(path, { accessToken, cache: "no-store", signal });
}

export function resolveMarkdownImageSource(src: string): string {
  if (typeof window === "undefined") {
    return src;
  }
  try {
    const targetURL = new URL(src, window.location.origin);
    if (targetURL.pathname.startsWith("/api/v1/")) {
      return `${resolveApiBaseURL()}${targetURL.pathname}${targetURL.search}${targetURL.hash}`;
    }
  } catch {
    if (src.startsWith("/api/v1/")) {
      return `${resolveApiBaseURL()}${src}`;
    }
  }
  return src;
}

export function resolveProtectedMarkdownImageSource(src: string): string | null {
  if (typeof window === "undefined") {
    return null;
  }
  try {
    const targetURL = new URL(resolveMarkdownImageSource(src), window.location.origin);
    const apiURL = new URL(resolveApiBaseURL() || window.location.origin, window.location.origin);
    if (targetURL.origin !== apiURL.origin) {
      return null;
    }
    if (/^\/api\/v1\/files\/[^/]+\/content$/.test(targetURL.pathname)) {
      return targetURL.toString();
    }
  } catch {
    return null;
  }
  return null;
}

export function resolveMarkdownImageDownloadName(src: string, alt: string | undefined): string {
  const url = new URL(src, window.location.origin);
  const pathname = url.pathname.split("/").filter(Boolean).at(-1) || "";
  if (pathname.includes(".") && pathname.split(".").at(-1)?.length) {
    return pathname;
  }
  const baseName = (alt?.trim() || "image").replace(/[\\/:*?"<>|]+/g, "-");
  return `${baseName}.png`;
}

// Upper bound for the image host to start responding, so a stalled host cannot hang the action.
// Only the headers are bounded; a large image on a slow link can still finish downloading.
const IMAGE_DOWNLOAD_HEADER_TIMEOUT_MS = 30_000;

export async function downloadMarkdownImageSource(src: string, fileName: string): Promise<void> {
  const protectedPath = resolveProtectedMarkdownImagePath(src);
  const response = await withHeaderTimeout(
    (signal) =>
      protectedPath ? fetchProtectedMarkdownImage(protectedPath, signal) : fetch(resolveMarkdownImageSource(src), { signal }),
    { timeoutMs: IMAGE_DOWNLOAD_HEADER_TIMEOUT_MS },
  );
  if (!response.ok) {
    throw new Error("Failed to download image");
  }
  const blob = await response.blob();
  const blobURL = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = blobURL;
  link.download = fileName;
  try {
    document.body.appendChild(link);
    link.click();
  } finally {
    link.remove();
    URL.revokeObjectURL(blobURL);
  }
}

export async function loadProtectedMarkdownImageBlobURL(src: string, signal: AbortSignal): Promise<string> {
  const protectedPath = resolveProtectedMarkdownImagePath(src);
  if (!protectedPath) {
    throw new Error("Not a protected image source");
  }
  const response = await fetchProtectedMarkdownImage(protectedPath, signal);
  return URL.createObjectURL(await response.blob());
}
