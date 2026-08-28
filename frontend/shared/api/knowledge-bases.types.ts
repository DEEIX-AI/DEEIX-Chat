import type {
  AddKnowledgeBaseFilesRequest as ContractAddKnowledgeBaseFilesRequest,
  KnowledgeBaseDataResponse,
  KnowledgeBaseDeleteDataResponse,
  KnowledgeBaseFileDataResponse,
  KnowledgeBaseFileMutationDataResponse,
  KnowledgeBaseFilePageResponseDoc,
  KnowledgeBaseFileProcessingSnapshotResponse,
  KnowledgeBaseFileProcessingStatusResponse,
  KnowledgeBasePageResponseDoc,
  KnowledgeBaseResponse,
  PatchMyKnowledgeBaseRequest as ContractPatchMyKnowledgeBaseRequest,
  PatchKnowledgeBaseRequest as ContractPatchKnowledgeBaseRequest,
  WriteMyKnowledgeBaseRequest as ContractWriteMyKnowledgeBaseRequest,
  WriteKnowledgeBaseRequest as ContractWriteKnowledgeBaseRequest,
} from "@deeix/api-contract";

export type KnowledgeBaseScope = "builtin" | "user";

export type KnowledgeBaseDTO = Omit<KnowledgeBaseResponse, "scope"> & {
  scope: KnowledgeBaseScope;
};

export type KnowledgeBasePage = Omit<KnowledgeBasePageResponseDoc["data"], "results"> & {
  results: KnowledgeBaseDTO[];
};

export type KnowledgeBaseFileDTO = KnowledgeBaseFilePageResponseDoc["data"]["results"][number];
export type KnowledgeBaseFileProcessingStatusDTO = KnowledgeBaseFileProcessingStatusResponse;
export type KnowledgeBaseFileProcessingSnapshotDTO = Omit<KnowledgeBaseFileProcessingSnapshotResponse, "knowledgeBase"> & {
  knowledgeBase: KnowledgeBaseDTO;
};
export type KnowledgeBaseFilePage = KnowledgeBaseFilePageResponseDoc["data"];
export type KnowledgeBaseData = Omit<KnowledgeBaseDataResponse, "knowledgeBase"> & {
  knowledgeBase: KnowledgeBaseDTO;
};
export type WriteKnowledgeBaseRequest = ContractWriteKnowledgeBaseRequest;
export type PatchKnowledgeBaseRequest = ContractPatchKnowledgeBaseRequest;
export type WriteMyKnowledgeBaseRequest = ContractWriteMyKnowledgeBaseRequest;
export type PatchMyKnowledgeBaseRequest = ContractPatchMyKnowledgeBaseRequest;
export type AddKnowledgeBaseFilesRequest = ContractAddKnowledgeBaseFilesRequest;
export type KnowledgeBaseDeleteData = KnowledgeBaseDeleteDataResponse;
export type KnowledgeBaseFileMutationData = KnowledgeBaseFileMutationDataResponse;
export type KnowledgeBaseFileData = KnowledgeBaseFileDataResponse;
