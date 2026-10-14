"use client";

import { parseBlockSource, scanUIBlockFences, widgetKey, widgetTitle } from "@deeix/core";
import { X } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { Button } from "@/components/ui/button";
import type { ChatUIBlockStateStore } from "@/features/chat/model/chat-ui-block-state-store";
import type { ChatAreaMessage } from "@/features/chat/types/messages";
import { UIBlockHost, UIWidgetHost } from "@/shared/components/markdown/ui-blocks";
import { UIBlockStateProvider, useMessageUIBlockStateBinding } from "@/shared/components/markdown/ui-blocks/ui-block-state";

type ChatUIBlockPanelProps = {
  // Undefined once the message is no longer shown (other branch, deleted).
  message: ChatAreaMessage | undefined;
  blockKey: string;
  store: ChatUIBlockStateStore;
  onClose: () => void;
};

// An interactive block of a reply, opened in the side panel so it stays in view
// while the conversation goes on. It reads and writes the message's block state
// through the same store as the inline copy, so both always show the same.
export function ChatUIBlockPanel({ message, blockKey, store, onClose }: ChatUIBlockPanelProps) {
  const t = useTranslations("chat.markdown.uiBlock");
  const content = message?.content ?? "";
  const located = React.useMemo(() => findBlock(content, blockKey), [blockKey, content]);
  const serverStateRef = React.useRef(message?.uiState);
  serverStateRef.current = message?.uiState;
  const messageKey = message?.key ?? "";
  const binding = React.useMemo(
    () => (messageKey ? store.bindingFor(messageKey, () => serverStateRef.current) : undefined),
    [messageKey, store],
  );
  const guardedBinding = useMessageUIBlockStateBinding(binding, content);

  // The block left the conversation on screen (branch switch, deletion, edit).
  const missing = !located && !message?.isStreaming;
  React.useEffect(() => {
    if (missing) {
      onClose();
    }
  }, [missing, onClose]);

  const title = located?.title || t("panelTitle");
  const host = !located ? null : located.kind === "widget" ? (
    <UIWidgetHost raw={located.raw} complete />
  ) : (
    <UIBlockHost raw={located.raw} streaming={Boolean(message?.isStreaming)} />
  );
  return (
    <aside className="flex h-full min-h-0 w-full flex-col border-l border-border/55 bg-background" aria-label={title}>
      <div className="flex h-12 shrink-0 items-center justify-between gap-2 border-b border-border/40 px-3">
        <h2 className="min-w-0 truncate text-sm font-semibold tracking-tight">{title}</h2>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="size-7 shrink-0 rounded-md text-muted-foreground hover:bg-foreground/[0.04] hover:text-foreground"
          aria-label={t("closePanel")}
          title={t("closePanel")}
          onClick={onClose}
        >
          <X className="size-3" />
        </Button>
      </div>
      <div className="min-h-0 flex-1 overflow-auto px-4">
        {guardedBinding && host ? <UIBlockStateProvider binding={guardedBinding}>{host}</UIBlockStateProvider> : host}
      </div>
    </aside>
  );
}

function findBlock(content: string, blockKey: string): { kind: "block" | "widget"; raw: string; title: string } | null {
  for (const fence of scanUIBlockFences(content)) {
    if (!fence.closed) {
      continue;
    }
    if (fence.kind === "widget") {
      if (widgetKey(fence.body) === blockKey) {
        return { kind: "widget", raw: fence.body, title: widgetTitle(fence.body) };
      }
      continue;
    }
    const parsed = parseBlockSource(fence.body, false);
    if (parsed.status === "ok" && parsed.block.key === blockKey) {
      const title = parsed.block.root.props.title;
      return { kind: "block", raw: fence.body, title: typeof title === "string" ? title.trim() : "" };
    }
  }
  return null;
}
