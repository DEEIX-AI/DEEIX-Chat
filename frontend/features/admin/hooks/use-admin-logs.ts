import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  listAdminAuditLogs,
  listAdminConversationEvents,
  listAdminPaymentOrders,
  listAdminSystemEvents,
  listAdminUsageLogs,
  listAdminUserAuthEvents,
} from "@/features/admin/api";
import { listAdminLLMModels } from "@/features/admin/api/llm";
import { listAllAdminPages } from "@/features/admin/api/shared";
import type {
  AdminAuditLogDTO,
  AdminConversationEventDTO,
  AdminPaymentOrderDTO,
  AdminSystemEventDTO,
  AdminUsageLogDTO,
  AdminUserAuthEventDTO,
} from "@/features/admin/api/admin.types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import type { ModelSelectOption } from "@/shared/components/model-select";
import { resolveModelOptionIconUrl } from "@/shared/lib/model-option-display";

export const ADMIN_LOGS_PAGE_SIZE = 25;

export const AUDIT_LOG_SORT_OPTIONS = [
  { labelKey: "sort.idDesc", value: "id_desc" },
  { labelKey: "sort.idAsc", value: "id_asc" },
  { labelKey: "sort.createdDesc", value: "created_desc" },
  { labelKey: "sort.createdAsc", value: "created_asc" },
] as const;

export const SECURITY_LOG_SORT_OPTIONS = [
  { labelKey: "sort.occurredDesc", value: "occurred_desc" },
  { labelKey: "sort.occurredAsc", value: "occurred_asc" },
  { labelKey: "sort.idDesc", value: "id_desc" },
  { labelKey: "sort.idAsc", value: "id_asc" },
] as const;

export const SYSTEM_EVENT_SORT_OPTIONS = [
  { labelKey: "sort.createdDesc", value: "created_desc" },
  { labelKey: "sort.createdAsc", value: "created_asc" },
  { labelKey: "sort.idDesc", value: "id_desc" },
  { labelKey: "sort.idAsc", value: "id_asc" },
] as const;

export const USAGE_LOG_SORT_OPTIONS = [
  { labelKey: "sort.callTimeDesc", value: "created_desc" },
  { labelKey: "sort.callTimeAsc", value: "created_asc" },
  { labelKey: "sort.costDesc", value: "cost_desc" },
  { labelKey: "sort.tokensDesc", value: "tokens_desc" },
  { labelKey: "sort.latencyDesc", value: "latency_desc" },
] as const;

export const PAYMENT_ORDER_SORT_OPTIONS = [
  { labelKey: "sort.createdDesc", value: "created_desc" },
  { labelKey: "sort.createdAsc", value: "created_asc" },
  { labelKey: "sort.paidDesc", value: "paid_desc" },
  { labelKey: "sort.amountDesc", value: "amount_desc" },
] as const;

export const CONVERSATION_EVENT_SORT_OPTIONS = [
  { labelKey: "sort.createdDesc", value: "created_desc" },
  { labelKey: "sort.createdAsc", value: "created_asc" },
  { labelKey: "sort.latencyDesc", value: "latency_desc" },
  { labelKey: "sort.sequenceAsc", value: "seq_asc" },
] as const;

export type AuditLogSortValue = (typeof AUDIT_LOG_SORT_OPTIONS)[number]["value"];
export type SecurityLogSortValue = (typeof SECURITY_LOG_SORT_OPTIONS)[number]["value"];
export type SystemEventSortValue = (typeof SYSTEM_EVENT_SORT_OPTIONS)[number]["value"];
export type UsageLogSortValue = (typeof USAGE_LOG_SORT_OPTIONS)[number]["value"];
export type PaymentOrderSortValue = (typeof PAYMENT_ORDER_SORT_OPTIONS)[number]["value"];
export type ConversationEventSortValue = (typeof CONVERSATION_EVENT_SORT_OPTIONS)[number]["value"];

const AUDIT_RESOURCE_VALUES = [
  "user",
  "conversation",
  "conversation_project",
  "message",
  "file",
  "memory",
  "settings",
] as const;

const AUDIT_RESOURCE_LABEL_KEYS: Record<string, string> = {
  user: "audit.resources.user",
  conversation: "audit.resources.conversation",
  conversation_project: "audit.resources.conversation_project",
  message: "audit.resources.message",
  file: "audit.resources.file",
  memory: "audit.resources.memory",
  settings: "audit.resources.settings",
};

