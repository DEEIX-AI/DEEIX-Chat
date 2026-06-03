"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { SpinnerLabel } from "@/components/ui/spinner";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  batchDeleteAdminLLMModels,
  deleteAdminLLMModel,
} from "@/features/admin/api";
import type {
  AdminBatchDeleteData,
  AdminLLMModelDTO,
} from "@/features/admin/api/llm.types";

import { resolveErrorMessage } from "@/features/admin/types/llm";

function summarizeBatchDeleteResult(result: AdminBatchDeleteData, t: (key: string, values?: Record<string, number>) => string): string {
  return t("deleteDialog.batchSummary", {
    success: result.successCount,
    notFound: result.notFoundCount,
    failed: result.failedCount,
  });
}

type DeleteModelDialogProps = {
  target: AdminLLMModelDTO | null;
  onClose: () => void;
  onDeleted: () => void;
};

export function DeleteModelDialog({
  target,
  onClose,
  onDeleted,
}: DeleteModelDialogProps) {
  const t = useTranslations("adminModels");
  const commonT = useTranslations("common");
  const [pending, setPending] = React.useState(false);

  const handleDelete = React.useCallback(async () => {
    if (!target) return;

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
      toast.error(t("toast.modelDeleteFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setPending(false);
    }
  }, [onDeleted, t, target]);

  return (
    <AlertDialog open={!!target} onOpenChange={(open) => !open && !pending && onClose()}>
      <AlertDialogContent className="sm:max-w-[400px]">
        <AlertDialogHeader>
          <AlertDialogTitle>{t("deleteDialog.title")}</AlertDialogTitle>
          <AlertDialogDescription>
            {t("deleteDialog.description", { model: target?.platformModelName ?? "" })}
          </AlertDialogDescription>
        </AlertDialogHeader>

        <AlertDialogFooter>
          <AlertDialogCancel disabled={pending}>
            {commonT("actions.cancel")}
          </AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            onClick={(event) => {
              event.preventDefault();
              void handleDelete();
            }}
            disabled={pending}
          >
            {pending ? <SpinnerLabel>{t("deleteDialog.deleting")}</SpinnerLabel> : t("deleteDialog.confirm")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

type BulkDeleteModelsDialogProps = {
  targets: AdminLLMModelDTO[];
  open: boolean;
  onClose: () => void;
  onDeleted: (result: AdminBatchDeleteData) => void;
};

export function BulkDeleteModelsDialog({
  targets,
  open,
  onClose,
  onDeleted,
}: BulkDeleteModelsDialogProps) {
  const t = useTranslations("adminModels");
  const commonT = useTranslations("common");
  const [pending, setPending] = React.useState(false);

  const visibleTargets = React.useMemo(() => targets.slice(0, 6), [targets]);
  const hiddenCount = Math.max(0, targets.length - visibleTargets.length);

  const handleDelete = React.useCallback(async () => {
    if (targets.length === 0) return;

    const token = await resolveAccessToken();
    if (!token) {
      toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
      return;
    }

    setPending(true);
    try {
      const result = await batchDeleteAdminLLMModels(token, {
        ids: targets.map((item) => item.id),
      });

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
      toast.error(t("toast.bulkDeleteFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setPending(false);
    }
  }, [onDeleted, t, targets]);

  return (
    <AlertDialog open={open} onOpenChange={(nextOpen) => !nextOpen && !pending && onClose()}>
      <AlertDialogContent className="sm:max-w-[480px]">
        <AlertDialogHeader>
          <AlertDialogTitle>{t("deleteDialog.bulkTitle")}</AlertDialogTitle>
          <AlertDialogDescription>
            {t("deleteDialog.bulkDescription", { count: targets.length })}
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className="space-y-3 pt-1">
          <div className="flex flex-wrap gap-1.5">
            {visibleTargets.map((item) => (
              <Badge key={item.id} variant="secondary" className="max-w-full text-xs" title={item.platformModelName}>
                {item.platformModelName}
              </Badge>
            ))}
            {hiddenCount > 0 ? (
              <Badge variant="outline" className="text-xs text-muted-foreground">
                {t("deleteDialog.remaining", { count: hiddenCount })}
              </Badge>
            ) : null}
          </div>
        </div>

        <AlertDialogFooter>
          <AlertDialogCancel disabled={pending}>
            {commonT("actions.cancel")}
          </AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            onClick={(event) => {
              event.preventDefault();
              void handleDelete();
            }}
            disabled={pending || targets.length === 0}
          >
            {pending ? <SpinnerLabel>{t("deleteDialog.deleting")}</SpinnerLabel> : t("deleteDialog.bulkConfirm", { count: targets.length })}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
