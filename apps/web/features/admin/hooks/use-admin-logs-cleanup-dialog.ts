"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { cleanupAdminLogs, type AdminLogCleanupType } from "@/features/admin/api/audit";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

// cleanupDateToISOString converts the date input to the cleanup cutoff (ISO string at 00:00 of that day); returns null for invalid dates.
export function cleanupDateToISOString(value: string): string | null {
  const [yearText, monthText, dayText] = value.trim().split("-");
  const year = Number.parseInt(yearText ?? "", 10);
  const month = Number.parseInt(monthText ?? "", 10);
  const day = Number.parseInt(dayText ?? "", 10);
  if (!year || !month || !day) {
    return null;
  }
  const date = new Date(year, month - 1, day, 0, 0, 0, 0);
  if (
    Number.isNaN(date.getTime()) ||
    date.getFullYear() !== year ||
    date.getMonth() !== month - 1 ||
    date.getDate() !== day
  ) {
    return null;
  }
  return date.toISOString();
}

// useAdminLogsCleanupDialog owns the log cleanup dialog's form state and submit orchestration.
export function useAdminLogsCleanupDialog({
  onOpenChange,
  onSuccess,
}: {
  onOpenChange: (open: boolean) => void;
  onSuccess: (type: AdminLogCleanupType) => void;
}) {
  const t = useTranslations("adminLogs.cleanup");
  const [logType, setLogType] = React.useState<AdminLogCleanupType>("audit");
  const [date, setDate] = React.useState("");
  const [pending, setPending] = React.useState(false);

  const handleOpenChange = React.useCallback((nextOpen: boolean) => {
    if (pending) {
      return;
    }
    onOpenChange(nextOpen);
    if (!nextOpen) {
      setLogType("audit");
      setDate("");
    }
  }, [onOpenChange, pending]);

  const submit = React.useCallback(async () => {
    const before = cleanupDateToISOString(date);
    if (!before) {
      return;
    }

    setPending(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      const result = await cleanupAdminLogs(token, { type: logType, before });
      toast.success(t("toast.success", { count: result.deletedCount }));
      onSuccess(logType);
      onOpenChange(false);
      setLogType("audit");
      setDate("");
    } catch (error) {
      toast.error(t("toast.failed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setPending(false);
    }
  }, [date, logType, onOpenChange, onSuccess, t]);

  return { logType, setLogType, date, setDate, pending, handleOpenChange, submit };
}
