import type { LucideIcon } from "lucide-react";
import type { FileFilterKey } from "@/entities/file";

export type {
  FileFilterKey,
  FilePreviewKind,
} from "@/entities/file";

export type FileFilterValue = Exclude<FileFilterKey, "all">;

export type FileSortKey = "created" | "name" | "size" | "last_used";

export type FileFilterOption = { value: FileFilterKey; icon: LucideIcon };

export type FileSortOption = { value: FileSortKey };
