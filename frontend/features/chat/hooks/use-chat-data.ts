"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { cancelMessageGeneration, listMessagesPage, resumeMessageGenerationStream } from "@/shared/api/conversation";
import { buildMediaImagePreviewMarkdown } from "@/features/chat/model/media-image-preview";
import type { MessageDTO } from "@/shared/api/conversation.types";

const MESSAGE_PAGE_SIZE = 100;

type ChatDataState = {
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

type ResumeTextReplayState = {
  baseContent: string;
  replayedContent: string;
  visibleContent: string;
};

function appendResumedTextDelta(state: ResumeTextReplayState, delta: string): string {
  if (!delta) {
    return state.visibleContent;
  }

  state.replayedContent += delta;
  const { baseContent, replayedContent } = state;
  if (!baseContent) {
    state.visibleContent = replayedContent;
    return state.visibleContent;
  }

  if (
    replayedContent === baseContent ||
    baseContent.startsWith(replayedContent) ||
    baseContent.includes(replayedContent)
  ) {
    state.visibleContent = baseContent;
    return state.visibleContent;
  }

  if (replayedContent.startsWith(baseContent)) {
    state.visibleContent = replayedContent;
    return state.visibleContent;
  }

  const maxOverlapLength = Math.min(baseContent.length, replayedContent.length);
  for (let length = maxOverlapLength; length > 0; length -= 1) {
    if (baseContent.endsWith(replayedContent.slice(0, length))) {
      state.visibleContent = `${baseContent}${replayedContent.slice(length)}`;
      return state.visibleContent;
    }
  }

  state.visibleContent = `${state.visibleContent}${delta}`;
  return state.visibleContent;
}

export function useChatData(
  conversationID: string | null,
  {
    activeGenerationRunsRef,
    failedGenerationRunsRef,
  }: {
    activeGenerationRunsRef?: React.RefObject<Set<string>>;
    failedGenerationRunsRef?: React.RefObject<Set<string>>;
  } = {},
) {
  const t = useTranslations("chat.data");
  const tSubmit = useTranslations("chat.submit");
  const [state, setState] = React.useState<ChatDataState>({
    loading: Boolean(conversationID),
    loadingOlder: false,
    errorMsg: "",
    messages: [],
    total: 0,
    hasOlder: false,
  });
  const [reloadToken, setReloadToken] = React.useState(0);
  const [resumingRunID, setResumingRunID] = React.useState("");
  const stateRef = React.useRef(state);
  stateRef.current = state;
  const previousConversationIDRef = React.useRef<string | null>(conversationID);
  const resumeSeqByRunRef = React.useRef<Record<string, number>>({});
  const pendingAssistantContentRef = React.useRef("");
  const resumeTextReplayByRunRef = React.useRef<Record<string, ResumeTextReplayState>>({});
  const activeResumeStreamRef = React.useRef<ActiveResumeStream | null>(null);

  React.useEffect(() => {
    let cancelled = false;

    async function load() {
      if (!conversationID) {
        setState({
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

        setState({
          loading: false,
          loadingOlder: false,
          errorMsg: "",
          messages: data.results,
          total: data.total,
          hasOlder: data.results.length < data.total,
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
    if (!conversationID || current.loading || current.loadingOlder || !current.hasOlder || current.messages.length === 0) {
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

  const loadAllOlderMessages = React.useCallback(async ({ maxPages = 50 }: { maxPages?: number } = {}) => {
    for (let iteration = 0; iteration < maxPages; iteration += 1) {
      if (!stateRef.current.hasOlder) {
        return true;
      }
      const loaded = await loadOlderMessages();
      if (!loaded) {
        return !stateRef.current.hasOlder;
      }
    }
    return !stateRef.current.hasOlder;
  }, [loadOlderMessages]);

  const cancelResumedGeneration = React.useCallback(async () => {
    const active = activeResumeStreamRef.current;
    if (!active) {
      return false;
    }

    active.controller.abort();
    setResumingRunID("");

    const token = active.accessToken ?? (await resolveAccessToken());
    if (!token) {
      return false;
    }

    const result = await cancelMessageGeneration(token, active.runID).catch(() => null);
    reload();
    return Boolean(result?.canceled);
  }, [reload]);

  const pendingAssistant = React.useMemo(() => {
    for (let index = state.messages.length - 1; index >= 0; index -= 1) {
      const message = state.messages[index];
      if (message.role === "assistant" && message.status === "pending") {
        return message;
      }
    }
    return null;
  }, [state.messages]);

  const pendingRunID = pendingAssistant?.runID?.trim() || "";

  React.useEffect(() => {
    pendingAssistantContentRef.current = pendingAssistant?.content ?? "";
  }, [pendingAssistant?.content]);

  React.useEffect(() => {
    if (
      !conversationID ||
      !pendingRunID ||
      activeGenerationRunsRef?.current.has(pendingRunID) ||
      failedGenerationRunsRef?.current.has(pendingRunID)
    ) {
      setResumingRunID("");
      return;
    }

    const controller = new AbortController();
    let closed = false;
    const afterSeq = resumeSeqByRunRef.current[pendingRunID] ?? 0;
    const baseContent = pendingAssistantContentRef.current;
    const resumeTextReplayByRun = resumeTextReplayByRunRef.current;
    const clearResumeTextReplay = () => {
      delete resumeTextReplayByRun[pendingRunID];
    };
    resumeTextReplayByRun[pendingRunID] = {
      baseContent,
      replayedContent: afterSeq > 0 ? baseContent : "",
      visibleContent: baseContent,
    };
    activeResumeStreamRef.current = {
      controller,
      runID: pendingRunID,
      accessToken: null,
    };
    setResumingRunID(pendingRunID);

    async function resume() {
      try {
        const token = await resolveAccessToken();
        if (!token || controller.signal.aborted) {
          return;
        }
        if (activeResumeStreamRef.current?.controller === controller) {
          activeResumeStreamRef.current.accessToken = token;
        }
        const completed = await resumeMessageGenerationStream(token, pendingRunID, {
          signal: controller.signal,
          afterSeq,
          onEventSeq: (seq) => {
            resumeSeqByRunRef.current[pendingRunID] = Math.max(resumeSeqByRunRef.current[pendingRunID] ?? 0, seq);
          },
          onMediaStatus: (event) => {
            const status = event.status.trim();
            const activityLabel =
              status === "queued"
                ? tSubmit("mediaStatus.queued")
                : status === "running"
                  ? tSubmit("mediaStatus.running")
                  : status === "saving_artifact"
                    ? tSubmit("mediaStatus.savingArtifact")
                    : event.message.trim() || status;
            setState((prev) => ({
              ...prev,
              messages: prev.messages.map((message) =>
                message.runID === pendingRunID && message.role === "assistant" && message.status === "pending"
                  ? { ...message, activityLabel, contentType: "image" }
                  : message,
              ),
            }));
          },
          onMediaImageDelta: (event) => {
            clearResumeTextReplay();
            const previewMarkdown = buildMediaImagePreviewMarkdown(event, tSubmit("imagePreviewAlt"));
            if (!previewMarkdown) {
              return;
            }
            setState((prev) => ({
              ...prev,
              messages: prev.messages.map((message) =>
                message.runID === pendingRunID && message.role === "assistant" && message.status === "pending"
                  ? { ...message, content: previewMarkdown, contentType: "image", activityLabel: "" }
                  : message,
              ),
            }));
          },
          onDelta: (delta) => {
            const replayState =
              resumeTextReplayByRun[pendingRunID] ??
              (resumeTextReplayByRun[pendingRunID] = {
                baseContent: "",
                replayedContent: "",
                visibleContent: "",
              });
            const nextContent = appendResumedTextDelta(replayState, delta);
            setState((prev) => ({
              ...prev,
              messages: prev.messages.map((message) =>
                message.runID === pendingRunID && message.role === "assistant" && message.status === "pending"
                  ? { ...message, content: nextContent }
                  : message,
              ),
            }));
          },
          onProcessUpdate: (event) => {
            setState((prev) => ({
              ...prev,
              messages: prev.messages.map((message) =>
                message.runID === pendingRunID && message.role === "assistant" && message.status === "pending"
                  ? { ...message, processTrace: event.trace }
                  : message,
              ),
            }));
          },
          onUpstreamThinkDelta: (event) => {
            setState((prev) => ({
              ...prev,
              messages: prev.messages.map((message) =>
                message.runID === pendingRunID && message.role === "assistant" && message.status === "pending"
                  ? { ...message, processTrace: event.trace }
                  : message,
              ),
            }));
          },
          onUsage: (event) => {
            setState((prev) => ({
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
        });
        if (!controller.signal.aborted && completed === null) {
          clearResumeTextReplay();
          reload();
        }
        if (!controller.signal.aborted && completed) {
          delete resumeSeqByRunRef.current[pendingRunID];
          clearResumeTextReplay();
          reload();
        }
      } catch (error) {
        if (!controller.signal.aborted && error instanceof Error && error.name !== "AbortError") {
          clearResumeTextReplay();
          setResumingRunID("");
          reload();
        }
      } finally {
        if (activeResumeStreamRef.current?.controller === controller) {
          activeResumeStreamRef.current = null;
        }
        if (!controller.signal.aborted && !closed) {
          setResumingRunID("");
        }
      }
    }

    void resume();
    return () => {
      closed = true;
      controller.abort();
      clearResumeTextReplay();
      if (activeResumeStreamRef.current?.controller === controller) {
        activeResumeStreamRef.current = null;
      }
    };
  }, [activeGenerationRunsRef, conversationID, failedGenerationRunsRef, pendingRunID, reload, tSubmit]);

  React.useEffect(() => {
    if (
      !conversationID ||
      !pendingAssistant ||
      activeGenerationRunsRef?.current.has(pendingRunID) ||
      failedGenerationRunsRef?.current.has(pendingRunID) ||
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
  }, [activeGenerationRunsRef, conversationID, failedGenerationRunsRef, pendingAssistant, pendingRunID, reload, resumingRunID]);

  return {
    ...state,
    cancelResumedGeneration,
    loadOlderMessages,
    loadAllOlderMessages,
    reload,
    replaceMessage,
    resumingRunID,
  };
}
