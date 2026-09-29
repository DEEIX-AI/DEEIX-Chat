import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { listAdminUsers } from "@/features/admin/api";
import type { AdminUserDTO } from "@/features/admin/api/admin-types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { useDebouncedValue } from "@/shared/hooks/use-debounced-value";

/**
 * Searches users for the statistics subject picker. Only runs while the picker
 * is open on the user tab; the sequence guard drops responses superseded by a
 * newer query so fast typing never shows stale results.
 */
export function useAdminStatisticsSubjectUsers({ enabled, query }: { enabled: boolean; query: string }) {
  const t = useTranslations("adminStatistics.filters.subjectSelect");
  const debouncedQuery = useDebouncedValue(query.trim());
  const [users, setUsers] = React.useState<AdminUserDTO[]>([]);
  const [loading, setLoading] = React.useState(false);
  const requestSequenceRef = React.useRef(0);

  React.useEffect(() => {
    if (!enabled) return;
    const requestSequence = requestSequenceRef.current + 1;
    requestSequenceRef.current = requestSequence;
    setLoading(true);
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) return;
        const result = await listAdminUsers(token, {
          page: 1,
          pageSize: 20,
          query: debouncedQuery,
        });
        if (requestSequence === requestSequenceRef.current) {
          setUsers(result.results);
        }
      } catch (error) {
        if (requestSequence === requestSequenceRef.current) {
          toast.error(t("loadFailed"), { description: resolveAdminErrorMessage(error) });
        }
      } finally {
        if (requestSequence === requestSequenceRef.current) setLoading(false);
      }
    })();
  }, [debouncedQuery, enabled, t]);

  return { users, loading };
}
