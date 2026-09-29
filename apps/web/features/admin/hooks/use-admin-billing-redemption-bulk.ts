"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  batchDeleteAdminRedemptionCodes,
  deleteAdminRedemptionCode,
  revealAdminRedemptionCode,
  updateAdminRedemptionCode,
} from "@/features/admin/api";
import type { AdminRedemptionCodeDTO } from "@/features/admin/api/billing-types";
import { downloadJSONFile } from "@/features/admin/model/billing-settings";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import type {
  RedemptionListOptions,
  RedemptionListOverrides,
} from "@/features/admin/hooks/use-admin-billing-redemption-codes";
import type { CopyActionInput } from "@/shared/components/copy-action";
import { mergeBatchResultData, runBulkActionInChunks } from "@/shared/lib/bulk-action";

type UseAdminBillingRedemptionBulkOptions = {
  redemptionCodes: AdminRedemptionCodeDTO[];
  setRedemptionCodes: React.Dispatch<React.SetStateAction<AdminRedemptionCodeDTO[]>>;
  redemptionTotal: number;
  setRedemptionTotal: React.Dispatch<React.SetStateAction<number>>;
  selectedRedemptionIDs: Set<number>;
  setSelectedRedemptionIDs: React.Dispatch<React.SetStateAction<Set<number>>>;
  loadRedemptionCodes: (overrides?: RedemptionListOverrides, options?: RedemptionListOptions) => Promise<void>;
  copy: (value: string, overrides?: CopyActionInput) => Promise<boolean>;
};

function redemptionCodesExportFilename(): string {
  const date = new Date().toISOString().slice(0, 10);
  return `deeix-chat-redemption-codes-${date}.json`;
}

