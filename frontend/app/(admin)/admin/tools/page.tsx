import { AdminShell } from "@/features/admin/components/admin-shell";
import { AdminToolSettingsPage } from "@/features/admin/components/sections/settings/admin-settings-tools";

export default function AdminToolSettingsRoute() {
  return (
    <AdminShell activeSection="tool-settings" basePath="/admin">
      <AdminToolSettingsPage />
    </AdminShell>
  );
}
