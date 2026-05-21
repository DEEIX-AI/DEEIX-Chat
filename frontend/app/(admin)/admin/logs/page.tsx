import { AdminShell } from "@/features/admin/components/admin-shell";
import { AdminLogsPage as AdminLogsSection } from "@/features/admin/components/sections/logs/admin-logs";

export default function AdminLogsPage() {
  return (
    <AdminShell activeSection="logs" basePath="/admin">
      <AdminLogsSection />
    </AdminShell>
  );
}
