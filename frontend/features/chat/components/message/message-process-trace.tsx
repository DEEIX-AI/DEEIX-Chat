"use client";

import * as React from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { ChevronDown } from "@/components/animate-ui/icons/chevron-down";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/animate-ui/components/radix/accordion";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

import type {
  ChatMessageProcessTrace,
  ChatPromptTrace,
  ChatTraceBlock,
  ChatTraceEvent,
  RAGCitation,
} from "@/features/chat/types/messages";
import { StreamdownRender } from "@/shared/components/markdown/streamdown-render";
import { cn } from "@/lib/utils";

const TRACE_KIND_CONTEXT_PLANNING = "context_planning";
const TRACE_KIND_RAG = "content_retrieval";
const TRACE_KIND_FILE_CONTEXT = "file_context";
const TRACE_KIND_CONTEXT_COMPACTION = "context_compaction";
const TRACE_KIND_SKILL_CONTEXT = "skill_context";
// Legacy persisted traces only. New traces are rendered from payload_json.trace_stages.
const TRACE_LABEL_UPSTREAM_RESULT = "\u8bf7\u6c42\u7ed3\u679c";
const TRACE_TRIGGER_UPSTREAM_REQUEST = "\u4e0a\u6e38\u6a21\u578b\u8bf7\u6c42\u89e6\u53d1";
const TRACE_LABEL_CONTEXT_PLANNING = "\u4e0a\u4e0b\u6587\u89c4\u5212";
const TRACE_LABEL_RAG = "\u5185\u5bb9\u68c0\u7d22";
const TRACE_LABEL_FILE_CONTEXT = "\u6587\u4ef6\u4e0a\u4e0b\u6587";
const TRACE_LABEL_CONTEXT_COMPACTION = "\u4e0a\u4e0b\u6587\u538b\u7f29";
const TRACE_TRIGGER_DETAIL_RE = /^([^；]+\u89e6\u53d1)；\s*(.*)$/;
const TRACE_BENIGN_UNSUPPORTED_RE = /\u534f\u8bae\u6216\u5206\u652f\u4e0d\u652f\u6301/;
const TRACE_ERROR_DETAIL_RE = /\u5931\u8d25|\u9519\u8bef|\u4e0d\u652f\u6301/;
const TRACE_ROOT_CLASS = "chat-screenshot-omit mb-2 w-full pr-4 sm:pr-6";

type ProcessTraceLabels = {
  fileBadges: {
    directRead: string;
    budget: string;
    retrieval: string;
    fullContext: string;
    skipped: string;
    file: string;
    descriptions: {
      directRead: string;
      budget: string;
      retrieval: string;
      fullContext: string;
      skipped: string;
      file: string;
    };
  };
  rag: {
    sourceFallback: (fileID: string) => string;
    chunksShort: (count: number, scorePercent: number) => string;
    retrievalSources: string;
    matchedContents: (count: number) => string;
    matchSummary: (count: number, sharePercent: number, scorePercent: number) => string;
    summary: (count: number) => string;
    completed: (fileCount: number, chunkCount: number) => string;
    incompleteWithFullText: string;
    incompleteNoFullText: string;
    emptyWithFullText: string;
    emptyNoFullText: string;
    lowScoreWithFullText: string;
    lowScoreNoFullText: string;
    skippedFallback: string;
  };
  fileContext: {
    includedSummary: (count: number) => string;
    includedDetail: (count: number) => string;
    skipped: (count: number) => string;
    ready: (counts: string) => string;
    separator: string;
  };
  tool: {
    status: {
      calling: string;
      completed: string;
      reused: string;
      failed: string;
    };
    names: {
      webSearch: string;
      codeInterpreter: string;
      imageGeneration: string;
      shell: string;
      generic: string;
      thinking: string;
    };
    detail: {
      request: string;
      response: string;
      error: string;
      expand: string;
      collapse: string;
      sourceFallback: (index: number) => string;
      generatedImageAlt: (index: number) => string;
      query: string;
      action: string;
      source: string;
      code: string;
      output: string;
      resultFile: string;
      prompt: string;
      command: string;
      latencySeparator: string;
    };
    nativeStatus: {
      webSearchActive: string;
      webSearchDone: string;
      webSearchFailed: string;
      codeActive: string;
      codeDone: string;
      codeFailed: string;
      imageActive: string;
      imageDone: string;
      imageFailed: string;
      shellActive: string;
      shellDone: string;
      shellFailed: string;
      genericActive: string;
      genericDone: string;
      genericFailed: string;
    };
    chain: {
      titleActive: string;
      titleDone: string;
      summaryCount: (count: number) => string;
      summaryFallback: string;
    };
    trace: {
      titleActive: string;
      titleDone: string;
      summaryDone: string;
      summaryActive: (count: number) => string;
      summaryCount: (count: number) => string;
      summaryFailed: (count: number, failed: number) => string;
    };
  };
  think: {
    titleActive: string;
    titleDone: string;
    subtitleActive: string;
    subtitleDone: string;
  };
  promptTrace: {
    modes: {
      stateful: string;
      fullRetry: string;
      full: string;
    };
    reasons: {
      missingStoredFingerprint: string;
      missingCurrentFingerprint: string;
      fingerprintMismatch: string;
      previousRejected: string;
    };
    sentSummary: (mode: string, sent: number, full: number, tokens: number) => string;
    savedHistory: (messages: number, tokens: number) => string;
    cacheableBlocks: (count: number) => string;
    historicalEvidence: (count: number) => string;
    dynamicSources: (count: number) => string;
    listSeparator: string;
    extraSummary: (items: string) => string;
    reasonLine: (reason: string) => string;
    preparedSummary: (tokens: number) => string;
    statefulSummary: (messages: number) => string;
  };
  stages: {
    contextPlanning: string;
    contentRetrieval: string;
    fileContext: string;
    contextCompaction: string;
    skillContext: string;
    requestResult: string;
    upstreamRequestTriggered: string;
  };
  process: {
    titleActive: string;
    titleDone: string;
  };
  compaction: {
    summary: (fromTurn: number, toTurn: number) => string;
    detail: string;
    range: (fromTurn: number, toTurn: number) => string;
    tokens: (sourceTokens: number, summaryTokens: number) => string;
  };
};

function useProcessTraceLabels(): ProcessTraceLabels {
  const t = useTranslations("chat.processTrace");

  return React.useMemo(
    () => ({
      fileBadges: {
        directRead: t("fileBadges.directRead"),
        budget: t("fileBadges.budget"),
        retrieval: t("fileBadges.retrieval"),
        fullContext: t("fileBadges.fullContext"),
        skipped: t("fileBadges.skipped"),
        file: t("fileBadges.file"),
        descriptions: {
          directRead: t("fileBadges.descriptions.directRead"),
          budget: t("fileBadges.descriptions.budget"),
          retrieval: t("fileBadges.descriptions.retrieval"),
          fullContext: t("fileBadges.descriptions.fullContext"),
          skipped: t("fileBadges.descriptions.skipped"),
          file: t("fileBadges.descriptions.file"),
        },
      },
      rag: {
        sourceFallback: (fileID: string) => t("rag.sourceFallback", { fileID }),
        chunksShort: (count: number, scorePercent: number) => t("rag.chunksShort", { count, scorePercent }),
        retrievalSources: t("rag.retrievalSources"),
        matchedContents: (count: number) => t("rag.matchedContents", { count }),
        matchSummary: (count: number, sharePercent: number, scorePercent: number) =>
          t("rag.matchSummary", { count, sharePercent, scorePercent }),
        summary: (count: number) => t("rag.summary", { count }),
        completed: (fileCount: number, chunkCount: number) => t("rag.completed", { fileCount, chunkCount }),
        incompleteWithFullText: t("rag.incompleteWithFullText"),
        incompleteNoFullText: t("rag.incompleteNoFullText"),
        emptyWithFullText: t("rag.emptyWithFullText"),
        emptyNoFullText: t("rag.emptyNoFullText"),
        lowScoreWithFullText: t("rag.lowScoreWithFullText"),
        lowScoreNoFullText: t("rag.lowScoreNoFullText"),
        skippedFallback: t("rag.skippedFallback"),
      },
      fileContext: {
        includedSummary: (count: number) => t("fileContext.includedSummary", { count }),
        includedDetail: (count: number) => t("fileContext.includedDetail", { count }),
        skipped: (count: number) => t("fileContext.skipped", { count }),
        ready: (counts: string) => t("fileContext.ready", { counts }),
        separator: t("fileContext.separator"),
      },
      tool: {
        status: {
          calling: t("tool.status.calling"),
          completed: t("tool.status.completed"),
          reused: t("tool.status.reused"),
          failed: t("tool.status.failed"),
        },
        names: {
          webSearch: t("tool.names.webSearch"),
          codeInterpreter: t("tool.names.codeInterpreter"),
          imageGeneration: t("tool.names.imageGeneration"),
          shell: t("tool.names.shell"),
          generic: t("tool.names.generic"),
          thinking: t("tool.names.thinking"),
        },
        detail: {
          request: t("tool.detail.request"),
          response: t("tool.detail.response"),
          error: t("tool.detail.error"),
          expand: t("tool.detail.expand"),
          collapse: t("tool.detail.collapse"),
          sourceFallback: (index: number) => t("tool.detail.sourceFallback", { index }),
          generatedImageAlt: (index: number) => t("tool.detail.generatedImageAlt", { index }),
          query: t("tool.detail.query"),
          action: t("tool.detail.action"),
          source: t("tool.detail.source"),
          code: t("tool.detail.code"),
          output: t("tool.detail.output"),
          resultFile: t("tool.detail.resultFile"),
          prompt: t("tool.detail.prompt"),
          command: t("tool.detail.command"),
          latencySeparator: t("tool.detail.latencySeparator"),
        },
        nativeStatus: {
          webSearchActive: t("tool.nativeStatus.webSearchActive"),
          webSearchDone: t("tool.nativeStatus.webSearchDone"),
          webSearchFailed: t("tool.nativeStatus.webSearchFailed"),
          codeActive: t("tool.nativeStatus.codeActive"),
          codeDone: t("tool.nativeStatus.codeDone"),
          codeFailed: t("tool.nativeStatus.codeFailed"),
          imageActive: t("tool.nativeStatus.imageActive"),
          imageDone: t("tool.nativeStatus.imageDone"),
          imageFailed: t("tool.nativeStatus.imageFailed"),
          shellActive: t("tool.nativeStatus.shellActive"),
          shellDone: t("tool.nativeStatus.shellDone"),
          shellFailed: t("tool.nativeStatus.shellFailed"),
          genericActive: t("tool.nativeStatus.genericActive"),
          genericDone: t("tool.nativeStatus.genericDone"),
          genericFailed: t("tool.nativeStatus.genericFailed"),
        },
        chain: {
          titleActive: t("tool.chain.titleActive"),
          titleDone: t("tool.chain.titleDone"),
          summaryCount: (count: number) => t("tool.chain.summaryCount", { count }),
          summaryFallback: t("tool.chain.summaryFallback"),
        },
        trace: {
          titleActive: t("tool.trace.titleActive"),
          titleDone: t("tool.trace.titleDone"),
          summaryDone: t("tool.trace.summaryDone"),
          summaryActive: (count: number) => t("tool.trace.summaryActive", { count }),
          summaryCount: (count: number) => t("tool.trace.summaryCount", { count }),
          summaryFailed: (count: number, failed: number) => t("tool.trace.summaryFailed", { count, failed }),
        },
      },
      think: {
        titleActive: t("think.titleActive"),
        titleDone: t("think.titleDone"),
        subtitleActive: t("think.subtitleActive"),
        subtitleDone: t("think.subtitleDone"),
      },
      promptTrace: {
        modes: {
          stateful: t("promptTrace.modes.stateful"),
          fullRetry: t("promptTrace.modes.fullRetry"),
          full: t("promptTrace.modes.full"),
        },
        reasons: {
          missingStoredFingerprint: t("promptTrace.reasons.missingStoredFingerprint"),
          missingCurrentFingerprint: t("promptTrace.reasons.missingCurrentFingerprint"),
          fingerprintMismatch: t("promptTrace.reasons.fingerprintMismatch"),
          previousRejected: t("promptTrace.reasons.previousRejected"),
        },
        sentSummary: (mode: string, sent: number, full: number, tokens: number) =>
          t("promptTrace.sentSummary", { mode, sent, full, tokens }),
        savedHistory: (messages: number, tokens: number) => t("promptTrace.savedHistory", { messages, tokens }),
        cacheableBlocks: (count: number) => t("promptTrace.cacheableBlocks", { count }),
        historicalEvidence: (count: number) => t("promptTrace.historicalEvidence", { count }),
        dynamicSources: (count: number) => t("promptTrace.dynamicSources", { count }),
        listSeparator: t("promptTrace.listSeparator"),
        extraSummary: (items: string) => t("promptTrace.extraSummary", { items }),
        reasonLine: (reason: string) => t("promptTrace.reasonLine", { reason }),
        preparedSummary: (tokens: number) => t("promptTrace.preparedSummary", { tokens }),
        statefulSummary: (messages: number) => t("promptTrace.statefulSummary", { messages }),
      },
      stages: {
        contextPlanning: t("stages.contextPlanning"),
        contentRetrieval: t("stages.contentRetrieval"),
        fileContext: t("stages.fileContext"),
        contextCompaction: t("stages.contextCompaction"),
        skillContext: t("stages.skillContext"),
        requestResult: t("stages.requestResult"),
        upstreamRequestTriggered: t("stages.upstreamRequestTriggered"),
      },
      process: {
        titleActive: t("process.titleActive"),
        titleDone: t("process.titleDone"),
      },
      compaction: {
        summary: (fromTurn: number, toTurn: number) => t("compaction.summary", { fromTurn, toTurn }),
        detail: t("compaction.detail"),
        range: (fromTurn: number, toTurn: number) => t("compaction.range", { fromTurn, toTurn }),
        tokens: (sourceTokens: number, summaryTokens: number) => t("compaction.tokens", { sourceTokens, summaryTokens }),
      },
    }),
    [t],
  );
}