const AUDIT_ACTION_VALUES = [
  "login",
  "stream_message",
  "create_conversation",
  "rename_conversation",
  "export_conversation",
  "delete_conversation",
  "set_conversation_star",
  "set_conversation_archive",
  "set_conversation_project",
  "batch_set_conversation_project",
  "create_conversation_project",
  "update_conversation_project",
  "delete_conversation_project",
  "reorder_conversation_projects",
  "create_conversation_share",
  "regenerate_conversation_share",
  "revoke_conversation_share",
  "revoke_conversation_shares",
  "clone_shared_conversation",
  "update_message",
  "set_message_feedback",
  "upload_file",
  "update_file",
  "delete_file",
  "upsert_user_memory",
  "delete_user_memory",
  "settings.update",
  "admin_create_user",
  "admin_patch_user",
  "admin_update_user_status",
  "admin_revoke_user_sessions",
  "admin_reset_user_password",
  "admin_reset_user_2fa",
  "admin_delete_user",
] as const;

const AUDIT_ACTION_LABEL_KEYS: Record<string, string> = {
  login: "audit.actions.login",
  stream_message: "audit.actions.stream_message",
  create_conversation: "audit.actions.create_conversation",
  rename_conversation: "audit.actions.rename_conversation",
  export_conversation: "audit.actions.export_conversation",
  delete_conversation: "audit.actions.delete_conversation",
  set_conversation_star: "audit.actions.set_conversation_star",
  set_conversation_archive: "audit.actions.set_conversation_archive",
  set_conversation_project: "audit.actions.set_conversation_project",
  batch_set_conversation_project: "audit.actions.batch_set_conversation_project",
  create_conversation_project: "audit.actions.create_conversation_project",
  update_conversation_project: "audit.actions.update_conversation_project",
  delete_conversation_project: "audit.actions.delete_conversation_project",
  reorder_conversation_projects: "audit.actions.reorder_conversation_projects",
  create_conversation_share: "audit.actions.create_conversation_share",
  regenerate_conversation_share: "audit.actions.regenerate_conversation_share",
  revoke_conversation_share: "audit.actions.revoke_conversation_share",
  revoke_conversation_shares: "audit.actions.revoke_conversation_shares",
  clone_shared_conversation: "audit.actions.clone_shared_conversation",
  update_message: "audit.actions.update_message",
  set_message_feedback: "audit.actions.set_message_feedback",
  upload_file: "audit.actions.upload_file",
  update_file: "audit.actions.update_file",
  delete_file: "audit.actions.delete_file",
  upsert_user_memory: "audit.actions.upsert_user_memory",
  delete_user_memory: "audit.actions.delete_user_memory",
  "settings.update": "audit.actions.settings_update",
  admin_create_user: "audit.actions.admin_create_user",
  admin_patch_user: "audit.actions.admin_patch_user",
  admin_update_user_status: "audit.actions.admin_update_user_status",
  admin_revoke_user_sessions: "audit.actions.admin_revoke_user_sessions",
  admin_reset_user_password: "audit.actions.admin_reset_user_password",
  admin_reset_user_2fa: "audit.actions.admin_reset_user_2fa",
  admin_delete_user: "audit.actions.admin_delete_user",
};

type UseAdminLogsState = {
  auditLogs: AdminAuditLogDTO[];
  total: number;
  page: number;
  pageSize: number;
  pageCount: number;
  loading: boolean;
  query: string;
  setQuery: (value: string) => void;
  resourceFilter: string;
  setResourceFilter: (value: string) => void;
  actionFilter: string;
  setActionFilter: (value: string) => void;
  createdFromFilter: string;
  setCreatedFromFilter: (value: string) => void;
  createdToFilter: string;
  setCreatedToFilter: (value: string) => void;
  sortValue: AuditLogSortValue;
  setSortValue: (value: AuditLogSortValue) => void;
  resourceOptions: Array<{ label: string; value: string }>;
  actionOptions: Array<{ label: string; value: string }>;
  loadAuditLogs: (page?: number, pageSize?: number) => Promise<void>;
};

type UseAdminSecurityLogsState = {
  events: AdminUserAuthEventDTO[];
  sortedEvents: AdminUserAuthEventDTO[];
  total: number;
  page: number;
  pageSize: number;
  pageCount: number;
  loading: boolean;
  query: string;
  setQuery: (value: string) => void;
  resultFilter: string;
  setResultFilter: (value: string) => void;
  sortValue: SecurityLogSortValue;
  setSortValue: (value: SecurityLogSortValue) => void;
  loadSecurityLogs: (page?: number, pageSize?: number) => Promise<void>;
};

type UseAdminSystemEventsState = {
  events: AdminSystemEventDTO[];
  total: number;
  page: number;
  pageSize: number;
  pageCount: number;
  loading: boolean;
  query: string;
  setQuery: (value: string) => void;
  levelFilter: string;
  setLevelFilter: (value: string) => void;
  sourceFilter: string;
  setSourceFilter: (value: string) => void;
  eventFilter: string;
  setEventFilter: (value: string) => void;
  createdFromFilter: string;
  setCreatedFromFilter: (value: string) => void;
  createdToFilter: string;
  setCreatedToFilter: (value: string) => void;
  sortValue: SystemEventSortValue;
  setSortValue: (value: SystemEventSortValue) => void;
  sourceOptions: Array<{ label: string; value: string }>;
  eventOptions: Array<{ label: string; value: string }>;
  loadSystemEvents: (page?: number, pageSize?: number) => Promise<void>;
};

