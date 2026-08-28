import { toBranchKey } from "@/features/chat/model/chat-thread";
import { resolvePersistedPublicID } from "@/features/chat/model/message-submit";
import type { PendingAttachment, PendingExchange } from "@/features/chat/types/chat-runtime";
import type { ChatAreaMessage } from "@/features/chat/types/messages";
import type { ConversationDTO, ConversationOptions } from "@/shared/api/conversation.types";
import type { SkillSummaryDTO } from "@/shared/api/skills.types";
import { createSecureUUID } from "@/shared/lib/secure-id";

export const MAX_CONCURRENT_RUNS = 5;

export type BranchScope = {
  conversationScopeKey: string;
  branchScopePath: string[];
  branchScopeRunID: string;
};

export type ActiveStream = BranchScope & {
  controller: AbortController;
  runID: string;
  accessToken: string | null;
  cancelRequested: boolean;
  cancelSettlementTimer: number | null;
};

export type QueuedChatSubmission = BranchScope & {
  id: string;
  clientRunID: string;
  parentRunID: string | null;
  conversationPublicID: string | null;
  conversation: ConversationDTO | null;
  parentMessagePublicID: string | null;
  content: string;
  attachments: PendingAttachment[];
  platformModelName: string;
  options: ConversationOptions;
  selectedToolIDs: number[];
  selectedSkills: SkillSummaryDTO[];
  selectedKnowledgeBaseIDs: string[];
  htmlVisualPromptEnabled: boolean;
};

export function clearCancelSettlementTimer(active: ActiveStream) {
  if (active.cancelSettlementTimer === null) {
    return;
  }
  window.clearTimeout(active.cancelSettlementTimer);
  active.cancelSettlementTimer = null;
}

export function replaceCompletedBranchSelection(
  previous: Record<string, string>,
  branch: Pick<
    PendingExchange,
    "parentPublicID" | "tempUserPublicID" | "tempAssistantPublicID" | "reuseUserMessage"
  >,
  userPublicID: string,
  assistantPublicID: string,
): Record<string, string> {
  const next = { ...previous };
  let changed = false;
  const parentKey = toBranchKey(branch.parentPublicID);
  const tempUserPublicID = branch.tempUserPublicID;
  const tempAssistantPublicID = branch.tempAssistantPublicID;

  if (!branch.reuseUserMessage && next[parentKey] === tempUserPublicID) {
    next[parentKey] = userPublicID;
    changed = true;
  }
  if (next[tempUserPublicID] === tempAssistantPublicID) {
    delete next[tempUserPublicID];
    if (!branch.reuseUserMessage && next[parentKey] === userPublicID) {
      next[userPublicID] = assistantPublicID;
    }
    changed = true;
  }
  if (branch.reuseUserMessage && next[toBranchKey(userPublicID)] === tempAssistantPublicID) {
    next[toBranchKey(userPublicID)] = assistantPublicID;
    changed = true;
  }
  return changed ? next : previous;
}

export function buildBranchScopePath(messages: ChatAreaMessage[]): string[] {
  return messages.map((message) => message.publicID.trim()).filter(Boolean);
}

export function buildSubmissionBranchScopePath(
  messages: ChatAreaMessage[],
  parentMessagePublicID: string | null | undefined,
): string[] {
  const visiblePath = buildBranchScopePath(messages);
  const parentPublicID = parentMessagePublicID?.trim() || "";
  if (!parentPublicID) {
    return [];
  }
  const parentIndex = visiblePath.indexOf(parentPublicID);
  return parentIndex >= 0 ? visiblePath.slice(0, parentIndex + 1) : visiblePath;
}

export function branchScopePathsEqual(left: readonly string[], right: readonly string[]): boolean {
  return left.length === right.length && left.every((publicID, index) => publicID === right[index]);
}

export function branchScopesEqual(left: BranchScope, right: BranchScope): boolean {
  return (
    left.conversationScopeKey === right.conversationScopeKey &&
    left.branchScopeRunID === right.branchScopeRunID &&
    branchScopePathsEqual(left.branchScopePath, right.branchScopePath)
  );
}

export function branchScopeID(scope: BranchScope): string {
  return JSON.stringify([
    scope.conversationScopeKey,
    scope.branchScopeRunID,
    ...scope.branchScopePath,
  ]);
}

