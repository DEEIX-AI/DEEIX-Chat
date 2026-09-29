import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  type ContentModerationConfig,
  getContentModerationConfig,
  probeContentModeration,
  updateContentModerationConfig,
} from "@/features/admin/api/content-moderation";
import type { ServiceRuntimeState } from "@/features/admin/components/shared/settings-runtime-panel";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export type ServiceDraft = {
  baseUrl: string;
  apiKey: string;
  model: string;
  timeoutSeconds: string;
  maxConcurrency: string;
  queueCapacity: string;
};

function createServiceDraft(config: ContentModerationConfig): ServiceDraft {
  return {
    baseUrl: config.baseUrl,
    apiKey: "",
    model: config.model,
    timeoutSeconds: String(config.timeoutSeconds),
    maxConcurrency: String(config.maxConcurrency),
    queueCapacity: String(config.queueCapacity),
  };
}

function parseIntegerDraft(value: string, min: number, max: number): number | null {
  const normalized = value.trim();
  if (!/^\d+$/.test(normalized)) return null;
  const parsed = Number(normalized);
  return Number.isSafeInteger(parsed) && parsed >= min && parsed <= max ? parsed : null;
}

function createPolicyPayload(config: ContentModerationConfig) {
  return {
    inputTextCategories: config.policy.inputTextCategories,
    inputImageCategories: config.policy.inputImageCategories,
    outputTextCategories: config.policy.outputTextCategories,
    outputImageCategories: config.policy.outputImageCategories,
  };
}

function hasSelectedPolicy(config: ContentModerationConfig): boolean {
  return Object.values(createPolicyPayload(config)).some((categories) => categories.length > 0);
}

/**
 * Owns the content-moderation settings flow: loading the config (superadmin
 * only), the service/policy drafts with their dirty flags, saving each section
 * and probing the moderation service.
 */
