import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import * as React from "react";

import { writeTwoFactorChallenge } from "@/features/auth/model/login-page";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { exchangeProviderAuthBridgeGrant, exchangeProviderBindBridgeGrant } from "@/shared/api/auth";
import { ApiError } from "@/shared/api/http-client";
import { normalizeAuthNextPath } from "@/shared/auth/local-path";
import { clearProviderBridgeRequest, readProviderBridgeRequest } from "@/shared/auth/provider-bridge";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { isRecord, readString } from "@/shared/lib/type-guards";
import { resolveOAuthClientId } from "@/shared/platform/desktop-oauth";
import { completeNativeSignIn } from "@/shared/platform/desktop-session";

const PROVIDER_EMAIL_CONFLICT_ERROR_CODE = "auth.provider_email_conflict";
const PROVIDER_EMAIL_CONFLICT_ACTION_SIGN_IN_THEN_BIND = "sign_in_then_bind";

export type EmailConflictState = {
  providerSlug?: string;
  email?: string;
};

type ProviderEmailConflictError = ApiError & {
  details: Record<string, unknown>;
};

function constantTimeStringEqual(left: string, right: string): boolean {
  if (left.length !== right.length) return false;
  let difference = 0;
  for (let index = 0; index < left.length; index += 1) {
    difference |= left.charCodeAt(index) ^ right.charCodeAt(index);
  }
  return difference === 0;
}

function isProviderEmailConflictError(error: unknown): error is ProviderEmailConflictError {
  if (!(error instanceof ApiError) || error.errorCode !== PROVIDER_EMAIL_CONFLICT_ERROR_CODE) {
    return false;
  }
  return isRecord(error.details) && error.details.action === PROVIDER_EMAIL_CONFLICT_ACTION_SIGN_IN_THEN_BIND;
}

/**
 * Completes the provider OAuth bridge on the callback page. Runs exactly once
 * per mount: the stored bridge request is consumed (read + cleared) before the
 * state check so a grant can never be exchanged twice, and the state is
 * compared in constant time. Login results either finish sign-in, hand a 2FA
 * challenge to the login page, or surface an email conflict.
 */
export function useAuthProviderCallback() {
  const t = useTranslations("login.oauthCallback");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const router = useRouter();
  const [error, setError] = React.useState("");
  const [emailConflict, setEmailConflict] = React.useState<EmailConflictState | null>(null);
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
    const grant = params.get("grant") ?? "";
    if (provider && grant) {
      const stored = readProviderBridgeRequest(provider);
      clearProviderBridgeRequest(provider);
      if (!stored || !constantTimeStringEqual(params.get("state") ?? "", stored.state)) {
        setError(t("expiredSession"));
        return;
      }
      const exchangeInput = { clientID: resolveOAuthClientId(), grant, codeVerifier: stored.verifier };
      const nextPath = normalizeAuthNextPath(stored.next);
      if (stored.intent === "bind") {
        void resolveAccessToken()
          .then((accessToken) => {
            if (!accessToken) {
              throw new Error(t("bindSessionExpired"));
            }
            return exchangeProviderBindBridgeGrant(accessToken, provider, exchangeInput);
          })
          .then(() => {
            router.replace(nextPath);
          })
          .catch((caught) => {
            setError(resolveErrorMessage(caught, t("bindFailed")));
          });
        return;
      }
      void exchangeProviderAuthBridgeGrant(provider, exchangeInput)
        .then((result) => {
          if (result.twoFactorRequired) {
            if (!writeTwoFactorChallenge(result.twoFactorChallengeToken ?? "", result.verificationMethods)) {
              // Without the stored challenge the login page could not ask for the code.
              setError(t("loginFailed"));
              return;
            }
            router.replace(`/login?next=${encodeURIComponent(nextPath)}`);
            return;
          }
          void completeNativeSignIn(result);
          router.replace(nextPath);
        })
        .catch((caught) => {
          if (isProviderEmailConflictError(caught)) {
            setEmailConflict({
              providerSlug: readString(caught.details, "providerSlug")?.trim() || undefined,
              email: readString(caught.details, "email")?.trim() || undefined,
            });
            return;
          }
          setError(resolveErrorMessage(caught, t("loginFailed")));
        });
      return;
    }
    setError(t("missingParams"));
  }, [resolveErrorMessage, router, t]);

  return { error, emailConflict };
}
