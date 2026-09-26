"use client";

import * as React from "react";

import { ArrowUpRight, Check, Copy, Wrench } from "lucide-react";

import { AgentTraceStep } from "@/features/chat/components/message/message-agent-trace-step";
import { MessageToolResultDialog } from "@/features/chat/components/message/message-tool-result-dialog";
import { useCopyAction } from "@/shared/components/copy-action";
import { useAutoExpandDisclosure } from "@/shared/hooks/use-auto-expand-disclosure";
import { useChatElapsedDurationMS } from "@/features/chat/hooks/use-chat-elapsed-duration";
import type { ChatTraceBlock } from "@/features/chat/types/messages";
import type { ProcessTraceLabels } from "@/features/chat/hooks/use-chat-trace-labels";
import { cn } from "@/lib/utils";
import { formatDurationMS } from "@/features/chat/model/duration";
import type { TraceDisplayEvent } from "@/features/chat/model/message-process-trace";
import { StreamdownRender } from "@/shared/components/markdown/streamdown-render";
import {
  collectToolImageSources,
  collectToolSearchSources,
  collectToolStrings,
  countToolPayloadItems,
  firstToolPayloadString,
  formatToolPayload,
  isToolPayloadRecord,
  parseToolPayload,
  readToolPayloadNumber,
  readToolPayloadString,
  resolveToolResultCategory,
  type ToolResultCategory,
  type ToolSearchSource,
} from "@/features/chat/model/tool-result-presentation";

type ToolTraceCall = {
  tool_call_id?: string;
  id?: string;
  call_id?: string;
  detail_run_id?: string;
  name?: string;
  type?: string;
  status?: string;
  latency_ms?: number;
  error?: string;
  input?: string;
  input_preview?: string;
  input_detail?: string;
  output?: string;
  output_text?: string;
  output_preview?: string;
  output_detail?: string;
  output_presentation?: unknown;
  input_size?: number;
};

function parseToolTraceCalls(payloadJson: string | undefined): ToolTraceCall[] {
  if (!payloadJson) return [];
  try {
    const parsed = JSON.parse(payloadJson) as unknown;
    if (!isToolPayloadRecord(parsed) || !Array.isArray(parsed.tool_calls)) return [];

    return parsed.tool_calls.flatMap((value): ToolTraceCall[] => {
      if (!isToolPayloadRecord(value)) return [];
      return [
        {
          tool_call_id: readToolPayloadString(value.tool_call_id) || undefined,
          id: readToolPayloadString(value.id) || undefined,
          call_id: readToolPayloadString(value.call_id) || undefined,
          detail_run_id: readToolPayloadString(value.detail_run_id) || undefined,
          name: readToolPayloadString(value.name) || undefined,
          type: readToolPayloadString(value.type) || undefined,
          status: readToolPayloadString(value.status) || undefined,
          latency_ms: readToolPayloadNumber(value.latency_ms) ?? undefined,
          error: readToolPayloadString(value.error) || undefined,
          input: readToolPayloadString(value.input) || undefined,
          input_preview: readToolPayloadString(value.input_preview) || undefined,
          input_detail: readToolPayloadString(value.input_detail) || undefined,
          output: readToolPayloadString(value.output) || undefined,
          output_text: readToolPayloadString(value.output_text) || undefined,
          output_preview: readToolPayloadString(value.output_preview) || undefined,
          output_detail: readToolPayloadString(value.output_detail) || undefined,
          output_presentation: value.output_presentation,
          input_size: readToolPayloadNumber(value.input_size) ?? undefined,
        },
      ];
    });
  } catch {
    return [];
  }
}

function normalizeToolTraceStatus(status: string | undefined): string {
  return status?.trim().toLowerCase() || "";
}

function isToolTraceStatusActive(status: string | undefined): boolean {
  return ["requested", "streaming", "queued", "in_progress", "searching"].includes(
    normalizeToolTraceStatus(status),
  );
}

