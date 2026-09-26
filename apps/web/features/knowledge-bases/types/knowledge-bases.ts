import type { KnowledgeBaseFileDTO } from "@/shared/api/knowledge-bases.types";

export type KnowledgeBaseMode = "user" | "admin";

export type KnowledgeBaseSortKey = "default" | "name" | "created" | "updated" | "files";

export type KnowledgeBaseDraft = {
  publicID?: string;
  name: string;
  description: string;
};

export type KnowledgeBasePreviewTarget = {
  knowledgeBaseID: string;
  admin: boolean;
  file: KnowledgeBaseFileDTO;
};

export type KnowledgeBaseMobileView = "list" | "detail";
