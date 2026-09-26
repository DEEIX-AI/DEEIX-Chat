export { AppearancePreferencesProvider } from "@/features/settings/components/appearance-preferences-provider";
export { AppearancePreferencesSync } from "@/features/settings/components/appearance-preferences-sync";
export { useSettingsChatPreferences } from "@/features/settings/hooks/use-settings-chat-preferences";

// 以下导出是 chat 等其它 feature 消费的偏好设置契约。
// 新增跨 feature 消费必须经本出口，不允许深入内部路径。
export { parseSendShortcut } from "@/features/settings/utils/chat-settings";
export { useChatFontPreference, useChatFontWeightPreference } from "@/features/settings/utils/chat-font";
export { useFontSizePreference } from "@/features/settings/utils/font-size";
export type { SendShortcut } from "@/features/settings/types/settings";
