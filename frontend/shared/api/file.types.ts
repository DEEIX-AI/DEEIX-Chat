export type FileObjectDTO = {
  fileID: string;
  purpose: string;
  fileName: string;
  mimeType: string;
  detectedMIME: string;
  fileCategory: string;
  sizeBytes: number;
  sha256: string;
  status: string;
  processingStatus: string;
  processingReady: boolean;
  processingErrorCode: string;
  processingErrorMessage: string;
  extractStatus: string;
  embedStatus: string;
  embedError: string;
  chunkCount: number;
  ragOptOut: boolean;
  lastAccessedAt: string | null;
  expiresAt: string | null;
  createdAt: string;
  updatedAt: string;
};

export type FileProcessingStatusDTO = {
  fileID: string;
  detectedMIME: string;
  fileCategory: string;
  processingStatus: string;
  processingReady: boolean;
  extractStatus: string;
  embedStatus: string;
  previewText: string;
  ocrUsed: boolean;
  ragReady: boolean;
  ragReason: string;
  errorCode: string;
  errorMessage: string;
  extractChars: number;
  extractPages: number;
  startedAt: string | null;
  completedAt: string | null;
};

export type FileExtractDTO = {
  fileID: string;
  extractText: string;
  previewText: string;
  extractChars: number;
  extractPages: number;
  ocrUsed: boolean;
};

export type ChatFilePolicyDTO = {
  maxMessageFiles: number;
  maxUploadFileBytes: number;
  allowedMIMETypes: string[];
  imageMaxBytes: number;
  docMaxBytes: number;
  effectiveImageMaxBytes: number;
  effectiveDocMaxBytes: number;
  fullContextMaxBytes: number;
  fullContextMaxTokens: number;
  fullContextPDFMaxPages: number;
  ragAvailable: boolean;
  ragAvailabilityReason: string;
  capabilityMode: "full_context_only" | "full_context_and_rag";
  fileMode: "auto" | "full_context" | "rag";
};

export type UserStorageQuotaDTO = {
  userID: number;
  quotaBytes: number;
  usedBytes: number;
  reservedBytes: number;
  createdAt: string;
  updatedAt: string;
};

export type FileListResult = {
  total: number;
  results: FileObjectDTO[];
  quota: UserStorageQuotaDTO;
};

export type UploadFileResult = {
  file: FileObjectDTO;
  quota: UserStorageQuotaDTO;
  reused: boolean;
};

export type DeleteFileResult = {
  deleted: boolean;
  fileID: string;
  quota: UserStorageQuotaDTO;
};
