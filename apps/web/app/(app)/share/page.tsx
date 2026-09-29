import { Suspense } from "react";

import { PublicSharePage } from "@/features/share";

export default function Page() {
  return (
    <Suspense fallback={null}>
      <PublicSharePage />
    </Suspense>
  );
}
