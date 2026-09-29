"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  deleteAdminLLMModelIcon,
  listAdminLLMModelIcons,
  uploadAdminLLMModelIcon,
} from "@/features/admin/api";
import type { AdminLLMModelIconAssetListItem } from "@/features/admin/api/llm-types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { ApiError } from "@/shared/api/http-client";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { isRecord, readFiniteNumber } from "@/shared/lib/type-guards";

const MAX_ICON_BYTES = 1 << 20;
const ALLOWED_ICON_TYPES = new Set(["image/png", "image/jpeg", "image/webp"]);

function iconReferenceCount(error: unknown): number {
  if (!(error instanceof ApiError) || error.errorCode !== "llm.model_icon_asset_in_use" || !isRecord(error.details)) {
    return 0;
  }
  return readFiniteNumber(error.details, "referenceCount") ?? 0;
}

type UseAdminModelsIconsOptions = {
  pickerOpen: boolean;
  value: string;
  onChange: (value: string) => void;
  onUploadingChange?: (uploading: boolean) => void;
};

export function useAdminModelsIcons({ pickerOpen, value, onChange, onUploadingChange }: UseAdminModelsIconsOptions) {
  const t = useTranslations("adminModels.iconAsset");
  const mountedRef = React.useRef(true);
  const [uploading, setUploading] = React.useState(false);
  const [uploadedIcons, setUploadedIcons] = React.useState<AdminLLMModelIconAssetListItem[] | null>(null);
  const [uploadedIconsLoading, setUploadedIconsLoading] = React.useState(false);
  const [uploadedIconsLoadFailed, setUploadedIconsLoadFailed] = React.useState(false);
  const [uploadedIconsReloadKey, setUploadedIconsReloadKey] = React.useState(0);
  const [deleting, setDeleting] = React.useState(false);

  React.useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      onUploadingChange?.(false);
    };
  }, [onUploadingChange]);

  React.useEffect(() => {
    if (!pickerOpen) {
      return;
    }
    let canceled = false;
    setUploadedIcons(null);
    setUploadedIconsLoadFailed(false);
    setUploadedIconsLoading(true);
    void resolveAccessToken()
      .then(async (token) => {
        if (!token) {
          throw new Error("session expired");
        }
        return listAdminLLMModelIcons(token, { page: 1, pageSize: 100 });
      })
      .then((page) => {
        if (!canceled) {
          setUploadedIcons(page.results);
          setUploadedIconsLoadFailed(false);
        }
      })
      .catch(() => {
        if (!canceled) {
          setUploadedIcons([]);
          setUploadedIconsLoadFailed(true);
        }
      })
      .finally(() => {
        if (!canceled) {
          setUploadedIconsLoading(false);
        }
      });
    return () => {
      canceled = true;
    };
  }, [pickerOpen, uploadedIconsReloadKey]);

  const reloadUploadedIcons = React.useCallback(() => {
    setUploadedIconsReloadKey((current) => current + 1);
  }, []);

  const setUploadState = React.useCallback((next: boolean) => {
    if (!mountedRef.current) {
      return;
    }
    setUploading(next);
    onUploadingChange?.(next);
  }, [onUploadingChange]);

  /** `onUploaded` runs only while mounted, before the success toast. */
  const uploadIcon = React.useCallback(async (file: File, onUploaded: (ref: string) => void) => {
    if (file.type && !ALLOWED_ICON_TYPES.has(file.type)) {
      toast.error(t("invalidType"));
      return;
    }
    if (file.size <= 0 || file.size > MAX_ICON_BYTES) {
      toast.error(t("fileTooLarge"));
      return;
    }

    setUploadState(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("sessionExpired"));
        return;
      }
      const asset = await uploadAdminLLMModelIcon(token, file);
      if (mountedRef.current) {
        onUploaded(asset.ref);
        toast.success(asset.reused ? t("reused") : t("uploaded"));
      }
    } catch (error) {
      if (mountedRef.current) {
        toast.error(t("uploadFailed"), { description: resolveAdminErrorMessage(error) });
      }
    } finally {
      setUploadState(false);
    }
  }, [setUploadState, t]);

  /** `onDeleted` runs only while mounted, before the success toast. */
  const deleteIcon = React.useCallback(async (target: AdminLLMModelIconAssetListItem, onDeleted: () => void) => {
    setDeleting(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("sessionExpired"));
        return;
      }
      await deleteAdminLLMModelIcon(token, target.publicID);
      if (mountedRef.current) {
        setUploadedIcons((current) => current?.filter((item) => item.publicID !== target.publicID) ?? current);
        if (value.trim().toLowerCase() === target.ref.toLowerCase()) {
          onChange("");
        }
        onDeleted();
        toast.success(t("removed"));
      }
    } catch (error) {
      if (mountedRef.current) {
        const referenceCount = iconReferenceCount(error);
        if (referenceCount > 0) {
          toast.error(t("removeInUse", { count: referenceCount }));
        } else {
          toast.error(t("removeFailed"), { description: resolveAdminErrorMessage(error) });
        }
      }
    } finally {
      if (mountedRef.current) {
        setDeleting(false);
      }
    }
  }, [onChange, t, value]);

  return {
    uploading,
    uploadedIcons,
    uploadedIconsLoading,
    uploadedIconsLoadFailed,
    reloadUploadedIcons,
    deleting,
    uploadIcon,
    deleteIcon,
  };
}
