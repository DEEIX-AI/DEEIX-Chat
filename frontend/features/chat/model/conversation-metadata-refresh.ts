import { getConversation } from "@/shared/api/conversation";
import type { ConversationDTO, SendMessageResult } from "@/shared/api/conversation.types";

const CONVERSATION_METADATA_REFRESH_MAX_WAIT_MS = 45_000;
const CONVERSATION_METADATA_REFRESH_INITIAL_DELAY_MS = 800;
const CONVERSATION_METADATA_REFRESH_MAX_DELAY_MS = 5_000;
const CONVERSATION_METADATA_REFRESH_BACKOFF = 1.5;

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => {
    window.setTimeout(resolve, ms);
  });
}

function normalizeLabelsJSON(value: string | null | undefined): string {
  const normalized = value?.trim();
  return normalized && normalized !== "null" ? normalized : "[]";
}

export function isPlaceholderConversationTitle(title: string): boolean {
  const value = title.trim().toLowerCase();
  return ["new chat", "新对话"].includes(value);
}

function isFallbackConversationTitle(title: string, fallbackTitle: string): boolean {
  const normalizedFallback = fallbackTitle.trim();
  return normalizedFallback !== "" && title.trim() === normalizedFallback;
}

export function conversationTitleFromFirstUserMessage(content: string): string {
  const value = content.trim().replace(/\s+/g, " ").replace(/^[\s"'`“”‘’]+|[\s"'`“”‘’]+$/g, "");
  if (!value) {
    return "";
  }
  return Array.from(value).slice(0, 16).join("").trim();
}

function hasPendingGeneratedConversationMetadata(
  item: ConversationDTO | null,
  autoGenerateLabels: boolean,
  fallbackTitle = "",
): boolean {
  return (
    !item ||
    isPlaceholderConversationTitle(item.title) ||
    isFallbackConversationTitle(item.title, fallbackTitle) ||
    (autoGenerateLabels && normalizeLabelsJSON(item.labelsJSON) === "[]")
  );
}

function hasGeneratedConversationMetadataChanged(
  previous: ConversationDTO | null,
  next: ConversationDTO,
): boolean {
  const previousTitle = previous?.title?.trim() ?? "";
  const nextTitle = next.title.trim();
  if (nextTitle && nextTitle !== previousTitle && !isPlaceholderConversationTitle(nextTitle)) {
    return true;
  }
  return normalizeLabelsJSON(next.labelsJSON) !== normalizeLabelsJSON(previous?.labelsJSON);
}

export function shouldPollGeneratedConversationMetadata(
  item: ConversationDTO | null,
  result: SendMessageResult | null | undefined,
  autoGenerateLabels: boolean,
  fallbackTitle = "",
): boolean {
  if (!hasPendingGeneratedConversationMetadata(item, autoGenerateLabels, fallbackTitle)) {
    return false;
  }
  const hint = result?.metadataRefreshHint?.trim();
  if (!hint) {
    return true;
  }
  return hint === "pending";
}

export async function refreshGeneratedConversationMetadata(
  accessToken: string,
  conversationPublicID: string,
  previous: ConversationDTO | null,
  autoGenerateLabels: boolean,
  fallbackTitle: string,
  touchByPublicID: (publicID: string, patch: Partial<ConversationDTO>) => void,
): Promise<void> {
  let elapsedMS = 0;
  let delayMS = CONVERSATION_METADATA_REFRESH_INITIAL_DELAY_MS;
  let current = previous;

  while (elapsedMS < CONVERSATION_METADATA_REFRESH_MAX_WAIT_MS) {
    const nextDelayMS = Math.min(delayMS, CONVERSATION_METADATA_REFRESH_MAX_WAIT_MS - elapsedMS);
    await sleep(nextDelayMS);
    elapsedMS += nextDelayMS;

    let latest: ConversationDTO;
    try {
      latest = await getConversation(accessToken, conversationPublicID);
    } catch {
      continue;
    }
    if (hasGeneratedConversationMetadataChanged(current, latest)) {
      touchByPublicID(conversationPublicID, latest);
      current = latest;
      if (!hasPendingGeneratedConversationMetadata(latest, autoGenerateLabels, fallbackTitle)) {
        return;
      }
    }

    delayMS = Math.min(
      Math.round(delayMS * CONVERSATION_METADATA_REFRESH_BACKOFF),
      CONVERSATION_METADATA_REFRESH_MAX_DELAY_MS,
    );
  }
}