export function isSuccessfulBranchParentStatus(status: string | null | undefined): boolean {
  const normalized = status?.trim().toLowerCase() || "";
  return normalized === "success" || normalized === "interrupted";
}

export function isFailedBranchParentStatus(status: string | null | undefined): boolean {
  const normalized = status?.trim().toLowerCase() || "";
  return ["error", "canceled", "cancelled", "blocked", "unavailable"].includes(normalized);
}

export function branchScopeIsVisible(
  scope: BranchScope,
  visibleConversationScopeKey: string,
  visibleMessages: ChatAreaMessage[],
): boolean {
  return (
    scope.conversationScopeKey === visibleConversationScopeKey &&
    visibleMessages.some((message) => message.runID === scope.branchScopeRunID)
  );
}

export function findSuccessfulBranchParentMessage(
  messages: ChatAreaMessage[],
  runID: string | null | undefined,
): ChatAreaMessage | undefined {
  const normalizedRunID = runID?.trim() || "";
  if (!normalizedRunID) {
    return undefined;
  }
  return messages.find(
    (message) =>
      message.role === "assistant" &&
      message.runID === normalizedRunID &&
      Boolean(resolvePersistedPublicID(message.publicID)) &&
      !message.isPending &&
      !message.isStreaming &&
      isSuccessfulBranchParentStatus(message.status),
  );
}

export function branchRunIsVisible(
  scope: BranchScope,
  runID: string | null | undefined,
  visibleConversationScopeKey: string,
  visibleBranchScopePath: readonly string[],
  visibleMessages: ChatAreaMessage[],
): boolean {
  const normalizedRunID = runID?.trim() || "";
  if (scope.conversationScopeKey !== visibleConversationScopeKey) {
    return false;
  }
  if (normalizedRunID && visibleMessages.some((message) => message.runID === normalizedRunID)) {
    return true;
  }
  return (
    branchScopePathsEqual(scope.branchScopePath, visibleBranchScopePath) &&
    (scope.branchScopeRunID === normalizedRunID ||
      branchScopeIsVisible(scope, visibleConversationScopeKey, visibleMessages))
  );
}

export function findVisibleActiveStreamByRunID(
  activeStreams: Map<string, ActiveStream>,
  runID: string,
  visibleConversationScopeKey: string,
  visibleBranchScopePath: readonly string[],
  visibleMessages: ChatAreaMessage[],
): ActiveStream | undefined {
  const candidate = runID ? activeStreams.get(runID) : undefined;
  if (
    candidate &&
    branchRunIsVisible(
      candidate,
      candidate.runID,
      visibleConversationScopeKey,
      visibleBranchScopePath,
      visibleMessages,
    )
  ) {
    return candidate;
  }
  return undefined;
}

export function findLastVisibleActiveStream(
  activeStreams: Map<string, ActiveStream>,
  visibleConversationScopeKey: string,
  visibleBranchScopePath: readonly string[],
  visibleMessages: ChatAreaMessage[],
): ActiveStream | undefined {
  return Array.from(activeStreams.values())
    .filter((item) =>
      branchRunIsVisible(
        item,
        item.runID,
        visibleConversationScopeKey,
        visibleBranchScopePath,
        visibleMessages,
      ),
    )
    .at(-1);
}

export function rechainQueuedSubmissions(
  submissions: QueuedChatSubmission[],
  scope: BranchScope,
  rootParentRunID: string | null,
  rootParentMessagePublicID: string | null,
): QueuedChatSubmission[] {
  let parentRunID = rootParentRunID;
  let firstSubmission = true;
  return submissions.map((submission) => {
    if (!branchScopesEqual(submission, scope)) {
      return submission;
    }
    const parentMessagePublicID = firstSubmission
      ? rootParentMessagePublicID
      : submission.parentMessagePublicID;
    const nextSubmission =
      submission.parentRunID === parentRunID &&
      submission.parentMessagePublicID === parentMessagePublicID
        ? submission
        : { ...submission, parentRunID, parentMessagePublicID };
    parentRunID = submission.clientRunID;
    firstSubmission = false;
    return nextSubmission;
  });
}

export function createClientRunID(): string {
  const randomID = createSecureUUID().replaceAll("-", "");
  return `run_${randomID}`.slice(0, 64);
}
