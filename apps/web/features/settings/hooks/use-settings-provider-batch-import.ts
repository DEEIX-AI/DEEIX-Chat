"use client";

import type { ProviderImportLink } from "@deeix/core";
import * as React from "react";
import { useTranslations } from "next-intl";

import { findModelProviderPreset } from "@/entities/model";
import { modelSelectionPayload } from "@/features/settings/model/model-protocol-choices";
import { PRESELECT_ALL_MAX_MODELS, resolveProviderProtocol } from "@/features/settings/model/model-provider-protocols";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { ApiError } from "@/shared/api/http-client";
import type {
  CreatePersonalProviderPayload,
  PersonalProviderAvailableModelDTO,
  PersonalProviderDTO,
  PersonalProviderProbePayload,
} from "@/shared/api/personal-providers-types";

export type ProviderBatchImportStatus =
  | { kind: "pending" }
  | { kind: "probing" }
  | { kind: "saving" }
  | { kind: "throttled"; seconds: number }
  | { kind: "imported"; modelCount: number }
  | { kind: "needsSelection"; modelCount: number }
  | { kind: "noModels" }
  | { kind: "overLimit" }
  | { kind: "failed"; message: string };

/** Cap on Retry-After; the server window is one minute. */
const MAX_THROTTLE_WAIT_SECONDS = 60;

class BatchImportCancelled extends Error {}

/** Draft for the add dialog, prefilled from a link entry. */
export function providerImportDraft(link: ProviderImportLink) {
  const preset = findModelProviderPreset(link.baseURL);
  return {
    name: link.name || preset?.name || "",
    protocol: resolveProviderProtocol(link.protocol, preset),
    baseURL: link.baseURL,
    apiKey: link.apiKey,
  };
}

/**
 * Imports link entries sequentially. Each import costs two requests against a
 * per-user rate limit, so a 429 waits for Retry-After instead of failing the entry.
 * Catalogs above PRESELECT_ALL_MAX_MODELS are left for the user to select.
 */
export function useSettingsProviderBatchImport({
  links,
  modelProtocols,
  slotsLeft,
  probe,
  create,
}: {
  links: readonly ProviderImportLink[];
  modelProtocols: string[];
  /** Remaining provider slots when the screen opens. */
  slotsLeft: number;
  probe: (payload: PersonalProviderProbePayload) => Promise<PersonalProviderAvailableModelDTO[]>;
  create: (payload: CreatePersonalProviderPayload) => Promise<PersonalProviderDTO>;
}) {
  const t = useTranslations("settings.modelsPage.dialog");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [statuses, setStatuses] = React.useState<ProviderBatchImportStatus[]>(() => links.map(() => ({ kind: "pending" })));
  // Latest statuses, readable after an async run completes.
  const statusesRef = React.useRef(statuses);
  const [running, setRunning] = React.useState(false);
  const mountedRef = React.useRef(true);
  // Tracked locally; the provider list updates a render later.
  const slotsLeftRef = React.useRef(slotsLeft);

  React.useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const setStatus = React.useCallback((index: number, status: ProviderBatchImportStatus) => {
    if (!mountedRef.current) throw new BatchImportCancelled();
    statusesRef.current = statusesRef.current.map((item, itemIndex) => (itemIndex === index ? status : item));
    setStatuses(statusesRef.current);
  }, []);

  /** Retries a request after a 429. */
  const throttled = React.useCallback(
    async <T>(index: number, request: () => Promise<T>, resume: ProviderBatchImportStatus): Promise<T> => {
      for (;;) {
        try {
          return await request();
        } catch (error) {
          if (!(error instanceof ApiError) || error.status !== 429) throw error;
          const wait = Math.min(Math.max(Math.ceil(error.retryAfterSeconds ?? MAX_THROTTLE_WAIT_SECONDS), 1), MAX_THROTTLE_WAIT_SECONDS);
          for (let seconds = wait; seconds > 0; seconds -= 1) {
            setStatus(index, { kind: "throttled", seconds });
            await new Promise((resolve) => window.setTimeout(resolve, 1000));
          }
          setStatus(index, resume);
        }
      }
    },
    [setStatus],
  );

  const importEntry = React.useCallback(
    async (index: number) => {
      const link = links[index];
      if (!link) return;
      const draft = providerImportDraft(link);
      try {
        setStatus(index, { kind: "probing" });
        const endpoint = { protocol: draft.protocol, baseURL: draft.baseURL, apiKey: draft.apiKey };
        const models = await throttled(index, () => probe(endpoint), { kind: "probing" });
        if (models.length === 0) {
          setStatus(index, { kind: "noModels" });
          return;
        }
        if (models.length > PRESELECT_ALL_MAX_MODELS) {
          setStatus(index, { kind: "needsSelection", modelCount: models.length });
          return;
        }
        setStatus(index, { kind: "saving" });
        const selected = new Map(models.map((model) => [model.name, [...model.suggestedProtocols]]));
        const payload: CreatePersonalProviderPayload = {
          ...endpoint,
          name: draft.name || undefined,
          models: modelSelectionPayload(models, selected, modelProtocols),
          source: "link",
        };
        const created = await throttled(index, () => create(payload), { kind: "saving" });
        slotsLeftRef.current -= 1;
        setStatus(index, { kind: "imported", modelCount: created.models.length });
      } catch (error) {
        if (error instanceof BatchImportCancelled || !mountedRef.current) return;
        setStatus(index, { kind: "failed", message: resolveErrorMessage(error, t("saveFailed")) });
      }
    },
    [create, links, modelProtocols, probe, resolveErrorMessage, setStatus, t, throttled],
  );

  /** Imports pending and failed entries. Resolves to the final statuses, or null if unmounted. */
  const runAll = React.useCallback(async (): Promise<readonly ProviderBatchImportStatus[] | null> => {
    setRunning(true);
    try {
      for (const [index, status] of statusesRef.current.entries()) {
        if (!mountedRef.current) return null;
        if (status.kind !== "pending" && status.kind !== "failed" && status.kind !== "overLimit") continue;
        if (slotsLeftRef.current <= 0) {
          setStatus(index, { kind: "overLimit" });
          continue;
        }
        await importEntry(index);
      }
      return mountedRef.current ? statusesRef.current : null;
    } catch (error) {
      if (error instanceof BatchImportCancelled) return null;
      throw error;
    } finally {
      if (mountedRef.current) setRunning(false);
    }
  }, [importEntry, setStatus]);

  /** Retries one entry. Resolves like runAll. */
  const retry = React.useCallback(
    async (index: number): Promise<readonly ProviderBatchImportStatus[] | null> => {
      setRunning(true);
      try {
        await importEntry(index);
        return mountedRef.current ? statusesRef.current : null;
      } finally {
        if (mountedRef.current) setRunning(false);
      }
    },
    [importEntry],
  );

  /** Marks an entry saved from the add dialog. */
  const markImported = React.useCallback(
    (index: number, modelCount: number): readonly ProviderBatchImportStatus[] => {
      slotsLeftRef.current -= 1;
      setStatus(index, { kind: "imported", modelCount });
      return statusesRef.current;
    },
    [setStatus],
  );

  return { statuses, running, runAll, retry, markImported };
}