type UseAdminUsageLogsState = {
  logs: AdminUsageLogDTO[];
  total: number;
  page: number;
  pageSize: number;
  pageCount: number;
  loading: boolean;
  query: string;
  setQuery: (value: string) => void;
  platformModelFilter: string;
  setPlatformModelFilter: (value: string) => void;
  billingModeFilter: string;
  setBillingModeFilter: (value: string) => void;
  createdFromFilter: string;
  setCreatedFromFilter: (value: string) => void;
  createdToFilter: string;
  setCreatedToFilter: (value: string) => void;
  sortValue: UsageLogSortValue;
  setSortValue: (value: UsageLogSortValue) => void;
  platformModelOptions: ModelSelectOption[];
  loadUsageLogs: (page?: number, pageSize?: number) => Promise<void>;
};

type UseAdminPaymentOrdersState = {
  orders: AdminPaymentOrderDTO[];
  total: number;
  page: number;
  pageSize: number;
  pageCount: number;
  loading: boolean;
  query: string;
  setQuery: (value: string) => void;
  orderTypeFilter: string;
  setOrderTypeFilter: (value: string) => void;
  providerFilter: string;
  setProviderFilter: (value: string) => void;
  statusFilter: string;
  setStatusFilter: (value: string) => void;
  createdFromFilter: string;
  setCreatedFromFilter: (value: string) => void;
  createdToFilter: string;
  setCreatedToFilter: (value: string) => void;
  sortValue: PaymentOrderSortValue;
  setSortValue: (value: PaymentOrderSortValue) => void;
  loadPaymentOrders: (page?: number, pageSize?: number) => Promise<void>;
};

type UseAdminConversationEventsState = {
  events: AdminConversationEventDTO[];
  total: number;
  page: number;
  pageSize: number;
  pageCount: number;
  loading: boolean;
  query: string;
  setQuery: (value: string) => void;
  eventScopeFilter: string;
  setEventScopeFilter: (value: string) => void;
  eventTypeFilter: string;
  setEventTypeFilter: (value: string) => void;
  statusFilter: string;
  setStatusFilter: (value: string) => void;
  createdFromFilter: string;
  setCreatedFromFilter: (value: string) => void;
  createdToFilter: string;
  setCreatedToFilter: (value: string) => void;
  sortValue: ConversationEventSortValue;
  setSortValue: (value: ConversationEventSortValue) => void;
  eventTypeOptions: Array<{ label: string; value: string }>;
  loadConversationEvents: (page?: number, pageSize?: number) => Promise<void>;
};

function parsePositiveInt(value: string): number | undefined {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined;
}

function toRFC3339DateRangeBound(value: string, bound: "start" | "end"): string | undefined {
  if (!value.trim()) {
    return undefined;
  }
  const dateOnly = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value.trim());
  if (dateOnly) {
    const year = Number.parseInt(dateOnly[1], 10);
    const month = Number.parseInt(dateOnly[2], 10);
    const day = Number.parseInt(dateOnly[3], 10);
    const date =
      bound === "start"
        ? new Date(year, month - 1, day, 0, 0, 0, 0)
        : new Date(year, month - 1, day, 23, 59, 59, 999);
    return Number.isNaN(date.getTime()) ? undefined : date.toISOString();
  }

  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString();
}

function appendObservedAuditOptions(
  baseValues: readonly string[],
  observedValues: string[],
  labelForValue: (value: string) => string,
  allLabel: string,
): Array<{ label: string; value: string }> {
  const orderedValues = new Set<string>(baseValues);
  for (const value of observedValues) {
    const normalized = value.trim();
    if (normalized) {
      orderedValues.add(normalized);
    }
  }
  return [
    { label: allLabel, value: "" },
    ...Array.from(orderedValues).map((value) => ({
      label: labelForValue(value),
      value,
    })),
  ];
}

