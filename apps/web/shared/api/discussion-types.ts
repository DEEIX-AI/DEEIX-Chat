export type DiscussionPlaceholder = {
  key: string;
  label: string;
  defaultModel?: string;
  required?: boolean;
};

export type DiscussionStep = {
  id: string;
  participant: string;
  prompt: string;
};

export type DiscussionRound = {
  index: number;
  parallel: boolean;
  /**
   * Prompt variants per execution mode. The editor keeps them isolated, and steps[].prompt mirrors
   * the currently selected mode for older readers and for the runtime fallback.
   */
  parallelPrompt?: string;
  serialPrompt?: string;
  steps: DiscussionStep[];
};

export type DiscussionSummary = {
  participant: string;
  prompt: string;
};

export type DiscussionTemplateConfig = {
  placeholders: DiscussionPlaceholder[];
  rounds: DiscussionRound[];
  summary: DiscussionSummary;
};

export type DiscussionTemplateDTO = {
  id: number;
  publicID: string;
  name: string;
  description: string;
  icon: string;
  status: "active" | "inactive";
  maxRounds: number;
  config: DiscussionTemplateConfig;
  sortOrder: number;
  createdAt: string;
  updatedAt: string;
};

export type DiscussionTemplatePayload = {
  name: string;
  description?: string;
  icon?: string;
  status?: "active" | "inactive";
  maxRounds?: number;
  config: DiscussionTemplateConfig;
  sortOrder?: number;
};

export type DiscussionPromptMessage = {
  role: string;
  content: string;
};

export type DiscussionTurn = {
  round: number;
  stepId: string;
  participant: string;
  model: string;
  status: "running" | "completed" | "error" | string;
  content?: string;
  error?: string;
  /** Exact prompt sent to the model; debug output, absent on messages recorded before it existed. */
  prompt?: string;
  promptMessages?: DiscussionPromptMessage[];
  inputTokens?: number;
  outputTokens?: number;
  cacheReadTokens?: number;
  cacheWriteTokens?: number;
  reasoningTokens?: number;
  latencyMs?: number;
};

export type DiscussionRuntimeRound = {
  index: number;
  parallel?: boolean;
  steps: Array<{
    id: string;
    participant: string;
    model: string;
    prompt: string;
  }>;
};

export type DiscussionRuntimeSummary = {
  participant: string;
  model: string;
  prompt: string;
};

export type DiscussionState = {
  enabled: boolean;
  status: string;
  templateId: string;
  templateName: string;
  placeholders: DiscussionPlaceholder[];
  rounds: DiscussionRuntimeRound[];
  summary: DiscussionRuntimeSummary;
  /** Exact prompt sent to the summary model; debug output. */
  summaryPrompt?: string;
  summaryPromptMessages?: DiscussionPromptMessage[];
  turns: DiscussionTurn[];
  finalContent?: string;
  error?: string;
  createdAt: number;
  updatedAt: number;
};

export type DiscussionSendRequest = {
  contentType: "text" | "markdown";
  content: string;
  templatePublicID: string;
  models: Record<string, string>;
  clientRunID?: string;
  parentMessagePublicID?: string;
  sourceMessagePublicID?: string;
  branchReason?: "default" | "retry" | "edit";
};
