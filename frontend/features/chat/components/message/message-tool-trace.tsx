"use client";

import * as React from "react";

import { ChevronDown } from "@/components/animate-ui/icons/chevron-down";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Marker, MarkerContent } from "@/components/ui/marker";
import type { ChatTraceBlock } from "@/features/chat/types/messages";
import {
  useProcessTraceLabels,
  type ProcessTraceLabels,
} from "@/features/chat/hooks/use-process-trace-labels";
import { cn } from "@/lib/utils";
import { TRACE_ROOT_CLASS } from "@/features/chat/components/shared/message-process-trace-shared";
import type { TraceDisplayEvent } from "@/features/chat/model/message-process-trace";

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
  input_preview?: string;
  input_detail?: string;
  output?: string;
  output_text?: string;
  output_preview?: string;
  output_detail?: string;
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

function isToolTraceStatusActive(status: string | undefined): boolean {
  return ["requested", "streaming", "queued", "in_progress", "searching"].includes(status?.trim() || "");
}

export function hasActiveToolTraceCalls(payloadJson: string | undefined): boolean {
  return parseToolTraceCalls(payloadJson).some((call) => isToolTraceStatusActive(call.status));
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

function readStringList(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value.map((item) => readString(item)).filter(Boolean);
}

function firstStringFromRecord(record: Record<string, unknown>, keys: string[]): string {
  for (const key of keys) {
    const value = readString(record[key]);
    if (value) return value;
  }
  return "";
}

function firstStringListFromRecord(record: Record<string, unknown>, keys: string[]): string[] {
  for (const key of keys) {
    const values = readStringList(record[key]);
    if (values.length > 0) return values;
  }
  return [];
}

function collectToolStrings(value: unknown, keys: string[], result: string[] = []): string[] {
  if (Array.isArray(value)) {
    value.forEach((item) => {
      collectToolStrings(item, keys, result);
    });
    return result;
  }
  if (!isRecord(value)) return result;
  for (const key of keys) {
    const text = readString(value[key]);
    if (text) result.push(text);
  }
  Object.values(value).forEach((item) => {
    collectToolStrings(item, keys, result);
  });
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
    value.forEach((item) => {
      collectToolImageSources(item, result);
    });
    return Array.from(new Set(result));
  }
  if (!isRecord(value)) return Array.from(new Set(result));
  for (const key of ["url", "uri", "image_url", "b64_json", "base64", "partial_image_b64", "result"]) {
    const source = normalizeImageSource(readString(value[key]));
    if (source) result.push(source);
  }
  Object.values(value).forEach((item) => {
    collectToolImageSources(item, result);
  });
  return Array.from(new Set(result));
}

function normalizeToolName(value: string | undefined): string {
  return value?.trim().replace(/_call_output$/, "").replace(/_call$/, "") || "";
}

function resolveNativeToolKind(call: ToolTraceCall): NativeToolKind {
  const name = normalizeToolName(call.name);
  const type = normalizeToolName(call.type);
  const value = `${name} ${type}`;
  if (value.includes("web_search") || value.includes("google_search") || value.includes("url_context")) return "web_search";
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
  const input = formatToolPayload(call.input_detail) || formatToolPayload(call.input_preview) || formatToolPayload(call.input);
  const output = failed
    ? formatToolPayload(call.error)
    : formatToolPayload(call.output_detail) || formatToolPayload(call.output) || formatToolPayload(call.output_text) || formatToolPayload(call.output_preview);
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


function toolInputRecord(call: ToolTraceCall): Record<string, unknown> {
  const input = toolInputPayload(call);
  return isRecord(input) ? input : {};
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

function toolInputText(call: ToolTraceCall, keys: string[]): string {
  const input = toolInputPayload(call);
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

function geminiWebSearchQuery(output: unknown): string {
  if (!isRecord(output)) return "";
  return firstStringListFromRecord(output, ["queries", "webSearchQueries"]).join(", ");
}

function geminiWebSearchSummary(output: unknown): string {
  if (!isRecord(output)) return "";
  const chunks = Array.isArray(output.groundingChunks) ? output.groundingChunks.length : 0;
  const supports = Array.isArray(output.groundingSupports) ? output.groundingSupports.length : 0;
  const supportCount = readNumber(output.support_count);
  const urls = collectToolStrings(output, ["url", "uri", "retrievedUrl"]);
  const parts = [];
  if (chunks > 0) parts.push(`${chunks} sources`);
  if (supportCount !== null && supportCount > 0) parts.push(`${supportCount} grounding supports`);
  if (supportCount === null && supports > 0) parts.push(`${supports} grounding supports`);
  if (urls.length > 0) parts.push(urls.slice(0, 4).map(safeURLHostname).join(", "));
  return parts.join(" · ");
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
    const query = firstStringFromRecord(input, ["query", "q"]) || firstStringListFromRecord(input, ["queries"]).join(", ") || geminiWebSearchQuery(output) || toolOutputText(call, ["query"]);
    const actionType = firstStringFromRecord(input, ["type", "action"]);
    const urls = collectToolStrings(output, [...urlKeys, "retrievedUrl"]);
    const responseText = urls.length === 0
      ? geminiWebSearchSummary(output) || formatToolPayload(call.output_detail) || formatToolPayload(call.output) || formatToolPayload(call.output_text) || formatToolPayload(call.output_preview)
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
    const language = firstStringFromRecord(input, ["language"]);
    const logs = collectToolStrings(output, ["logs", "stdout", "stderr", "text", "output"]).join("\n\n");
    const artifactURLs = collectToolStrings(output, urlKeys);
    return (
      <div className={cn("space-y-2 text-muted-foreground/84", failed && "text-destructive/80")}>
        <div>{statusText}</div>
        {code ? (
          <div>
            <ToolMiniLabel>{language ? `${labels.tool.detail.code} · ${language}` : labels.tool.detail.code}</ToolMiniLabel>
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
    const command = firstStringFromRecord(input, ["cmd", "command", "input"]) || readString(toolInputPayload(call));
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
        toolInput: toolInputValue(call),
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
      toolInput: toolInputValue(call),
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

export function MessageToolChainTrace({
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
            iconPosition="none"
            className="group/tool-chain min-h-0 justify-between gap-1.5 py-0.5 text-left no-underline hover:no-underline"
          >
            <div className="min-w-0 flex-1">
              <div className="flex items-center">
                <Marker
                  render={<span />}
                  className={cn(
                    "inline-flex min-h-0 w-auto text-[13px] font-medium transition-colors",
                    !streaming && "text-muted-foreground group-hover/tool-chain:text-foreground",
                  )}
                >
                  <MarkerContent className={cn("min-w-0", streaming && "shimmer")}>
                    {streaming ? labels.tool.chain.titleActive : labels.tool.chain.titleDone}
                  </MarkerContent>
                </Marker>
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
          <AccordionContent className="px-0 pb-0 pt-1.5 duration-[350ms] ease-in-out">
            <ToolChainRows steps={steps} labels={labels} />
          </AccordionContent>
        </AccordionItem>
      </Accordion>
    </div>
  );
}