export function useAdminLogs(): UseAdminLogsState {
  const t = useTranslations("adminLogs");
  const [auditLogs, setAuditLogs] = React.useState<AdminAuditLogDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(ADMIN_LOGS_PAGE_SIZE);
  const [loading, setLoading] = React.useState(true);
  const [query, setQueryState] = React.useState("");
  const [debouncedQuery, setDebouncedQuery] = React.useState("");
  const [resourceFilter, setResourceFilterState] = React.useState("");
  const [actionFilter, setActionFilterState] = React.useState("");
  const [createdFromFilter, setCreatedFromFilterState] = React.useState("");
  const [createdToFilter, setCreatedToFilterState] = React.useState("");
  const [sortValue, setSortValueState] = React.useState<AuditLogSortValue>("id_desc");
  const requestSeqRef = React.useRef(0);

  React.useEffect(() => {
    const timer = window.setTimeout(() => {
      setDebouncedQuery(query.trim());
    }, 250);
    return () => window.clearTimeout(timer);
  }, [query]);

  const loadAuditLogs = React.useCallback(async (nextPage = 1, nextPageSize = pageSize) => {
    const requestSeq = requestSeqRef.current + 1;
    requestSeqRef.current = requestSeq;
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }

      const data = await listAdminAuditLogs(token, {
        page: nextPage,
        pageSize: nextPageSize,
        query: /^\d+$/.test(debouncedQuery) ? undefined : debouncedQuery,
        resource: resourceFilter,
        action: actionFilter,
        actorUserID: parsePositiveInt(debouncedQuery),
        createdFrom: toRFC3339DateRangeBound(createdFromFilter, "start"),
        createdTo: toRFC3339DateRangeBound(createdToFilter, "end"),
        sort: sortValue,
      });
      if (requestSeq !== requestSeqRef.current) {
        return;
      }

      setAuditLogs(data.results);
      setTotal(data.total);
      setPage(nextPage);
      setPageSize(nextPageSize);
    } catch (error) {
      toast.error(t("toast.auditLoadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      if (requestSeq === requestSeqRef.current) {
        setLoading(false);
      }
    }
  }, [actionFilter, createdFromFilter, createdToFilter, debouncedQuery, pageSize, resourceFilter, sortValue, t]);

  React.useEffect(() => {
    void loadAuditLogs(1);
  }, [loadAuditLogs]);

  const setQuery = React.useCallback((value: string) => {
    setQueryState(value);
    setPage(1);
  }, []);

  const setResourceFilter = React.useCallback((value: string) => {
    setResourceFilterState(value);
    setPage(1);
  }, []);

  const setActionFilter = React.useCallback((value: string) => {
    setActionFilterState(value);
    setPage(1);
  }, []);

  const setCreatedFromFilter = React.useCallback((value: string) => {
    setCreatedFromFilterState(value);
    setPage(1);
  }, []);

  const setCreatedToFilter = React.useCallback((value: string) => {
    setCreatedToFilterState(value);
    setPage(1);
  }, []);

  const setSortValue = React.useCallback((value: AuditLogSortValue) => {
    setSortValueState(value);
    setPage(1);
  }, []);

  const resourceOptions = React.useMemo(() => {
    const observedValues = auditLogs.map((item) => item.resource);
    return appendObservedAuditOptions(
      AUDIT_RESOURCE_VALUES,
      observedValues,
      (value) => {
        const labelKey = AUDIT_RESOURCE_LABEL_KEYS[value];
        return labelKey ? t(labelKey) : value;
      },
      t("filters.allResources"),
    );
  }, [auditLogs, t]);

  const actionOptions = React.useMemo(() => {
    const observedValues = auditLogs.map((item) => item.action);
    return appendObservedAuditOptions(
      AUDIT_ACTION_VALUES,
      observedValues,
      (value) => {
        const labelKey = AUDIT_ACTION_LABEL_KEYS[value];
        return labelKey ? t(labelKey) : value;
      },
      t("filters.allActions"),
    );
  }, [auditLogs, t]);

  return {
    auditLogs,
    total,
    page,
    pageSize,
    pageCount: Math.max(1, Math.ceil(total / pageSize)),
    loading,
    query,
    setQuery,
    resourceFilter,
    setResourceFilter,
    actionFilter,
    setActionFilter,
    createdFromFilter,
    setCreatedFromFilter,
    createdToFilter,
    setCreatedToFilter,
    sortValue,
    setSortValue,
    resourceOptions,
    actionOptions,
    loadAuditLogs,
  };
}

