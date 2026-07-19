import type {
  PatchSkillRequest as ContractPatchSkillRequest,
  SkillDataResponse,
  SkillDeleteDataResponse,
  SkillPageResponseDoc,
  SkillResponse,
  SkillSummaryResponse,
  SkillSummaryPageResponseDoc,
  WriteSkillRequest as ContractWriteSkillRequest,
} from "@deeix/api-contract";

export type SkillScope = "builtin" | "user";

export type SkillSummaryDTO = Omit<SkillSummaryResponse, "scope"> & {
  scope: SkillScope;
};

export type SkillDTO = Omit<SkillResponse, "scope"> & {
  scope: SkillScope;
};

type ContractSkillSummaryPage = SkillSummaryPageResponseDoc["data"];
type ContractSkillPage = SkillPageResponseDoc["data"];

export type SkillSummaryPage = Omit<ContractSkillSummaryPage, "results"> & {
  results: SkillSummaryDTO[];
};

export type SkillPage = Omit<ContractSkillPage, "results"> & {
  results: SkillDTO[];
};

export type WriteSkillRequest = ContractWriteSkillRequest;

export type PatchSkillRequest = ContractPatchSkillRequest;

export type SkillData = Omit<SkillDataResponse, "skill"> & {
  skill: SkillDTO;
};

export type SkillDeleteData = SkillDeleteDataResponse;
