import type { SettingsGrouped } from "@/shared/api/settings.types";
import { resolveLocalizedErrorMessage } from "@/i18n/resolve-error-message";

export type SettingsFieldType = "int" | "bool" | "string" | "password" | "textarea" | "select" | "tabs" | "button";

export type VisibilityRule =
  | { field: string; equals: string }
  | { field: string; notEquals: string }
  | { field: string; nonEmpty: true }
  | { field: string; in: string[] }
  | { all: VisibilityRule[] }
  | { any: VisibilityRule[] };

export type ServiceName = "tika" | "docling" | "mineru" | "tesseract" | "rapidocr" | "embedding";

export type SettingsValueField = {
  namespace: string;
  key: string;
  label: string;
  description: string;
  type: SettingsFieldType;
  placeholder?: string;
  valueUnit?: "mb";
  options?: Array<{ label: string; value: string }>;
  visibleWhen?: VisibilityRule;
  subgroupKey?: string;
  subgroupTitle?: string;
  subgroupDescription?: string;
  runtimeService?: ServiceName;
};

export type SettingsField = SettingsValueField;

export type SettingsGroup = {
  key: string;
  title: string;
  description: string;
  fields: SettingsField[];
};

export type VisibleFieldBlock =
  | { kind: "field"; field: SettingsField }
  | {
      kind: "subgroup";
      key: string;
      title: string;
      description?: string;
      fields: SettingsField[];
    };

export type ServiceRuntimeData = {
  status: string;
  reachable: boolean;
  message: string;
  baseURL: string;
};

export type ServiceState = {
  data: ServiceRuntimeData | null;
  loading: boolean;
  action: "" | "test";
};

export const TASK_MODEL_FOLLOW = "follow";

export const EXTRACT_ENGINE_POLICIES = {
  BUILTIN: "builtin",
  TIKA: "tika",
  DOCLING: "docling",
  MINERU: "mineru",
} as const;

export const TIKA_SERVICE_SOURCES = {
  EXTERNAL: "external",
  MANAGED: "managed",
} as const;

export const OCR_ENGINES = {
  RAPIDOCR: "rapidocr",
  TESSERACT: "tesseract",
  PADDLE: "paddle",
  TENCENT: "tencent",
  ALIYUN: "aliyun",
  LLM: "llm",
} as const;

export const EMBEDDING_MODES = {
  OFF: "false",
  ON: "true",
} as const;

export const FULL_CONTEXT_LIMIT_MODES = {
  OFF: "false",
  ON: "true",
} as const;

export const MINERU_SERVICE_SOURCES = {
  CLOUD: "cloud",
  SELF_HOSTED: "self_hosted",
} as const;

export const EMBEDDING_READY_RULE: VisibilityRule = {
  all: [
    { field: "file.embedding_enabled", equals: EMBEDDING_MODES.ON },
    { field: "file.embedding_host", nonEmpty: true },
    { field: "file.rag_model", nonEmpty: true },
  ],
};

export const OCR_ENABLED_RULE: VisibilityRule = {
  any: [
    { field: "extract.image_ocr_enabled", equals: "true" },
    { field: "extract.pdf_ocr_fallback_enabled", equals: "true" },
  ],
};

export const SERVICE_NAMES: ServiceName[] = ["tika", "docling", "mineru", "tesseract", "rapidocr", "embedding"];

export const SERVICE_LABELS: Record<ServiceName, string> = {
  tika: "Tika",
  docling: "Docling",
  mineru: "MinerU",
  tesseract: "Tesseract OCR",
  rapidocr: "RapidOCR",
  embedding: "Embedding",
};

export const SERVICE_DIRTY_FIELDS: Record<ServiceName, string[]> = {
  tika: ["extract.engine", "extract.tika_base_url", "extract.tika_timeout_seconds", "extract.tika_auth_token"],
  docling: ["extract.engine", "extract.docling_base_url", "extract.docling_timeout_seconds", "extract.docling_auth_token"],
  mineru: ["extract.engine", "extract.mineru_source", "extract.mineru_base_url", "extract.mineru_timeout_seconds", "extract.mineru_auth_token"],
  tesseract: ["extract.tesseract_ocr_base_url", "extract.tesseract_ocr_timeout_seconds", "extract.tesseract_ocr_auth_token"],
  rapidocr: ["extract.rapidocr_base_url", "extract.rapidocr_timeout_seconds", "extract.rapidocr_auth_token"],
  embedding: ["file.embedding_enabled", "file.embedding_host", "file.embedding_key", "file.rag_model", "file.embedding_timeout_seconds"],
};

export const INITIAL_SERVICE_STATES: Record<ServiceName, ServiceState> = {
  tika: { data: null, loading: false, action: "" },
  docling: { data: null, loading: false, action: "" },
  mineru: { data: null, loading: false, action: "" },
  tesseract: { data: null, loading: false, action: "" },
  rapidocr: { data: null, loading: false, action: "" },
  embedding: { data: null, loading: false, action: "" },
};

