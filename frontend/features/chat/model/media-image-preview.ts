import type { StreamMessageEvent } from "@/shared/api/conversation.types";

function escapeMarkdownImageAlt(value: string): string {
  return value.replaceAll("[", "").replaceAll("]", "").replaceAll("\n", " ").trim();
}

export function buildMediaImagePreviewMarkdown(
  event: Extract<StreamMessageEvent, { type: "media_image_delta" }>,
  fallbackAlt: string,
): string {
  const b64 = event.b64_json.trim();
  if (!b64) {
    return "";
  }
  const source = b64.startsWith("data:")
    ? b64
    : `data:${event.mime_type?.trim() || "image/png"};base64,${b64}`;
  const alt = escapeMarkdownImageAlt(event.revised_prompt?.trim() || fallbackAlt);
  return `![${alt}](${source})`;
}
