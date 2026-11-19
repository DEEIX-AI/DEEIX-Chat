"use client";

import * as React from "react";

import {
  createAdminDiscussionTemplate,
  deleteAdminDiscussionTemplate,
  listAdminDiscussionTemplates,
  updateAdminDiscussionTemplate,
} from "@/shared/api/discussion";
import type {
  DiscussionTemplateDTO,
  DiscussionTemplatePayload,
} from "@/shared/api/discussion-types";
import { listPublicModels } from "@/shared/api/model";
import type { PublicModelDTO } from "@/shared/api/model-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export function useAdminDiscussionsTemplates() {
  const [templates, setTemplates] = React.useState<DiscussionTemplateDTO[]>([]);
  const [models, setModels] = React.useState<PublicModelDTO[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState("");

  React.useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) {
          return;
        }
        const [items, modelItems] = await Promise.all([
          listAdminDiscussionTemplates(token),
          listPublicModels(token).catch((): PublicModelDTO[] => []),
        ]);
        if (cancelled) {
          return;
        }
        setTemplates(items);
        setModels(modelItems.filter((item) => !item.kindsJSON.includes("discussion")));
      } catch (err) {
        if (!cancelled) {
          setError(String(err));
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const saveTemplate = React.useCallback(
    async (
      publicID: string | undefined,
      payload: DiscussionTemplatePayload,
    ): Promise<DiscussionTemplateDTO | null> => {
      setError("");
      try {
        const token = await resolveAccessToken();
        if (!token) {
          return null;
        }
        const saved = publicID
          ? await updateAdminDiscussionTemplate(token, publicID, payload)
          : await createAdminDiscussionTemplate(token, payload);
        setTemplates((current) => {
          const next = current.some((item) => item.publicID === saved.publicID)
            ? current.map((item) => (item.publicID === saved.publicID ? saved : item))
            : [...current, saved];
          return next.sort((left, right) => left.sortOrder - right.sortOrder || left.name.localeCompare(right.name));
        });
        return saved;
      } catch (err) {
        setError(String(err));
        return null;
      }
    },
    [],
  );

  const deleteTemplate = React.useCallback(async (publicID: string): Promise<boolean> => {
    setError("");
    try {
      const token = await resolveAccessToken();
      if (!token) {
        return false;
      }
      await deleteAdminDiscussionTemplate(token, publicID);
      setTemplates((current) => current.filter((item) => item.publicID !== publicID));
      return true;
    } catch (err) {
      setError(String(err));
      return false;
    }
  }, []);

  return { templates, models, loading, error, saveTemplate, deleteTemplate };
}