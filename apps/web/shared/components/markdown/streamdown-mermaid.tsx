"use client";

import { TriangleAlert, Workflow } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";
import { type MermaidErrorComponentProps, type MermaidOptions, useIsCodeFenceIncomplete } from "streamdown";

import { Button } from "@/components/ui/button";

/**
 * True while a message contains a Mermaid fence but the lazily imported Mermaid plugin has not
 * arrived yet. Streamdown would otherwise render the diagram source as a plain code block first.
 */
export const MarkdownMermaidPendingContext = React.createContext(false);

/**
 * Single loading surface for every phase before a diagram exists: plugin import, Streamdown's
 * lazy chunk, deferred render, the first Mermaid layout and unparseable partial charts while
 * streaming. Visibility is driven by `StreamdownAdapterStyles`, which hides the upstream
 * placeholders behind this one.
 */
export function MarkdownMermaidLoading() {
  const t = useTranslations("chat.markdown.diagram");

  return (
    <div
      aria-live="polite"
      className="relative my-4 hidden h-40 w-full items-center justify-center overflow-hidden rounded-xl bg-muted/30"
      data-markdown-mermaid-loading=""
      role="status"
    >
      <span
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 [background-image:radial-gradient(var(--border)_1px,transparent_1px)] [background-size:14px_14px] [mask-image:radial-gradient(ellipse_at_center,black_20%,transparent_75%)]"
      />
      <span className="relative inline-flex items-center gap-1.5 text-[12px] font-medium text-muted-foreground">
        <Workflow aria-hidden="true" className="size-3.5 shrink-0" />
        <span className="shimmer">{t("rendering")}</span>
      </span>
    </div>
  );
}

function MarkdownMermaidError({ chart, error, retry }: MermaidErrorComponentProps) {
  const t = useTranslations("chat.markdown.diagram");
  const commonActions = useTranslations("common.actions");
  const fenceStreaming = useIsCodeFenceIncomplete();

  // A chart that is still streaming is usually just unfinished; keep the loading surface
  // instead of flashing a parse error on every token.
  if (fenceStreaming) {
    return null;
  }

  return (
    <div className="my-1 w-full rounded-xl bg-muted/40 px-3 py-2.5 text-left text-[12px]" data-markdown-mermaid-error="">
      <div className="flex items-center justify-between gap-2">
        <span className="inline-flex min-w-0 items-center gap-1.5 font-medium text-muted-foreground">
          <TriangleAlert aria-hidden="true" className="size-3.5 shrink-0" />
          {t("renderFailed")}
        </span>
        <Button className="shrink-0 text-muted-foreground" onClick={retry} size="xs" type="button" variant="ghost">
          {commonActions("retry")}
        </Button>
      </div>
      <div className="mt-1 break-words font-mono text-[11px] leading-5 text-muted-foreground/80">{error}</div>
      <details className="mt-1.5">
        <summary className="cursor-pointer select-none text-[11px] text-muted-foreground transition-colors hover:text-foreground">
          {t("viewSource")}
        </summary>
        <div className="mt-1.5 max-h-60 overflow-auto whitespace-pre rounded-lg bg-background/60 p-2 font-mono text-[11px] leading-5 text-foreground/85">
          {chart}
        </div>
      </details>
    </div>
  );
}

export const STREAMDOWN_MERMAID_OPTIONS: MermaidOptions = {
  errorComponent: MarkdownMermaidError,
};
