import type { ReactNode } from "react";

import { AdminAccessGate, AdminShell } from "@/features/admin";
import { ProjectWorkspace } from "@/features/layouts";

export default function AdminLayout({ children }: { children: ReactNode }) {
  return (
    <ProjectWorkspace>
      <AdminAccessGate>
        <AdminShell basePath="/admin">{children}</AdminShell>
      </AdminAccessGate>
    </ProjectWorkspace>
  );
}
