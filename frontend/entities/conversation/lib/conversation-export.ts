"use client";

import type { ConversationExportDTO } from "@/shared/api/conversation.types";

function safeFileNamePart(value: string) {
  const normalized = value
    .trim()
    .replace(/[\\/:*?"<>|]+/g, "-")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");
  return normalized || "conversation";
}

function formatExportTimestamp(value: string) {
  const date = new Date(value);
  const source = Number.isNaN(date.getTime()) ? new Date() : date;
  const pad = (part: number) => String(part).padStart(2, "0");
  return [
    source.getFullYear(),
    pad(source.getMonth() + 1),
    pad(source.getDate()),
    "-",
    pad(source.getHours()),
    pad(source.getMinutes()),
    pad(source.getSeconds()),
  ].join("");
}

function resolveConversationExportFileName(data: ConversationExportDTO) {
  const title = data.conversation?.title?.trim() || data.conversation?.publicID || "conversation";
  return `conversation-${safeFileNamePart(title)}-${formatExportTimestamp(data.exportedAt)}.json`;
}

export function downloadConversationExport(data: ConversationExportDTO) {
  const blob = new Blob([`${JSON.stringify(data, null, 2)}\n`], {
    type: "application/json;charset=utf-8",
  });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = resolveConversationExportFileName(data);
  link.rel = "noopener";
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}
