"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { mapServerMessage } from "@/features/chat/model/chat-thread";
import { toPendingProcessTrace } from "@/features/chat/model/message-submit";
import {
  clearLiveUpstreamThinkTrace,
  upsertLiveUpstreamThinkTrace,
} from "@/features/chat/model/upstream-think-store";
import type { ChatAreaMessage } from "@/features/chat/types/messages";
import { streamTemporaryChatMessage } from "@/shared/api/conversation";
import type { ConversationOptions, TemporaryChatHistoryMessage } from "@/shared/api/conversation.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { createSecureUUID } from "@/shared/lib/secure-id";

type TemporaryMessage = TemporaryChatHistoryMessage & {
  id: string;
  model?: string;
  runID?: string;
  streaming?: boolean;
  failed?: boolean;
  inputTokens?: number;
  outputTokens?: number;
  latencyMS?: number;
  activityLabel?: string;
  processTrace?: ChatAreaMessage["processTrace"];
  knowledgeSources?: ChatAreaMessage["knowledgeSources"];
};

type TemporaryChatRuntimeInput = {
  active: boolean;
  draft: string;
  model: string;
  options: ConversationOptions;
  selectedToolIDs: number[];
  selectedSkillIDs: number[];
  selectedKnowledgeBaseIDs: string[];
  htmlVisualPromptEnabled: boolean;
  onDraftChange: (value: string) => void;
};

