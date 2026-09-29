"use client";

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
import { SpinnerLabel } from "@/components/ui/spinner";
import { useAdminUpstreamsActions } from "@/features/admin/hooks/use-admin-upstreams-actions";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import type { AdminBatchDeleteData, AdminLLMUpstreamView } from "@/features/admin/api/llm-types";

// ---------------------------------------------------------------------------
// DeleteUpstreamDialog
// ---------------------------------------------------------------------------

type DeleteUpstreamDialogProps = {
  upstream: AdminLLMUpstreamView | null;
  onClose: () => void;
  onDeleted: (id: number) => void;
};

export function DeleteUpstreamDialog({
  upstream,
  onClose,
  onDeleted,
}: DeleteUpstreamDialogProps) {
  const t = useTranslations("adminUpstreams");
  const tActions = useTranslations("common.actions");
  const { pending, deleteUpstream } = useAdminUpstreamsActions();
  const stableUpstream = useDialogSnapshot(upstream);

  async function handleConfirm() {
    if (!upstream) return;
    await deleteUpstream(upstream, { onDone: onDeleted, onClose });
  }

  return (
    <AlertDialog open={upstream !== null} onOpenChange={(open) => !open && onClose()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t("deleteDialog.title")}</AlertDialogTitle>
          <AlertDialogDescription>
            {t("deleteDialog.description", { name: stableUpstream?.name ?? "" })}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={pending} onClick={onClose}>
            {tActions("cancel")}
          </AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={pending}
            onClick={(e) => {
              e.preventDefault();
              void handleConfirm();
            }}
          >
            {pending ? <SpinnerLabel>{t("deleteDialog.deleting")}</SpinnerLabel> : tActions("delete")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

type BulkDeleteUpstreamsDialogProps = {
  open: boolean;
  targets: AdminLLMUpstreamView[];
  onClose: () => void;
  onDeleted: (result: AdminBatchDeleteData) => void;
};

export function BulkDeleteUpstreamsDialog({
  open,
  targets,
  onClose,
  onDeleted,
}: BulkDeleteUpstreamsDialogProps) {
  const t = useTranslations("adminUpstreams");
  const tActions = useTranslations("common.actions");
  const { pending, bulkDeleteUpstreams } = useAdminUpstreamsActions();

  const visibleTargets = targets.slice(0, 6);

  async function handleConfirm() {
    if (targets.length === 0) return;
    await bulkDeleteUpstreams(targets, { onDone: onDeleted, onClose });
  }

  return (
    <AlertDialog open={open} onOpenChange={(nextOpen) => !nextOpen && !pending && onClose()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t("deleteDialog.bulkTitle")}</AlertDialogTitle>
          <AlertDialogDescription>
            {t("deleteDialog.bulkDescription")}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <div className="flex flex-wrap gap-1.5">
          {visibleTargets.map((item) => (
            <span
              key={item.id}
              className="inline-flex items-center rounded-md bg-muted px-2 py-1 text-xs text-muted-foreground"
            >
              {item.name}
            </span>
          ))}
        </div>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={pending} onClick={onClose}>
            {tActions("cancel")}
          </AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={pending || targets.length === 0}
            onClick={(event) => {
              event.preventDefault();
              void handleConfirm();
            }}
          >
            {pending ? <SpinnerLabel>{t("deleteDialog.deleting")}</SpinnerLabel> : t("deleteDialog.confirmBulk")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

// ---------------------------------------------------------------------------
// CircuitActionDialog
// ---------------------------------------------------------------------------

type CircuitActionDialogProps = {
  upstream: AdminLLMUpstreamView | null;
  action: "open" | "reset";
  onClose: () => void;
  onDone: (updated: AdminLLMUpstreamView) => void;
};

export function CircuitActionDialog({
  upstream,
  action,
  onClose,
  onDone,
}: CircuitActionDialogProps) {
  const t = useTranslations("adminUpstreams");
  const tActions = useTranslations("common.actions");
  const { pending, runCircuitAction } = useAdminUpstreamsActions();
  const stableUpstream = useDialogSnapshot(upstream);

  const isOpen = action === "open";
  const title = isOpen ? t("circuitDialog.openTitle") : t("circuitDialog.resetTitle");
  const description = isOpen
    ? t("circuitDialog.openDescription", { name: stableUpstream?.name ?? "" })
    : t("circuitDialog.resetDescription", { name: stableUpstream?.name ?? "" });

  async function handleConfirm() {
    if (!upstream) return;
    await runCircuitAction(upstream, action, { onDone, onClose });
  }

  return (
    <AlertDialog open={upstream !== null} onOpenChange={(open) => !open && onClose()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={pending} onClick={onClose}>
            {tActions("cancel")}
          </AlertDialogCancel>
          <AlertDialogAction
            disabled={pending}
            onClick={(e) => {
              e.preventDefault();
              void handleConfirm();
            }}
          >
            {pending ? (
              <SpinnerLabel>{isOpen ? t("circuitDialog.opening") : t("circuitDialog.resetting")}</SpinnerLabel>
            ) : (
              title
            )}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
