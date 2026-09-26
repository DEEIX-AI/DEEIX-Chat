import type { ActiveSessionDTO, UserDTO } from "@/shared/api/auth.types";

type Translate = (key: string, values?: Record<string, string | number>) => string;

export function formatDateTime(value: string | null | undefined, locale = "en-US") {
  if (!value) {
    return "-";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }

  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "long",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  }).format(date);
}

export function resolveSessionTitle(session: ActiveSessionDTO, t?: Translate) {
  const browserName = session.browserName.trim();
  const osName = session.osName.trim();
  if (browserName && osName) {
    return `${browserName} (${osName})`;
  }
  return session.deviceLabel.trim() || session.deviceName.trim() || t?.("session.unknownDevice") || "Unknown device";
}

export function resolveSessionLocation(session: ActiveSessionDTO, t?: Translate) {
  const parts = [session.cityName.trim(), session.regionName.trim(), session.countryCode.trim()].filter(Boolean);
  return parts.join(", ") || t?.("session.unknownLocation") || "Unknown location";
}

export function resolveSessionIP(session: ActiveSessionDTO, t?: Translate) {
  return session.clientIP.trim() || t?.("session.unknownIP") || "Unknown IP";
}

export function shouldUseEmailBootstrap(viewer: UserDTO | null): boolean {
  if (!viewer) {
    return false;
  }
  if (!viewer.email.trim()) {
    return true;
  }
  return viewer.emailSource === "provider_unverified" && !viewer.emailVerifiedAt && !viewer.emailBootstrapUsedAt;
}

export function resolveEmailTitle(viewer: UserDTO | null, t?: Translate): string {
  if (!viewer?.email) {
    return t?.("email.label") || "Email";
  }
  return t?.("email.withAddress", { email: viewer.email }) || `Email (${viewer.email})`;
}

export function resolveEmailValue(viewer: UserDTO | null, emailVerificationEnabled: boolean, t?: Translate): string | undefined {
  if (!viewer?.email || !emailVerificationEnabled) {
    return undefined;
  }
  return viewer.emailVerifiedAt ? t?.("email.verified") || "Verified" : t?.("email.unverified") || "Unverified";
}
