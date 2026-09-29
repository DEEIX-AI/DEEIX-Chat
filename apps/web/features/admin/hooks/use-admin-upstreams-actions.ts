"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  batchDeleteAdminLLMUpstreams,
  deleteAdminLLMUpstream,
  openAdminLLMUpstreamCircuit,
  resetAdminLLMUpstreamCircuit,
} from "@/features/admin/api";
import type { AdminBatchDeleteData, AdminLLMUpstreamView } from "@/features/admin/api/llm-types";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { mergeBatchResultData, runBulkActionInChunks } from "@/shared/lib/bulk-action";

function summarizeBatchDeleteResult(
  result: AdminBatchDeleteData,
  translate: (key: string, values: Record<string, number>) => string,
): string {
  return translate("deleteDialog.summary", {
    success: result.successCount,
    notFound: result.notFoundCount,
    failed: result.failedCount,
  });
}

type UpstreamActionCallbacks<T> = {
  onDone: (value: T) => void;
  onClose: () => void;
};

// Each confirmation dialog mounts its own instance, so `pending` is per dialog.
export function useAdminUpstreamsActions() {
  const t = useTranslations("adminUpstreams");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [pending, setPending] = React.useState(false);

  async function deleteUpstream(upstream: AdminLLMUpstreamView, { onDone, onClose }: UpstreamActionCallbacks<number>) {
    setPending(true);
    try {
      const token = await resolveAccessToken();
      await deleteAdminLLMUpstream(token, upstream.id);
      onDone(upstream.id);
      toast.success(t("toast.upstreamDeleted"));
      onClose();
    } catch (error) {
      toast.error(t("toast.deleteFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setPending(false);
    }
  }

  async function bulkDeleteUpstreams(
    targets: AdminLLMUpstreamView[],
    { onDone, onClose }: UpstreamActionCallbacks<AdminBatchDeleteData>,
  ) {
    setPending(true);
    try {
      const token = await resolveAccessToken();
      const result = mergeBatchResultData(await runBulkActionInChunks({
        items: targets.map((item) => item.id),
        title: t("deleteDialog.deleting"),
        runChunk: (ids) => batchDeleteAdminLLMUpstreams(token, { ids }),
      }));
      onDone(result);
      if (result.failedCount > 0) {
        toast.error(t("toast.bulkDeletePartialFailed"), {
          description: summarizeBatchDeleteResult(result, t),
        });
      } else {
        toast.success(t("toast.bulkDeleteDone"), {
          description: summarizeBatchDeleteResult(result, t),
        });
      }
      onClose();
    } catch (error) {
      toast.error(t("toast.bulkDeleteFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setPending(false);
    }
  }

  async function runCircuitAction(
    upstream: AdminLLMUpstreamView,
    action: "open" | "reset",
    { onDone, onClose }: UpstreamActionCallbacks<AdminLLMUpstreamView>,
  ) {
    const isOpen = action === "open";
    setPending(true);
    try {
      const token = await resolveAccessToken();
      if (isOpen) {
        await openAdminLLMUpstreamCircuit(token, upstream.id);
        onDone({ ...upstream, circuitOpen: true });
        toast.success(t("toast.circuitOpened"));
      } else {
        await resetAdminLLMUpstreamCircuit(token, upstream.id);
        onDone({ ...upstream, circuitOpen: false });
        toast.success(t("toast.circuitReset"));
      }
      onClose();
    } catch (error) {
      toast.error(isOpen ? t("toast.circuitOpenFailed") : t("toast.circuitResetFailed"), {
        description: resolveErrorMessage(error),
      });
    } finally {
      setPending(false);
    }
  }

  return { pending, deleteUpstream, bulkDeleteUpstreams, runCircuitAction };
}
