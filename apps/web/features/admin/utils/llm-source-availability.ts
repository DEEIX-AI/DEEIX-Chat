import type {
  AdminLLMModelUpstreamSourceDTO,
  AdminLLMStatus,
} from "@/features/admin/api/llm.types";

export function isAdminLLMSourceAvailable(
  source: AdminLLMModelUpstreamSourceDTO,
  modelStatus: AdminLLMStatus,
): boolean {
  return modelStatus === "active" &&
    source.status === "active" &&
    source.upstreamStatus === "active" &&
    source.upstreamModelStatus === "active" &&
    !source.circuitOpen;
}

export function applySourceAvailabilityDelta(
  current: number,
  sourceCount: number,
  previousAvailable: boolean,
  nextAvailable: boolean,
): number {
  if (previousAvailable === nextAvailable) {
    return current;
  }
  const delta = nextAvailable ? 1 : -1;
  return Math.min(sourceCount, Math.max(0, current + delta));
}