// Plaintext reveal, copy/export, bulk status and delete actions for
// redemption codes. All of them share one pending flag, like the original UI.
export function useAdminBillingRedemptionBulk({
  redemptionCodes,
  setRedemptionCodes,
  redemptionTotal,
  setRedemptionTotal,
  selectedRedemptionIDs,
  setSelectedRedemptionIDs,
  loadRedemptionCodes,
  copy,
}: UseAdminBillingRedemptionBulkOptions) {
  const t = useTranslations("adminBilling");
  const [redemptionBulkPending, setRedemptionBulkPending] = React.useState(false);

  async function fetchRedemptionCodePlaintextWithToken(accessToken: string, item: AdminRedemptionCodeDTO): Promise<string> {
    const data = await revealAdminRedemptionCode(accessToken, item.id);
    const code = data.code.code?.trim();
    if (!code) {
      throw new Error(t("toast.redemptionCodeRevealUnavailable"));
    }
    return code;
  }

  async function revealRedemptionCode(item: AdminRedemptionCodeDTO): Promise<string> {
    const token = await resolveAccessToken();
    if (!token) {
      throw new Error(t("toast.sessionExpired"));
    }
    return fetchRedemptionCodePlaintextWithToken(token, item);
  }

  async function revealSelectedRedemptionCodes(): Promise<{
    results: Array<{ item: AdminRedemptionCodeDTO; code: string }>;
    failedCount: number;
  }> {
    const selectedItems = redemptionCodes.filter((item) => selectedRedemptionIDs.has(item.id));
    if (selectedItems.length === 0) {
      return { results: [], failedCount: 0 };
    }
    const token = await resolveAccessToken();
    if (!token) {
      throw new Error(t("toast.sessionExpired"));
    }
    const results: Array<{ item: AdminRedemptionCodeDTO; code: string }> = [];
    let failedCount = 0;
    for (const item of selectedItems) {
      try {
        const code = await fetchRedemptionCodePlaintextWithToken(token, item);
        results.push({ item, code });
      } catch {
        failedCount += 1;
      }
    }
    if (results.length === 0 && failedCount > 0) {
      throw new Error(t("toast.redemptionBulkRevealSkipped", { count: failedCount }));
    }
    return { results, failedCount };
  }

  async function copySelectedRedemptionCodes() {
    setRedemptionBulkPending(true);
    try {
      const { results, failedCount } = await revealSelectedRedemptionCodes();
      if (results.length === 0) return;
      const copied = await copy(results.map((result) => result.code).join("\n"), {
        key: "selected-redemption-codes",
        copied: t("toast.redemptionBulkCopied", { count: results.length }),
        copiedDescription: failedCount > 0 ? t("toast.redemptionBulkRevealSkipped", { count: failedCount }) : undefined,
        failed: t("toast.redemptionBulkCopyFailed"),
      });
      if (!copied) {
        return;
      }
    } catch (error) {
      toast.error(t("toast.redemptionBulkCopyFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setRedemptionBulkPending(false);
    }
  }

  async function exportSelectedRedemptionCodes() {
    setRedemptionBulkPending(true);
    try {
      const { results, failedCount } = await revealSelectedRedemptionCodes();
      if (results.length === 0) return;
      downloadJSONFile(redemptionCodesExportFilename(), {
        exportedAt: new Date().toISOString(),
        total: results.length,
        results: results.map(({ item, code }) => ({
          id: item.id,
          code,
          codeHint: item.codeHint,
          mode: item.mode,
          rewardType: item.rewardType,
          creditUSD: item.creditUSD,
          planID: item.planID,
          durationDays: item.durationDays,
          maxRedemptions: item.maxRedemptions,
          perUserLimit: item.perUserLimit,
          redeemedCount: item.redeemedCount,
          remainingRedemptions: item.remainingRedemptions,
          status: item.status,
          expiresAt: item.expiresAt,
          description: item.description,
          createdAt: item.createdAt,
          updatedAt: item.updatedAt,
        })),
      });
      toast.success(t("toast.redemptionBulkExported", { count: results.length }), {
        description: failedCount > 0 ? t("toast.redemptionBulkRevealSkipped", { count: failedCount }) : undefined,
      });
    } catch (error) {
      toast.error(t("toast.redemptionBulkExportFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setRedemptionBulkPending(false);
    }
  }

  /** Resolves `true` on success so the caller can close its confirmation dialog. */
  async function applyRedemptionBulkStatus(status: "active" | "inactive"): Promise<boolean> {
    const ids = Array.from(selectedRedemptionIDs);
    if (ids.length === 0) return false;
    const previousRedemptionCodes = redemptionCodes;
    const idSet = new Set(ids);
    const updatedAt = new Date().toISOString();
    setRedemptionCodes((current) => current.map((item) => (
      idSet.has(item.id) ? { ...item, status, updatedAt } : item
    )));
    setRedemptionBulkPending(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        setRedemptionCodes(previousRedemptionCodes);
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return false;
      }
      const updatedCodes = (await runBulkActionInChunks({
        chunkSize: 10,
        items: ids,
        title: t("redemption.bulkPending"),
        runChunk: async (chunk) => {
          const codes: AdminRedemptionCodeDTO[] = [];
          for (const id of chunk) {
            const data = await updateAdminRedemptionCode(token, id, { status });
            codes.push(data.code);
          }
          return codes;
        },
      })).flat();
      setRedemptionCodes((current) => current.map((item) => updatedCodes.find((code) => code.id === item.id) ?? item));
      setSelectedRedemptionIDs(new Set());
      toast.success(status === "active" ? t("toast.redemptionBulkEnabled", { count: ids.length }) : t("toast.redemptionBulkDisabled", { count: ids.length }));
      void loadRedemptionCodes({}, { showLoading: false });
      return true;
    } catch (error) {
      setRedemptionCodes(previousRedemptionCodes);
      toast.error(t("toast.redemptionBulkFailed"), { description: resolveAdminErrorMessage(error) });
      return false;
    } finally {
      setRedemptionBulkPending(false);
    }
  }

  /** Resolves `true` on success so the caller can close its confirmation dialog. */
  async function deleteSelectedRedemptionCodes(): Promise<boolean> {
    const ids = Array.from(selectedRedemptionIDs);
    if (ids.length === 0) return false;
    const previousRedemptionCodes = redemptionCodes;
    const previousRedemptionTotal = redemptionTotal;
    const idSet = new Set(ids);
    const removedVisibleCount = redemptionCodes.filter((item) => idSet.has(item.id)).length;
    setRedemptionCodes((current) => current.filter((item) => !idSet.has(item.id)));
    setRedemptionTotal((current) => Math.max(0, current - removedVisibleCount));
    setRedemptionBulkPending(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        setRedemptionCodes(previousRedemptionCodes);
        setRedemptionTotal(previousRedemptionTotal);
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return false;
      }
      const result = mergeBatchResultData(await runBulkActionInChunks({
        items: ids,
        title: t("redemption.bulkDeleteTitle"),
        runChunk: (chunk) => batchDeleteAdminRedemptionCodes(token, { ids: chunk }),
      }));
      setSelectedRedemptionIDs(new Set());
      if (result.failedCount > 0) {
        toast.error(t("toast.redemptionDeletePartialFailed"), {
          description: t("toast.redemptionDeleteSummary", {
            successCount: result.successCount,
            notFoundCount: result.notFoundCount,
            failedCount: result.failedCount,
          }),
        });
      } else {
        toast.success(t("toast.redemptionDeleted", { count: result.successCount }), {
          description: result.notFoundCount > 0
            ? t("toast.redemptionDeleteSummary", {
              successCount: result.successCount,
              notFoundCount: result.notFoundCount,
              failedCount: result.failedCount,
            })
            : undefined,
        });
      }
      void loadRedemptionCodes({}, { showLoading: false });
      return true;
    } catch (error) {
      setRedemptionCodes(previousRedemptionCodes);
      setRedemptionTotal(previousRedemptionTotal);
      toast.error(t("toast.redemptionDeleteFailed"), { description: resolveAdminErrorMessage(error) });
      return false;
    } finally {
      setRedemptionBulkPending(false);
    }
  }

  /** Resolves `true` on success so the caller can close its confirmation dialog. */
  async function deleteSingleRedemptionCode(target: AdminRedemptionCodeDTO): Promise<boolean> {
    const previousRedemptionCodes = redemptionCodes;
    const previousRedemptionTotal = redemptionTotal;
    const removedVisibleCount = redemptionCodes.some((item) => item.id === target.id) ? 1 : 0;
    setRedemptionCodes((current) => current.filter((item) => item.id !== target.id));
    setRedemptionTotal((current) => Math.max(0, current - removedVisibleCount));
    setRedemptionBulkPending(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        setRedemptionCodes(previousRedemptionCodes);
        setRedemptionTotal(previousRedemptionTotal);
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return false;
      }
      await deleteAdminRedemptionCode(token, target.id);
      setSelectedRedemptionIDs((current) => {
        const next = new Set(current);
        next.delete(target.id);
        return next;
      });
      toast.success(t("toast.redemptionDeleted", { count: 1 }));
      void loadRedemptionCodes({}, { showLoading: false });
      return true;
    } catch (error) {
      setRedemptionCodes(previousRedemptionCodes);
      setRedemptionTotal(previousRedemptionTotal);
      toast.error(t("toast.redemptionDeleteFailed"), { description: resolveAdminErrorMessage(error) });
      return false;
    } finally {
      setRedemptionBulkPending(false);
    }
  }

  return {
    redemptionBulkPending,
    revealRedemptionCode,
    copySelectedRedemptionCodes,
    exportSelectedRedemptionCodes,
    applyRedemptionBulkStatus,
    deleteSelectedRedemptionCodes,
    deleteSingleRedemptionCode,
  };
}
