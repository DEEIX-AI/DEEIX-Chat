import {
  type ChatSubmitBlockReason,
  type ChatSubmitTask,
  resolveChatSubmitDecision,
} from "@/features/chat/model/chat-task";
import { sanitizeConversationOptions } from "@/features/chat/model/conversation-options";
import { resolvePersistedPublicID } from "@/features/chat/model/message-submit";
import {
  type ActiveStream,
  type BranchScope,
  branchRunIsVisible,
  branchScopePathsEqual,
  branchScopesEqual,
  buildSubmissionBranchScopePath,
  createClientRunID,
  MAX_CONCURRENT_RUNS,
  type QueuedChatSubmission,
} from "@/features/chat/model/message-submit-branching";
import { resolveImageLoadingAspectRatio } from "@/features/chat/model/message-submit-media";
import type { ChatModelOption, PendingAttachment } from "@/features/chat/types/chat-runtime";
import type { ChatAreaMessage, ImageLoadingAspectRatio } from "@/features/chat/types/messages";
import type { ConversationOptions } from "@/shared/api/conversation.types";
import type { SkillSummaryDTO } from "@/shared/api/skills.types";

export type ChatSubmissionBranchReason = "default" | "retry" | "edit";

export type ChatSubmissionBlock =
  | { kind: "invalid" }
  | { kind: "concurrent_limit" }
  | { kind: "media_unsupported"; reason: ChatSubmitBlockReason }
  | { kind: "no_model" };

export type ChatSubmissionPlan = {
  payloadContent: string;
  platformModelName: string;
  selectedToolIDs: number[];
  selectedSkills: SkillSummaryDTO[];
  selectedKnowledgeBaseIDs: string[];
  htmlVisualPromptEnabled: boolean;
  sanitizedOptions: ConversationOptions;
  submitTask: ChatSubmitTask;
  branchReason: ChatSubmissionBranchReason;
  clientRunID: string;
  exchangeKey: string;
  targetConversationScopeKey: string;
  targetBranchScope: BranchScope;
  shouldFollowSubmittedBranch: boolean;
  effectiveAttachments: PendingAttachment[];
  resolvedParentPublicID: string | null;
  resolvedSourcePublicID: string | null;
  assistantOnlyBranch: boolean;
  pendingParentPublicID: string | null;
  tempUserPublicID: string;
  tempAssistantPublicID: string;
  pendingUserPublicID: string;
  assistantImageAspectRatio: ImageLoadingAspectRatio | undefined;
  assistantContentType: string;
};

export type ChatSubmissionPlanResult =
  | { ok: true; plan: ChatSubmissionPlan; attachmentsTruncated: boolean }
  | { ok: false; block: ChatSubmissionBlock; attachmentsTruncated: boolean };

