"use client";

import { useEffect, useState } from "react";
import { Bot } from "lucide-react";

import { cn } from "@/lib/utils";

const LOBEHUB_ICON_PREFIX = "/vendor/lobehub-icons/";
const LOBEHUB_ICON_SPRITE = `${LOBEHUB_ICON_PREFIX}__sprite.svg`;
const LOBEHUB_ICON_SPRITE_CONTAINER_ID = "lobehub-icon-sprite";

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

export function LobeHubIcon({
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
  const symbolHref = iconUrl ? resolveLobeHubSymbolHref(iconUrl) : null;
  const [spriteLoaded, setSpriteLoaded] = useState(spriteReady);
  const shouldRenderSymbol = Boolean(symbolHref && (spriteReady || spriteLoaded));

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
      {symbolHref && shouldRenderSymbol ? (
        <svg
          aria-hidden="true"
          className="block size-full dark:invert"
          focusable="false"
        >
          <use href={symbolHref} />
        </svg>
      ) : iconUrl ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          alt=""
          aria-hidden="true"
          className="block size-full object-contain dark:invert"
          decoding="async"
          loading="lazy"
          src={iconUrl}
        />
      ) : (
        <Bot className={cn("size-full text-muted-foreground", fallbackClassName)} />
      )}
      <span className="sr-only">{label}</span>
    </span>
  );
}
