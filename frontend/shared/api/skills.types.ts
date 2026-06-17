export type SkillScope = "builtin" | "user";

export type SkillSummaryDTO = {
  id: number;
  scope: SkillScope;
  title: string;
  trigger: string;
  description: string;
  enabled: boolean;
  sortOrder: number;
  createdAt: string;
  updatedAt: string;
};

export type SkillDTO = SkillSummaryDTO & {
  markdown: string;
  createdByUserID: number;
  updatedByUserID: number;
};

export type SkillSummaryPage = {
  results: SkillSummaryDTO[];
  total: number;
};

export type SkillPage = {
  results: SkillDTO[];
  total: number;
};

export type WriteSkillRequest = {
  title: string;
  trigger: string;
  description: string;
  markdown: string;
  enabled: boolean;
  sortOrder: number;
};

export type PatchSkillRequest = Partial<WriteSkillRequest>;

export type SkillData = {
  skill: SkillDTO;
};

export type SkillDeleteData = {
  deleted: boolean;
};
