import type {
  AdminUserResponse,
  AuditLogResponse,
  ErrorLogResponse,
  AuthEventResponse,
  ConversationEventResponse,
  CreateUserRequest,
  DeleteUserResponse,
  ImportOpenWebUIUsersRequest as ContractImportOpenWebUIUsersRequest,
  ImportOpenWebUIUsersResponse,
  PatchUserRequest,
  PaymentOrderResponse,
  RedemptionRecordResponse,
  ResetUserPasswordRequest,
  ResetUserPasswordResponse,
  RevokeUserSessionsResponse,
  UpdateUserStatusRequest,
  UserDataResponse,
  UsageLogResponse,
} from "@deeix/api-contract";
import type { PagePayload } from "@/shared/api/common-types";
import type { ADMIN_USER_ROLES, ADMIN_USER_STATUSES } from "@/features/admin/model/admin-unions";

export type AdminUserStatus = (typeof ADMIN_USER_STATUSES)[number];
export type AdminUserRole = (typeof ADMIN_USER_ROLES)[number];

export type AdminUserDTO = AdminUserResponse;

export type CreateAdminUserRequest = CreateUserRequest;

export type UpdateAdminUserStatusRequest = Omit<UpdateUserStatusRequest, "status"> & {
  status: AdminUserStatus;
};

export type PatchAdminUserRequest = Omit<PatchUserRequest, "role" | "status"> & {
  role?: AdminUserRole;
  status?: AdminUserStatus;
};

export type ResetAdminUserPasswordRequest = ResetUserPasswordRequest;

export type ImportOpenWebUIUsersRequest = ContractImportOpenWebUIUsersRequest;

export type AdminUserData = Omit<UserDataResponse, "user"> & {
  user: AdminUserDTO;
};

export type RevokeAdminUserSessionsData = RevokeUserSessionsResponse;

export type ResetAdminUserPasswordData = ResetUserPasswordResponse;

export type ResetAdminUserTwoFactorData = {
  reset: boolean;
};

export type DeleteAdminUserData = DeleteUserResponse;

export type ImportOpenWebUIUsersData = Omit<ImportOpenWebUIUsersResponse, "dedupeField" | "source"> & {
  source: "openwebui";
  dedupeField: "email";
};

export type AdminUserAuthEventDTO = AuthEventResponse;

export type AdminAuditLogDTO = AuditLogResponse;
export type AdminErrorLogDTO = ErrorLogResponse;


export type AdminUsageLogDTO = Omit<UsageLogResponse, "billingAt">;

export type AdminPaymentOrderDTO = PaymentOrderResponse;

export type AdminRedemptionRecordDTO = RedemptionRecordResponse;

export type AdminConversationEventDTO = ConversationEventResponse;

export type ListAdminUsersResult = PagePayload<AdminUserDTO>;
export type ListAdminUserAuthEventsResult = PagePayload<AdminUserAuthEventDTO>;
export type ListAdminAuditLogsResult = PagePayload<AdminAuditLogDTO>;
export type ListAdminUsageLogsResult = PagePayload<AdminUsageLogDTO>;
export type ListAdminPaymentOrdersResult = PagePayload<AdminPaymentOrderDTO>;
export type ListAdminRedemptionsResult = PagePayload<AdminRedemptionRecordDTO>;
export type ListAdminConversationEventsResult = PagePayload<AdminConversationEventDTO>;

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
