"use client";

import { Bot } from "lucide-react";
import { useEffect, useState, useSyncExternalStore } from "react";
import { cn } from "@/lib/utils";
import { resolveApiBaseURL } from "@/shared/api/http-client";
import { fetchWithHeaderTimeout } from "@/shared/lib/fetch-timeout";

const LOBEHUB_ICON_PREFIX = "/vendor/lobehub-icons/";
const LOBEHUB_ICON_SPRITE = `${LOBEHUB_ICON_PREFIX}__sprite.svg`;
const LOBEHUB_ICON_SPRITE_CONTAINER_ID = "lobehub-icon-sprite";
const MODEL_ICON_API_PREFIX = "/api/v1/llm/icon-assets/";
// Same-origin static asset; only the response headers are bounded so a slow body still completes.
const SPRITE_HEADER_TIMEOUT_MS = 15_000;
// After a failure, later mounts retry only once the backoff window has passed, and give up
// after a few attempts; icons keep rendering the per-icon SVG files in the meantime.
const SPRITE_RETRY_BASE_DELAY_MS = 30_000;
const SPRITE_MAX_ATTEMPTS = 3;

let spriteReady = false;
let spriteRequest: Promise<void> | null = null;
let spriteFailureCount = 0;
let spriteLastFailureAt = 0;
const spriteListeners = new Set<() => void>();

function subscribeSprite(listener: () => void): () => void {
  spriteListeners.add(listener);
  return () => {
    spriteListeners.delete(listener);
  };
}

function getSpriteReadySnapshot(): boolean {
  return spriteReady;
}

function getSpriteReadyServerSnapshot(): boolean {
  return false;
}

function markSpriteReady() {
  spriteReady = true;
  for (const listener of spriteListeners) {
    listener();
  }
}

function parseLobeHubIconID(iconUrl: string): string | null {
  if (!iconUrl.startsWith(LOBEHUB_ICON_PREFIX) || !iconUrl.endsWith(".svg")) {
    return null;
  }
  const iconID = iconUrl.slice(LOBEHUB_ICON_PREFIX.length, -4);
  return iconID && iconID !== "__sprite" ? iconID : null;
}

function spriteRetryAllowed(now: number): boolean {
  if (spriteFailureCount === 0) {
    return true;
  }
  if (spriteFailureCount >= SPRITE_MAX_ATTEMPTS) {
    return false;
  }
  return now - spriteLastFailureAt >= SPRITE_RETRY_BASE_DELAY_MS * 2 ** (spriteFailureCount - 1);
}

async function loadLobeHubSprite(): Promise<void> {
  const response = await fetchWithHeaderTimeout(LOBEHUB_ICON_SPRITE, {
    cache: "force-cache",
    timeoutMs: SPRITE_HEADER_TIMEOUT_MS,
  });
  if (!response.ok) {
    throw new Error(`Failed to load LobeHub icon sprite: ${response.status}`);
  }
  const sprite = await response.text();
  if (!document.getElementById(LOBEHUB_ICON_SPRITE_CONTAINER_ID)) {
    const container = document.createElement("div");
    container.id = LOBEHUB_ICON_SPRITE_CONTAINER_ID;
    container.hidden = true;
    container.setAttribute("aria-hidden", "true");
    container.innerHTML = sprite;
    document.body.prepend(container);
  }
}

// Starts (or joins) the shared sprite request. Never rejects; subscribers are notified on success.
function requestLobeHubSprite(): void {
  if (typeof document === "undefined" || spriteReady || spriteRequest) {
    return;
  }
  if (document.getElementById(LOBEHUB_ICON_SPRITE_CONTAINER_ID)) {
    markSpriteReady();
    return;
  }
  if (!spriteRetryAllowed(Date.now())) {
    return;
  }
  spriteRequest = loadLobeHubSprite()
    .then(() => {
      spriteFailureCount = 0;
      markSpriteReady();
    })
    .catch(() => {
      spriteFailureCount += 1;
      spriteLastFailureAt = Date.now();
    })
    .finally(() => {
      spriteRequest = null;
    });
}

function resolveLobeHubSymbolHref(iconUrl: string): string | null {
  const iconID = parseLobeHubIconID(iconUrl);
  return iconID ? `#${iconID}` : null;
}

// ModelIcon renders both bundled sprite icons and administrator-provided image URLs.
export function ModelIcon({
  iconUrl,
  label,
  size = 16,
  className,
  fallbackClassName,
}: {
  iconUrl?: string | null;
  label: string;
  size?: number;
  className?: string;
  fallbackClassName?: string;
}) {
  const dimension = `${size}px`;
  const managedIconNeedsRuntimeBaseURL = iconUrl?.startsWith(MODEL_ICON_API_PREFIX) ?? false;
  const [runtimeApiBaseURL, setRuntimeApiBaseURL] = useState<string | null>(null);
  const resolvedIconURL = managedIconNeedsRuntimeBaseURL
    ? runtimeApiBaseURL === null ? null : `${runtimeApiBaseURL}${iconUrl}`
    : iconUrl;
  const symbolHref = resolvedIconURL ? resolveLobeHubSymbolHref(resolvedIconURL) : null;
  const spriteLoaded = useSyncExternalStore(subscribeSprite, getSpriteReadySnapshot, getSpriteReadyServerSnapshot);
  const [failedImageURL, setFailedImageURL] = useState<string | null>(null);
  const imageFailed = Boolean(resolvedIconURL && failedImageURL === resolvedIconURL);
  const shouldRenderSymbol = Boolean(symbolHref && spriteLoaded);

  useEffect(() => {
    if (managedIconNeedsRuntimeBaseURL) {
      setRuntimeApiBaseURL(resolveApiBaseURL());
    } else {
      setRuntimeApiBaseURL(null);
    }
  }, [managedIconNeedsRuntimeBaseURL]);

  useEffect(() => {
    if (symbolHref && !spriteLoaded) {
      requestLobeHubSprite();
    }
  }, [spriteLoaded, symbolHref]);

  return (
    <span className={cn("inline-flex shrink-0 items-center justify-center", className)} style={{ width: dimension, height: dimension }}>
      {managedIconNeedsRuntimeBaseURL && runtimeApiBaseURL === null ? null : symbolHref && shouldRenderSymbol ? (
        <svg aria-hidden="true" className="block size-full dark:invert" focusable="false">
          <use href={symbolHref} />
        </svg>
      ) : resolvedIconURL && !imageFailed ? (
        <img
          alt=""
          aria-hidden="true"
          className={cn("block size-full object-contain", symbolHref && "dark:invert")}
          decoding="async"
          loading="lazy"
          onError={() => setFailedImageURL(resolvedIconURL)}
          referrerPolicy="no-referrer"
          src={resolvedIconURL}
        />
      ) : (
        <Bot className={cn("size-full text-muted-foreground", fallbackClassName)} />
      )}
      <span className="sr-only">{label}</span>
    </span>
  );
}
