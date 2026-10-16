"use client";

import { ChevronLeft, ChevronRight, Download, ExternalLink, ImageOff, X } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { cn } from "@/lib/utils";
import { MediaActionBar, MediaActionButton } from "@/shared/components/media-action-bar";
import { useImageDisplaySource } from "@/shared/hooks/use-image-display-source";
import { downloadMarkdownImageSource, resolveMarkdownImageDownloadName } from "@/shared/lib/markdown-image-source";
import { safeLinkURL } from "./block-url";

type BlockImageProps = {
  src: string;
  alt: string;
  // Sizing of the frame. The frame owns the aspect ratio, so nothing below
  // moves when the image arrives or fails.
  className?: string;
  fit?: "cover" | "contain";
  // A failed image keeps its frame with a placeholder (with the alt text, or
  // just an icon where the frame is a thumbnail), or removes itself where the
  // rest of the item stands on its own (a card's cover).
  fallback?: "placeholder" | "icon" | "hide";
  onOpen?: () => void;
};

/** An image from component props: skeleton while loading, fades in, degrades on failure. */
export function BlockImage({ src, alt, className, fit = "cover", fallback = "placeholder", onOpen }: BlockImageProps) {
  const t = useTranslations("chat.markdown");
  const { displaySrc, fetchFailed } = useImageDisplaySource(src);
  const [loadedSrc, setLoadedSrc] = React.useState("");
  const [erroredSrc, setErroredSrc] = React.useState("");
  const loaded = Boolean(displaySrc) && loadedSrc === displaySrc;
  const failed = fetchFailed || (Boolean(displaySrc) && erroredSrc === displaySrc);

  if (failed && fallback === "hide") {
    return null;
  }

  const frameClass = cn("relative block overflow-hidden bg-muted/40", className);
  const content = failed ? (
    <span className="absolute inset-0 flex flex-col items-center justify-center gap-1.5 p-3 text-center text-muted-foreground">
      <ImageOff className="size-4 shrink-0" strokeWidth={1.6} aria-label={fallback === "icon" ? alt || t("imageUnavailable") : undefined} />
      {fallback === "icon" ? null : <span className="line-clamp-2 text-[11px] leading-4">{alt || t("imageUnavailable")}</span>}
    </span>
  ) : (
    <>
      {loaded ? null : <span className="absolute inset-0 animate-pulse bg-muted/60" />}
      {displaySrc ? (
        <img
          src={displaySrc}
          alt={alt}
          loading="lazy"
          decoding="async"
          draggable={false}
          // Many image hosts refuse hotlinks by Referer, and the conversation URL is nobody's business.
          referrerPolicy="no-referrer"
          onLoad={(event) => setLoadedSrc(event.currentTarget.getAttribute("src") ?? "")}
          onError={(event) => setErroredSrc(event.currentTarget.getAttribute("src") ?? "")}
          className={cn(
            "absolute inset-0 size-full transition-opacity duration-300",
            fit === "cover" ? "object-cover" : "object-contain",
            loaded ? "opacity-100" : "opacity-0",
          )}
        />
      ) : null}
    </>
  );

  if (onOpen && !failed) {
    return (
      <button
        type="button"
        onClick={onOpen}
        aria-label={alt ? `${t("previewImage")}: ${alt}` : t("previewImage")}
        className={cn(frameClass, "cursor-zoom-in outline-none focus-visible:ring-2 focus-visible:ring-ring/60")}
      >
        {content}
      </button>
    );
  }
  return <span className={frameClass}>{content}</span>;
}

export type LightboxImage = {
  src: string;
  alt: string;
  caption?: string;
  source?: string;
  href?: string;
};

type ImageLightboxProps = {
  images: readonly LightboxImage[];
  // Index of the image on show; null keeps the lightbox closed.
  index: number | null;
  onIndexChange: (index: number | null) => void;
};

