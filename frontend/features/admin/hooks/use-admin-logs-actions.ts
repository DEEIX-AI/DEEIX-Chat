"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import type {
  AdminAuditLogDTO,
  AdminConversationEventDTO,
  AdminPaymentOrderDTO,
  AdminRedemptionRecordDTO,
  AdminSystemEventDTO,
  AdminUsageLogDTO,
  AdminUserAuthEventDTO,
} from "@/features/admin/api/admin.types";
import {
  cleanupAdminConversationRuns,
  cleanupAdminLogs,
  getAdminConversationEvent,
  type AdminLogCleanupType,
} from "@/features/admin/api/audit";
import { getAdminBillingConfig } from "@/features/admin/api/billing";
import type { useAdminConversationEvents } from "@/features/admin/hooks/use-admin-logs";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  normalizeBillingDisplayCurrency,
  type BillingDisplayOptions,
} from "@/shared/lib/billing-display";

const RUN_CLEANUP_MAX_SELECTION = 100;

export type LogDetail =
  | { kind: "audit"; item: AdminAuditLogDTO }
  | { kind: "auth"; item: AdminUserAuthEventDTO }
  | { kind: "usage"; item: AdminUsageLogDTO }
  | { kind: "system"; item: AdminSystemEventDTO }
  | { kind: "order"; item: AdminPaymentOrderDTO }
  | { kind: "redemption"; item: AdminRedemptionRecordDTO }
  | { kind: "conversation"; item: AdminConversationEventDTO };

// cleanupDateToISOString 将日期输入转换为清理截止时间（当天 00:00 的 ISO 串），非法日期返回 null。
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

// useAdminConversationRunsCleanup 承载会话运行日志的选择与批量清理编排。
export function useAdminConversationRunsCleanup(logs: ReturnType<typeof useAdminConversationEvents>) {
  const t = useTranslations("adminLogs");
  const [selectedRunIDs, setSelectedRunIDs] = React.useState<Set<string>>(new Set());
  const [cleanupOpen, setCleanupOpen] = React.useState(false);
  const [cleanupPending, setCleanupPending] = React.useState(false);

  const visibleRunIDs = React.useMemo(
    () => [...new Set(logs.events.map((item) => item.runID.trim()).filter(Boolean))],
    [logs.events],
  );
  const allVisibleSelected = visibleRunIDs.length > 0 && visibleRunIDs.every((runID) => selectedRunIDs.has(runID));
  const someVisibleSelected = visibleRunIDs.some((runID) => selectedRunIDs.has(runID));

  React.useEffect(() => {
    setSelectedRunIDs(new Set());
  }, [logs.events]);

  const toggleRun = React.useCallback((runID: string, selected: boolean) => {
    if (!runID) return;
    setSelectedRunIDs((current) => {
      const next = new Set(current);
      if (selected) {
        if (next.size >= RUN_CLEANUP_MAX_SELECTION && !next.has(runID)) {
          toast.error(t("conversation.cleanup.maxSelection"));
          return current;
        }
        next.add(runID);
      } else {
        next.delete(runID);
      }
      return next;
    });
  }, [t]);

  const toggleVisibleRuns = React.useCallback((selected: boolean) => {
    if (!selected) {
      setSelectedRunIDs(new Set());
      return;
    }
    if (visibleRunIDs.length > RUN_CLEANUP_MAX_SELECTION) {
      toast.error(t("conversation.cleanup.maxSelection"));
    }
    setSelectedRunIDs(new Set(visibleRunIDs.slice(0, RUN_CLEANUP_MAX_SELECTION)));
  }, [t, visibleRunIDs]);

  const cleanupSelectedRuns = React.useCallback(async () => {
    const runIDs = [...selectedRunIDs];
    if (runIDs.length === 0) return;
    setCleanupPending(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      const result = await cleanupAdminConversationRuns(token, { runIDs });
      toast.success(t("conversation.cleanup.success", {
        runs: result.runCount,
        events: result.deletedCount,
      }));
      setCleanupOpen(false);
      setSelectedRunIDs(new Set());
      await logs.loadConversationEvents(logs.page, logs.pageSize);
    } catch (error) {
      toast.error(t("conversation.cleanup.failed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setCleanupPending(false);
    }
  }, [logs, selectedRunIDs, t]);

  return {
    selectedRunIDs,
    visibleRunIDs,
    allVisibleSelected,
    someVisibleSelected,
    cleanupOpen,
    setCleanupOpen,
    cleanupPending,
    toggleRun,
    toggleVisibleRuns,
    cleanupSelectedRuns,
  };
}

// useAdminLogCleanupDialog 承载日志清理弹窗的表单状态与提交编排。
export function useAdminLogCleanupDialog({
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

// useAdminBillingDisplayOptions 加载管理端计费显示配置（币种与汇率），失败时回退 USD。
export function useAdminBillingDisplayOptions(): BillingDisplayOptions {
  const [billingDisplay, setBillingDisplay] = React.useState<BillingDisplayOptions>({
    currency: "USD",
    usdToCnyRate: null,
  });

  React.useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) return;
        const result = await getAdminBillingConfig(token);
        if (cancelled) return;
        setBillingDisplay({
          currency: normalizeBillingDisplayCurrency(result.config.displayCurrency),
          usdToCnyRate: result.config.usdToCNYRate ?? null,
        });
      } catch {
        if (!cancelled) {
          setBillingDisplay({ currency: "USD", usdToCnyRate: null });
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  return billingDisplay;
}

// useAdminLogDetail 承载日志详情面板状态与会话事件详情的按需加载（带请求版本防竞态）。
export function useAdminLogDetail() {
  const t = useTranslations("adminLogs");
  const [detail, setDetail] = React.useState<LogDetail | null>(null);
  const [conversationDetailLoading, setConversationDetailLoading] = React.useState(false);
  const detailRequestRef = React.useRef(0);

  const openConversationDetail = React.useCallback(async (item: AdminConversationEventDTO) => {
    const requestID = detailRequestRef.current + 1;
    detailRequestRef.current = requestID;
    setDetail({ kind: "conversation", item });
    setConversationDetailLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      const loaded = await getAdminConversationEvent(token, item.id);
      if (detailRequestRef.current === requestID) {
        setDetail({ kind: "conversation", item: loaded });
      }
    } catch (error) {
      if (detailRequestRef.current === requestID) {
        toast.error(t("toast.conversationEventDetailLoadFailed"), { description: resolveAdminErrorMessage(error) });
      }
    } finally {
      if (detailRequestRef.current === requestID) {
        setConversationDetailLoading(false);
      }
    }
  }, [t]);

  const closeDetail = React.useCallback(() => {
    detailRequestRef.current += 1;
    setConversationDetailLoading(false);
    setDetail(null);
  }, []);

  return { detail, setDetail, conversationDetailLoading, openConversationDetail, closeDetail };
}
