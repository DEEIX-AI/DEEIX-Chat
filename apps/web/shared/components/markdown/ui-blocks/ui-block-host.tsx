"use client";

import { parseBlockSource } from "@deeix/core";
import { ChevronDown, LayoutGrid, PanelRight } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { HeightTransition } from "@/components/ui/height-transition";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { CollapsibleMotionContent } from "@/shared/components/collapsible-motion-content";
import { resolveDefinition, type UIBlockRegistry, type UIBlockSkeletonProps } from "./block";
import { LiveBlock } from "./live/live-block";
import { useUIBlockOpen } from "./ui-block-actions";
import { useUIBlockRegistry } from "./registry";
import type { Issue } from "./schema";
import { UIBlockErrorBoundary } from "./ui-block-error-boundary";
import { UIBlockFrame } from "./ui-block-frame";

type UIBlockHostProps = {
  raw: string;
  streaming: boolean;
};

// The host owns the block's outer spacing and animates every height change —
// the skeleton growing as items stream in, and the swap to the real component —
// so nothing below the block jumps. A component that throws while rendering is
// contained here and falls back to the raw source like an invalid block.
export function UIBlockHost({ raw, streaming }: UIBlockHostProps) {
  return (
    <HeightTransition className="my-3">
      <UIBlockErrorBoundary resetKey={raw} fallback={<UIBlockFallback reason="crashed" component={peekComponentName(raw)} raw={raw} />}>
        <UIBlockBody raw={raw} streaming={streaming} />
      </UIBlockErrorBoundary>
    </HeightTransition>
  );
}

function UIBlockBody({ raw, streaming }: UIBlockHostProps) {
  const registry = useUIBlockRegistry();
  const parsed = React.useMemo(() => parseBlockSource(raw, streaming), [raw, streaming]);

  if (parsed.status === "incomplete") {
    return <UIBlockSkeleton registry={registry} raw={raw} />;
  }
  if (parsed.status === "invalid") {
    return <UIBlockFallback reason="invalid" component={peekComponentName(raw)} issues={[{ path: "$", message: parsed.message }]} raw={raw} />;
  }

  const { block } = parsed;
  // A block with state, computed values or children renders node by node while
  // it streams in; a static block waits until it is complete.
  if (block.live) {
    return (
      <UIBlockOpenable blockKey={streaming ? "" : block.key}>
        <UIBlockFrame>
          <LiveBlock block={block} streaming={streaming} />
        </UIBlockFrame>
      </UIBlockOpenable>
    );
  }
  if (!block.root.closed) {
    // A finished message whose block was cut off will never complete.
    if (!streaming) {
      return <UIBlockFallback reason="invalid" component={block.component} issues={[{ path: "$", message: "incomplete JSON" }]} raw={raw} />;
    }
    return <UIBlockSkeleton registry={registry} raw={raw} />;
  }
  const definition = resolveDefinition(registry, block.component, block.version);
  if (!definition) {
    return <UIBlockFallback reason="unknown" component={block.component} version={block.version} raw={raw} />;
  }

  const rendered = definition.render(block.key, block.root.props);
  if (!rendered.ok) {
    return <UIBlockFallback reason="invalid" component={block.component} issues={rendered.issues} raw={raw} />;
  }

  return (
    <UIBlockOpenable blockKey={streaming ? "" : block.key}>
      <UIBlockFrame>{rendered.element}</UIBlockFrame>
    </UIBlockOpenable>
  );
}

// A finished block in a message can be opened in the side panel, which shows
// the same block with the same state. The control sits under the block, not on
// it, so it never covers the component's own controls; on pointer devices it
// appears on hover.
function UIBlockOpenable({ blockKey, children }: { blockKey: string; children: React.ReactNode }) {
  const t = useTranslations("chat.markdown.uiBlock");
  const open = useUIBlockOpen();
  if (!open || !blockKey) {
    return children;
  }
  return (
    <div className="group/ui-block">
      {children}
      <div className="mt-1 flex justify-end">
        <button
          type="button"
          onClick={() => open(blockKey)}
          className="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[11px] text-muted-foreground transition-[opacity,color,background-color] hover:bg-accent hover:text-accent-foreground focus-visible:opacity-100 [@media(hover:hover)]:opacity-0 [@media(hover:hover)]:group-hover/ui-block:opacity-100"
        >
          <PanelRight className="size-3" strokeWidth={1.8} />
          {t("openInPanel")}
        </button>
      </div>
    </div>
  );
}

