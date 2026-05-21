import type { LucideIcon } from "lucide-react";

export type FileFilterKey =
  | "all"
  | "image"
  | "document"
  | "spreadsheet"
  | "presentation"
  | "code"
  | "pdf"
  | "audio"
  | "video";

export type FileFilterValue = Exclude<FileFilterKey, "all">;

export type FileSortKey = "created" | "name" | "size" | "last_used";

export type FilePreviewKind =
  | "image"
  | "pdf"
  | "audio"
  | "video"
  | "docx"
  | "spreadsheet"
  | "native"
  | "markdown"
  | "code"
  | "text"
  | "unsupported";

export type FileFilterOption = { value: FileFilterKey; icon: LucideIcon };

export type FileSortOption = { value: FileSortKey };
