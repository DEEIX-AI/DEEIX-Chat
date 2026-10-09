"use client";

import { maskProviderApiKey, parseProviderImportLink, type ProviderImportLinkResult } from "@deeix/core";
import { useRouter } from "next/navigation";
import { InfoIcon, ShieldAlertIcon } from "lucide-react";
import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { SpinnerLabel } from "@/components/ui/spinner";
import { findModelProviderPreset, ModelIcon, resolveModelIconURL } from "@/entities/model";
import { type ModelProviderDraft, ModelsProviderDialog } from "@/features/settings/components/sections/models/models-provider-dialog";
import { useSettingsModelProviders } from "@/features/settings/hooks/use-settings-model-providers";
import {
  captureProviderImportFragment,
  clearPendingProviderImport,
  readPendingProviderImport,
} from "@/features/settings/utils/provider-import";
import { UserLocaleSync } from "@/i18n/user-locale-sync";
import { AuthGuard } from "@/shared/auth/auth-guard";
import { AppLogo } from "@/shared/components/app-logo";
import { usePersonalProviderAccess } from "@/shared/hooks/use-personal-provider-access";

const MODELS_SETTINGS_PATH = "/settings/models";

const PRIMARY_BUTTON_CLASS = "h-9 w-full rounded-md bg-foreground text-sm font-semibold text-background shadow-none hover:bg-foreground/90";
const CANCEL_BUTTON_CLASS = "h-9 w-full rounded-md text-sm font-normal text-muted-foreground shadow-none hover:bg-muted/60 hover:text-foreground";

/**
 * Route entry for `/import#v=1&url=…&key=…`. The key is taken out of the URL
 * before anything else runs (including the login redirect); the user then
 * confirms the address and picks models. Nothing is saved without that step.
 */
