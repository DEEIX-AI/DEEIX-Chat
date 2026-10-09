import type { Metadata } from "next";

import { ProviderImportPage } from "@/features/settings";

// Defence in depth: the key is scrubbed from the URL on load, and no request
// made from this page reveals where it was opened from.
export const metadata: Metadata = {
  referrer: "no-referrer",
  robots: { index: false, follow: false },
};

// Outside the (project) workspace on purpose: the page must take the key out of
// the URL fragment before the auth guard can redirect to login.
export default function Page() {
  return <ProviderImportPage />;
}
