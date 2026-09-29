"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { upsertAdminLLMUpstreamModel } from "@/features/admin/api";
import type { UpsertAdminLLMUpstreamModelRequest } from "@/features/admin/api/llm-types";
import { displayToKindsJson, type NewBindingFormState } from "@/features/admin/model/upstreams-models";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export function useAdminUpstreamsModelBinding() {
  const t = useTranslations("adminUpstreams");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [saving, setSaving] = React.useState(false);

  /** `onCreated` runs right after the success toast. */
  async function createBinding(upstreamId: number, form: NewBindingFormState, onCreated: () => void) {
    if (!form.upstreamModelName.trim() || !form.platformModelName.trim()) {
      toast.error(t("modelsDialog.bindingNamesRequired"));
      return;
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      const payload: UpsertAdminLLMUpstreamModelRequest = {
        upstreamModelName: form.upstreamModelName.trim(),
        platformModelName: form.platformModelName.trim(),
        protocols: form.protocols,
        kindsJSON: displayToKindsJson(form.kindsDisplay),
        status: form.status,
        priority: 1,
        weight: 1,
      };
      await upsertAdminLLMUpstreamModel(token, upstreamId, payload);
      toast.success(t("modelsDialog.bindingCreated"));
      onCreated();
    } catch (err) {
      toast.error(t("toast.createFailed"), { description: resolveErrorMessage(err) });
    } finally {
      setSaving(false);
    }
  }

  return { saving, createBinding };
}
