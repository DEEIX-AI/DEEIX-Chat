export { AppearancePreferencesProvider } from "@/features/settings/components/appearance-preferences-provider";
export { AppearancePreferencesSync } from "@/features/settings/components/appearance-preferences-sync";
export { useSettingsChatPreferences } from "@/features/settings/hooks/use-settings-chat-preferences";

// The exports below are the preference settings contract consumed by chat and other features.
// New cross-feature consumers must go through this entry point, never deep internal paths.
export { parseSendShortcut } from "@/features/settings/utils/chat-settings";
export { useChatFontPreference, useChatFontWeightPreference } from "@/features/settings/utils/chat-font";
export {
  readLocalAppearancePreferences,
  serializeAppearancePreferences,
} from "@/features/settings/utils/appearance-preferences";
export { useFontSizePreference } from "@/features/settings/utils/font-size";
export type { SendShortcut } from "@/features/settings/types/settings";
export { AppSettingsPanel } from "@/features/settings/components/app-settings-panel";
export { SettingsAbout } from "@/features/settings/components/sections/about/settings-about";
export { SettingsAccount } from "@/features/settings/components/sections/account/settings-account";
export { SettingsChat } from "@/features/settings/components/sections/chat/settings-chat";
export { ProviderImportPage } from "@/features/settings/components/provider-import-page";
export { SettingsModels } from "@/features/settings/components/sections/models/settings-models";
// The import-link format is documented for administrators too, so New API / One API sites can be set up.
export { ModelsImportLinkDialog } from "@/features/settings/components/sections/models/models-import-link-dialog";
export { SettingsGeneral } from "@/features/settings/components/sections/general/settings-general";
export { SettingsSubscription } from "@/features/settings/components/sections/subscription/settings-subscription";
