import { TabStrip } from "@/features/platform/components/tab-strip";

// Rendered in the desktop shell's "chrome" webview; never navigated to by users.
export default function Page() {
  return <TabStrip />;
}
