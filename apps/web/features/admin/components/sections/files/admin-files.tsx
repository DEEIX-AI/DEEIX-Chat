"use client";

import { Save } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useTranslations } from "next-intl";
import * as React from "react";
import { Button } from "@/components/ui/button";
import { useAdminFilesEmbeddingStatus } from "@/features/admin/hooks/use-admin-files-embedding-status";
import { useAdminFilesSettings } from "@/features/admin/hooks/use-admin-files-settings";
import {
  EMBEDDING_MODES,
  EXTRACT_ENGINE_POLICIES,
  isServiceDirty,
  mergeAllowedMIMETypes,
  resolveFieldID,
  resolveMinerUFileTypeFormats,
  resolveMissingMinerUMIMETypes,
  resolveVisibleFieldBlocks,
  resolveVisibleFields,
  SETTINGS_GROUPS,
  type ServiceName,
  type SettingsField,
  type SettingsGroup,
} from "@/features/admin/model/files-settings";
import {
  SettingsFieldInset,
  SettingsFieldItem,
  SettingsFieldList,
  SettingsPage,
  SettingsSection,
  SettingsSectionSeparator,
} from "@/shared/components/settings-layout";
import {
  type ServiceRuntimeActionName,
  SettingsFieldEditor,
  type SettingsFieldServiceRuntime,
} from "../../shared/settings-runtime-panel";
import { FilesEmbeddingStatusPanel } from "./files-embedding-status";

function resolveMinerUFileTypeOptionMeta(value: string, label: string, settingsMap: Record<string, string>) {
  const meta = resolveMinerUFileTypeFormats(value, settingsMap["extract.mineru_source"] ?? "").join("/");
  return meta && meta !== label ? meta : undefined;
}

function toEditorField(field: SettingsField, translate: (key: string) => string, settingsMap: Record<string, string>) {
  const fieldKey = `fields.${field.namespace}.${field.key}`;
  const fieldID = resolveFieldID(field);
  return {
    id: fieldID,
    label: translate(`${fieldKey}.label`),
    description: translate(`${fieldKey}.description`),
    type: field.type,
    placeholder: field.placeholder ? translate(`${fieldKey}.placeholder`) : undefined,
    valueUnit: field.valueUnit,
    options: field.options?.map((option) => {
      const label = translate(`${fieldKey}.options.${option.value}`);
      const meta =
        fieldID === "extract.mineru_file_types"
          ? resolveMinerUFileTypeOptionMeta(option.value, label, settingsMap)
          : undefined;
      return {
        ...option,
        label,
        meta,
      };
    }),
  } as const;
}

