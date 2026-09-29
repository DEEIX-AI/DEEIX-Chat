import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  getAdminDoclingRuntime,
  getAdminEmbeddingRuntime,
  getAdminMinerURuntime,
  getAdminRapidOCRRuntime,
  getAdminTesseractRuntime,
  getAdminTikaRuntime,
  listAdminSettings,
  patchAdminSettings,
} from "@/features/admin/api";
import {
  applySettingsDefaults,
  denormalizeMBValue,
  EMBEDDING_MODES,
  EXTRACT_ENGINE_POLICIES,
  flattenSettings,
  INITIAL_SERVICE_STATES,
  isEmbeddingServiceConfigured,
  isSettingsValueField,
  normalizeMinerUFileTypes,
  OCR_ENGINES,
  resolveFieldID,
  resolveMinerUSource,
  resolveOCREngine,
  resolveVisibleFields,
  SERVICE_LABELS,
  SETTINGS_GROUPS,
  type ServiceName,
  type ServiceRuntimeData,
  type ServiceState,
  type SettingsGroup,
  TIKA_SERVICE_SOURCES,
  usesTika,
} from "@/features/admin/model/files-settings";
import type { ServiceRuntimeActionName } from "@/features/admin/components/shared/settings-runtime-panel";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import type { PatchSettingItem } from "@/shared/api/settings-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { configuredSettingsMap, settingHasValue } from "@/features/admin/utils/settings-meta";

const SERVICE_LOADERS: Record<ServiceName, (token: string) => Promise<ServiceRuntimeData>> = {
  tika: getAdminTikaRuntime,
  docling: getAdminDoclingRuntime,
  mineru: getAdminMinerURuntime,
  tesseract: getAdminTesseractRuntime,
  rapidocr: getAdminRapidOCRRuntime,
  embedding: getAdminEmbeddingRuntime,
};

/**
 * Files/extraction/embedding settings: loading, per-field change rules (which
 * cascade dependent toggles off), validation and saving one group at a time,
 * plus on-demand runtime probes for the external extraction services. The
 * embedding status callbacks come from `useAdminFilesEmbeddingStatus` so a save
 * that changes the embedding setup can refresh or clear that panel.
 */
