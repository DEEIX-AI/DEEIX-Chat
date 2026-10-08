"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import {
  createPersonalProvider,
  deletePersonalProvider,
  listPersonalProviderModels,
  listPersonalProviders,
  probePersonalProvider,
  updatePersonalProvider,
} from "@/shared/api/personal-providers";
import type {
  CreatePersonalProviderPayload,
  PersonalProviderDTO,
  PersonalProviderProbePayload,
  UpdatePersonalProviderPayload,
} from "@/shared/api/personal-providers-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

async function requireToken(): Promise<string> {
  const token = await resolveAccessToken();
  if (!token) throw new Error("missing access token");
  return token;
}

/**
 * State and actions for the user's own model providers. Keys only travel from
 * the form to the server; nothing here keeps or reads one back.
 */
export function useSettingsModelProviders(enabled: boolean) {
  const t = useTranslations("settings.modelsPage");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [providers, setProviders] = React.useState<PersonalProviderDTO[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [busyID, setBusyID] = React.useState("");

  const reload = React.useCallback(async () => {
    if (!enabled) {
      setProviders([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const data = await listPersonalProviders(await requireToken());
      setProviders(data.providers ?? []);
    } catch (error) {
      toast.error(resolveErrorMessage(error, t("toasts.loadFailed")));
    } finally {
      setLoading(false);
    }
  }, [enabled, resolveErrorMessage, t]);

  React.useEffect(() => {
    void reload();
  }, [reload]);

  const probe = React.useCallback(async (payload: PersonalProviderProbePayload): Promise<string[]> => {
    const data = await probePersonalProvider(await requireToken(), payload);
    return data.models ?? [];
  }, []);

  const create = React.useCallback(async (payload: CreatePersonalProviderPayload): Promise<PersonalProviderDTO> => {
    const data = await createPersonalProvider(await requireToken(), payload);
    setProviders((current) => [...current, data.provider]);
    return data.provider;
  }, []);

  const runOnProvider = React.useCallback(
    async (id: string, action: (token: string) => Promise<PersonalProviderDTO | null>, successMessage: string) => {
      setBusyID(id);
      try {
        const next = await action(await requireToken());
        setProviders((current) =>
          next ? current.map((item) => (item.id === id ? next : item)) : current.filter((item) => item.id !== id),
        );
        toast.success(successMessage);
        return true;
      } catch (error) {
        toast.error(resolveErrorMessage(error, t("toasts.saveFailed")));
        return false;
      } finally {
        setBusyID("");
      }
    },
    [resolveErrorMessage, t],
  );

  const update = React.useCallback(
    (id: string, payload: UpdatePersonalProviderPayload, successMessage: string) =>
      runOnProvider(id, async (token) => (await updatePersonalProvider(token, id, payload)).provider, successMessage),
    [runOnProvider],
  );

  const remove = React.useCallback(
    (id: string) =>
      runOnProvider(
        id,
        async (token) => {
          await deletePersonalProvider(token, id);
          return null;
        },
        t("toasts.deleted"),
      ),
    [runOnProvider, t],
  );

  const listModels = React.useCallback(async (id: string): Promise<string[]> => {
    const data = await listPersonalProviderModels(await requireToken(), id);
    return data.models ?? [];
  }, []);

  /** Test the saved key against the provider; the server records the result on the row either way. */
  const check = React.useCallback(
    async (id: string) => {
      setBusyID(id);
      try {
        const models = await listModels(id);
        toast.success(t("toasts.checkPassed", { count: models.length }));
      } catch (error) {
        toast.error(resolveErrorMessage(error, t("toasts.checkFailed")));
      } finally {
        setBusyID("");
        await reload();
      }
    },
    [listModels, reload, resolveErrorMessage, t],
  );

  return { providers, loading, busyID, reload, probe, create, update, remove, listModels, check };
}
