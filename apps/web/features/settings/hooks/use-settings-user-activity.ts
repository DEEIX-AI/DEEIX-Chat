import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { getUserActivity } from "@/shared/api/stats";
import type { UserActivityDailyDTO } from "@/shared/api/stats-types";

export type ActivityDay = {
  date: string;
  requests: number;
  tokens: number;
};

const ACTIVITY_DAYS = 365;

function parseActivityDay(value: UserActivityDailyDTO): ActivityDay {
  return {
    date: value.date,
    requests: Number(value.requestCount) || 0,
    tokens: Number(value.tokenUsage) || 0,
  };
}

function sortActivityDays(items: UserActivityDailyDTO[]): ActivityDay[] {
  return items.map(parseActivityDay).sort((left, right) => left.date.localeCompare(right.date));
}

/** Past-year daily activity, sorted by date; `null` while loading, `[]` after a failed load. */
export function useSettingsUserActivity(accessToken: string): ActivityDay[] | null {
  const t = useTranslations("settings.subscriptionPage.activity");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [days, setDays] = React.useState<ActivityDay[] | null>(null);

  React.useEffect(() => {
    const controller = new AbortController();
    getUserActivity(accessToken, { days: ACTIVITY_DAYS, signal: controller.signal })
      .then((items) => {
        if (!controller.signal.aborted) setDays(sortActivityDays(items ?? []));
      })
      .catch((error) => {
        if (controller.signal.aborted) return;
        toast.error(t("loadFailed"), { description: resolveErrorMessage(error, t("loadFailed")) });
        setDays([]);
      });
    return () => controller.abort();
  }, [accessToken, resolveErrorMessage, t]);

  return days;
}