export const SETTINGS_GROUPS: SettingsGroup[] = [
  {
    key: "uploadLimits",
    title: "Upload limits",
    description: "Controls attachment count, MIME allowlist, and file size limits for chat messages.",
    fields: [
      {
        namespace: "storage",
        key: "max_message_files",
        label: "Attachment limit",
        description: "Maximum number of attachments allowed in one chat message.",
        type: "int",
        placeholder: "Enter a count"
      },
      {
        namespace: "file",
        key: "allowed_mime_types",
        label: "Attachment MIME allowlist",
        description: "Allowed attachment MIME types. Separate multiple values with commas.",
        type: "textarea",
        placeholder: "MIME types, comma separated"
      },
      { namespace: "storage",
        key: "max_upload_file_bytes",
        label: "Default size limit",
        description: "Default size limit for a single attachment. The UI uses MB.",
        type: "int",
        valueUnit: "mb",
        placeholder: "Size limit (MB)"
      },
      {
        namespace: "storage",
        key: "user_storage_quota_bytes",
        label: "User storage quota",
        description: "Maximum total file storage per user. The UI uses MB; set 0 for unlimited.",
        type: "int",
        valueUnit: "mb",
        placeholder: "0 for unlimited"
      },
      {
        namespace: "file",
        key: "image_max_bytes",
        label: "Image size limit",
        description: "Override the size limit for image attachments. Falls back to the default attachment limit when empty.",
        type: "int",
        placeholder: "Leave empty to use the default attachment limit",
        valueUnit: "mb",
        subgroupKey: "upload_limits_override",
      },
      {
        namespace: "file",
        key: "doc_max_bytes",
        label: "Document size limit",
        description: "Override the size limit for document attachments. Falls back to the default attachment limit when empty.",
        type: "int",
        placeholder: "Leave empty to use the default attachment limit",
        valueUnit: "mb",
        subgroupKey: "upload_limits_override",
      },
    ],
  },
  {
    key: "extraction",
    title: "Content extraction",
    description: "Controls the primary extraction pipeline, shared OCR engine, and OCR activation scope.",
    fields: [
      {
        namespace: "extract",
        key: "engine",
        label: "Extraction engine",
        description: "Primary engine used for file content extraction.",
        type: "select",
        options: [
          { label: "Built-in", value: EXTRACT_ENGINE_POLICIES.BUILTIN },
          { label: "Tika", value: EXTRACT_ENGINE_POLICIES.TIKA },
          { label: "Docling", value: EXTRACT_ENGINE_POLICIES.DOCLING },
          { label: "MinerU", value: EXTRACT_ENGINE_POLICIES.MINERU },
        ],
      },
      {
        namespace: "extract",
        key: "tika_base_url",
        label: "Tika service URL *",
        description: "Apache Tika service URL.",
        type: "string",
        placeholder: "Service URL",
        visibleWhen: { field: "extract.engine", equals: EXTRACT_ENGINE_POLICIES.TIKA },
        subgroupKey: "tika",
        runtimeService: "tika",
      },
      {
        namespace: "extract",
        key: "docling_base_url",
        label: "Docling service URL *",
        description: "Docling service URL.",
        type: "string",
        placeholder: "Service URL",
        visibleWhen: { field: "extract.engine", equals: EXTRACT_ENGINE_POLICIES.DOCLING },
        subgroupKey: "docling",
        runtimeService: "docling",
      },
      {
        namespace: "extract",
        key: "docling_auth_token",
        label: "Docling auth key",
        description: "Auth key sent when calling the Docling service.",
        type: "password",
        placeholder: "Auth key (optional)",
        visibleWhen: { field: "extract.engine", equals: EXTRACT_ENGINE_POLICIES.DOCLING },
        subgroupKey: "docling",
      },
      {
        namespace: "extract",
        key: "docling_timeout_seconds",
        label: "Docling timeout",
        description: "Maximum wait time for one Docling parsing request, in seconds.",
        type: "int",
        placeholder: "Timeout (seconds)",
        visibleWhen: { field: "extract.engine", equals: EXTRACT_ENGINE_POLICIES.DOCLING },
        subgroupKey: "docling",
      },
      {
        namespace: "extract",
        key: "mineru_source",
        label: "MinerU service type",
        description: "Choose MinerU cloud service or a self-hosted service.",
        type: "tabs",
        options: [
          { label: "Cloud", value: MINERU_SERVICE_SOURCES.CLOUD },
          { label: "Self-hosted", value: MINERU_SERVICE_SOURCES.SELF_HOSTED },
        ],
        visibleWhen: { field: "extract.engine", equals: EXTRACT_ENGINE_POLICIES.MINERU },
        subgroupKey: "mineru",
      },
      {
        namespace: "extract",
        key: "mineru_base_url",
        label: "MinerU service URL *",
        description: "MinerU service URL.",
        type: "string",
        placeholder: "Service URL",
        visibleWhen: { field: "extract.engine", equals: EXTRACT_ENGINE_POLICIES.MINERU },
        subgroupKey: "mineru",
        runtimeService: "mineru",
      },
      {
        namespace: "extract",
        key: "mineru_auth_token",
        label: "MinerU auth key",
        description: "Token or API key used when calling MinerU.",
        type: "password",
        placeholder: "Auth key (optional)",
        visibleWhen: { field: "extract.engine", equals: EXTRACT_ENGINE_POLICIES.MINERU },
        subgroupKey: "mineru",
      },
      {
        namespace: "extract",
        key: "mineru_timeout_seconds",
        label: "MinerU timeout",
        description: "Maximum wait time for one MinerU parsing request, in seconds.",
        type: "int",
        placeholder: "Timeout (seconds)",
        visibleWhen: { field: "extract.engine", equals: EXTRACT_ENGINE_POLICIES.MINERU },
        subgroupKey: "mineru",
      },
      {
        namespace: "extract",
        key: "tika_auth_token",
        label: "Tika auth key",
        description: "Auth key sent when calling the Tika service.",
        type: "password",
        placeholder: "Auth key (optional)",
        visibleWhen: { field: "extract.engine", equals: EXTRACT_ENGINE_POLICIES.TIKA },
        subgroupKey: "tika",
      },
      {
        namespace: "extract",
        key: "tika_timeout_seconds",
        label: "Tika timeout",
        description: "Maximum wait time for one Tika parsing request, in seconds.",
        type: "int",
        placeholder: "Timeout (seconds)",
        visibleWhen: { field: "extract.engine", equals: EXTRACT_ENGINE_POLICIES.TIKA },
        subgroupKey: "tika",
      },
      {
        namespace: "extract",
        key: "image_ocr_enabled",
        label: "Image OCR",
        description: "Send image attachments through OCR and generate text for retrieval.",
        type: "bool",
      },
      {
        namespace: "extract",
        key: "pdf_ocr_fallback_enabled",
        label: "PDF OCR fallback",
        description: "Use OCR fallback when native PDF text extraction fails or has poor quality.",
        type: "bool",
      },
      {
        namespace: "extract",
        key: "ocr_engine",
        label: "OCR engine",
        description: "Shared OCR engine for image OCR and PDF OCR fallback.",
        type: "select",
        options: [
          { label: "Rapid OCR", value: OCR_ENGINES.RAPIDOCR },
          { label: "Tesseract OCR", value: OCR_ENGINES.TESSERACT },
          { label: "Paddle OCR", value: OCR_ENGINES.PADDLE },
          { label: "Tencent Cloud OCR", value: OCR_ENGINES.TENCENT },
          { label: "Alibaba Cloud OCR", value: OCR_ENGINES.ALIYUN },
          { label: "LLM OCR", value: OCR_ENGINES.LLM },
        ],
        visibleWhen: OCR_ENABLED_RULE,
      },
      {
        namespace: "extract",
        key: "tesseract_ocr_base_url",
        label: "Tesseract OCR service URL *",
        description: "Tesseract OCR service URL.",
        type: "string",
        placeholder: "Service URL",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.TESSERACT }] },
        subgroupKey: "tesseract_ocr",
        runtimeService: "tesseract",
      },
      {
        namespace: "extract",
        key: "tesseract_ocr_auth_token",
        label: "Tesseract OCR auth key",
        description: "Auth key sent when calling Tesseract OCR.",
        type: "password",
        placeholder: "Auth key (optional)",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.TESSERACT }] },
        subgroupKey: "tesseract_ocr",
      },
      {
        namespace: "extract",
        key: "tesseract_ocr_timeout_seconds",
        label: "Tesseract OCR timeout",
        description: "Maximum wait time for one Tesseract OCR request, in seconds.",
        type: "int",
        placeholder: "Timeout (seconds)",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.TESSERACT }] },
        subgroupKey: "tesseract_ocr",
      },
      {
        namespace: "extract",
        key: "rapidocr_base_url",
        label: "Rapid OCR service URL *",
        description: "Rapid OCR service URL.",
        type: "string",
        placeholder: "Service URL",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.RAPIDOCR }] },
        subgroupKey: "rapidocr",
        runtimeService: "rapidocr",
      },
      {
        namespace: "extract",
        key: "rapidocr_auth_token",
        label: "Rapid OCR auth key",
        description: "Auth key sent when calling Rapid OCR.",
        type: "password",
        placeholder: "Auth key (optional)",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.RAPIDOCR }] },
        subgroupKey: "rapidocr",
      },
      {
        namespace: "extract",
        key: "rapidocr_timeout_seconds",
        label: "Rapid OCR timeout",
        description: "Maximum wait time for one Rapid OCR request, in seconds.",
        type: "int",
        placeholder: "Timeout (seconds)",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.RAPIDOCR }] },
        subgroupKey: "rapidocr",
      },
      {
        namespace: "extract",
        key: "paddle_ocr_base_url",
        label: "Paddle OCR service URL *",
        description: "Paddle OCR service URL.",
        type: "string",
        placeholder: "Service URL",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.PADDLE }] },
        subgroupKey: "paddle_ocr",
      },
      {
        namespace: "extract",
        key: "paddle_ocr_auth_token",
        label: "Paddle OCR auth key",
        description: "Auth key sent when calling Paddle OCR.",
        type: "password",
        placeholder: "Auth key (optional)",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.PADDLE }] },
        subgroupKey: "paddle_ocr",
      },
      {
        namespace: "extract",
        key: "paddle_ocr_timeout_seconds",
        label: "Paddle OCR timeout",
        description: "Maximum wait time for one Paddle OCR request, in seconds.",
        type: "int",
        placeholder: "Timeout (seconds)",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.PADDLE }] },
        subgroupKey: "paddle_ocr",
      },
      {
        namespace: "extract",
        key: "tencent_ocr_secret_id",
        label: "Tencent Cloud SecretId *",
        description: "SecretId required for Tencent Cloud OCR API.",
        type: "string",
        placeholder: "SecretId",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.TENCENT }] },
        subgroupKey: "tencent_ocr",
      },
      {
        namespace: "extract",
        key: "tencent_ocr_secret_key",
        label: "Tencent Cloud SecretKey *",
        description: "SecretKey required for Tencent Cloud OCR API.",
        type: "password",
        placeholder: "SecretKey",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.TENCENT }] },
        subgroupKey: "tencent_ocr",
      },
      {
        namespace: "extract",
        key: "tencent_ocr_region",
        label: "Tencent Cloud region",
        description: "Request region for Tencent Cloud OCR API.",
        type: "string",
        placeholder: "Region",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.TENCENT }] },
        subgroupKey: "tencent_ocr",
      },
      {
        namespace: "extract",
        key: "tencent_ocr_endpoint",
        label: "Tencent Cloud endpoint",
        description: "Endpoint for Tencent Cloud OCR API.",
        type: "string",
        placeholder: "Endpoint",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.TENCENT }] },
        subgroupKey: "tencent_ocr",
      },
      {
        namespace: "extract",
        key: "tencent_ocr_timeout_seconds",
        label: "Tencent Cloud OCR timeout",
        description: "Maximum wait time for one Tencent Cloud OCR request, in seconds.",
        type: "int",
        placeholder: "Timeout (seconds)",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.TENCENT }] },
        subgroupKey: "tencent_ocr",
      },
      {
        namespace: "extract",
        key: "aliyun_ocr_access_key_id",
        label: "Alibaba Cloud AccessKey ID *",
        description: "AccessKey ID required for Alibaba Cloud OCR API.",
        type: "string",
        placeholder: "AccessKey ID",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.ALIYUN }] },
        subgroupKey: "aliyun_ocr",
      },
      {
        namespace: "extract",
        key: "aliyun_ocr_access_key_secret",
        label: "Alibaba Cloud AccessKey Secret *",
        description: "AccessKey Secret required for Alibaba Cloud OCR API.",
        type: "password",
        placeholder: "AccessKey Secret",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.ALIYUN }] },
        subgroupKey: "aliyun_ocr",
      },
      {
        namespace: "extract",
        key: "aliyun_ocr_region",
        label: "Alibaba Cloud region",
        description: "Request region for Alibaba Cloud OCR API.",
        type: "string",
        placeholder: "Region",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.ALIYUN }] },
        subgroupKey: "aliyun_ocr",
      },
      {
        namespace: "extract",
        key: "aliyun_ocr_endpoint",
        label: "Alibaba Cloud endpoint",
        description: "Endpoint for Alibaba Cloud OCR API.",
        type: "string",
        placeholder: "Endpoint",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.ALIYUN }] },
        subgroupKey: "aliyun_ocr",
      },
      {
        namespace: "extract",
        key: "aliyun_ocr_timeout_seconds",
        label: "Alibaba Cloud OCR timeout",
        description: "Maximum wait time for one Alibaba Cloud OCR request, in seconds.",
        type: "int",
        placeholder: "Timeout (seconds)",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.ALIYUN }] },
        subgroupKey: "aliyun_ocr",
      },
      {
        namespace: "extract",
        key: "llm_ocr_base_url",
        label: "LLM OCR service URL *",
        description: "OpenAI-compatible vision model service URL.",
        type: "string",
        placeholder: "Service URL",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.LLM }] },
        subgroupKey: "llm_ocr",
      },
      {
        namespace: "extract",
        key: "llm_ocr_auth_token",
        label: "LLM OCR auth key",
        description: "API key used when calling the LLM OCR service.",
        type: "password",
        placeholder: "API Key (optional)",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.LLM }] },
        subgroupKey: "llm_ocr",
      },
      {
        namespace: "extract",
        key: "llm_ocr_model",
        label: "LLM OCR model *",
        description: "Vision model name used for OCR.",
        type: "string",
        placeholder: "Model name",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.LLM }] },
        subgroupKey: "llm_ocr",
      },
      {
        namespace: "extract",
        key: "llm_ocr_timeout_seconds",
        label: "LLM OCR timeout",
        description: "Maximum wait time for one LLM OCR request, in seconds.",
        type: "int",
        placeholder: "Timeout (seconds)",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.LLM }] },
        subgroupKey: "llm_ocr",
      },
      {
        namespace: "extract",
        key: "llm_ocr_prompt",
        label: "LLM OCR prompt",
        description: "OCR instruction sent to the vision model.",
        type: "textarea",
        placeholder: "Prompt (optional)",
        visibleWhen: { all: [OCR_ENABLED_RULE, { field: "extract.ocr_engine", equals: OCR_ENGINES.LLM }] },
        subgroupKey: "llm_ocr",
      },
    ],
  },
  {
    key: "fullContext",
    title: "Full-text injection",
    description: "Controls whether extracted file text can be injected directly into context instead of using RAG.",
    fields: [
      {
        namespace: "file",
        key: "full_context_limit_enabled",
        label: "Full-text injection limits",
        description: "Enable byte, token, and PDF page limits for full-text injection.",
        type: "select",
        options: [
          { label: "Off", value: FULL_CONTEXT_LIMIT_MODES.OFF },
          { label: "On", value: FULL_CONTEXT_LIMIT_MODES.ON },
        ],
      },
      {
        namespace: "file",
        key: "file_full_context_max_bytes",
        label: "Full-text byte limit",
        description: "Maximum extracted text bytes allowed for full-context injection. Leave empty or use 0 for no byte limit.",
        type: "int",
        placeholder: "Empty or 0 means unlimited",
        visibleWhen: { field: "file.full_context_limit_enabled", equals: FULL_CONTEXT_LIMIT_MODES.ON },
      },
      {
        namespace: "file",
        key: "full_context_max_tokens",
        label: "Full-text token limit",
        description: "Maximum token budget allowed for full-context injection. Leave empty or use 0 for no token limit.",
        type: "int",
        placeholder: "Empty or 0 means unlimited",
        visibleWhen: { field: "file.full_context_limit_enabled", equals: FULL_CONTEXT_LIMIT_MODES.ON },
      },
      {
        namespace: "file",
        key: "full_context_pdf_max_pages",
        label: "Full-text page limit",
        description: "Maximum PDF pages allowed for full-context injection. Leave empty or use 0 for no page limit.",
        type: "int",
        placeholder: "Empty or 0 means unlimited",
        visibleWhen: { field: "file.full_context_limit_enabled", equals: FULL_CONTEXT_LIMIT_MODES.ON },
      },
    ],
  },
  {
    key: "embedding",
    title: "Vector retrieval",
    description: "Configures whether the external Embedding service is enabled. Changing the model marks old vectors stale.",
    fields: [
      {
        namespace: "file",
        key: "embedding_enabled",
        label: "Embedding",
        description: "Enable file vectorization and vector retrieval. Related pipelines are disabled when off.",
        type: "select",
        options: [
          { label: "Off", value: EMBEDDING_MODES.OFF },
          { label: "On", value: EMBEDDING_MODES.ON },
        ],
      },
      {
        namespace: "file",
        key: "embedding_host",
        label: "Embedding service URL *",
        description: "Embedding service URL. The service must be compatible with the OpenAI /embeddings API.",
        type: "string",
        placeholder: "Service URL",
        visibleWhen: { field: "file.embedding_enabled", equals: EMBEDDING_MODES.ON },
        subgroupKey: "embedding_service",
        subgroupTitle: "Service configuration",
        subgroupDescription: "The Embedding service uses an OpenAI-compatible /embeddings API.",
        runtimeService: "embedding",
      },
      {
        namespace: "file",
        key: "embedding_key",
        label: "Embedding auth key",
        description: "Auth key used when calling the Embedding service.",
        type: "password",
        placeholder: "Auth key",
        visibleWhen: { field: "file.embedding_enabled", equals: EMBEDDING_MODES.ON },
        subgroupKey: "embedding_service",
      },
      {
        namespace: "file",
        key: "rag_model",
        label: "Embedding model *",
        description: "Model name used for vectorization.",
        type: "string",
        placeholder: "Model name",
        visibleWhen: { field: "file.embedding_enabled", equals: EMBEDDING_MODES.ON },
        subgroupKey: "embedding_service",
      },
      {
        namespace: "file",
        key: "embedding_timeout_seconds",
        label: "Embedding timeout",
        description: "Maximum wait time for one Embedding API request, in seconds.",
        type: "int",
        placeholder: "Timeout (seconds)",
        visibleWhen: { field: "file.embedding_enabled", equals: EMBEDDING_MODES.ON },
        subgroupKey: "embedding_service",
      },
      {
        namespace: "file",
        key: "embedding_output_dimensions",
        label: "Vector dimensions",
        description: "Vector dimensions used consistently for storage and retrieval.",
        type: "int",
        placeholder: "Vector dimensions",
        visibleWhen: { field: "file.embedding_enabled", equals: EMBEDDING_MODES.ON },
      },
      {
        namespace: "file",
        key: "embedding_normalize",
        label: "Normalize vectors",
        description: "Normalize vectors before storage and retrieval.",
        type: "bool",
        visibleWhen: { field: "file.embedding_enabled", equals: EMBEDDING_MODES.ON },
      },
      {
        namespace: "file",
        key: "embed_trigger_on_upload",
        label: "Auto-vectorize",
        description: "Trigger Embedding asynchronously after file upload completes.",
        type: "bool",
        visibleWhen: { field: "file.embedding_enabled", equals: EMBEDDING_MODES.ON },
      },
      {
        namespace: "file",
        key: "embed_batch_size",
        label: "Batch size",
        description: "Number of text chunks sent in one Embedding API request.",
        type: "int",
        placeholder: "Batch size",
        visibleWhen: { field: "file.embedding_enabled", equals: EMBEDDING_MODES.ON },
      },
      {
        namespace: "file",
        key: "embed_chunk_size_tokens",
        label: "Chunk size",
        description: "Document vectorization chunk size, estimated by tokens.",
        type: "int",
        placeholder: "Chunk size (tokens)",
        visibleWhen: { field: "file.embedding_enabled", equals: EMBEDDING_MODES.ON },
      },
      {
        namespace: "file",
        key: "embed_chunk_overlap_tokens",
        label: "Chunk overlap",
        description: "Token overlap between adjacent chunks.",
        type: "int",
        placeholder: "Chunk overlap (tokens)",
        visibleWhen: { field: "file.embedding_enabled", equals: EMBEDDING_MODES.ON },
      },
    ],
  },
  {
    key: "semantic",
    title: "Semantic enhancement",
    description: "Semantic recall for historical messages based on vector retrieval. Requires message embedding to be ready.",
    fields: [
      {
        namespace: "chat",
        key: "message_embedding_enabled",
        label: "Message embedding",
        description: "Generate message vectors asynchronously after each conversation turn. Requires an available Embedding service.",
        type: "bool",
      },
      {
        namespace: "chat",
        key: "semantic_context_enabled",
        label: "Semantic context recall",
        description: "Recall historically relevant snippets and inject them into context before sending a message.",
        type: "bool",
      },
    ],
  },
  {
    key: "rag",
    title: "RAG",
    description: "Controls retrieval, similarity thresholds, injection budget, and readiness wait strategy.",
    fields: [
      { namespace: "chat", key: "rag_enabled", label: "RAG", description: "Allow files to enter the retrieval-augmented pipeline. Requires an available Embedding service.", type: "bool" },
      { namespace: "file", key: "rag_top_k", label: "Retrieved chunks", description: "Number of RAG chunks returned and injected.", type: "int", placeholder: "Chunk count", visibleWhen: { all: [EMBEDDING_READY_RULE, { field: "chat.rag_enabled", equals: "true" }] } },
      { namespace: "chat", key: "rag_min_similarity", label: "Similarity threshold", description: "Minimum similarity threshold for injecting retrieved chunks.", type: "string", placeholder: "Similarity threshold", visibleWhen: { all: [EMBEDDING_READY_RULE, { field: "chat.rag_enabled", equals: "true" }] } },
      { namespace: "chat", key: "rag_token_budget", label: "Injected token budget", description: "Token budget available for injecting retrieved chunks into context.", type: "int", placeholder: "Token budget", visibleWhen: { all: [EMBEDDING_READY_RULE, { field: "chat.rag_enabled", equals: "true" }] } },
      { namespace: "chat", key: "rag_fetch_multiplier", label: "Fetch multiplier", description: "Candidate fetch multiplier. The system fetches more candidates before reranking and filtering.", type: "int", placeholder: "Fetch multiplier", visibleWhen: { all: [EMBEDDING_READY_RULE, { field: "chat.rag_enabled", equals: "true" }] } },
      { namespace: "chat", key: "rag_wait_ready_ms", label: "Ready wait", description: "Maximum wait time for vectorization readiness before sending a request, in milliseconds.", type: "int", placeholder: "Wait time (ms)", visibleWhen: { all: [EMBEDDING_READY_RULE, { field: "chat.rag_enabled", equals: "true" }] } },
      { namespace: "chat", key: "rag_query_history_turns", label: "Query history turns", description: "Recent user turns included when generating the retrieval query.", type: "int", placeholder: "History turns", visibleWhen: { all: [EMBEDDING_READY_RULE, { field: "chat.rag_enabled", equals: "true" }] } },
      { namespace: "chat", key: "rag_retrieval_cache_ttl_seconds", label: "Retrieval cache TTL", description: "RAG retrieval result cache duration, in seconds.", type: "int", placeholder: "Cache duration (seconds)", visibleWhen: { all: [EMBEDDING_READY_RULE, { field: "chat.rag_enabled", equals: "true" }] } },
      { namespace: "chat", key: "rag_hybrid_enabled", label: "Hybrid retrieval (BM25 + vector)", description: "Run vector retrieval and full-text retrieval in parallel, then merge results with RRF.", type: "bool", visibleWhen: { all: [EMBEDDING_READY_RULE, { field: "chat.rag_enabled", equals: "true" }] } },
    ],
  },
  {
    key: "contextCompression",
    title: "Context compression",
    description: "Controls context truncation, compression triggers, summary generation, and fallback behavior.",
    fields: [
      {
        namespace: "chat",
        key: "context_token_budget_enabled",
        label: "Token budget truncation",
        description: "Automatically compute budget from the current model context window and trim sendable history.",
        type: "bool",
      },
      {
        namespace: "chat",
        key: "context_max_turns",
        label: "Compression trigger turns",
        description: "Turn-count threshold for triggering context compression.",
        type: "int",
        placeholder: "Trigger turns",
      },
      {
        namespace: "chat",
        key: "context_compact_trigger_tokens",
        label: "Compression token threshold",
        description: "Token-scale threshold for triggering context compression.",
        type: "int",
        placeholder: "Token threshold",
      },
      {
        namespace: "chat",
        key: "context_compact_preserve_recent_turns",
        label: "Recent turns to preserve",
        description: "Recent original turns to keep during compression. Older content is summarized.",
        type: "int",
        placeholder: "Turns to preserve",
      },
      {
        namespace: "chat",
        key: "context_compact_highlights_per_role",
        label: "Highlights per role",
        description: "Maximum summary highlights retained for each user and assistant role during template compression.",
        type: "int",
        placeholder: "Highlight count",
      },
      {
        namespace: "chat",
        key: "context_compact_snippet_chars",
        label: "Snippet character limit",
        description: "Maximum characters retained in one message snippet during template compression.",
        type: "int",
        placeholder: "Character limit",
      },
      {
        namespace: "chat",
        key: "context_artifact_retention_days",
        label: "Evidence retention days",
        description: "Retention period for context evidence, including RAG results, historical evidence, and tool results. 0 disables automatic expiry.",
        type: "int",
        placeholder: "Retention days",
      },
      {
        namespace: "chat",
        key: "compact_llm_enabled",
        label: "LLM compression",
        description: "Call a model to generate semantic summaries. When off, only template summaries are used.",
        type: "bool",
        subgroupKey: "compact_llm",
        subgroupTitle: "LLM compression policy",
        subgroupDescription: "Controls summary model calls, execution mode, and failure fallback.",
      },
      {
        namespace: "chat",
        key: "compact_async_enabled",
        label: "Async compression",
        description: "Run compression tasks asynchronously in the background to reduce blocking in the current response pipeline.",
        type: "bool",
        subgroupKey: "compact_llm",
        visibleWhen: { field: "chat.compact_llm_enabled", equals: "true" },
      },
      {
        namespace: "chat",
        key: "compact_task_model",
        label: "Compression model",
        description: "Chat model used to generate compression summaries. Follow current model falls back to the default chat model when the active conversation model is not chat-capable.",
        type: "select",
        options: [{ label: "Follow current model", value: TASK_MODEL_FOLLOW }],
        visibleWhen: { field: "chat.compact_llm_enabled", equals: "true" },
        subgroupKey: "compact_llm",
      },
      {
        namespace: "chat",
        key: "compact_max_failures",
        label: "Circuit threshold",
        description: "Consecutive failure threshold for LLM compression. After reaching it, the system falls back to template summaries.",
        type: "int",
        placeholder: "Circuit threshold",
        subgroupKey: "compact_llm",
        visibleWhen: { field: "chat.compact_llm_enabled", equals: "true" },
      },
      {
        namespace: "chat",
        key: "compact_system_prompt",
        label: "Full summary prompt",
        description: "Custom instruction sent to the LLM for full compression. Leave empty to use the built-in default.",
        type: "textarea",
        placeholder: "Leave empty to use the default prompt",
        subgroupKey: "compact_llm",
        visibleWhen: { field: "chat.compact_llm_enabled", equals: "true" },
      },
      {
        namespace: "chat",
        key: "compact_light_prompt",
        label: "Light summary prompt",
        description: "Custom instruction sent to the LLM for light compression. Leave empty to use the built-in default.",
        type: "textarea",
        placeholder: "Leave empty to use the default prompt",
        subgroupKey: "compact_llm",
        visibleWhen: { field: "chat.compact_llm_enabled", equals: "true" },
      },
    ],
  },
];

