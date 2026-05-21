import type {
  AdminBatchDeleteData,
  AdminLLMAdapter,
  AdminLLMRemoteModelItem,
  AdminLLMStatus,
  AdminLLMUpstreamModelDTO,
} from "@/features/admin/api/llm.types";
import { parseKindsJSON, stringifyKinds } from "@/shared/model/llm-schema";

export type RowDraft = AdminLLMUpstreamModelDTO & {
  draftKey: string;
  isDirty: boolean;
  kindsDisplay: string;
  platformModelNameDraft: string;
};

export type NewBindingFormState = {
  upstreamModelName: string;
  platformModelName: string;
  protocol: AdminLLMAdapter;
  kindsDisplay: string;
  status: AdminLLMStatus;
};

export const DEFAULT_NEW_BINDING: NewBindingFormState = {
  upstreamModelName: "",
  platformModelName: "",
  protocol: "openai_responses",
  kindsDisplay: "chat",
  status: "active",
};

export function kindsJsonToDisplay(kindsJson: string): string {
  if (!kindsJson) return "chat";
  const kinds = parseKindsJSON(kindsJson);
  return kinds.length > 0 ? kinds.join(",") : "chat";
}

export function displayToKindsJson(display: string): string {
  const kinds = display
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
  return stringifyKinds(kinds);
}

export function buildRowDrafts(items: AdminLLMUpstreamModelDTO[]): RowDraft[] {
  return items.map((item) => ({
    ...item,
    draftKey: item.routeID > 0 ? `route:${item.routeID}` : `upstream:${item.upstreamID}:${item.upstreamModelName}`,
    platformModelNameDraft: item.platformModelName,
    isDirty: false,
    kindsDisplay: kindsJsonToDisplay(item.upstreamModelKindsJSON || item.modelKindsJSON),
  }));
}

export type UpstreamModelMessages = {
  upstreamModelRequired: string;
  activeRouteRequiresPlatformModel: string;
  duplicateBinding: (upstreamModelName: string, platformModelName: string) => string;
  batchDeleteSummary: (successCount: number, notFoundCount: number, failedCount: number) => string;
  importSummary: (result: {
    importedCount: number;
    failedCount: number;
    createdPlatform: number;
    createdRoutes: number;
    existingRoutes: number;
  }) => string;
};

export function validateRowDrafts(
  rows: RowDraft[],
  messages: Pick<UpstreamModelMessages, "upstreamModelRequired" | "activeRouteRequiresPlatformModel" | "duplicateBinding">,
): string | undefined {
  const bindingOwners = new Map<string, string>();
  for (const row of rows) {
    const platformModelName = row.platformModelNameDraft.trim();
    const upstreamModelName = row.upstreamModelName.trim();
    if (!upstreamModelName) {
      return messages.upstreamModelRequired;
    }
    if (row.routeStatus === "active" && !platformModelName) {
      return messages.activeRouteRequiresPlatformModel;
    }
    if (!platformModelName) {
      continue;
    }
    const bindingKey = `${upstreamModelName}\u0000${platformModelName}`;
    const existingOwner = bindingOwners.get(bindingKey);
    if (existingOwner && existingOwner !== row.draftKey) {
      return messages.duplicateBinding(upstreamModelName, platformModelName);
    }
    bindingOwners.set(bindingKey, row.draftKey);
  }
  return undefined;
}

export function createDraftPlatformModelNameMap(items: AdminLLMRemoteModelItem[]): Map<string, string> {
  const platformModelNames = new Map<string, string>();
  for (const item of items) {
    platformModelNames.set(item.upstreamModelName, item.suggestedPlatformModelName || item.upstreamModelName);
  }
  return platformModelNames;
}

export function summarizeBatchDeleteResult(
  result: AdminBatchDeleteData,
  messages: Pick<UpstreamModelMessages, "batchDeleteSummary">,
): string {
  return messages.batchDeleteSummary(result.successCount, result.notFoundCount, result.failedCount);
}

export function summarizeImportResult(result: {
  importedCount: number;
  failedCount: number;
  createdPlatform: number;
  createdRoutes: number;
  existingRoutes: number;
}, messages: Pick<UpstreamModelMessages, "importSummary">): string {
  return messages.importSummary(result);
}
