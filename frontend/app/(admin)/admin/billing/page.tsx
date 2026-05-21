import { AdminShell } from "@/features/admin/components/admin-shell";
import { AdminBillingPage } from "@/features/admin/components/sections/billing/admin-billing";

export default function AdminBillingRoute() {
  return (
    <AdminShell activeSection="billing" basePath="/admin">
      <AdminBillingPage />
    </AdminShell>
  );
}
