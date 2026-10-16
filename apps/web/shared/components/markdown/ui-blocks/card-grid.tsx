"use client";

import { ExternalLink } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { HeightTransition } from "@/components/ui/height-transition";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { SlideSwitch } from "@/shared/components/slide-switch";
import type { UIBlockDefinition, UIBlockRenderProps, UIBlockSkeletonProps } from "./block";
import { BlockImage, ImageLightbox } from "./block-image";
import { joinTextBlocks, textTitle } from "./block-text";
import { safeImageSource, safeLinkURL } from "./block-url";
import { s } from "./schema";
import { UIBlockFrame } from "./ui-block-frame";
import { useUIBlockState } from "./ui-block-state";

type CardGridItem = {
  tag?: string;
  source?: string;
  title: string;
  summary?: string;
  url?: string;
  // Cover image; a card whose image fails to load simply shows without one.
  image?: string;
};

export type CardGridProps = {
  title: string;
  items: CardGridItem[];
};

const ALL_FILTER = "*";

function CardGrid({ id, props }: UIBlockRenderProps<CardGridProps>) {
  const t = useTranslations("chat.markdown.uiBlock");
  // Slide direction follows tab order: picking a chip to the right slides in from the right.
  const [direction, setDirection] = React.useState<1 | -1>(1);
  // Filter chips derive from item tags in first-seen order; the model does not declare them separately.
  const filters = React.useMemo(
    () => Array.from(new Set(props.items.map((item) => item.tag).filter((tag): tag is string => Boolean(tag)))),
    [props.items],
  );
  // A stored tag the items no longer carry falls back to showing everything.
  const [{ filter }, setState] = useUIBlockState(id, {
    defaults: { filter: ALL_FILTER },
    restore: (saved, defaults) => (typeof saved?.filter === "string" && filters.includes(saved.filter) ? { filter: saved.filter } : defaults),
    title: props.title,
  });
  const visible = React.useMemo(
    () => (filter === ALL_FILTER ? props.items : props.items.filter((item) => item.tag === filter)),
    [filter, props.items],
  );
  const tabs = React.useMemo(() => [ALL_FILTER, ...filters], [filters]);
  // Covers of cards without a link open here; a linked card's cover is part of the link.
  const [preview, setPreview] = React.useState<string | null>(null);
  const previewItem = preview ? props.items.find((item) => safeImageSource(item.image) === preview) : undefined;
  const selectFilter = (next: string) => {
    setDirection(tabs.indexOf(next) >= tabs.indexOf(filter) ? 1 : -1);
    setState({ filter: next });
  };

  return (
    <div className="px-4 pb-4 pt-5">
      <div className="flex items-baseline justify-between gap-3">
        <h3 className="min-w-0 text-lg font-semibold leading-7">{props.title}</h3>
        <span className="shrink-0 text-xs tabular-nums text-muted-foreground">{t("itemCount", { count: visible.length })}</span>
      </div>

      {filters.length > 1 ? (
        <div className="mt-3 flex flex-wrap gap-1.5" role="tablist" aria-label={t("filters")}>
          <FilterChip active={filter === ALL_FILTER} onClick={() => selectFilter(ALL_FILTER)}>
            {t("all")}
          </FilterChip>
          {filters.map((value) => (
            <FilterChip key={`${id}-${value}`} active={filter === value} onClick={() => selectFilter(value)}>
              {value}
            </FilterChip>
          ))}
        </div>
      ) : null}

      <HeightTransition className="mt-3">
        <SlideSwitch itemKey={filter} direction={direction}>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            {visible.map((item, index) => (
              <CardGridCard key={`${id}-${index}-${item.title}`} item={item} onOpenImage={(src) => setPreview(src)} />
            ))}
          </div>
          {visible.length === 0 ? <p className="py-6 text-center text-xs text-muted-foreground">{t("empty")}</p> : null}
        </SlideSwitch>
      </HeightTransition>
      <ImageLightbox
        images={preview ? [{ src: preview, alt: previewItem?.title ?? "", caption: previewItem?.title, source: previewItem?.source }] : []}
        index={preview ? 0 : null}
        onIndexChange={(index) => (index === null ? setPreview(null) : undefined)}
      />
    </div>
  );
}

