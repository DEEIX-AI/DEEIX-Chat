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
import { ProviderBatchImportScreen } from "@/features/settings/components/provider-batch-import-screen";
import {
  CANCEL_BUTTON_CLASS,
  ImportFrame,
  MODELS_SETTINGS_PATH,
  PRIMARY_BUTTON_CLASS,
} from "@/features/settings/components/provider-import-frame";
import { useSettingsModelProviders } from "@/features/settings/hooks/use-settings-model-providers";
import {
  captureProviderImportFragment,
  clearPendingProviderImport,
  readPendingProviderImport,
} from "@/features/settings/utils/provider-import";
import { UserLocaleSync } from "@/i18n/user-locale-sync";
import { AuthGuard } from "@/shared/auth/auth-guard";
import { usePersonalProviderAccess } from "@/shared/hooks/use-personal-provider-access";

/**
 * `/import` route. The key is removed from the URL before anything else runs,
 * including the login redirect. Nothing is saved until the user confirms.
 */
export function ProviderImportPage() {
  const [fragment, setFragment] = React.useState<string | null>(null);

  React.useLayoutEffect(() => {
    // Prefer a new link over the one stashed before sign-in.
    setFragment(captureProviderImportFragment() || readPendingProviderImport());
    // Pasting a link into a tab already on /import only changes the hash.
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
      {/* Rendered outside the app shell, so the account locale is applied here. */}
      <UserLocaleSync />
      {/* Remount for each link. */}
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

  // Signed in; the stash is no longer needed.
  React.useEffect(() => {
    clearPendingProviderImport();
  }, []);

  const draft = React.useMemo<Partial<ModelProviderDraft> | undefined>(() => {
    const link = parsed.ok ? parsed.links[0] : undefined;
    if (!link) return undefined;
    return {
      name: link.name,
      baseURL: link.baseURL,
      apiKey: link.apiKey,
      protocol: link.protocol ?? undefined,
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
    const reason = t(`invalid.${parsed.error}`);
    return (
      <ImportFrame title={t("invalid.title")} description={parsed.entry ? t("invalid.entry", { entry: parsed.entry, reason }) : reason}>
        <Button type="button" className={PRIMARY_BUTTON_CLASS} onClick={leave}>
          {t("backToChat")}
        </Button>
      </ImportFrame>
    );
  }

  const atLimit = providers.length >= access.maxPerUser;
  const [link] = parsed.links;
  if (parsed.links.length > 1 || !link) {
    return (
      <ProviderBatchImportScreen
        links={parsed.links}
        providers={providers}
        maxPerUser={access.maxPerUser}
        modelProtocols={access.modelProtocols}
        probe={probe}
        create={create}
        onLeave={leave}
      />
    );
  }
  const preset = findModelProviderPreset(link.baseURL);
  const existing = providers.find((provider) => provider.host === link.host);

  return (
    <ImportFrame title={t("title")}>
      <div className="space-y-5">
        <div className="overflow-hidden rounded-lg border border-border/60">
          <div className="flex min-w-0 items-center gap-3 px-3.5 py-3">
            <ModelIcon iconUrl={preset ? resolveModelIconURL(preset.icon) : null} label={link.host} />
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">{link.name || preset?.name || link.host}</p>
              {/* Show the full address; it is what a malicious link would change. */}
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
