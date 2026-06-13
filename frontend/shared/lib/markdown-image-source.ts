import { resolveApiBaseURL } from "@/shared/api/http-client";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

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

export async function downloadMarkdownImageSource(src: string, fileName: string): Promise<void> {
  const protectedSrc = resolveProtectedMarkdownImageSource(src);
  const accessToken = protectedSrc ? await resolveAccessToken() : null;
  const response = await fetch(resolveMarkdownImageSource(src), {
    headers: protectedSrc && accessToken ? { Authorization: `Bearer ${accessToken}` } : undefined,
  });
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
  const accessToken = await resolveAccessToken();
  if (!accessToken) {
    throw new Error("Missing access token");
  }
  const response = await fetch(src, {
    cache: "no-store",
    headers: { Authorization: `Bearer ${accessToken}` },
    signal,
  });
  if (!response.ok) {
    throw new Error("Failed to load image");
  }
  return URL.createObjectURL(await response.blob());
}