function FilterChip({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={onClick}
      className={cn(
        "rounded-full border-[0.5px] px-3 py-1 text-xs transition-colors",
        active
          ? "border-primary/40 bg-primary/10 text-foreground"
          : "border-border text-muted-foreground hover:bg-accent hover:text-accent-foreground",
      )}
    >
      {children}
    </button>
  );
}

function CardGridCard({ item, onOpenImage }: { item: CardGridItem; onOpenImage: (src: string) => void }) {
  const link = safeLinkURL(item.url);
  const image = safeImageSource(item.image);
  const body = (
    <>
      {image ? (
        <BlockImage
          src={image}
          alt={item.title}
          fallback="hide"
          className="-mx-3 -mt-3 mb-3 aspect-[16/9] rounded-t-lg"
          onOpen={link ? undefined : () => onOpenImage(image)}
        />
      ) : null}
      <div className="flex items-center justify-between gap-2">
        {item.tag ? <span className="rounded-md bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground">{item.tag}</span> : <span />}
        {item.source ? <span className="min-w-0 truncate text-[11px] text-muted-foreground">{item.source}</span> : null}
      </div>
      <p className="mt-2 text-sm font-medium leading-5">{item.title}</p>
      {item.summary ? <p className="mt-1 text-xs leading-5 text-muted-foreground">{item.summary}</p> : null}
    </>
  );
  const className = "flex flex-col overflow-hidden rounded-lg border-[0.5px] border-border bg-background p-3";
  if (link) {
    return (
      <a href={link.href} target="_blank" rel="noopener noreferrer" className={cn(className, "group transition-colors hover:bg-accent/40")}>
        {body}
        <span className="mt-auto inline-flex items-center gap-1 pt-2.5 text-[11px] text-muted-foreground group-hover:text-foreground">
          <ExternalLink className="size-3 shrink-0" strokeWidth={1.8} />
          <span className="truncate">{link.host}</span>
        </span>
      </a>
    );
  }
  return <div className={className}>{body}</div>;
}

// Mirrors the rendered geometry: title row, filter chips, then cards two-up.
function CardGridSkeleton({ items = 0 }: UIBlockSkeletonProps) {
  const cards = Math.max(2, items);
  return (
    <UIBlockFrame className="px-4 pb-4 pt-5">
      <Skeleton className="h-7 w-1/3 rounded-md" />
      <div className="mt-3 flex gap-1.5">
        {Array.from({ length: 3 }).map((_, index) => (
          <Skeleton key={`card-grid-chip-${index}`} className="h-7 w-16 rounded-full" />
        ))}
      </div>
      <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
        {Array.from({ length: cards }).map((_, index) => (
          <Skeleton key={`card-grid-card-${index}`} className="h-[7.5rem] rounded-lg" />
        ))}
      </div>
    </UIBlockFrame>
  );
}

function cardGridText(props: CardGridProps): string {
  const items = props.items.map((item) => {
    const details = [item.summary?.trim(), item.source?.trim(), safeLinkURL(item.url)?.href].filter(Boolean).join(" · ");
    const tag = item.tag?.trim() ? `[${item.tag.trim()}] ` : "";
    return `- ${tag}**${item.title.trim()}**${details ? ` — ${details}` : ""}`;
  });
  return joinTextBlocks([...textTitle(props.title), items.join("\n")]);
}

export const cardGridDefinition: UIBlockDefinition<CardGridProps> = {
  name: "card-grid",
  version: 1,
  schema: s.object(
    {
      title: s.string(),
      items: s.array(
        s.object(
          {
            tag: s.string(),
            source: s.string(),
            title: s.string(),
            summary: s.string(),
            url: s.string(),
            image: s.string(),
          },
          ["title"],
        ),
        { minItems: 1 },
      ),
    },
    ["title", "items"],
  ),
  Component: CardGrid,
  Skeleton: CardGridSkeleton,
  toText: cardGridText,
};