function UIBlockSkeleton({ registry, raw }: { registry: UIBlockRegistry; raw: string }) {
  const Skeleton = resolveDefinition(registry, peekComponentName(raw), 1)?.Skeleton;
  const items = peekItemCount(raw);
  return Skeleton ? <Skeleton items={items} /> : <GenericSkeleton items={items} />;
}

// While streaming, the component name is usually the first key to arrive;
// reading it lets the host show a component-specific skeleton early.
function peekComponentName(raw: string): string {
  const match = /"component"\s*:\s*"([a-z0-9-]+)"/.exec(raw);
  return match?.[1] ?? "";
}

// Objects that have started inside the first array of props ≈ items so far.
// Every builtin keeps its list (items, tasks, rows, questions…) as the first
// array, so the skeleton can grow one row at a time with the stream.
function peekItemCount(raw: string): number {
  const start = raw.indexOf("[");
  if (start < 0) {
    return 0;
  }
  let count = 0;
  let depth = 0;
  let inString = false;
  for (let index = start + 1; index < raw.length; index += 1) {
    const char = raw[index];
    if (inString) {
      if (char === "\\") {
        index += 1;
      } else if (char === '"') {
        inString = false;
      }
      continue;
    }
    if (char === '"') {
      inString = true;
    } else if (char === "{") {
      if (depth === 0) {
        count += 1;
      }
      depth += 1;
    } else if (char === "}") {
      depth = Math.max(0, depth - 1);
    } else if (char === "]" && depth === 0) {
      break;
    }
  }
  return count;
}


function GenericSkeleton({ items = 0 }: UIBlockSkeletonProps) {
  return (
    <UIBlockFrame className="p-4">
      <Skeleton className="h-5 w-1/3 rounded-full" />
      <div className="mt-3 space-y-2">
        {Array.from({ length: Math.max(2, items) }).map((_, index) => (
          <Skeleton key={`ui-block-skeleton-${index}`} className="h-8 rounded-md" />
        ))}
      </div>
    </UIBlockFrame>
  );
}

type UIBlockFallbackProps = {
  reason: "invalid" | "unknown" | "crashed";
  component?: string;
  version?: number;
  issues?: Issue[];
  raw: string;
};

function UIBlockFallback({ reason, component, version, issues, raw }: UIBlockFallbackProps) {
  const t = useTranslations("chat.markdown.uiBlock");
  const [open, setOpen] = React.useState(false);
  const contentID = React.useId();
  const title = fallbackTitle(t, reason, component, version);

  return (
    <UIBlockFrame>
      <button
        type="button"
        aria-expanded={open}
        aria-controls={contentID}
        onClick={() => setOpen((value) => !value)}
        className="flex w-full items-center gap-2 px-4 py-3 text-left text-xs text-muted-foreground transition-colors hover:bg-accent/40"
      >
        <LayoutGrid className="size-3.5 shrink-0" strokeWidth={1.8} />
        <span className="min-w-0 flex-1 truncate">{title}</span>
        <ChevronDown className={cn("size-3.5 shrink-0 transition-transform", open && "rotate-180")} strokeWidth={1.8} />
      </button>
      <CollapsibleMotionContent id={contentID} open={open}>
        {issues && issues.length > 0 ? (
          <ul className="border-t-[0.5px] border-border px-4 py-2 font-mono text-[11px] leading-5 text-muted-foreground">
            {issues.slice(0, 8).map((issue) => (
              <li key={`${issue.path}:${issue.message}`}>
                {issue.path}: {issue.message}
              </li>
            ))}
          </ul>
        ) : null}
        {/* The markdown container resets <pre> (no padding, overflow visible), so the wrapper owns padding and scrolling. */}
        <div className="max-h-64 overflow-auto border-t-[0.5px] border-border px-4 py-3">
          <pre className="w-max min-w-full whitespace-pre font-mono text-[11px] leading-5 text-foreground/92">{raw.trim()}</pre>
        </div>
      </CollapsibleMotionContent>
    </UIBlockFrame>
  );
}

function fallbackTitle(
  t: ReturnType<typeof useTranslations<"chat.markdown.uiBlock">>,
  reason: UIBlockFallbackProps["reason"],
  component: string | undefined,
  version: number | undefined,
): string {
  switch (reason) {
    case "unknown":
      return t("unknownComponent", { component: `${component ?? "?"}@${version ?? "?"}` });
    case "crashed":
      return t("renderFailed", { component: component ?? "" });
    default:
      return t("invalidProps", { component: component ?? "" });
  }
}
