"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { cancelMessageGeneration, listMessages, resumeMessageGenerationStream } from "@/shared/api/conversation";
import { buildMediaImagePreviewMarkdown } from "@/features/chat/model/media-image-preview";
import type { MessageDTO } from "@/shared/api/conversation.types";

type ChatDataState = {
  loading: boolean;
  errorMsg: string;
  messages: MessageDTO[];
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
    errorMsg: "",
    messages: [],
  });
  const [reloadToken, setReloadToken] = React.useState(0);
  const [resumingRunID, setResumingRunID] = React.useState("");
  const previousConversationIDRef = React.useRef<string | null>(conversationID);
  const resumeSeqByRunRef = React.useRef<Record<string, number>>({});
  const activeResumeStreamRef = React.useRef<ActiveResumeStream | null>(null);

  React.useEffect(() => {
    let cancelled = false;

    async function load() {
      if (!conversationID) {
        setState({
          loading: false,
          errorMsg: "",
          messages: [],
        });
        return;
      }

      const isConversationSwitch = previousConversationIDRef.current !== conversationID;
      previousConversationIDRef.current = conversationID;
      setState((prev) => ({
        loading: isConversationSwitch || prev.messages.length === 0,
        errorMsg: "",
        messages: isConversationSwitch ? [] : prev.messages,
      }));
      try {
        const token = await resolveAccessToken();
        if (!token) {
          if (!cancelled) {
            setState({
              loading: false,
              errorMsg: t("signInRequired"),
              messages: [],
            });
          }
          return;
        }

        const messages = await listMessages(token, conversationID);
        if (cancelled) {
          return;
        }

        setState({
          loading: false,
          errorMsg: "",
          messages,
        });
      } catch {
        if (!cancelled) {
          setState((prev) => ({
            ...prev,
            loading: false,
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
            setState((prev) => ({
              ...prev,
              messages: prev.messages.map((message) =>
                message.runID === pendingRunID && message.role === "assistant" && message.status === "pending"
                  ? { ...message, content: `${message.content}${delta}` }
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
          reload();
        }
        if (!controller.signal.aborted && completed) {
          delete resumeSeqByRunRef.current[pendingRunID];
          reload();
        }
      } catch (error) {
        if (!controller.signal.aborted && error instanceof Error && error.name !== "AbortError") {
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
    reload,
    replaceMessage,
    resumingRunID,
  };
}
