"use client";

import * as React from "react";
import Link from "next/link";

import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { ChatPromptTrace, ChatTraceBlock, RAGCitation } from "@/features/chat/types/messages";
import type { ProcessTraceLabels } from "@/features/chat/hooks/use-chat-trace-labels";
import {
  displayTraceStageLabel,
  displayTraceTrigger,
  filterProcessTraceStages,
  isFileContextTraceStage,
  isRAGTraceStage,
  isTraceStageError,
  localizeTraceDetailItems,
  mergePromptTraceStage,
  normalizeTraceListItem,
  parseStructuredTraceStages,
  parseTraceStages,
  type FileContextBadge,
  type TraceStage,
} from "@/features/chat/model/message-process-trace";
import { StreamdownRender } from "@/shared/components/markdown/streamdown-render";
import { cn } from "@/lib/utils";

export const TRACE_ROOT_CLASS = "chat-screenshot-omit mb-2 w-full pr-4 sm:pr-6";

function FileContextBadgeList({ badges }: { badges: FileContextBadge[] }) {
  if (badges.length === 0) return null;
  return (
    <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
      {badges.map((item, index) => {
        const className =
          "inline-flex max-w-[180px] items-center gap-1 rounded-full border border-border/35 bg-background/45 px-1.5 py-0 text-[11px] leading-5 text-muted-foreground/76 transition-colors hover:border-border hover:text-foreground";
        const content = (
          <>
            <span className="truncate font-medium">{item.name}</span>
            <span className="shrink-0 text-muted-foreground/50">{item.label}</span>
          </>
        );
        const tooltip = (
          <TooltipContent side="top" className="max-w-[320px] break-words">
            <div className="space-y-1">
              <div className="font-medium text-background">{item.name}</div>
              {item.description ? <div>{item.description}</div> : null}
            </div>
          </TooltipContent>
        );
        if (item.fileID) {
          return (
            <Tooltip key={`${item.fileID}-${item.label}-${index}`}>
              <TooltipTrigger asChild>
                <Link
                  href={`/files?file=${encodeURIComponent(item.fileID)}&tab=${item.tab}`}
                  className={className}
                >
                  {content}
                </Link>
              </TooltipTrigger>
              {tooltip}
            </Tooltip>
          );
        }
        return (
          <Tooltip key={`${item.name}-${item.label}-${index}`}>
            <TooltipTrigger asChild>
              <span className={className}>{content}</span>
            </TooltipTrigger>
            {tooltip}
          </Tooltip>
        );
      })}
    </div>
  );
}

type GroupedRAGCitation = {
  fileID: string;
  fileName: string;
  chunkCount: number;
  sharePercent: number;
  maxScore: number;
  previews: string[];
};

function groupRAGCitations(citations: RAGCitation[], labels: ProcessTraceLabels): GroupedRAGCitation[] {
  if (citations.length === 0) return [];
  const grouped = new Map<string, GroupedRAGCitation>();
  for (const item of citations) {
    const fileID = item.file_id?.trim() || "unknown";
    const fileName = item.file_name?.trim() || labels.rag.sourceFallback(fileID);
    const current = grouped.get(fileID) ?? {
      fileID,
      fileName,
      chunkCount: 0,
      sharePercent: 0,
      maxScore: 0,
      previews: [],
    };
    current.chunkCount += 1;
    current.maxScore = Math.max(current.maxScore, item.score || 0);
    const preview = item.preview
      ?.replace(/\s+/g, " ")
      .replace(/\.{4,}/g, "…")
      .replace(/…{2,}/g, "…")
      .trim();
    if (preview && !current.previews.includes(preview)) {
      current.previews.push(preview);
    }
    grouped.set(fileID, current);
  }
  const total = citations.length;
  return Array.from(grouped.values())
    .map((item) => ({
      ...item,
      sharePercent: total > 0 ? Math.round((item.chunkCount / total) * 100) : 0,
    }))
    .sort((left, right) => {
      if (right.chunkCount !== left.chunkCount) return right.chunkCount - left.chunkCount;
      return right.maxScore - left.maxScore;
    });
}

