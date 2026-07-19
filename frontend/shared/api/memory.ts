import type { Memories } from "@deeix/api-contract";
import { authedRequest } from "@/shared/api/authed-client";
import { pathParam } from "@/shared/api/http-client";
import type { UserMemoryDTO } from "@/shared/api/memory.types";

export async function listUserMemories(accessToken: string): Promise<UserMemoryDTO[]> {
  return authedRequest<UserMemoryDTO[]>("/api/v1/memories/profile", {
    method: "GET",
    accessToken,
  });
}

export async function upsertUserMemory(
  accessToken: string,
  key: string,
  value: string,
  scope: Memories.ProfileUpdate.RequestBody["scope"],
): Promise<{ saved: boolean }> {
  const body: Memories.ProfileUpdate.RequestBody = { memoryKey: key, value, scope };
  return authedRequest<Memories.ProfileUpdate.ResponseBody["data"]>("/api/v1/memories/profile", {
    method: "PUT",
    accessToken,
    body,
  });
}

export async function deleteUserMemory(
  accessToken: string,
  memoryKey: string,
): Promise<{ saved: boolean }> {
  return authedRequest<Memories.ProfileDelete.ResponseBody["data"]>(`/api/v1/memories/profile/${pathParam(memoryKey)}`, {
    method: "DELETE",
    accessToken,
  });
}
