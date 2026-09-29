import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { importOpenWebUIUsers } from "@/features/admin/api";
import type { ImportOpenWebUIUsersData, ImportOpenWebUIUsersRequest } from "@/features/admin/api/admin-types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

/**
 * OpenWebUI user import dialog flow. A dry run only shows the preview result;
 * a real import closes the dialog and reloads the first page of users.
 */
export function useAdminUsersOpenWebUIImport({
  page,
  setPage,
  onLoadUsers,
}: {
  page: number;
  setPage: (value: number) => void;
  onLoadUsers: () => Promise<void>;
}) {
  const t = useTranslations("adminUsers");
  const [openWebUIImportOpen, setOpenWebUIImportOpen] = React.useState(false);
  const [openWebUIImportPending, setOpenWebUIImportPending] = React.useState(false);
  const [openWebUIImportResult, setOpenWebUIImportResult] = React.useState<ImportOpenWebUIUsersData | null>(null);

  async function handleImportOpenWebUI(payload: ImportOpenWebUIUsersRequest) {
    if (openWebUIImportPending) {
      return;
    }
    setOpenWebUIImportPending(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      const result = await importOpenWebUIUsers(token, payload);
      setOpenWebUIImportResult(result);
      if (payload.dryRun) {
        toast.success(t("importOpenWebUI.toastPreviewSucceeded", {
          imported: result.imported,
          skipped: result.skippedExistingEmail + result.skippedDuplicateSourceEmail,
        }));
        return;
      }
      toast.success(t("importOpenWebUI.toastSucceeded", {
        imported: result.imported,
        skipped: result.skippedExistingEmail + result.skippedDuplicateSourceEmail,
      }));
      setOpenWebUIImportOpen(false);
      if (page === 1) {
        await onLoadUsers();
      } else {
        setPage(1);
      }
    } catch (error) {
      toast.error(t("importOpenWebUI.toastFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setOpenWebUIImportPending(false);
    }
  }

  return {
    openWebUIImportOpen,
    setOpenWebUIImportOpen,
    openWebUIImportPending,
    openWebUIImportResult,
    setOpenWebUIImportResult,
    handleImportOpenWebUI,
  };
}
