"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { resolveLocalizedErrorMessage, toErrorMessagePath } from "@/i18n/resolve-error-message";
import { ApiError } from "@/shared/api/http-client";

function toMessageKey(errorCode: string): string {
  return toErrorMessagePath(errorCode).join(".");
}

export function useLocalizedErrorMessage() {
  const errors = useTranslations("errors");
  const common = useTranslations("common.errors");

  return React.useCallback(
    (error: unknown, fallback?: string) => {
      if (error instanceof ApiError && error.errorCode) {
        const key = toMessageKey(error.errorCode);
        try {
          const translated = errors(key);
          if (translated && translated !== key && translated !== `errors.${key}`) {
            return translated;
          }
        } catch {
          // Fall through to backend fallback.
        }
      }

      return resolveLocalizedErrorMessage(error, fallback || common("unknown"));
    },
    [common, errors],
  );
}
