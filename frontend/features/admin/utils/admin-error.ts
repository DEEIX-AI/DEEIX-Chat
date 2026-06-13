import { resolveLocalizedErrorMessage } from "@/i18n/resolve-error-message";

export function resolveAdminErrorMessage(error: unknown, fallback?: string): string {
  return resolveLocalizedErrorMessage(error, fallback);
}
