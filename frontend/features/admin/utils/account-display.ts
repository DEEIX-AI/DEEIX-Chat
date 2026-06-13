import type { UserDTO } from "@/shared/api/auth.types";

const USER_STATUS_LABELS: Record<string, string> = {
  pending_activation: "Pending activation",
  active: "Active",
  locked: "Locked",
  suspended: "Suspended",
  deactivated: "Deactivated",
};

const AUTH_EVENT_RESULT_LABELS: Record<string, string> = {
  success: "Success",
  failure: "Failed",
  blocked: "Blocked",
};

const SUBSCRIPTION_STATUS_LABELS: Record<string, string> = {
  active: "Active",
  trialing: "Trialing",
  past_due: "Past due",
  canceled: "Canceled",
  unpaid: "Unpaid",
  incomplete: "Incomplete",
  incomplete_expired: "Incomplete expired",
  paused: "Paused",
};

const BILLING_ACCOUNT_STATUS_LABELS: Record<string, string> = {
  active: "Active",
  frozen: "Frozen",
  closed: "Closed",
  suspended: "Suspended",
};

export function resolveSubscriptionExpiryDate(value: string): Date | undefined {
  const text = value.trim();
  if (!text) {
    return undefined;
  }
  const dateOnlyMatch = /^(\d{4})-(\d{2})-(\d{2})$/.exec(text);
  if (dateOnlyMatch) {
    const [, year, month, day] = dateOnlyMatch;
    const date = new Date(Number(year), Number(month) - 1, Number(day));
    return Number.isNaN(date.getTime()) ? undefined : date;
  }
  const date = new Date(text);
  return Number.isNaN(date.getTime()) ? undefined : date;
}

export function resolveSubscriptionExpiryISO(value: string): string | undefined {
  const date = resolveSubscriptionExpiryDate(value);
  if (!date) {
    return undefined;
  }
  if (/^\d{4}-\d{2}-\d{2}$/.test(value.trim())) {
    date.setHours(23, 59, 59, 999);
  }
  return date.toISOString();
}

export function resolveSubscriptionExpiryInputValue(value: string | null | undefined): string {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export function formatDateTime(value: string | null | undefined, locale = "en-US"): string {
  if (!value) {
    return "-";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }

  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function resolveValue(value: string | null | undefined): string {
  return value?.trim() || "-";
}

export function resolveUserStatusLabel(value: string | null | undefined): string {
  const key = value?.trim() ?? "";
  return USER_STATUS_LABELS[key] ?? resolveValue(value);
}

export function resolveAuthEventResultLabel(value: string | null | undefined): string {
  const key = value?.trim() ?? "";
  return AUTH_EVENT_RESULT_LABELS[key] ?? resolveValue(value);
}

export function resolveSubscriptionStatusLabel(value: string | null | undefined): string {
  const key = value?.trim() ?? "";
  return SUBSCRIPTION_STATUS_LABELS[key] ?? resolveValue(value);
}

export function resolveBillingAccountStatusLabel(value: string | null | undefined): string {
  const key = value?.trim() ?? "";
  return BILLING_ACCOUNT_STATUS_LABELS[key] ?? resolveValue(value);
}

export function resolveUserInitial(user: UserDTO): string {
  const source = user.displayName.trim() || user.username.trim() || user.publicID.trim() || String(user.id);
  return source.charAt(0).toUpperCase();
}

export function resolveCreateUserInitial(username: string, displayName: string): string {
  return (displayName.trim() || username.trim() || "U").charAt(0).toUpperCase();
}

export function resolveDetailValue(value: string | number | null | undefined): string {
  if (value === null || value === undefined) {
    return "-";
  }
  if (typeof value === "string") {
    return value.trim() || "-";
  }
  return String(value);
}

export function formatBillingBalance(value: number | null | undefined): string {
  if (!Number.isFinite(value ?? NaN) || (value ?? 0) <= 0) {
    return "$0.000000";
  }
  return `$${(value ?? 0).toLocaleString("en-US", {
    minimumFractionDigits: 6,
    maximumFractionDigits: 6,
  })}`;
}