export function useAdminSecurityLogs(): UseAdminSecurityLogsState {
  const t = useTranslations("adminLogs");
  const [events, setEvents] = React.useState<AdminUserAuthEventDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(ADMIN_LOGS_PAGE_SIZE);
  const [loading, setLoading] = React.useState(true);
  const [query, setQueryState] = React.useState("");
  const [debouncedQuery, setDebouncedQuery] = React.useState("");
  const [resultFilter, setResultFilterState] = React.useState("");
  const [sortValue, setSortValueState] = React.useState<SecurityLogSortValue>("occurred_desc");
  const requestSeqRef = React.useRef(0);

  React.useEffect(() => {
    const timer = window.setTimeout(() => {
      setDebouncedQuery(query.trim());
    }, 250);
    return () => window.clearTimeout(timer);
  }, [query]);

  const loadSecurityLogs = React.useCallback(async (nextPage = 1, nextPageSize = pageSize) => {
    const requestSeq = requestSeqRef.current + 1;
    requestSeqRef.current = requestSeq;
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }

      const data = await listAdminUserAuthEvents(token, {
        page: nextPage,
        pageSize: nextPageSize,
        userID: parsePositiveInt(debouncedQuery),
        eventType: /^\d+$/.test(debouncedQuery) ? undefined : debouncedQuery || undefined,
        result: resultFilter || undefined,
      });
      if (requestSeq !== requestSeqRef.current) {
        return;
      }

      setEvents(data.results);
      setTotal(data.total);
      setPage(nextPage);
      setPageSize(nextPageSize);
    } catch (error) {
      toast.error(t("toast.authLoadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      if (requestSeq === requestSeqRef.current) {
        setLoading(false);
      }
    }
  }, [debouncedQuery, pageSize, resultFilter, t]);

  React.useEffect(() => {
    void loadSecurityLogs(1);
  }, [loadSecurityLogs]);

  const setQuery = React.useCallback((value: string) => {
    setQueryState(value);
    setPage(1);
  }, []);

  const setResultFilter = React.useCallback((value: string) => {
    setResultFilterState(value);
    setPage(1);
  }, []);

  const setSortValue = React.useCallback((value: SecurityLogSortValue) => {
    setSortValueState(value);
    setPage(1);
  }, []);

  const sortedEvents = React.useMemo(() => {
    const next = [...events];
    const occurredTimestamps = new Map(next.map((item) => [item.id, new Date(item.occurredAt || 0).getTime()]));
    next.sort((left, right) => {
      switch (sortValue) {
        case "occurred_asc":
          return (occurredTimestamps.get(left.id) ?? 0) - (occurredTimestamps.get(right.id) ?? 0);
        case "id_desc":
          return right.id - left.id;
        case "id_asc":
          return left.id - right.id;
        case "occurred_desc":
        default:
          return (occurredTimestamps.get(right.id) ?? 0) - (occurredTimestamps.get(left.id) ?? 0);
      }
    });
    return next;
  }, [events, sortValue]);

  return {
    events,
    sortedEvents,
    total,
    page,
    pageSize,
    pageCount: Math.max(1, Math.ceil(total / pageSize)),
    loading,
    query,
    setQuery,
    resultFilter,
    setResultFilter,
    sortValue,
    setSortValue,
    loadSecurityLogs,
  };
}

export function useAdminSystemEvents(): UseAdminSystemEventsState {
  const t = useTranslations("adminLogs");
  const [events, setEvents] = React.useState<AdminSystemEventDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(ADMIN_LOGS_PAGE_SIZE);
  const [loading, setLoading] = React.useState(true);
  const [query, setQueryState] = React.useState("");
  const [debouncedQuery, setDebouncedQuery] = React.useState("");
  const [levelFilter, setLevelFilterState] = React.useState("");
  const [sourceFilter, setSourceFilterState] = React.useState("");
  const [eventFilter, setEventFilterState] = React.useState("");
  const [createdFromFilter, setCreatedFromFilterState] = React.useState("");
  const [createdToFilter, setCreatedToFilterState] = React.useState("");
  const [sortValue, setSortValueState] = React.useState<SystemEventSortValue>("created_desc");
  const requestSeqRef = React.useRef(0);

  React.useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQuery(query.trim()), 250);
    return () => window.clearTimeout(timer);
  }, [query]);

  const loadSystemEvents = React.useCallback(async (nextPage = 1, nextPageSize = pageSize) => {
    const requestSeq = requestSeqRef.current + 1;
    requestSeqRef.current = requestSeq;
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      const data = await listAdminSystemEvents(token, {
        page: nextPage,
        pageSize: nextPageSize,
        query: debouncedQuery,
        level: levelFilter,
        source: sourceFilter,
        event: eventFilter,
        createdFrom: toRFC3339DateRangeBound(createdFromFilter, "start"),
        createdTo: toRFC3339DateRangeBound(createdToFilter, "end"),
        sort: sortValue,
      });
      if (requestSeq !== requestSeqRef.current) {
        return;
      }
      setEvents(data.results);
      setTotal(data.total);
      setPage(nextPage);
      setPageSize(nextPageSize);
    } catch (error) {
      toast.error(t("toast.systemLoadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      if (requestSeq === requestSeqRef.current) {
        setLoading(false);
      }
    }
  }, [createdFromFilter, createdToFilter, debouncedQuery, eventFilter, levelFilter, pageSize, sortValue, sourceFilter, t]);

  React.useEffect(() => {
    void loadSystemEvents(1);
  }, [loadSystemEvents]);

  const setQuery = React.useCallback((value: string) => {
    setQueryState(value);
    setPage(1);
  }, []);
  const setLevelFilter = React.useCallback((value: string) => {
    setLevelFilterState(value);
    setPage(1);
  }, []);
  const setSourceFilter = React.useCallback((value: string) => {
    setSourceFilterState(value);
    setPage(1);
  }, []);
  const setEventFilter = React.useCallback((value: string) => {
    setEventFilterState(value);
    setPage(1);
  }, []);
  const setCreatedFromFilter = React.useCallback((value: string) => {
    setCreatedFromFilterState(value);
    setPage(1);
  }, []);
  const setCreatedToFilter = React.useCallback((value: string) => {
    setCreatedToFilterState(value);
    setPage(1);
  }, []);
  const setSortValue = React.useCallback((value: SystemEventSortValue) => {
    setSortValueState(value);
    setPage(1);
  }, []);

  const sourceOptions = React.useMemo(() => {
    const values = new Set(events.map((item) => item.source.trim()).filter(Boolean));
    return [{ label: t("filters.allSources"), value: "" }, ...[...values].sort().map((value) => ({ label: value, value }))];
  }, [events, t]);

  const eventOptions = React.useMemo(() => {
    const values = new Set(events.map((item) => item.event.trim()).filter(Boolean));
    return [{ label: t("filters.allEvents"), value: "" }, ...[...values].sort().map((value) => ({ label: value, value }))];
  }, [events, t]);

  return {
    events,
    total,
    page,
    pageSize,
    pageCount: Math.max(1, Math.ceil(total / pageSize)),
    loading,
    query,
    setQuery,
    levelFilter,
    setLevelFilter,
    sourceFilter,
    setSourceFilter,
    eventFilter,
    setEventFilter,
    createdFromFilter,
    setCreatedFromFilter,
    createdToFilter,
    setCreatedToFilter,
    sortValue,
    setSortValue,
    sourceOptions,
    eventOptions,
    loadSystemEvents,
  };
}

