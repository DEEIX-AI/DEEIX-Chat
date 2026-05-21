import type { ChatFilePolicyDTO } from "@/shared/api/file.types";

export type UploadCategory = "image" | "pdf" | "word" | "excel" | "text" | "unknown";

const TEXT_FILE_EXTENSIONS = [
  "txt",
  "js",
  "ts",
  "tsx",
  "jsx",
  "py",
  "go",
  "rs",
  "java",
  "sql",
  "yaml",
  "yml",
  "toml",
  "sh",
  "html",
  "htm",
  "css",
  "ini",
  "conf",
] as const;

function resolveFileExtension(fileName: string): string {
  const normalizedName = fileName.trim().toLowerCase();
  return normalizedName.includes(".") ? normalizedName.split(".").pop() || "" : "";
}

export function normalizeUploadMime(file: File): string {
  const mime = file.type.trim().toLowerCase();
  const ext = resolveFileExtension(file.name);

  if (mime.startsWith("image/")) {
    return mime;
  }

  switch (ext) {
    case "pdf":
      return "application/pdf";
    case "docx":
      return "application/vnd.openxmlformats-officedocument.wordprocessingml.document";
    case "doc":
      return "application/msword";
    case "xlsx":
      return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet";
    case "xls":
      return "application/vnd.ms-excel";
    case "csv":
      return "text/csv";
    case "md":
    case "markdown":
      return "text/markdown";
    case "json":
      return "application/json";
    case "xml":
      return "application/xml";
  }

  if (mime) {
    return mime;
  }

  if (TEXT_FILE_EXTENSIONS.includes(ext as (typeof TEXT_FILE_EXTENSIONS)[number])) {
    return "text/plain";
  }

  return "";
}

export function isAllowedUploadMime(file: File, policy: ChatFilePolicyDTO | null): boolean {
  if (!policy || policy.allowedMIMETypes.length === 0) {
    return true;
  }

  const mime = normalizeUploadMime(file);
  return Boolean(mime && policy.allowedMIMETypes.includes(mime));
}

export function inferUploadCategory(file: File): UploadCategory {
  const mime = file.type.trim().toLowerCase();
  const ext = resolveFileExtension(file.name);

  if (mime.startsWith("image/")) {
    return "image";
  }
  if (mime === "application/pdf" || ext === "pdf") {
    return "pdf";
  }
  if (mime.includes("wordprocessingml") || mime.includes("msword") || ext === "docx" || ext === "doc") {
    return "word";
  }
  if (
    mime.includes("spreadsheetml") ||
    mime.includes("ms-excel") ||
    mime === "text/csv" ||
    ext === "xlsx" ||
    ext === "xls" ||
    ext === "csv"
  ) {
    return "excel";
  }
  if (mime.startsWith("text/") || ["json", "xml", "md", "markdown", ...TEXT_FILE_EXTENSIONS].includes(ext)) {
    return "text";
  }

  return "unknown";
}

export function resolveEffectiveUploadLimit(policy: ChatFilePolicyDTO | null, category: UploadCategory): number {
  if (!policy) {
    return 0;
  }

  if (category === "image") {
    return policy.effectiveImageMaxBytes || policy.imageMaxBytes || policy.maxUploadFileBytes;
  }

  return policy.effectiveDocMaxBytes || policy.docMaxBytes || policy.maxUploadFileBytes;
}

type UploadPolicyRejectionLabels = {
  mimeNotAllowed: string;
  fullContextLimitExceeded: (limitKB: number) => string;
  sizeLimitExceeded: (limitKB: number) => string;
};

const DEFAULT_UPLOAD_POLICY_REJECTION_LABELS: UploadPolicyRejectionLabels = {
  mimeNotAllowed: "This file type is not included in the admin MIME allowlist.",
  fullContextLimitExceeded: (limitKB) => `Vector retrieval is disabled. Only small files that fit full-context injection can be uploaded, and this file exceeds the ${limitKB} KB limit.`,
  sizeLimitExceeded: (limitKB) => `This file exceeds the ${limitKB} KB limit.`,
};

export function resolveUploadPolicyRejection(
  file: File,
  policy: ChatFilePolicyDTO | null,
  labels: UploadPolicyRejectionLabels = DEFAULT_UPLOAD_POLICY_REJECTION_LABELS,
): string | null {
  if (!policy) {
    return null;
  }

  const category = inferUploadCategory(file);

  if (!isAllowedUploadMime(file, policy)) {
    return labels.mimeNotAllowed;
  }

  const limit = resolveEffectiveUploadLimit(policy, category);
  if (limit > 0 && file.size > limit) {
    const limitKB = Math.round(limit / 1024);
    if (policy.capabilityMode === "full_context_only" && category !== "image") {
      return labels.fullContextLimitExceeded(limitKB);
    }
    return labels.sizeLimitExceeded(limitKB);
  }

  return null;
}
