"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Link2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { SpinnerLabel } from "@/components/ui/spinner";
import { useAuthProviderCallback } from "@/features/auth/hooks/use-auth-provider-callback";
import { AppLogo } from "@/shared/components/app-logo";

const ACCOUNT_SETTINGS_PATH = "/settings/account";

export function AuthCallbackPage() {
  const t = useTranslations("login.oauthCallback");
  const router = useRouter();
  const { error, emailConflict } = useAuthProviderCallback();

  const redirectToLogin = React.useCallback(() => {
    router.replace("/login");
  }, [router]);

  const redirectToLoginWithAccountSettingsNext = React.useCallback(() => {
    router.replace(`/login?next=${encodeURIComponent(ACCOUNT_SETTINGS_PATH)}`);
  }, [router]);

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
