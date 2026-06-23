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
  setPage: (value: number) => void;
  pageSize: number;
  setPageSize: (value: number) => void;
  pageCount: number;
  query: string;
  setQuery: (value: string) => void;
  loading: boolean;
  loadUsers: () => Promise<void>;
  setUsersOptimistic: React.Dispatch<React.SetStateAction<UserDTO[]>>;
  setTotalOptimistic: React.Dispatch<React.SetStateAction<number>>;
};

export function useAdminAccounts(): UseAdminAccountsState {
  const t = useTranslations("adminUsers.toast");
  const [users, setUsers] = React.useState<UserDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSizeState] = React.useState(USERS_PAGE_SIZE_DEFAULT);
  const [query, setQueryState] = React.useState("");
  const [debouncedQuery, setDebouncedQuery] = React.useState("");
  const [loading, setLoading] = React.useState(true);
  const [, startTableTransition] = React.useTransition();
  const requestSeqRef = React.useRef(0);

  React.useEffect(() => {
    const timer = window.setTimeout(() => {
      setDebouncedQuery(query.trim());
    }, 250);
    return () => window.clearTimeout(timer);
  }, [query]);

  const loadUsers = React.useCallback(
    async () => {
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
          page,
          pageSize,
          query: debouncedQuery,
        });
        if (requestSeq !== requestSeqRef.current) {
          return;
        }
        startTableTransition(() => {
          setUsers(data.results);
          setTotal(data.total);
        });
      } catch (error) {
        toast.error(t("usersLoadFailed"), { description: resolveAdminErrorMessage(error) });
      } finally {
        if (requestSeq === requestSeqRef.current) {
          setLoading(false);
        }
      }
    },
    [debouncedQuery, page, pageSize, startTableTransition, t],
  );

  React.useEffect(() => {
    void loadUsers();
  }, [loadUsers]);

  const pageCount = Math.max(1, Math.ceil(total / pageSize));

  const setQuery = React.useCallback((value: string) => {
    setQueryState(value);
    setPage(1);
  }, []);

  const setPageSize = React.useCallback((value: number) => {
    setPageSizeState(value);
    setPage(1);
  }, []);

  return {
    users,
    total,
    page,
    setPage,
    pageSize,
    setPageSize,
    pageCount,
    query,
    setQuery,
    loading,
    loadUsers,
    setUsersOptimistic: setUsers,
    setTotalOptimistic: setTotal,
  };
}