/** Full-size preview with caption and attribution; arrows step through `images`. */
export function ImageLightbox({ images, index, onIndexChange }: ImageLightboxProps) {
  const t = useTranslations("chat.markdown");
  const tb = useTranslations("chat.markdown.uiBlock");
  const common = useTranslations("common.actions");
  const current = index !== null ? images[index] : undefined;
  const count = images.length;
  const step = (delta: number) => {
    if (index !== null && count > 1) {
      onIndexChange((index + delta + count) % count);
    }
  };
  const link = safeLinkURL(current?.href);

  return (
    <Dialog
      open={current !== undefined}
      onOpenChange={(open) => {
        if (!open) {
          onIndexChange(null);
        }
      }}
    >
      <DialogContent
        className="flex w-fit max-w-[96vw] flex-col items-center gap-2 overflow-visible border-0 bg-transparent p-0 shadow-none backdrop-blur-none sm:max-w-[96vw]"
        onKeyDown={(event) => {
          if (event.key === "ArrowLeft") {
            event.preventDefault();
            step(-1);
          } else if (event.key === "ArrowRight") {
            event.preventDefault();
            step(1);
          }
        }}
      >
        <DialogTitle className="sr-only">{current?.caption || current?.alt || t("previewImage")}</DialogTitle>
        <DialogDescription className="sr-only">{t("previewImage")}</DialogDescription>
        {current ? (
          <>
            <div className="relative">
              <LightboxPicture key={current.src} image={current} />
              <MediaActionBar className="absolute right-2 top-2">
                <MediaActionButton label={t("downloadImage")} onClick={() => void downloadImage(current, () => toast.error(t("downloadImageFailed")))}>
                  <Download className="size-3.5" />
                </MediaActionButton>
                <MediaActionButton label={common("close")} onClick={() => onIndexChange(null)}>
                  <X className="size-3.5" />
                </MediaActionButton>
              </MediaActionBar>
              {count > 1 ? (
                <>
                  <LightboxStep side="left" label={tb("previousImage")} onClick={() => step(-1)} />
                  <LightboxStep side="right" label={tb("nextImage")} onClick={() => step(1)} />
                </>
              ) : null}
            </div>
            {current.caption || current.source || link || count > 1 ? (
              <div className="flex max-w-[min(48rem,96vw)] items-baseline gap-3 rounded-lg bg-background/95 px-3 py-2 text-xs shadow-lg">
                {count > 1 ? <span className="shrink-0 tabular-nums text-muted-foreground">{`${(index ?? 0) + 1} / ${count}`}</span> : null}
                {current.caption ? <p className="min-w-0 flex-1 leading-5 text-foreground">{current.caption}</p> : <span className="flex-1" />}
                {current.source || link ? (
                  link ? (
                    <a
                      href={link.href}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex shrink-0 items-center gap-1 text-muted-foreground hover:text-foreground"
                    >
                      <ExternalLink className="size-3" strokeWidth={1.8} />
                      {current.source || link.host}
                    </a>
                  ) : (
                    <span className="shrink-0 text-muted-foreground">{current.source}</span>
                  )
                ) : null}
              </div>
            ) : null}
          </>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

function LightboxPicture({ image }: { image: LightboxImage }) {
  const t = useTranslations("chat.markdown");
  const { displaySrc, fetchFailed } = useImageDisplaySource(image.src);
  const [erroredSrc, setErroredSrc] = React.useState("");
  if (fetchFailed || (displaySrc && erroredSrc === displaySrc)) {
    return (
      <div className="flex h-60 w-80 max-w-[90vw] flex-col items-center justify-center gap-2 rounded-lg bg-background/95 text-sm text-muted-foreground">
        <ImageOff className="size-5" strokeWidth={1.6} />
        {image.alt || t("imageUnavailable")}
      </div>
    );
  }
  if (!displaySrc) {
    return <div className="h-60 w-80 max-w-[90vw] animate-pulse rounded-lg bg-background/40" />;
  }
  return (
    <img
      src={displaySrc}
      alt={image.alt}
      referrerPolicy="no-referrer"
      onError={(event) => setErroredSrc(event.currentTarget.getAttribute("src") ?? "")}
      className="block max-h-[82vh] max-w-[96vw] rounded-lg bg-background/5 object-contain shadow-2xl"
    />
  );
}

function LightboxStep({ side, label, onClick }: { side: "left" | "right"; label: string; onClick: () => void }) {
  const Icon = side === "left" ? ChevronLeft : ChevronRight;
  return (
    <MediaActionBar className={cn("absolute top-1/2 -translate-y-1/2", side === "left" ? "left-2" : "right-2")}>
      <MediaActionButton label={label} onClick={onClick} className="size-8">
        <Icon className="size-4" />
      </MediaActionButton>
    </MediaActionBar>
  );
}

async function downloadImage(image: LightboxImage, onFailure: () => void) {
  try {
    await downloadMarkdownImageSource(image.src, resolveMarkdownImageDownloadName(image.src, image.alt));
  } catch {
    // Public hosts often refuse cross-origin fetches; the browser can still save the file itself.
    // Conversation files need the bearer token, so opening them directly would only 401.
    if (!image.src.startsWith("/api/v1/")) {
      window.open(image.src, "_blank", "noopener,noreferrer");
      return;
    }
    onFailure();
  }
}
