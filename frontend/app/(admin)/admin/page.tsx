import { AdminShell } from "@/features/admin/components/admin-shell";
import { AdminAccountsPage } from "@/features/admin/components/sections/accounts/admin-accounts";

export default function Page() {
  return (
    <AdminShell activeSection="accounts" basePath="/admin">
      <AdminAccountsPage />
    </AdminShell>
  );
}
