import { Suspense } from "react";

import { PublicSharePage } from "@/features/share/components/public-share-page";

export default function Page() {
  return (
    <Suspense fallback={null}>
      <PublicSharePage />
    </Suspense>
  );
}