export function useAdminUsageLogs(): UseAdminUsageLogsState {
  const t = useTranslations("adminLogs");
  const [logs, setLogs] = React.useState<AdminUsageLogDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(ADMIN_LOGS_PAGE_SIZE);
  const [loading, setLoading] = React.useState(true);
  const [query, setQueryState] = React.useState("");
  const [debouncedQuery, setDebouncedQuery] = React.useState("");
  const [platformModelFilter, setPlatformModelFilterState] = React.useState("");
  const [billingModeFilter, setBillingModeFilterState] = React.useState("");
  const [platformModelOptions, setPlatformModelOptions] = React.useState<ModelSelectOption[]>([]);
  const [createdFromFilter, setCreatedFromFilterState] = React.useState("");
  const [createdToFilter, setCreatedToFilterState] = React.useState("");
  const [sortValue, setSortValueState] = React.useState<UsageLogSortValue>("created_desc");
  const requestSeqRef = React.useRef(0);

  React.useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQuery(query.trim()), 250);
    return () => window.clearTimeout(timer);
  }, [query]);

  const loadUsageLogs = React.useCallback(async (nextPage = 1, nextPageSize = pageSize) => {
    const requestSeq = requestSeqRef.current + 1;
    requestSeqRef.current = requestSeq;
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      const data = await listAdminUsageLogs(token, {
        page: nextPage,
        pageSize: nextPageSize,
        query: /^\d+$/.test(debouncedQuery) ? undefined : debouncedQuery,
        userID: parsePositiveInt(debouncedQuery),
        platformModelName: platformModelFilter,
        billingMode: billingModeFilter,
        createdFrom: toRFC3339DateRangeBound(createdFromFilter, "start"),
        createdTo: toRFC3339DateRangeBound(createdToFilter, "end"),
        sort: sortValue,
      });
      if (requestSeq !== requestSeqRef.current) return;
      setLogs(data.results);
      setTotal(data.total);
      setPage(nextPage);
      setPageSize(nextPageSize);
    } catch (error) {
      toast.error(t("toast.usageLoadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      if (requestSeq === requestSeqRef.current) {
        setLoading(false);
      }
    }
  }, [billingModeFilter, createdFromFilter, createdToFilter, debouncedQuery, pageSize, platformModelFilter, sortValue, t]);

  React.useEffect(() => {
    void loadUsageLogs(1);
  }, [loadUsageLogs]);

  React.useEffect(() => {
    let cancelled = false;

    async function loadPlatformModels() {
      try {
        const token = await resolveAccessToken();
        if (!token) {
          return;
        }
        const models = await listAllAdminPages((options) => listAdminLLMModels(token, { ...options, onlyActive: false }));
        if (cancelled) {
          return;
        }
        const options = models
          .map((item) => ({
            label: item.platformModelName.trim(),
            value: item.platformModelName.trim(),
            iconUrl: resolveModelOptionIconUrl({
              platformModelName: item.platformModelName,
              vendor: item.vendor ?? "",
              icon: item.icon ?? "",
            }),
          }))
          .filter((item) => item.value);
        const dedupedOptions = [...new Map(options.map((item) => [item.value, item])).values()]
          .sort((a, b) => a.label.localeCompare(b.label));
        setPlatformModelOptions(dedupedOptions);
      } catch (error) {
        if (!cancelled) {
          toast.error(t("toast.modelFilterLoadFailed"), { description: resolveAdminErrorMessage(error) });
        }
      }
    }

    void loadPlatformModels();
    return () => {
      cancelled = true;
    };
  }, [t]);

  const setQuery = React.useCallback((value: string) => {
    setQueryState(value);
    setPage(1);
  }, []);
  const setPlatformModelFilter = React.useCallback((value: string) => {
    setPlatformModelFilterState(value);
    setPage(1);
  }, []);
  const setBillingModeFilter = React.useCallback((value: string) => {
    setBillingModeFilterState(value);
    setPage(1);
  }, []);
  const setCreatedFromFilter = React.useCallback((value: string) => {
    setCreatedFromFilterState(value);
    setPage(1);
  }, []);
  const setCreatedToFilter = React.useCallback((value: string) => {
    setCreatedToFilterState(value);
    setPage(1);
  }, []);
  const setSortValue = React.useCallback((value: UsageLogSortValue) => {
    setSortValueState(value);
    setPage(1);
  }, []);

  return {
    logs,
    total,
    page,
    pageSize,
    pageCount: Math.max(1, Math.ceil(total / pageSize)),
    loading,
    query,
    setQuery,
    platformModelFilter,
    setPlatformModelFilter,
    billingModeFilter,
    setBillingModeFilter,
    createdFromFilter,
    setCreatedFromFilter,
    createdToFilter,
    setCreatedToFilter,
    sortValue,
    setSortValue,
    platformModelOptions,
    loadUsageLogs,
  };
}

