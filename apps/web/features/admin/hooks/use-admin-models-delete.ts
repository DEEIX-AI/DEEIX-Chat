"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { batchDeleteAdminLLMModels, deleteAdminLLMModel } from "@/features/admin/api";
import type { AdminBatchDeleteData, AdminLLMModelDTO } from "@/features/admin/api/llm-types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { mergeBatchResultData, runBulkActionInChunks } from "@/shared/lib/bulk-action";

function summarizeBatchDeleteResult(result: AdminBatchDeleteData, t: (key: string, values?: Record<string, number>) => string): string {
  return t("deleteDialog.batchSummary", {
    success: result.successCount,
    notFound: result.notFoundCount,
    failed: result.failedCount,
  });
}

// Each delete dialog mounts its own instance, so `pending` is per dialog.
export function useAdminModelsDelete() {
  const t = useTranslations("adminModels");
  const [pending, setPending] = React.useState(false);

  const deleteModel = React.useCallback(async (target: AdminLLMModelDTO, onDeleted: () => void) => {
    const token = await resolveAccessToken();
    if (!token) {
      toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
      return;
    }

    setPending(true);
    try {
      await deleteAdminLLMModel(token, target.id);
      toast.success(t("toast.modelDeleted"));
      onDeleted();
    } catch (error) {
      toast.error(t("toast.modelDeleteFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setPending(false);
    }
  }, [t]);

  const bulkDeleteModels = React.useCallback(async (
    targets: AdminLLMModelDTO[],
    onDeleted: (result: AdminBatchDeleteData) => void,
  ) => {
    const token = await resolveAccessToken();
    if (!token) {
      toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
      return;
    }

    setPending(true);
    try {
      const result = mergeBatchResultData(await runBulkActionInChunks({
        items: targets.map((item) => item.id),
        title: t("deleteDialog.deleting"),
        runChunk: (ids) => batchDeleteAdminLLMModels(token, { ids }),
      }));

      onDeleted(result);
      if (result.failedCount > 0) {
        toast.error(t("toast.bulkDeletePartialFailed"), {
          description: summarizeBatchDeleteResult(result, t),
        });
      } else {
        toast.success(t("toast.bulkDeleteCompleted"), {
          description: summarizeBatchDeleteResult(result, t),
        });
      }
    } catch (error) {
      toast.error(t("toast.bulkDeleteFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setPending(false);
    }
  }, [t]);

  return { pending, deleteModel, bulkDeleteModels };
}
