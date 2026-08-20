import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { listAdminLLMSettings, updateAdminLLMSetting } from "@/features/admin/api";
import type { AdminLLMSetting } from "@/features/admin/api/llm.types";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

const CIRCUIT_BREAKER_DEFAULTS_KEY = "circuit_breaker.defaults";

type CircuitBreakerDefaults = Record<string, unknown> & {
  enabled?: boolean;
};

function parseCircuitBreakerDefaults(value: string): CircuitBreakerDefaults {
  try {
    const parsed: unknown = JSON.parse(value);
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      return parsed as CircuitBreakerDefaults;
    }
  } catch {
    // Invalid historical values are effectively disabled; the backend owns validation.
  }
  return {};
}

export function useAdminCircuitBreaker() {
  const t = useTranslations("adminModels.circuitBreaker");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [setting, setSetting] = React.useState<AdminLLMSetting | null>(null);
  const [defaults, setDefaults] = React.useState<CircuitBreakerDefaults>({});
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);

  React.useEffect(() => {
    let active = true;
    async function load() {
      try {
        const token = await resolveAccessToken();
        const settings = await listAdminLLMSettings(token);
        const item = settings.find((candidate) => candidate.key === CIRCUIT_BREAKER_DEFAULTS_KEY) ?? null;
        if (!active) return;
        setSetting(item);
        setDefaults(item ? parseCircuitBreakerDefaults(item.value) : {});
      } catch (error) {
        if (active) {
          toast.error(t("loadFailed"), { description: resolveErrorMessage(error) });
        }
      } finally {
        if (active) setLoading(false);
      }
    }
    void load();
    return () => {
      active = false;
    };
  }, [resolveErrorMessage, t]);

  const updateEnabled = React.useCallback(
    async (checked: boolean): Promise<boolean> => {
      if (!setting || saving) return false;
      setSaving(true);
      const nextDefaults = { ...defaults, enabled: checked };
      try {
        const token = await resolveAccessToken();
        const updated = await updateAdminLLMSetting(token, setting.key, JSON.stringify(nextDefaults));
        setSetting(updated);
        setDefaults(parseCircuitBreakerDefaults(updated.value));
        toast.success(checked ? t("enabledToast") : t("disabledToast"));
        return true;
      } catch (error) {
        toast.error(t("updateFailed"), { description: resolveErrorMessage(error) });
        return false;
      } finally {
        setSaving(false);
      }
    },
    [defaults, resolveErrorMessage, saving, setting, t],
  );

  return {
    available: setting !== null,
    enabled: defaults.enabled === true,
    loading,
    saving,
    updateEnabled,
  };
}
