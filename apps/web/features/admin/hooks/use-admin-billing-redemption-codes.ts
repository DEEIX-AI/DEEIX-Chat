"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  createAdminRedemptionCodes,
  listAdminRedemptionCodes,
  updateAdminRedemptionCode,
} from "@/features/admin/api";
import type { AdminBillingMode, AdminRedemptionCodeDTO } from "@/features/admin/api/billing-types";
import { DEFAULT_PAGE_SIZE } from "@/features/admin/model/billing-settings";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export type RedemptionFormState = {
  id?: number;
  code: string;
  quantity: string;
  mode: "usage" | "period";
  creditUSD: string;
  planID: string;
  durationDays: string;
  maxRedemptions: string;
  perUserLimit: string;
  expiresAt: string;
  description: string;
  status: "active" | "inactive";
};

export type RedemptionListOverrides = {
  page?: number;
  pageSize?: number;
  query?: string;
  mode?: string;
  status?: string;
  availability?: string;
};

export type RedemptionListOptions = { showLoading?: boolean; showError?: boolean };

type UseAdminBillingRedemptionCodesOptions = {
  billingMode: AdminBillingMode;
};

function parseOptionalPositiveInt(value: string): number | null | undefined {
  const text = value.trim();
  if (!text) return null;
  const parsed = Number(text);
  if (!Number.isInteger(parsed) || parsed <= 0) return undefined;
  return parsed;
}

function parseRequiredPositiveInt(value: string): number | undefined {
  const parsed = parseOptionalPositiveInt(value);
  return parsed && parsed > 0 ? parsed : undefined;
}

function isRedemptionCodeFormatValid(value: string): boolean {
  const text = value.trim();
  return !text || /^[A-Za-z0-9_-]{3,64}$/.test(text);
}

