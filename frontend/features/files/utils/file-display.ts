import {
  FileArchive,
  FileAudio2,
  FileCode2,
  FileImage,
  FileSpreadsheet,
  FileText,
  FileType2,
  FileVideo2,
  type LucideIcon,
} from "lucide-react";

import type { FileObjectDTO } from "@/shared/api/file.types";

import type { FileFilterKey, FileFilterOption, FilePreviewKind, FileSortOption } from "@/features/files/types/files";

export const FILE_FILTER_OPTIONS: FileFilterOption[] = [
  { value: "all", icon: FileArchive },
  { value: "image", icon: FileImage },
  { value: "audio", icon: FileAudio2 },
  { value: "video", icon: FileVideo2 },
  { value: "document", icon: FileText },
  { value: "spreadsheet", icon: FileSpreadsheet },
  { value: "presentation", icon: FileType2 },
  { value: "pdf", icon: FileText },
  { value: "code", icon: FileCode2 },
];

export const FILE_SORT_OPTIONS: FileSortOption[] = [
  { value: "size" },
  { value: "name" },
  { value: "created" },
  { value: "last_used" },
];

const MARKDOWN_EXTENSIONS = new Set(["md", "mdx", "markdown"]);
const CODE_EXTENSIONS = new Set([
  "c",
  "cc",
  "cpp",
  "css",
  "go",
  "h",
  "html",
  "java",
  "js",
  "json",
  "jsx",
  "mjs",
  "py",
  "rb",
  "rs",
  "sh",
  "sql",
  "svg",
  "toml",
  "ts",
  "tsx",
  "xml",
  "yaml",
  "yml",
]);
const PLAIN_TEXT_EXTENSIONS = new Set(["txt", "log", "csv"]);
const IMAGE_EXTENSIONS = new Set(["png", "jpg", "jpeg", "gif", "webp", "bmp", "heic", "heif"]);
const DOCUMENT_EXTENSIONS = new Set(["doc", "docx", "rtf", "odt", "pages"]);
const SPREADSHEET_EXTENSIONS = new Set(["xls", "xlsx", "csv", "ods", "numbers"]);
const PRESENTATION_EXTENSIONS = new Set(["ppt", "pptx", "odp", "key"]);

function isDocxMimeType(mimeType: string): boolean {
  return mimeType.includes("officedocument.wordprocessingml.document");
}

function isSpreadsheetMimeType(mimeType: string): boolean {
  return mimeType.includes("officedocument.spreadsheetml") || mimeType.includes("vnd.ms-excel");
}

function isOfficeDocumentMimeType(mimeType: string): boolean {
  return (
    mimeType.includes("msword") ||
    isDocxMimeType(mimeType) ||
    isSpreadsheetMimeType(mimeType) ||
    mimeType.includes("officedocument.presentationml") ||
    mimeType.includes("vnd.ms-powerpoint") ||
    mimeType.includes("rtf") ||
    mimeType.includes("opendocument.text") ||
    mimeType.includes("opendocument.spreadsheet") ||
    mimeType.includes("opendocument.presentation")
  );
}

export function resolveFileExtension(fileName: string): string {
  const normalized = fileName.trim().toLowerCase();
  const segments = normalized.split(".");
  return segments.length > 1 ? segments[segments.length - 1] || "" : "";
}

export function formatBytes(sizeBytes: number): string {
  if (!Number.isFinite(sizeBytes) || sizeBytes <= 0) {
    return "0 B";
  }

  const units = ["B", "KB", "MB", "GB", "TB"];
  const exponent = Math.min(Math.floor(Math.log(sizeBytes) / Math.log(1024)), units.length - 1);
  const value = sizeBytes / 1024 ** exponent;
  return `${value >= 100 || exponent === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[exponent]}`;
}

