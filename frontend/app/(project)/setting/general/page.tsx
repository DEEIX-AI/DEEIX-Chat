import { AppSettingsPanel } from "@/features/settings/components/app-settings-panel";
import { SettingsGeneral } from "@/features/settings/components/sections/settings-general";

export default function Page() {
  return (
    <AppSettingsPanel activeSection="general" basePath="/setting">
      <SettingsGeneral />
    </AppSettingsPanel>
  );
}
