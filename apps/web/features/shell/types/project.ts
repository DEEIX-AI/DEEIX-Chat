export type ProjectDraft = {
  publicID?: string;
  name: string;
  systemPrompt: string;
  defaultModel: string;
  mcpDefaultMode: "inherit" | "custom";
  defaultMCPToolIDs: number[];
  defaultSkillIDs: number[];
  defaultKnowledgeBaseIDs: string[];
};
