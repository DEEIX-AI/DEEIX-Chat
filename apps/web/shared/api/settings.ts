import { authedRequest } from "@/shared/api/authed-client";
import type { ModelOptionPolicy, NativeToolDefinition } from "@/shared/api/settings-types";
import { isRecord, readBoolean, readString } from "@/shared/lib/type-guards";

type ModelOptionPolicyResponse = {
  mode: string;
  allowedPathsJSON: string;
  deniedPathsJSON: string;
  nativeTools?: NativeToolDefinition[];
};

export type MCPPolicy = {
  maxSelectedToolsPerMessage: number;
};

export type ChatContextPolicy = {
  contextCompactEnabled: boolean;
};

/** Default desktop download page; mirrors the backend `desktop:download_url` default. */
export const DEFAULT_DESKTOP_DOWNLOAD_URL = "https://deeix.com/download";

export type FeaturePolicy = {
  knowledgeBaseEnabled: boolean;
  processTraceEnabled: boolean;
  /** Whether the web user menu offers the desktop app download entry. */
  desktopDownloadEnabled: boolean;
  /** Download page URL; empty when the entry is disabled. */
  desktopDownloadURL: string;
};

export async function getModelOptionPolicy(accessToken: string): Promise<ModelOptionPolicy> {
  const data = await authedRequest<ModelOptionPolicyResponse>(
    "/api/v1/settings/model-option-policy",
    { accessToken },
    true,
  );
  return {
    mode: data.mode,
    allowedPathsJSON: data.allowedPathsJSON,
    deniedPathsJSON: data.deniedPathsJSON,
    nativeTools: data.nativeTools ?? [],
  };
}

export async function getMCPPolicy(accessToken: string): Promise<MCPPolicy> {
  const data = await authedRequest<MCPPolicy>(
    "/api/v1/settings/mcp-policy",
    { accessToken },
    true,
  );
  return {
    maxSelectedToolsPerMessage: data.maxSelectedToolsPerMessage,
  };
}

export async function getChatContextPolicy(accessToken: string): Promise<ChatContextPolicy> {
  return authedRequest<ChatContextPolicy>(
    "/api/v1/settings/chat-context-policy",
    { accessToken },
    true,
  );
}

/**
 * Reads the feature policy. Only fields present with the expected type are returned, so an
 * older server that lacks a field leaves the caller's default in place.
 */
export async function getFeaturePolicy(accessToken: string): Promise<Partial<FeaturePolicy>> {
  const data = await authedRequest<unknown>(
    "/api/v1/settings/feature-policy",
    { accessToken },
    true,
  );
  return parseFeaturePolicy(data);
}

function parseFeaturePolicy(data: unknown): Partial<FeaturePolicy> {
  if (!isRecord(data)) return {};
  const policy: Partial<FeaturePolicy> = {};
  const knowledgeBaseEnabled = readBoolean(data, "knowledgeBaseEnabled");
  if (knowledgeBaseEnabled !== undefined) policy.knowledgeBaseEnabled = knowledgeBaseEnabled;
  const processTraceEnabled = readBoolean(data, "processTraceEnabled");
  if (processTraceEnabled !== undefined) policy.processTraceEnabled = processTraceEnabled;
  const desktopDownloadEnabled = readBoolean(data, "desktopDownloadEnabled");
  if (desktopDownloadEnabled !== undefined) policy.desktopDownloadEnabled = desktopDownloadEnabled;
  const desktopDownloadURL = readString(data, "desktopDownloadURL");
  if (desktopDownloadURL !== undefined) policy.desktopDownloadURL = desktopDownloadURL.trim();
  return policy;
}
