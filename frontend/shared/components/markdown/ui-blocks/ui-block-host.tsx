"use client";

import { ChevronDown, LayoutGrid } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { CollapsibleMotionContent } from "@/shared/components/collapsible-motion-content";
import { parseUIBlock, resolveDefinition, validateProps } from "./block";
import { useUIBlockRegistry } from "./registry";
import type { Issue } from "./schema";
import { UIBlockFrame } from "./ui-block-frame";

type UIBlockHostProps = {
  raw: string;
  streaming: boolean;
};

export function UIBlockHost({ raw, streaming }: UIBlockHostProps) {
  const registry = useUIBlockRegistry();
  const parsed = React.useMemo(() => parseUIBlock(raw, streaming), [raw, streaming]);

  if (parsed.status === "incomplete") {
    const Skeleton = resolveDefinition(registry, peekComponentName(raw), 1)?.Skeleton;
    return Skeleton ? <Skeleton /> : <GenericSkeleton />;
  }
  if (parsed.status === "invalid") {
    return <UIBlockFallback reason="invalid" component={peekComponentName(raw)} issues={parsed.issues} raw={parsed.raw} />;
  }

  const { envelope } = parsed;
  const definition = resolveDefinition(registry, envelope.component, envelope.version);
  if (!definition) {
    return <UIBlockFallback reason="unknown" component={envelope.component} version={envelope.version} raw={parsed.raw} />;
  }

  const props = validateProps(definition, envelope.props);
  if (!props.ok) {
    return <UIBlockFallback reason="invalid" component={envelope.component} issues={props.issues} raw={parsed.raw} />;
  }

  const Component = definition.Component;
  return (
    <UIBlockFrame>
      <Component id={envelope.id} props={props.value} definition={definition} />
    </UIBlockFrame>
  );
}

// While streaming, the component name is usually the first key to arrive;
// reading it lets the host show a component-specific skeleton early.
function peekComponentName(raw: string): string {
  const match = /"component"\s*:\s*"([a-z0-9-]+)"/.exec(raw);
  return match?.[1] ?? "";
}


function GenericSkeleton() {
  return (
    <UIBlockFrame className="p-4">
      <Skeleton className="h-4 w-1/3 rounded-full" />
      <div className="mt-4 grid grid-cols-2 gap-3">
        {Array.from({ length: 4 }).map((_, index) => (
          <Skeleton key={`ui-block-skeleton-${index}`} className="h-20 rounded-lg" />
        ))}
      </div>
    </UIBlockFrame>
  );
}

type UIBlockFallbackProps = {
  reason: "invalid" | "unknown";
  component?: string;
  version?: number;
  issues?: Issue[];
  raw: string;
};

function UIBlockFallback({ reason, component, version, issues, raw }: UIBlockFallbackProps) {
  const t = useTranslations("chat.markdown.uiBlock");
  const [open, setOpen] = React.useState(false);
  const contentID = React.useId();
  const title = reason === "unknown"
    ? t("unknownComponent", { component: `${component ?? "?"}@${version ?? "?"}` })
    : t("invalidProps", { component: component ?? "" });

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