function isToolTraceStatusFailed(status: string | undefined): boolean {
  return ["error", "failed"].includes(normalizeToolTraceStatus(status));
}

function toolResultCategory(call: ToolTraceCall) {
  return resolveToolResultCategory({
    name: call.name,
    type: call.type,
    input: toolInputPayload(call),
    output: call.output_presentation ?? toolOutputPayload(call),
  });
}

function toolStatusLabel(status: string | undefined, labels: ProcessTraceLabels): string {
  switch (normalizeToolTraceStatus(status)) {
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

function toolTraceCallLabel(call: ToolTraceCall, category: ToolResultCategory, labels: ProcessTraceLabels): string {
  switch (category) {
    case "web_search":
      return labels.tool.names.webSearch;
    case "code_execution":
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
  const failed = isToolTraceStatusFailed(call.status);
  const input = formatToolPayload(call.input_detail) || formatToolPayload(call.input_preview) || formatToolPayload(call.input);
  const output = failed
    ? formatToolPayload(call.error)
    : formatToolPayload(call.output_detail) || formatToolPayload(call.output) || formatToolPayload(call.output_text) || formatToolPayload(call.output_preview);
  const parts = [toolStatusLabel(call.status, labels)].filter(Boolean);

  if (input) {
    parts.push(`${labels.tool.detail.request}\n${input}`);
  }
  if (output) {
    parts.push(`${failed ? labels.tool.detail.error : labels.tool.detail.response}\n${output}`);
  }

  return { detail: parts.join("\n"), failed };
}

function ToolPre({ children, failed }: { children: string; failed?: boolean }) {
  if (!children.trim()) return null;
  return (
    <pre
      className={cn(
        "max-h-[7.25rem] overflow-auto rounded-md bg-background/48 px-2.5 py-2 font-mono text-[11px] leading-5",
        "whitespace-pre-wrap break-words text-muted-foreground/88",
        failed && "bg-destructive/5 text-destructive/85",
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

function normalizeToolLink(value: string): string {
  const url = value.trim();
  if (url.startsWith("/")) return url;
  try {
    const parsed = new URL(url);
    return parsed.protocol === "http:" || parsed.protocol === "https:" ? url : "";
  } catch {
    return "";
  }
}

function ToolSearchSources({ sources, labels }: { sources: ToolSearchSource[]; labels: ProcessTraceLabels }) {
  const unique = sources
    .flatMap((source) => {
      const url = normalizeToolLink(source.url);
      return url ? [{ ...source, url }] : [];
    })
    .slice(0, 8);
  if (unique.length === 0) return null;
  return (
    <div className="divide-y divide-border/20">
      {unique.map((source, index) => (
        <a
          key={`${source.url}-${index}`}
          href={source.url}
          target="_blank"
          rel="noreferrer"
          className="group/tool-source flex min-w-0 items-center gap-2 py-2 transition-colors"
          title={source.url}
        >
          <span className="min-w-0 flex-1">
            <span className="block truncate text-[12px] font-medium leading-5 text-foreground/80 transition-colors group-hover/tool-source:text-foreground">
              {source.title || safeURLHostname(source.url) || labels.tool.detail.sourceFallback(index + 1)}
            </span>
            <span className="block min-w-0 truncate text-[11px] leading-4 text-muted-foreground/62">
              {[safeURLHostname(source.url), source.snippet].filter(Boolean).join(" · ")}
            </span>
          </span>
          <ArrowUpRight className="size-3 shrink-0 text-muted-foreground/38 transition-colors group-hover/tool-source:text-muted-foreground/72" />
        </a>
      ))}
    </div>
  );
}

function ToolReadableText({ children }: { children: string }) {
  const markdown = children.trim();
  if (!markdown) return null;
  return (
    <div className="max-h-40 overflow-auto pr-1 text-[12px] leading-5 text-muted-foreground/82">
      <StreamdownRender
        content={markdown}
        variant="thinking"
        className="[&_ol]:my-0.5 [&_ul]:my-0.5 [&_li]:pl-0 [&_li]:leading-5"
      />
    </div>
  );
}

function ToolArtifactLinks({ urls, labels }: { urls: string[]; labels: ProcessTraceLabels }) {
  const unique = Array.from(new Set(urls.map(normalizeToolLink).filter(Boolean))).slice(0, 8);
  if (unique.length === 0) return null;
  return (
    <div className="flex flex-wrap gap-1.5">
      {unique.map((url, index) => (
        <a
          key={`${url}-${index}`}
          href={url}
          target="_blank"
          rel="noreferrer"
          className="max-w-[220px] truncate rounded-full bg-background/55 px-2 py-0.5 text-[11px] font-medium text-muted-foreground/78 transition-colors hover:text-foreground"
          title={url}
        >
          {safeURLHostname(url) || labels.tool.detail.sourceFallback(index + 1)}
        </a>
      ))}
    </div>
  );
}

function ToolPreviewImage({ src, alt }: { src: string; alt: string }) {
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

function toolInputPayload(call: ToolTraceCall): unknown {
  return parseToolPayload(call.input_detail) ?? parseToolPayload(call.input_preview) ?? parseToolPayload(call.input);
}

function toolInputValue(call: ToolTraceCall): string {
  return call.input_detail?.trim() || call.input_preview?.trim() || call.input?.trim() || "";
}

function toolOutputPayload(call: ToolTraceCall): unknown {
  return parseToolPayload(call.output_detail) ?? parseToolPayload(call.output) ?? parseToolPayload(call.output_text) ?? parseToolPayload(call.output_preview);
}

function toolOutputPayloads(call: ToolTraceCall): unknown[] {
  const values = [
    call.output_presentation,
    parseToolPayload(call.output_detail),
    parseToolPayload(call.output),
    parseToolPayload(call.output_text),
    parseToolPayload(call.output_preview),
  ];
  return values.filter((value): value is unknown => value !== null && value !== undefined);
}

function toolSearchSources(call: ToolTraceCall): ToolSearchSource[] {
  const sources = new Map<string, ToolSearchSource>();
  for (const payload of toolOutputPayloads(call)) {
    for (const source of collectToolSearchSources(payload)) {
      const current = sources.get(source.url);
      sources.set(source.url, {
        url: source.url,
        title: current?.title || source.title,
        snippet: current?.snippet || source.snippet,
      });
    }
  }
  return Array.from(sources.values());
}

function toolSearchNarrative(call: ToolTraceCall): string {
  const text = toolDisplayText(call);
  return text && collectToolSearchSources(text).length === 0 ? text : "";
}

function toolOutputText(call: ToolTraceCall, keys: string[]): string {
  const output = toolOutputPayload(call);
  if (isToolPayloadRecord(output)) {
    return firstToolPayloadString(output, keys);
  }
  return readToolPayloadString(output);
}

function hasToolArguments(call: ToolTraceCall): boolean {
  const input = toolInputPayload(call);
  return isToolPayloadRecord(input) && Object.keys(input).length > 0;
}

function toolDisplayText(call: ToolTraceCall): string {
  const presentation = call.output_presentation;
  if (isToolPayloadRecord(presentation)) {
    const text = readToolPayloadString(presentation.text);
    if (text) return text;
  }
  return readToolPayloadString(parseToolPayload(call.output_preview));
}

function toolSearchSummary(output: unknown, sources: ToolSearchSource[], labels: ProcessTraceLabels): string {
  const supports = countToolPayloadItems(output, ["groundingSupports", "support_count"]);
  const parts = [];
  if (sources.length > 0) parts.push(labels.tool.detail.sourceCount(sources.length));
  if (supports > 0) parts.push(labels.tool.detail.groundingSupportCount(supports));
  return parts.join(" · ");
}

export type ToolChainStep = {
  key: string;
  label: string;
  detail: string;
  failed: boolean;
  startedAt?: string;
  latencyMS?: number;
  toolCallID?: string;
  toolType?: string;
  toolName?: string;
  toolInput?: string;
  toolStatus?: string;
  toolCategory?: ToolResultCategory;
  toolCall?: ToolTraceCall;
};

function toolTraceCallID(call: ToolTraceCall): string {
  return call.tool_call_id?.trim() || call.id?.trim() || call.call_id?.trim() || "";
}

function toolTraceCallKey(call: ToolTraceCall): string {
  const callID = toolTraceCallID(call);
  if (callID) {
    return `id:${callID}`;
  }

  const toolKind = [call.type, call.name]
    .map((value) => value?.trim().toLowerCase() || "")
    .join(":");
  const input = call.input_preview?.trim() || call.input?.trim() || call.input_detail?.trim() || "";
  const boundedInput = input.slice(0, 256);
  const inputSize = call.input_size ?? input.length;
  return boundedInput
    ? `kind:${toolKind}:input:${inputSize}:${boundedInput}`
    : `kind:${toolKind}`;
}

function toolTraceFallbackKey(
  source: Pick<ChatTraceBlock, "roundID" | "parentEventID" | "title">,
): string {
  const scope = source.roundID?.trim()
    || source.parentEventID?.trim()
    || source.title?.trim().toLowerCase()
    || "generic";
  return `fallback:${scope}`;
}

function toolTraceStatusRank(status: string | undefined): number {
  switch (normalizeToolTraceStatus(status)) {
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
  if (left.key === right.key) return true;
  if (left.toolCallID && right.toolCallID) return left.toolCallID === right.toolCallID;
  const leftName = left.toolName?.trim() || "";
  const rightName = right.toolName?.trim() || "";
  const leftType = left.toolType?.trim() || "";
  const rightType = right.toolType?.trim() || "";
  const sameKind = leftName && rightName ? leftName === rightName : Boolean(leftType && rightType && leftType === rightType);
  if (!sameKind) return false;
  const leftInput = left.toolInput?.trim() || "";
  const rightInput = right.toolInput?.trim() || "";
  if (leftInput && rightInput) return leftInput === rightInput;

  const leftDetail = left.detail.trim();
  const rightDetail = right.detail.trim();
  return Boolean(leftDetail && rightDetail && leftDetail === rightDetail);
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
      result[existingIndex] = { ...step, key: current.key };
    }
  }
  return result;
}

export function buildToolGroupSteps(
  toolEvents: TraceDisplayEvent[],
  toolBlock: ChatTraceBlock | undefined,
  labels: ProcessTraceLabels,
): ToolChainStep[] {
  return dedupeToolChainSteps([
    ...buildToolChainSteps(toolEvents, labels),
    ...buildToolChainStepsFromBlock(toolBlock, labels),
  ]);
}

export function buildToolChainSteps(events: TraceDisplayEvent[], labels: ProcessTraceLabels): ToolChainStep[] {
  return events.flatMap<ToolChainStep>((item) => {
    const event = item.event;
    if (item.kind !== "tool") {
      return [];
    }

    const calls = parseToolTraceCalls(event.payloadJson);
    if (calls.length === 0) {
      return [
        {
          key: toolTraceFallbackKey(event),
          label: labels.tool.names.generic,
          detail: event.contentMarkdown?.trim() || event.summary?.trim() || event.title?.trim() || "",
          failed: isToolTraceStatusFailed(event.status),
          startedAt: event.startedAt,
          toolStatus: event.status?.trim(),
        },
      ];
    }

    return calls.map((call) => {
      const category = toolResultCategory(call);
      const label = toolTraceCallLabel(call, category, labels);
      const { detail, failed } = toolTraceCallDetail(call, labels);
      return {
        key: toolTraceCallKey(call),
        label,
        detail,
        failed,
        startedAt: event.startedAt,
        latencyMS: call.latency_ms,
        toolCallID: toolTraceCallID(call),
        toolType: call.type?.trim(),
        toolName: call.name?.trim(),
        toolInput: toolInputValue(call),
        toolStatus: call.status?.trim() || event.status?.trim(),
        toolCategory: category,
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
        key: toolTraceFallbackKey(block),
        label: labels.tool.names.generic,
        detail,
        failed: isToolTraceStatusFailed(block.status),
        startedAt: block.startedAt,
        toolStatus: block.status?.trim(),
      },
    ];
  }
  return calls.map((call) => {
    const category = toolResultCategory(call);
    const label = toolTraceCallLabel(call, category, labels);
    const { detail, failed } = toolTraceCallDetail(call, labels);
    return {
      key: toolTraceCallKey(call),
      label,
      detail,
      failed,
      startedAt: block.startedAt,
      latencyMS: call.latency_ms,
      toolCallID: toolTraceCallID(call),
      toolType: call.type?.trim(),
      toolName: call.name?.trim(),
      toolInput: toolInputValue(call),
      toolStatus: call.status?.trim() || block.status?.trim(),
      toolCategory: category,
      toolCall: call,
    };
  });
}

export function isToolChainStepActive(step: ToolChainStep): boolean {
  return isToolTraceStatusActive(step.toolCall?.status) || isToolTraceStatusActive(step.toolStatus);
}

function isToolStepDone(step: ToolChainStep): boolean {
  const status = normalizeToolTraceStatus(step.toolCall?.status || step.toolStatus);
  return status === "success" || status === "completed" || status === "reused";
}

function ToolStepStatusIcon({ step }: { step: ToolChainStep }) {
  return (
    <Wrench
      className={cn(
        "size-3 text-muted-foreground/68",
        isToolChainStepActive(step) && "text-foreground/78",
        step.failed && "text-destructive",
      )}
    />
  );
}

function formatArgumentValue(value: unknown): string {
  try {
    if (Array.isArray(value)) {
      return value.map((item) => (typeof item === "object" ? JSON.stringify(item) : String(item))).join(", ");
    }
    if (typeof value === "object" && value !== null) {
      return Object.entries(value)
        .map(([key, item]) => `${key}: ${typeof item === "object" ? JSON.stringify(item) : String(item)}`)
        .join(", ");
    }
  } catch {
    return String(value);
  }
  return String(value);
}

function ToolArgumentCopyButton({ value, labels }: { value: string; labels: ProcessTraceLabels }) {
  const { copy, isCopied } = useCopyAction({
    messages: { copied: labels.tool.detail.copied, failed: labels.tool.detail.copyFailed },
  });
  const done = isCopied(value);
  return (
    <button
      type="button"
      title={labels.tool.detail.copy}
      aria-label={labels.tool.detail.copy}
      className={cn(
        "shrink-0 rounded p-1 text-muted-foreground/50 transition-colors hover:bg-muted/50 hover:text-foreground",
        "opacity-0 focus-visible:opacity-100 group-hover/tool-arg-row:opacity-100",
      )}
      onClick={(event) => {
        event.stopPropagation();
        void copy(value, { key: value });
      }}
    >
      {done ? <Check className="size-3 text-emerald-600 dark:text-emerald-400" /> : <Copy className="size-3" />}
    </button>
  );
}

function ToolArgumentsCard({ call, labels }: { call: ToolTraceCall; labels: ProcessTraceLabels }) {
  const input = toolInputPayload(call);
  if (!isToolPayloadRecord(input) || Object.keys(input).length === 0) {
    return null;
  }

  const argumentsList = Object.entries(input).map(([key, value]) => {
    const text = formatArgumentValue(value);
    const compact = (value === null || ["string", "number", "boolean"].includes(typeof value))
      && !text.includes("\n")
      && text.length <= 48;
    return { key, text, compact };
  });
  const compactArguments = argumentsList.filter((argument) => argument.compact);
  const expandedArguments = argumentsList.filter((argument) => !argument.compact);

  return (
    <div className="space-y-1.5 pb-1 pt-1" aria-label={labels.tool.detail.argumentsTitle}>
      {compactArguments.length > 0 ? (
        <div className="flex flex-wrap gap-1.5">
          {compactArguments.map(({ key, text }) => (
            <div
              key={key}
              className="group/tool-arg-row inline-flex min-w-0 max-w-full items-center gap-1 rounded bg-muted/30 px-1.5 py-0.5 leading-4"
            >
              <code className="shrink-0 font-mono text-[10px] text-muted-foreground/58">{key}</code>
              <span className="max-w-48 truncate text-[11px] text-foreground/72" title={text}>{text}</span>
              <ToolArgumentCopyButton value={`${key}: ${text}`} labels={labels} />
            </div>
          ))}
        </div>
      ) : null}
      {expandedArguments.length > 0 ? (
        <div className="space-y-1.5">
          {expandedArguments.map(({ key, text }) => (
            <div key={key} className="group/tool-arg-row min-w-0">
              <div className="flex min-h-5 items-center gap-1">
                <code className="min-w-0 truncate font-mono text-[11px] text-muted-foreground/58">{key}</code>
                <ToolArgumentCopyButton value={`${key}: ${text}`} labels={labels} />
              </div>
              <ToolPre>{text}</ToolPre>
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function ToolResultCard({
  call,
  category,
  labels,
  divided,
}: {
  call: ToolTraceCall;
  category: ToolResultCategory;
  labels: ProcessTraceLabels;
  divided: boolean;
}) {
  const failedStatus = isToolTraceStatusFailed(call.status);
  const output = toolOutputPayload(call);
  let content: React.ReactNode = null;
  let meta = "";

  if (failedStatus) {
    const errorText = call.error?.trim();
    if (errorText) {
      content = <ToolPre failed>{errorText}</ToolPre>;
    }
  } else if (category === "web_search") {
    const sources = toolSearchSources(call);
    const narrative = toolSearchNarrative(call);
    const summary = toolSearchSummary(output, sources, labels);
    meta = summary;
    if (sources.length > 0 || narrative) {
      content = (
        <>
          {narrative ? <ToolReadableText>{narrative}</ToolReadableText> : null}
          {sources.length > 0 ? <ToolSearchSources sources={sources} labels={labels} /> : null}
        </>
      );
    }
  } else if (category === "code_execution") {
    const logs = collectToolStrings(output, ["logs", "stdout", "stderr", "text", "output"]).join("\n\n");
    const artifactURLs = collectToolStrings(output, ["url", "uri", "image_url"]);
    if (logs || artifactURLs.length > 0) {
      content = (
        <>
          {logs ? <ToolPre failed={failedStatus}>{logs}</ToolPre> : null}
          {artifactURLs.length > 0 ? <ToolArtifactLinks urls={artifactURLs} labels={labels} /> : null}
        </>
      );
    } else {
      const text = toolDisplayText(call);
      content = text ? <ToolPre>{text}</ToolPre> : null;
    }
  } else if (category === "image_generation") {
    const urls = collectToolImageSources(output);
    if (urls.length > 0) {
      content = <ToolImageGrid urls={urls} labels={labels} />;
    } else {
      const text = toolDisplayText(call);
      content = text ? <ToolPre>{text}</ToolPre> : null;
    }
  } else if (category === "shell") {
    const stdout = toolOutputText(call, ["stdout", "output"]);
    const stderr = toolOutputText(call, ["stderr", "error"]);
    const exitCode = isToolPayloadRecord(output)
      ? readToolPayloadNumber(output.exit_code) ?? readToolPayloadNumber(output.code)
      : null;
    if (stdout || stderr || exitCode !== null) {
      content = (
        <>
          {stdout ? <ToolPre>{stdout}</ToolPre> : null}
          {stderr ? <ToolPre failed>{stderr}</ToolPre> : null}
          {exitCode !== null ? <div className="text-[11px] text-muted-foreground/62">{labels.tool.detail.exitCode(exitCode)}</div> : null}
        </>
      );
    } else {
      const text = toolDisplayText(call);
      content = text ? <ToolPre>{text}</ToolPre> : null;
    }
  } else {
    const text = toolDisplayText(call);
    if (text) {
      content = <ToolReadableText>{text}</ToolReadableText>;
    }
  }

  if (!content) {
    return null;
  }
  const showHeading = category !== "generic" || Boolean(meta);
  return (
    <div className={cn("pb-1 pt-1", divided && "mt-1.5 pt-1.5")}>
      {showHeading ? (
        <div className="mb-1 flex items-center gap-1.5 text-[11px] leading-4 text-muted-foreground/58">
          <span className="font-medium">{labels.tool.detail.resultTitle}</span>
          {meta ? <span>· {meta}</span> : null}
        </div>
      ) : null}
      <div className="space-y-2 text-muted-foreground/84">{content}</div>
    </div>
  );
}

function ToolCallDetailCard({
  step,
  labels,
  runID,
  allowFullResult,
}: {
  step: ToolChainStep;
  labels: ProcessTraceLabels;
  runID?: string;
  allowFullResult: boolean;
}) {
  const call = step.toolCall;
  const hasArguments = Boolean(call && hasToolArguments(call));
  const toolName = step.toolName?.trim() || step.label;
  const toolCallID = call ? toolTraceCallID(call) : "";
  const detailRunID = call?.detail_run_id?.trim() || runID?.trim() || "";
  const canViewFullResult = Boolean(allowFullResult && detailRunID && toolCallID);
  return (
    <div className="w-full max-w-[48rem]">
      <div className="flex min-h-6 min-w-0 items-center gap-2 px-0.5">
        <span className="min-w-0 flex-1 truncate text-[12px] leading-5 text-muted-foreground/78" title={toolName}>
          {labels.tool.detail.nameLabel} · {toolName}
        </span>
        {canViewFullResult ? (
          <MessageToolResultDialog
            runID={detailRunID}
            toolCallID={toolCallID}
            toolName={toolName}
          />
        ) : null}
      </div>
      <div className="min-w-0 px-0.5">
        {call ? (
          <>
            <ToolArgumentsCard call={call} labels={labels} />
            <ToolResultCard
              call={call}
              category={step.toolCategory || toolResultCategory(call)}
              labels={labels}
              divided={hasArguments}
            />
          </>
        ) : step.detail ? (
          <div className="whitespace-pre-wrap break-words py-1 text-[12px] leading-5 text-muted-foreground/84">{step.detail}</div>
        ) : null}
      </div>
    </div>
  );
}

export function AgentToolStepRow({
  step,
  labels,
  autoExpand,
  runID,
  allowFullResult = false,
}: {
  step: ToolChainStep;
  labels: ProcessTraceLabels;
  autoExpand: boolean;
  runID?: string;
  allowFullResult?: boolean;
}) {
  const active = isToolChainStepActive(step);
  const { open, onOpenChange } = useAutoExpandDisclosure({ active, autoExpand });
  const failed = step.failed;
  const statusText = isToolStepDone(step) ? "" : toolStatusLabel(step.toolCall?.status ?? step.toolStatus, labels);
  const expandable = Boolean(step.toolCall || step.detail);
  const liveDurationMS = useChatElapsedDurationMS(active, step.startedAt);
  const durationText = formatDurationMS(active ? liveDurationMS : step.latencyMS);

  return (
    <li className="group/agent-trace-step">
      <AgentTraceStep
        icon={<ToolStepStatusIcon step={step} />}
        title={labels.tool.names.generic}
        status={statusText || undefined}
        duration={durationText}
        open={open}
        expandable={expandable}
        failed={failed}
        loading={active}
        onOpenChange={onOpenChange}
      >
        <ToolCallDetailCard
          step={step}
          labels={labels}
          runID={runID}
          allowFullResult={allowFullResult}
        />
      </AgentTraceStep>
    </li>
  );
}