export function useAdminContentModeration() {
  const t = useTranslations("adminContentModeration");
  const { user } = useAuthSession();
  const isSuperAdmin = user?.role === "superadmin";
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [probing, setProbing] = React.useState(false);
  const [config, setConfig] = React.useState<ContentModerationConfig | null>(null);
  const [savedConfig, setSavedConfig] = React.useState<ContentModerationConfig | null>(null);
  const [serviceDraft, setServiceDraft] = React.useState<ServiceDraft | null>(null);
  const [textCategories, setTextCategories] = React.useState<string[]>([]);
  const [imageCategories, setImageCategories] = React.useState<string[]>([]);
  const [probeRuntime, setProbeRuntime] = React.useState<ServiceRuntimeState | null>(null);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      if (!isSuperAdmin) return;
      const cfgRes = await getContentModerationConfig(token);
      setConfig(cfgRes.config);
      setSavedConfig(cfgRes.config);
      setServiceDraft(createServiceDraft(cfgRes.config));
      setTextCategories(cfgRes.categories.text);
      setImageCategories(cfgRes.categories.image);
    } catch (error) {
      toast.error(t("loadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setLoading(false);
    }
  }, [isSuperAdmin, t]);

  React.useEffect(() => {
    void load();
  }, [load]);

  const serviceDirty = React.useMemo(
    () => Boolean(
      config &&
      savedConfig &&
      serviceDraft &&
      (config.enabled !== savedConfig.enabled ||
        serviceDraft.baseUrl !== savedConfig.baseUrl ||
        serviceDraft.model !== savedConfig.model ||
        serviceDraft.timeoutSeconds !== String(savedConfig.timeoutSeconds) ||
        serviceDraft.maxConcurrency !== String(savedConfig.maxConcurrency) ||
        serviceDraft.queueCapacity !== String(savedConfig.queueCapacity) ||
        serviceDraft.apiKey.trim()),
    ),
    [config, savedConfig, serviceDraft],
  );
  const policyDirty = React.useMemo(
    () => Boolean(config && savedConfig && JSON.stringify(config.policy) !== JSON.stringify(savedConfig.policy)),
    [config, savedConfig],
  );

  function updateServiceDraft(key: keyof ServiceDraft, value: string) {
    setServiceDraft((current) => (current ? { ...current, [key]: value } : current));
    setProbeRuntime(null);
  }

  const saveService = async () => {
    if (!config || !savedConfig || !serviceDraft || !isSuperAdmin) return;
    const enabling = config.enabled && !savedConfig.enabled;
    if (enabling && !hasSelectedPolicy(config)) {
      toast.error(t("saveFailed"), { description: t("validation.policyRequired") });
      return;
    }
    if (config.enabled && !config.hasAPIKey && !serviceDraft.apiKey.trim()) {
      toast.error(t("saveFailed"), { description: t("validation.apiKeyRequired") });
      return;
    }
    let timeoutSeconds: number | null = null;
    let maxConcurrency: number | null = null;
    let queueCapacity: number | null = null;
    if (config.enabled) {
      timeoutSeconds = parseIntegerDraft(serviceDraft.timeoutSeconds, 1, 60);
      maxConcurrency = parseIntegerDraft(serviceDraft.maxConcurrency, 1, 64);
      queueCapacity = parseIntegerDraft(serviceDraft.queueCapacity, 1, 4096);
      if (timeoutSeconds === null || maxConcurrency === null || queueCapacity === null) {
        const invalidNumericField = timeoutSeconds === null
          ? { key: "timeoutSeconds", min: 1, max: 60 }
          : maxConcurrency === null
            ? { key: "maxConcurrency", min: 1, max: 64 }
            : { key: "queueCapacity", min: 1, max: 4096 };
        toast.error(t("saveFailed"), {
          description: t("validation.integerRange", {
            field: t(`fields.${invalidNumericField.key}`),
            min: invalidNumericField.min,
            max: invalidNumericField.max,
          }),
        });
        return;
      }
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      const res = await updateContentModerationConfig(
        token,
        config.enabled
          ? {
              enabled: true,
              baseUrl: serviceDraft.baseUrl,
              model: serviceDraft.model,
              timeoutSeconds: timeoutSeconds ?? undefined,
              maxConcurrency: maxConcurrency ?? undefined,
              queueCapacity: queueCapacity ?? undefined,
              apiKey: serviceDraft.apiKey.trim() || undefined,
              policy: enabling ? createPolicyPayload(config) : undefined,
            }
          : { enabled: false },
      );
      setConfig({
        ...res.config,
        policy: enabling ? res.config.policy : config.policy,
      });
      setSavedConfig(res.config);
      setServiceDraft(createServiceDraft(res.config));
      setProbeRuntime(null);
      toast.success(t("saved"));
    } catch (error) {
      toast.error(t("saveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  };

  const savePolicy = async () => {
    if (!config || !savedConfig?.enabled || !isSuperAdmin) return;
    if (!hasSelectedPolicy(config)) {
      toast.error(t("saveFailed"), { description: t("validation.policyRequired") });
      return;
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      const res = await updateContentModerationConfig(token, {
        policy: createPolicyPayload(config),
      });
      setConfig((current) => (current ? { ...current, policy: res.config.policy } : res.config));
      setSavedConfig(res.config);
      setProbeRuntime(null);
      toast.success(t("saved"));
    } catch (error) {
      toast.error(t("saveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  };

  const probe = async () => {
    if (!isSuperAdmin) return;
    setProbing(true);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      const res = await probeContentModeration(token);
      const valid = res.text.valid && res.image.valid;
      setProbeRuntime({
        status: valid ? "available" : "unhealthy",
        reachable: valid,
        message: [res.text.error, res.image.error].filter(Boolean).join(" · ") || undefined,
        details: [
          {
            label: t("probeText"),
            value: `${res.text.valid ? t("valid") : t("invalid")} · ${res.text.latencyMS}ms`,
          },
          {
            label: t("probeImage"),
            value: `${res.image.valid ? t("valid") : t("invalid")} · ${res.image.latencyMS}ms`,
          },
        ],
      });
    } catch (error) {
      const message = resolveAdminErrorMessage(error);
      setProbeRuntime({ status: "failed", reachable: false, message });
      toast.error(t("probeFailed"), { description: message });
    } finally {
      setProbing(false);
    }
  };

  return {
    isSuperAdmin,
    loading,
    saving,
    probing,
    config,
    setConfig,
    savedConfig,
    serviceDraft,
    textCategories,
    imageCategories,
    probeRuntime,
    setProbeRuntime,
    serviceDirty,
    policyDirty,
    updateServiceDraft,
    saveService,
    savePolicy,
    probe,
  };
}
