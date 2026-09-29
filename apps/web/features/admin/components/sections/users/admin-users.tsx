"use client";

import { UsersPanel } from "./users-panel";
import { useAdminUsers } from "@/features/admin/hooks/use-admin-users";

export function AdminUsersPage() {
  const accounts = useAdminUsers();

  return (
    <div className="pb-10">
      <UsersPanel
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