function parseRAGCitations(payloadJson: string | undefined): RAGCitation[] {
  if (!payloadJson) return [];
  try {
    const parsed = JSON.parse(payloadJson) as { citations?: RAGCitation[] };
    return Array.isArray(parsed.citations) ? parsed.citations : [];
  } catch {
    return [];
  }
}

type FileContextBadge = {
  fileID?: string;
  name: string;
  label: string;
  description?: string;
  tab: "extract" | "preview";
};

type FileContextCounts = {
  included: number;
  skipped: number;
};

type RAGTraceCounts = {
  fileCount: number;
  chunkCount: number;
};

type CompactionTracePayload = {
  fromTurn: number;
  toTurn: number;
  sourceTokens: number;
  summaryTokens: number;
};

function readStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value.map((item) => (typeof item === "string" ? item.trim() : "")).filter(Boolean);
}

function readArrayCount(value: unknown): number {
  return Array.isArray(value) ? value.length : 0;
}

function parseTracePayload(payloadJson: string | undefined): Record<string, unknown> | null {
  if (!payloadJson) return null;
  try {
    const parsed = JSON.parse(payloadJson) as unknown;
    return isRecord(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

function parseFileContextCounts(payloadJson: string | undefined): FileContextCounts | null {
  const parsed = parseTracePayload(payloadJson);
  if (!parsed) return null;
  const groups = isRecord(parsed.file_group_refs)
    ? parsed.file_group_refs
    : isRecord(parsed.file_groups)
      ? parsed.file_groups
      : null;
  if (!groups) {
    const fileRefsCount = readArrayCount(parsed.file_refs);
    const fileNamesCount = readArrayCount(parsed.file_names);
    const included = fileRefsCount || fileNamesCount;
    return included > 0 ? { included, skipped: 0 } : null;
  }
  const skipped = readArrayCount(groups.skipped);
  const included =
    readArrayCount(groups.direct_images) +
    readArrayCount(groups.adaptive) +
    readArrayCount(groups.retrieval) +
    readArrayCount(groups.full_context);
  if (included <= 0 && skipped <= 0) return null;
  return { included, skipped };
}

function parseRAGTraceCounts(payloadJson: string | undefined): RAGTraceCounts | null {
  const parsed = parseTracePayload(payloadJson);
  if (!parsed) return null;
  const fileCount = readArrayCount(parsed.file_names);
  const chunkCount = typeof parsed.hit_chunk_count === "number" && Number.isFinite(parsed.hit_chunk_count)
    ? parsed.hit_chunk_count
    : readArrayCount(parsed.citations);
  if (fileCount <= 0 && chunkCount <= 0) return null;
  return { fileCount, chunkCount };
}

function parseCompactionTracePayload(payloadJson: string | undefined): CompactionTracePayload | null {
  const parsed = parseTracePayload(payloadJson);
  if (!parsed) return null;
  const fromTurn = readNumber(parsed.from_turn);
  const toTurn = readNumber(parsed.to_turn);
  const sourceTokens = readNumber(parsed.source_tokens);
  const summaryTokens = readNumber(parsed.summary_tokens);
  if (fromTurn === null || toTurn === null || sourceTokens === null || summaryTokens === null) return null;
  return { fromTurn, toTurn, sourceTokens, summaryTokens };
}

function formatFileContextCounts(counts: FileContextCounts, labels: ProcessTraceLabels, includedDetail: boolean): string {
  const parts = [];
  if (counts.included > 0 || counts.skipped === 0) {
    parts.push(includedDetail ? labels.fileContext.includedDetail(counts.included) : labels.fileContext.includedSummary(counts.included));
  }
  if (counts.skipped > 0) {
    parts.push(labels.fileContext.skipped(counts.skipped));
  }
  return parts.join(labels.fileContext.separator);
}

function readFileContextBadges(value: unknown, label: string, description: string, tab: "extract" | "preview"): FileContextBadge[] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((item) => {
    if (typeof item === "string") {
      const name = item.trim();
      return name ? [{ name, label, description, tab }] : [];
    }
    if (!isRecord(item)) return [];
    const fileID = firstStringFromRecord(item, ["file_id", "fileID", "id"]);
    const name = firstStringFromRecord(item, ["file_name", "fileName", "name", "title"]) || fileID;
    return name ? [{ fileID, name, label, description, tab }] : [];
  });
}

function parseFileContextBadges(payloadJson: string | undefined, labels: ProcessTraceLabels): FileContextBadge[] {
  if (!payloadJson) return [];
  try {
    const parsed = JSON.parse(payloadJson) as {
      file_names?: string[];
      file_refs?: unknown[];
      file_groups?: Record<string, unknown>;
      file_group_refs?: Record<string, unknown>;
    };
    const groups = parsed.file_group_refs ?? parsed.file_groups ?? {};
    const badges = [
      ...readFileContextBadges(groups.direct_images, labels.fileBadges.directRead, labels.fileBadges.descriptions.directRead, "preview"),
      ...readFileContextBadges(groups.adaptive, labels.fileBadges.budget, labels.fileBadges.descriptions.budget, "extract"),
      ...readFileContextBadges(groups.retrieval, labels.fileBadges.retrieval, labels.fileBadges.descriptions.retrieval, "extract"),
      ...readFileContextBadges(groups.full_context, labels.fileBadges.fullContext, labels.fileBadges.descriptions.fullContext, "extract"),
      ...readFileContextBadges(groups.skipped, labels.fileBadges.skipped, labels.fileBadges.descriptions.skipped, "extract"),
    ];
    if (badges.length > 0) return badges;
    const refs = readFileContextBadges(parsed.file_refs, labels.fileBadges.file, labels.fileBadges.descriptions.file, "extract");
    if (refs.length > 0) return refs;
    return readStringArray(parsed.file_names).map((name) => ({
      name,
      label: labels.fileBadges.file,
      description: labels.fileBadges.descriptions.file,
      tab: "extract",
    }));
  } catch {
    return [];
  }
}

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
    if (item.preview?.trim() && !current.previews.includes(item.preview.trim())) {
      current.previews.push(item.preview.trim());
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

function RAGCitationList({
  citations,
  embedded = false,
  labels,
}: {
  citations: RAGCitation[];
  embedded?: boolean;
  labels: ProcessTraceLabels;
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
    <div className="border-t border-border/30 pt-2">
      <div className="mb-2 flex items-center justify-between gap-3">
        <span className="text-[11px] font-medium text-muted-foreground/76">{labels.rag.retrievalSources}</span>
        <span className="text-[10px] text-muted-foreground/56">{labels.rag.matchedContents(citations.length)}</span>
      </div>

      <div className="space-y-2">
        {grouped.map((item) => (
          <div key={item.fileID} className="rounded-lg border border-border/35 bg-background/40 px-2.5 py-2">
            <div className="flex items-center justify-between gap-3">
              <div className="min-w-0">
                <div className="truncate text-[12px] font-medium text-foreground/88">{item.fileName}</div>
                <div className="mt-0.5 text-[11px] text-muted-foreground/62">
                  {labels.rag.matchSummary(item.chunkCount, item.sharePercent, Math.round(item.maxScore * 100))}
                </div>
              </div>
              <div className="w-20 shrink-0">
                <div className="h-1.5 overflow-hidden rounded-full bg-muted/60">
                  <div className="h-full rounded-full bg-foreground/70" style={{ width: `${item.sharePercent}%` }} />
                </div>
              </div>
            </div>
            {item.previews.length > 0 ? (
              <div className="mt-2 flex flex-wrap gap-1">
                {item.previews.slice(0, 3).map((preview, index) => (
                  <span
                    key={`${item.fileID}-${index}`}
                    className="max-w-full truncate rounded-full border border-border/35 bg-muted/18 px-1.5 py-0 text-[11px] leading-5 text-muted-foreground/76"
                    title={preview}
                  >
                    {preview}
                  </span>
                ))}
              </div>
            ) : null}
          </div>
        ))}
      </div>
    </div>
  );
}

type TraceStage = {
  label: string;
  kind?: string;
  status?: string;
  trigger: string;
  detail: string;
  details: string[];
  structured?: boolean;
};

type ToolTraceCall = {
  tool_call_id?: string;
  id?: string;
  call_id?: string;
  name: string;
  type?: string;
  status: string;
  latency_ms?: number;
  error?: string;
  input?: string;
  output?: string;
  output_text?: string;
  output_preview?: string;
};

type NativeToolKind = "web_search" | "code_interpreter" | "image_generation" | "shell" | "generic";

const TOOL_DETAIL_COLLAPSED_LINES = 8;
const TOOL_DETAIL_LINE_HEIGHT_REM = 1.25;

function parseToolTraceCalls(payloadJson: string | undefined): ToolTraceCall[] {
  if (!payloadJson) return [];
  try {
    const parsed = JSON.parse(payloadJson) as { tool_calls?: ToolTraceCall[] };
    return Array.isArray(parsed.tool_calls) ? parsed.tool_calls : [];
  } catch {
    return [];
  }
}

function shouldCollapseToolDetail(value: string): boolean {
  const text = value.trim();
  if (!text) return false;
  return text.split(/\r?\n/).length > TOOL_DETAIL_COLLAPSED_LINES || text.length > 420;
}

function formatToolPayload(value: string | undefined): string {
  const text = value?.trim();
  if (!text) return "";
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return text;
  }
}

function parseToolPayload(value: string | undefined): unknown {
  const text = value?.trim();
  if (!text) return null;
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return text;
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function readString(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function readNumber(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function firstStringFromRecord(record: Record<string, unknown>, keys: string[]): string {
  for (const key of keys) {
    const value = readString(record[key]);
    if (value) return value;
  }
  return "";
}

function collectToolStrings(value: unknown, keys: string[], result: string[] = []): string[] {
  if (Array.isArray(value)) {
    value.forEach((item) => collectToolStrings(item, keys, result));
    return result;
  }
  if (!isRecord(value)) return result;
  for (const key of keys) {
    const text = readString(value[key]);
    if (text) result.push(text);
  }
  Object.values(value).forEach((item) => collectToolStrings(item, keys, result));
  return Array.from(new Set(result));
}

function normalizeImageSource(value: string): string {
  const text = value.trim();
  if (!text) return "";
  if (/^(https?:|data:image\/|blob:)/i.test(text)) return text;
  if (/^[A-Za-z0-9+/=\s]+$/.test(text) && text.replace(/\s/g, "").length > 80) {
    return `data:image/png;base64,${text.replace(/\s/g, "")}`;
  }
  return "";
}

function collectToolImageSources(value: unknown, result: string[] = []): string[] {
  if (typeof value === "string") {
    const source = normalizeImageSource(value);
    if (source) result.push(source);
    return Array.from(new Set(result));
  }
  if (Array.isArray(value)) {
    value.forEach((item) => collectToolImageSources(item, result));
    return Array.from(new Set(result));
  }
  if (!isRecord(value)) return Array.from(new Set(result));
  for (const key of ["url", "uri", "image_url", "b64_json", "base64", "partial_image_b64", "result"]) {
    const source = normalizeImageSource(readString(value[key]));
    if (source) result.push(source);
  }
  Object.values(value).forEach((item) => collectToolImageSources(item, result));
  return Array.from(new Set(result));
}

function normalizeToolName(value: string | undefined): string {
  return value?.trim().replace(/_call_output$/, "").replace(/_call$/, "") || "";
}

function resolveNativeToolKind(call: ToolTraceCall): NativeToolKind {
  const name = normalizeToolName(call.name);
  const type = normalizeToolName(call.type);
  const value = `${name} ${type}`;
  if (value.includes("web_search")) return "web_search";
  if (value.includes("code_interpreter") || value.includes("code_execution")) return "code_interpreter";
  if (value.includes("image_generation")) return "image_generation";
  if (value.includes("shell")) return "shell";
  return "generic";
}

function toolStatusLabel(status: string | undefined, labels: ProcessTraceLabels): string {
  switch (status?.trim()) {
    case "requested":
    case "streaming":
    case "queued":
    case "in_progress":
    case "searching":
      return labels.tool.status.calling;
    case "success":
    case "completed":
      return labels.tool.status.completed;
    case "reused":
      return labels.tool.status.reused;
    case "error":
    case "failed":
      return labels.tool.status.failed;
    default:
      return status?.trim() || "";
  }
}

function toolTraceCallLabel(call: ToolTraceCall, labels: ProcessTraceLabels): string {
  switch (resolveNativeToolKind(call)) {
    case "web_search":
      return labels.tool.names.webSearch;
    case "code_interpreter":
      return labels.tool.names.codeInterpreter;
    case "image_generation":
      return labels.tool.names.imageGeneration;
    case "shell":
      return labels.tool.names.shell;
    default:
      return call.name?.trim() || call.type?.trim() || labels.tool.names.generic;
  }
}

function toolTraceCallDetail(call: ToolTraceCall, labels: ProcessTraceLabels): { detail: string; failed: boolean } {
  const status = call.status?.trim();
  const failed = status === "error" || status === "failed";
  const input = formatToolPayload(call.input);
  const output = failed
    ? formatToolPayload(call.error)
    : formatToolPayload(call.output) || formatToolPayload(call.output_text) || formatToolPayload(call.output_preview);
  const parts = [toolStatusLabel(status, labels)].filter(Boolean);

  if (input) {
    parts.push(`${labels.tool.detail.request}\n${input}`);
  }
  if (output) {
    parts.push(`${failed ? labels.tool.detail.error : labels.tool.detail.response}\n${output}`);
  }

  return { detail: parts.join("\n"), failed };
}

function nativeToolStatusText(call: ToolTraceCall, labels: ProcessTraceLabels): string {
  const status = call.status?.trim();
  const done = status === "success" || status === "completed" || status === "reused";
  const failed = status === "error" || status === "failed";
  switch (resolveNativeToolKind(call)) {
    case "web_search":
      return failed ? labels.tool.nativeStatus.webSearchFailed : done ? labels.tool.nativeStatus.webSearchDone : labels.tool.nativeStatus.webSearchActive;
    case "code_interpreter":
      return failed ? labels.tool.nativeStatus.codeFailed : done ? labels.tool.nativeStatus.codeDone : labels.tool.nativeStatus.codeActive;
    case "image_generation":
      return failed ? labels.tool.nativeStatus.imageFailed : done ? labels.tool.nativeStatus.imageDone : labels.tool.nativeStatus.imageActive;
    case "shell":
      return failed ? labels.tool.nativeStatus.shellFailed : done ? labels.tool.nativeStatus.shellDone : labels.tool.nativeStatus.shellActive;
    default:
      return failed ? labels.tool.nativeStatus.genericFailed : done ? labels.tool.nativeStatus.genericDone : labels.tool.nativeStatus.genericActive;
  }
}

function localizeToolTraceSummary(block: ChatTraceBlock, calls: ToolTraceCall[], labels: ProcessTraceLabels): string {
  if (calls.length > 0) {
    const failed = calls.filter((call) => call.status === "error" || call.status === "failed").length;
    const active = calls.filter((call) => ["requested", "streaming", "queued", "in_progress", "searching"].includes(call.status?.trim())).length;
    if (failed > 0) {
      return labels.tool.trace.summaryFailed(calls.length, failed);
    }
    if (active > 0) {
      return labels.tool.trace.summaryActive(calls.length);
    }
    return labels.tool.trace.summaryCount(calls.length);
  }
  return block.summary?.trim() || labels.tool.trace.summaryDone;
}

function ToolDetailExpandButton({
  open,
  floating,
  onClick,
  labels,
}: {
  open: boolean;
  floating?: boolean;
  onClick: () => void;
  labels: ProcessTraceLabels;
}) {
  const button = (
    <button
      type="button"
      className={cn(
        "inline-flex items-center gap-0.5 px-0.5 py-0.5",
        "text-[11px] font-medium text-muted-foreground/70 transition-colors hover:text-foreground",
      )}
      onClick={onClick}
    >
      <span>{open ? labels.tool.detail.collapse : labels.tool.detail.expand}</span>
      <ChevronDown className={cn("size-3 transition-transform", open && "rotate-180")} />
    </button>
  );

  if (floating) {
    return <div className="absolute bottom-0 right-0 z-10">{button}</div>;
  }

  return <div className="mt-1 flex justify-end">{button}</div>;
}

function ToolMiniLabel({ children }: { children: React.ReactNode }) {
  return <div className="mb-1 text-[11px] font-medium leading-4 text-muted-foreground/58">{children}</div>;
}

function ToolPre({ children, failed }: { children: string; failed?: boolean }) {
  if (!children.trim()) return null;
  return (
    <pre
      className={cn(
        "max-h-56 overflow-auto rounded-md border border-border/35 bg-muted/25 px-2.5 py-2 font-mono text-[11px] leading-5",
        "whitespace-pre-wrap break-words text-muted-foreground/88",
        failed && "border-destructive/25 bg-destructive/5 text-destructive/85",
      )}
    >
      {children}
    </pre>
  );
}

function safeURLHostname(url: string): string {
  try {
    return new URL(url).hostname;
  } catch {
    return url;
  }
}

function ToolSourceLinks({ urls, labels }: { urls: string[]; labels: ProcessTraceLabels }) {
  const unique = Array.from(new Set(urls.map((item) => item.trim()).filter(Boolean))).slice(0, 8);
  if (unique.length === 0) return null;
  return (
    <div className="flex flex-wrap gap-1.5">
      {unique.map((url, index) => (
        <a
          key={`${url}-${index}`}
          href={url}
          target="_blank"
          rel="noreferrer"
          className="max-w-[220px] truncate rounded-full border border-border/40 bg-background/55 px-2 py-0.5 text-[11px] font-medium text-muted-foreground/78 transition-colors hover:border-border hover:text-foreground"
          title={url}
        >
          {safeURLHostname(url) || labels.tool.detail.sourceFallback(index + 1)}
        </a>
      ))}
    </div>
  );
}

function ToolPreviewImage({ src, alt }: { src: string; alt: string }) {
  // eslint-disable-next-line @next/next/no-img-element -- Tool image URLs are arbitrary external artifacts, not app-managed image assets.
  return <img src={src} alt={alt} loading="lazy" decoding="async" className="aspect-square w-full object-cover transition-opacity group-hover/image:opacity-90" />;
}

function ToolImageGrid({ urls, labels }: { urls: string[]; labels: ProcessTraceLabels }) {
  const unique = Array.from(new Set(urls.map((item) => item.trim()).filter(Boolean))).slice(0, 4);
  if (unique.length === 0) return null;
  return (
    <div className="grid grid-cols-2 gap-2 sm:grid-cols-[repeat(auto-fit,minmax(120px,180px))]">
      {unique.map((url, index) => (
        <a
          key={`${url}-${index}`}
          href={url}
          target="_blank"
          rel="noreferrer"
          className="group/image relative block aspect-square overflow-hidden rounded-md border border-border/40 bg-muted/20"
          title={url}
        >
          <ToolPreviewImage src={url} alt={labels.tool.detail.generatedImageAlt(index + 1)} />
        </a>
      ))}
    </div>
  );
}

function ToolDetailText({
  failed,
  open,
  canExpand,
  children,
  onToggle,
  labels,
}: {
  failed?: boolean;
  open: boolean;
  canExpand: boolean;
  children: React.ReactNode;
  onToggle: () => void;
  labels: ProcessTraceLabels;
}) {
  const contentRef = React.useRef<HTMLDivElement>(null);
  const [contentHeight, setContentHeight] = React.useState(0);

  React.useLayoutEffect(() => {
    if (!canExpand || !contentRef.current) {
      return;
    }
    const element = contentRef.current;
    const updateHeight = () => setContentHeight(element.scrollHeight);
    updateHeight();
    const resizeObserver = new ResizeObserver(updateHeight);
    resizeObserver.observe(element);
    return () => resizeObserver.disconnect();
  }, [canExpand, children]);

  const maxHeight = canExpand
    ? open
      ? `${contentHeight}px`
      : `${TOOL_DETAIL_COLLAPSED_LINES * TOOL_DETAIL_LINE_HEIGHT_REM}rem`
    : undefined;

  return (
    <>
      <div className="relative">
        <div
          ref={contentRef}
          className={cn(
            "overflow-hidden whitespace-pre-wrap break-words text-muted-foreground/84 transition-[max-height] duration-200 ease-out",
            failed && "text-destructive/80",
          )}
          style={maxHeight ? { maxHeight } : undefined}
        >
          {children}
        </div>
        {canExpand && !open ? (
          <>
            <div className="pointer-events-none absolute inset-x-0 bottom-0 h-8 bg-gradient-to-b from-transparent via-background/88 to-background" />
            <ToolDetailExpandButton open={open} floating labels={labels} onClick={onToggle} />
          </>
        ) : null}
      </div>
      {canExpand && open ? <ToolDetailExpandButton open={open} labels={labels} onClick={onToggle} /> : null}
    </>
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

function readTraceStagePayloads(payloadJson: string | undefined): Record<string, unknown>[] {
  const parsed = parseTracePayload(payloadJson);
  if (!parsed) return [];
  const stages = parsed.trace_stages;
  if (Array.isArray(stages)) {
    return stages.filter(isRecord);
  }
  return isRecord(parsed.trace_stage) ? [parsed.trace_stage] : [];
}

function traceStageKind(stage: TraceStage): string {
  if (stage.kind) return stage.kind;
  switch (stage.label) {
    case TRACE_LABEL_CONTEXT_PLANNING:
      return TRACE_KIND_CONTEXT_PLANNING;
    case TRACE_LABEL_RAG:
      return TRACE_KIND_RAG;
    case TRACE_LABEL_FILE_CONTEXT:
      return TRACE_KIND_FILE_CONTEXT;
    case TRACE_LABEL_CONTEXT_COMPACTION:
      return TRACE_KIND_CONTEXT_COMPACTION;
    default:
      return stage.label;
  }
}

function isContextPlanningTraceStage(stage: TraceStage): boolean {
  return traceStageKind(stage) === TRACE_KIND_CONTEXT_PLANNING;
}

function isRAGTraceStage(stage: TraceStage): boolean {
  return traceStageKind(stage) === TRACE_KIND_RAG;
}

function isFileContextTraceStage(stage: TraceStage): boolean {
  return traceStageKind(stage) === TRACE_KIND_FILE_CONTEXT;
}

function isTraceStageError(stage: TraceStage): boolean {
  if (stage.structured) {
    return ["error", "failed"].includes(stage.status?.trim() ?? "");
  }
  return TRACE_ERROR_DETAIL_RE.test(stage.detail);
}

function structuredFileContextDetail(stage: Record<string, unknown>, labels: ProcessTraceLabels): string {
  const included = readNumber(stage.included_count) ?? 0;
  const skipped = readNumber(stage.skipped_count) ?? 0;
  return labels.fileContext.ready(formatFileContextCounts({ included, skipped }, labels, true));
}

function structuredRAGDetail(stage: Record<string, unknown>, labels: ProcessTraceLabels): string {
  const status = readString(stage.status);
  const fallback = readString(stage.fallback);
  const fileCount = readNumber(stage.file_count) ?? 0;
  const chunkCount = readNumber(stage.chunk_count) ?? 0;
  const hasFullText = fallback === "full_text";

  switch (status) {
    case "completed":
      return labels.rag.completed(fileCount, chunkCount);
    case "empty":
      return hasFullText ? labels.rag.emptyWithFullText : labels.rag.emptyNoFullText;
    case "low_score":
      return hasFullText ? labels.rag.lowScoreWithFullText : labels.rag.lowScoreNoFullText;
    case "skipped":
      return labels.rag.skippedFallback;
    default:
      return hasFullText ? labels.rag.incompleteWithFullText : labels.rag.incompleteNoFullText;
  }
}

function structuredCompactionDetails(stage: Record<string, unknown>, labels: ProcessTraceLabels): string[] {
  const fromTurn = readNumber(stage.from_turn) ?? 0;
  const toTurn = readNumber(stage.to_turn) ?? 0;
  const sourceTokens = readNumber(stage.source_tokens) ?? 0;
  const summaryTokens = readNumber(stage.summary_tokens) ?? 0;
  return [
    labels.compaction.detail,
    labels.compaction.range(fromTurn, toTurn),
    labels.compaction.tokens(sourceTokens, summaryTokens),
  ];
}

function structuredTraceStageDetails(stage: Record<string, unknown>, labels: ProcessTraceLabels): string[] {
  switch (readString(stage.kind)) {
    case TRACE_KIND_FILE_CONTEXT:
      return [structuredFileContextDetail(stage, labels)];
    case TRACE_KIND_RAG:
      return [structuredRAGDetail(stage, labels)];
    case TRACE_KIND_CONTEXT_COMPACTION:
      return structuredCompactionDetails(stage, labels);
    default:
      return [];
  }
}

function parseStructuredTraceStages(payloadJson: string | undefined, labels: ProcessTraceLabels): TraceStage[] {
  return readTraceStagePayloads(payloadJson).flatMap((payload) => {
    const kind = readString(payload.kind);
    if (!kind) return [];
    const details = structuredTraceStageDetails(payload, labels);
    if (details.length === 0) return [];
    return [{
      label: kind,
      kind,
      status: readString(payload.status),
      trigger: "",
      detail: details.join("\n"),
      details,
      structured: true,
    }];
  }).filter((stage, index, stages) => {
    const previous = stages[index - 1];
    return !previous || previous.kind !== stage.kind || previous.status !== stage.status || previous.detail !== stage.detail;
  });
}

function structuredProcessSummaryFromPayload(payloadJson: string | undefined, labels: ProcessTraceLabels): string {
  const stages = readTraceStagePayloads(payloadJson);
  const last = [...stages].reverse().find((stage) => readString(stage.kind));
  if (!last) return "";
  const kind = readString(last.kind);
  if (kind === TRACE_KIND_FILE_CONTEXT) {
    const included = readNumber(last.included_count) ?? 0;
    const skipped = readNumber(last.skipped_count) ?? 0;
    return formatFileContextCounts({ included, skipped }, labels, false);
  }
  if (kind === TRACE_KIND_RAG) {
    const status = readString(last.status);
    const fallback = readString(last.fallback);
    const hasFullText = fallback === "full_text";
    if (status === "completed") {
      return labels.rag.summary(readNumber(last.chunk_count) ?? 0);
    }
    if (status === "empty") {
      return hasFullText ? labels.rag.emptyWithFullText : labels.rag.emptyNoFullText;
    }
    if (status === "low_score") {
      return hasFullText ? labels.rag.lowScoreWithFullText : labels.rag.lowScoreNoFullText;
    }
    if (status === "skipped") {
      return labels.rag.skippedFallback;
    }
    return hasFullText ? labels.rag.incompleteWithFullText : labels.rag.incompleteNoFullText;
  }
  if (kind === TRACE_KIND_CONTEXT_COMPACTION) {
    const fromTurn = readNumber(last.from_turn);
    const toTurn = readNumber(last.to_turn);
    if (fromTurn !== null && toTurn !== null) {
      return labels.compaction.summary(fromTurn, toTurn);
    }
  }
  return "";
}

function parseTraceStages(content: unknown): TraceStage[] {
  if (typeof content !== "string") return [];
  const stages: TraceStage[] = [];

  for (const rawLine of content.split(/\n+/)) {
    const line = rawLine.trim();
    if (!line) continue;

    const match = line.match(/^\*\*([^*]+)\*\*(?:[：:]\s*)?(.*)$/);
    if (match) {
      const label = match[1].trim();
      const detail = match[2].trim();
      const triggerMatch = detail.match(TRACE_TRIGGER_DETAIL_RE);
      const trigger = triggerMatch?.[1]?.trim() ?? "";
      const firstDetail = (trigger ? triggerMatch?.[2] : detail)?.trim() ?? "";
      stages.push({
        label,
        detail,
        trigger,
        details: firstDetail ? [firstDetail] : [],
      });
      continue;
    }

    const current = stages[stages.length - 1];
    if (!current) continue;
    current.details.push(line);
    current.detail = [current.detail, line].filter(Boolean).join("\n");
  }

  return stages;
}

function isUpstreamFailureTraceStage(stage: TraceStage): boolean {
  return stage.label === TRACE_LABEL_UPSTREAM_RESULT && stage.trigger === TRACE_TRIGGER_UPSTREAM_REQUEST;
}

function isBenignPromptTraceDisabledDetail(stage: TraceStage, detail: string): boolean {
  return isContextPlanningTraceStage(stage) && TRACE_BENIGN_UNSUPPORTED_RE.test(detail);
}

function sanitizeProcessTraceStage(stage: TraceStage): TraceStage | null {
  if (!isContextPlanningTraceStage(stage)) {
    return stage;
  }
  const details = stage.details.filter((detail) => !isBenignPromptTraceDisabledDetail(stage, detail));
  const detail = details.join("\n").trim();
  if (!detail) {
    return null;
  }
  return { ...stage, detail, details };
}

function filterProcessTraceStages(stages: TraceStage[]): TraceStage[] {
  return stages.flatMap((stage) => {
    if (isUpstreamFailureTraceStage(stage)) {
      return [];
    }
    const sanitized = sanitizeProcessTraceStage(stage);
    return sanitized ? [sanitized] : [];
  });
}

function normalizeTraceListItem(text: string): string {
  return text.replace(/^[-*]\s+/, "").trim();
}

function localizeTraceDetail(stage: TraceStage, detail: string, payloadJson: string | undefined, labels: ProcessTraceLabels): string {
  const normalized = normalizeTraceListItem(detail);
  if (isFileContextTraceStage(stage) && /^文件已就绪/.test(normalized)) {
    const counts = parseFileContextCounts(payloadJson);
    if (counts) {
      return labels.fileContext.ready(formatFileContextCounts(counts, labels, true));
    }
  }
  if (isRAGTraceStage(stage) && /^检索已完成/.test(normalized)) {
    const counts = parseRAGTraceCounts(payloadJson);
    if (counts) {
      return labels.rag.completed(counts.fileCount, counts.chunkCount);
    }
  }
  if (isRAGTraceStage(stage) && /^文件已检索/.test(normalized)) {
    const switchedToFullText = normalized.includes("已改用全文");
    const noFullText = normalized.includes("没有可用全文");
    if (normalized.includes("检索未完成")) {
      return noFullText ? labels.rag.incompleteNoFullText : switchedToFullText ? labels.rag.incompleteWithFullText : normalized;
    }
    if (normalized.includes("检索结果低于相似度阈值")) {
      return noFullText ? labels.rag.lowScoreNoFullText : switchedToFullText ? labels.rag.lowScoreWithFullText : normalized;
    }
    if (normalized.includes("未检索到相关片段")) {
      return noFullText ? labels.rag.emptyNoFullText : switchedToFullText ? labels.rag.emptyWithFullText : normalized;
    }
  }
  if (isRAGTraceStage(stage) && normalized.includes("文件超出预算或没有可用提取文本")) {
    return labels.rag.skippedFallback;
  }
  return normalized;
}

function localizeTraceDetailItems(stage: TraceStage, detailItems: string[], payloadJson: string | undefined, labels: ProcessTraceLabels): string[] {
  if (stage.structured) {
    return detailItems;
  }
  if (traceStageKind(stage) === TRACE_KIND_CONTEXT_COMPACTION) {
    const payload = parseCompactionTracePayload(payloadJson);
    if (payload) {
      return [
        labels.compaction.detail,
        labels.compaction.range(payload.fromTurn, payload.toTurn),
        labels.compaction.tokens(payload.sourceTokens, payload.summaryTokens),
      ];
    }
  }
  return detailItems.map((item) => localizeTraceDetail(stage, item, payloadJson, labels)).filter(Boolean);
}

function localizeProcessSummary(summary: string, payloadJson: string | undefined, labels: ProcessTraceLabels): string {
  const structuredSummary = structuredProcessSummaryFromPayload(payloadJson, labels);
  if (structuredSummary) {
    return structuredSummary;
  }
  const normalized = summary.trim();
  if (/^(已纳入|未纳入)/.test(normalized)) {
    const counts = parseFileContextCounts(payloadJson);
    if (counts) {
      return formatFileContextCounts(counts, labels, false);
    }
  }
  if (/^检索到\s+\d+\s+段相关内容/.test(normalized)) {
    const counts = parseRAGTraceCounts(payloadJson);
    if (counts) {
      return labels.rag.summary(counts.chunkCount);
    }
  }
  const preparedMatch = normalized.match(/^准备\s+(\d+)\s+tokens\s+上下文$/);
  if (preparedMatch) {
    return labels.promptTrace.preparedSummary(Number(preparedMatch[1]));
  }
  const statefulMatch = normalized.match(/^续接发送\s+(\d+)\s+条消息$/);
  if (statefulMatch) {
    return labels.promptTrace.statefulSummary(Number(statefulMatch[1]));
  }
  if (/^已压缩第\s+\d+-\d+\s+轮上下文$/.test(normalized)) {
    const payload = parseCompactionTracePayload(payloadJson);
    if (payload) {
      return labels.compaction.summary(payload.fromTurn, payload.toTurn);
    }
  }
  return normalized;
}

function displayTraceStageLabel(label: string, labels: ProcessTraceLabels): string {
  switch (label) {
    case TRACE_KIND_CONTEXT_PLANNING:
    case TRACE_LABEL_CONTEXT_PLANNING:
      return labels.stages.contextPlanning;
    case TRACE_KIND_RAG:
    case TRACE_LABEL_RAG:
      return labels.stages.contentRetrieval;
    case TRACE_KIND_FILE_CONTEXT:
    case TRACE_LABEL_FILE_CONTEXT:
      return labels.stages.fileContext;
    case TRACE_KIND_CONTEXT_COMPACTION:
    case TRACE_LABEL_CONTEXT_COMPACTION:
      return labels.stages.contextCompaction;
    case TRACE_KIND_SKILL_CONTEXT:
      return labels.stages.skillContext;
    case TRACE_LABEL_UPSTREAM_RESULT:
      return labels.stages.requestResult;
    default:
      return label;
  }
}

function displayTraceTrigger(trigger: string, labels: ProcessTraceLabels): string {
  return trigger === TRACE_TRIGGER_UPSTREAM_REQUEST ? labels.stages.upstreamRequestTriggered : trigger;
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

function toolInputRecord(call: ToolTraceCall): Record<string, unknown> {
  const input = parseToolPayload(call.input);
  return isRecord(input) ? input : {};
}

function toolOutputPayload(call: ToolTraceCall): unknown {
  return parseToolPayload(call.output) ?? parseToolPayload(call.output_text) ?? parseToolPayload(call.output_preview);
}

function toolInputText(call: ToolTraceCall, keys: string[]): string {
  const input = parseToolPayload(call.input);
  if (isRecord(input)) {
    return firstStringFromRecord(input, keys);
  }
  return readString(input);
}

function toolOutputText(call: ToolTraceCall, keys: string[]): string {
  const output = toolOutputPayload(call);
  if (isRecord(output)) {
    return firstStringFromRecord(output, keys);
  }
  return readString(output);
}

function ToolTraceStructuredContent({
  call,
  rawDetail,
  failed,
  open,
  canExpand,
  onToggle,
  labels,
}: {
  call: ToolTraceCall;
  rawDetail: string;
  failed: boolean;
  open: boolean;
  canExpand: boolean;
  onToggle: () => void;
  labels: ProcessTraceLabels;
}) {
  const kind = resolveNativeToolKind(call);
  const input = toolInputRecord(call);
  const output = toolOutputPayload(call);
  const statusText = nativeToolStatusText(call, labels);
  const urlKeys = ["url", "uri", "image_url"];

  if (kind === "web_search") {
    const query = firstStringFromRecord(input, ["query", "q"]) || toolOutputText(call, ["query"]);
    const actionType = firstStringFromRecord(input, ["type", "action"]);
    const urls = collectToolStrings(output, urlKeys);
    const responseText = urls.length === 0
      ? formatToolPayload(call.output) || formatToolPayload(call.output_text) || formatToolPayload(call.output_preview)
      : "";
    const hasRequest = Boolean(query || (actionType && actionType !== query));
    const hasResponse = urls.length > 0 || Boolean(responseText);
    return (
      <div className={cn("space-y-2 text-muted-foreground/84", failed && "text-destructive/80")}>
        <div>{statusText}</div>
        {hasRequest ? (
          <div>
            <ToolMiniLabel>{labels.tool.detail.request}</ToolMiniLabel>
            <div className="space-y-1">
              {query ? <div className="break-words">{labels.tool.detail.query}: {query}</div> : null}
              {actionType && actionType !== query ? <div className="break-words">{labels.tool.detail.action}: {actionType}</div> : null}
            </div>
          </div>
        ) : null}
        {hasResponse ? (
          <div>
            <ToolMiniLabel>{failed ? labels.tool.detail.error : labels.tool.detail.response}</ToolMiniLabel>
            {urls.length > 0 ? <ToolSourceLinks urls={urls} labels={labels} /> : null}
            {responseText ? <ToolPre failed={failed}>{responseText}</ToolPre> : null}
          </div>
        ) : rawDetail ? (
          <ToolDetailText failed={failed} open={open} canExpand={canExpand} labels={labels} onToggle={onToggle}>
            {rawDetail}
          </ToolDetailText>
        ) : null}
      </div>
    );
  }

  if (kind === "code_interpreter") {
    const code = toolInputText(call, ["code", "input"]);
    const logs = collectToolStrings(output, ["logs", "stdout", "stderr", "text", "output"]).join("\n\n");
    const artifactURLs = collectToolStrings(output, urlKeys);
    return (
      <div className={cn("space-y-2 text-muted-foreground/84", failed && "text-destructive/80")}>
        <div>{statusText}</div>
        {code ? (
          <div>
            <ToolMiniLabel>{labels.tool.detail.code}</ToolMiniLabel>
            <ToolPre>{code}</ToolPre>
          </div>
        ) : null}
        {logs ? (
          <div>
            <ToolMiniLabel>{labels.tool.detail.output}</ToolMiniLabel>
            <ToolPre failed={failed}>{logs}</ToolPre>
          </div>
        ) : null}
        {artifactURLs.length > 0 ? (
          <div>
            <ToolMiniLabel>{labels.tool.detail.resultFile}</ToolMiniLabel>
            <ToolSourceLinks urls={artifactURLs} labels={labels} />
          </div>
        ) : null}
        {!code && !logs && artifactURLs.length === 0 && rawDetail ? (
          <ToolDetailText failed={failed} open={open} canExpand={canExpand} labels={labels} onToggle={onToggle}>
            {rawDetail}
          </ToolDetailText>
        ) : null}
      </div>
    );
  }

  if (kind === "image_generation") {
    const urls = collectToolImageSources(output);
    const prompt = toolInputText(call, ["prompt", "input"]);
    return (
      <div className={cn("space-y-2 text-muted-foreground/84", failed && "text-destructive/80")}>
        <div>{statusText}</div>
        {prompt ? <div className="break-words">{labels.tool.detail.prompt}: {prompt}</div> : null}
        {urls.length > 0 ? <ToolImageGrid urls={urls} labels={labels} /> : null}
        {urls.length === 0 && rawDetail ? (
          <ToolDetailText failed={failed} open={open} canExpand={canExpand} labels={labels} onToggle={onToggle}>
            {rawDetail}
          </ToolDetailText>
        ) : null}
      </div>
    );
  }

  if (kind === "shell") {
    const command = firstStringFromRecord(input, ["cmd", "command", "input"]) || readString(parseToolPayload(call.input));
    const stdout = toolOutputText(call, ["stdout", "output"]);
    const stderr = toolOutputText(call, ["stderr", "error"]);
    const exitCode = isRecord(output) ? readNumber(output.exit_code) ?? readNumber(output.code) : null;
    return (
      <div className={cn("space-y-2 text-muted-foreground/84", failed && "text-destructive/80")}>
        <div>{statusText}</div>
        {command ? (
          <div>
            <ToolMiniLabel>{labels.tool.detail.command}</ToolMiniLabel>
            <ToolPre>{command}</ToolPre>
          </div>
        ) : null}
        {stdout ? (
          <div>
            <ToolMiniLabel>stdout</ToolMiniLabel>
            <ToolPre>{stdout}</ToolPre>
          </div>
        ) : null}
        {stderr ? (
          <div>
            <ToolMiniLabel>stderr</ToolMiniLabel>
            <ToolPre failed>{stderr}</ToolPre>
          </div>
        ) : null}
        {exitCode !== null ? <div className="text-[11px] text-muted-foreground/62">exit code: {exitCode}</div> : null}
        {!command && !stdout && !stderr && rawDetail ? (
          <ToolDetailText failed={failed} open={open} canExpand={canExpand} labels={labels} onToggle={onToggle}>
            {rawDetail}
          </ToolDetailText>
        ) : null}
      </div>
    );
  }

  return (
    <ToolDetailText failed={failed} open={open} canExpand={canExpand} labels={labels} onToggle={onToggle}>
      {call.latency_ms && call.latency_ms > 0 ? <span>{call.latency_ms}ms</span> : null}
      {call.latency_ms && call.latency_ms > 0 && rawDetail ? <span>{labels.tool.detail.latencySeparator}</span> : null}
      {rawDetail ? <span>{rawDetail}</span> : null}
    </ToolDetailText>
  );
}

function ToolTraceRows({ calls, labels }: { calls: ToolTraceCall[]; labels: ProcessTraceLabels }) {
  const [expanded, setExpanded] = React.useState<Set<number>>(() => new Set());

  if (calls.length === 0) return null;

  return (
    <ol className="space-y-0.5">
      {calls.map((call, index) => {
        const label = toolTraceCallLabel(call, labels);
        const { detail: rawDetail, failed } = toolTraceCallDetail(call, labels);
        const open = expanded.has(index);
        const canExpand = shouldCollapseToolDetail(rawDetail);

        return (
          <li
            key={`${label}-${index}-${call.latency_ms ?? 0}`}
            className={cn(
              "group/tool-row grid grid-cols-[0.875rem_8rem_minmax(0,1fr)] gap-x-5 gap-y-0.5 text-[12px] leading-5",
              "max-sm:grid-cols-[0.875rem_minmax(0,1fr)] max-sm:gap-x-2",
            )}
          >
            <div className="relative flex justify-center">
              {index > 0 ? <span className="absolute -top-0.5 bottom-1/2 w-px bg-border/42" /> : null}
              {index < calls.length - 1 ? <span className="absolute bottom-[-0.125rem] top-1/2 w-px bg-border/42" /> : null}
              <span
                className={cn(
                  "relative z-10 mt-[0.45rem] size-1.5 rounded-full bg-muted-foreground/38 ring-4 ring-background transition-colors group-hover/tool-row:bg-foreground/58",
                  failed && "bg-destructive/80",
                )}
              />
            </div>
            <div className="min-w-0 max-sm:col-start-2">
              <span
                className={cn(
                  "block truncate font-medium text-muted-foreground/76 transition-colors group-hover/tool-row:text-foreground/88",
                  failed && "text-destructive/85 group-hover/tool-row:text-destructive",
                )}
              >
                {label}
              </span>
            </div>
            <div className="min-w-0 pb-2 max-sm:col-start-2">
              <ToolTraceStructuredContent
                call={call}
                rawDetail={rawDetail}
                failed={failed}
                open={open}
                canExpand={canExpand}
                labels={labels}
                onToggle={() =>
                  setExpanded((current) => {
                    const next = new Set(current);
                    if (next.has(index)) {
                      next.delete(index);
                    } else {
                      next.add(index);
                    }
                    return next;
                  })
                }
              />
            </div>
          </li>
        );
      })}
    </ol>
  );
}

function TraceContent({
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

function traceEventToBlock(event: ChatTraceEvent): ChatTraceBlock {
  return {
    title: event.title,
    summary: event.summary,
    contentMarkdown: event.contentMarkdown,
    status: event.status,
    stage: event.stage,
    roundID: event.roundID,
    parentEventID: event.parentEventID,
    updatedAt: event.updatedAt,
    payloadJson: event.payloadJson,
  };
}

function isToolTraceEvent(event: ChatTraceEvent): boolean {
  if (event.stage === "think" || event.phase === "upstream_think" || event.eventType === "think") {
    return false;
  }
  return event.stage === "tool" || event.phase === "tools" || event.eventType === "tool";
}

function isThinkTraceEvent(event: ChatTraceEvent): boolean {
  return event.stage === "think" || event.phase === "upstream_think" || event.eventType === "think";
}

type TraceDisplayEvent = {
  event: ChatTraceEvent;
  kind: "think" | "tool";
};

function buildTraceDisplayEvents(events: ChatTraceEvent[]): TraceDisplayEvent[] {
  return events
    .filter((event) => isToolTraceEvent(event) || isThinkTraceEvent(event))
    .sort((left, right) => left.seq - right.seq)
    .map((event) => {
      if (isThinkTraceEvent(event)) {
        return { event, kind: "think" };
      }
      return { event, kind: "tool" };
    });
}

function traceBlockDisplayText(block: Pick<ChatTraceBlock, "contentMarkdown" | "summary">): string {
  return block.contentMarkdown?.trim() || block.summary?.trim() || "";
}

type OrderedThinkBlock = ChatTraceBlock & {
  seq: number;
};

function mergeThinkTraceBlock(events: TraceDisplayEvent[], activeThinkBlock?: ChatTraceBlock): ChatTraceBlock | undefined {
  const blocks: OrderedThinkBlock[] = events
    .filter((item) => item.kind === "think")
    .map((item) => ({ ...traceEventToBlock(item.event), seq: item.event.seq }));

  if (activeThinkBlock) {
    const activeText = traceBlockDisplayText(activeThinkBlock);
    const activeIndex = blocks.findIndex((block) => {
      const sameRound = Boolean(activeThinkBlock.roundID && block.roundID === activeThinkBlock.roundID);
      const sameParent = Boolean(activeThinkBlock.parentEventID && block.parentEventID === activeThinkBlock.parentEventID);
      const sameText = Boolean(activeText && traceBlockDisplayText(block) === activeText);
      return sameRound || sameParent || sameText;
    });
    if (activeIndex >= 0) {
      blocks[activeIndex] = { ...activeThinkBlock, seq: blocks[activeIndex].seq };
    } else {
      blocks.push({ ...activeThinkBlock, seq: Number.MAX_SAFE_INTEGER });
    }
  }

  if (blocks.length === 0) {
    return undefined;
  }

  const ordered = [...blocks].sort((left, right) => left.seq - right.seq);
  const parts: string[] = [];
  for (const block of ordered) {
    const text = traceBlockDisplayText(block);
    if (text && !parts.includes(text)) {
      parts.push(text);
    }
  }
  if (parts.length === 0) {
    return undefined;
  }

  const latest = ordered[ordered.length - 1];
  return {
    ...latest,
    stage: "think",
    status: ordered.some((block) => block.status === "streaming") ? "streaming" : latest.status || "completed",
    contentMarkdown: parts.join("\n\n"),
    contentSegments: parts,
  };
}

function splitTraceDisplayEvents(events: TraceDisplayEvent[], activeThinkBlock?: ChatTraceBlock) {
  return {
    toolEvents: events.filter((item) => item.kind === "tool"),
    thinkBlock: mergeThinkTraceBlock(events, activeThinkBlock),
  };
}

export function MessageTraceEventBlocks({
  events: traceEvents,
  activeToolBlock,
  activeThinkBlock,
  messageStreaming,
  autoCollapseReady,
}: {
  events: ChatTraceEvent[];
  activeToolBlock?: ChatTraceBlock;
  activeThinkBlock?: ChatTraceBlock;
  messageStreaming?: boolean;
  autoCollapseReady?: boolean;
}) {
  const displayEvents = React.useMemo(() => buildTraceDisplayEvents(traceEvents), [traceEvents]);
  const { toolEvents, thinkBlock } = React.useMemo(
    () => splitTraceDisplayEvents(displayEvents, activeThinkBlock),
    [activeThinkBlock, displayEvents],
  );
  if (toolEvents.length === 0 && !activeToolBlock && !thinkBlock) {
    return null;
  }

  return (
    <>
      <MessageToolChainTrace
        events={toolEvents}
        activeToolBlock={activeToolBlock}
        streaming={Boolean(messageStreaming && (activeToolBlock?.status === "streaming" || toolEvents.some((item) => item.event.status === "streaming")))}
        autoCollapseReady={autoCollapseReady || Boolean(thinkBlock)}
      />
      {thinkBlock ? (
        <MessageUpstreamThink
          block={thinkBlock}
          streaming={Boolean(messageStreaming && thinkBlock.status === "streaming")}
          autoCollapseReady={autoCollapseReady}
        />
      ) : null}
    </>
  );
}

type ToolChainStep = {
  key: string;
  label: string;
  detail: string;
  failed: boolean;
  latencyMS?: number;
  toolCallID?: string;
  toolType?: string;
  toolName?: string;
  toolInput?: string;
  toolStatus?: string;
  toolCall?: ToolTraceCall;
};

function toolTraceCallID(call: ToolTraceCall): string {
  return call.tool_call_id?.trim() || call.id?.trim() || call.call_id?.trim() || "";
}

function toolTraceStatusRank(status: string | undefined): number {
  switch (status?.trim()) {
    case "error":
    case "failed":
      return 4;
    case "success":
    case "completed":
    case "reused":
      return 3;
    case "requested":
    case "streaming":
    case "queued":
    case "in_progress":
    case "searching":
      return 2;
    default:
      return 1;
  }
}

function sameToolChainCall(left: ToolChainStep, right: ToolChainStep): boolean {
  if (left.toolCallID && right.toolCallID) return left.toolCallID === right.toolCallID;
  const leftName = left.toolName?.trim() || "";
  const rightName = right.toolName?.trim() || "";
  const leftType = left.toolType?.trim() || "";
  const rightType = right.toolType?.trim() || "";
  const sameKind = leftName && rightName ? leftName === rightName : Boolean(leftType && rightType && leftType === rightType);
  if (!sameKind) return false;
  const leftInput = left.toolInput?.trim() || "";
  const rightInput = right.toolInput?.trim() || "";
  return !leftInput || !rightInput || leftInput === rightInput;
}

function dedupeToolChainSteps(steps: ToolChainStep[]): ToolChainStep[] {
  const result: ToolChainStep[] = [];
  for (const step of steps) {
    const existingIndex = result.findIndex((item) => sameToolChainCall(item, step));
    if (existingIndex < 0) {
      result.push(step);
      continue;
    }
    const current = result[existingIndex];
    const nextRank = toolTraceStatusRank(step.toolStatus);
    const currentRank = toolTraceStatusRank(current.toolStatus);
    if (nextRank > currentRank || (nextRank === currentRank && step.detail.length >= current.detail.length)) {
      result[existingIndex] = step;
    }
  }
  return result;
}

function buildToolChainSteps(events: TraceDisplayEvent[], labels: ProcessTraceLabels): ToolChainStep[] {
  return events.flatMap<ToolChainStep>((item, eventIndex) => {
    const event = item.event;
    if (item.kind !== "tool") {
      return [];
    }

    const calls = parseToolTraceCalls(event.payloadJson);
    if (calls.length === 0) {
      return [
        {
          key: event.eventID || `tool-${event.seq}`,
          label: labels.tool.names.generic,
          detail: event.contentMarkdown?.trim() || event.summary?.trim() || event.title?.trim() || "",
          failed: event.status === "error",
        },
      ];
    }

    return calls.map((call, callIndex) => {
      const label = toolTraceCallLabel(call, labels);
      const { detail, failed } = toolTraceCallDetail(call, labels);
      return {
        key: `${event.eventID || event.seq}-${label}-${callIndex}-${eventIndex}`,
        label,
        detail,
        failed,
        latencyMS: call.latency_ms,
        toolCallID: toolTraceCallID(call),
        toolType: call.type?.trim(),
        toolName: call.name?.trim(),
        toolInput: call.input?.trim(),
        toolStatus: call.status?.trim(),
        toolCall: call,
      };
    });
  });
}

function buildToolChainStepsFromBlock(block: ChatTraceBlock | undefined, labels: ProcessTraceLabels): ToolChainStep[] {
  if (!block) {
    return [];
  }
  const calls = parseToolTraceCalls(block.payloadJson);
  if (calls.length === 0) {
    const detail = block.contentMarkdown?.trim() || block.summary?.trim() || block.title?.trim() || "";
    if (!detail) return [];
    return [
      {
        key: "active-tool",
        label: labels.tool.names.generic,
        detail,
        failed: block.status === "error",
      },
    ];
  }
  return calls.map((call, index) => {
    const label = toolTraceCallLabel(call, labels);
    const { detail, failed } = toolTraceCallDetail(call, labels);
    return {
      key: `active-tool-${label}-${index}`,
      label,
      detail,
      failed,
      latencyMS: call.latency_ms,
      toolCallID: toolTraceCallID(call),
      toolType: call.type?.trim(),
      toolName: call.name?.trim(),
      toolInput: call.input?.trim(),
      toolStatus: call.status?.trim(),
      toolCall: call,
    };
  });
}

function ToolChainRows({ steps, labels }: { steps: ToolChainStep[]; labels: ProcessTraceLabels }) {
  const [expanded, setExpanded] = React.useState<Set<string>>(() => new Set());

  if (steps.length === 0) return null;

  return (
    <ol className="space-y-0.5">
      {steps.map((step, index) => {
        const open = expanded.has(step.key);
        const canExpand = shouldCollapseToolDetail(step.detail);

        return (
          <li
            key={step.key}
            className={cn(
              "group/tool-chain-row grid grid-cols-[0.875rem_8rem_minmax(0,1fr)] gap-x-5 gap-y-0.5 text-[12px] leading-5",
              "max-sm:grid-cols-[0.875rem_minmax(0,1fr)] max-sm:gap-x-2",
            )}
          >
            <div className="relative flex justify-center">
              {index > 0 ? <span className="absolute -top-0.5 bottom-1/2 w-px bg-border/42" /> : null}
              {index < steps.length - 1 ? <span className="absolute bottom-[-0.125rem] top-1/2 w-px bg-border/42" /> : null}
              <span
                className={cn(
                  "relative z-10 mt-[0.45rem] size-1.5 rounded-full bg-muted-foreground/38 ring-4 ring-background transition-colors group-hover/tool-chain-row:bg-foreground/58",
                  step.failed && "bg-destructive/80",
                )}
              />
            </div>
            <div className="min-w-0 max-sm:col-start-2">
              <span
                className={cn(
                  "block truncate font-medium text-muted-foreground/76 transition-colors group-hover/tool-chain-row:text-foreground/88",
                  step.failed && "text-destructive/85 group-hover/tool-chain-row:text-destructive",
                )}
              >
                {step.label}
              </span>
            </div>
            <div className="min-w-0 pb-2 max-sm:col-start-2">
              {step.toolCall ? (
                <ToolTraceStructuredContent
                  call={step.toolCall}
                  rawDetail={step.detail}
                  failed={step.failed}
                  open={open}
                  canExpand={canExpand}
                  labels={labels}
                  onToggle={() =>
                    setExpanded((current) => {
                      const next = new Set(current);
                      if (next.has(step.key)) {
                        next.delete(step.key);
                      } else {
                        next.add(step.key);
                      }
                      return next;
                    })
                  }
                />
              ) : (
                <ToolDetailText
                  failed={step.failed}
                  open={open}
                  canExpand={canExpand}
                  labels={labels}
                  onToggle={() =>
                    setExpanded((current) => {
                      const next = new Set(current);
                      if (next.has(step.key)) {
                        next.delete(step.key);
                      } else {
                        next.add(step.key);
                      }
                      return next;
                    })
                  }
                >
                  {step.latencyMS && step.latencyMS > 0 ? <span>{step.latencyMS}ms</span> : null}
                  {step.latencyMS && step.latencyMS > 0 && step.detail ? <span>{labels.tool.detail.latencySeparator}</span> : null}
                  {step.detail ? <span>{step.detail}</span> : null}
                </ToolDetailText>
              )}
            </div>
          </li>
        );
      })}
    </ol>
  );
}

function MessageToolChainTrace({
  events,
  activeToolBlock,
  streaming,
  autoCollapseReady,
}: {
  events: TraceDisplayEvent[];
  activeToolBlock?: ChatTraceBlock;
  streaming?: boolean;
  autoCollapseReady?: boolean;
}) {
  const labels = useProcessTraceLabels();
  const steps = React.useMemo(
    () =>
      dedupeToolChainSteps([
        ...buildToolChainSteps(events, labels),
        ...buildToolChainStepsFromBlock(activeToolBlock, labels),
      ]),
    [activeToolBlock, events, labels],
  );
  const [accordionValue, setAccordionValue] = React.useState(() => (streaming ? "message-tool-chain" : ""));

  React.useEffect(() => {
    if (streaming) {
      setAccordionValue("message-tool-chain");
      return;
    }
    if (autoCollapseReady) {
      setAccordionValue("");
    }
  }, [autoCollapseReady, streaming]);

  if (steps.length === 0) {
    return null;
  }

  const open = accordionValue === "message-tool-chain";

  return (
    <div className={TRACE_ROOT_CLASS}>
      <Accordion
        type="single"
        collapsible
        value={accordionValue}
        onValueChange={(value) => setAccordionValue(value || "")}
        className="w-full"
      >
        <AccordionItem value="message-tool-chain" className="border-b-0">
          <AccordionTrigger
            showArrow={false}
            className="group/tool-chain min-h-0 gap-1.5 py-0.5 text-left no-underline hover:no-underline"
          >
            <div className="min-w-0 flex-1">
              <div className="flex items-center">
                <span
                  className={cn(
                    "text-[13px] font-medium transition-colors",
                    streaming ? "thinking-shimmer" : "text-muted-foreground group-hover/tool-chain:text-foreground",
                  )}
                >
                  {streaming ? labels.tool.chain.titleActive : labels.tool.chain.titleDone}
                </span>
              </div>
              <div className="mt-0.5 truncate text-[11px] font-normal leading-4 text-muted-foreground/62">
                {steps.length > 0 ? labels.tool.chain.summaryCount(steps.length) : labels.tool.chain.summaryFallback}
              </div>
            </div>
            <ChevronDown
              className={cn(
                "mt-0.5 size-3.5 shrink-0 text-muted-foreground transition-transform duration-200 group-hover/tool-chain:text-foreground",
                open && "rotate-180",
              )}
            />
          </AccordionTrigger>
          <AccordionContent className="pb-0 pt-1.5">
            <ToolChainRows steps={steps} labels={labels} />
          </AccordionContent>
        </AccordionItem>
      </Accordion>
    </div>
  );
}

export function MessageUpstreamThink({
  block,
  streaming,
  autoCollapseReady,
  title,
  subtitle,
}: {
  block?: ChatTraceBlock;
  streaming?: boolean;
  autoCollapseReady?: boolean;
  title?: string;
  subtitle?: string;
}) {
  const labels = useProcessTraceLabels();
  const [accordionValue, setAccordionValue] = React.useState(() => (streaming ? "upstream-think" : ""));
  const wasStreamingRef = React.useRef(Boolean(streaming));

  React.useEffect(() => {
    if (streaming) {
      setAccordionValue("upstream-think");
      wasStreamingRef.current = true;
      return;
    }

    if (wasStreamingRef.current && autoCollapseReady) {
      setAccordionValue("");
    }
    if (autoCollapseReady) {
      wasStreamingRef.current = false;
    }
  }, [autoCollapseReady, streaming]);

  if (!block) {
    return null;
  }

  const open = accordionValue === "upstream-think";
  const resolvedTitle = title ?? (streaming ? labels.think.titleActive : labels.think.titleDone);
  const resolvedSubtitle = subtitle ?? (streaming ? labels.think.subtitleActive : labels.think.subtitleDone);
  const contentSegments = block.contentSegments?.filter((item) => item.trim()) ?? [];

  return (
    <div className={TRACE_ROOT_CLASS}>
      <Accordion
        type="single"
        collapsible
        value={accordionValue}
        onValueChange={(value) => setAccordionValue(value || "")}
        className="w-full"
      >
        <AccordionItem value="upstream-think" className="border-b-0">
          <AccordionTrigger
            showArrow={false}
            className="group items-start gap-1.5 py-0 text-left no-underline hover:no-underline"
          >
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-1.5">
                <span
                  className={cn(
                    "text-[13px] font-medium transition-colors",
                    streaming ? "thinking-shimmer" : "text-muted-foreground group-hover:text-foreground",
                  )}
                >
                  {resolvedTitle}
                </span>
              </div>
              <div className="mt-0.5 truncate text-[11px] font-normal leading-4 text-muted-foreground/62">{resolvedSubtitle}</div>
            </div>
            <ChevronDown
              className={cn(
                "mt-0.5 size-3.5 shrink-0 text-muted-foreground transition-transform duration-200 group-hover:text-foreground",
                open && "rotate-180",
              )}
            />
          </AccordionTrigger>
          <AccordionContent className="pb-0 pt-1.5">
            {contentSegments.length > 0 ? (
              <div className="space-y-3">
                {contentSegments.map((content, index) => (
                  <StreamdownRender
                    key={`${index}-${content.slice(0, 24)}`}
                    content={content}
                    streaming={Boolean(streaming && index === contentSegments.length - 1)}
                    variant="thinking"
                  />
                ))}
              </div>
            ) : (
              <StreamdownRender content={block.contentMarkdown} streaming={Boolean(streaming)} variant="thinking" />
            )}
          </AccordionContent>
        </AccordionItem>
      </Accordion>
    </div>
  );
}

export function MessageToolTrace({
  block,
  streaming,
  autoCollapseReady,
  title,
}: {
  block?: ChatTraceBlock;
  streaming?: boolean;
  autoCollapseReady?: boolean;
  title?: string;
}) {
  const labels = useProcessTraceLabels();
  const [accordionValue, setAccordionValue] = React.useState(() => (streaming ? "message-tool-trace" : ""));

  React.useEffect(() => {
    if (streaming) {
      setAccordionValue("message-tool-trace");
      return;
    }
    if (autoCollapseReady) {
      setAccordionValue("");
    }
  }, [autoCollapseReady, streaming]);

  if (!block) {
    return null;
  }

  const open = accordionValue === "message-tool-trace";
  const resolvedTitle = title ?? (streaming ? labels.tool.trace.titleActive : labels.tool.trace.titleDone);
  const toolCalls = parseToolTraceCalls(block.payloadJson);
  const summary = localizeToolTraceSummary(block, toolCalls, labels);

  return (
    <div className={TRACE_ROOT_CLASS}>
      <Accordion
        type="single"
        collapsible
        value={accordionValue}
        onValueChange={(value) => setAccordionValue(value || "")}
        className="w-full"
      >
        <AccordionItem value="message-tool-trace" className="border-b-0">
          <AccordionTrigger
            showArrow={false}
            className="group/tool min-h-0 gap-1.5 py-0.5 text-left no-underline hover:no-underline"
          >
            <div className="min-w-0 flex-1">
              <div className="flex items-center">
                <span
                  className={cn(
                    "text-[13px] font-medium transition-colors",
                    streaming ? "thinking-shimmer" : "text-muted-foreground group-hover/tool:text-foreground",
                  )}
                >
                  {resolvedTitle}
                </span>
              </div>
              <div className="mt-0.5 truncate text-[11px] font-normal leading-4 text-muted-foreground/62">{summary}</div>
            </div>
            <ChevronDown
              className={cn(
                "mt-0.5 size-3.5 shrink-0 text-muted-foreground transition-transform duration-200 group-hover/tool:text-foreground",
                open && "rotate-180",
              )}
            />
          </AccordionTrigger>
          <AccordionContent className="pb-0 pt-1.5">
            {toolCalls.length > 0 ? (
              <ToolTraceRows calls={toolCalls} labels={labels} />
            ) : (
              <TraceContent block={block} streaming={Boolean(streaming)} labels={labels} />
            )}
          </AccordionContent>
        </AccordionItem>
      </Accordion>
    </div>
  );
}

function buildProcessSummary(trace: ChatMessageProcessTrace): string {
  if (trace.process?.summary) {
    return trace.process.summary;
  }
  return "";
}

function promptTraceModeSentence(mode: string, labels: ProcessTraceLabels): string {
  switch (mode.trim()) {
    case "stateful":
      return labels.promptTrace.modes.stateful;
    case "full_retry":
      return labels.promptTrace.modes.fullRetry;
    default:
      return labels.promptTrace.modes.full;
  }
}

function promptTraceReasonLabel(reason: string, labels: ProcessTraceLabels): string {
  switch (reason.trim()) {
    case "":
      return "";
    case "route_or_branch_not_eligible":
      return "";
    case "missing_stored_fingerprint":
      return labels.promptTrace.reasons.missingStoredFingerprint;
    case "missing_current_fingerprint":
      return labels.promptTrace.reasons.missingCurrentFingerprint;
    case "prompt_fingerprint_mismatch":
      return labels.promptTrace.reasons.fingerprintMismatch;
    case "previous_response_rejected":
      return labels.promptTrace.reasons.previousRejected;
    default:
      return reason;
  }
}

function promptTraceStage(trace: ChatPromptTrace | undefined, labels: ProcessTraceLabels): TraceStage | null {
  if (!trace) return null;
  const cacheableBlocks = trace.blocks.filter((block) => block.cacheable).length;
  const historicalEvidence = trace.blocks
    .filter((block) => block.kind === "historical_evidence")
    .reduce((total, block) => total + block.sourceCount, 0);
  const dynamicSources = trace.blocks
    .filter((block) => block.kind === "dynamic_context")
    .reduce((total, block) => total + block.sourceCount, 0);

  const details = [
    labels.promptTrace.sentSummary(promptTraceModeSentence(trace.mode, labels), trace.sentMessageCount, trace.fullMessageCount, trace.sentTokenEstimate),
  ];
  if (trace.statefulSavedMessages > 0 || trace.statefulSavedTokens > 0) {
    details.push(labels.promptTrace.savedHistory(trace.statefulSavedMessages, trace.statefulSavedTokens));
  }

  const extras = [];
  if (cacheableBlocks > 0) extras.push(labels.promptTrace.cacheableBlocks(cacheableBlocks));
  if (historicalEvidence > 0) extras.push(labels.promptTrace.historicalEvidence(historicalEvidence));
  if (dynamicSources > 0) extras.push(labels.promptTrace.dynamicSources(dynamicSources));
  if (extras.length > 0) {
    details.push(labels.promptTrace.extraSummary(extras.join(labels.promptTrace.listSeparator)));
  }

  const reason = promptTraceReasonLabel(trace.statefulDisabledReason, labels);
  if (reason && !trace.statefulUsed) {
    details.push(labels.promptTrace.reasonLine(reason));
  }

  return {
    label: TRACE_KIND_CONTEXT_PLANNING,
    kind: TRACE_KIND_CONTEXT_PLANNING,
    trigger: "",
    detail: details.join("\n"),
    details,
    structured: true,
  };
}

function mergePromptTraceStage(stages: TraceStage[], trace: ChatPromptTrace | undefined, labels: ProcessTraceLabels): TraceStage[] {
  const stage = promptTraceStage(trace, labels);
  if (!stage) {
    return stages;
  }
  let replaced = false;
  const result = stages.flatMap((item) => {
    if (!isContextPlanningTraceStage(item)) {
      return [item];
    }
    if (replaced) {
      return [];
    }
    replaced = true;
    return [stage];
  });
  return replaced ? result : [...result, stage];
}

export function MessageProcessTrace({
  trace,
  active,
  autoCollapseReady,
}: {
  trace?: ChatMessageProcessTrace;
  active?: boolean;
  autoCollapseReady?: boolean;
}) {
  const labels = useProcessTraceLabels();
  const processStreaming = Boolean(active && trace?.process?.status === "streaming");
  const [accordionValue, setAccordionValue] = React.useState(() => (processStreaming ? "message-process-trace" : ""));

  React.useEffect(() => {
    if (processStreaming) {
      setAccordionValue("message-process-trace");
      return;
    }
    if (autoCollapseReady) {
      setAccordionValue("");
    }
  }, [autoCollapseReady, processStreaming]);

  if (!trace?.enabled || !trace.process) {
    return null;
  }

  const summary = localizeProcessSummary(buildProcessSummary(trace), trace.process.payloadJson, labels);
  const citations = parseRAGCitations(trace.process.payloadJson);
  const fileBadges = parseFileContextBadges(trace.process.payloadJson, labels);
  const structuredStages = parseStructuredTraceStages(trace.process.payloadJson, labels);
  const parsedStages = structuredStages.length > 0 ? [] : parseTraceStages(trace.process.contentMarkdown);
  const stages = filterProcessTraceStages(mergePromptTraceStage(structuredStages.length > 0 ? structuredStages : parsedStages, trace.promptTrace, labels));
  const hasRAGStage = stages.some(isRAGTraceStage);
  const hasRenderableProcessContent = stages.length > 0 || (trace.process.contentMarkdown.trim() && parsedStages.length === 0);
  if (!hasRenderableProcessContent && citations.length === 0 && !trace.promptTrace) {
    return null;
  }
  const open = accordionValue === "message-process-trace";

  return (
    <div className={TRACE_ROOT_CLASS}>
      <Accordion
        type="single"
        collapsible
        value={accordionValue}
        onValueChange={(value) => setAccordionValue(value || "")}
        className="w-full"
      >
        <AccordionItem value="message-process-trace" className="border-b-0">
          <AccordionTrigger
            showArrow={false}
            className="group/trace min-h-0 gap-1.5 py-0.5 text-left no-underline hover:no-underline"
          >
            <div className="min-w-0 flex-1">
              <div className="flex items-center">
                <span
                  className={cn(
                    "text-[13px] font-medium transition-colors",
                    processStreaming ? "thinking-shimmer" : "text-muted-foreground group-hover/trace:text-foreground",
                  )}
                >
                  {processStreaming ? labels.process.titleActive : labels.process.titleDone}
                </span>
              </div>
              {summary ? (
                <div className="mt-0.5 truncate text-[11px] font-normal leading-4 text-muted-foreground/62">{summary}</div>
              ) : null}
            </div>
            <ChevronDown
              className={cn(
                "mt-0.5 size-3.5 shrink-0 text-muted-foreground transition-transform duration-200 group-hover/trace:text-foreground",
                open && "rotate-180",
              )}
            />
          </AccordionTrigger>
          <AccordionContent className="space-y-2.5 pb-0 pt-1.5">
            <TraceContent
              block={trace.process}
              streaming={processStreaming}
              citations={citations}
              fileBadges={fileBadges}
              promptTrace={trace.promptTrace}
              labels={labels}
            />
            {!hasRAGStage ? <RAGCitationList citations={citations} labels={labels} /> : null}
          </AccordionContent>
        </AccordionItem>
      </Accordion>
    </div>
  );
}