export function RAGCitationList({
  citations,
  embedded = false,
  labels,
  className,
  showScores = true,
}: {
  citations: RAGCitation[];
  embedded?: boolean;
  labels: ProcessTraceLabels;
  className?: string;
  showScores?: boolean;
}) {
  if (citations.length === 0) return null;
  const grouped = groupRAGCitations(citations, labels);
  if (embedded) {
    return (
      <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
        {grouped.map((item) => (
          <span
            key={item.fileID}
            className="inline-flex max-w-[180px] items-center gap-1 rounded-full border border-border/35 bg-background/45 px-1.5 py-0 text-[11px] leading-5 text-muted-foreground/76"
            title={item.previews.join("\n")}
          >
            <span className="truncate font-medium">{item.fileName}</span>
            <span className="shrink-0 text-muted-foreground/50">
              {labels.rag.chunksShort(item.chunkCount, Math.round(item.maxScore * 100))}
            </span>
          </span>
        ))}
      </div>
    );
  }
  return (
    <div className={cn("border-t border-border/25 pt-2", className)}>
      <div className="mb-2 flex items-center justify-between gap-3 px-0.5">
        <span className="text-[11px] font-medium text-foreground/72">{labels.rag.retrievalSources}</span>
        <span className="text-[10px] text-muted-foreground/50">{labels.rag.chunksTotal(citations.length)}</span>
      </div>

      <div className="space-y-3">
        {grouped.map((item) => {
          const itemMeta = showScores
            ? labels.rag.chunksShort(item.chunkCount, Math.round(item.maxScore * 100))
            : grouped.length > 1
              ? labels.rag.chunksTotal(item.chunkCount)
              : null;
          return (
            <div key={item.fileID}>
              <div className="flex min-w-0 items-center justify-between gap-3 px-0.5">
                <div className="flex min-w-0 items-center gap-2">
                  <span className="size-1 shrink-0 rounded-full bg-foreground/40" aria-hidden="true" />
                  <div className="truncate text-[11px] font-medium text-foreground/82" title={item.fileName}>{item.fileName}</div>
                </div>
                {itemMeta ? <span className="shrink-0 text-[10px] text-muted-foreground/50">{itemMeta}</span> : null}
              </div>
              {item.previews.length > 0 ? (
                <div className="mt-1.5 divide-y divide-border/20 overflow-hidden rounded-md bg-muted/25">
                  {item.previews.slice(0, 3).map((preview, index) => (
                    <div key={`${item.fileID}-${index}`} className="px-2.5 py-2">
                      <p
                        className="line-clamp-2 text-[10px] leading-4 text-muted-foreground/68"
                        title={preview}
                      >
                        {preview}
                      </p>
                    </div>
                  ))}
                </div>
              ) : null}
            </div>
          );
        })}
      </div>
    </div>
  );
}

function StreamingTraceText({
  text,
  active,
  className,
}: {
  text: string;
  active: boolean;
  className?: string;
}) {
  const chars = React.useMemo(() => Array.from(text), [text]);
  const [visibleCount, setVisibleCount] = React.useState(() => (active ? 0 : chars.length));

  React.useEffect(() => {
    if (!active) {
      setVisibleCount(chars.length);
      return;
    }

    setVisibleCount(0);
    const timer = window.setInterval(() => {
      setVisibleCount((current) => {
        if (current >= chars.length) {
          window.clearInterval(timer);
          return chars.length;
        }
        return Math.min(chars.length, current + 2);
      });
    }, 18);

    return () => window.clearInterval(timer);
  }, [active, chars.length, text]);

  return <span className={className}>{chars.slice(0, visibleCount).join("")}</span>;
}