export function ProviderImportPage() {
  const [fragment, setFragment] = React.useState<string | null>(null);

  React.useLayoutEffect(() => {
    // A fresh link wins; otherwise resume the one stashed before signing in.
    setFragment(captureProviderImportFragment() || readPendingProviderImport());
    // A link pasted into a tab already on /import only changes the hash; read it the same way.
    const onHashChange = () => {
      const next = captureProviderImportFragment();
      if (next) setFragment(next);
    };
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);

  if (fragment === null) return null;

  return (
    <AuthGuard>
      {/* This page lives outside the project shell, so apply the account language here too. */}
      <UserLocaleSync />
      {/* Keyed by the link so a new link starts from a clean confirmation screen. */}
      <ProviderImportScreen key={fragment} fragment={fragment} />
    </AuthGuard>
  );
}

function ProviderImportScreen({ fragment }: { fragment: string }) {
  const t = useTranslations("settings.importPage");
  const router = useRouter();
  const access = usePersonalProviderAccess();
  const parsed = React.useMemo<ProviderImportLinkResult>(() => parseProviderImportLink(fragment), [fragment]);
  const { providers, loading, probe, create } = useSettingsModelProviders(access.enabled);
  const [dialogOpen, setDialogOpen] = React.useState(false);

  // Signed in now: the stash has served its purpose.
  React.useEffect(() => {
    clearPendingProviderImport();
  }, []);

  const draft = React.useMemo<Partial<ModelProviderDraft> | undefined>(() => {
    if (!parsed.ok) return undefined;
    return {
      name: parsed.link.name,
      baseURL: parsed.link.baseURL,
      apiKey: parsed.link.apiKey,
      protocol: parsed.link.protocol ?? undefined,
    };
  }, [parsed]);

  const leave = () => {
    clearPendingProviderImport();
    router.replace("/chat");
  };

  if (!access.loaded || (access.enabled && loading)) {
    return (
      <ImportFrame>
        <div className="flex h-24 items-center justify-center text-xs text-muted-foreground">
          <SpinnerLabel>{t("loading")}</SpinnerLabel>
        </div>
      </ImportFrame>
    );
  }

  if (!access.enabled) {
    return (
      <ImportFrame title={t("unavailable.title")} description={t("unavailable.description")}>
        <Button type="button" className={PRIMARY_BUTTON_CLASS} onClick={leave}>
          {t("backToChat")}
        </Button>
      </ImportFrame>
    );
  }

  if (!parsed.ok) {
    return (
      <ImportFrame title={t("invalid.title")} description={t(`invalid.${parsed.error}`)}>
        <Button type="button" className={PRIMARY_BUTTON_CLASS} onClick={leave}>
          {t("backToChat")}
        </Button>
      </ImportFrame>
    );
  }

  const { link } = parsed;
  const preset = findModelProviderPreset(link.baseURL);
  const atLimit = providers.length >= access.maxPerUser;
  const existing = providers.find((provider) => provider.host === link.host);

  return (
    <ImportFrame title={t("title")}>
      <div className="space-y-5">
        <div className="overflow-hidden rounded-lg border border-border/60">
          <div className="flex min-w-0 items-center gap-3 px-3.5 py-3">
            <ModelIcon iconUrl={preset ? resolveModelIconURL(preset.icon) : null} label={link.host} />
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">{link.name || preset?.name || link.host}</p>
              {/* The address is the one thing a malicious link changes; keep it in full view. */}
              <p className="truncate text-xs text-muted-foreground" title={link.baseURL}>
                {link.baseURL}
              </p>
            </div>
          </div>
          <div className="flex items-center justify-between gap-4 border-t border-border/60 px-3.5 py-2 text-xs">
            <span className="text-muted-foreground">{t("fields.key")}</span>
            <span className="truncate tracking-wider">{maskProviderApiKey(link.apiKey)}</span>
          </div>
        </div>

        <div className="space-y-1.5 text-xs leading-relaxed">
          <p className="flex gap-1.5 text-muted-foreground">
            <ShieldAlertIcon aria-hidden className="mt-0.5 size-3.5 shrink-0" />
            <span>{t("warning", { host: link.host })}</span>
          </p>
          {existing && !atLimit ? (
            <p className="flex gap-1.5 text-amber-700 dark:text-amber-300">
              <InfoIcon aria-hidden className="mt-0.5 size-3.5 shrink-0" />
              <span>{t("duplicate", { name: existing.name })}</span>
            </p>
          ) : null}
          {atLimit ? (
            <p className="flex gap-1.5 text-amber-700 dark:text-amber-300">
              <InfoIcon aria-hidden className="mt-0.5 size-3.5 shrink-0" />
              <span>{t("limitReached", { max: access.maxPerUser })}</span>
            </p>
          ) : null}
        </div>

        <div className="space-y-1.5">
          {atLimit ? (
            <Button type="button" className={PRIMARY_BUTTON_CLASS} onClick={() => router.push(MODELS_SETTINGS_PATH)}>
              {t("manage")}
            </Button>
          ) : (
            <Button type="button" className={PRIMARY_BUTTON_CLASS} onClick={() => setDialogOpen(true)}>
              {t("continue")}
            </Button>
          )}
          <Button type="button" variant="ghost" className={CANCEL_BUTTON_CLASS} onClick={leave}>
            {t("cancel")}
          </Button>
        </div>
      </div>

      <ModelsProviderDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        modelProtocols={access.modelProtocols}
        initialDraft={draft}
        source="link"
        lockEndpoint
        onProbe={probe}
        onCreate={async (payload) => {
          const created = await create(payload);
          toast.success(t("imported"));
          router.replace(MODELS_SETTINGS_PATH);
          return created;
        }}
      />
    </ImportFrame>
  );
}

function ImportFrame({ title, description, children }: { title?: string; description?: string; children: React.ReactNode }) {
  return (
    <main className="flex min-h-screen animate-in items-center justify-center px-4 py-8 text-foreground fade-in-0 duration-300">
      <div className="w-full max-w-[360px]">
        <AppLogo width={32} height={32} priority className="mx-auto h-9 w-auto" />
        <div className="space-y-5 px-2 pt-7">
          {title ? (
            <div className="space-y-1.5 text-center">
              <h1 className="text-base font-semibold">{title}</h1>
              {description ? <p className="text-xs leading-relaxed text-muted-foreground">{description}</p> : null}
            </div>
          ) : null}
          {children}
        </div>
      </div>
    </main>
  );
}
