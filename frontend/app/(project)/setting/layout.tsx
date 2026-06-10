import type { ReactNode } from "react";

import { AppSettingsPanel } from "@/features/settings/components/app-settings-panel";

export default function SettingsLayout({ children }: { children: ReactNode }) {
  return <AppSettingsPanel basePath="/setting">{children}</AppSettingsPanel>;
}
