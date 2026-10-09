import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import {
  deleteAdminPersonalProviders,
  listAdminPersonalProviders,
  setAdminPersonalProvidersSuspended,
  suspendAdminPersonalProviderHost,
} from "@/shared/api/personal-providers";
import type { AdminPersonalProviderDTO, PersonalProviderAffectedData } from "@/shared/api/personal-providers-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

const DEFAULT_PAGE_SIZE = 25;

type UserKeysConfirm =
  | { kind: "suspend"; ids: string[]; label: string }
  | { kind: "suspendHost"; host: string }
  | { kind: "delete"; ids: string[]; label: string };

async function requireToken(): Promise<string> {
  const token = await resolveAccessToken();
  if (!token) throw new Error("missing access token");
  return token;
}

/**
 * Every provider users have added, for governance. The admin API returns hosts
 * and key hints only; there is no path from here to a key.
 */
export function useAdminUserKeysProviders() {
  const t = useTranslations("adminUserKeys");
  const [items, setItems] = React.useState<AdminPersonalProviderDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(DEFAULT_PAGE_SIZE);
  const [query, setQueryValue] = React.useState("");
  const [status, setStatusValue] = React.useState("");
  const [loading, setLoading] = React.useState(true);
  const [acting, setActing] = React.useState(false);
  const [selected, setSelected] = React.useState<Set<string>>(() => new Set());
  const [confirm, setConfirm] = React.useState<UserKeysConfirm | null>(null);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const data = await listAdminPersonalProviders(await requireToken(), { query, status, page, pageSize });
      setItems(data.results ?? []);
      setTotal(data.total ?? 0);
      setSelected(new Set());
    } catch (error) {
      toast.error(resolveAdminErrorMessage(error, t("toasts.loadFailed")));
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, query, status, t]);

  React.useEffect(() => {
    void load();
  }, [load]);

  const run = React.useCallback(
    async (action: (token: string) => Promise<PersonalProviderAffectedData>) => {
      setActing(true);
      try {
        const { affected } = await action(await requireToken());
        toast.success(t("toasts.affected", { count: affected }));
        await load();
      } catch (error) {
        toast.error(resolveAdminErrorMessage(error, t("toasts.actionFailed")));
      } finally {
        setActing(false);
      }
    },
    [load, t],
  );

  const resume = React.useCallback(
    (ids: string[]) => run((token) => setAdminPersonalProvidersSuspended(token, ids, false)),
    [run],
  );

  const confirmPending = React.useCallback(async () => {
    const pending = confirm;
    setConfirm(null);
    if (!pending) return;
    if (pending.kind === "suspend") await run((token) => setAdminPersonalProvidersSuspended(token, pending.ids, true));
    if (pending.kind === "suspendHost") await run((token) => suspendAdminPersonalProviderHost(token, pending.host));
    if (pending.kind === "delete") await run((token) => deleteAdminPersonalProviders(token, pending.ids));
  }, [confirm, run]);

  const toggleSelected = React.useCallback((id: string, checked: boolean) => {
    setSelected((current) => {
      const next = new Set(current);
      if (checked) next.add(id);
      else next.delete(id);
      return next;
    });
  }, []);

  return {
    items,
    total,
    page,
    pageSize,
    pageCount: Math.max(1, Math.ceil(total / pageSize)),
    setPage,
    setPageSize: (size: number) => {
      setPageSize(size);
      setPage(1);
    },
    query,
    setQuery: (value: string) => {
      setQueryValue(value);
      setPage(1);
    },
    status,
    setStatus: (value: string) => {
      setStatusValue(value);
      setPage(1);
    },
    loading,
    acting,
    reload: load,
    selected,
    toggleSelected,
    selectAll: (checked: boolean) => setSelected(checked ? new Set(items.map((item) => item.id)) : new Set()),
    resume,
    confirm,
    setConfirm,
    confirmPending,
  };
}
