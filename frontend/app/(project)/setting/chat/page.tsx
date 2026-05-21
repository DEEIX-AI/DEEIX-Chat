import { AppSettingsPanel } from "@/features/settings/components/app-settings-panel";
import { SettingsChat } from "@/features/settings/components/sections/settings-chat";

export default function SettingsChatPage() {
  return (
    <AppSettingsPanel activeSection="chat" basePath="/setting">
      <SettingsChat />
    </AppSettingsPanel>
  );
}
