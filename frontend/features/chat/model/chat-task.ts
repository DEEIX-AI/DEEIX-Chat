import type { ChatModelOption, PendingAttachment } from "@/features/chat/types/chat-runtime";

export type ChatSubmitTask = "chat" | "image_generation" | "image_edit";
export type ChatSubmitBlockReason =
  | "image_edit_input_required"
  | "image_edit_unsupported"
  | "image_generation_rejects_attachments"
  | "image_task_rejects_non_image_attachments"
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

export function resolveChatSubmitDecision(
  model: ChatModelOption | null,
  attachments: PendingAttachment[],
): ChatSubmitDecision {
  const kinds = new Set(model?.kinds ?? []);
  const attachmentCount = attachments.length;
  const imageAttachmentCount = attachments.filter(isImageAttachment).length;
  const nonImageAttachmentCount = attachmentCount - imageAttachmentCount;
  const supportsChat = kinds.size === 0 || kinds.has("chat") || kinds.has("audio");
  const supportsImageGeneration = kinds.has("image_gen");
  const supportsImageEdit = kinds.has("image_edit");
  const baseDecision = {
    attachmentCount,
    imageAttachmentCount,
    nonImageAttachmentCount,
    supportsChat,
    supportsImageGeneration,
    supportsImageEdit,
  };

  if (
    nonImageAttachmentCount > 0 &&
    (supportsImageGeneration || supportsImageEdit) &&
    (imageAttachmentCount > 0 || !supportsChat)
  ) {
    if (imageAttachmentCount > 0 && supportsImageEdit) {
      return buildDecision("image_edit", "image_task_rejects_non_image_attachments", baseDecision);
    }
    if (supportsImageGeneration) {
      return buildDecision("image_generation", "image_task_rejects_non_image_attachments", baseDecision);
    }
    return buildDecision("chat", "image_task_rejects_non_image_attachments", baseDecision);
  }

  if (imageAttachmentCount > 0) {
    if (supportsImageEdit) {
      return buildDecision("image_edit", null, baseDecision);
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
    return buildDecision("chat", "model_task_unsupported", baseDecision);
  }

  if (supportsImageGeneration) {
    return buildDecision("image_generation", null, baseDecision);
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
  return task === "image_generation" || task === "image_edit";
}
