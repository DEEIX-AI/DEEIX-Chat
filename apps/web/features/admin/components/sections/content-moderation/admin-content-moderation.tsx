"use client";

import { Save } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { SpinnerLabel } from "@/components/ui/spinner";
import { type ServiceDraft, useAdminContentModeration } from "@/features/admin/hooks/use-admin-content-moderation";
import { cn } from "@/lib/utils";
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
} from "../../shared/settings-runtime-panel";
import { ModerationCategorySelector } from "./content-moderation-category-selector";

export function AdminContentModerationPage() {
  const t = useTranslations("adminContentModeration");
  const {
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
  } = useAdminContentModeration();

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

  if (!config || !savedConfig || !serviceDraft) {
    return (
      <SettingsPage>
        <SettingsSection title={t("title")}>
          <p className="text-sm text-muted-foreground">{t("loadFailed")}</p>
        </SettingsSection>
      </SettingsPage>
    );
  }

  const renderSaveAction = (visible: boolean, onSave: () => Promise<void>) =>
    visible ? (
      <Button type="button" size="sm" disabled={saving || probing} onClick={() => void onSave()}>
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
  const serviceSaveAction = renderSaveAction(serviceDirty, saveService);
  const policySaveAction = renderSaveAction(savedConfig.enabled && config.enabled && policyDirty, savePolicy);
  const serviceFields: ReadonlyArray<{
    key: keyof ServiceDraft;
    type: "string" | "password" | "int";
    value: string;
    savedValue: string;
    placeholder?: string;
  }> = [
    {
      key: "baseUrl",
      type: "string",
      value: serviceDraft.baseUrl,
      savedValue: savedConfig.baseUrl,
    },
    {
      key: "apiKey",
      type: "password",
      value: serviceDraft.apiKey,
      savedValue: "",
      placeholder: t("fields.apiKeyPlaceholder"),
    },
    {
      key: "model",
      type: "string",
      value: serviceDraft.model,
      savedValue: savedConfig.model,
    },
    {
      key: "timeoutSeconds",
      type: "int",
      value: serviceDraft.timeoutSeconds,
      savedValue: String(savedConfig.timeoutSeconds),
    },
    {
      key: "maxConcurrency",
      type: "int",
      value: serviceDraft.maxConcurrency,
      savedValue: String(savedConfig.maxConcurrency),
    },
    {
      key: "queueCapacity",
      type: "int",
      value: serviceDraft.queueCapacity,
      savedValue: String(savedConfig.queueCapacity),
    },
  ];
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
                                ? Boolean(serviceDraft.apiKey.trim())
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
