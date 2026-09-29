import { TabStrip } from "@/features/desktop";

// Rendered in the desktop shell's "chrome" webview; never navigated to by users.
export default function Page() {
  return <TabStrip />;
}
