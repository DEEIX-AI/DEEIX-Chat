"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import { buildMediaImagePreviewMarkdown } from "@/features/chat/model/media-image-preview";
import { upsertLiveUpstreamThinkTrace } from "@/features/chat/model/upstream-think-store";
import { cancelMessageGeneration, listMessagesPage, resumeMessageGenerationStream } from "@/shared/api/conversation";
import type { MessageDTO } from "@/shared/api/conversation.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

const MESSAGE_PAGE_SIZE = 100;

type ChatDataState = {
  conversationPublicID: string;
  loading: boolean;
  loadingOlder: boolean;
  errorMsg: string;
  messages: MessageDTO[];
  total: number;
  hasOlder: boolean;
};

type ActiveResumeStream = {
  controller: AbortController;
  runID: string;
  accessToken: string | null;
};

export function useChatData(
  conversationID: string | null,
  {
    activeGenerationRunsRef,
    activeGenerationRunsRevision = 0,
    onConversationRunFinished,
  }: {
    activeGenerationRunsRef?: React.RefObject<Set<string>>;
    activeGenerationRunsRevision?: number;
    onConversationRunFinished?: (runID: string) => void;
  } = {},
) {
  const t = useTranslations("chat.data");
  const tSubmit = useTranslations("chat.submit");
  const [state, setState] = React.useState<ChatDataState>({
    conversationPublicID: conversationID ?? "",
    loading: Boolean(conversationID),
    loadingOlder: false,
    errorMsg: "",
    messages: [],
    total: 0,
    hasOlder: false,
  });
  const [reloadToken, setReloadToken] = React.useState(0);
  const [resumingRunID, setResumingRunID] = React.useState("");
  const [resumingActivityLabel, setResumingActivityLabel] = React.useState("");
  const stateRef = React.useRef(state);
  stateRef.current = state;
  const previousConversationIDRef = React.useRef<string | null>(conversationID);
  const resumeSeqByRunRef = React.useRef<Record<string, number>>({});
  const pendingAssistantContentRef = React.useRef("");
  const resumedTextByRunRef = React.useRef<Record<string, string>>({});
  const activeResumeStreamRef = React.useRef<ActiveResumeStream | null>(null);
  // 恢复游标只在对应的可见内容仍被保留时有效，两者必须同步清理。
  const clearResumeCheckpoint = React.useCallback((runID: string) => {
    const normalizedRunID = runID.trim();
    if (!normalizedRunID) {
      return;
    }
    delete resumeSeqByRunRef.current[normalizedRunID];
    delete resumedTextByRunRef.current[normalizedRunID];
  }, []);

  React.useEffect(() => {
    let cancelled = false;

    async function load() {
      if (!conversationID) {
        setState({
          conversationPublicID: "",
          loading: false,
          loadingOlder: false,
          errorMsg: "",
          messages: [],
          total: 0,
          hasOlder: false,
        });
        return;
      }

      const isConversationSwitch = previousConversationIDRef.current !== conversationID;
      previousConversationIDRef.current = conversationID;
      setState((prev) => ({
        conversationPublicID: conversationID,
        loading: isConversationSwitch || prev.messages.length === 0,
        loadingOlder: false,
        errorMsg: "",
        messages: isConversationSwitch ? [] : prev.messages,
        total: isConversationSwitch ? 0 : prev.total,
        hasOlder: isConversationSwitch ? false : prev.hasOlder,
      }));
      try {
        const token = await resolveAccessToken();
        if (!token) {
          if (!cancelled) {
            setState({
              conversationPublicID: conversationID,
              loading: false,
              loadingOlder: false,
              errorMsg: t("signInRequired"),
              messages: [],
              total: 0,
              hasOlder: false,
            });
          }
          return;
        }

        const data = await listMessagesPage(token, conversationID, {
          page: 1,
          pageSize: MESSAGE_PAGE_SIZE,
          tail: true,
        });
        if (cancelled) {
          return;
        }

        setState((prev) => {
          const firstTailMessageID = data.results[0]?.id ?? 0;
          // 只有已加载过额外历史页时才保留旧区间，避免普通 reload 无限累积 tail 消息。
          const loadedOlderMessages =
            isConversationSwitch ||
            firstTailMessageID <= 0 ||
            prev.messages.length <= MESSAGE_PAGE_SIZE
              ? []
              : prev.messages.filter((message) => message.id < firstTailMessageID);
          const messages = [...loadedOlderMessages, ...data.results];
          return {
            conversationPublicID: conversationID,
            loading: false,
            loadingOlder: false,
            errorMsg: "",
            messages,
            total: data.total,
            hasOlder: messages.length < data.total,
          };
        });
      } catch {
        if (!cancelled) {
          setState((prev) => ({
            ...prev,
            loading: false,
            loadingOlder: false,
            errorMsg: t("loadFailed"),
          }));
        }
      }
    }

    void load();
    return () => {
      cancelled = true;
    };
  }, [conversationID, reloadToken, t]);

  const reload = React.useCallback(() => {
    setReloadToken((prev) => prev + 1);
  }, []);

  const replaceMessage = React.useCallback((nextMessage: MessageDTO) => {
    setState((prev) => ({
      ...prev,
      messages: prev.messages.map((message) =>
        message.publicID === nextMessage.publicID ? nextMessage : message,
      ),
    }));
  }, []);

  const loadOlderMessages = React.useCallback(async () => {
    const current = stateRef.current;
    if (
      !conversationID ||
      current.conversationPublicID !== conversationID ||
      current.loading ||
      current.loadingOlder ||
      !current.hasOlder ||
      current.messages.length === 0
    ) {
      return false;
    }

    const beforeID = current.messages[0]?.id ?? 0;
    if (beforeID <= 0) {
      setState((prev) => {
        const next = { ...prev, hasOlder: false };
        stateRef.current = next;
        return next;
      });
      return false;
    }

    setState((prev) => {
      const next = { ...prev, loadingOlder: true };
      stateRef.current = next;
      return next;
    });
    try {
      const token = await resolveAccessToken();
      if (!token) {
        setState((prev) => {
          const next = { ...prev, loadingOlder: false, hasOlder: false };
          stateRef.current = next;
          return next;
        });
        return false;
      }

      const data = await listMessagesPage(token, conversationID, {
        pageSize: MESSAGE_PAGE_SIZE,
        beforeID,
      });
      if (previousConversationIDRef.current !== conversationID) {
        return false;
      }
      let loaded = false;
      setState((prev) => {
        const existingPublicIDs = new Set(prev.messages.map((message) => message.publicID));
        const olderMessages = data.results.filter((message) => !existingPublicIDs.has(message.publicID));
        const messages = [...olderMessages, ...prev.messages];
        loaded = olderMessages.length > 0;
        const next = {
          ...prev,
          loadingOlder: false,
          messages,
          total: data.total,
          hasOlder: loaded && messages.length < data.total,
        };
        stateRef.current = next;
        return next;
      });
      return loaded;
    } catch {
      setState((prev) => {
        const next = { ...prev, loadingOlder: false };
        stateRef.current = next;
        return next;
      });
      return false;
    }
  }, [conversationID]);

  const cancelResumedGeneration = React.useCallback(async () => {
    const active = activeResumeStreamRef.current;
    if (!active) {
      return false;
    }

    active.controller.abort();
    clearResumeCheckpoint(active.runID);
    setResumingRunID("");

    const token = active.accessToken ?? (await resolveAccessToken());
    if (!token) {
      return false;
    }

    const result = await cancelMessageGeneration(token, active.runID).catch((): null => null);
    if (result?.canceled) {
      onConversationRunFinished?.(active.runID);
    }
    reload();
    return Boolean(result?.canceled);
  }, [clearResumeCheckpoint, onConversationRunFinished, reload]);

  const pendingAssistant = React.useMemo(() => {
    if (!conversationID || state.conversationPublicID !== conversationID) {
      return null;
    }
    for (let index = state.messages.length - 1; index >= 0; index -= 1) {
      const message = state.messages[index];
      if (message.role === "assistant" && message.status === "pending") {
        return message;
      }
    }
    return null;
  }, [conversationID, state.conversationPublicID, state.messages]);

  const pendingRunID = pendingAssistant?.runID?.trim() || "";
  // revision 仅用于重新读取可变 Set；effect 只依赖当前 pending run 的实际活动状态。
  const pendingRunIsActive = React.useMemo(
    () => Boolean(pendingRunID && activeGenerationRunsRef?.current.has(pendingRunID)),
    [activeGenerationRunsRef, activeGenerationRunsRevision, pendingRunID],
  );

  React.useEffect(() => {
    pendingAssistantContentRef.current = pendingAssistant?.content ?? "";
  }, [pendingAssistant?.content]);

  React.useEffect(() => {
    if (
      !conversationID ||
      !pendingRunID ||
      pendingRunIsActive
    ) {
      setResumingRunID("");
      setResumingActivityLabel("");
      return;
    }

    const controller = new AbortController();
    let closed = false;
    const afterSeq = resumeSeqByRunRef.current[pendingRunID] ?? 0;
    const baseContent = pendingAssistantContentRef.current;
    const resumedTextByRun = resumedTextByRunRef.current;
    const clearResumedText = () => {
      delete resumedTextByRun[pendingRunID];
    };
    const isResumeInactive = () => closed || controller.signal.aborted;
    const updateResumeState = (update: (current: ChatDataState) => ChatDataState) => {
      setState((current) =>
        isResumeInactive() || current.conversationPublicID !== conversationID
          ? current
          : update(current),
      );
    };
    resumedTextByRun[pendingRunID] = baseContent;
    activeResumeStreamRef.current = {
      controller,
      runID: pendingRunID,
      accessToken: null,
    };
    setResumingRunID(pendingRunID);
    setResumingActivityLabel("");

    async function resume() {
      try {
        const token = await resolveAccessToken();
        if (!token || controller.signal.aborted) {
          return;
        }
        if (activeResumeStreamRef.current?.controller === controller) {
          activeResumeStreamRef.current.accessToken = token;
        }
        await resumeMessageGenerationStream(token, pendingRunID, {
          signal: controller.signal,
          afterSeq,
          onTerminal: () => {
            onConversationRunFinished?.(pendingRunID);
          },
          onEventSeq: (seq) => {
            if (isResumeInactive()) {
              return;
            }
            resumeSeqByRunRef.current[pendingRunID] = Math.max(resumeSeqByRunRef.current[pendingRunID] ?? 0, seq);
          },
          onMediaStatus: (event) => {
            if (isResumeInactive()) {
              return;
            }
            const status = event.status.trim();
            const contentType = event.content_type === "video" ? "video" : "image";
            const activityLabel =
              status === "queued"
                ? tSubmit(contentType === "video" ? "mediaStatus.videoQueued" : "mediaStatus.queued")
                : status === "running"
                  ? tSubmit(contentType === "video" ? "mediaStatus.videoRunning" : "mediaStatus.running")
                  : status === "saving_artifact"
                    ? tSubmit(contentType === "video" ? "mediaStatus.videoSavingArtifact" : "mediaStatus.savingArtifact")
                    : event.message.trim() || status;
            setResumingActivityLabel(activityLabel);
            updateResumeState((prev) => ({
              ...prev,
              messages: prev.messages.map((message) =>
                message.runID === pendingRunID && message.role === "assistant" && message.status === "pending"
                  ? { ...message, contentType }
                  : message,
              ),
            }));
          },
          onMediaImageDelta: (event) => {
            if (isResumeInactive()) {
              return;
            }
            clearResumedText();
            const previewMarkdown = buildMediaImagePreviewMarkdown(event, tSubmit("imagePreviewAlt"));
            if (!previewMarkdown) {
              return;
            }
            setResumingActivityLabel("");
            updateResumeState((prev) => ({
              ...prev,
              messages: prev.messages.map((message) =>
                message.runID === pendingRunID && message.role === "assistant" && message.status === "pending"
                  ? { ...message, content: previewMarkdown, contentType: "image" }
                  : message,
              ),
            }));
          },
          onTextSnapshot: (content) => {
            if (isResumeInactive()) {
              return;
            }
            setResumingActivityLabel("");
            resumedTextByRun[pendingRunID] = content;
            updateResumeState((prev) => ({
              ...prev,
              messages: prev.messages.map((message) =>
                message.runID === pendingRunID && message.role === "assistant" && message.status === "pending"
                  ? { ...message, content, contentType: "text" }
                  : message,
              ),
            }));
          },
          onDelta: (delta) => {
            if (isResumeInactive()) {
              return;
            }
            setResumingActivityLabel("");
            const nextContent = `${resumedTextByRun[pendingRunID] ?? ""}${delta}`;
            resumedTextByRun[pendingRunID] = nextContent;
            updateResumeState((prev) => ({
              ...prev,
              messages: prev.messages.map((message) =>
                message.runID === pendingRunID && message.role === "assistant" && message.status === "pending"
                  ? { ...message, content: nextContent }
                  : message,
              ),
            }));
          },
          onProcessUpdate: (event) => {
            updateResumeState((prev) => ({
              ...prev,
              messages: prev.messages.map((message) =>
                message.runID === pendingRunID && message.role === "assistant" && message.status === "pending"
                  ? { ...message, processTrace: event.trace ?? message.processTrace }
                  : message,
              ),
            }));
          },
          onUpstreamThinkDelta: (event) => {
            if (isResumeInactive()) {
              return;
            }
            upsertLiveUpstreamThinkTrace(pendingRunID, event);
          },
          onUsage: (event) => {
            updateResumeState((prev) => ({
              ...prev,
              messages: prev.messages.map((message) =>
                message.runID === pendingRunID && message.role === "assistant" && message.status === "pending"
                  ? {
                      ...message,
                      inputTokens: event.input_tokens > 0 ? event.input_tokens : message.inputTokens,
                      outputTokens: event.output_tokens > 0 ? event.output_tokens : message.outputTokens,
                      cacheReadTokens:
                        event.cache_read_tokens > 0 ? event.cache_read_tokens : message.cacheReadTokens,
                      cacheWriteTokens:
                        event.cache_write_tokens > 0 ? event.cache_write_tokens : message.cacheWriteTokens,
                      reasoningTokens:
                        event.reasoning_tokens > 0 ? event.reasoning_tokens : message.reasoningTokens,
                    }
                  : message,
              ),
            }));
          },
          onModerationChecking: () => {
            if (!isResumeInactive()) {
              setResumingActivityLabel(tSubmit("moderationChecking"));
            }
          },
          onModerationBlocked: (event) => {
            if (isResumeInactive()) {
              return;
            }
            clearResumedText();
            setResumingActivityLabel("");
            const categories = Array.isArray(event.categories) ? event.categories : [];
            updateResumeState((prev) => ({
              ...prev,
              messages: prev.messages.map((message) =>
                message.runID === pendingRunID && message.role === "assistant"
                  ? {
                      ...message,
                      status: "blocked",
                      content: "",
                      contentType: "text",
                      attachments: "[]",
                      processTrace: undefined,
                      errorCode: "content_moderation.blocked",
                      errorMessage: tSubmit("moderationBlockedDescription"),
                      moderation: {
                        state: "blocked",
                        direction: event.direction,
                        eventID: event.eventID,
                        categories,
                      },
                    }
                  : message,
              ),
            }));
          },
        });
        if (!controller.signal.aborted) {
          setResumingActivityLabel("");
          clearResumeCheckpoint(pendingRunID);
          reload();
        }
      } catch (error) {
        if (!controller.signal.aborted && error instanceof Error && error.name !== "AbortError") {
          clearResumeCheckpoint(pendingRunID);
          setResumingRunID("");
          setResumingActivityLabel("");
          reload();
        }
      } finally {
        if (activeResumeStreamRef.current?.controller === controller) {
          activeResumeStreamRef.current = null;
        }
        if (!controller.signal.aborted && !closed) {
          setResumingRunID("");
          setResumingActivityLabel("");
        }
      }
    }

    void resume();
    return () => {
      closed = true;
      controller.abort();
      setResumingActivityLabel("");
      clearResumeCheckpoint(pendingRunID);
      if (activeResumeStreamRef.current?.controller === controller) {
        activeResumeStreamRef.current = null;
      }
    };
  }, [
    clearResumeCheckpoint,
    conversationID,
    pendingRunID,
    pendingRunIsActive,
    onConversationRunFinished,
    reload,
    tSubmit,
  ]);

  React.useEffect(() => {
    if (
      !conversationID ||
      !pendingAssistant ||
      pendingRunIsActive ||
      (pendingRunID && pendingRunID === resumingRunID)
    ) {
      return;
    }
    const timer = window.setTimeout(() => {
      reload();
    }, 1500);
    return () => {
      window.clearTimeout(timer);
    };
  }, [conversationID, pendingAssistant, pendingRunID, pendingRunIsActive, reload, resumingRunID]);

  return {
    ...state,
    cancelResumedGeneration,
    loadOlderMessages,
    reload,
    replaceMessage,
    resumingActivityLabel,
    resumingRunID,
  };
}
