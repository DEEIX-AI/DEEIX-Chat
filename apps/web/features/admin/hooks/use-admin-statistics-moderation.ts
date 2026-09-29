import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { type DailyStat, getContentModerationStats } from "@/features/admin/api/content-moderation";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

/** Loads the daily content-moderation counters shown on the statistics page. */
export function useAdminStatisticsModeration() {
  const t = useTranslations("adminStatistics.moderation");
  const [loading, setLoading] = React.useState(true);
  const [items, setItems] = React.useState<DailyStat[]>([]);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      const res = await getContentModerationStats(token);
      setItems(res.items ?? []);
    } catch {
      toast.error(t("loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  React.useEffect(() => {
    void load();
  }, [load]);

  return { loading, items, load };
}
