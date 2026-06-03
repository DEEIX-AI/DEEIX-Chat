import type {
  AdminLLMAdapter,
  AdminLLMStatus,
  AdminLLMUpstreamModelDTO,
  BindAdminLLMModelUpstreamSourceRequest,
} from "@/features/admin/api/llm.types";

export type ModelSourceBindDraft = {
  upstreamID: string;
  upstreamModelID: string;
  protocol: AdminLLMAdapter | "";
  priority: string;
  weight: string;
  status: AdminLLMStatus;
};

export type ModelSourceBindDraftRow = {
  id: string;
  draft: ModelSourceBindDraft;
};

export const DEFAULT_MODEL_SOURCE_BIND_DRAFT: ModelSourceBindDraft = {
  upstreamID: "",
  upstreamModelID: "",
  protocol: "",
  priority: "1",
  weight: "1",
  status: "active",
};

export type ModelSourceBindDraftError =
  | "required"
  | "protocolRequired"
  | "priorityMustBePositive"
  | "weightMustBePositive"
  | "duplicate";

export type ModelSourceBindDraftResult =
  | { status: "empty" }
  | { status: "invalid"; error: ModelSourceBindDraftError }
  | { status: "valid"; payload: BindAdminLLMModelUpstreamSourceRequest };

export type ModelSourceBindDraftsResult =
  | { status: "invalid"; error: ModelSourceBindDraftError; rowID: string }
  | { status: "valid"; payloads: BindAdminLLMModelUpstreamSourceRequest[] };

let sourceBindDraftRowSeq = 0;

export function createModelSourceBindDraftRow(
  draft: ModelSourceBindDraft = DEFAULT_MODEL_SOURCE_BIND_DRAFT,
): ModelSourceBindDraftRow {
  sourceBindDraftRowSeq += 1;
  return {
    id: `source-bind-${Date.now()}-${sourceBindDraftRowSeq}`,
    draft,
  };
}

export function uniqueUpstreamModels(items: AdminLLMUpstreamModelDTO[]): AdminLLMUpstreamModelDTO[] {
  const seen = new Set<number>();
  const results: AdminLLMUpstreamModelDTO[] = [];
  for (const item of items) {
    if (seen.has(item.id)) {
      continue;
    }
    seen.add(item.id);
    results.push(item);
  }
  return results;
}

export function modelSourceBindDraftHasSelection(draft: ModelSourceBindDraft): boolean {
  return Boolean(draft.upstreamID.trim() || draft.upstreamModelID.trim() || draft.protocol);
}

function parseStrictPositiveInteger(value: string): number | null {
  const trimmed = value.trim();
  if (!/^\d+$/.test(trimmed)) {
    return null;
  }
  const parsed = Number.parseInt(trimmed, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}

export function resolveModelSourceBindDraft(
  draft: ModelSourceBindDraft,
): ModelSourceBindDraftResult {
  if (!modelSourceBindDraftHasSelection(draft)) {
    return { status: "empty" };
  }

  const upstreamID = Number.parseInt(draft.upstreamID, 10);
  const upstreamModelID = Number.parseInt(draft.upstreamModelID, 10);
  if (
    !Number.isFinite(upstreamID) ||
    upstreamID <= 0 ||
    !Number.isFinite(upstreamModelID) ||
    upstreamModelID <= 0
  ) {
    return { status: "invalid", error: "required" };
  }
  if (!draft.protocol) {
    return { status: "invalid", error: "protocolRequired" };
  }

  const priority = parseStrictPositiveInteger(draft.priority);
  if (priority === null) {
    return { status: "invalid", error: "priorityMustBePositive" };
  }
  const weight = parseStrictPositiveInteger(draft.weight);
  if (weight === null) {
    return { status: "invalid", error: "weightMustBePositive" };
  }

  return {
    status: "valid",
    payload: {
      upstreamID,
      upstreamModelID,
      protocol: draft.protocol,
      status: draft.status,
      priority,
      weight,
    },
  };
}

export function resolveModelSourceBindDraftRows(
  rows: ModelSourceBindDraftRow[],
): ModelSourceBindDraftsResult {
  const payloads: BindAdminLLMModelUpstreamSourceRequest[] = [];
  const seen = new Set<string>();

  for (const row of rows) {
    const result = resolveModelSourceBindDraft(row.draft);
    if (result.status === "empty") {
      continue;
    }
    if (result.status === "invalid") {
      return { status: "invalid", error: result.error, rowID: row.id };
    }

    const duplicateKey = [
      result.payload.upstreamID,
      result.payload.upstreamModelID,
      result.payload.protocol ?? "",
    ].join(":");
    if (seen.has(duplicateKey)) {
      return { status: "invalid", error: "duplicate", rowID: row.id };
    }
    seen.add(duplicateKey);
    payloads.push(result.payload);
  }

  return { status: "valid", payloads };
}
