import { AdminShell } from "@/features/admin/components/admin-shell";
import { AdminUpstreamsPage } from "@/features/admin/components/sections/upstreams/admin-upstreams";

export default function AdminChannelsPage() {
  return (
    <AdminShell activeSection="channels" basePath="/admin">
      <AdminUpstreamsPage />
    </AdminShell>
  );
}
