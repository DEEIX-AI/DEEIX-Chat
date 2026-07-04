import type { ChatModelOption, PendingAttachment } from "@/features/chat/types/chat-runtime";
import type { ConversationOptions } from "@/shared/api/conversation.types";

export type ChatSubmitTask = "chat" | "image_generation" | "image_edit" | "video_generation";
export type ChatSubmitBlockReason =
  | "image_edit_input_required"
  | "image_edit_unsupported"
  | "image_generation_rejects_attachments"
  | "image_task_rejects_non_image_attachments"
  | "video_generation_too_many_images"
  | "video_task_rejects_non_image_attachments"
  | "model_task_unsupported";

export type ChatSubmitDecision = {
  task: ChatSubmitTask;
  blockedReason: ChatSubmitBlockReason | null;
  attachmentCount: number;
  imageAttachmentCount: number;
  nonImageAttachmentCount: number;
  supportsChat: boolean;
  supportsImageGeneration: boolean;
  supportsImageEdit: boolean;
  supportsVideoGeneration: boolean;
};

function isImageAttachment(item: PendingAttachment): boolean {
  return item.fileCategory === "image" || item.mimeType.toLowerCase().startsWith("image/");
}

function buildDecision(
  task: ChatSubmitTask,
  blockedReason: ChatSubmitBlockReason | null,
  input: Omit<ChatSubmitDecision, "task" | "blockedReason">,
): ChatSubmitDecision {
  return {
    task,
    blockedReason,
    ...input,
  };
}

function optionObject(value: unknown): Record<string, unknown> | null {
  return value !== null && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function responseFormatType(value: unknown): "image" | "video" | "text" | "" {
  if (Array.isArray(value)) {
    if (value.some((item) => responseFormatType(item) === "video")) return "video";
    if (value.some((item) => responseFormatType(item) === "image")) return "image";
    if (value.some((item) => responseFormatType(item) === "text")) return "text";
    return "";
  }
  const format = optionObject(value);
  const type = typeof format?.type === "string" ? format.type.trim().toLowerCase() : "";
  if (type === "image" || type === "video" || type === "text") {
    return type;
  }
  return "";
}

function requestedResponseType(options?: ConversationOptions): "image" | "video" | "text" | "" {
  if (!options) {
    return "";
  }
  return responseFormatType(options.response_format ?? options.responseFormat);
}

export function resolveChatSubmitDecision(
  model: ChatModelOption | null,
  attachments: PendingAttachment[],
  options?: ConversationOptions,
): ChatSubmitDecision {
  const kinds = new Set(model?.kinds ?? []);
  const attachmentCount = attachments.length;
  const imageAttachmentCount = attachments.filter(isImageAttachment).length;
  const nonImageAttachmentCount = attachmentCount - imageAttachmentCount;
  const supportsChat = kinds.size === 0 || kinds.has("chat") || kinds.has("audio");
  const supportsImageGeneration = kinds.has("image_gen");
  const supportsImageEdit = kinds.has("image_edit");
  const supportsVideoGeneration = kinds.has("video_gen");
  const baseDecision = {
    attachmentCount,
    imageAttachmentCount,
    nonImageAttachmentCount,
    supportsChat,
    supportsImageGeneration,
    supportsImageEdit,
    supportsVideoGeneration,
  };
  const requestedType = requestedResponseType(options);

  if (
    nonImageAttachmentCount > 0 &&
    (supportsImageGeneration || supportsImageEdit || supportsVideoGeneration) &&
    (imageAttachmentCount > 0 || !supportsChat)
  ) {
    if (imageAttachmentCount > 0 && supportsImageEdit) {
      return buildDecision("image_edit", "image_task_rejects_non_image_attachments", baseDecision);
    }
    if (supportsVideoGeneration) {
      return buildDecision("video_generation", "video_task_rejects_non_image_attachments", baseDecision);
    }
    if (supportsImageGeneration) {
      return buildDecision("image_generation", "image_task_rejects_non_image_attachments", baseDecision);
    }
    return buildDecision("chat", "image_task_rejects_non_image_attachments", baseDecision);
  }

  if (imageAttachmentCount > 0) {
    if (requestedType === "video" && supportsVideoGeneration) {
      if (imageAttachmentCount > 1) {
        return buildDecision("video_generation", "video_generation_too_many_images", baseDecision);
      }
      return buildDecision("video_generation", null, baseDecision);
    }
    if (supportsImageEdit) {
      return buildDecision("image_edit", null, baseDecision);
    }
    if (supportsVideoGeneration) {
      if (imageAttachmentCount > 1) {
        return buildDecision("video_generation", "video_generation_too_many_images", baseDecision);
      }
      return buildDecision("video_generation", null, baseDecision);
    }
    if (supportsChat) {
      return buildDecision("chat", null, baseDecision);
    }
    return buildDecision("chat", "image_edit_unsupported", baseDecision);
  }

  if (attachmentCount > 0) {
    if (supportsChat) {
      return buildDecision("chat", null, baseDecision);
    }
    if (supportsImageGeneration) {
      return buildDecision("image_generation", "image_generation_rejects_attachments", baseDecision);
    }
    if (supportsVideoGeneration) {
      return buildDecision("video_generation", "video_task_rejects_non_image_attachments", baseDecision);
    }
    return buildDecision("chat", "model_task_unsupported", baseDecision);
  }

  if (requestedType === "video" && supportsVideoGeneration) {
    return buildDecision("video_generation", null, baseDecision);
  }
  if (requestedType === "image" && supportsImageGeneration) {
    return buildDecision("image_generation", null, baseDecision);
  }
  if (supportsChat) {
    return buildDecision("chat", null, baseDecision);
  }
  if (supportsImageGeneration) {
    return buildDecision("image_generation", null, baseDecision);
  }
  if (supportsVideoGeneration) {
    return buildDecision("video_generation", null, baseDecision);
  }
  if (supportsImageEdit && !supportsChat) {
    return buildDecision("image_edit", "image_edit_input_required", baseDecision);
  }
  if (!supportsChat) {
    return buildDecision("chat", "model_task_unsupported", baseDecision);
  }

  return buildDecision("chat", null, baseDecision);
}

export function isMediaSubmitTask(task: ChatSubmitTask): boolean {
  return task === "image_generation" || task === "image_edit" || task === "video_generation";
}
