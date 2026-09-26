import { Suspense } from "react";

import { AdminLogsPage as AdminLogsSection } from "@/features/admin/components/sections/logs/admin-logs";

export default function AdminLogsPage() {
  return (
    <Suspense fallback={null}>
      <AdminLogsSection />
    </Suspense>
  );
}
