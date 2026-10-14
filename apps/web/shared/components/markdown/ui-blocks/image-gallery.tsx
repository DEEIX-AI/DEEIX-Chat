"use client";

import { useTranslations } from "next-intl";
import * as React from "react";

import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import type { UIBlockDefinition, UIBlockRenderProps, UIBlockSkeletonProps } from "./block";
import { BlockImage, ImageLightbox, type LightboxImage } from "./block-image";
import { joinTextBlocks, textTitle } from "./block-text";
import { safeImageSource, safeLinkURL } from "./block-url";
import { s } from "./schema";
import { UIBlockFrame } from "./ui-block-frame";

type GalleryImage = {
  src: string;
  alt?: string;
  caption?: string;
  // Attribution, e.g. the site or photographer; url is the page the image came from.
  source?: string;
  url?: string;
};

export type ImageGalleryProps = {
  title?: string;
  images: GalleryImage[];
};

// One image gets the full width; two sit side by side; more form a grid.
// Tiles share one aspect ratio so rows line up whatever the source sizes are;
// the lightbox shows each image whole.
function gridClass(count: number): string {
  if (count === 1) return "grid-cols-1";
  if (count === 2) return "grid-cols-2";
  return "grid-cols-2 sm:grid-cols-3";
}

function toLightboxImages(images: readonly GalleryImage[]): LightboxImage[] {
  return images.flatMap((image) => {
    const src = safeImageSource(image.src);
    return src ? [{ src, alt: image.alt?.trim() || image.caption?.trim() || "", caption: image.caption, source: image.source, href: image.url }] : [];
  });
}

function ImageGallery({ id, props }: UIBlockRenderProps<ImageGalleryProps>) {
  const t = useTranslations("chat.markdown");
  const tb = useTranslations("chat.markdown.uiBlock");
  const images = React.useMemo(() => toLightboxImages(props.images), [props.images]);
  const [openIndex, setOpenIndex] = React.useState<number | null>(null);
  const single = images.length === 1;

  return (
    <div className="px-4 pb-4 pt-5">
      {props.title ? (
        <div className="mb-3 flex items-baseline justify-between gap-3">
          <h3 className="min-w-0 text-lg font-semibold leading-7">{props.title}</h3>
          {images.length > 1 ? <span className="shrink-0 text-xs tabular-nums text-muted-foreground">{tb("imageCount", { count: images.length })}</span> : null}
        </div>
      ) : null}
      {images.length === 0 ? (
        <p className="py-6 text-center text-xs text-muted-foreground">{t("imageUnavailable")}</p>
      ) : (
        <div className={cn("grid gap-3", gridClass(images.length))}>
          {images.map((image, index) => {
            const link = safeLinkURL(image.href);
            return (
              <figure key={`${id}-${index}-${image.src}`} className="min-w-0">
                <BlockImage
                  src={image.src}
                  alt={image.alt}
                  fit={single ? "contain" : "cover"}
                  className={cn("w-full rounded-lg", single ? "aspect-[16/10]" : "aspect-[4/3]")}
                  onOpen={() => setOpenIndex(index)}
                />
                {image.caption || image.source || link ? (
                  <figcaption className="mt-1.5 space-y-0.5">
                    {image.caption ? <p className="line-clamp-2 text-xs leading-5" title={image.caption}>{image.caption}</p> : null}
                    {image.source || link ? (
                      <p className="truncate text-[11px] text-muted-foreground">{image.source || link?.host}</p>
                    ) : null}
                  </figcaption>
                ) : null}
              </figure>
            );
          })}
        </div>
      )}
      <ImageLightbox images={images} index={openIndex} onIndexChange={setOpenIndex} />
    </div>
  );
}

// Title row, then tiles in the grid the images will take.
function ImageGallerySkeleton({ items = 0 }: UIBlockSkeletonProps) {
  const count = Math.max(1, items);
  return (
    <UIBlockFrame className="px-4 pb-4 pt-5">
      <Skeleton className="mb-3 h-7 w-1/3 rounded-md" />
      <div className={cn("grid gap-3", gridClass(count))}>
        {Array.from({ length: count }).map((_, index) => (
          <Skeleton key={`image-gallery-${index}`} className={cn("w-full rounded-lg", count === 1 ? "aspect-[16/10]" : "aspect-[4/3]")} />
        ))}
      </div>
    </UIBlockFrame>
  );
}

function imageGalleryText(props: ImageGalleryProps): string {
  const images = toLightboxImages(props.images).map((image) => {
    const details = [image.caption?.trim(), image.source?.trim()].filter(Boolean).join(" · ");
    return `![${image.alt.replaceAll("]", "\\]")}](${image.src})${details ? `\n${details}` : ""}`;
  });
  return joinTextBlocks([...textTitle(props.title), ...images]);
}

export const imageGalleryDefinition: UIBlockDefinition<ImageGalleryProps> = {
  name: "image-gallery",
  version: 1,
  schema: s.object(
    {
      title: s.string(),
      images: s.array(
        s.object(
          {
            src: s.string(),
            alt: s.string(),
            caption: s.string(),
            source: s.string(),
            url: s.string(),
          },
          ["src"],
        ),
        { minItems: 1 },
      ),
    },
    ["images"],
  ),
  Component: ImageGallery,
  Skeleton: ImageGallerySkeleton,
  toText: imageGalleryText,
};