export function usesTika(policy: string): boolean {
  return policy === EXTRACT_ENGINE_POLICIES.TIKA;
}

export function resolveOCREngine(engine: string): string {
  switch (engine) {
    case OCR_ENGINES.TESSERACT:
      return OCR_ENGINES.TESSERACT;
    case OCR_ENGINES.PADDLE:
      return OCR_ENGINES.PADDLE;
    case OCR_ENGINES.TENCENT:
      return OCR_ENGINES.TENCENT;
    case OCR_ENGINES.ALIYUN:
      return OCR_ENGINES.ALIYUN;
    case OCR_ENGINES.LLM:
      return OCR_ENGINES.LLM;
    default:
      return OCR_ENGINES.RAPIDOCR;
  }
}

export function resolveMinerUSource(source: string): string {
  return source === MINERU_SERVICE_SOURCES.SELF_HOSTED ? MINERU_SERVICE_SOURCES.SELF_HOSTED : MINERU_SERVICE_SOURCES.CLOUD;
}

export function resolveMinerUDefaultBaseURL(source: string): string {
  return resolveMinerUSource(source) === MINERU_SERVICE_SOURCES.SELF_HOSTED
    ? "http://127.0.0.1:8000"
    : "https://mineru.net/api/v4";
}

export function resolveActiveServices(settings: Record<string, string>): Set<ServiceName> {
  const active = new Set<ServiceName>();
  const engine = settings["extract.engine"] ?? "";
  if (engine === EXTRACT_ENGINE_POLICIES.TIKA) active.add("tika");
  if (engine === EXTRACT_ENGINE_POLICIES.DOCLING) active.add("docling");
  if (engine === EXTRACT_ENGINE_POLICIES.MINERU) active.add("mineru");
  const ocrEnabled = matchesVisibilityRule(OCR_ENABLED_RULE, settings);
  const ocr = ocrEnabled ? resolveOCREngine(settings["extract.ocr_engine"] ?? "") : "";
  if (ocr === OCR_ENGINES.TESSERACT) active.add("tesseract");
  if (ocr === OCR_ENGINES.RAPIDOCR) active.add("rapidocr");
  if (settings["file.embedding_enabled"] === EMBEDDING_MODES.ON) active.add("embedding");
  return active;
}

