"use client";

import { AccountsUsers } from "./accounts-users";
import { useAdminAccounts } from "@/features/admin/hooks/use-admin-accounts";

export function AdminAccountsPage() {
  const accounts = useAdminAccounts();

  return (
    <div className="pb-10">
      <AccountsUsers
        items={accounts.users}
        total={accounts.total}
        page={accounts.page}
        setPage={accounts.setPage}
        pageSize={accounts.pageSize}
        setPageSize={accounts.setPageSize}
        pageCount={accounts.pageCount}
        query={accounts.query}
        setQuery={accounts.setQuery}
        loading={accounts.loading}
        onLoadUsers={accounts.loadUsers}
        onSetUsers={accounts.setUsersOptimistic}
        onSetTotal={accounts.setTotalOptimistic}
      />
    </div>
  );
}
