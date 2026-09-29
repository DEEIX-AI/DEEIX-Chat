import { isOneOf } from "@/shared/lib/type-guards";

// Single source of truth for admin string unions. The API types derive their
// unions from these arrays, and the guards narrow raw strings from selects,
// tabs, query params and payloads without `as` assertions.

export const ADMIN_USER_STATUSES = ["pending_activation", "active", "locked", "suspended", "deactivated"] as const;
export const isAdminUserStatus = isOneOf(ADMIN_USER_STATUSES);

export const ADMIN_USER_ROLES = ["user", "admin", "superadmin"] as const;
export const isAdminUserRole = isOneOf(ADMIN_USER_ROLES);

export const ADMIN_LLM_STATUSES = ["active", "inactive"] as const;
export const isAdminLLMStatus = isOneOf(ADMIN_LLM_STATUSES);

export const ADMIN_LLM_MODEL_ACCESS_SCOPES = ["public", "internal"] as const;
export const isAdminLLMModelAccessScope = isOneOf(ADMIN_LLM_MODEL_ACCESS_SCOPES);

export const ADMIN_LLM_ADAPTERS = [
  "openai_responses",
  "openrouter_chat_completions",
  "openrouter_responses",
  "openrouter_images",
  "openai_chat_completions",
  "openai_image_generations",
  "openai_image_edits",
  "openai_video_generations",
  "anthropic_messages",
  "google_generate_content",
  "google_image_generation",
  "gemini_interactions",
  "xai_responses",
  "xai_image",
  "xai_image_edits",
  "xai_video",
  "xai_video_extensions",
] as const;
export const isAdminLLMAdapter = isOneOf(ADMIN_LLM_ADAPTERS);

export const ADMIN_LLM_COMPATIBLES = ["openai", "anthropic", "google", "xai", "openrouter", "custom"] as const;
export const isAdminLLMCompatible = isOneOf(ADMIN_LLM_COMPATIBLES);

export const ADMIN_LLM_CB_LOGICS = ["or", "and"] as const;
export const isAdminLLMCbLogic = isOneOf(ADMIN_LLM_CB_LOGICS);

export const ADMIN_LLM_MODEL_CB_POLICY_MODES = ["default", "enforced"] as const;
export const isAdminLLMModelCbPolicyMode = isOneOf(ADMIN_LLM_MODEL_CB_POLICY_MODES);

export const ADMIN_USAGE_STATISTICS_RANK_BY = ["cost", "tokens", "calls"] as const;
export const isAdminUsageStatisticsRankBy = isOneOf(ADMIN_USAGE_STATISTICS_RANK_BY);

export const PERMISSION_GROUP_MODEL_RULE_TYPES = ["all", "vendor", "protocol", "upstream"] as const;
export const isPermissionGroupModelRuleType = isOneOf(PERMISSION_GROUP_MODEL_RULE_TYPES);

export const ADMIN_LOG_CLEANUP_TYPES = ["audit", "auth", "usage", "orders", "conversation", "moderation"] as const;
export const isAdminLogCleanupType = isOneOf(ADMIN_LOG_CLEANUP_TYPES);
