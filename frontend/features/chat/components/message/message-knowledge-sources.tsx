"use client";

import { BookOpenText, ChevronDown } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { RAGCitationList } from "@/features/chat/components/shared/message-process-trace-shared";
import { useChatTraceLabels } from "@/features/chat/hooks/use-chat-trace-labels";
import { parseRAGCitations } from "@/features/chat/model/message-process-trace";
import type { ChatMessageProcessTrace, RAGCitation } from "@/features/chat/types/messages";

export function MessageKnowledgeSources({
  trace,
  sources,
  streaming,
}: {
  trace?: ChatMessageProcessTrace;
  sources?: RAGCitation[];
  streaming: boolean;
}) {
  const t = useTranslations("chat.messages");
  const labels = useChatTraceLabels();
  const citations = React.useMemo(
    () => (sources && sources.length > 0 ? sources : parseRAGCitations(trace?.process?.payloadJson)),
    [sources, trace?.process?.payloadJson],
  );
  const sourceCount = React.useMemo(
    () => new Set(citations.map((item) => item.file_id?.trim() || item.file_name?.trim()).filter(Boolean)).size,
    [citations],
  );

  if (streaming || citations.length === 0 || sourceCount === 0) {
    return null;
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="mt-2 h-7 gap-1.5 rounded-full px-2 text-[11px] font-normal text-muted-foreground shadow-none hover:bg-muted/55 hover:text-foreground"
        >
          <BookOpenText className="size-3.5" strokeWidth={1.6} />
          {t("knowledgeSources", { count: sourceCount })}
          <ChevronDown className="size-3 text-muted-foreground/70" strokeWidth={1.6} />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        side="top"
        align="start"
        sideOffset={6}
        className="max-h-[min(28rem,70vh)] w-[min(26rem,calc(100vw-2rem))] overflow-y-auto p-2.5"
      >
        <RAGCitationList citations={citations} labels={labels} className="border-t-0 pt-0" showScores={false} />
      </PopoverContent>
    </Popover>
  );
}