export function formatDateTime(value: string | null, locale = "en-US"): string {
  if (!value) {
    return "-";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }

  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function formatCompactDateTime(value: string | null, locale = "en-US"): string {
  if (!value) {
    return "-";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }

  return new Intl.DateTimeFormat(locale, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function formatFileStatus(status: string): string {
  const normalized = status.trim().toLowerCase();
  if (!normalized) {
    return "Unknown";
  }
  if (["active", "ready", "available", "success"].includes(normalized)) {
    return "Available";
  }
  if (["pending", "processing"].includes(normalized)) {
    return "Processing";
  }
  if (["failed", "error"].includes(normalized)) {
    return "Failed";
  }
  return status;
}

export function isFileReady(status: string): boolean {
  return ["active", "ready", "available", "success"].includes(status.trim().toLowerCase());
}

export function resolveFileFilter(file: Pick<FileObjectDTO, "fileName" | "mimeType">): FileFilterKey {
  const mimeType = file.mimeType.trim().toLowerCase();
  const extension = resolveFileExtension(file.fileName);

  if (mimeType.startsWith("image/") || IMAGE_EXTENSIONS.has(extension)) {
    return "image";
  }
  if (mimeType === "application/pdf" || extension === "pdf") {
    return "pdf";
  }
  if (mimeType.startsWith("audio/")) {
    return "audio";
  }
  if (mimeType.startsWith("video/")) {
    return "video";
  }
  if (SPREADSHEET_EXTENSIONS.has(extension) || mimeType.includes("spreadsheet") || mimeType.includes("excel") || mimeType.includes("csv")) {
    return "spreadsheet";
  }
  if (PRESENTATION_EXTENSIONS.has(extension) || mimeType.includes("presentation") || mimeType.includes("powerpoint")) {
    return "presentation";
  }
  if (DOCUMENT_EXTENSIONS.has(extension) || mimeType.includes("word") || mimeType.includes("rtf") || mimeType.includes("opendocument.text")) {
    return "document";
  }
  if (
    MARKDOWN_EXTENSIONS.has(extension) ||
    CODE_EXTENSIONS.has(extension) ||
    mimeType.startsWith("text/") ||
    mimeType.includes("json") ||
    mimeType.includes("javascript") ||
    mimeType.includes("typescript") ||
    mimeType.includes("xml") ||
    mimeType.includes("html") ||
    mimeType.includes("css") ||
    mimeType.includes("yaml") ||
    mimeType.includes("toml") ||
    mimeType.includes("sql") ||
    mimeType.includes("markdown")
  ) {
    return "code";
  }
  return "document";
}

export function resolveFileIcon(file: Pick<FileObjectDTO, "fileName" | "mimeType">): LucideIcon {
  switch (resolveFileFilter(file)) {
    case "image":
      return FileImage;
    case "spreadsheet":
      return FileSpreadsheet;
    case "presentation":
      return FileType2;
    case "pdf":
      return FileText;
    case "audio":
      return FileAudio2;
    case "video":
      return FileVideo2;
    case "code":
      return FileCode2;
    case "document":
      return FileText;
    default:
      return FileArchive;
  }
}

export function resolveFilePreviewKind(file: Pick<FileObjectDTO, "fileName" | "mimeType">, contentType: string): FilePreviewKind {
  const mimeType = contentType.split(";")[0]?.trim().toLowerCase() || file.mimeType.trim().toLowerCase();
  const extension = resolveFileExtension(file.fileName);
  const filter = resolveFileFilter(file);

  if (extension === "docx" || isDocxMimeType(mimeType)) {
    return "docx";
  }
  if (extension === "csv" || mimeType === "text/csv") {
    return "spreadsheet";
  }
  if (
    (DOCUMENT_EXTENSIONS.has(extension) && extension !== "docx") ||
    PRESENTATION_EXTENSIONS.has(extension) ||
    (SPREADSHEET_EXTENSIONS.has(extension) && extension !== "csv") ||
    isOfficeDocumentMimeType(mimeType)
  ) {
    return "native";
  }

  if (filter === "image") {
    return "image";
  }
  if (filter === "pdf") {
    return "pdf";
  }
  if (filter === "audio") {
    return "audio";
  }
  if (filter === "video") {
    return "video";
  }
  if (MARKDOWN_EXTENSIONS.has(extension) || mimeType.includes("markdown")) {
    return "markdown";
  }
  if (CODE_EXTENSIONS.has(extension) || mimeType.includes("json") || mimeType.includes("javascript") || mimeType.includes("typescript") || mimeType.includes("xml") || mimeType.includes("html") || mimeType.includes("css") || mimeType.includes("yaml") || mimeType.includes("toml") || mimeType.includes("sql")) {
    return "code";
  }
  if (mimeType.startsWith("text/") || PLAIN_TEXT_EXTENSIONS.has(extension)) {
    return "text";
  }
  return "unsupported";
}

export function isImageFile(file: Pick<FileObjectDTO, "fileName" | "mimeType">): boolean {
  return resolveFileFilter(file) === "image";
}
