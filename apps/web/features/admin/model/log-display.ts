// 管理端日志通用展示格式化：时间、用户名、JSON、计数与金额。

export function formatDateTime(value: string | null | undefined, locale: string): string {
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

export function resolveUserDisplayName(label: string, username: string, fallbackID: number): string {
  const name = label.trim() || username.trim();
  return name || String(fallbackID);
}

export function formatJSON(raw: string | null | undefined): string {
  const value = raw?.trim();
  if (!value) {
    return "{}";
  }
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

export function parseJSONRecord(raw: string | null | undefined): Record<string, unknown> | null {
  const value = raw?.trim();
  if (!value) {
    return null;
  }
  try {
    const parsed = JSON.parse(value) as unknown;
    return parsed && typeof parsed === "object" && !Array.isArray(parsed)
      ? (parsed as Record<string, unknown>)
      : null;
  } catch {
    return null;
  }
}

export function formatCount(value: number | null | undefined, locale: string): string {
  return new Intl.NumberFormat(locale).format(value ?? 0);
}

export function formatMoneyCents(value: number | null | undefined, currency: string): string {
  const amount = (value ?? 0) / 100;
  const normalizedCurrency = currency.trim().toUpperCase();
  if (!normalizedCurrency) {
    return amount.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  }
  try {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: normalizedCurrency,
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(amount);
  } catch {
    return `${amount.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ${normalizedCurrency}`;
  }
}
