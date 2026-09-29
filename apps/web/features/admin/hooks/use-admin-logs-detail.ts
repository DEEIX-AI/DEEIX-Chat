"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import type {
  AdminAuditLogDTO,
  AdminConversationEventDTO,
  AdminPaymentOrderDTO,
  AdminRedemptionRecordDTO,
  AdminUsageLogDTO,
  AdminUserAuthEventDTO,
} from "@/features/admin/api/admin-types";
import { getAdminConversationEvent } from "@/features/admin/api/audit";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export type LogDetail =
  | { kind: "audit"; item: AdminAuditLogDTO }
  | { kind: "auth"; item: AdminUserAuthEventDTO }
  | { kind: "usage"; item: AdminUsageLogDTO }
  | { kind: "order"; item: AdminPaymentOrderDTO }
  | { kind: "redemption"; item: AdminRedemptionRecordDTO }
  | { kind: "conversation"; item: AdminConversationEventDTO };

// useAdminLogsDetail owns the log detail panel state and on-demand loading of conversation event details (request versioning guards against races).
export function useAdminLogsDetail() {
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
