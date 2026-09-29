"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

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
import type {
  AdminBatchDeleteData,
  AdminLLMModelDTO,
} from "@/features/admin/api/llm-types";
import { useAdminModelsDelete } from "@/features/admin/hooks/use-admin-models-delete";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";

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
  const { pending, deleteModel } = useAdminModelsDelete();
  const stableTarget = useDialogSnapshot(target);

  const handleDelete = React.useCallback(async () => {
    if (!target) return;
    await deleteModel(target, onDeleted);
  }, [deleteModel, onDeleted, target]);

  return (
    <AlertDialog open={!!target} onOpenChange={(open) => !open && !pending && onClose()}>
      <AlertDialogContent className="sm:max-w-[400px]">
        <AlertDialogHeader>
          <AlertDialogTitle>{t("deleteDialog.title")}</AlertDialogTitle>
          <AlertDialogDescription>
            {t("deleteDialog.description", { model: stableTarget?.platformModelName ?? "" })}
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
  const { pending, bulkDeleteModels } = useAdminModelsDelete();

  const visibleTargets = React.useMemo(() => targets.slice(0, 6), [targets]);

  const handleDelete = React.useCallback(async () => {
    if (targets.length === 0) return;
    await bulkDeleteModels(targets, onDeleted);
  }, [bulkDeleteModels, onDeleted, targets]);

  return (
    <AlertDialog open={open} onOpenChange={(nextOpen) => !nextOpen && !pending && onClose()}>
      <AlertDialogContent className="sm:max-w-[480px]">
        <AlertDialogHeader>
          <AlertDialogTitle>{t("deleteDialog.bulkTitle")}</AlertDialogTitle>
          <AlertDialogDescription>
            {t("deleteDialog.bulkDescription")}
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className="space-y-3 pt-1">
          <div className="flex flex-wrap gap-1.5">
            {visibleTargets.map((item) => (
              <Badge key={item.id} variant="secondary" className="max-w-full text-xs" title={item.platformModelName}>
                {item.platformModelName}
              </Badge>
            ))}
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
            {pending ? <SpinnerLabel>{t("deleteDialog.deleting")}</SpinnerLabel> : t("deleteDialog.bulkConfirm")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
