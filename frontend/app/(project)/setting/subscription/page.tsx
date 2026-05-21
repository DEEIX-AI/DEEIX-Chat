import { AppSettingsPanel } from "@/features/settings/components/app-settings-panel";
import { SettingsSubscription } from "@/features/settings/components/sections/settings-subscription";

export default function SettingsSubscriptionPage() {
  return (
    <AppSettingsPanel activeSection="subscription" basePath="/setting">
      <SettingsSubscription />
    </AppSettingsPanel>
  );
}
