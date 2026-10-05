"use client";

import { Bot } from "lucide-react";
import type { CSSProperties, ReactNode } from "react";
import { useEffect, useState } from "react";
import { cn } from "@/lib/utils";
import { lobehubIconTone, lobehubIconURL, parseLobehubIconSlug } from "@/entities/model/lib/lobehub-icons";
import { resolveApiBaseURL } from "@/shared/api/http-client";

const MODEL_ICON_API_PREFIX = "/api/v1/llm/icon-assets/";

// Monochrome icons are drawn with currentColor, which an <img> cannot inherit. Painting the text
// color through the icon as a mask keeps them in step with the surrounding text in both themes.
function maskStyle(url: string): CSSProperties {
  const image = `url("${url}")`;
  return {
    backgroundColor: "currentColor",
    maskImage: image,
    maskPosition: "center",
    maskRepeat: "no-repeat",
    maskSize: "contain",
    WebkitMaskImage: image,
    WebkitMaskPosition: "center",
    WebkitMaskRepeat: "no-repeat",
    WebkitMaskSize: "contain",
  };
}

// ModelIcon renders bundled LobeHub icons and administrator-provided image URLs.
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
  const bundledSlug = resolvedIconURL ? parseLobehubIconSlug(resolvedIconURL) : null;
  // Bundled icons always load from the current versioned path, also when a legacy URL was stored.
  const iconSrc = bundledSlug ? lobehubIconURL(bundledSlug) : resolvedIconURL;
  // Unknown slugs fall through to a plain image, so a missing file still ends in the fallback icon.
  const tone = bundledSlug ? lobehubIconTone(bundledSlug) : null;
  const [failedImageURL, setFailedImageURL] = useState<string | null>(null);
  const imageFailed = Boolean(iconSrc && failedImageURL === iconSrc);

  useEffect(() => {
    if (managedIconNeedsRuntimeBaseURL) {
      setRuntimeApiBaseURL(resolveApiBaseURL());
    } else {
      setRuntimeApiBaseURL(null);
    }
  }, [managedIconNeedsRuntimeBaseURL]);

  let content: ReactNode;
  if (managedIconNeedsRuntimeBaseURL && runtimeApiBaseURL === null) {
    content = null;
  } else if (iconSrc && tone === "mono") {
    content = <span aria-hidden="true" className="block size-full" style={maskStyle(iconSrc)} />;
  } else if (iconSrc && !imageFailed) {
    content = (
      <img
        alt=""
        aria-hidden="true"
        // Mixed icons pair brand colors with currentColor parts that an image renders black; in dark
        // mode invert and rotate the hue back so those parts turn light while the brand colors hold.
        className={cn("block size-full object-contain", tone === "mixed" && "dark:invert dark:hue-rotate-180")}
        decoding="async"
        loading="lazy"
        onError={() => setFailedImageURL(iconSrc)}
        referrerPolicy="no-referrer"
        src={iconSrc}
      />
    );
  } else {
    content = <Bot className={cn("size-full text-muted-foreground", fallbackClassName)} />;
  }

  return (
    <span className={cn("inline-flex shrink-0 items-center justify-center", className)} style={{ width: dimension, height: dimension }}>
      {content}
      <span className="sr-only">{label}</span>
    </span>
  );
}
