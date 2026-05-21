import type { PagePayload } from "@/shared/api/common.types";
import type { UserDTO } from "@/shared/api/auth.types";

export type AdminUserStatus = "pending_activation" | "active" | "locked" | "suspended" | "deactivated";
export type AdminUserRole = "user" | "superadmin";

export type CreateAdminUserRequest = {
  username: string;
  password: string;
  avatarURL?: string;
  displayName?: string;
  email?: string;
  phone?: string;
  timezone?: string;
  locale?: string;
  subscriptionTier?: string;
  subscriptionExpiresAt?: string;
};

export type UpdateAdminUserStatusRequest = {
  status: AdminUserStatus;
  reason?: string;
};

export type PatchAdminUserRequest = {
  avatarURL?: string;
  displayName?: string;
  email?: string;
  phone?: string;
  role?: AdminUserRole;
  status?: AdminUserStatus;
  timezone?: string;
  locale?: string;
  profilePreferences?: string;
  subscriptionTier?: string;
  subscriptionExpiresAt?: string;
  reason?: string;
};

export type ResetAdminUserPasswordRequest = {
  newPassword: string;
  mustResetPassword?: boolean;
};

export type AdminUserData = {
  user: UserDTO;
};

export type RevokeAdminUserSessionsData = {
  revoked: boolean;
};

export type ResetAdminUserPasswordData = {
  reset: boolean;
};

export type ResetAdminUserTwoFactorData = {
  reset: boolean;
};

export type DeleteAdminUserData = {
  deleted: boolean;
};

export type AdminUserAuthEventDTO = {
  id: number;
  requestID: string;
  userID: number;
  username: string;
  userDisplayName: string;
  userLabel: string;
  eventType: string;
  result: string;
  reason: string;
  clientIP: string;
  userAgent: string;
  detailJSON: string;
  occurredAt: string;
  createdAt: string;
  updatedAt: string;
};

export type AdminAuditLogDTO = {
  id: number;
  requestID: string;
  actorUserID: number;
  actorUsername: string;
  actorDisplayName: string;
  actorLabel: string;
  action: string;
  resource: string;
  resourceID: string;
  ip: string;
  userAgent: string;
  detailJSON: string;
  createdAt: string;
  updatedAt: string;
};

export type AdminSystemEventDTO = {
  id: number;
  requestID: string;
  traceID: string;
  level: string;
  source: string;
  event: string;
  resource: string;
  resourceID: string;
  message: string;
  detailJSON: string;
  createdAt: string;
  updatedAt: string;
};

export type AdminUsageLogDTO = {
  id: number;
  userID: number;
  username: string;
  userDisplayName: string;
  userLabel: string;
  conversationID: number;
  providerProtocol: string;
  upstreamName: string;
  platformModelName: string;
  routedBindingCode: string;
  upstreamModelName: string;
  isFreeModel: boolean;
  usageDate: string;
  inputTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  cacheWrite5mTokens: number;
  cacheWrite1hTokens: number;
  outputTokens: number;
  reasoningTokens: number;
  callCount: number;
  durationSeconds: number;
  latencyMS: number;
  usageSpeed: string;
  serviceTier: string;
  billedCurrency: string;
  billedNanousd: number;
  billedUSD: number;
  pricingSnapshotJSON: string;
  createdAt: string;
  updatedAt: string;
};

export type ListAdminUsersResult = PagePayload<UserDTO>;
export type ListAdminUserAuthEventsResult = PagePayload<AdminUserAuthEventDTO>;
export type ListAdminAuditLogsResult = PagePayload<AdminAuditLogDTO>;
export type ListAdminSystemEventsResult = PagePayload<AdminSystemEventDTO>;
export type ListAdminUsageLogsResult = PagePayload<AdminUsageLogDTO>;

export type TikaRuntimeStatus =
  | "running"
  | "stopped"
  | "unhealthy"
  | "failed"
  | "unavailable"
  | "unconfigured"
  | "created"
  | "exited"
  | "paused"
  | "restarting";

export type AdminServiceRuntimeView = {
  source: "external" | "managed";
  baseURL: string;
  containerName: string;
  image: string;
  network: string;
  status: TikaRuntimeStatus | string;
  reachable: boolean;
  message: string;
};

export type AdminTikaRuntimeView = AdminServiceRuntimeView;
export type AdminDoclingRuntimeView = AdminServiceRuntimeView;
export type AdminTesseractRuntimeView = AdminServiceRuntimeView;
export type AdminRapidOCRRuntimeView = AdminServiceRuntimeView;
export type AdminMinerURuntimeView = AdminServiceRuntimeView;
export type AdminEmbeddingRuntimeView = AdminServiceRuntimeView;
