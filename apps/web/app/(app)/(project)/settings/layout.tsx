import type { ReactNode } from "react";

import { AppSettingsPanel } from "@/features/settings";

export default function SettingsLayout({ children }: { children: ReactNode }) {
  return <AppSettingsPanel basePath="/settings">{children}</AppSettingsPanel>;
}
