"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Link2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { SpinnerLabel } from "@/components/ui/spinner";
import {
  providerAuthBridgeStorageKey,
  providerPKCEStorageKey,
  TWO_FACTOR_CHALLENGE_STORAGE_KEY,
  TWO_FACTOR_METHODS_STORAGE_KEY,
} from "@/features/auth/model/login-page";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { completeProviderBind, completeProviderLogin, exchangeProviderAuthBridgeGrant } from "@/shared/api/auth";
import { ApiError } from "@/shared/api/http-client";
import { DEFAULT_AUTH_NEXT_PATH, normalizeAuthNextPath } from "@/shared/auth/local-path";
import { AppLogo } from "@/shared/components/app-logo";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { writeSessionSnapshot } from "@/shared/auth/session";

const PROVIDER_EMAIL_CONFLICT_ERROR_CODE = "auth.provider_email_conflict";
const PROVIDER_EMAIL_CONFLICT_ACTION_SIGN_IN_THEN_BIND = "sign_in_then_bind";
const ACCOUNT_SETTINGS_PATH = "/setting/account";

type ProviderEmailConflictDetails = {
  action?: string;
  providerSlug?: string;
  email?: string;
};

type EmailConflictState = {
  providerSlug?: string;
  email?: string;
};

export function AuthCallbackPage() {
  const t = useTranslations("login.oauthCallback");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const router = useRouter();
  const [error, setError] = React.useState("");
  const [emailConflict, setEmailConflict] = React.useState<EmailConflictState | null>(null);
  const handledRef = React.useRef(false);

  const redirectToLogin = React.useCallback(() => {
    router.replace("/login");
  }, [router]);

  const redirectToLoginWithAccountSettingsNext = React.useCallback(() => {
    router.replace(`/login?next=${encodeURIComponent(ACCOUNT_SETTINGS_PATH)}`);
  }, [router]);

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
    const grant = params.get("grant") ?? "";
    if (provider && grant) {
      const stored = readProviderAuthBridgeRequest(provider);
      window.sessionStorage.removeItem(providerAuthBridgeStorageKey(provider));
      if (!stored || !constantTimeStringEqual(params.get("state") ?? "", stored.state)) {
        setError(t("expiredSession"));
        return;
      }
      void exchangeProviderAuthBridgeGrant(provider, {
        clientID: "deeix-web",
        grant,
        codeVerifier: stored.verifier,
      })
        .then((result) => {
          if (result.twoFactorRequired) {
            window.sessionStorage.setItem(TWO_FACTOR_CHALLENGE_STORAGE_KEY, result.twoFactorChallengeToken ?? "");
            window.sessionStorage.setItem(TWO_FACTOR_METHODS_STORAGE_KEY, JSON.stringify(result.verificationMethods ?? ["two_factor"]));
            router.replace(`/login?next=${encodeURIComponent(stored.next)}`);
            return;
          }
          writeSessionSnapshot({ accessToken: result.accessToken, sessionID: result.sessionID });
          router.replace(stored.next);
        })
        .catch((caught) => {
          if (isProviderEmailConflictError(caught)) {
            const details = caught.details as ProviderEmailConflictDetails | undefined;
            setEmailConflict({
              providerSlug: details?.providerSlug?.trim() || undefined,
              email: details?.email?.trim() || undefined,
            });
            return;
          }
          setError(resolveErrorMessage(caught, t("loginFailed")));
        });
      return;
    }
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
        if (isProviderEmailConflictError(caught)) {
          const details = caught.details as ProviderEmailConflictDetails | undefined;
          setEmailConflict({
            providerSlug: details?.providerSlug?.trim() || undefined,
            email: details?.email?.trim() || undefined,
          });
          return;
        }
        setError(resolveErrorMessage(caught, t("loginFailed")));
      });
  }, [resolveErrorMessage, router, t]);

  const conflictProviderLabel = React.useMemo(() => {
    if (!emailConflict?.providerSlug) {
      return t("emailConflict.providerUnknown");
    }
    return emailConflict.providerSlug;
  }, [emailConflict?.providerSlug, t]);

  return (
    <main className="flex min-h-screen items-center justify-center px-4 py-8 text-foreground">
      <div className="w-full max-w-[360px]">
        <div className="flex flex-col items-center text-center">
          <AppLogo width={32} height={32} priority className="h-9 w-auto" />
        </div>

        {error ? (
          <div className="mt-7 space-y-5 text-center">
            <div className="space-y-2">
              <h1 className="text-xl font-semibold leading-7">{t("errorTitle")}</h1>
              <p className="break-words text-sm leading-6 text-muted-foreground">{error}</p>
            </div>
            <Button type="button" className="h-9 w-full rounded-md bg-foreground text-sm font-semibold text-background shadow-none hover:bg-foreground/90" onClick={redirectToLogin}>
              {t("backToLogin")}
            </Button>
          </div>
        ) : emailConflict ? (
          <div className="mt-7 space-y-5">
            <div className="space-y-2 text-center">
              <div className="space-y-2">
                <h1 className="text-xl font-semibold leading-7">{t("emailConflict.title")}</h1>
                <p className="text-sm leading-6 text-muted-foreground">{t("emailConflict.description")}</p>
              </div>
            </div>

            <div className="space-y-2 rounded-md bg-muted/60 px-3 py-3 text-sm">
              <div className="flex min-w-0 items-center justify-between gap-3">
                <span className="shrink-0 text-muted-foreground">{t("emailConflict.providerLabel")}</span>
                <span className="min-w-0 truncate text-right font-medium text-foreground">{conflictProviderLabel}</span>
              </div>
              {emailConflict.email ? (
                <div className="flex min-w-0 items-center justify-between gap-3">
                  <span className="shrink-0 text-muted-foreground">{t("emailConflict.emailLabel")}</span>
                  <span className="min-w-0 truncate text-right font-medium text-foreground">{emailConflict.email}</span>
                </div>
              ) : null}
            </div>

            <div className="space-y-3">
              <Button type="button" className="h-9 w-full rounded-md bg-foreground text-sm font-semibold text-background shadow-none hover:bg-foreground/90" onClick={redirectToLoginWithAccountSettingsNext}>
                <Link2 className="size-4" aria-hidden="true" />
                {t("emailConflict.signInExistingAccount")}
              </Button>
            </div>

            <p className="text-center text-xs leading-5 text-muted-foreground">
              {t("emailConflict.notePrefix")}
              <button
                type="button"
                className="font-medium text-foreground underline-offset-4 hover:underline focus-visible:underline focus-visible:outline-none"
                onClick={redirectToLogin}
              >
                {t("emailConflict.noteLink")}
              </button>
              {t("emailConflict.noteSuffix")}
            </p>
          </div>
        ) : (
          <div className="mt-7 flex flex-col items-center gap-3 text-center text-sm text-muted-foreground">
            <SpinnerLabel>{t("loading")}</SpinnerLabel>
            <p>{t("loadingDescription")}</p>
          </div>
        )}
      </div>
    </main>
  );
}

