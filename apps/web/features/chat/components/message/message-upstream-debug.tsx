"use client";

import { Check, Copy } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { Accordion, AccordionContent, AccordionItem } from "@/components/ui/accordion";
import {
  formatUpstreamDebugLines,
  formatUpstreamDebugRaw,
  formatUpstreamDebugSize,
  parseUpstreamDebugBody,
  type UpstreamDebugLine,
} from "@/features/chat/model/upstream-debug-format";
import type { ChatInlineAlert } from "@/features/chat/types/messages";
import { cn } from "@/lib/utils";
import { useCopyAction } from "@/shared/components/copy-action";

type UpstreamDebugViewMode = "formatted" | "raw";
type UpstreamDebugSide = "request" | "response";

const DEBUG_TOOL_BUTTON_CLASSNAME =
  "inline-flex h-5 items-center rounded px-1.5 text-[11px] outline-none transition-colors hover:text-foreground focus-visible:bg-muted";

// UpstreamExchangeDetails shows the redacted upstream request and response behind a failed generation.
export function UpstreamExchangeDetails({
  details,
  open,
  onOpenChange,
}: {
  details?: ChatInlineAlert["details"];
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <Accordion
      type="single"
      collapsible
      value={open ? "upstream-debug" : ""}
      onValueChange={(value) => onOpenChange(value === "upstream-debug")}
      className="w-full min-w-0 max-w-full text-xs text-foreground"
    >
      <AccordionItem value="upstream-debug" className="w-full min-w-0 max-w-full border-b-0">
        <AccordionContent className="w-full min-w-0 max-w-full pb-0 pt-3">
          <UpstreamDebugPanel details={details} />
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  );
}

// UpstreamDebugPanel keeps the request/response switch, view mode and copy in one header bar so the
// panel spends a single row on controls instead of a separate tab strip above it.
function UpstreamDebugPanel({ details }: { details?: ChatInlineAlert["details"] }) {
  const t = useTranslations("chat.messages");
  const [side, setSide] = React.useState<UpstreamDebugSide>("request");
  const [viewMode, setViewMode] = React.useState<UpstreamDebugViewMode>("formatted");
  const request = details?.request;
  const response = details?.response;
  const body = (side === "request" ? request?.body : response?.body) ?? "";
  const label =
    side === "request"
      ? [request?.method?.trim(), request?.path?.trim()].filter(Boolean).join(" ")
      : response?.statusCode
        ? `HTTP ${response.statusCode}`
        : "";
  const lines = React.useMemo(() => formatUpstreamDebugLines(parseUpstreamDebugBody(body)), [body]);
  const raw = React.useMemo(() => formatUpstreamDebugRaw(body), [body]);
  const { copy, copied } = useCopyAction({
    messages: { copied: t("debugCopied"), failed: t("debugCopyFailed") },
  });
  const empty = lines.length === 0;

  return (
    <div className="min-w-0 overflow-hidden rounded-md bg-muted/45">
      <div className="flex min-w-0 items-center gap-2 border-b border-border/40 px-1.5 py-1.5 text-[11px] leading-4 text-muted-foreground">
        <DebugSegmented
          label={t("debugSide")}
          value={side}
          options={[
            { value: "request", label: t("debugRequest") },
            { value: "response", label: t("debugResponse") },
          ]}
          onChange={setSide}
        />
        <span aria-hidden="true" className="h-3 w-px shrink-0 bg-border" />
        <span className="min-w-0 truncate font-mono text-foreground/70">{label}</span>
        {empty ? null : <span className="shrink-0 tabular-nums text-muted-foreground/60">{formatUpstreamDebugSize(body)}</span>}
        {empty ? null : (
          <div className="ml-auto flex shrink-0 items-center gap-1">
            <DebugSegmented
              label={t("debugViewMode")}
              value={viewMode}
              options={[
                { value: "formatted", label: t("debugFormatted") },
                { value: "raw", label: t("debugRaw") },
              ]}
              onChange={setViewMode}
            />
            <button
              type="button"
              aria-label={t("debugCopy")}
              onClick={() => void copy(raw)}
              className={cn(DEBUG_TOOL_BUTTON_CLASSNAME, "size-6 justify-center px-0 text-muted-foreground hover:bg-foreground/[0.05]")}
            >
              {copied ? <Check className="size-3" strokeWidth={1.6} /> : <Copy className="size-3" strokeWidth={1.6} />}
            </button>
          </div>
        )}
      </div>
      <div className="max-h-96 min-w-0 overflow-y-auto px-3 py-2.5 text-[12px] leading-5 break-words [overflow-wrap:anywhere]">
        {empty ? (
          <span className="text-muted-foreground">{t("debugEmpty")}</span>
        ) : viewMode === "raw" ? (
          <pre className="whitespace-pre-wrap font-mono">{raw}</pre>
        ) : (
          groupUpstreamDebugLines(lines).map((group, index) =>
            group.kind === "block" ? (
              <UpstreamDebugBlockView key={index} depth={group.depth} lines={group.lines} />
            ) : (
              <UpstreamDebugLineView key={index} line={group.line} />
            ),
          )
        )}
      </div>
    </div>
  );
}

