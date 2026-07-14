import { authedFetch, authedRequest } from "@/shared/api/authed-client";

export type AdminDatabaseBackupInfo = { driver: "postgres" | "sqlite"; maxRestoreBytes: number };

export function getAdminDatabaseBackupInfo(accessToken: string): Promise<AdminDatabaseBackupInfo> {
  return authedRequest<AdminDatabaseBackupInfo>("/api/v1/admin/database/backup-info", { accessToken });
}

export async function downloadAdminDatabaseBackup(accessToken: string): Promise<void> {
  const response = await authedFetch("/api/v1/admin/database/backup", { method: "GET", accessToken });
  const blob = await response.blob();
  const disposition = response.headers.get("content-disposition") || "";
  const filename = disposition.match(/filename="?([^";]+)"?/i)?.[1] || "deeix-chat-backup";
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}

export async function restoreAdminDatabaseBackup(accessToken: string, file: File): Promise<void> {
  const body = new FormData();
  body.append("file", file);
  await authedFetch("/api/v1/admin/database/restore", { method: "POST", accessToken, body });
}