export function planChatSubmission(input: {
  content: string;
  currentAttachments: PendingAttachment[];
  parentMessagePublicID?: string | null;
  sourceMessagePublicID?: string | null;
  branchReason?: ChatSubmissionBranchReason;
  queuedSubmission?: QueuedChatSubmission;
  attachmentFallbackContent: string;
  uploading: boolean;
  maxFilesPerMessage: number;
  modelOptions: ChatModelOption[];
  selectedPlatformModelName: string;
  options: ConversationOptions;
  selectedToolIDs: number[];
  selectedSkills: SkillSummaryDTO[];
  selectedKnowledgeBaseIDs: string[];
  htmlVisualPromptEnabled: boolean;
  visibleConversationScopeKey: string;
  visibleBranchScopePath: readonly string[];
  visibleMessages: ChatAreaMessage[];
  combinedMessages: ChatAreaMessage[];
  activeStreams: ActiveStream[];
}): ChatSubmissionPlanResult {
  const { content, currentAttachments, queuedSubmission, activeStreams, combinedMessages } = input;
  const payloadContent = content || input.attachmentFallbackContent;
  const platformModelName = (queuedSubmission?.platformModelName ?? input.selectedPlatformModelName).trim();
  const requestOptions = queuedSubmission?.options ?? input.options;
  const selectedToolIDs = queuedSubmission?.selectedToolIDs ?? input.selectedToolIDs;
  const selectedSkills = queuedSubmission?.selectedSkills ?? input.selectedSkills;
  const selectedKnowledgeBaseIDs = queuedSubmission?.selectedKnowledgeBaseIDs ?? input.selectedKnowledgeBaseIDs;
  const htmlVisualPromptEnabled = queuedSubmission?.htmlVisualPromptEnabled ?? input.htmlVisualPromptEnabled;
  const targetConversationScopeKey = queuedSubmission?.conversationScopeKey ?? input.visibleConversationScopeKey;
  const resolvedParentPublicID = resolvePersistedPublicID(input.parentMessagePublicID);
  const targetBranchScopePath =
    queuedSubmission?.branchScopePath.slice() ??
    buildSubmissionBranchScopePath(input.visibleMessages, resolvedParentPublicID);
  const clientRunID = queuedSubmission?.clientRunID ?? createClientRunID();
  const targetBranchScope: BranchScope = {
    conversationScopeKey: targetConversationScopeKey,
    branchScopePath: targetBranchScopePath,
    branchScopeRunID: queuedSubmission?.branchScopeRunID ?? clientRunID,
  };
  const shouldFollowSubmittedBranch =
    !queuedSubmission ||
    branchRunIsVisible(
      targetBranchScope,
      clientRunID,
      input.visibleConversationScopeKey,
      input.visibleBranchScopePath,
      input.visibleMessages,
    );
  const selectedModel =
    input.modelOptions.find((item) => item.platformModelName === platformModelName) ?? null;
  const branchReason = input.branchReason ?? "default";
  const concurrentBranchRun = branchReason === "retry" || branchReason === "edit";
  const targetConversationHasActiveStream = activeStreams.some((active) =>
    queuedSubmission
      ? branchScopesEqual(active, targetBranchScope)
      : active.conversationScopeKey === targetConversationScopeKey &&
        branchScopePathsEqual(active.branchScopePath, targetBranchScopePath),
  );
  if (
    (!content && currentAttachments.length === 0) ||
    (!queuedSubmission && input.uploading) ||
    (!concurrentBranchRun && targetConversationHasActiveStream)
  ) {
    return { ok: false, block: { kind: "invalid" }, attachmentsTruncated: false };
  }
  if (activeStreams.length >= MAX_CONCURRENT_RUNS) {
    return { ok: false, block: { kind: "concurrent_limit" }, attachmentsTruncated: false };
  }
  if (concurrentBranchRun) {
    const activeRunIDs = new Set(activeStreams.map((active) => active.runID));
    for (const message of combinedMessages) {
      const runID = message.runID?.trim() || "";
      if (
        message.role === "assistant" &&
        runID &&
        (message.isPending || message.isStreaming || message.status?.trim().toLowerCase() === "pending")
      ) {
        activeRunIDs.add(runID);
      }
    }
    if (activeRunIDs.size >= MAX_CONCURRENT_RUNS) {
      return { ok: false, block: { kind: "concurrent_limit" }, attachmentsTruncated: false };
    }
  }
  const effectiveAttachments =
    input.maxFilesPerMessage > 0 && currentAttachments.length > input.maxFilesPerMessage
      ? currentAttachments.slice(0, input.maxFilesPerMessage)
      : currentAttachments;
  const attachmentsTruncated = effectiveAttachments.length < currentAttachments.length;
  const sanitizedOptions = sanitizeConversationOptions(requestOptions);
  const submitDecision = resolveChatSubmitDecision(selectedModel, effectiveAttachments, sanitizedOptions);
  if (submitDecision.blockedReason) {
    return {
      ok: false,
      block: { kind: "media_unsupported", reason: submitDecision.blockedReason },
      attachmentsTruncated,
    };
  }
  if (!platformModelName) {
    return { ok: false, block: { kind: "no_model" }, attachmentsTruncated };
  }
  const submitTask = submitDecision.task;

  const exchangeKey = `local-exchange-${clientRunID}`;
  const resolvedSourcePublicID = resolvePersistedPublicID(input.sourceMessagePublicID);
  const assistantOnlyBranch =
    branchReason === "retry" &&
    Boolean(resolvedParentPublicID && resolvedSourcePublicID) &&
    combinedMessages.some((item) => item.publicID === resolvedSourcePublicID && item.role === "assistant");
  const reusedUserMessage = assistantOnlyBranch
    ? combinedMessages.find(
        (item) => item.publicID === resolvedParentPublicID && item.role === "user",
      ) ?? null
    : null;
  const pendingParentPublicID = assistantOnlyBranch
    ? reusedUserMessage?.parentPublicID ?? null
    : resolvedParentPublicID;
  const tempUserPublicID = `${exchangeKey}-user`;
  const tempAssistantPublicID = `${exchangeKey}-assistant`;
  const pendingUserPublicID =
    assistantOnlyBranch && resolvedParentPublicID ? resolvedParentPublicID : tempUserPublicID;
  const assistantImageAspectRatio =
    submitTask === "image_generation" || submitTask === "image_edit"
      ? resolveImageLoadingAspectRatio(sanitizedOptions)
      : undefined;
  const assistantContentType =
    submitTask === "chat"
      ? "markdown"
      : submitTask === "video_generation" || submitTask === "video_extension"
        ? "video"
        : "image";

  return {
    ok: true,
    attachmentsTruncated,
    plan: {
      payloadContent,
      platformModelName,
      selectedToolIDs,
      selectedSkills,
      selectedKnowledgeBaseIDs,
      htmlVisualPromptEnabled,
      sanitizedOptions,
      submitTask,
      branchReason,
      clientRunID,
      exchangeKey,
      targetConversationScopeKey,
      targetBranchScope,
      shouldFollowSubmittedBranch,
      effectiveAttachments,
      resolvedParentPublicID,
      resolvedSourcePublicID,
      assistantOnlyBranch,
      pendingParentPublicID,
      tempUserPublicID,
      tempAssistantPublicID,
      pendingUserPublicID,
      assistantImageAspectRatio,
      assistantContentType,
    },
  };
}
