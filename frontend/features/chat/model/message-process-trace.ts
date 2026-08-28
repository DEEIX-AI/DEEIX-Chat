import type { ProcessTraceLabels } from "@/features/chat/hooks/use-chat-trace-labels";
import type { ChatPromptTrace, ChatTraceEvent, RAGCitation } from "@/features/chat/types/messages";

export const TRACE_KIND_CONTEXT_PLANNING = "context_planning";
export const TRACE_KIND_RAG = "content_retrieval";
export const TRACE_KIND_FILE_CONTEXT = "file_context";
export const TRACE_KIND_CONTEXT_COMPACTION = "context_compaction";
export const TRACE_KIND_SKILL_CONTEXT = "skill_context";

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

export type FileContextBadge = {
  fileID?: string;
  name: string;
  label: string;
  description?: string;
  tab: "extract" | "preview";
};

export type FileContextCounts = {
  included: number;
  skipped: number;
};

export type RAGTraceCounts = {
  fileCount: number;
  chunkCount: number;
};

export type CompactionTracePayload = {
  fromTurn: number;
  toTurn: number;
  sourceTokens: number;
  summaryTokens: number;
};

export type TraceStage = {
  label: string;
  kind?: string;
  status?: string;
  trigger: string;
  detail: string;
  details: string[];
  structured?: boolean;
};

export type TraceDisplayEvent = {
  event: ChatTraceEvent;
  kind: "think" | "tool";
};

export function parseRAGCitations(payloadJson: string | undefined): RAGCitation[] {
  if (!payloadJson) return [];
  try {
    const parsed = JSON.parse(payloadJson) as { citations?: RAGCitation[] };
    return Array.isArray(parsed.citations) ? parsed.citations : [];
  } catch {
    return [];
  }
}

function readStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value.map((item) => (typeof item === "string" ? item.trim() : "")).filter(Boolean);
}

function readArrayCount(value: unknown): number {
  return Array.isArray(value) ? value.length : 0;
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
  const chunkCount =
    typeof parsed.hit_chunk_count === "number" && Number.isFinite(parsed.hit_chunk_count)
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
    parts.push(
      includedDetail
        ? labels.fileContext.includedDetail(counts.included)
        : labels.fileContext.includedSummary(counts.included),
    );
  }
  if (counts.skipped > 0) {
    parts.push(labels.fileContext.skipped(counts.skipped));
  }
  return parts.join(labels.fileContext.separator);
}

function readFileContextBadges(
  value: unknown,
  label: string,
  description: string,
  tab: "extract" | "preview",
): FileContextBadge[] {
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

export function parseFileContextBadges(payloadJson: string | undefined, labels: ProcessTraceLabels): FileContextBadge[] {
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
      ...readFileContextBadges(
        groups.full_context,
        labels.fileBadges.fullContext,
        labels.fileBadges.descriptions.fullContext,
        "extract",
      ),
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

export function isContextPlanningTraceStage(stage: TraceStage): boolean {
  return traceStageKind(stage) === TRACE_KIND_CONTEXT_PLANNING;
}

export function isRAGTraceStage(stage: TraceStage): boolean {
  return traceStageKind(stage) === TRACE_KIND_RAG;
}

export function isFileContextTraceStage(stage: TraceStage): boolean {
  return traceStageKind(stage) === TRACE_KIND_FILE_CONTEXT;
}

export function isTraceStageError(stage: TraceStage): boolean {
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
	const status = readString(stage.status);
	if (status === "pending") return [labels.compaction.pending];
	if (status === "failed") return [labels.compaction.failed];
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

export function parseStructuredTraceStages(payloadJson: string | undefined, labels: ProcessTraceLabels): TraceStage[] {
  return readTraceStagePayloads(payloadJson)
    .flatMap((payload) => {
      const kind = readString(payload.kind);
      if (!kind) return [];
      const details = structuredTraceStageDetails(payload, labels);
      if (details.length === 0) return [];
      return [
        {
          label: kind,
          kind,
          status: readString(payload.status),
          trigger: "",
          detail: details.join("\n"),
          details,
          structured: true,
        },
      ];
    })
    .filter((stage, index, stages) => {
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
	const status = readString(last.status);
	if (status === "pending") return labels.compaction.pending;
	if (status === "failed") return labels.compaction.failed;
    const fromTurn = readNumber(last.from_turn);
    const toTurn = readNumber(last.to_turn);
    if (fromTurn !== null && toTurn !== null) {
      return labels.compaction.summary(fromTurn, toTurn);
    }
  }
  return "";
}

export function parseTraceStages(content: unknown): TraceStage[] {
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

export function filterProcessTraceStages(stages: TraceStage[]): TraceStage[] {
  return stages.flatMap((stage) => {
    if (isUpstreamFailureTraceStage(stage)) {
      return [];
    }
    const sanitized = sanitizeProcessTraceStage(stage);
    return sanitized ? [sanitized] : [];
  });
}

export function normalizeTraceListItem(text: string): string {
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

export function localizeTraceDetailItems(
  stage: TraceStage,
  detailItems: string[],
  payloadJson: string | undefined,
  labels: ProcessTraceLabels,
): string[] {
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

export function localizeProcessSummary(summary: string, payloadJson: string | undefined, labels: ProcessTraceLabels): string {
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

export function displayTraceStageLabel(label: string, labels: ProcessTraceLabels): string {
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

export function displayTraceTrigger(trigger: string, labels: ProcessTraceLabels): string {
  return trigger === TRACE_TRIGGER_UPSTREAM_REQUEST ? labels.stages.upstreamRequestTriggered : trigger;
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
    labels.promptTrace.sentSummary(
      promptTraceModeSentence(trace.mode, labels),
      trace.sentMessageCount,
      trace.fullMessageCount,
      trace.sentTokenEstimate,
    ),
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

export function mergePromptTraceStage(stages: TraceStage[], trace: ChatPromptTrace | undefined, labels: ProcessTraceLabels): TraceStage[] {
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