function DebugSegmented<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: ReadonlyArray<{ value: T; label: string }>;
  onChange: (value: T) => void;
}) {
  return (
    <div role="radiogroup" aria-label={label} className="flex shrink-0 items-center gap-0.5 rounded-md bg-foreground/[0.05] p-0.5">
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          role="radio"
          aria-checked={value === option.value}
          onClick={() => onChange(option.value)}
          className={cn(
            DEBUG_TOOL_BUTTON_CLASSNAME,
            value === option.value
              ? "bg-background text-foreground shadow-xs"
              : "text-muted-foreground hover:text-foreground",
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}

type UpstreamDebugLineGroup =
  | { kind: "line"; line: UpstreamDebugLine }
  | { kind: "block"; depth: number; lines: string[] };

// Consecutive lines of one multi-line string render as a single prose block.
function groupUpstreamDebugLines(lines: UpstreamDebugLine[]): UpstreamDebugLineGroup[] {
  const groups: UpstreamDebugLineGroup[] = [];
  for (const line of lines) {
    const previous = groups.at(-1);
    if (line.block && previous?.kind === "block" && previous.depth === line.depth) {
      previous.lines.push(line.value ?? "");
    } else if (line.block) {
      groups.push({ kind: "block", depth: line.depth, lines: [line.value ?? ""] });
    } else {
      groups.push({ kind: "line", line });
    }
  }
  return groups;
}

function indentStyle(depth: number): React.CSSProperties {
  return { paddingLeft: `${depth}rem` };
}

// Structure (keys, item labels, scalars) stays monospace; long string content such as prompts reads
// as prose behind a guide line, so CJK text and wrapped sentences are not squeezed into a code font.
function UpstreamDebugBlockView({ depth, lines }: { depth: number; lines: string[] }) {
  return (
    <div style={indentStyle(depth)} className="my-0.5">
      <div className="whitespace-pre-wrap border-l-2 border-border/70 pl-2.5 text-[12.5px] leading-relaxed text-foreground/85">
        {lines.join("\n")}
      </div>
    </div>
  );
}

function UpstreamDebugLineView({ line }: { line: UpstreamDebugLine }) {
  return (
    <div style={indentStyle(line.depth)} className="min-h-5 font-mono">
      {line.label ? <span className="text-muted-foreground">{line.label}</span> : null}
      {line.tag ? (
        <span className="ml-1.5 rounded bg-foreground/[0.06] px-1 py-px text-[11px] text-foreground/80">{line.tag}</span>
      ) : null}
      {line.label && line.value !== undefined ? <span className="text-muted-foreground">: </span> : null}
      {line.value === undefined ? null : <span className="whitespace-pre-wrap text-foreground">{line.value}</span>}
    </div>
  );
}
