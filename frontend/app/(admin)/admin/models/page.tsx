import { AdminShell } from "@/features/admin/components/admin-shell";
import { AdminModelsPage as AdminModelsSection } from "@/features/admin/components/sections/models/admin-models";

export default function AdminModelsPage() {
  return (
    <AdminShell activeSection="models" basePath="/admin">
      <AdminModelsSection />
    </AdminShell>
  );
}
