"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { SpinnerLabel } from "@/components/ui/spinner";
import {
  providerPKCEStorageKey,
  TWO_FACTOR_CHALLENGE_STORAGE_KEY,
  TWO_FACTOR_METHODS_STORAGE_KEY,
} from "@/features/auth/model/login-page";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { completeProviderBind, completeProviderLogin } from "@/shared/api/auth";
import { DEFAULT_AUTH_NEXT_PATH, normalizeAuthNextPath } from "@/shared/auth/local-path";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { writeSessionSnapshot } from "@/shared/auth/session";

export function AuthCallbackPage() {
  const t = useTranslations("login.oauthCallback");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const router = useRouter();
  const [error, setError] = React.useState("");
  const handledRef = React.useRef(false);

  React.useEffect(() => {
    if (handledRef.current) {
      return;
    }
    handledRef.current = true;

    const params = new URLSearchParams(window.location.search);
    const errorMessage = params.get("error");
    if (errorMessage) {
      setError(t("providerError", { error: errorMessage }));
      return;
    }

    const provider = params.get("provider") ?? "";
    const code = params.get("code") ?? "";
    const state = params.get("state") ?? "";
    const parsedState = parseProviderState(state);
    const intent = parsedState.intent;
    const nextPath = parsedState.next;
    if (!provider || !code || !state) {
      setError(t("missingParams"));
      return;
    }
    const codeVerifier = window.sessionStorage.getItem(providerPKCEStorageKey(provider)) ?? "";
    window.sessionStorage.removeItem(providerPKCEStorageKey(provider));
    if (!codeVerifier) {
      setError(t("expiredSession"));
      return;
    }

    const redirectURI = `${window.location.origin}${window.location.pathname}?provider=${encodeURIComponent(provider)}`;
    if (intent === "bind") {
      void resolveAccessToken()
        .then((accessToken) => {
          if (!accessToken) {
            throw new Error(t("bindSessionExpired"));
          }
          return completeProviderBind(accessToken, provider, code, state, redirectURI, codeVerifier);
        })
        .then(() => {
          router.replace(nextPath);
        })
        .catch((caught) => {
          setError(resolveErrorMessage(caught, t("bindFailed")));
        });
      return;
    }

    void completeProviderLogin(provider, code, state, redirectURI, codeVerifier, intent)
      .then((result) => {
        if (result.twoFactorRequired) {
          window.sessionStorage.setItem(TWO_FACTOR_CHALLENGE_STORAGE_KEY, result.twoFactorChallengeToken ?? "");
          window.sessionStorage.setItem(TWO_FACTOR_METHODS_STORAGE_KEY, JSON.stringify(result.verificationMethods ?? ["two_factor"]));
          router.replace(`/login?next=${encodeURIComponent(nextPath)}`);
          return;
        }
        writeSessionSnapshot({
          accessToken: result.accessToken,
          sessionID: result.sessionID,
        });
        router.replace(nextPath);
      })
      .catch((caught) => {
        setError(resolveErrorMessage(caught, t("loginFailed")));
      });
  }, [resolveErrorMessage, router, t]);

  return (
    <main className="flex min-h-screen items-center justify-center px-4 text-sm text-muted-foreground">
      {error ? <span>{error}</span> : <SpinnerLabel>{t("loading")}</SpinnerLabel>}
    </main>
  );
}

function parseProviderState(raw: string): { next: string; intent: "login" | "register" | "bind" } {
  try {
    const [encodedPayload] = raw.split(".");
    if (!encodedPayload) return { next: DEFAULT_AUTH_NEXT_PATH, intent: "login" };
    const padded = encodedPayload.replaceAll("-", "+").replaceAll("_", "/").padEnd(Math.ceil(encodedPayload.length / 4) * 4, "=");
    const parsed = JSON.parse(atob(padded)) as { next?: string; intent?: string };
    const intent = parsed.intent ?? "";
    return {
      next: normalizeAuthNextPath(parsed.next),
      intent: intent === "register" ? "register" : intent === "bind" ? "bind" : "login",
    };
  } catch {
    return { next: DEFAULT_AUTH_NEXT_PATH, intent: "login" };
  }
}
