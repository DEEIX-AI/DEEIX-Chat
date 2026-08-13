"use client";

import { Save } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { SpinnerLabel } from "@/components/ui/spinner";
import {
  type ContentModerationConfig,
  getContentModerationConfig,
  probeContentModeration,
  updateContentModerationConfig,
} from "@/features/admin/api/content-moderation";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { cn } from "@/lib/utils";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  SettingsFieldInset,
  SettingsFieldItem,
  SettingsFieldList,
  SettingsFieldRow,
  SettingsPage,
  SettingsSection,
  SettingsSectionSeparator,
} from "@/shared/components/settings-layout";

import {
  type ServiceRuntimeState,
  SettingsFieldEditor,
  type SettingsFieldServiceRuntime,
} from "../shared/settings-runtime-panel";
import { ModerationCategorySelector } from "./moderation-category-selector";

type ServiceConfigField = "baseUrl" | "model" | "timeoutSeconds" | "maxConcurrency" | "queueCapacity";
type ServiceDraftField = ServiceConfigField | "apiKey";

export function AdminContentModeration() {
  const t = useTranslations("adminContentModeration");
  const { user } = useAuthSession();
  const isSuperAdmin = user?.role === "superadmin";
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [probing, setProbing] = React.useState(false);
  const [config, setConfig] = React.useState<ContentModerationConfig | null>(null);
  const [savedConfig, setSavedConfig] = React.useState<ContentModerationConfig | null>(null);
  const [textCategories, setTextCategories] = React.useState<string[]>([]);
  const [imageCategories, setImageCategories] = React.useState<string[]>([]);
  const [apiKeyDraft, setApiKeyDraft] = React.useState("");
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
      (config.enabled !== savedConfig.enabled ||
        config.baseUrl !== savedConfig.baseUrl ||
        config.model !== savedConfig.model ||
        config.timeoutSeconds !== savedConfig.timeoutSeconds ||
        config.maxConcurrency !== savedConfig.maxConcurrency ||
        config.queueCapacity !== savedConfig.queueCapacity ||
        apiKeyDraft.trim()),
    ),
    [apiKeyDraft, config, savedConfig],
  );
  const policyDirty = React.useMemo(
    () => Boolean(config && savedConfig && JSON.stringify(config.policy) !== JSON.stringify(savedConfig.policy)),
    [config, savedConfig],
  );

  function updateServiceField<Key extends ServiceConfigField>(
    key: Key,
    value: ContentModerationConfig[Key],
  ) {
    setConfig((current) => (current ? { ...current, [key]: value } : current));
    setProbeRuntime(null);
  }

  function updateServiceDraft(key: ServiceDraftField, value: string) {
    switch (key) {
      case "apiKey":
        setApiKeyDraft(value);
        setProbeRuntime(null);
        break;
      case "baseUrl":
      case "model":
        updateServiceField(key, value);
        break;
      case "timeoutSeconds":
        updateServiceField(key, Number(value) || 10);
        break;
      case "maxConcurrency":
        updateServiceField(key, Number(value) || 4);
        break;
      case "queueCapacity":
        updateServiceField(key, Number(value) || 256);
        break;
    }
  }

  const save = async () => {
    if (!config || !isSuperAdmin) return;
    const hasSelectedPolicy = [
      config.policy.inputTextCategories,
      config.policy.inputImageCategories,
      config.policy.outputTextCategories,
      config.policy.outputImageCategories,
    ].some((categories) => categories.length > 0);
    if (config.enabled && !hasSelectedPolicy) {
      toast.error(t("saveFailed"), { description: t("validation.policyRequired") });
      return;
    }
    if (config.enabled && !config.hasAPIKey && !apiKeyDraft.trim()) {
      toast.error(t("saveFailed"), { description: t("validation.apiKeyRequired") });
      return;
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      const res = await updateContentModerationConfig(token, {
        enabled: config.enabled,
        baseUrl: config.baseUrl,
        model: config.model,
        timeoutSeconds: config.timeoutSeconds,
        maxConcurrency: config.maxConcurrency,
        queueCapacity: config.queueCapacity,
        apiKey: apiKeyDraft.trim() || undefined,
        policy: {
          inputTextCategories: config.policy.inputTextCategories,
          inputImageCategories: config.policy.inputImageCategories,
          outputTextCategories: config.policy.outputTextCategories,
          outputImageCategories: config.policy.outputImageCategories,
        },
      });
      setConfig(res.config);
      setSavedConfig(res.config);
      setApiKeyDraft("");
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

  if (loading) {
    return (
      <SettingsPage>
        <SettingsSection title={t("title")}>
          <p className="text-sm text-muted-foreground">{t("loading")}</p>
        </SettingsSection>
      </SettingsPage>
    );
  }

  if (!isSuperAdmin) {
    return (
      <SettingsPage>
        <SettingsSection title={t("title")}>
          <p className="text-sm text-muted-foreground">{t("superAdminOnly")}</p>
        </SettingsSection>
      </SettingsPage>
    );
  }

  if (!config || !savedConfig) {
    return (
      <SettingsPage>
        <SettingsSection title={t("title")}>
          <p className="text-sm text-muted-foreground">{t("loadFailed")}</p>
        </SettingsSection>
      </SettingsPage>
    );
  }

  const renderSaveAction = (visible: boolean) =>
    visible ? (
      <Button type="button" size="sm" disabled={saving || probing} onClick={() => void save()}>
        {saving ? (
          <SpinnerLabel>{t("saving")}</SpinnerLabel>
        ) : (
          <>
            <Save className="size-3.5" />
            {t("save")}
          </>
        )}
      </Button>
    ) : null;
  const serviceSaveAction = renderSaveAction(serviceDirty);
  const policySaveAction = renderSaveAction(config.enabled && policyDirty);
  const serviceFields = [
    {
      key: "baseUrl",
      type: "string",
      value: config.baseUrl,
      savedValue: savedConfig.baseUrl,
      placeholder: undefined,
    },
    {
      key: "apiKey",
      type: "password",
      value: apiKeyDraft,
      savedValue: "",
      placeholder: t("fields.apiKeyPlaceholder"),
    },
    {
      key: "model",
      type: "string",
      value: config.model,
      savedValue: savedConfig.model,
      placeholder: undefined,
    },
    {
      key: "timeoutSeconds",
      type: "int",
      value: String(config.timeoutSeconds),
      savedValue: String(savedConfig.timeoutSeconds),
      placeholder: undefined,
    },
    {
      key: "maxConcurrency",
      type: "int",
      value: String(config.maxConcurrency),
      savedValue: String(savedConfig.maxConcurrency),
      placeholder: undefined,
    },
    {
      key: "queueCapacity",
      type: "int",
      value: String(config.queueCapacity),
      savedValue: String(savedConfig.queueCapacity),
      placeholder: undefined,
    },
  ] as const;
  const serviceConfigured = savedConfig.enabled && savedConfig.hasAPIKey;
  const serviceRuntimeState: ServiceRuntimeState | null = serviceConfigured
    ? probeRuntime
    : { status: "unconfigured", reachable: false };
  const serviceRuntimeField: SettingsFieldServiceRuntime = {
    runtime: serviceRuntimeState,
    loading: probing,
    actionDisabled: saving || probing || serviceDirty || !serviceConfigured,
    pendingAction: probing ? "test" : "",
    actions: [
      {
        key: "test",
        label: t("probe"),
        icon: "bugplay",
        action: "test",
        spinWhen: "test",
      },
    ],
    onAction: (action) => {
      if (action === "test") void probe();
    },
  };

  return (
    <SettingsPage>
      <SettingsSection title={t("sections.service")} actions={serviceSaveAction}>
        <SettingsFieldList>
          <SettingsFieldItem>
            <SettingsFieldEditor
              field={{
                id: "content-moderation-enabled",
                label: t("fields.enabled"),
                description: t("fieldDescriptions.enabled"),
                type: "bool",
              }}
              value={config.enabled ? "true" : "false"}
              dirty={config.enabled !== savedConfig.enabled}
              disabled={saving || probing}
              onChange={(value) => {
                setConfig((current) => (current ? { ...current, enabled: value === "true" } : current));
                setProbeRuntime(null);
              }}
            />
          </SettingsFieldItem>
          <AnimatePresence initial={false}>
            {config.enabled ? (
              <motion.div
                key="content-moderation-settings"
                initial={{ opacity: 0, gridTemplateRows: "0fr" }}
                animate={{ opacity: 1, gridTemplateRows: "1fr" }}
                exit={{ opacity: 0, gridTemplateRows: "0fr" }}
                transition={{ duration: 0.2, ease: [0.22, 1, 0.36, 1] }}
                style={{ display: "grid" }}
              >
                <div className="overflow-hidden">
                  <SettingsFieldItem index={1}>
                    <SettingsFieldInset>
                      <SettingsFieldList className="gap-3 md:gap-4">
                        {serviceFields.map((field) => (
                          <SettingsFieldEditor
                            key={field.key}
                            field={{
                              id: `content-moderation-${field.key}`,
                              label: t(`fields.${field.key}`),
                              description:
                                field.key === "apiKey" && !config.hasAPIKey
                                  ? t("fieldDescriptions.apiKeyRequired")
                                  : t(`fieldDescriptions.${field.key}`),
                              type: field.type,
                              placeholder: field.placeholder,
                              ...(field.key === "baseUrl" ? { serviceRuntime: serviceRuntimeField } : {}),
                            }}
                            value={field.value}
                            configured={field.key === "apiKey" ? config.hasAPIKey : undefined}
                            dirty={
                              field.key === "apiKey"
                                ? Boolean(apiKeyDraft.trim())
                                : field.value !== field.savedValue
                            }
                            disabled={saving || probing}
                            onChange={(value) => updateServiceDraft(field.key, value)}
                          />
                        ))}
                      </SettingsFieldList>
                    </SettingsFieldInset>
                  </SettingsFieldItem>
                </div>
              </motion.div>
            ) : null}
          </AnimatePresence>
        </SettingsFieldList>
      </SettingsSection>

      <SettingsSectionSeparator />

      <SettingsSection title={t("sections.policy")} actions={policySaveAction}>
        <SettingsFieldList
          className={cn("transition-opacity", !config.enabled && "opacity-50")}
        >
          {(
            [
              ["inputText", "inputTextCategories", textCategories],
              ["inputImage", "inputImageCategories", imageCategories],
              ["outputText", "outputTextCategories", textCategories],
              ["outputImage", "outputImageCategories", imageCategories],
            ] as const
          ).map(([labelKey, field, options], index) => {
            const selectedValue = config.policy[field] ?? [];
            const selectedCount = options.filter((category) =>
              selectedValue.includes(category),
            ).length;
            return (
              <SettingsFieldItem key={field} index={index}>
                <SettingsFieldRow
                  title={t(`surfaces.${labelKey}`)}
                  description={t(`surfaceDescriptions.${labelKey}`)}
                >
                  <ModerationCategorySelector
                    options={options}
                    value={selectedValue}
                    selectAllLabel={t("selectAll")}
                    emptyLabel={t("noneEnabled")}
                    selectedLabel={t("enabledCount", {
                      selected: selectedCount,
                      total: options.length,
                    })}
                    disabled={saving || probing || !config.enabled}
                    disabledHint={!config.enabled ? t("policyRequiresEnabled") : undefined}
                    onChange={(next) =>
                      setConfig((current) =>
                        current
                          ? {
                              ...current,
                              policy: { ...current.policy, [field]: next },
                            }
                          : current,
                      )
                    }
                  />
                </SettingsFieldRow>
              </SettingsFieldItem>
            );
          })}
        </SettingsFieldList>
      </SettingsSection>
    </SettingsPage>
  );
}
