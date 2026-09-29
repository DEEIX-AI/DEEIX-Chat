import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { exportAllConversations } from "@/features/admin/api";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { downloadBlob, readExportManifest } from "@/shared/lib/export-download";

/**
 * Exports every conversation as an archive. The manifest inside the archive
 * decides whether the export is reported as complete or partial.
 */
export function useAdminConversationExport() {
  const t = useTranslations("adminConversation");
  const [exporting, setExporting] = React.useState(false);


  const handleExportConversations = React.useCallback(async () => {
    setExporting(true);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      const { blob, fileName } = await exportAllConversations(token);
      const manifest = await readExportManifest(blob);
      downloadBlob(blob, fileName);
      if (manifest && (!manifest.complete || (manifest.failed ?? 0) > 0)) {
        toast.warning(t("dataExport.partial", { exported: manifest.exported ?? 0, failed: manifest.failed ?? 0 }));
      } else if (manifest) {
        toast.success(t("dataExport.success", { count: manifest.exported ?? 0 }));
      }
    } catch {
      toast.error(t("dataExport.failed"));
    } finally {
      setExporting(false);
    }
  }, [t]);

  return { exporting, handleExportConversations };
}
