import type { LucideIcon } from "lucide-react";
import type {
  FileFilterKey,
  FilePreviewKind,
} from "@/shared/lib/file-display";

export type {
  FileFilterKey,
  FilePreviewKind,
} from "@/shared/lib/file-display";

export type FileFilterValue = Exclude<FileFilterKey, "all">;

export type FileSortKey = "created" | "name" | "size" | "last_used";

export type FileFilterOption = { value: FileFilterKey; icon: LucideIcon };

export type FileSortOption = { value: FileSortKey };