export function isServiceDirty(
  name: ServiceName,
  settingsMap: Record<string, string>,
  savedMap: Record<string, string>,
): boolean {
  const dirty = SERVICE_DIRTY_FIELDS[name].some((field) => (settingsMap[field] ?? "") !== (savedMap[field] ?? ""));
  const ocrModeChanged =
    resolveOCREngine(settingsMap["extract.ocr_engine"] ?? "") !==
    resolveOCREngine(savedMap["extract.ocr_engine"] ?? "") ||
    (settingsMap["extract.image_ocr_enabled"] ?? "") !== (savedMap["extract.image_ocr_enabled"] ?? "") ||
    (settingsMap["extract.pdf_ocr_fallback_enabled"] ?? "") !== (savedMap["extract.pdf_ocr_fallback_enabled"] ?? "");
  return name === "tesseract" || name === "rapidocr" ? dirty || ocrModeChanged : dirty;
}

export function resolveFieldID(field: SettingsField): string {
  return `${field.namespace}.${field.key}`;
}

export function isSettingsValueField(field: SettingsField): field is SettingsValueField {
  return true;
}

export function matchesVisibilityRule(rule: VisibilityRule | undefined, settings: Record<string, string>): boolean {
  if (!rule) {
    return true;
  }
  if ("all" in rule) {
    return rule.all.every((item) => matchesVisibilityRule(item, settings));
  }
  if ("any" in rule) {
    return rule.any.some((item) => matchesVisibilityRule(item, settings));
  }

  const currentValue = settings[rule.field] ?? "";
  if ("equals" in rule) {
    return currentValue === rule.equals;
  }
  if ("notEquals" in rule) {
    return currentValue !== rule.notEquals;
  }
  if ("nonEmpty" in rule) {
    return currentValue.trim().length > 0;
  }
  if ("in" in rule) {
    return rule.in.includes(currentValue);
  }
  return true;
}

