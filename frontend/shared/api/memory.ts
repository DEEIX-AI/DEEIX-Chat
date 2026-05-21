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
  scope: string,
): Promise<{ saved: boolean }> {
  return authedRequest<{ saved: boolean }>("/api/v1/memories/profile", {
    method: "PUT",
    accessToken,
    body: { memoryKey: key, value, scope },
  });
}

export async function deleteUserMemory(
  accessToken: string,
  memoryKey: string,
): Promise<{ saved: boolean }> {
  return authedRequest<{ saved: boolean }>(`/api/v1/memories/profile/${pathParam(memoryKey)}`, {
    method: "DELETE",
    accessToken,
  });
}
