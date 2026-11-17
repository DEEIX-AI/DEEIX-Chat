// Public entry of the user-settings entity; code outside entities/user-settings/ imports only this file.
// Unused re-exports are tree-shaken (package.json "sideEffects"), so UI and logic share one entry.
export {
  CHAT_CONTENT_ALIGN_OPTIONS,
  CHAT_CONTENT_WIDTH_OPTIONS,
  type ChatContentAlign,
  type ChatContentWidth,
  type ChatContentWidthOption,
  parseChatContentAlign,
  parseChatContentWidth,
  resolveChatContentAlignClassName,
  resolveChatContentWidthClassName,
} from "@/entities/user-settings/model/chat-content-width";
export {
  loadUserSettingsSnapshot,
  updateUserSettings,
  useUserSettings,
} from "@/entities/user-settings/model/user-settings-store";
