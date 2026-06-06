"use client";

import * as React from "react";
import { Save } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { TaskModelField, type ModelOption } from "../shared/model-field";
import {
  SettingsFieldEditor,
  type ServiceRuntimeActionName,
  type SettingsFieldServiceRuntime,
} from "../shared/settings-runtime-panel";
import { Button } from "@/components/ui/button";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  SettingsFieldInset,
  SettingsFieldItem,
  SettingsFieldList,
  SettingsPage,
  SettingsSection,
  SettingsSectionSeparator,
} from "@/shared/components/settings-layout";
import {
  applySettingsDefaults,
  denormalizeMBValue,
  EMBEDDING_MODES,
  EXTRACT_ENGINE_POLICIES,
  flattenSettings,
  INITIAL_SERVICE_STATES,
  isEmbeddingServiceConfigured,
  isServiceDirty,
  isSettingsValueField,
  OCR_ENGINES,
  resolveActiveServices,
  resolveErrorMessage,
  resolveFieldID,
  resolveMinerUSource,
  resolveOCREngine,
  resolveVisibleFieldBlocks,
  resolveVisibleFields,
  SERVICE_LABELS,
  SERVICE_NAMES,
  SETTINGS_GROUPS,
  TASK_MODEL_FOLLOW,
  TIKA_SERVICE_SOURCES,
  usesTika,
  type ServiceName,
  type ServiceRuntimeData,
  type ServiceState,
  type SettingsField,
  type SettingsGroup,
} from "@/features/admin/model/chat-files";
import {
  type AdminEmbeddingIndexStatus,
  getAdminDoclingRuntime,
  getAdminEmbeddingRuntime,
  getAdminEmbeddingStatus,
  getAdminReferenceData,
  getAdminMinerURuntime,
  getAdminRapidOCRRuntime,
  getAdminTesseractRuntime,
  getAdminTikaRuntime,
  listAdminSettings,
  patchAdminSettings,
  triggerAdminEmbeddingReindex,
} from "@/features/admin/api";
import { buildTaskModelOptions } from "@/features/admin/model/task-model-options";
import { cn } from "@/lib/utils";
import type { PatchSettingItem } from "@/shared/api/settings.types";
import { configuredSettingsMap, settingHasValue } from "@/shared/lib/settings-meta";

const SERVICE_LOADERS: Record<ServiceName, (token: string) => Promise<ServiceRuntimeData>> = {
  tika: getAdminTikaRuntime as (token: string) => Promise<ServiceRuntimeData>,
  docling: getAdminDoclingRuntime as (token: string) => Promise<ServiceRuntimeData>,
  mineru: getAdminMinerURuntime as (token: string) => Promise<ServiceRuntimeData>,
  tesseract: getAdminTesseractRuntime as (token: string) => Promise<ServiceRuntimeData>,
  rapidocr: getAdminRapidOCRRuntime as (token: string) => Promise<ServiceRuntimeData>,
  embedding: getAdminEmbeddingRuntime as (token: string) => Promise<ServiceRuntimeData>,
};

function translateOptional(
  translate: (key: string, values?: Record<string, string | number>) => string,
  key: string,
  fallback: string,
): string {
  try {
    return translate(key);
  } catch {
    return fallback;
  }
}

function toEditorField(field: SettingsField, translate: (key: string) => string) {
  const fieldKey = `fields.${field.namespace}.${field.key}`;
  return {
    id: resolveFieldID(field),
    label: translateOptional(translate, `${fieldKey}.label`, field.label),
    description: translateOptional(translate, `${fieldKey}.description`, field.description),
    type: field.type,
    placeholder: field.placeholder ? translateOptional(translate, `${fieldKey}.placeholder`, field.placeholder) : undefined,
    valueUnit: field.valueUnit,
    options: field.options?.map((option) => ({
      ...option,
      label: translateOptional(translate, `${fieldKey}.options.${option.value}`, option.label),
    })),
  } as const;
}

