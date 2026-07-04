import { getConversationDefaultModelCandidate } from "@/shared/api/conversation";
import { listPublicModels } from "@/shared/api/model";
import type { PublicModelDTO } from "@/shared/api/model.types";
import { getUserSettings } from "@/shared/api/user-settings";

export type ConversationDefaultModelSource = "explicit" | "user_default" | "system_default" | "recommended" | "none";

export type ConversationDefaultModelResult = {
  platformModelName: string;
  source: ConversationDefaultModelSource;
};

type ResolveConversationDefaultModelInput = {
  accessToken: string;
  explicitModel?: string;
  availableModels?: PublicModelDTO[];
  userDefaultModel?: string;
};

function findAvailableModel(models: PublicModelDTO[], platformModelName: string): string {
  const normalizedName = platformModelName.trim();
  if (!normalizedName) {
    return "";
  }
  return models.some((item) => item.platformModelName === normalizedName) ? normalizedName : "";
}

export async function resolveConversationDefaultModel({
  accessToken,
  explicitModel,
  availableModels,
  userDefaultModel,
}: ResolveConversationDefaultModelInput): Promise<ConversationDefaultModelResult> {
  const models = availableModels ?? await listPublicModels(accessToken);
  const explicit = findAvailableModel(models, explicitModel ?? "");
  if (explicit) {
    return { platformModelName: explicit, source: "explicit" };
  }

  const defaultModel = userDefaultModel ?? (await getUserSettings(accessToken).catch(() => ({})))["chat.default_model"];
  const userDefault = findAvailableModel(models, defaultModel ?? "");
  if (userDefault) {
    return { platformModelName: userDefault, source: "user_default" };
  }

  const candidate = await getConversationDefaultModelCandidate(accessToken).catch(() => null);
  const candidateModel = findAvailableModel(models, candidate?.platformModelName ?? "");
  if (candidateModel) {
    return {
      platformModelName: candidateModel,
      source: "system_default",
    };
  }

  const recommended = models[0]?.platformModelName?.trim() ?? "";
  return recommended
    ? { platformModelName: recommended, source: "recommended" }
    : { platformModelName: "", source: "none" };
}
