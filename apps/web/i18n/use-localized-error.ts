"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { errorReferenceFor, resolveLocalizedErrorMessage, toErrorMessagePath } from "@/i18n/resolve-error-message";
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
        if (error.errorCode === "request.invalid_body") {
          return resolveLocalizedErrorMessage(error, fallback || common("unknown"));
        }

        const key = toMessageKey(error.errorCode);
        if (errors.has(key)) {
          const translated = errors(key);
          if (translated && translated !== key && translated !== `errors.${key}`) {
            const requestId = errorReferenceFor(error);
            return requestId ? `${translated} ${common("errorReference", { requestId })}` : translated;
          }
        }
      }

      return resolveLocalizedErrorMessage(error, fallback || common("unknown"));
    },
    [common, errors],
  );
}
