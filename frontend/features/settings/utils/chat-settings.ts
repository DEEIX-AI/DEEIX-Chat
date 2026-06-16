import { parseChatContentWidth } from "@/shared/model/chat-content-width";
import type { ChatInputHeight, ChatSettings, FileMode, ModelVendorGroup, SendShortcut } from "@/features/settings/types/settings";
import type { UserSettingsMap } from "@/shared/api/user-settings";
import type { PublicModelDTO } from "@/shared/api/model.types";
import { platformSendShortcut } from "@/shared/lib/platform-shortcuts";

const FILE_MODES: FileMode[] = ["auto", "full_context", "rag"];
const INPUT_HEIGHTS: ChatInputHeight[] = ["compact", "standard", "loose"];
const SEND_SHORTCUTS: SendShortcut[] = ["enter", "ctrl_enter", "meta_enter"];

export const DEFAULT_CHAT_SETTINGS: ChatSettings = {
  defaultModel: "",
  sendShortcut: "enter",
  showTokenUsage: true,
  showModelInfo: true,
  showLatency: true,
  showBillingCost: true,
  markdownRender: true,
  autoGenerateTitle: true,
  deleteFilesByDefault: false,
  contextCompactAuto: false,
  restoreDraftOnFailure: true,
  preserveConversationDrafts: true,
  inputHeight: "standard",
  contentWidth: "compact",
  fileMode: "auto",
};

export function parseChatSettings(map: UserSettingsMap): ChatSettings {
  const fileMode = map["chat.file_mode"];
  const inputHeight = map["chat.input_height"];
  const contentWidth = map["chat.content_width"];
  const sendShortcut = map["chat.send_on_enter"];

  return {
    defaultModel: map["chat.default_model"] ?? "",
    sendShortcut: parseSendShortcut(sendShortcut),
    showTokenUsage: map["chat.show_token_usage"] !== "false",
    showModelInfo: map["chat.show_model_info"] !== "false",
    showLatency: map["chat.show_latency"] !== "false",
    showBillingCost: map["chat.show_billing_cost"] !== "false",
    markdownRender: map["chat.markdown_render"] !== "false",
    autoGenerateTitle: map["chat.auto_generate_title"] !== "false",
    deleteFilesByDefault: map["chat.delete_conversation_files_by_default"] === "true",
    contextCompactAuto: map["chat.context_compact_auto"] === "true",
    restoreDraftOnFailure: map["chat.restore_draft_on_failure"] !== "false",
    preserveConversationDrafts: map["chat.preserve_conversation_drafts"] !== "false",
    inputHeight: INPUT_HEIGHTS.includes(inputHeight as ChatInputHeight) ? (inputHeight as ChatInputHeight) : "standard",
    contentWidth: parseChatContentWidth(contentWidth),
    fileMode: FILE_MODES.includes(fileMode as FileMode) ? (fileMode as FileMode) : "auto",
  };
}

export function parseSendShortcut(value: string | undefined): SendShortcut {
  if (value === "enter") {
    return "enter";
  }
  if (SEND_SHORTCUTS.includes(value as SendShortcut)) {
    return platformSendShortcut();
  }
  return "enter";
}

export function groupModelsByVendor(models: PublicModelDTO[]): ModelVendorGroup[] {
  const groups = new Map<string, PublicModelDTO[]>();

  for (const model of models) {
    const vendor = model.vendor || "other";
    const items = groups.get(vendor) ?? [];
    items.push(model);
    groups.set(vendor, items);
  }

  return Array.from(groups.entries());
}