export function resolveVisibleFields(group: SettingsGroup, settings: Record<string, string>) {
  return group.fields.filter((field) => matchesVisibilityRule(field.visibleWhen, settings));
}

export function resolveVisibleFieldBlocks(group: SettingsGroup, settings: Record<string, string>): VisibleFieldBlock[] {
  const visibleFields = resolveVisibleFields(group, settings);
  const blocks: VisibleFieldBlock[] = [];
  for (const field of visibleFields) {
    if (!field.subgroupKey) {
      blocks.push({ kind: "field", field });
      continue;
    }
    const last = blocks[blocks.length - 1];
    if (last?.kind === "subgroup" && last.key === field.subgroupKey) {
      last.fields.push(field);
      continue;
    }
    blocks.push({
      kind: "subgroup",
      key: field.subgroupKey,
      title: field.subgroupTitle ?? field.subgroupKey,
      description: field.subgroupDescription,
      fields: [field],
    });
  }
  return blocks;
}

const BYTES_PER_MB = 1024 * 1024;

export function normalizeMBValue(raw: string): string {
  const trimmed = raw.trim();
  if (!trimmed) {
    return "";
  }
  const bytes = Number(trimmed);
  if (!Number.isFinite(bytes)) {
    return trimmed;
  }
  const mb = bytes / BYTES_PER_MB;
  return Number.isInteger(mb) ? String(mb) : String(Number(mb.toFixed(2)));
}

