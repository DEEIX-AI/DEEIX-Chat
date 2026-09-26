"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  listAdminLLMModelDisplayGroups,
  listAdminLLMModelVendors,
} from "@/features/admin/api";
import { listAllAdminPages } from "@/features/admin/api/shared";
import type {
  AdminLLMModelDisplayGroupDTO,
  AdminLLMModelVendorDTO,
} from "@/features/admin/api/llm.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";

// useAdminModelPresentation 加载模型技术厂商和可选展示分组目录。
export function useAdminModelPresentation() {
  const t = useTranslations("adminModels.presentation");
  const [vendors, setVendors] = React.useState<AdminLLMModelVendorDTO[]>([]);
  const [displayGroups, setDisplayGroups] = React.useState<AdminLLMModelDisplayGroupDTO[]>([]);
  const [loading, setLoading] = React.useState(true);

  const reload = React.useCallback(async () => {
    const token = await resolveAccessToken();
    if (!token) {
      setVendors([]);
      setDisplayGroups([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const [vendors, displayGroups] = await Promise.all([
        listAllAdminPages((options) => listAdminLLMModelVendors(token, options)),
        listAllAdminPages((options) => listAdminLLMModelDisplayGroups(token, options)),
      ]);
      setVendors(vendors);
      setDisplayGroups(displayGroups);
    } catch (error) {
      setVendors([]);
      setDisplayGroups([]);
      toast.error(t("loadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setLoading(false);
    }
  }, [t]);

  React.useEffect(() => {
    void reload();
  }, [reload]);

  return { vendors, displayGroups, loading, reload };
}
