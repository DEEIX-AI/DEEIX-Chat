"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { isDesktopApp } from "@/shared/platform";
import {
  DEFAULT_PROXY_SETTINGS,
  getNetworkProxy,
  isProxyError,
  type ProxyErrorField,
  type ProxySettings,
  type ProxyStatus,
  setNetworkProxy,
} from "@/shared/platform/desktop-proxy";
import { relaunchApp } from "@/shared/platform/desktop-updater";

export type DesktopNetworkProxyState = {
  /** "unsupported" in a browser, where there is nothing to configure. */
  status: "loading" | "ready" | "failed" | "unsupported";
  proxy: ProxyStatus | null;
  draft: ProxySettings;
  setDraft: React.Dispatch<React.SetStateAction<ProxySettings>>;
  /** `undefined` keeps the stored password, `""` removes it. */
  password: string | undefined;
  setPassword: (password: string | undefined) => void;
  dirty: boolean;
  saving: boolean;
  /** The input the shell refused, with a message ready to show under it. */
  fieldError: { field: ProxyErrorField; message: string } | null;
  save: () => Promise<void>;
  reset: () => void;
  restart: () => void;
};

/**
 * The desktop network proxy setting: what is saved, what this launch uses, and
 * the edit in progress. Saving writes `network.json` and the keychain through
 * the shell; the new setting applies from the next launch.
 */
export function useDesktopNetworkProxy(): DesktopNetworkProxyState {
  const t = useTranslations("desktopNetwork");
  const [status, setStatus] = React.useState<DesktopNetworkProxyState["status"]>("loading");
  const [proxy, setProxy] = React.useState<ProxyStatus | null>(null);
  const [draft, setDraft] = React.useState<ProxySettings>(DEFAULT_PROXY_SETTINGS);
  const [password, setPassword] = React.useState<string | undefined>(undefined);
  const [saving, setSaving] = React.useState(false);
  const [fieldError, setFieldError] = React.useState<DesktopNetworkProxyState["fieldError"]>(null);

  React.useEffect(() => {
    if (!isDesktopApp()) {
      setStatus("unsupported");
      return;
    }
    let cancelled = false;
    getNetworkProxy().then(
      (loaded) => {
        if (cancelled) return;
        if (loaded === null) {
          setStatus("unsupported");
          return;
        }
        setProxy(loaded);
        setDraft(loaded.settings);
        setStatus("ready");
      },
      () => {
        if (cancelled) return;
        setStatus("failed");
        toast.error(t("toasts.loadFailed"));
      },
    );
    return () => {
      cancelled = true;
    };
  }, [t]);

  // A stale message would point at an input the user has since changed.
  const editDraft = React.useCallback<React.Dispatch<React.SetStateAction<ProxySettings>>>((next) => {
    setFieldError(null);
    setDraft(next);
  }, []);

  const editPassword = React.useCallback((next: string | undefined) => {
    setFieldError(null);
    setPassword(next);
  }, []);

  const dirty = proxy !== null && (password !== undefined || !sameSettings(draft, proxy.settings));

  const reset = React.useCallback(() => {
    if (proxy) setDraft(proxy.settings);
    setPassword(undefined);
    setFieldError(null);
  }, [proxy]);

  const save = React.useCallback(async () => {
    setSaving(true);
    try {
      const saved = await setNetworkProxy({ settings: draft, password });
      setProxy(saved);
      setDraft(saved.settings);
      setPassword(undefined);
      setFieldError(null);
      toast.success(t("toasts.saved"), { description: saved.restartRequired ? t("toasts.savedDescription") : undefined });
    } catch (error) {
      if (isProxyError(error) && error.kind === "invalid" && error.field) {
        setFieldError({
          field: error.field,
          message: error.field === "bypass" ? t("errors.bypass", { value: error.value ?? "" }) : t(`errors.${error.field}`),
        });
      } else if (isProxyError(error) && error.kind === "policy") {
        toast.error(t("errors.policy"));
      } else {
        const message = isProxyError(error) ? error.message : error instanceof Error ? error.message : String(error);
        toast.error(t("toasts.saveFailed"), { description: t("errors.storage", { message }) });
      }
    } finally {
      setSaving(false);
    }
  }, [draft, password, t]);

  const restart = React.useCallback(() => {
    void relaunchApp();
  }, []);

  return {
    status,
    proxy,
    draft,
    setDraft: editDraft,
    password,
    setPassword: editPassword,
    dirty,
    saving,
    fieldError,
    save,
    reset,
    restart,
  };
}

function sameSettings(a: ProxySettings, b: ProxySettings): boolean {
  return (
    a.mode === b.mode &&
    a.scheme === b.scheme &&
    a.host === b.host &&
    a.port === b.port &&
    a.username === b.username &&
    a.bypass === b.bypass
  );
}