function readProviderAuthBridgeRequest(slug: string): { verifier: string; state: string; intent: "login" | "register"; next: string } | null {
  try {
    const raw = window.sessionStorage.getItem(providerAuthBridgeStorageKey(slug));
    if (!raw) return null;
    const parsed = JSON.parse(raw) as { verifier?: string; state?: string; intent?: string; next?: string };
    if (!parsed.verifier || !parsed.state) return null;
    return {
      verifier: parsed.verifier,
      state: parsed.state,
      intent: parsed.intent === "register" ? "register" : "login",
      next: normalizeAuthNextPath(parsed.next),
    };
  } catch {
    return null;
  }
}

function constantTimeStringEqual(left: string, right: string): boolean {
  if (left.length !== right.length) return false;
  let difference = 0;
  for (let index = 0; index < left.length; index += 1) {
    difference |= left.charCodeAt(index) ^ right.charCodeAt(index);
  }
  return difference === 0;
}

function isProviderEmailConflictError(error: unknown): boolean {
  if (!(error instanceof ApiError) || error.errorCode !== PROVIDER_EMAIL_CONFLICT_ERROR_CODE) {
    return false;
  }
  const details = error.details as ProviderEmailConflictDetails | undefined;
  return details?.action === PROVIDER_EMAIL_CONFLICT_ACTION_SIGN_IN_THEN_BIND;
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
