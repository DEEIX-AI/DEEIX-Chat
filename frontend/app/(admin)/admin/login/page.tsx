import { AdminShell } from "@/features/admin/components/admin-shell";
import { AdminLoginSettingsPage } from "@/features/admin/components/sections/settings/admin-settings-login";

export default function Page() {
  return (
    <AdminShell activeSection="login-settings" basePath="/admin">
      <AdminLoginSettingsPage />
    </AdminShell>
  );
}
