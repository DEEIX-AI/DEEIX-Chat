import { AppSettingsPanel } from "@/features/settings/components/app-settings-panel";
import { SettingsAccount } from "@/features/settings/components/sections/settings-account";

export default function SettingsAccountPage() {
  return (
    <AppSettingsPanel activeSection="account" basePath="/setting">
      <SettingsAccount />
    </AppSettingsPanel>
  );
}
