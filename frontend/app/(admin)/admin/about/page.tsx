import { AdminShell } from "@/features/admin/components/admin-shell";
import { AdminAboutPage } from "@/features/admin/components/sections/admin-about";

export default function Page() {
  return (
    <AdminShell activeSection="about" basePath="/admin">
      <AdminAboutPage />
    </AdminShell>
  );
}
