import type { LoginOptionsData, LoginPageSettings } from "@/shared/api/auth.types";
import { ApiError } from "@/shared/api/http-client";
import { DEFAULT_AUTH_NEXT_PATH } from "@/shared/auth/local-path";

export type LoginMode = "login" | "register";
export type ProviderAuthIntent = "login" | "register";

export const DEFAULT_LOGIN_SETTINGS: LoginPageSettings = {
  title: "Sign in to DEEIX Chat",
  defaultNextPath: DEFAULT_AUTH_NEXT_PATH,
};

export const DEFAULT_LOGIN_OPTIONS: LoginOptionsData = {
  usernameEnabled: true,
  emailEnabled: true,
  emailRegistrationEnabled: true,
  emailVerificationEnabled: false,
  turnstileRegistrationEnabled: false,
  turnstileSiteKey: "",
  providers: [],
};

export const TWO_FACTOR_CHALLENGE_STORAGE_KEY = "deeix-chat:2fa:challenge";
export const TWO_FACTOR_METHODS_STORAGE_KEY = "deeix-chat:2fa:methods";

export function normalizeTwoFactorInput(value: string): string {
  return value.replace(/[^a-zA-Z0-9-]/g, "").slice(0, 32);
}

export function normalizeRegisterCode(value: string): string {
  return value.replace(/\D/g, "").slice(0, 6);
}

export function providerPKCEStorageKey(slug: string): string {
  return `deeix-chat:oauth:${slug}:pkce_verifier`;
}

export function isTwoFactorChallengeExpired(error: unknown): boolean {
  return error instanceof ApiError && error.status === 401 && error.message === "two factor challenge expired";
}

function base64URL(bytes: Uint8Array): string {
  let binary = "";
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte);
  });
  return btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replaceAll("=", "");
}

export async function createProviderPKCE() {
  const verifierBytes = new Uint8Array(48);
  window.crypto.getRandomValues(verifierBytes);
  const verifier = base64URL(verifierBytes);
  const digest = await window.crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier));
  return {
    verifier,
    challenge: base64URL(new Uint8Array(digest)),
  };
}