export function AdminFilesSettingsPage() {
  const t = useTranslations("adminFiles");
  const tActions = useTranslations("common.actions");
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [settingsMap, setSettingsMap] = React.useState<Record<string, string>>(() => applySettingsDefaults({}));
  const [savedMap, setSavedMap] = React.useState<Record<string, string>>(() => applySettingsDefaults({}));
  const [configuredMap, setConfiguredMap] = React.useState<Record<string, boolean>>({});
  const [serviceStates, setServiceStates] = React.useState<Record<ServiceName, ServiceState>>(INITIAL_SERVICE_STATES);
  const [embeddingStatus, setEmbeddingStatus] = React.useState<AdminEmbeddingIndexStatus | null>(null);
  const [embeddingStatusLoading, setEmbeddingStatusLoading] = React.useState(false);
  const [reindexing, setReindexing] = React.useState(false);
  const [modelOptions, setModelOptions] = React.useState<ModelOption[]>(() =>
    buildTaskModelOptions({
      models: [],
      followLabel: t("model.followCurrent"),
      followValue: TASK_MODEL_FOLLOW,
    }),
  );

  const loadEmbeddingStatus = React.useCallback(async () => {
    setEmbeddingStatusLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      const status = await getAdminEmbeddingStatus(token);
      setEmbeddingStatus(status);
    } catch {
      setEmbeddingStatus(null);
    } finally {
      setEmbeddingStatusLoading(false);
    }
  }, []);

  const handleReindex = React.useCallback(async () => {
    setReindexing(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"));
        return;
      }
      const result = await triggerAdminEmbeddingReindex(token);
      toast.success(t("toast.reindexSubmitted"), {
        description: t("toast.reindexSubmittedDescription", { count: result.submitted }),
      });
      setTimeout(() => { void loadEmbeddingStatus(); }, 1500);
    } catch (error) {
      toast.error(t("toast.reindexFailed"), { description: resolveErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setReindexing(false);
    }
  }, [loadEmbeddingStatus, t]);

  const loadServiceRuntime = React.useCallback(async (name: ServiceName) => {
    setServiceStates((prev) => ({ ...prev, [name]: { ...prev[name], loading: true } }));
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      const data = await SERVICE_LOADERS[name](token);
      setServiceStates((prev) => ({ ...prev, [name]: { ...prev[name], data, loading: false } }));
    } catch {
      setServiceStates((prev) => ({ ...prev, [name]: { ...prev[name], data: null, loading: false } }));
    }
  }, []);

  const handleServiceAction = React.useCallback(
    async (_action: Extract<ServiceRuntimeActionName, "test">, name: ServiceName) => {
      setServiceStates((prev) => ({ ...prev, [name]: { ...prev[name], action: "test" } }));
      try {
        await loadServiceRuntime(name);
      } catch (error) {
        toast.error(t("toast.serviceTestFailed", { service: SERVICE_LABELS[name] }), {
          description: resolveErrorMessage(error, t("toast.unknownError")),
        });
      } finally {
        setServiceStates((prev) => ({ ...prev, [name]: { ...prev[name], action: "" } }));
      }
    },
    [loadServiceRuntime, t],
  );

  const syncServiceRuntimes = React.useCallback(
    (flattened: Record<string, string>) => {
      const active = resolveActiveServices(flattened);
      for (const name of SERVICE_NAMES) {
        if (active.has(name)) {
          void loadServiceRuntime(name);
        } else {
          setServiceStates((prev) => ({ ...prev, [name]: { ...prev[name], data: null } }));
        }
      }
    },
    [loadServiceRuntime],
  );

  const loadSettings = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const [grouped, referenceData] = await Promise.all([
        listAdminSettings(token),
        getAdminReferenceData(token).catch(() => null),
      ]);
      const nextModelOptions = buildTaskModelOptions({
        models: referenceData?.models ?? [],
        followLabel: t("model.followCurrent"),
        followValue: TASK_MODEL_FOLLOW,
      });
      const flattened = applySettingsDefaults(flattenSettings(SETTINGS_GROUPS, grouped));
      setConfiguredMap(configuredSettingsMap(grouped));
      setModelOptions(nextModelOptions);
      setSettingsMap(flattened);
      setSavedMap(flattened);
      syncServiceRuntimes(flattened);
      if (flattened["file.embedding_enabled"] === EMBEDDING_MODES.ON) {
        void loadEmbeddingStatus();
      } else {
        setEmbeddingStatus(null);
      }
    } catch (error) {
      toast.error(t("toast.loadFailed"), { description: resolveErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setLoading(false);
    }
  }, [loadEmbeddingStatus, syncServiceRuntimes, t]);

  React.useEffect(() => {
    void loadSettings();
  }, [loadSettings]);

  const dirtyFieldIDs = React.useMemo(() => {
    const result = new Set<string>();
    for (const group of SETTINGS_GROUPS) {
      for (const field of resolveVisibleFields(group, settingsMap)) {
        const fieldID = resolveFieldID(field);
        if ((settingsMap[fieldID] ?? "") !== (savedMap[fieldID] ?? "")) {
          result.add(fieldID);
        }
      }
    }
    return result;
  }, [savedMap, settingsMap]);

  const handleFieldChange = React.useCallback((fieldID: string, value: string) => {
    setSettingsMap((prev) => {
      let next =
        fieldID === "extract.mineru_source"
          ? { ...prev, [fieldID]: value }
          : { ...prev, [fieldID]: value };
      if ((fieldID === "file.embedding_enabled" || fieldID === "file.embedding_host" || fieldID === "file.rag_model") && !isEmbeddingServiceConfigured(next)) {
        next = {
          ...next,
          "chat.rag_enabled": "false",
          "chat.message_embedding_enabled": "false",
          "chat.semantic_context_enabled": "false",
        };
      }
      if (fieldID === "chat.rag_enabled" && value === "true" && !isEmbeddingServiceConfigured(next)) {
        toast.error(t("toast.cannotEnable"), { description: t("validation.embeddingRequired") });
        return prev;
      }
      if ((fieldID === "chat.message_embedding_enabled" || fieldID === "chat.semantic_context_enabled") && value === "true" && !isEmbeddingServiceConfigured(next)) {
        toast.error(t("toast.cannotEnable"), { description: t("validation.embeddingRequired") });
        return prev;
      }
      if (fieldID === "chat.semantic_context_enabled" && value === "true" && next["chat.message_embedding_enabled"] !== "true") {
        toast.error(t("toast.cannotEnable"), { description: t("validation.messageEmbeddingRequired") });
        return prev;
      }
      if (next["chat.message_embedding_enabled"] !== "true") {
        next = {
          ...next,
          "chat.semantic_context_enabled": "false",
        };
      }
      return next;
    });
  }, [t]);

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

  const handleSaveGroup = React.useCallback(
    async (group: SettingsGroup) => {
      const draftSettingsMap = { ...settingsMap };
      if (usesTika(draftSettingsMap["extract.engine"] ?? "") && !draftSettingsMap["extract.tika_base_url"]?.trim()) {
        toast.error(t("toast.saveFailed"), { description: t("validation.tikaBaseURLRequired") });
        return;
      }
      if ((draftSettingsMap["extract.engine"] ?? "") === EXTRACT_ENGINE_POLICIES.DOCLING && !draftSettingsMap["extract.docling_base_url"]?.trim()) {
        toast.error(t("toast.saveFailed"), { description: t("validation.doclingBaseURLRequired") });
        return;
      }
      if ((draftSettingsMap["extract.engine"] ?? "") === EXTRACT_ENGINE_POLICIES.MINERU && !draftSettingsMap["extract.mineru_base_url"]?.trim()) {
        toast.error(t("toast.saveFailed"), { description: t("validation.mineruBaseURLRequired") });
        return;
      }
      const ocrEngine = resolveOCREngine(draftSettingsMap["extract.ocr_engine"] ?? "");
      const ocrEnabled = draftSettingsMap["extract.image_ocr_enabled"] === "true" || draftSettingsMap["extract.pdf_ocr_fallback_enabled"] === "true";
      if (ocrEnabled && ocrEngine === OCR_ENGINES.TESSERACT && !draftSettingsMap["extract.tesseract_ocr_base_url"]?.trim()) {
        toast.error(t("toast.saveFailed"), { description: t("validation.tesseractBaseURLRequired") });
        return;
      }
      if (ocrEnabled && ocrEngine === OCR_ENGINES.RAPIDOCR && !draftSettingsMap["extract.rapidocr_base_url"]?.trim()) {
        toast.error(t("toast.saveFailed"), { description: t("validation.rapidOCRBaseURLRequired") });
        return;
      }
      if (ocrEnabled && ocrEngine === OCR_ENGINES.PADDLE && !draftSettingsMap["extract.paddle_ocr_base_url"]?.trim()) {
        toast.error(t("toast.saveFailed"), { description: t("validation.paddleOCRBaseURLRequired") });
        return;
      }
      if (ocrEnabled && ocrEngine === OCR_ENGINES.TENCENT && (!draftSettingsMap["extract.tencent_ocr_secret_id"]?.trim() || !settingHasValue(draftSettingsMap, configuredMap, "extract.tencent_ocr_secret_key") || !draftSettingsMap["extract.tencent_ocr_region"]?.trim())) {
        toast.error(t("toast.saveFailed"), { description: t("validation.tencentOCRRequired") });
        return;
      }
      if (ocrEnabled && ocrEngine === OCR_ENGINES.ALIYUN && (!draftSettingsMap["extract.aliyun_ocr_access_key_id"]?.trim() || !settingHasValue(draftSettingsMap, configuredMap, "extract.aliyun_ocr_access_key_secret") || !draftSettingsMap["extract.aliyun_ocr_region"]?.trim())) {
        toast.error(t("toast.saveFailed"), { description: t("validation.aliyunOCRRequired") });
        return;
      }
      if (ocrEnabled && ocrEngine === OCR_ENGINES.LLM && !draftSettingsMap["extract.llm_ocr_base_url"]?.trim()) {
        toast.error(t("toast.saveFailed"), { description: t("validation.llmOCRBaseURLRequired") });
        return;
      }
      if (ocrEnabled && ocrEngine === OCR_ENGINES.LLM && !draftSettingsMap["extract.llm_ocr_model"]?.trim()) {
        toast.error(t("toast.saveFailed"), { description: t("validation.llmOCRModelRequired") });
        return;
      }
      if (draftSettingsMap["file.embedding_enabled"] === EMBEDDING_MODES.ON && !draftSettingsMap["file.rag_model"]?.trim()) {
        toast.error(t("toast.saveFailed"), { description: t("validation.embeddingModelRequired") });
        return;
      }
      if (draftSettingsMap["file.embedding_enabled"] === EMBEDDING_MODES.ON && !draftSettingsMap["file.embedding_host"]?.trim()) {
        toast.error(t("toast.saveFailed"), { description: t("validation.embeddingHostRequired") });
        return;
      }
      const savingRAG = group.fields.some((field) => field.namespace === "chat" && field.key === "rag_enabled");
      if (savingRAG && draftSettingsMap["chat.rag_enabled"] === "true" && !isEmbeddingServiceConfigured(draftSettingsMap)) {
        toast.error(t("toast.saveFailed"), { description: t("validation.embeddingRequired") });
        return;
      }
      const savingSemanticEnhancement = group.fields.some((field) => field.namespace === "chat" && (field.key === "message_embedding_enabled" || field.key === "semantic_context_enabled"));
      if (savingSemanticEnhancement && draftSettingsMap["chat.message_embedding_enabled"] === "true" && !isEmbeddingServiceConfigured(draftSettingsMap)) {
        toast.error(t("toast.saveFailed"), { description: t("validation.embeddingRequired") });
        return;
      }
      if (savingSemanticEnhancement && draftSettingsMap["chat.semantic_context_enabled"] === "true" && draftSettingsMap["chat.message_embedding_enabled"] !== "true") {
        toast.error(t("toast.saveFailed"), { description: t("validation.messageEmbeddingRequiredBeforeSemantic") });
        return;
      }
      const nextSettingsMap = applySettingsDefaults(draftSettingsMap);

      const items: PatchSettingItem[] = resolveVisibleFields(group, nextSettingsMap)
        .filter(isSettingsValueField)
        .filter((field) => dirtyFieldIDs.has(resolveFieldID(field)))
        .map((field) => ({
          namespace: field.namespace,
          key: field.key,
          value: field.valueUnit === "mb"
            ? denormalizeMBValue(nextSettingsMap[resolveFieldID(field)] ?? "")
            : (nextSettingsMap[resolveFieldID(field)] ?? ""),
        }));
      if (group.fields.some((field) => field.namespace === "file" && (field.key === "embedding_enabled" || field.key === "embedding_host" || field.key === "rag_model")) && !isEmbeddingServiceConfigured(nextSettingsMap)) {
        const existingKeys = new Set(items.map((item) => `${item.namespace}.${item.key}`));
        for (const item of [
          { namespace: "chat", key: "rag_enabled", value: "false" },
          { namespace: "chat", key: "message_embedding_enabled", value: "false" },
          { namespace: "chat", key: "semantic_context_enabled", value: "false" },
        ] as PatchSettingItem[]) {
          if (!existingKeys.has(`${item.namespace}.${item.key}`)) items.push(item);
        }
      }

      if (group.fields.some((field) => field.namespace === "extract") && usesTika(nextSettingsMap["extract.engine"] ?? "")) {
        const existingKeys = new Set(items.map((item) => `${item.namespace}.${item.key}`));
        for (const item of [
          { namespace: "extract", key: "tika_source", value: TIKA_SERVICE_SOURCES.EXTERNAL },
          { namespace: "extract", key: "tika_base_url", value: nextSettingsMap["extract.tika_base_url"] ?? "" },
          { namespace: "extract", key: "tika_timeout_seconds", value: nextSettingsMap["extract.tika_timeout_seconds"] ?? "60" },
        ] as PatchSettingItem[]) {
          if (!existingKeys.has(`${item.namespace}.${item.key}`)) items.push(item);
        }
      }
      if (group.fields.some((field) => field.namespace === "extract") && (nextSettingsMap["extract.engine"] ?? "") === EXTRACT_ENGINE_POLICIES.DOCLING) {
        const existingKeys = new Set(items.map((item) => `${item.namespace}.${item.key}`));
        for (const item of [
          { namespace: "extract", key: "docling_base_url", value: nextSettingsMap["extract.docling_base_url"] ?? "" },
          { namespace: "extract", key: "docling_timeout_seconds", value: nextSettingsMap["extract.docling_timeout_seconds"] ?? "60" },
        ] as PatchSettingItem[]) {
          if (!existingKeys.has(`${item.namespace}.${item.key}`)) items.push(item);
        }
      }
      if (group.fields.some((field) => field.namespace === "extract") && (nextSettingsMap["extract.engine"] ?? "") === EXTRACT_ENGINE_POLICIES.MINERU) {
        const existingKeys = new Set(items.map((item) => `${item.namespace}.${item.key}`));
        for (const item of [
          { namespace: "extract", key: "mineru_source", value: resolveMinerUSource(nextSettingsMap["extract.mineru_source"] ?? "") },
          { namespace: "extract", key: "mineru_base_url", value: nextSettingsMap["extract.mineru_base_url"] ?? "" },
          { namespace: "extract", key: "mineru_timeout_seconds", value: nextSettingsMap["extract.mineru_timeout_seconds"] ?? "180" },
        ] as PatchSettingItem[]) {
          if (!existingKeys.has(`${item.namespace}.${item.key}`)) items.push(item);
        }
      }
      if (group.fields.some((field) => field.namespace === "extract")) {
        const existingKeys = new Set(items.map((item) => `${item.namespace}.${item.key}`));
        const providerDefaults: PatchSettingItem[] =
          ocrEngine === OCR_ENGINES.TESSERACT
            ? [
                { namespace: "extract", key: "tesseract_ocr_base_url", value: nextSettingsMap["extract.tesseract_ocr_base_url"] ?? "" },
                { namespace: "extract", key: "tesseract_ocr_timeout_seconds", value: nextSettingsMap["extract.tesseract_ocr_timeout_seconds"] ?? "60" },
              ]
            : ocrEngine === OCR_ENGINES.RAPIDOCR
            ? [
                { namespace: "extract", key: "rapidocr_source", value: TIKA_SERVICE_SOURCES.EXTERNAL },
                { namespace: "extract", key: "rapidocr_base_url", value: nextSettingsMap["extract.rapidocr_base_url"] ?? "" },
                { namespace: "extract", key: "rapidocr_timeout_seconds", value: nextSettingsMap["extract.rapidocr_timeout_seconds"] ?? "60" },
              ]
            : ocrEngine === OCR_ENGINES.PADDLE
              ? [
                  { namespace: "extract", key: "paddle_ocr_timeout_seconds", value: nextSettingsMap["extract.paddle_ocr_timeout_seconds"] ?? "60" },
                ]
            : ocrEngine === OCR_ENGINES.TENCENT
              ? [
                  { namespace: "extract", key: "tencent_ocr_region", value: nextSettingsMap["extract.tencent_ocr_region"] ?? "ap-guangzhou" },
                  { namespace: "extract", key: "tencent_ocr_endpoint", value: nextSettingsMap["extract.tencent_ocr_endpoint"] ?? "ocr.tencentcloudapi.com" },
                  { namespace: "extract", key: "tencent_ocr_timeout_seconds", value: nextSettingsMap["extract.tencent_ocr_timeout_seconds"] ?? "60" },
                ]
            : ocrEngine === OCR_ENGINES.ALIYUN
              ? [
                  { namespace: "extract", key: "aliyun_ocr_region", value: nextSettingsMap["extract.aliyun_ocr_region"] ?? "cn-hangzhou" },
                  { namespace: "extract", key: "aliyun_ocr_endpoint", value: nextSettingsMap["extract.aliyun_ocr_endpoint"] ?? "ocr-api.cn-hangzhou.aliyuncs.com" },
                  { namespace: "extract", key: "aliyun_ocr_timeout_seconds", value: nextSettingsMap["extract.aliyun_ocr_timeout_seconds"] ?? "60" },
                ]
            : ocrEngine === OCR_ENGINES.LLM
              ? [
                  { namespace: "extract", key: "llm_ocr_model", value: nextSettingsMap["extract.llm_ocr_model"] ?? "" },
                  { namespace: "extract", key: "llm_ocr_timeout_seconds", value: nextSettingsMap["extract.llm_ocr_timeout_seconds"] ?? "60" },
                ]
              : [];
        for (const item of providerDefaults) {
          if (!existingKeys.has(`${item.namespace}.${item.key}`)) items.push(item);
        }
      }

      if (items.length === 0) {
        return;
      }

      const embeddingModelWillChange =
        items.some((item) => item.namespace === "file" && item.key === "rag_model" && item.value !== (savedMap["file.rag_model"] ?? "")) ||
        items.some((item) => item.namespace === "file" && item.key === "embedding_output_dimensions" && item.value !== (savedMap["file.embedding_output_dimensions"] ?? ""));

      setSaving(true);
      try {
        const token = await resolveAccessToken();
        if (!token) {
          toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
          return;
        }
        const grouped = await patchAdminSettings(token, { items });
        const flattened = applySettingsDefaults(flattenSettings(SETTINGS_GROUPS, grouped));
        setConfiguredMap(configuredSettingsMap(grouped));
        setSettingsMap(flattened);
        setSavedMap(flattened);
        syncServiceRuntimes(flattened);
        if (embeddingModelWillChange) {
          toast.warning(t("toast.embeddingModelChanged"), {
            description: t("toast.embeddingModelChangedDescription"),
            duration: 8000,
          });
          if (flattened["file.embedding_enabled"] === EMBEDDING_MODES.ON) {
            void loadEmbeddingStatus();
          } else {
            setEmbeddingStatus(null);
          }
        } else {
          toast.success(t("toast.groupUpdated", { group: translateOptional(t, `groups.${group.key}.title`, group.title) }));
          if (group.fields.some((f) => f.namespace === "file" && (f.key === "embedding_enabled" || f.key === "rag_model" || f.key === "embedding_host"))) {
            if (flattened["file.embedding_enabled"] === EMBEDDING_MODES.ON) {
              void loadEmbeddingStatus();
            } else {
              setEmbeddingStatus(null);
            }
          }
        }
      } catch (error) {
        toast.error(t("toast.saveFailed"), { description: resolveErrorMessage(error, t("toast.unknownError")) });
      } finally {
        setSaving(false);
      }
    },
    [configuredMap, dirtyFieldIDs, loadEmbeddingStatus, savedMap, settingsMap, syncServiceRuntimes, t],
  );

  const requestSaveGroup = React.useCallback((group: SettingsGroup) => {
    void handleSaveGroup(group);
  }, [handleSaveGroup]);

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
                title={translateOptional(t, `groups.${group.key}.title`, group.title)}
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
                        if (fieldID === "chat.compact_task_model") {
                          return (
                            <SettingsFieldItem key={fieldID} index={blockIndex}>
                              <TaskModelField
                                id={fieldID}
                                label={translateOptional(t, `fields.${block.field.namespace}.${block.field.key}.label`, block.field.label)}
                                description={translateOptional(t, `fields.${block.field.namespace}.${block.field.key}.description`, block.field.description)}
                                value={settingsMap[fieldID] ?? ""}
                                fallbackValue={TASK_MODEL_FOLLOW}
                                dirty={(settingsMap[fieldID] ?? "") !== (savedMap[fieldID] ?? "")}
                                disabled={loading || saving}
                                modelOptions={modelOptions}
                                onChange={(value) => handleFieldChange(fieldID, value)}
                              />
                            </SettingsFieldItem>
                          );
                        }
                        return (
                          <SettingsFieldItem key={fieldID} index={blockIndex}>
                            <SettingsFieldEditor
                              field={{
                                ...toEditorField(block.field, t),
                                ...(block.field.runtimeService
                                  ? { serviceRuntime: resolveServiceRuntime(block.field.runtimeService) }
                                  : {}),
                              }}
                              value={settingsMap[fieldID] ?? ""}
                              configured={configuredMap[fieldID]}
                              dirty={(settingsMap[fieldID] ?? "") !== (savedMap[fieldID] ?? "")}
                              disabled={loading || saving}
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
                          <div className="overflow-hidden">
                            <SettingsFieldItem index={blockIndex}>
                              <SettingsFieldInset>
                                <SettingsFieldList className="gap-3 md:gap-4">
                                  {block.fields.map((field) => {
                                    const fieldID = resolveFieldID(field);
                                    if (fieldID === "chat.compact_task_model") {
                                      return (
                                        <TaskModelField
                                          key={fieldID}
                                          id={fieldID}
                                          label={translateOptional(t, `fields.${field.namespace}.${field.key}.label`, field.label)}
                                          description={translateOptional(t, `fields.${field.namespace}.${field.key}.description`, field.description)}
                                          value={settingsMap[fieldID] ?? ""}
                                          fallbackValue={TASK_MODEL_FOLLOW}
                                          dirty={(settingsMap[fieldID] ?? "") !== (savedMap[fieldID] ?? "")}
                                          disabled={loading || saving}
                                          modelOptions={modelOptions}
                                          onChange={(value) => handleFieldChange(fieldID, value)}
                                        />
                                      );
                                    }
                                    return (
                                      <SettingsFieldEditor
                                        key={fieldID}
                                        field={{
                                          ...toEditorField(field, t),
                                          ...(field.runtimeService
                                            ? { serviceRuntime: resolveServiceRuntime(field.runtimeService) }
                                            : {}),
                                        }}
                                        value={settingsMap[fieldID] ?? ""}
                                        configured={configuredMap[fieldID]}
                                        dirty={(settingsMap[fieldID] ?? "") !== (savedMap[fieldID] ?? "")}
                                        disabled={loading || saving}
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
                  <div className="min-w-0 space-y-3 rounded-lg border border-border/60 bg-muted/30 p-4">
                    <div className="flex min-w-0 items-center justify-between gap-3">
                      <div className="min-w-0 space-y-0.5">
                        <p className="text-xs font-medium">{t("embeddingStatus.title")}</p>
                        {embeddingStatus?.modelSignature ? (
                          <p className="min-w-0 break-all font-mono text-[11px] text-muted-foreground">{embeddingStatus.modelSignature}</p>
                        ) : (
                          <p className="text-[11px] text-muted-foreground">{t("embeddingStatus.noSignature")}</p>
                        )}
                      </div>
                      <Button
                        type="button"
                        size="sm"
                        variant="default"
                        disabled={reindexing || embeddingStatusLoading || loading || saving}
                        onClick={() => void handleReindex()}
                      >
                        {reindexing ? t("embeddingStatus.reindexing") : t("embeddingStatus.reindex")}
                      </Button>
                    </div>
                    {embeddingStatus ? (
                      <div className="grid min-w-0 grid-cols-2 gap-2 text-center sm:grid-cols-4">
                        {[
                          { label: t("embeddingStatus.ready"), value: embeddingStatus.readyCount, color: "text-green-600 dark:text-green-400" },
                          { label: t("embeddingStatus.stale"), value: embeddingStatus.staleCount, color: embeddingStatus.staleCount > 0 ? "text-amber-600 dark:text-amber-400" : "text-muted-foreground" },
                          { label: t("embeddingStatus.pending"), value: embeddingStatus.pendingCount, color: "text-muted-foreground" },
                          { label: t("embeddingStatus.failed"), value: embeddingStatus.failedCount, color: embeddingStatus.failedCount > 0 ? "text-destructive" : "text-muted-foreground" },
                        ].map(({ label, value, color }) => (
                          <div key={label} className="rounded-md bg-background/60 py-2 px-1 border border-border/40">
                            <p className={cn("text-base font-semibold tabular-nums", color)}>{value}</p>
                            <p className="text-[10px] text-muted-foreground mt-0.5">{label}</p>
                          </div>
                        ))}
                      </div>
                    ) : embeddingStatusLoading ? (
                      <div className="grid min-w-0 grid-cols-2 gap-2 sm:grid-cols-4" aria-hidden="true">
                        {Array.from({ length: 4 }).map((_, index) => (
                          <div key={`embedding-status-skeleton-${index}`} className="rounded-md border border-border/40 bg-background/60 px-2 py-2">
                            <div className="mx-auto h-5 w-8 animate-pulse rounded-sm bg-muted/70" />
                            <div className="mx-auto mt-1.5 h-2.5 w-10 animate-pulse rounded-sm bg-muted/60" />
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-[11px] text-muted-foreground">
                        {t("embeddingStatus.empty")}
                      </p>
                    )}
                    {embeddingStatus?.needsReindex && (
                      <p className="text-[11px] text-amber-600 dark:text-amber-400">
                        {t("embeddingStatus.needsReindex")}
                      </p>
                    )}
                  </div>
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
