import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { createPermissionGroup } from "@/features/admin/api/permission-groups";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

/**
 * Creates a permission group. The rate multiplier is entered as a factor
 * ("1.5") and stored as an integer percent; invalid input falls back to 100%.
 */
export function useAdminGroupsCreate({
  onOpenChange,
  onCreated,
}: {
  onOpenChange: (open: boolean) => void;
  onCreated: () => Promise<void>;
}) {
  const t = useTranslations("adminGroups");
  const [saving, setSaving] = React.useState(false);

  const createGroup = React.useCallback(async ({
    name,
    description,
    rateMultiplier,
  }: {
    name: string;
    description: string;
    rateMultiplier: string;
  }) => {
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      const parsed = parseFloat(rateMultiplier);
      const rateMultiplierPercent =
        Number.isFinite(parsed) && parsed > 0 ? Math.round(parsed * 100) : 100;
      await createPermissionGroup(token, { name, description, rateMultiplierPercent });
      toast.success(t("created"));
      onOpenChange(false);
      await onCreated();
    } catch (error) {
      toast.error(resolveAdminErrorMessage(error, t("saveFailed")));
    } finally {
      setSaving(false);
    }
  }, [onCreated, onOpenChange, t]);

  return { saving, createGroup };
}
