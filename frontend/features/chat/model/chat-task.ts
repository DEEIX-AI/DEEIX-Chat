import type { ChatModelOption, PendingAttachment } from "@/features/chat/types/chat-runtime";

export type ChatSubmitTask = "chat" | "image_generation" | "image_edit";

export function resolveChatSubmitTask(
  model: ChatModelOption | null,
  attachments: PendingAttachment[],
): ChatSubmitTask {
  const kinds = new Set(model?.kinds ?? []);
  const hasImageAttachment = attachments.some((item) => item.fileCategory === "image");
  if (hasImageAttachment && kinds.has("image_edit")) {
    return "image_edit";
  }
  if (!hasImageAttachment && kinds.has("image_gen")) {
    return "image_generation";
  }
  return "chat";
}

export function isMediaSubmitTask(task: ChatSubmitTask): boolean {
  return task === "image_generation" || task === "image_edit";
}