export function denormalizeMBValue(raw: string): string {
  const trimmed = raw.trim();
  if (!trimmed) {
    return "";
  }
  const mb = Number(trimmed);
  if (!Number.isFinite(mb)) {
    return trimmed;
  }
  return String(Math.round(mb * BYTES_PER_MB));
}

export function flattenSettings(groups: SettingsGroup[], grouped: SettingsGrouped): Record<string, string> {
  const result: Record<string, string> = {};
  const fieldMap = new Map<string, SettingsValueField>();
  for (const group of groups) {
    for (const field of group.fields) {
      fieldMap.set(resolveFieldID(field), field);
    }
  }
  for (const [namespace, items] of Object.entries(grouped)) {
    for (const item of items) {
      const fieldID = `${namespace}.${item.key}`;
      const field = fieldMap.get(fieldID);
      const rawValue = item.value ?? "";
      result[fieldID] = field?.valueUnit === "mb" ? normalizeMBValue(rawValue) : rawValue;
    }
  }
  return result;
}

export function resolveErrorMessage(error: unknown, fallback?: string): string {
  return resolveLocalizedErrorMessage(error, fallback);
}

export function isEmbeddingServiceConfigured(settings: Record<string, string>): boolean {
  return (
    settings["file.embedding_enabled"] === EMBEDDING_MODES.ON &&
    Boolean((settings["file.rag_model"] ?? "").trim()) &&
    Boolean((settings["file.embedding_host"] ?? "").trim())
  );
}