function TraceStageRows({
  stages,
  streaming,
  citations,
  fileBadges,
  payloadJson,
  labels,
}: {
  stages: TraceStage[];
  streaming: boolean;
  citations: RAGCitation[];
  fileBadges: FileContextBadge[];
  payloadJson?: string;
  labels: ProcessTraceLabels;
}) {
  return (
    <ol className="space-y-0.5">
      {stages.map((stage, index) => {
        const isError = isTraceStageError(stage);
        const activeStreamingStage = streaming && index === stages.length - 1;
        const detailItems = stage.details.length > 0 ? stage.details : stage.detail ? [stage.detail] : [];
        const displayDetailItems = localizeTraceDetailItems(stage, detailItems, payloadJson, labels);
        const showCitations = isRAGTraceStage(stage) && citations.length > 0;
        const showFileBadges = isFileContextTraceStage(stage) && fileBadges.length > 0;
        return (
          <li
            key={`${stage.label}-${index}`}
            className={cn(
              "group/stage grid grid-cols-[0.875rem_8rem_minmax(0,1fr)] gap-x-5 gap-y-0.5 text-[12px] leading-5",
              "max-sm:grid-cols-[0.875rem_minmax(0,1fr)] max-sm:gap-x-2",
            )}
          >
            <div className="relative flex justify-center">
              {index > 0 ? <span className="absolute -top-0.5 bottom-1/2 w-px bg-border/42" /> : null}
              {index < stages.length - 1 ? <span className="absolute bottom-[-0.125rem] top-1/2 w-px bg-border/42" /> : null}
              <span
                className={cn(
                  "relative z-10 mt-[0.45rem] size-1.5 rounded-full bg-muted-foreground/38 ring-4 ring-background transition-colors group-hover/stage:bg-foreground/58",
                  isError && "bg-destructive/80",
                )}
              />
            </div>
            <div className="min-w-0 max-sm:col-start-2">
              <span
                className={cn(
                  "block truncate font-medium text-muted-foreground/76 transition-colors group-hover/stage:text-foreground/88",
                  isError && "text-destructive/85 group-hover/stage:text-destructive",
                )}
              >
                {displayTraceStageLabel(stage.label, labels)}
              </span>
            </div>
            <div className="min-w-0 space-y-0.5 pb-2 max-sm:col-start-2">
              {stage.trigger ? (
                <div className={cn("break-words text-[12px] leading-5 text-muted-foreground/58", isError && "text-destructive/65")}>
                  {displayTraceTrigger(stage.trigger, labels)}
                </div>
              ) : null}
              {displayDetailItems.map((detailText, detailIndex) => (
                /^[-*]\s+/.test(detailText) ? (
                  <div
                    key={`${stage.label}-${index}-detail-${detailIndex}`}
                    className={cn("flex min-w-0 gap-1.5 text-muted-foreground/84", isError && "text-destructive/80")}
                  >
                    <span className="mt-[0.45rem] size-1 shrink-0 rounded-full bg-current opacity-45" />
                    <p className="min-w-0 whitespace-normal break-words">
                      <StreamingTraceText text={normalizeTraceListItem(detailText)} active={activeStreamingStage} />
                    </p>
                  </div>
                ) : (
                  <p
                    key={`${stage.label}-${index}-detail-${detailIndex}`}
                    className={cn("min-w-0 whitespace-normal break-words text-muted-foreground/84", isError && "text-destructive/80")}
                  >
                    <StreamingTraceText text={detailText} active={activeStreamingStage} />
                  </p>
                )
              ))}
              {showFileBadges ? <FileContextBadgeList badges={fileBadges} /> : null}
              {showCitations ? <RAGCitationList citations={citations} embedded labels={labels} /> : null}
            </div>
          </li>
        );
      })}
    </ol>
  );
}

export function TraceContent({
  block,
  streaming,
  citations = [],
  fileBadges = [],
  promptTrace,
  labels,
}: {
  block: ChatTraceBlock;
  streaming: boolean;
  citations?: RAGCitation[];
  fileBadges?: FileContextBadge[];
  promptTrace?: ChatPromptTrace;
  labels: ProcessTraceLabels;
}) {
  const structuredStages = parseStructuredTraceStages(block.payloadJson, labels);
  const parsedStages = structuredStages.length > 0 ? [] : parseTraceStages(block.contentMarkdown);
  const stages = filterProcessTraceStages(mergePromptTraceStage(structuredStages.length > 0 ? structuredStages : parsedStages, promptTrace, labels));
  if (stages.length > 0) {
    return (
      <TraceStageRows
        stages={stages}
        streaming={streaming}
        citations={citations}
        fileBadges={fileBadges}
        payloadJson={block.payloadJson}
        labels={labels}
      />
    );
  }
  if (parsedStages.length > 0) {
    return null;
  }
  if (!block.contentMarkdown.trim()) {
    return null;
  }

  return (
    <section className="text-[12px] leading-5 text-muted-foreground/84">
      <StreamdownRender
        content={block.contentMarkdown}
        streaming={streaming}
        variant="thinking"
        className="[&_ul]:my-0 [&_ul]:space-y-0.5 [&_li]:pl-0 [&_li]:leading-5"
      />
    </section>
  );
}
