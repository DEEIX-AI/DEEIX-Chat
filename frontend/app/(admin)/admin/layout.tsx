import type { ReactNode } from "react";

import { AdminShell } from "@/features/admin/components/admin-shell";

export default function AdminLayout({ children }: { children: ReactNode }) {
  return <AdminShell basePath="/admin">{children}</AdminShell>;
}