export function useAdminPaymentOrders(): UseAdminPaymentOrdersState {
  const t = useTranslations("adminLogs");
  const [orders, setOrders] = React.useState<AdminPaymentOrderDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(ADMIN_LOGS_PAGE_SIZE);
  const [loading, setLoading] = React.useState(true);
  const [query, setQueryState] = React.useState("");
  const [debouncedQuery, setDebouncedQuery] = React.useState("");
  const [orderTypeFilter, setOrderTypeFilterState] = React.useState("");
  const [providerFilter, setProviderFilterState] = React.useState("");
  const [statusFilter, setStatusFilterState] = React.useState("");
  const [createdFromFilter, setCreatedFromFilterState] = React.useState("");
  const [createdToFilter, setCreatedToFilterState] = React.useState("");
  const [sortValue, setSortValueState] = React.useState<PaymentOrderSortValue>("created_desc");
  const requestSeqRef = React.useRef(0);

  React.useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQuery(query.trim()), 250);
    return () => window.clearTimeout(timer);
  }, [query]);

  const loadPaymentOrders = React.useCallback(async (nextPage = 1, nextPageSize = pageSize) => {
    const requestSeq = requestSeqRef.current + 1;
    requestSeqRef.current = requestSeq;
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      const data = await listAdminPaymentOrders(token, {
        page: nextPage,
        pageSize: nextPageSize,
        query: /^\d+$/.test(debouncedQuery) ? undefined : debouncedQuery,
        userID: parsePositiveInt(debouncedQuery),
        orderType: orderTypeFilter,
        provider: providerFilter,
        status: statusFilter,
        createdFrom: toRFC3339DateRangeBound(createdFromFilter, "start"),
        createdTo: toRFC3339DateRangeBound(createdToFilter, "end"),
        sort: sortValue,
      });
      if (requestSeq !== requestSeqRef.current) return;
      setOrders(data.results);
      setTotal(data.total);
      setPage(nextPage);
      setPageSize(nextPageSize);
    } catch (error) {
      toast.error(t("toast.ordersLoadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      if (requestSeq === requestSeqRef.current) {
        setLoading(false);
      }
    }
  }, [createdFromFilter, createdToFilter, debouncedQuery, orderTypeFilter, pageSize, providerFilter, sortValue, statusFilter, t]);

  React.useEffect(() => {
    void loadPaymentOrders(1);
  }, [loadPaymentOrders]);

  const setQuery = React.useCallback((value: string) => {
    setQueryState(value);
    setPage(1);
  }, []);
  const setOrderTypeFilter = React.useCallback((value: string) => {
    setOrderTypeFilterState(value);
    setPage(1);
  }, []);
  const setProviderFilter = React.useCallback((value: string) => {
    setProviderFilterState(value);
    setPage(1);
  }, []);
  const setStatusFilter = React.useCallback((value: string) => {
    setStatusFilterState(value);
    setPage(1);
  }, []);
  const setCreatedFromFilter = React.useCallback((value: string) => {
    setCreatedFromFilterState(value);
    setPage(1);
  }, []);
  const setCreatedToFilter = React.useCallback((value: string) => {
    setCreatedToFilterState(value);
    setPage(1);
  }, []);
  const setSortValue = React.useCallback((value: PaymentOrderSortValue) => {
    setSortValueState(value);
    setPage(1);
  }, []);

  return {
    orders,
    total,
    page,
    pageSize,
    pageCount: Math.max(1, Math.ceil(total / pageSize)),
    loading,
    query,
    setQuery,
    orderTypeFilter,
    setOrderTypeFilter,
    providerFilter,
    setProviderFilter,
    statusFilter,
    setStatusFilter,
    createdFromFilter,
    setCreatedFromFilter,
    createdToFilter,
    setCreatedToFilter,
    sortValue,
    setSortValue,
    loadPaymentOrders,
  };
}

