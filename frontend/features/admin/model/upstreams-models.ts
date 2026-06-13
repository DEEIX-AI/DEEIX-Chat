import type {
  AdminBatchDeleteData,
  AdminLLMAdapter,
  AdminLLMRemoteModelItem,
  AdminLLMStatus,
  AdminLLMUpstreamModelDTO,
} from "@/features/admin/api/llm.types";
import { sortProtocolsForDisplay } from "@/features/admin/utils/llm-display";
import { parseKindsJSON, stringifyKinds } from "@/shared/model/llm-schema";

export type RowDraft = AdminLLMUpstreamModelDTO & {
  draftKey: string;
  isDirty: boolean;
  kindsDisplay: string;
  platformModelNameDraft: string;
  protocols: AdminLLMAdapter[];
  routeIDsByProtocol: Record<string, number>;
};

export type NewBindingFormState = {
  upstreamModelName: string;
  platformModelName: string;
  protocols: AdminLLMAdapter[];
  kindsDisplay: string;
  status: AdminLLMStatus;
};

export const DEFAULT_NEW_BINDING: NewBindingFormState = {
  upstreamModelName: "",
  platformModelName: "",
  protocols: ["openai_responses"],
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
  const grouped = new Map<string, RowDraft>();
  for (const item of items) {
    const platformModelName = item.platformModelName || "";
    const draftKey = platformModelName
      ? `binding:${item.upstreamID}:${item.upstreamModelName}:${platformModelName}`
      : `upstream:${item.upstreamID}:${item.upstreamModelName}`;
    const existing = grouped.get(draftKey);
    if (existing) {
      if (item.protocol && !existing.protocols.includes(item.protocol)) {
        existing.protocols.push(item.protocol);
      }
      if (item.protocol && item.routeID > 0) {
        existing.routeIDsByProtocol[item.protocol] = item.routeID;
      }
      if (item.routeStatus === "active") {
        existing.routeStatus = "active";
      }
      if (item.routeID > 0 && (!existing.routeID || item.routeID < existing.routeID)) {
        existing.routeID = item.routeID;
      }
      continue;
    }
    grouped.set(draftKey, {
      ...item,
      draftKey,
      platformModelNameDraft: platformModelName,
      isDirty: false,
      kindsDisplay: kindsJsonToDisplay(item.upstreamModelKindsJSON || item.modelKindsJSON),
      protocols: item.protocol ? [item.protocol] : [],
      routeIDsByProtocol: item.protocol && item.routeID > 0 ? { [item.protocol]: item.routeID } : {},
    });
  }
  return Array.from(grouped.values()).map((row) => {
    const protocols = sortProtocolsForDisplay(row.protocols);
    return {
      ...row,
      protocol: protocols[0] ?? row.protocol,
      protocols,
    };
  });
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
    const protocols = row.protocols.length > 0 ? row.protocols : [row.protocol || row.suggestedProtocol || ""];
    for (const protocol of protocols) {
      const bindingKey = `${upstreamModelName}\u0000${platformModelName}\u0000${protocol}`;
      const existingOwner = bindingOwners.get(bindingKey);
      if (existingOwner && existingOwner !== row.draftKey) {
        return messages.duplicateBinding(upstreamModelName, platformModelName);
      }
      bindingOwners.set(bindingKey, row.draftKey);
    }
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