export function applySettingsDefaults(next: Record<string, string>): Record<string, string> {
  const result = { ...next };
  result["extract.tika_source"] = TIKA_SERVICE_SOURCES.EXTERNAL;
  result["extract.rapidocr_source"] = TIKA_SERVICE_SOURCES.EXTERNAL;
  result["extract.ocr_engine"] = resolveOCREngine(result["extract.ocr_engine"] ?? "");
  if (!["true", "false"].includes(result["extract.image_ocr_enabled"] ?? "")) {
    result["extract.image_ocr_enabled"] = "false";
  }
  if (!["true", "false"].includes(result["extract.pdf_ocr_fallback_enabled"] ?? "")) {
    result["extract.pdf_ocr_fallback_enabled"] = "false";
  }
  if (!["true", "false"].includes(result["file.embedding_enabled"] ?? "")) {
    result["file.embedding_enabled"] = EMBEDDING_MODES.OFF;
  }
  if (!["true", "false"].includes(result["file.full_context_limit_enabled"] ?? "")) {
    result["file.full_context_limit_enabled"] = FULL_CONTEXT_LIMIT_MODES.ON;
  }
  if (!["true", "false"].includes(result["chat.rag_enabled"] ?? "")) {
    result["chat.rag_enabled"] = "false";
  }
  if (!["true", "false"].includes(result["chat.message_embedding_enabled"] ?? "")) {
    result["chat.message_embedding_enabled"] = "false";
  }
  if (!["true", "false"].includes(result["chat.semantic_context_enabled"] ?? "")) {
    result["chat.semantic_context_enabled"] = "false";
  }
  if (usesTika(result["extract.engine"] ?? "")) {
    if (!(result["extract.tika_timeout_seconds"] ?? "").trim()) {
      result["extract.tika_timeout_seconds"] = "60";
    }
  }
  if ((result["extract.engine"] ?? "") === EXTRACT_ENGINE_POLICIES.DOCLING) {
    if (!(result["extract.docling_timeout_seconds"] ?? "").trim()) {
      result["extract.docling_timeout_seconds"] = "60";
    }
  }
  if ((result["extract.engine"] ?? "") === EXTRACT_ENGINE_POLICIES.MINERU) {
    result["extract.mineru_source"] = resolveMinerUSource(result["extract.mineru_source"] ?? "");
    if (!(result["extract.mineru_timeout_seconds"] ?? "").trim()) {
      result["extract.mineru_timeout_seconds"] = "180";
    }
  }
  const ocrEngine = resolveOCREngine(result["extract.ocr_engine"] ?? "");
  if (ocrEngine === OCR_ENGINES.TESSERACT) {
    if (!(result["extract.tesseract_ocr_timeout_seconds"] ?? "").trim()) {
      result["extract.tesseract_ocr_timeout_seconds"] = "60";
    }
  }
  if (ocrEngine === OCR_ENGINES.RAPIDOCR) {
    if (!(result["extract.rapidocr_timeout_seconds"] ?? "").trim()) {
      result["extract.rapidocr_timeout_seconds"] = "60";
    }
  }
  if (ocrEngine === OCR_ENGINES.PADDLE) {
    if (!(result["extract.paddle_ocr_timeout_seconds"] ?? "").trim()) {
      result["extract.paddle_ocr_timeout_seconds"] = "60";
    }
  }
  if (ocrEngine === OCR_ENGINES.TENCENT) {
    if (!(result["extract.tencent_ocr_region"] ?? "").trim()) {
      result["extract.tencent_ocr_region"] = "ap-guangzhou";
    }
    if (!(result["extract.tencent_ocr_endpoint"] ?? "").trim()) {
      result["extract.tencent_ocr_endpoint"] = "ocr.tencentcloudapi.com";
    }
    if (!(result["extract.tencent_ocr_timeout_seconds"] ?? "").trim()) {
      result["extract.tencent_ocr_timeout_seconds"] = "60";
    }
  }
  if (ocrEngine === OCR_ENGINES.ALIYUN) {
    if (!(result["extract.aliyun_ocr_region"] ?? "").trim()) {
      result["extract.aliyun_ocr_region"] = "cn-hangzhou";
    }
    if (!(result["extract.aliyun_ocr_endpoint"] ?? "").trim()) {
      result["extract.aliyun_ocr_endpoint"] = "ocr-api.cn-hangzhou.aliyuncs.com";
    }
    if (!(result["extract.aliyun_ocr_timeout_seconds"] ?? "").trim()) {
      result["extract.aliyun_ocr_timeout_seconds"] = "60";
    }
  }
  if (ocrEngine === OCR_ENGINES.LLM) {
    if (!(result["extract.llm_ocr_timeout_seconds"] ?? "").trim()) {
      result["extract.llm_ocr_timeout_seconds"] = "60";
    }
  }
  if (result["file.embedding_enabled"] === EMBEDDING_MODES.ON) {
    if (!(result["file.embedding_timeout_seconds"] ?? "").trim()) {
      result["file.embedding_timeout_seconds"] = "60";
    }
    if (!(result["file.embedding_output_dimensions"] ?? "").trim()) {
      result["file.embedding_output_dimensions"] = "1536";
    }
    if (!["true", "false"].includes(result["file.embedding_normalize"] ?? "")) {
      result["file.embedding_normalize"] = "true";
    }
    if (!["true", "false"].includes(result["file.embed_trigger_on_upload"] ?? "")) {
      result["file.embed_trigger_on_upload"] = "true";
    }
    if (!(result["file.embed_batch_size"] ?? "").trim()) {
      result["file.embed_batch_size"] = "20";
    }
    if (!(result["file.embed_chunk_size_tokens"] ?? "").trim()) {
      result["file.embed_chunk_size_tokens"] = "1024";
    }
    if (!(result["file.embed_chunk_overlap_tokens"] ?? "").trim()) {
      result["file.embed_chunk_overlap_tokens"] = "64";
    }
  }
  if (!isEmbeddingServiceConfigured(result)) {
    result["chat.rag_enabled"] = "false";
    result["chat.message_embedding_enabled"] = "false";
    result["chat.semantic_context_enabled"] = "false";
  }
  if (result["chat.message_embedding_enabled"] !== "true") {
    result["chat.semantic_context_enabled"] = "false";
  }
  if (result["chat.rag_enabled"] === "true") {
    if (!(result["file.rag_top_k"] ?? "").trim()) {
      result["file.rag_top_k"] = "5";
    }
    if (!(result["chat.rag_min_similarity"] ?? "").trim()) {
      result["chat.rag_min_similarity"] = "0.45";
    }
    if (!(result["chat.rag_token_budget"] ?? "").trim()) {
      result["chat.rag_token_budget"] = "2000";
    }
    if (!(result["chat.rag_fetch_multiplier"] ?? "").trim()) {
      result["chat.rag_fetch_multiplier"] = "3";
    }
    if (!(result["chat.rag_wait_ready_ms"] ?? "").trim()) {
      result["chat.rag_wait_ready_ms"] = "3000";
    }
    if (!(result["chat.rag_query_history_turns"] ?? "").trim()) {
      result["chat.rag_query_history_turns"] = "0";
    }
    if (!(result["chat.rag_retrieval_cache_ttl_seconds"] ?? "").trim()) {
      result["chat.rag_retrieval_cache_ttl_seconds"] = "120";
    }
    if (!["true", "false"].includes(result["chat.rag_hybrid_enabled"] ?? "")) {
      result["chat.rag_hybrid_enabled"] = "false";
    }
  }
  if (!(result["chat.compact_task_model"] ?? "").trim()) {
    result["chat.compact_task_model"] = TASK_MODEL_FOLLOW;
  }
  if (!(result["chat.context_artifact_retention_days"] ?? "").trim()) {
    result["chat.context_artifact_retention_days"] = "90";
  }
  return result;
}
