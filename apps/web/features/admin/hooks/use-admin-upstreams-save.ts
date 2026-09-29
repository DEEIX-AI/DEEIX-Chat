"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { createAdminLLMUpstream, updateAdminLLMUpstream } from "@/features/admin/api";
import type {
  AdminLLMUpstreamView,
  CreateAdminLLMUpstreamRequest,
  UpdateAdminLLMUpstreamRequest,
} from "@/features/admin/api/llm-types";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export type UpstreamSaveRequest =
  | { mode: "create"; payload: CreateAdminLLMUpstreamRequest }
  | { mode: "update"; id: number; payload: UpdateAdminLLMUpstreamRequest };

type UseAdminUpstreamsSaveOptions = {
  onSuccess: (item: AdminLLMUpstreamView) => void;
  onOpenChange: (open: boolean) => void;
  onManageModels?: (item: AdminLLMUpstreamView) => void;
};

export function useAdminUpstreamsSave({ onSuccess, onOpenChange, onManageModels }: UseAdminUpstreamsSaveOptions) {
  const t = useTranslations("adminUpstreams");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [pending, setPending] = React.useState(false);

  /**
   * `prepare` runs after the access token is resolved and returns the request
   * to send, or `null` to abort (it reports its own validation errors).
   * `formMode` only selects the failure toast.
   */
  async function submit(formMode: "create" | "edit", prepare: () => UpstreamSaveRequest | null) {
    setPending(true);
    try {
      const token = await resolveAccessToken();
      const request = prepare();
      if (!request) {
        return;
      }
      if (request.mode === "create") {
        const data = await createAdminLLMUpstream(token, request.payload);
        onSuccess(data.upstream);
        onOpenChange(false);
        onManageModels?.(data.upstream);
        toast.success(t("toast.upstreamCreated"));
        return;
      }
      const data = await updateAdminLLMUpstream(token, request.id, request.payload);
      onSuccess(data.upstream);
      onOpenChange(false);
      toast.success(t("toast.upstreamUpdated"));
    } catch (error) {
      toast.error(formMode === "create" ? t("toast.createFailed") : t("toast.updateFailed"), {
        description: resolveErrorMessage(error),
      });
    } finally {
      setPending(false);
    }
  }

  return { pending, submit };
}