// Paged redemption-code list with its filters, selection, per-row status
// toggle and the create/edit form submit. Bulk actions live in
// useAdminBillingRedemptionBulk, which operates on this hook's state.
export function useAdminBillingRedemptionCodes({ billingMode }: UseAdminBillingRedemptionCodesOptions) {
  const t = useTranslations("adminBilling");
  const [redemptionCodes, setRedemptionCodes] = React.useState<AdminRedemptionCodeDTO[]>([]);
  const [redemptionLoading, setRedemptionLoading] = React.useState(false);
  const [redemptionQuery, setRedemptionQuery] = React.useState("");
  const [redemptionModeFilter, setRedemptionModeFilter] = React.useState("");
  const [redemptionStatusFilter, setRedemptionStatusFilter] = React.useState("");
  const [redemptionAvailabilityFilter, setRedemptionAvailabilityFilter] = React.useState("");
  const [redemptionPage, setRedemptionPage] = React.useState(1);
  const [redemptionPageSize, setRedemptionPageSize] = React.useState(DEFAULT_PAGE_SIZE);
  const [redemptionTotal, setRedemptionTotal] = React.useState(0);
  const [redemptionSaving, setRedemptionSaving] = React.useState(false);
  const [selectedRedemptionIDs, setSelectedRedemptionIDs] = React.useState<Set<number>>(new Set());
  const [createdRedemptionCodes, setCreatedRedemptionCodes] = React.useState<string[]>([]);
  const [redemptionStatusPendingID, setRedemptionStatusPendingID] = React.useState<number | null>(null);
  const redemptionVisibleIDs = React.useMemo(() => redemptionCodes.map((item) => item.id), [redemptionCodes]);

  const loadRedemptionCodes = React.useCallback(async (
    overrides: RedemptionListOverrides = {},
    options: RedemptionListOptions = {},
  ) => {
    const showLoading = options.showLoading ?? true;
    const showError = options.showError ?? showLoading;
    if (showLoading) {
      setRedemptionLoading(true);
    }
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const result = await listAdminRedemptionCodes(token, {
        page: overrides.page ?? redemptionPage,
        pageSize: overrides.pageSize ?? redemptionPageSize,
        query: overrides.query ?? redemptionQuery,
        mode: overrides.mode ?? redemptionModeFilter,
        status: overrides.status ?? redemptionStatusFilter,
        availability: overrides.availability ?? redemptionAvailabilityFilter,
      });
      setRedemptionCodes(result.results ?? []);
      setRedemptionTotal(result.total ?? 0);
    } catch (error) {
      if (showError) {
        toast.error(t("toast.redemptionLoadFailed"), { description: resolveAdminErrorMessage(error) });
      }
    } finally {
      if (showLoading) {
        setRedemptionLoading(false);
      }
    }
  }, [redemptionAvailabilityFilter, redemptionModeFilter, redemptionPage, redemptionPageSize, redemptionQuery, redemptionStatusFilter, t]);

  React.useEffect(() => {
    void loadRedemptionCodes();
  }, [loadRedemptionCodes]);

  // Availability depends on the billing mode, so a mode switch must refresh the "available" view.
  React.useEffect(() => {
    if (redemptionAvailabilityFilter === "available") {
      void loadRedemptionCodes({}, { showLoading: false });
    }
  }, [billingMode, loadRedemptionCodes, redemptionAvailabilityFilter]);

  React.useEffect(() => {
    const visibleSet = new Set(redemptionVisibleIDs);
    setSelectedRedemptionIDs((current) => {
      const next = new Set<number>();
      current.forEach((id) => {
        if (visibleSet.has(id)) next.add(id);
      });
      return next.size === current.size ? current : next;
    });
  }, [redemptionVisibleIDs]);

  async function setRedemptionCodeStatus(item: AdminRedemptionCodeDTO, checked: boolean) {
    const status = checked ? "active" : "inactive";
    if (item.status === status) return;
    const previousRedemptionCodes = redemptionCodes;
    const updatedAt = new Date().toISOString();
    setRedemptionCodes((current) => current.map((code) => (
      code.id === item.id ? { ...code, status, updatedAt } : code
    )));
    setRedemptionStatusPendingID(item.id);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        setRedemptionCodes(previousRedemptionCodes);
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const data = await updateAdminRedemptionCode(token, item.id, { status });
      setRedemptionCodes((current) => current.map((code) => code.id === data.code.id ? data.code : code));
      toast.success(status === "active" ? t("toast.redemptionEnabled") : t("toast.redemptionDisabled"));
      void loadRedemptionCodes({}, { showLoading: false });
    } catch (error) {
      setRedemptionCodes(previousRedemptionCodes);
      toast.error(t("toast.redemptionUpdateFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setRedemptionStatusPendingID(null);
    }
  }

  /**
   * Resolves `true` once the code is saved so the caller can close its form.
   * `expiresAt` is the form's date-time value already converted by the picker:
   * `null` for no expiry, `undefined` when unparseable.
   */
  async function saveRedemptionCode(redemptionForm: RedemptionFormState, expiresAt: string | null | undefined): Promise<boolean> {
    const maxRedemptions = parseOptionalPositiveInt(redemptionForm.maxRedemptions);
    const perUserLimit = parseRequiredPositiveInt(redemptionForm.perUserLimit);
    if (!isRedemptionCodeFormatValid(redemptionForm.code)) {
      toast.error(t("toast.redemptionInvalid"), { description: t("toast.redemptionInvalidCodeFormat") });
      return false;
    }
    if (maxRedemptions === undefined) {
      toast.error(t("toast.redemptionInvalid"), { description: t("toast.redemptionInvalidMaxRedemptions") });
      return false;
    }
    if (!perUserLimit) {
      toast.error(t("toast.redemptionInvalid"), { description: t("toast.redemptionInvalidPerUserLimit") });
      return false;
    }
    if (expiresAt === undefined) {
      toast.error(t("toast.redemptionInvalid"), { description: t("toast.redemptionInvalidExpiresAt") });
      return false;
    }
    if (expiresAt !== null && new Date(expiresAt).getTime() <= Date.now()) {
      toast.error(t("toast.redemptionInvalid"), { description: t("toast.redemptionExpiredAtPast") });
      return false;
    }
    if (maxRedemptions !== null && perUserLimit > maxRedemptions) {
      toast.error(t("toast.redemptionUserLimitExceedsTotal"));
      return false;
    }

    setRedemptionSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return false;
      }

      if (redemptionForm.id) {
        const data = await updateAdminRedemptionCode(token, redemptionForm.id, {
          status: redemptionForm.status,
          maxRedemptions,
          perUserLimit,
          expiresAt,
          description: redemptionForm.description.trim(),
        });
        setRedemptionCodes((current) => current.map((item) => item.id === data.code.id ? data.code : item));
        toast.success(t("toast.redemptionUpdated"));
        void loadRedemptionCodes({}, { showLoading: false });
        return true;
      }

      const quantity = parseRequiredPositiveInt(redemptionForm.quantity);
      if (!quantity) {
        toast.error(t("toast.redemptionInvalid"), { description: t("toast.redemptionInvalidQuantity") });
        return false;
      }
      if (redemptionForm.code.trim() && quantity !== 1) {
        toast.error(t("toast.redemptionManualQuantityInvalid"));
        return false;
      }
      const payload = {
        code: redemptionForm.code.trim() || undefined,
        quantity,
        mode: redemptionForm.mode,
        maxRedemptions: maxRedemptions ?? undefined,
        perUserLimit,
        expiresAt,
        description: redemptionForm.description.trim() || undefined,
      };

      const data = redemptionForm.mode === "usage"
        ? await (async () => {
          const creditUSD = Number(redemptionForm.creditUSD);
          if (!Number.isFinite(creditUSD) || creditUSD <= 0) {
            throw new Error(t("toast.redemptionInvalidCredit"));
          }
          return createAdminRedemptionCodes(token, {
            ...payload,
            creditUSD,
          });
        })()
        : await (async () => {
          const planID = parseRequiredPositiveInt(redemptionForm.planID);
          const durationDays = parseRequiredPositiveInt(redemptionForm.durationDays);
          if (!planID || !durationDays) {
            throw new Error(!planID ? t("toast.redemptionInvalidPlan") : t("toast.redemptionInvalidDuration"));
          }
          return createAdminRedemptionCodes(token, {
            ...payload,
            planID,
            durationDays,
          });
        })();
      const created = data.results ?? [];
      setRedemptionCodes((current) => [...created, ...current].slice(0, redemptionPageSize));
      setRedemptionTotal((current) => current + created.length);
      setCreatedRedemptionCodes(created.map((item) => item.code || "").filter(Boolean));
      toast.success(t("toast.redemptionCreated", { count: created.length }));
      void loadRedemptionCodes({}, { showLoading: false });
      return true;
    } catch (error) {
      toast.error(redemptionForm.id ? t("toast.redemptionUpdateFailed") : t("toast.redemptionCreateFailed"), { description: resolveAdminErrorMessage(error) });
      return false;
    } finally {
      setRedemptionSaving(false);
    }
  }

  return {
    redemptionCodes,
    setRedemptionCodes,
    redemptionLoading,
    redemptionQuery,
    setRedemptionQuery,
    redemptionModeFilter,
    setRedemptionModeFilter,
    redemptionStatusFilter,
    setRedemptionStatusFilter,
    redemptionAvailabilityFilter,
    setRedemptionAvailabilityFilter,
    redemptionPage,
    setRedemptionPage,
    redemptionPageSize,
    setRedemptionPageSize,
    redemptionTotal,
    setRedemptionTotal,
    redemptionSaving,
    selectedRedemptionIDs,
    setSelectedRedemptionIDs,
    redemptionVisibleIDs,
    createdRedemptionCodes,
    setCreatedRedemptionCodes,
    redemptionStatusPendingID,
    loadRedemptionCodes,
    setRedemptionCodeStatus,
    saveRedemptionCode,
  };
}