export function useAdminConversationEvents(): UseAdminConversationEventsState {
  const t = useTranslations("adminLogs");
  const [events, setEvents] = React.useState<AdminConversationEventDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(ADMIN_LOGS_PAGE_SIZE);
  const [loading, setLoading] = React.useState(true);
  const [query, setQueryState] = React.useState("");
  const [debouncedQuery, setDebouncedQuery] = React.useState("");
  const [eventScopeFilter, setEventScopeFilterState] = React.useState("");
  const [eventTypeFilter, setEventTypeFilterState] = React.useState("");
  const [statusFilter, setStatusFilterState] = React.useState("");
  const [createdFromFilter, setCreatedFromFilterState] = React.useState("");
  const [createdToFilter, setCreatedToFilterState] = React.useState("");
  const [sortValue, setSortValueState] = React.useState<ConversationEventSortValue>("created_desc");
  const requestSeqRef = React.useRef(0);

  React.useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQuery(query.trim()), 250);
    return () => window.clearTimeout(timer);
  }, [query]);

  const loadConversationEvents = React.useCallback(async (nextPage = 1, nextPageSize = pageSize) => {
    const requestSeq = requestSeqRef.current + 1;
    requestSeqRef.current = requestSeq;
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      const data = await listAdminConversationEvents(token, {
        page: nextPage,
        pageSize: nextPageSize,
        query: /^\d+$/.test(debouncedQuery) ? undefined : debouncedQuery,
        userID: parsePositiveInt(debouncedQuery),
        eventScope: eventScopeFilter,
        eventType: eventTypeFilter,
        status: statusFilter,
        createdFrom: toRFC3339DateRangeBound(createdFromFilter, "start"),
        createdTo: toRFC3339DateRangeBound(createdToFilter, "end"),
        sort: sortValue,
      });
      if (requestSeq !== requestSeqRef.current) return;
      setEvents(data.results);
      setTotal(data.total);
      setPage(nextPage);
      setPageSize(nextPageSize);
    } catch (error) {
      toast.error(t("toast.conversationEventsLoadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      if (requestSeq === requestSeqRef.current) {
        setLoading(false);
      }
    }
  }, [createdFromFilter, createdToFilter, debouncedQuery, eventScopeFilter, eventTypeFilter, pageSize, sortValue, statusFilter, t]);

  React.useEffect(() => {
    void loadConversationEvents(1);
  }, [loadConversationEvents]);

  const eventTypeOptions = React.useMemo(() => {
    const values = [...new Set(events.map((item) => item.eventType).filter(Boolean))].sort((a, b) => a.localeCompare(b));
    return values.map((value) => ({ label: value, value }));
  }, [events]);

  const setQuery = React.useCallback((value: string) => {
    setQueryState(value);
    setPage(1);
  }, []);
  const setEventScopeFilter = React.useCallback((value: string) => {
    setEventScopeFilterState(value);
    setPage(1);
  }, []);
  const setEventTypeFilter = React.useCallback((value: string) => {
    setEventTypeFilterState(value);
    setPage(1);
  }, []);
  const setStatusFilter = React.useCallback((value: string) => {
    setStatusFilterState(value);
    setPage(1);
  }, []);
  const setCreatedFromFilter = React.useCallback((value: string) => {
    setCreatedFromFilterState(value);
    setPage(1);
  }, []);
  const setCreatedToFilter = React.useCallback((value: string) => {
    setCreatedToFilterState(value);
    setPage(1);
  }, []);
  const setSortValue = React.useCallback((value: ConversationEventSortValue) => {
    setSortValueState(value);
    setPage(1);
  }, []);

  return {
    events,
    total,
    page,
    pageSize,
    pageCount: Math.max(1, Math.ceil(total / pageSize)),
    loading,
    query,
    setQuery,
    eventScopeFilter,
    setEventScopeFilter,
    eventTypeFilter,
    setEventTypeFilter,
    statusFilter,
    setStatusFilter,
    createdFromFilter,
    setCreatedFromFilter,
    createdToFilter,
    setCreatedToFilter,
    sortValue,
    setSortValue,
    eventTypeOptions,
    loadConversationEvents,
  };
}
