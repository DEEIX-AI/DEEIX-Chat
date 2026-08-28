import type { useTranslations } from "next-intl";
import type { ChatSubmitBlockReason } from "@/features/chat/model/chat-task";
import type { ImageLoadingAspectRatio } from "@/features/chat/types/messages";
import type { ConversationOptions, StreamMessageEvent } from "@/shared/api/conversation.types";
import { ApiError } from "@/shared/api/http-client";

export function resolveSubmitBlockDescription(
  reason: ChatSubmitBlockReason,
  t: (key: string) => string,
): string {
  return t(`mediaInputBlocked.${reason}`);
}

export function resolveImageLoadingAspectRatio(
  options: ConversationOptions,
): ImageLoadingAspectRatio {
  const rawSize = typeof options.size === "string" ? options.size.trim() : "";
  const match = rawSize.match(/^(\d+)\s*x\s*(\d+)$/i);
  if (!match) {
    return "wide";
  }
  const width = Number(match[1]);
  const height = Number(match[2]);
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
    return "wide";
  }
  if (width > height) {
    return "wide";
  }
  if (height > width) {
    return "portrait";
  }
  return "square";
}

export function resolveVideoExtensionOptions(options: ConversationOptions): ConversationOptions {
  const duration = Number(options.duration);
  return {
    duration: Number.isInteger(duration) && duration >= 2 && duration <= 10 ? duration : 6,
  };
}

export function streamEventErrorToApiError(
  event: Extract<StreamMessageEvent, { type: "error" }>,
  fallback: string,
): ApiError {
  return new ApiError(event.message || fallback, event.status ?? 502, event.debug, event.errorCode);
}

export function resolveMediaStatusLabel(
  status: string,
  fallbackMessage: string,
  contentType: string | undefined,
  t: ReturnType<typeof useTranslations>,
): string {
  switch (status.trim()) {
    case "queued":
      if (contentType === "video") {
        return t("mediaStatus.videoQueued");
      }
      return t("mediaStatus.queued");
    case "running":
      if (contentType === "video") {
        return t("mediaStatus.videoRunning");
      }
      return t("mediaStatus.running");
    case "saving_artifact":
      if (contentType === "video") {
        return t("mediaStatus.videoSavingArtifact");
      }
      return t("mediaStatus.savingArtifact");
    default:
      return fallbackMessage.trim() || status.trim();
  }
}