export function useChatTemporaryRuntime({
  active,
  draft,
  model,
  options,
  selectedToolIDs,
  selectedSkillIDs,
  selectedKnowledgeBaseIDs,
  htmlVisualPromptEnabled,
  onDraftChange,
}: TemporaryChatRuntimeInput) {
  const t = useTranslations("chat.temporary");
  const [messages, setMessages] = React.useState<TemporaryMessage[]>([]);
  const [sending, setSending] = React.useState(false);
  const historyRef = React.useRef<TemporaryChatHistoryMessage[]>([]);
  const sessionIDRef = React.useRef("");
  const abortControllerRef = React.useRef<AbortController | null>(null);
  const sendingRef = React.useRef(false);
  const liveRunIDsRef = React.useRef(new Set<string>());

  const clearLiveTraces = React.useCallback(() => {
    liveRunIDsRef.current.forEach(clearLiveUpstreamThinkTrace);
    liveRunIDsRef.current.clear();
  }, []);

  const updateMessage = React.useCallback(
    (messageID: string, update: (message: TemporaryMessage) => TemporaryMessage) => {
      setMessages((current) => current.map((message) =>
        message.id === messageID ? update(message) : message
      ));
    },
    [],
  );

  const finishSending = React.useCallback((controller: AbortController) => {
    if (abortControllerRef.current !== controller) {
      return;
    }
    abortControllerRef.current = null;
    setSending(false);
    sendingRef.current = false;
  }, []);

  React.useEffect(() => {
    if (active) {
      return;
    }
    abortControllerRef.current?.abort();
    abortControllerRef.current = null;
    sendingRef.current = false;
    sessionIDRef.current = "";
    historyRef.current = [];
    clearLiveTraces();
    setSending(false);
    setMessages([]);
  }, [active, clearLiveTraces]);

  React.useEffect(() => {
    const abort = () => abortControllerRef.current?.abort();
    window.addEventListener("pagehide", abort);
    return () => {
      window.removeEventListener("pagehide", abort);
      abort();
      sessionIDRef.current = "";
      historyRef.current = [];
      clearLiveTraces();
    };
  }, [clearLiveTraces]);

  const stop = React.useCallback(() => {
    abortControllerRef.current?.abort();
  }, []);

  const send = React.useCallback(async () => {
    const content = draft.trim();
    const selectedModel = model.trim();
    if (!active || !content || !selectedModel || sendingRef.current) {
      return;
    }
    sendingRef.current = true;
    setSending(true);
    const controller = new AbortController();
    abortControllerRef.current = controller;
    const token = await resolveAccessToken().catch(() => "");
    if (controller.signal.aborted) {
      finishSending(controller);
      return;
    }
    if (!token) {
      finishSending(controller);
      toast.error(t("sessionExpired"));
      return;
    }
    if (!sessionIDRef.current) {
      sessionIDRef.current = createSecureUUID();
    }

    const userMessage: TemporaryMessage = { id: createSecureUUID(), role: "user", content };
    const assistantID = createSecureUUID();
    const clientRunID = createSecureUUID();
    const history: TemporaryChatHistoryMessage[] = [
      ...historyRef.current,
      { role: "user", content },
    ];
    onDraftChange("");
    setMessages((current) => [
      ...current,
      userMessage,
      { id: assistantID, role: "assistant", content: "", model: selectedModel, runID: clientRunID, streaming: true },
    ]);

    let streamedAssistantText = "";
    let moderationBlocked = false;
    try {
      const completed = await streamTemporaryChatMessage(
        token,
        {
          sessionID: sessionIDRef.current,
          clientRunID,
          model: selectedModel,
          options,
          selectedToolIDs: selectedToolIDs.length > 0 ? selectedToolIDs : undefined,
          skillIDs: selectedSkillIDs.length > 0 ? selectedSkillIDs : undefined,
          knowledgeBaseIDs: selectedKnowledgeBaseIDs.length > 0 ? selectedKnowledgeBaseIDs : undefined,
          htmlVisualPrompt: htmlVisualPromptEnabled || undefined,
          messages: history,
        },
        {
          signal: controller.signal,
          onDelta: (delta) => {
            streamedAssistantText += delta;
            updateMessage(assistantID, (message) => ({
              ...message,
              content: `${message.content}${delta}`,
              activityLabel: undefined,
            }));
          },
          onRagSearch: (message) => {
            updateMessage(assistantID, (item) => ({ ...item, activityLabel: message }));
          },
          onProcessUpdate: (event) => {
            updateMessage(assistantID, (message) => ({
              ...message,
              activityLabel: undefined,
              processTrace: event.trace
                ? toPendingProcessTrace(event.trace)
                : message.processTrace,
            }));
          },
          onUpstreamThinkDelta: (event) => {
            liveRunIDsRef.current.add(clientRunID);
            upsertLiveUpstreamThinkTrace(clientRunID, event);
          },
          onUsage: (event) => {
            updateMessage(assistantID, (message) => ({
              ...message,
              inputTokens: event.input_tokens > 0 ? event.input_tokens : message.inputTokens,
              outputTokens: event.output_tokens > 0 ? event.output_tokens : message.outputTokens,
            }));
          },
          onModerationBlocked: () => {
            moderationBlocked = true;
            updateMessage(assistantID, (message) => ({
              ...message,
              content: t("blocked"),
              streaming: false,
              failed: true,
            }));
          },
        },
      );
      if (controller.signal.aborted || abortControllerRef.current !== controller) {
        return;
      }
      const mappedAssistant = mapServerMessage(completed.assistantMessage);
      historyRef.current = [
        ...historyRef.current,
        { role: "user", content },
        { role: "assistant", content: completed.assistantMessage.content },
      ];
      updateMessage(assistantID, (message) => ({
        ...message,
        content: completed.assistantMessage.content,
        streaming: false,
        inputTokens: completed.userMessage.inputTokens,
        outputTokens: completed.assistantMessage.outputTokens,
        latencyMS: completed.assistantMessage.latencyMS,
        activityLabel: undefined,
        processTrace: mappedAssistant.processTrace,
        knowledgeSources: mappedAssistant.knowledgeSources,
      }));
    } catch (error) {
      const aborted = controller.signal.aborted;
      if (!moderationBlocked && streamedAssistantText.trim()) {
        historyRef.current = [
          ...historyRef.current,
          { role: "user", content },
          { role: "assistant", content: streamedAssistantText },
        ];
      }
      updateMessage(assistantID, (message) => ({
        ...message,
        content: message.content || (aborted ? t("stopped") : t("failed")),
        streaming: false,
        failed: true,
      }));
      if (!aborted) {
        toast.error(t("failed"), {
          description: error instanceof Error ? error.message : undefined,
        });
      }
    } finally {
      finishSending(controller);
    }
  }, [active, draft, finishSending, htmlVisualPromptEnabled, model, onDraftChange, options, selectedKnowledgeBaseIDs, selectedSkillIDs, selectedToolIDs, t, updateMessage]);

  const areaMessages = React.useMemo<ChatAreaMessage[]>(
    () => messages.map((message): ChatAreaMessage => ({
      key: message.id,
      publicID: message.id,
      parentPublicID: null,
      sourcePublicID: null,
      role: message.role,
      content: message.content,
      branchReason: "default",
      status: message.failed ? "failed" : message.streaming ? "processing" : "completed",
      isStreaming: message.streaming,
      platformModelName: message.model,
      runID: message.runID,
      inputTokens: message.inputTokens,
      outputTokens: message.outputTokens,
      latencyMS: message.latencyMS,
      activityLabel: message.activityLabel,
      processTrace: message.processTrace,
      knowledgeSources: message.knowledgeSources,
    })),
    [messages],
  );

  return {
    messages: areaMessages,
    sending,
    send,
    stop,
  };
}
