import { resolveModelIconURL, resolveModelIdentity } from "@/entities/model/lib/model-identity";
import { parseKindsJSON } from "@/entities/model/model/llm-schema";

function isRoutablePlatformModel(model: {
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

/** Model references in the user-key namespace: `personal:<provider id>/<upstream model>`. */
const PERSONAL_MODEL_REF = /^personal:[a-z0-9]{8,32}\/(.+)$/;

/** True for a model served by the user's own key rather than the platform. */
export function isPersonalModelRef(platformModelName: string): boolean {
  return PERSONAL_MODEL_REF.test(platformModelName.trim());
}

/** Name to show for a model; personal references show the upstream model name. */
export function resolveModelOptionLabel(platformModelName: string): string {
  const value = platformModelName.trim();
  return PERSONAL_MODEL_REF.exec(value)?.[1] ?? value;
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
  return resolveModelIconURL(identity.modelIcon);
}
