import { parseChatContentWidth } from "@/entities/user-settings";
import type { ChatInputHeight, ChatSettings, FileMode, ModelPresentationGroup, SendShortcut } from "@/features/settings/types/settings";
import type { UserSettingsMap } from "@/shared/api/user-settings";
import type { PublicModelDTO } from "@/shared/api/model-types";
import { platformSendShortcut } from "@/shared/lib/platform-shortcuts";
import { resolveModelPresentationGroup } from "@/entities/model";
import { isOneOf } from "@/shared/lib/type-guards";

const isFileMode = isOneOf(["auto", "full_context", "rag"] as const satisfies readonly FileMode[]);
const isChatInputHeight = isOneOf(["compact", "standard", "loose"] as const satisfies readonly ChatInputHeight[]);
const isSendShortcut = isOneOf(["enter", "ctrl_enter", "meta_enter"] as const satisfies readonly SendShortcut[]);

export const DEFAULT_CHAT_SETTINGS: ChatSettings = {
  defaultModel: "",
  sendShortcut: "enter",
  showTokenUsage: true,
  showModelInfo: true,
  showLatency: true,
  showBillingCost: true,
  showProcessTrace: true,
  markdownRender: true,
  autoExpandThinking: true,
  autoExpandToolCalls: true,
  autoGenerateTitle: true,
  autoGenerateLabels: true,
  deleteFilesByDefault: false,
  contextCompactAuto: false,
  restoreDraftOnFailure: true,
  preserveConversationDrafts: true,
  reuseModelOptions: true,
  reasoningContentPassback: true,
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
    showProcessTrace: map["chat.show_process_trace"] !== "false",
    markdownRender: map["chat.markdown_render"] !== "false",
    autoExpandThinking: map["chat.auto_expand_thinking"] !== "false",
    autoExpandToolCalls: map["chat.auto_expand_tool_calls"] !== "false",
    autoGenerateTitle: map["chat.auto_generate_title"] !== "false",
    autoGenerateLabels: map["chat.auto_generate_labels"] !== "false",
    deleteFilesByDefault: map["chat.delete_conversation_files_by_default"] === "true",
    contextCompactAuto: map["chat.context_compact_auto"] === "true",
    restoreDraftOnFailure: map["chat.restore_draft_on_failure"] !== "false",
    preserveConversationDrafts: map["chat.preserve_conversation_drafts"] !== "false",
    reuseModelOptions: map["chat.reuse_model_options"] !== "false",
    reasoningContentPassback: map["chat.reasoning_content_passback"] !== "false",
    inputHeight: isChatInputHeight(inputHeight) ? inputHeight : "standard",
    contentWidth: parseChatContentWidth(contentWidth),
    fileMode: isFileMode(fileMode) ? fileMode : "auto",
  };
}

export function parseSendShortcut(value: string | undefined): SendShortcut {
  if (value === "enter") {
    return "enter";
  }
  if (isSendShortcut(value)) {
    return platformSendShortcut();
  }
  return "enter";
}

export function groupModelsForPresentation(models: PublicModelDTO[]): ModelPresentationGroup[] {
  const groups = new Map<string, PublicModelDTO[]>();

  for (const model of models) {
    const groupKey = resolveModelPresentationGroup(model).key;
    const items = groups.get(groupKey) ?? [];
    items.push(model);
    groups.set(groupKey, items);
  }

  return Array.from(groups.entries());
}
