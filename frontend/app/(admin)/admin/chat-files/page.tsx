import { AdminShell } from "@/features/admin/components/admin-shell";
import { AdminFilesSettingsPage as AdminFilesSettingsSection } from "@/features/admin/components/sections/settings/admin-settings-files";

export default function AdminFilesSettingsPage() {
  return (
    <AdminShell activeSection="chat-files" basePath="/admin">
      <AdminFilesSettingsSection />
    </AdminShell>
  );
}
