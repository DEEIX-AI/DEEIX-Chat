import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { listAdminUsers } from "@/features/admin/api";
import type { UserDTO } from "@/shared/api/auth.types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";

const USERS_PAGE_SIZE_DEFAULT = 25;

type UseAdminAccountsState = {
  users: UserDTO[];
  total: number;
  page: number;
  pageSize: number;
  loading: boolean;
  loadUsers: (page: number, pageSize?: number) => Promise<void>;
  setUsersOptimistic: React.Dispatch<React.SetStateAction<UserDTO[]>>;
  setTotalOptimistic: React.Dispatch<React.SetStateAction<number>>;
};

export function useAdminAccounts(): UseAdminAccountsState {
  const t = useTranslations("adminUsers.toast");
  const [users, setUsers] = React.useState<UserDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(USERS_PAGE_SIZE_DEFAULT);
  const [loading, setLoading] = React.useState(true);
  const [, startTableTransition] = React.useTransition();
  const requestSeqRef = React.useRef(0);

  const loadUsers = React.useCallback(
    async (nextPage = 1, nextPageSize = pageSize) => {
      const requestSeq = requestSeqRef.current + 1;
      requestSeqRef.current = requestSeq;
      setLoading(true);
      try {
        const token = await resolveAccessToken();
        if (!token) {
          toast.error(t("sessionExpired"), { description: t("signInAgain") });
          return;
        }

        const data = await listAdminUsers(token, {
          page: nextPage,
          pageSize: nextPageSize,
        });
        if (requestSeq !== requestSeqRef.current) {
          return;
        }
        startTableTransition(() => {
          setUsers(data.results);
          setTotal(data.total);
          setPage(nextPage);
          setPageSize(nextPageSize);
        });
      } catch (error) {
        toast.error(t("usersLoadFailed"), { description: resolveAdminErrorMessage(error) });
      } finally {
        if (requestSeq === requestSeqRef.current) {
          setLoading(false);
        }
      }
    },
    [pageSize, startTableTransition, t],
  );

  React.useEffect(() => {
    void loadUsers(1);
  }, [loadUsers]);

  return {
    users,
    total,
    page,
    pageSize,
    loading,
    loadUsers,
    setUsersOptimistic: setUsers,
    setTotalOptimistic: setTotal,
  };
}