export function AdminFilesPage() {
  const t = useTranslations("adminFiles");
  const tActions = useTranslations("common.actions");
  const {
    embeddingStatus,
    embeddingStatusLoading,
    reindexing,
    loadEmbeddingStatus,
    refreshEmbeddingStatus,
    clearEmbeddingStatus,
    handleReindex,
  } = useAdminFilesEmbeddingStatus();
  const {
    loading,
    saving,
    settingsMap,
    savedMap,
    configuredMap,
    serviceStates,
    dirtyFieldIDs,
    handleServiceAction,
    handleFieldChange,
    handleSaveAllowedMIMETypes,
    handleSaveGroup,
  } = useAdminFilesSettings({ loadEmbeddingStatus, clearEmbeddingStatus });

  const resolveServiceRuntime = React.useCallback(
    (name: ServiceName): SettingsFieldServiceRuntime => {
      const state = serviceStates[name];
      const settingsDirty = isServiceDirty(name, settingsMap, savedMap);
      return {
        runtime: state.data
          ? {
              status: state.data.status,
              reachable: state.data.reachable,
              message: state.data.message,
              details: [{ label: t("runtime.address"), value: state.data.baseURL }],
            }
          : null,
        loading: state.loading || state.action === "test",
        actionDisabled: settingsDirty || loading || saving || state.loading || state.action === "test",
        pendingAction: state.action,
        actions: [{ key: "test", label: t("runtime.testConnection"), icon: "bugplay", action: "test", spinWhen: "test" }],
        onAction: (action: ServiceRuntimeActionName) => {
          if (action === "test") void handleServiceAction(action, name);
        },
      };
    },
    [handleServiceAction, loading, saving, serviceStates, settingsMap, savedMap, t],
  );

  const requestSaveGroup = React.useCallback((group: SettingsGroup) => {
    void handleSaveGroup(group);
  }, [handleSaveGroup]);

  const minerUMIMEHint = React.useMemo(() => {
    if ((settingsMap["extract.engine"] ?? "") !== EXTRACT_ENGINE_POLICIES.MINERU) {
      return null;
    }
    const missing = resolveMissingMinerUMIMETypes(settingsMap);
    if (missing.length === 0) {
      return null;
    }
    const labels = missing.map((item) => item.format).join(", ");
    const nextAllowlist = mergeAllowedMIMETypes(settingsMap["file.allowed_mime_types"] ?? "", missing);
    return (
      <p className="min-w-0 text-[11px] leading-5 text-muted-foreground">
        {t("mineruMimeHint.missing", { formats: labels })}
        <button
          type="button"
          className="ml-2 inline-flex text-foreground/70 underline underline-offset-4 transition-colors hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50"
          disabled={loading || saving}
          onClick={() => void handleSaveAllowedMIMETypes(nextAllowlist)}
        >
          {t("mineruMimeHint.addAndSave")}
        </button>
      </p>
    );
  }, [handleSaveAllowedMIMETypes, loading, saving, settingsMap, t]);

  const embeddingEnabled = settingsMap["file.embedding_enabled"] === EMBEDDING_MODES.ON;

  return (
    <SettingsPage>
      {SETTINGS_GROUPS.map((group, index) => {
        const visibleFields = resolveVisibleFields(group, settingsMap);
        const visibleBlocks = resolveVisibleFieldBlocks(group, settingsMap);
        return (
          <React.Fragment key={group.title}>
            {visibleFields.length > 0 && (
              <SettingsSection
                title={t(`groups.${group.key}.title`)}
                actions={
                  visibleFields.some((field) => dirtyFieldIDs.has(resolveFieldID(field))) ? (
                    <Button type="button" size="sm" disabled={loading || saving} onClick={() => requestSaveGroup(group)}>
                      <Save className="size-3.5" />
                      {tActions("save")}
                    </Button>
                  ) : null
                }
              >

                <SettingsFieldList>
                  <AnimatePresence initial={false}>
                    {visibleBlocks.map((block, blockIndex) => {
                      if (block.kind === "field") {
                        const fieldID = resolveFieldID(block.field);
                        return (
                          <SettingsFieldItem key={fieldID} index={blockIndex}>
                            <SettingsFieldEditor
                              field={{
                                ...toEditorField(block.field, t, settingsMap),
                                ...(block.field.runtimeService
                                  ? { serviceRuntime: resolveServiceRuntime(block.field.runtimeService) }
                                  : {}),
                              }}
                              value={settingsMap[fieldID] ?? ""}
                              configured={configuredMap[fieldID]}
                              dirty={(settingsMap[fieldID] ?? "") !== (savedMap[fieldID] ?? "")}
                              disabled={loading || saving}
                              afterControl={fieldID === "extract.mineru_file_types" ? minerUMIMEHint : undefined}
                              animateLayout={fieldID !== "extract.mineru_file_types"}
                              onChange={(value) => handleFieldChange(fieldID, value)}
                            />
                          </SettingsFieldItem>
                        );
                      }

                      return (
                        <motion.div
                          key={block.key}
                          className="min-w-0"
                          initial={{ opacity: 0, gridTemplateRows: "0fr" }}
                          animate={{ opacity: 1, gridTemplateRows: "1fr" }}
                          exit={{ opacity: 0, gridTemplateRows: "0fr" }}
                          transition={{ duration: 0.2, ease: [0.22, 1, 0.36, 1] }}
                          style={{ display: "grid" }}
                        >
                          <div className="-mx-px overflow-hidden px-px pb-px">
                            <SettingsFieldItem index={blockIndex}>
                              <SettingsFieldInset>
                                <SettingsFieldList className="gap-3 md:gap-4">
                                  {block.fields.map((field) => {
                                    const fieldID = resolveFieldID(field);
                                    return (
                                      <SettingsFieldEditor
                                        key={fieldID}
                                        field={{
                                          ...toEditorField(field, t, settingsMap),
                                          ...(field.runtimeService
                                            ? { serviceRuntime: resolveServiceRuntime(field.runtimeService) }
                                            : {}),
                                        }}
                                        value={settingsMap[fieldID] ?? ""}
                                        configured={configuredMap[fieldID]}
                                        dirty={(settingsMap[fieldID] ?? "") !== (savedMap[fieldID] ?? "")}
                                        disabled={loading || saving}
                                        afterControl={fieldID === "extract.mineru_file_types" ? minerUMIMEHint : undefined}
                                        animateLayout={fieldID !== "extract.mineru_file_types"}
                                        onChange={(value) => handleFieldChange(fieldID, value)}
                                      />
                                    );
                                  })}
                                </SettingsFieldList>
                              </SettingsFieldInset>
                            </SettingsFieldItem>
                          </div>
                        </motion.div>
                      );
                    })}
                  </AnimatePresence>
                </SettingsFieldList>

                {group.key === "embedding" && embeddingEnabled && (
                  <FilesEmbeddingStatusPanel
                    status={embeddingStatus}
                    loading={embeddingStatusLoading}
                    reindexing={reindexing}
                    disabled={loading || saving}
                    onReindex={(includeEmpty) => void handleReindex(includeEmpty)}
                    onStatusRefresh={() => void refreshEmbeddingStatus()}
                  />
                )}
              </SettingsSection>
            )}
            {index < SETTINGS_GROUPS.length - 1 ? <SettingsSectionSeparator /> : null}
          </React.Fragment>
        );
      })}
    </SettingsPage>
  );
}
