import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { listAdminSettingsByNamespace, patchAdminSettings } from "@/features/admin/api";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { overrideFeaturePolicy } from "@/shared/hooks/use-feature-policy";

/**
 * Reads and toggles the global knowledge-base switch. `featureEnabled` stays
 * `null` until the setting loads (or if loading fails) so the toggle stays
 * disabled instead of offering an action based on a guessed state.
 */
export function useAdminKnowledgeBasesFeature() {
  const t = useTranslations("knowledgeBases");
  const [featureEnabled, setFeatureEnabled] = React.useState<boolean | null>(null);
  const [featureSaving, setFeatureSaving] = React.useState(false);

  React.useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) return;
        const settings = await listAdminSettingsByNamespace(token, "knowledgebase");
        if (cancelled) return;
        setFeatureEnabled((settings.find((item) => item.key === "enabled")?.value ?? "true") !== "false");
      } catch {
        // On read failure stay in the unknown state: the toggle remains disabled to prevent mistaken actions in an error state.
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const toggleFeature = React.useCallback(async (next: boolean) => {
    const previous = featureEnabled;
    setFeatureSaving(true);
    setFeatureEnabled(next);
    try {
      const token = await resolveAccessToken();
      if (!token) throw new Error("missing access token");
      await patchAdminSettings(token, {
        items: [{ namespace: "knowledgebase", key: "enabled", value: String(next) }],
      });
      overrideFeaturePolicy({ knowledgeBaseEnabled: next });
      toast.success(t(next ? "featureToggle.enabledToast" : "featureToggle.disabledToast"));
    } catch (error) {
      setFeatureEnabled(previous);
      toast.error(resolveAdminErrorMessage(error, t("featureToggle.saveFailed")));
    } finally {
      setFeatureSaving(false);
    }
  }, [featureEnabled, t]);

  return { featureEnabled, featureSaving, toggleFeature };
}
