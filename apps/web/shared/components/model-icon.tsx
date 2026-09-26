"use client";

import { Bot } from "lucide-react";
import { useEffect, useState } from "react";
import { cn } from "@/lib/utils";
import { resolveApiBaseURL } from "@/shared/api/http-client";

const LOBEHUB_ICON_PREFIX = "/vendor/lobehub-icons/";
const LOBEHUB_ICON_SPRITE = `${LOBEHUB_ICON_PREFIX}__sprite.svg`;
const LOBEHUB_ICON_SPRITE_CONTAINER_ID = "lobehub-icon-sprite";
const MODEL_ICON_API_PREFIX = "/api/v1/llm/icon-assets/";

let spriteReady = false;
let spriteRequest: Promise<void> | null = null;

function parseLobeHubIconID(iconUrl: string): string | null {
  if (!iconUrl.startsWith(LOBEHUB_ICON_PREFIX) || !iconUrl.endsWith(".svg")) {
    return null;
  }
  const iconID = iconUrl.slice(LOBEHUB_ICON_PREFIX.length, -4);
  return iconID && iconID !== "__sprite" ? iconID : null;
}

function ensureLobeHubSprite(): Promise<void> {
  if (typeof document === "undefined") {
    return Promise.resolve();
  }
  if (spriteReady || document.getElementById(LOBEHUB_ICON_SPRITE_CONTAINER_ID)) {
    spriteReady = true;
    return Promise.resolve();
  }
  if (spriteRequest) {
    return spriteRequest;
  }
  spriteRequest = fetch(LOBEHUB_ICON_SPRITE, { cache: "force-cache" })
    .then(async (response) => {
      if (!response.ok) {
        throw new Error(`Failed to load LobeHub icon sprite: ${response.status}`);
      }
      const sprite = await response.text();
      if (document.getElementById(LOBEHUB_ICON_SPRITE_CONTAINER_ID)) {
        spriteReady = true;
        return;
      }
      const container = document.createElement("div");
      container.id = LOBEHUB_ICON_SPRITE_CONTAINER_ID;
      container.hidden = true;
      container.setAttribute("aria-hidden", "true");
      container.innerHTML = sprite;
      document.body.prepend(container);
      spriteReady = true;
    })
    .catch(() => {
      spriteReady = false;
    })
    .finally(() => {
      spriteRequest = null;
    });
  return spriteRequest;
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
  const [spriteLoaded, setSpriteLoaded] = useState(spriteReady);
  const [failedImageURL, setFailedImageURL] = useState<string | null>(null);
  const imageFailed = Boolean(resolvedIconURL && failedImageURL === resolvedIconURL);
  const shouldRenderSymbol = Boolean(symbolHref && (spriteReady || spriteLoaded));

  useEffect(() => {
    if (managedIconNeedsRuntimeBaseURL) {
      setRuntimeApiBaseURL(resolveApiBaseURL());
    } else {
      setRuntimeApiBaseURL(null);
    }
  }, [managedIconNeedsRuntimeBaseURL]);

  useEffect(() => {
    if (!symbolHref || spriteReady) {
      return;
    }
    let cancelled = false;
    void ensureLobeHubSprite().then(() => {
      if (!cancelled) {
        setSpriteLoaded(spriteReady);
      }
    });
    return () => {
      cancelled = true;
    };
  }, [symbolHref]);

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
