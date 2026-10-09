import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import type { AdminErrorLogDTO } from "@/features/admin/api/admin-types";
import { type AdminErrorLogStatusClass, listAdminErrorLogs } from "@/features/admin/api/audit";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { useDebouncedValue } from "@/shared/hooks/use-debounced-value";

/** Pages through failed-request error logs; stale responses are dropped by request id. */
export function useAdminLogsErrors({
  page,
  pageSize,
  query,
  statusClass,
}: {
  page: number;
  pageSize: number;
  query: string;
  statusClass: AdminErrorLogStatusClass | "";
}) {
  const t = useTranslations("adminLogs.errors");
  const [loading, setLoading] = React.useState(true);
  const [items, setItems] = React.useState<AdminErrorLogDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const requestRef = React.useRef(0);
  const debouncedQuery = useDebouncedValue(query.trim());

  const load = React.useCallback(async () => {
    const requestID = ++requestRef.current;
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      const res = await listAdminErrorLogs(token, {
        page,
        pageSize,
        query: debouncedQuery || undefined,
        statusClass: statusClass || undefined,
      });
      if (requestRef.current !== requestID) return;
      setItems(res.results ?? []);
      setTotal(res.total ?? 0);
    } catch (error) {
      if (requestRef.current === requestID) {
        toast.error(t("loadFailed"), { description: resolveAdminErrorMessage(error) });
      }
    } finally {
      if (requestRef.current === requestID) setLoading(false);
    }
  }, [debouncedQuery, page, pageSize, statusClass, t]);

  React.useEffect(() => {
    void load();
    return () => {
      requestRef.current += 1;
    };
  }, [load]);

  return { loading, items, total, load };
}
