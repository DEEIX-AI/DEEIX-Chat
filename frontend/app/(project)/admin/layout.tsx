import type { ReactNode } from "react";

import { AdminAccessGate, AdminShell } from "@/features/admin";

export default function AdminLayout({ children }: { children: ReactNode }) {
  return (
    <AdminAccessGate>
      <AdminShell basePath="/admin">{children}</AdminShell>
    </AdminAccessGate>
  );
}
