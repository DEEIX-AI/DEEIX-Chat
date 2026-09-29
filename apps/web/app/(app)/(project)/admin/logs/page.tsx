import { Suspense } from "react";

import { AdminLogsPage } from "@/features/admin";

export default function Page() {
  return (
    <Suspense fallback={null}>
      <AdminLogsPage />
    </Suspense>
  );
}
