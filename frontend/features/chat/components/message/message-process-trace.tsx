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
import type { ChatMessageProcessTrace } from "@/features/chat/types/messages";
import { useProcessTraceLabels } from "@/features/chat/hooks/use-process-trace-labels";
import { cn } from "@/lib/utils";
import {
  RAGCitationList,
  TRACE_ROOT_CLASS,
  TraceContent,
} from "@/features/chat/components/shared/message-process-trace-shared";
import {
  filterProcessTraceStages,
  isRAGTraceStage,
  localizeProcessSummary,
  mergePromptTraceStage,
  parseFileContextBadges,
  parseRAGCitations,
  parseStructuredTraceStages,
  parseTraceStages,
} from "@/features/chat/model/message-process-trace";

export { MessageTraceEventBlocks, MessageUpstreamThink } from "@/features/chat/components/message/message-thinking-trace";

function buildProcessSummary(trace: ChatMessageProcessTrace): string {
  if (trace.process?.summary) {
    return trace.process.summary;
  }
  return "";
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
            iconPosition="none"
            className="group/trace min-h-0 justify-between gap-1.5 py-0.5 text-left no-underline hover:no-underline"
          >
            <div className="min-w-0 flex-1">
              <div className="flex items-center">
                <Marker
                  render={<span />}
                  className={cn(
                    "inline-flex min-h-0 w-auto text-[13px] font-medium transition-colors",
                    !processStreaming && "text-muted-foreground group-hover/trace:text-foreground",
                  )}
                >
                  <MarkerContent className={cn("min-w-0", processStreaming && "shimmer")}>
                    {processStreaming ? labels.process.titleActive : labels.process.titleDone}
                  </MarkerContent>
                </Marker>
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
          <AccordionContent className="space-y-2.5 px-0 pb-0 pt-1.5 duration-[350ms] ease-in-out">
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
