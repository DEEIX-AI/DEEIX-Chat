import type { LoginOptionsData, LoginPageSettings, SecurityVerificationMethod } from "@/shared/api/auth-types";
import { ApiError } from "@/shared/api/http-client";
import { DEFAULT_AUTH_NEXT_PATH } from "@/shared/auth/local-path";
import { isRecord } from "@/shared/lib/type-guards";

export type LoginMode = "login" | "register" | "reset-password";
export type ProviderAuthIntent = "login" | "register";

export const DEFAULT_LOGIN_SETTINGS: LoginPageSettings = {
  defaultNextPath: DEFAULT_AUTH_NEXT_PATH,
};

export const DEFAULT_LOGIN_OPTIONS: LoginOptionsData = {
  usernameEnabled: true,
  emailEnabled: true,
  emailRegistrationEnabled: true,
  emailVerificationEnabled: false,
  passwordResetEnabled: false,
  turnstileRegistrationEnabled: false,
  turnstileSiteKey: "",
  providerAuthBridge: {
    callbackBaseURL: "",
    enabled: false,
    protocolVersion: 1,
  },
  providers: [],
};

const TWO_FACTOR_CHALLENGE_STORAGE_KEY = "deeix-chat:2fa:challenge";
// Written by older builds next to a plain-token challenge entry.
const LEGACY_TWO_FACTOR_METHODS_STORAGE_KEY = "deeix-chat:2fa:methods";
// Matches the backend two-factor challenge TTL (twoFactorChallengeTTL); an older
// token would only be rejected on submit, so it is dropped on read instead.
const TWO_FACTOR_CHALLENGE_TTL_MS = 5 * 60 * 1000;

export type TwoFactorChallenge = {
  token: string;
  methods: SecurityVerificationMethod[];
};

type StoredTwoFactorChallenge = TwoFactorChallenge & {
  expiresAt: number;
};

function normalizeVerificationMethods(value: readonly unknown[] | undefined): SecurityVerificationMethod[] {
  const methods = (value ?? []).filter(
    (item): item is SecurityVerificationMethod => item === "two_factor" || item === "email",
  );
  return methods.length > 0 ? methods : ["two_factor"];
}

function isStoredTwoFactorChallenge(value: unknown): value is StoredTwoFactorChallenge {
  return isRecord(value) && typeof value.token === "string" && Array.isArray(value.methods) && typeof value.expiresAt === "number";
}

// Hands a provider-login 2FA challenge from the callback page to the login page.
// Returns false when the challenge could not be stored (e.g. sessionStorage is unavailable).
export function writeTwoFactorChallenge(token: string, methods: readonly SecurityVerificationMethod[] | undefined): boolean {
  if (!token) {
    return false;
  }
  const stored: StoredTwoFactorChallenge = {
    token,
    methods: normalizeVerificationMethods(methods),
    expiresAt: Date.now() + TWO_FACTOR_CHALLENGE_TTL_MS,
  };
  try {
    window.sessionStorage.setItem(TWO_FACTOR_CHALLENGE_STORAGE_KEY, JSON.stringify(stored));
    return true;
  } catch {
    return false;
  }
}

// Reads the pending challenge without consuming it, so a refresh during 2FA keeps the prompt;
// the login page clears it on success, cancel or expiry, and the TTL bounds it otherwise.
// Malformed, expired and legacy entries (a plain token string plus a separate methods key) are
// removed and count as absent.
export function readTwoFactorChallenge(): TwoFactorChallenge | null {
  try {
    window.sessionStorage.removeItem(LEGACY_TWO_FACTOR_METHODS_STORAGE_KEY);
    const raw = window.sessionStorage.getItem(TWO_FACTOR_CHALLENGE_STORAGE_KEY);
    if (!raw) {
      return null;
    }
    let parsed: unknown = null;
    try {
      parsed = JSON.parse(raw);
    } catch {
      // Legacy plain-token entry; it carries no expiry, so it is not trusted.
    }
    if (!isStoredTwoFactorChallenge(parsed) || !parsed.token || parsed.expiresAt <= Date.now()) {
      window.sessionStorage.removeItem(TWO_FACTOR_CHALLENGE_STORAGE_KEY);
      return null;
    }
    return { token: parsed.token, methods: normalizeVerificationMethods(parsed.methods) };
  } catch {
    return null;
  }
}

export function clearTwoFactorChallenge(): void {
  try {
    window.sessionStorage.removeItem(TWO_FACTOR_CHALLENGE_STORAGE_KEY);
    window.sessionStorage.removeItem(LEGACY_TWO_FACTOR_METHODS_STORAGE_KEY);
  } catch {
    // sessionStorage may be unavailable; nothing was stored then.
  }
}

export function normalizeTwoFactorInput(value: string): string {
  return value.replace(/[^a-zA-Z0-9-]/g, "").slice(0, 32);
}

export function normalizeRegisterCode(value: string): string {
  return value.replace(/\D/g, "").slice(0, 6);
}

export function isTwoFactorChallengeExpired(error: unknown): boolean {
  return error instanceof ApiError && error.status === 401 && error.message === "two factor challenge expired";
}


