import { resolveLobeHubIconURL, resolveModelIdentity } from "@/shared/lib/model-identity";
import { parseKindsJSON } from "@/shared/model/llm-schema";

export function isRoutablePlatformModel(model: {
  platformModelName?: string | null;
  status?: string | null;
  activeSourceCount?: number | null;
}): boolean {
  return Boolean(
    model.platformModelName?.trim() &&
    model.status === "active" &&
    (model.activeSourceCount ?? 0) > 0,
  );
}

export function isRoutableChatPlatformModel(model: {
  platformModelName?: string | null;
  status?: string | null;
  activeSourceCount?: number | null;
  kindsJSON?: string | null;
}): boolean {
  if (!isRoutablePlatformModel(model)) return false;
  const kinds = parseKindsJSON(model.kindsJSON);
  return kinds.includes("chat");
}

export function resolveModelOptionLabel(platformModelName: string): string {
  return platformModelName.trim();
}

export function resolveModelOptionIconUrl({
  platformModelName,
  vendor,
  icon,
}: {
  platformModelName: string;
  vendor: string;
  icon: string;
}): string | null {
  const identity = resolveModelIdentity({
    code: platformModelName,
    vendor,
    icon,
  });
  return resolveLobeHubIconURL(identity.modelIcon);
}
