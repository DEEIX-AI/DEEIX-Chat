import { AppSettingsPanel } from "@/features/settings/components/app-settings-panel";
import { SettingsAbout } from "@/features/settings/components/sections/settings-about";

export default function SettingsAboutPage() {
  return (
    <AppSettingsPanel activeSection="about" basePath="/setting">
      <SettingsAbout />
    </AppSettingsPanel>
  );
}
