"use client";

import { useState } from "react";
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
import {
  batchDeleteAdminLLMUpstreams,
  deleteAdminLLMUpstream,
  openAdminLLMUpstreamCircuit,
  resetAdminLLMUpstreamCircuit,
} from "@/features/admin/api";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import type { AdminBatchDeleteData, AdminLLMUpstreamView } from "@/features/admin/api/llm.types";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { toast } from "sonner";

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
  const t = useTranslations("adminChannels");
  const tActions = useTranslations("common.actions");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [pending, setPending] = useState(false);

  async function handleConfirm() {
    if (!upstream) return;
    setPending(true);
    try {
      const token = await resolveAccessToken();
      await deleteAdminLLMUpstream(token, upstream.id);
      onDeleted(upstream.id);
      toast.success(t("toast.upstreamDeleted"));
      onClose();
    } catch (error) {
      toast.error(t("toast.deleteFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setPending(false);
    }
  }

  return (
    <AlertDialog open={upstream !== null} onOpenChange={(open) => !open && onClose()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t("deleteDialog.title")}</AlertDialogTitle>
          <AlertDialogDescription>
            {t("deleteDialog.description", { name: upstream?.name ?? "" })}
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
  const t = useTranslations("adminChannels");
  const tActions = useTranslations("common.actions");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [pending, setPending] = useState(false);

  const visibleTargets = targets.slice(0, 6);
  const hiddenCount = Math.max(0, targets.length - visibleTargets.length);

  async function handleConfirm() {
    if (targets.length === 0) return;
    setPending(true);
    try {
      const token = await resolveAccessToken();
      const result = await batchDeleteAdminLLMUpstreams(token, {
        ids: targets.map((item) => item.id),
      });
      onDeleted(result);
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

  return (
    <AlertDialog open={open} onOpenChange={(nextOpen) => !nextOpen && !pending && onClose()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t("deleteDialog.bulkTitle")}</AlertDialogTitle>
          <AlertDialogDescription>
            {t("deleteDialog.bulkDescription", { count: targets.length })}
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
          {hiddenCount > 0 ? (
            <span className="inline-flex items-center rounded-md border px-2 py-1 text-xs text-muted-foreground">
              {t("deleteDialog.moreTargets", { count: hiddenCount })}
            </span>
          ) : null}
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
            {pending ? <SpinnerLabel>{t("deleteDialog.deleting")}</SpinnerLabel> : t("deleteDialog.confirmBulk", { count: targets.length })}
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
  const t = useTranslations("adminChannels");
  const tActions = useTranslations("common.actions");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [pending, setPending] = useState(false);

  const isOpen = action === "open";
  const title = isOpen ? t("circuitDialog.openTitle") : t("circuitDialog.resetTitle");
  const description = isOpen
    ? t("circuitDialog.openDescription", { name: upstream?.name ?? "" })
    : t("circuitDialog.resetDescription", { name: upstream?.name ?? "" });

  async function handleConfirm() {
    if (!upstream) return;
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