export function useAdminFilesSettings({
  loadEmbeddingStatus,
  clearEmbeddingStatus,
}: {
  loadEmbeddingStatus: () => Promise<void>;
  clearEmbeddingStatus: () => void;
}) {
  const t = useTranslations("adminFiles");
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [settingsMap, setSettingsMap] = React.useState<Record<string, string>>(() => applySettingsDefaults({}));
  const [savedMap, setSavedMap] = React.useState<Record<string, string>>(() => applySettingsDefaults({}));
  const [configuredMap, setConfiguredMap] = React.useState<Record<string, boolean>>({});
  const [serviceStates, setServiceStates] = React.useState<Record<ServiceName, ServiceState>>(INITIAL_SERVICE_STATES);
  const loadServiceRuntime = React.useCallback(async (name: ServiceName) => {
    setServiceStates((prev) => ({ ...prev, [name]: { ...prev[name], loading: true } }));
    try {
      const token = await resolveAccessToken();
      if (!token) {
        setServiceStates((prev) => ({ ...prev, [name]: { ...prev[name], loading: false } }));
        return;
      }
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
          description: resolveAdminErrorMessage(error, t("toast.unknownError")),
        });
      } finally {
        setServiceStates((prev) => ({ ...prev, [name]: { ...prev[name], action: "" } }));
      }
    },
    [loadServiceRuntime, t],
  );

  const loadSettings = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const grouped = await listAdminSettings(token);
      const flattened = applySettingsDefaults(flattenSettings(SETTINGS_GROUPS, grouped));
      setConfiguredMap(configuredSettingsMap(grouped));
      setSettingsMap(flattened);
      setSavedMap(flattened);
      setServiceStates(INITIAL_SERVICE_STATES);
      if (flattened["file.embedding_enabled"] === EMBEDDING_MODES.ON) {
        void loadEmbeddingStatus();
      } else {
        clearEmbeddingStatus();
      }
    } catch (error) {
      toast.error(t("toast.loadFailed"), { description: resolveAdminErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setLoading(false);
    }
  }, [clearEmbeddingStatus, loadEmbeddingStatus, t]);

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
    if (fieldID === "file.embedding_enabled" && value !== EMBEDDING_MODES.ON) {
      clearEmbeddingStatus();
    }
    setSettingsMap((prev) => {
      let next = { ...prev, [fieldID]: value };
      if (fieldID === "extract.engine" && value === EXTRACT_ENGINE_POLICIES.MINERU) {
        next["extract.mineru_file_types"] = normalizeMinerUFileTypes(next["extract.mineru_file_types"] ?? "");
      }
      if (fieldID === "extract.mineru_file_types") {
        next["extract.mineru_file_types"] = normalizeMinerUFileTypes(value);
      }
      if (fieldID === "extract.ocr_engine" && value === OCR_ENGINES.MISTRAL) {
        if (!(next["extract.mistral_ocr_base_url"] ?? "").trim()) {
          next["extract.mistral_ocr_base_url"] = "https://api.mistral.ai/v1/ocr";
        }
        if (!(next["extract.mistral_ocr_model"] ?? "").trim()) {
          next["extract.mistral_ocr_model"] = "mistral-ocr-latest";
        }
        if (!(next["extract.mistral_ocr_timeout_seconds"] ?? "").trim()) {
          next["extract.mistral_ocr_timeout_seconds"] = "60";
        }
      }
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
  }, [clearEmbeddingStatus, t]);

  const handleSaveAllowedMIMETypes = React.useCallback(
    async (nextValue: string) => {
      setSaving(true);
      try {
        const token = await resolveAccessToken();
        if (!token) {
          toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
          return;
        }
        const grouped = await patchAdminSettings(token, {
          items: [{ namespace: "file", key: "allowed_mime_types", value: nextValue }],
        });
        const flattened = applySettingsDefaults(flattenSettings(SETTINGS_GROUPS, grouped));
        const savedValue = flattened["file.allowed_mime_types"] ?? nextValue;
        setConfiguredMap(configuredSettingsMap(grouped));
        setSettingsMap((current) => ({
          ...current,
          "file.allowed_mime_types": savedValue,
        }));
        setSavedMap(flattened);
        setServiceStates(INITIAL_SERVICE_STATES);
        toast.success(t("toast.mimeTypesUpdated"));
      } catch (error) {
        toast.error(t("toast.saveFailed"), { description: resolveAdminErrorMessage(error, t("toast.unknownError")) });
      } finally {
        setSaving(false);
      }
    },
    [t],
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
      if (ocrEnabled && ocrEngine === OCR_ENGINES.MISTRAL && !draftSettingsMap["extract.mistral_ocr_base_url"]?.trim()) {
        toast.error(t("toast.saveFailed"), { description: t("validation.mistralOCRBaseURLRequired") });
        return;
      }
      if (ocrEnabled && ocrEngine === OCR_ENGINES.MISTRAL && !settingHasValue(draftSettingsMap, configuredMap, "extract.mistral_ocr_auth_token")) {
        toast.error(t("toast.saveFailed"), { description: t("validation.mistralOCRAPIKeyRequired") });
        return;
      }
      if (ocrEnabled && ocrEngine === OCR_ENGINES.MISTRAL && !draftSettingsMap["extract.mistral_ocr_model"]?.trim()) {
        toast.error(t("toast.saveFailed"), { description: t("validation.mistralOCRModelRequired") });
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
        ] satisfies PatchSettingItem[]) {
          if (!existingKeys.has(`${item.namespace}.${item.key}`)) items.push(item);
        }
      }

      if (group.fields.some((field) => field.namespace === "extract") && usesTika(nextSettingsMap["extract.engine"] ?? "")) {
        const existingKeys = new Set(items.map((item) => `${item.namespace}.${item.key}`));
        for (const item of [
          { namespace: "extract", key: "tika_source", value: TIKA_SERVICE_SOURCES.EXTERNAL },
          { namespace: "extract", key: "tika_base_url", value: nextSettingsMap["extract.tika_base_url"] ?? "" },
          { namespace: "extract", key: "tika_timeout_seconds", value: nextSettingsMap["extract.tika_timeout_seconds"] ?? "60" },
        ] satisfies PatchSettingItem[]) {
          if (!existingKeys.has(`${item.namespace}.${item.key}`)) items.push(item);
        }
      }
      if (group.fields.some((field) => field.namespace === "extract") && (nextSettingsMap["extract.engine"] ?? "") === EXTRACT_ENGINE_POLICIES.DOCLING) {
        const existingKeys = new Set(items.map((item) => `${item.namespace}.${item.key}`));
        for (const item of [
          { namespace: "extract", key: "docling_base_url", value: nextSettingsMap["extract.docling_base_url"] ?? "" },
          { namespace: "extract", key: "docling_timeout_seconds", value: nextSettingsMap["extract.docling_timeout_seconds"] ?? "60" },
        ] satisfies PatchSettingItem[]) {
          if (!existingKeys.has(`${item.namespace}.${item.key}`)) items.push(item);
        }
      }
      if (group.fields.some((field) => field.namespace === "extract") && (nextSettingsMap["extract.engine"] ?? "") === EXTRACT_ENGINE_POLICIES.MINERU) {
        const existingKeys = new Set(items.map((item) => `${item.namespace}.${item.key}`));
        for (const item of [
          { namespace: "extract", key: "mineru_source", value: resolveMinerUSource(nextSettingsMap["extract.mineru_source"] ?? "") },
          { namespace: "extract", key: "mineru_base_url", value: nextSettingsMap["extract.mineru_base_url"] ?? "" },
          { namespace: "extract", key: "mineru_file_types", value: nextSettingsMap["extract.mineru_file_types"] ?? "" },
          { namespace: "extract", key: "mineru_timeout_seconds", value: nextSettingsMap["extract.mineru_timeout_seconds"] ?? "180" },
        ] satisfies PatchSettingItem[]) {
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
            : ocrEngine === OCR_ENGINES.MISTRAL
              ? [
                  { namespace: "extract", key: "mistral_ocr_base_url", value: nextSettingsMap["extract.mistral_ocr_base_url"] ?? "https://api.mistral.ai/v1/ocr" },
                  { namespace: "extract", key: "mistral_ocr_model", value: nextSettingsMap["extract.mistral_ocr_model"] ?? "mistral-ocr-latest" },
                  { namespace: "extract", key: "mistral_ocr_timeout_seconds", value: nextSettingsMap["extract.mistral_ocr_timeout_seconds"] ?? "60" },
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
        setServiceStates(INITIAL_SERVICE_STATES);
        if (embeddingModelWillChange) {
          toast.warning(t("toast.embeddingModelChanged"), {
            description: t("toast.embeddingModelChangedDescription"),
            duration: 8000,
          });
        } else {
          toast.success(t("toast.groupUpdated", { group: t(`groups.${group.key}.title`) }));
        }
        if (
          embeddingModelWillChange ||
          group.fields.some((field) =>
            field.namespace === "file" &&
            (field.key === "embedding_enabled" || field.key === "rag_model" || field.key === "embedding_host")
          )
        ) {
          if (flattened["file.embedding_enabled"] === EMBEDDING_MODES.ON) {
            void loadEmbeddingStatus();
          } else {
            clearEmbeddingStatus();
          }
        }
      } catch (error) {
        toast.error(t("toast.saveFailed"), { description: resolveAdminErrorMessage(error, t("toast.unknownError")) });
      } finally {
        setSaving(false);
      }
    },
    [clearEmbeddingStatus, configuredMap, dirtyFieldIDs, loadEmbeddingStatus, savedMap, settingsMap, t],
  );

  return {
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
  };
}
