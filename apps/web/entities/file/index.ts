// Public entry of the file entity; code outside entities/file/ imports only this file.
// Unused re-exports are tree-shaken (package.json "sideEffects"), so UI and logic share one entry.
export { DeleteFilesOption } from "@/entities/file/components/delete-files-option";
export { LazyFilePreviewDialog } from "@/entities/file/components/file-preview/lazy-file-preview-dialog";
export { PreviewDocument } from "@/entities/file/components/file-preview/preview-document";
export { PreviewDocx } from "@/entities/file/components/file-preview/preview-docx";
export { PreviewLoading } from "@/entities/file/components/file-preview/preview-loading";
export { PreviewMedia } from "@/entities/file/components/file-preview/preview-media";
export { PreviewPdf } from "@/entities/file/components/file-preview/preview-pdf";
export { PreviewSheet } from "@/entities/file/components/file-preview/preview-sheet";
export { PreviewText } from "@/entities/file/components/file-preview/preview-text";
export {
  type FileLibraryInvalidatedDetail,
  dispatchFileLibraryInvalidated,
  subscribeFileLibraryInvalidated,
} from "@/entities/file/events/file-library-events";
export type { FileContentLoader, PreviewDialogFile } from "@/entities/file/hooks/use-file-preview-dialog";
export {
  type FileStatusPollingResult,
  useFileProcessingStatusPolling,
  useFileStatusPolling,
} from "@/entities/file/hooks/use-file-processing-status-polling";
export {
  type FileFilterKey,
  type FilePreviewKind,
  formatBytes,
  formatDateTime,
  isFileReady,
  isImageFile,
  isReadableTextContent,
  resolveFileExtension,
  resolveFileFilter,
  resolveFileIcon,
  resolveFilePreviewKind,
} from "@/entities/file/lib/file-display";
export {
  canManuallyVectorizeFile,
  isFileProcessing,
  isVectorIndexOutdated,
  resolveEmbedStatusLabel,
  resolveExtractStatusLabel,
  resolveFileProcessingBadge,
  resolveFileProcessingToneClass,
  resolveFileRetrievalBadge,
} from "@/entities/file/lib/file-processing";
