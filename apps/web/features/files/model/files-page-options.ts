import {
  FileArchive,
  FileAudio2,
  FileCode2,
  FileImage,
  FileSpreadsheet,
  FileText,
  FileType2,
  FileVideo2,
} from "lucide-react";

import type { FileFilterOption, FileSortOption } from "@/features/files/types/files";

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
