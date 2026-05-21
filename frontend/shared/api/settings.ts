import { authedRequest } from "@/shared/api/authed-client";
import type { ModelOptionPolicy } from "@/shared/lib/model-option-policy";

type ModelOptionPolicyResponse = {
  mode: string;
  allowedPathsJSON: string;
  deniedPathsJSON: string;
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
  };
}
