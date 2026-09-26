"use client";

import { useTranslations } from "next-intl";
import * as React from "react";

import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { createAdminUIComponent, deleteAdminUIComponent, listAdminUIComponents, updateAdminUIComponent } from "@/shared/api/ui-components";
import { type UIComponentLibraryAPI, useUIComponentLibrary } from "@/shared/hooks/use-ui-component-library";

const ADMIN_API: UIComponentLibraryAPI = {
  list: (token, options, signal) => listAdminUIComponents(token, options, signal),
  create: createAdminUIComponent,
  update: updateAdminUIComponent,
  remove: deleteAdminUIComponent,
};

export function useAdminUIComponents() {
  const t = useTranslations("uiComponents.toast");
  const messages = React.useMemo(
    () => ({
      loadFailed: t("loadFailed"),
      invalid: t("invalid"),
      invalidSchema: t("invalidSchema"),
      tooLong: t("tooLong"),
      created: t("created"),
      updated: t("updated"),
      deleted: t("deleted"),
      createFailed: t("createFailed"),
      updateFailed: t("updateFailed"),
      deleteFailed: t("deleteFailed"),
    }),
    [t],
  );
  const resolveError = React.useCallback((error: unknown) => resolveAdminErrorMessage(error), []);
  return useUIComponentLibrary(ADMIN_API, messages, resolveError);
}
