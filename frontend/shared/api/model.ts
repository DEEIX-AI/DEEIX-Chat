import { authedRequest } from "@/shared/api/authed-client";
import type { PublicModelDTO } from "@/shared/api/model.types";

export async function listPublicModels(accessToken: string): Promise<PublicModelDTO[]> {
  return authedRequest<PublicModelDTO[]>(
    "/api/v1/models",
    {
      accessToken,
    },
    true,
  );
}
