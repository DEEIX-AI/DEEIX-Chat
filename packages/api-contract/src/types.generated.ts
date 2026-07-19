// Generated from backend/docs/swagger.json. Do not edit manually.
// Run `pnpm api:generate` from the workspace root to regenerate.
/* eslint-disable */
/* tslint:disable */
/*
 * ---------------------------------------------------------------
 * ## THIS FILE WAS GENERATED VIA SWAGGER-TYPESCRIPT-API        ##
 * ##                                                           ##
 * ## AUTHOR: acacode                                           ##
 * ## SOURCE: https://github.com/acacode/swagger-typescript-api ##
 * ---------------------------------------------------------------
 */

export interface ActiveSessionListResponse {
  results: ActiveSessionResponse[];
  total: number;
}

export interface ActiveSessionListResponseDoc {
  data: ActiveSessionListResponse;
  errorMsg: string;
}

export interface ActiveSessionResponse {
  browserName: string;
  cityName: string;
  clientIP: string;
  countryCode: string;
  createdAt: string;
  current: boolean;
  deviceLabel: string;
  deviceName: string;
  deviceType: string;
  expiresAt: string;
  geoAccuracy: string;
  geoSource: string;
  ipLatitude: number | null;
  ipLongitude: number | null;
  lastSeenAt: string | null;
  locationLabel: string;
  osName: string;
  preciseAccuracyMeters: number | null;
  preciseLatitude: number | null;
  preciseLocatedAt: string | null;
  preciseLongitude: number | null;
  regionName: string;
  sessionID: string;
  timezoneName: string;
  updatedAt: string;
}

export interface AdminAnnouncementListResponseDoc {
  data: {
    results: AnnouncementResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface AdminErrorDoc {
  data: any;
  details?: any;
  errorCode?: string;
  errorMsg: string;
  requestId?: string;
}

export interface AdminUserIdentityProviderSummaryResponse {
  id: number;
  logoURL: string;
  name: string;
  slug: string;
  type: string;
}

export interface AdminUserResponse {
  appearancePreferences: string;
  avatarURL: string;
  billingAccountCurrency: string;
  billingAccountStatus: string;
  billingBalanceNanousd: number;
  billingBalanceUSD: number;
  createdAt: string;
  displayName: string;
  email: string;
  emailVerifiedAt: string | null;
  id: number;
  identityProviders: AdminUserIdentityProviderSummaryResponse[];
  lastActiveAt: string | null;
  lastLoginAt: string | null;
  locale: string;
  phone: string;
  phoneVerifiedAt: string | null;
  profilePreferences: string;
  publicID: string;
  role: string;
  status: string;
  subscriptionExpiresAt: string | null;
  subscriptionPlanID: number | null;
  subscriptionPlanName: string;
  subscriptionStatus: string;
  subscriptionTier: string;
  timezone: string;
  twoFactorAvailable: boolean;
  twoFactorEnabled: boolean;
  twoFactorRecoveryCount: number;
  twoFactorRequired: boolean;
  updatedAt: string;
  username: string;
}

export interface AnnouncementCloseDataResponse {
  closed: boolean;
}

export interface AnnouncementCloseResponseDoc {
  data: AnnouncementCloseDataResponse;
  errorMsg: string;
}

export interface AnnouncementDataResponse {
  announcement: AnnouncementResponse;
}

export interface AnnouncementDeleteDataResponse {
  deleted: boolean;
}

export interface AnnouncementDeleteResponseDoc {
  data: AnnouncementDeleteDataResponse;
  errorMsg: string;
}

export interface AnnouncementDismissDataResponse {
  dismissed: boolean;
}

export interface AnnouncementDismissResponseDoc {
  data: AnnouncementDismissDataResponse;
  errorMsg: string;
}

export interface AnnouncementErrorDoc {
  data: any;
  details?: any;
  /** @example "invalid_request" */
  errorCode?: string;
  /** @example "invalid request" */
  errorMsg: string;
  /** @example "" */
  requestId?: string;
}

export interface AnnouncementListResponseDoc {
  data: AnnouncementResponse[];
  errorMsg: string;
}

export interface AnnouncementResponse {
  closedAt: string | null;
  contentMarkdown: string;
  createdAt: string;
  createdByUserID: number;
  expiresAt: string | null;
  id: number;
  pinned: boolean;
  priority: number;
  startsAt: string | null;
  status: string;
  title: string;
  type: string;
  updatedAt: string;
}

export interface AnnouncementResponseDoc {
  data: AnnouncementDataResponse;
  errorMsg: string;
}

export interface AnnouncementStateRequest {
  updatedAt: string;
}

export interface AuditLogListResponseDoc {
  data: {
    results: AuditLogResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface AuditLogResponse {
  action: string;
  actorDisplayName: string;
  actorLabel: string;
  actorUserID: number;
  actorUsername: string;
  createdAt: string;
  detailJSON: string;
  id: number;
  ip: string;
  requestID: string;
  resource: string;
  resourceID: string;
  updatedAt: string;
  userAgent: string;
}

export interface AuthErrorDoc {
  data: any;
  details?: any;
  errorCode?: string;
  errorMsg: string;
  requestId?: string;
}

export interface AuthEventResponse {
  clientIP: string;
  createdAt: string;
  detailJSON: string;
  eventType: string;
  id: number;
  occurredAt: string;
  reason: string;
  requestID: string;
  result: string;
  updatedAt: string;
  userAgent: string;
  userDisplayName: string;
  userID: number;
  userLabel: string;
  username: string;
}

export interface AuthUserIdentityProviderSummaryResponse {
  id: number;
  logoURL: string;
  name: string;
  slug: string;
  type: string;
}

export interface AuthUserResponse {
  appearancePreferences: string;
  avatarURL: string;
  createdAt: string;
  displayName: string;
  email: string;
  emailBootstrapUsedAt: string | null;
  emailSource: string;
  emailVerifiedAt: string | null;
  id: number;
  identityProviders: AuthUserIdentityProviderSummaryResponse[];
  initialSecurityRequired: boolean;
  initialUsernameRequired: boolean;
  lastActiveAt: string | null;
  lastLoginAt: string | null;
  locale: string;
  mustResetPassword: boolean;
  onboardingCompletedAt: string | null;
  passwordEnabled: boolean;
  passwordOrigin: string;
  passwordSetAt: string | null;
  phone: string;
  phoneVerifiedAt: string | null;
  profilePreferences: string;
  publicID: string;
  role: string;
  status: string;
  subscriptionExpiresAt: string | null;
  subscriptionPlanID: number | null;
  subscriptionPlanName: string;
  subscriptionStatus: string;
  subscriptionTier: string;
  timezone: string;
  twoFactorAvailable: boolean;
  twoFactorEnabled: boolean;
  twoFactorRecoveryCount: number;
  twoFactorRequired: boolean;
  updatedAt: string;
  username: string;
  usernameChangedAt: string | null;
}

export interface BatchDeleteRedemptionCodeDataResponse {
  failedCount: number;
  notFoundCount: number;
  results: BatchDeleteRedemptionCodeResultResponse[];
  successCount: number;
  total: number;
}

export interface BatchDeleteRedemptionCodeRequest {
  /** @minItems 1 */
  ids: number[];
}

export interface BatchDeleteRedemptionCodeResponseDoc {
  data: BatchDeleteRedemptionCodeDataResponse;
  errorMsg: string;
}

export interface BatchDeleteRedemptionCodeResultResponse {
  error?: string;
  id: number;
  status: string;
}

export interface BatchDeleteRequest {
  /** @minItems 1 */
  ids: number[];
}

export interface BatchDeleteResponse {
  failedCount: number;
  notFoundCount: number;
  results: BatchDeleteResultResponse[];
  successCount: number;
  total: number;
}

export interface BatchDeleteResponseDoc {
  data: BatchDeleteResponse;
  errorMsg: string;
}

export interface BatchDeleteResultResponse {
  error?: string;
  id: number;
  status: string;
}

export interface BatchSetConversationProjectRequest {
  /** @maxItems 1000 */
  conversationPublicIDs: string[];
  /** @maxLength 32 */
  projectID?: string;
}

export interface BatchSetConversationProjectResponse {
  updated: number;
}

export interface BatchSetConversationProjectResponseDoc {
  data: BatchSetConversationProjectResponse;
  errorMsg: string;
}

export interface BillingAccountDataResponse {
  account: BillingAccountResponse;
}

export interface BillingAccountResponse {
  balanceNanousd: number;
  balanceUSD: number;
  currency: string;
  status: string;
  updatedAt: string;
  userID: number;
}

export interface BillingAccountResponseDoc {
  data: BillingAccountDataResponse;
  errorMsg: string;
}

export interface BillingConfigDataResponse {
  config: BillingConfigResponse;
}

export interface BillingConfigRequest {
  displayCurrency?: "USD" | "CNY";
  mode: "self" | "period" | "usage";
  nativeToolBillingEnabled?: boolean;
  nativeToolPricing?: NativeToolPricingRequest[];
  /** @min 0 */
  prepaidAmountUSD?: number;
  usdToCNYRate?: number;
}

export interface BillingConfigResponse {
  displayCurrency: string;
  epayTypes: PaymentTypeResponse[];
  mode: string;
  nativeToolBillingEnabled: boolean;
  nativeToolPricing: NativeToolPricingResponse[];
  paymentProviders: string[];
  prepaidAmountNanousd: number;
  prepaidAmountUSD: number;
  usdToCNYRate: number;
}

export interface BillingConfigResponseDoc {
  data: BillingConfigDataResponse;
  errorMsg: string;
}

export interface BillingErrorDoc {
  data: any;
  details?: any;
  errorCode?: string;
  errorMsg: string;
  requestId?: string;
}

export interface BillingOverviewDataResponse {
  overview: BillingOverviewResponse;
}

export interface BillingOverviewResponse {
  account: BillingAccountResponse | null;
  mode: string;
  periodCreditNanousd: number;
  periodCreditUSD: number;
  periodEndAt: string | null;
  periodRemainingNanousd: number;
  periodRemainingUSD: number;
  periodStartAt: string | null;
  periodUsedNanousd: number;
  periodUsedUSD: number;
  plan: BillingPlanResponse | null;
  subscriptionEntitlements: SubscriptionEntitlementResponse[];
}

export interface BillingOverviewResponseDoc {
  data: BillingOverviewDataResponse;
  errorMsg: string;
}

export interface BillingPlanDataResponse {
  plan: BillingPlanResponse;
}

export interface BillingPlanResponse {
  code: string;
  description: string;
  discountPercent: number;
  featureJSON: string;
  id: number;
  isActive: boolean;
  name: string;
  periodCreditNanousd: number;
  periodCreditUSD: number;
  permissionGroupID: number | null;
  prices: BillingPriceResponse[];
  sortOrder: number;
}

export interface BillingPlanResponseDoc {
  data: BillingPlanDataResponse;
  errorMsg: string;
}

export interface BillingPriceResponse {
  amountCents: number;
  billingInterval: string;
  code: string;
  currency: string;
  id: number;
  isDefault: boolean;
  planID: number;
}

export interface BindModelUpstreamSourceRequest {
  /** @min 0 */
  cbDurationMin?: number;
  /** @min 0 */
  cbFailureThreshold?: number;
  /** @min 0 */
  cbWindowMin?: number;
  priority?: number;
  /** @maxLength 64 */
  protocol?: string;
  status?: "active" | "inactive";
  upstreamID: number;
  upstreamModelID: number;
  weight?: number;
}

export interface BrandingManifestIcon {
  purpose: string;
  sizes: string;
  src: string;
  type: string;
}

export interface BrandingManifestResponse {
  background_color: string;
  categories: string[];
  description: string;
  display: string;
  icons: BrandingManifestIcon[];
  id: string;
  lang: string;
  name: string;
  orientation: string;
  scope: string;
  short_name: string;
  start_url: string;
  theme_color: string;
}

export interface BrandingResponse {
  appleTouchIcon180URL: string;
  pwaIcon192URL: string;
  pwaIcon512URL: string;
  pwaMaskableIcon512URL: string;
  description: string;
  faviconURL: string;
  logoURL: string;
  shortName: string;
  title: string;
}

export interface BrandingResponseDoc {
  data: BrandingResponse;
  errorMsg: string;
}

export interface ChannelErrorDoc {
  data: any;
  details?: any;
  errorCode?: string;
  errorMsg: string;
  requestId?: string;
}

export interface CheckoutDataResponse {
  checkout: CheckoutResponse;
}

export interface CheckoutResponse {
  baseAmountCents: number;
  baseCurrency: string;
  checkoutURL: string;
  creditNanousd: number;
  creditUSD: number;
  expiredAt: string | null;
  externalCheckoutID: string;
  fxRate: string;
  orderNo: string;
  orderType: string;
  payAmountCents: number;
  payCurrency: string;
  provider: string;
  status: string;
}

export interface CheckoutResponseDoc {
  data: CheckoutDataResponse;
  errorMsg: string;
}

export interface CircuitResetResponse {
  reset: boolean;
}

export interface CleanupLogsRequest {
  before: string;
  type: string;
}

export interface CleanupLogsResponse {
  before: string;
  deletedCount: number;
  type: string;
}

export interface CleanupLogsResponseDoc {
  data: CleanupLogsResponse;
  errorMsg: string;
}

export interface ContextArtifactResponse {
  content: string;
  createdAt: string;
  expiresAt?: string;
  id: number;
  kind: string;
  messageID: number;
  metadataJSON: string;
  runID: string;
  score: number;
  sourceID: string;
  sourceTitle: string;
  sourceType: string;
  tokenEstimate: number;
}

export interface ContextArtifactResponseDoc {
  data: ContextArtifactResponse;
  errorMsg: string;
}

export interface ConversationCreateResponseDoc {
  data: ConversationResponse;
  errorMsg: string;
}

export interface ConversationDefaultModelCandidateResponse {
  platformModelName: string;
  source: string;
  usedAt: string | null;
}

export interface ConversationDefaultModelCandidateResponseDoc {
  data: ConversationDefaultModelCandidateResponse;
  errorMsg: string;
}

export interface ConversationDeleteResponse {
  deleted: boolean;
  deletedFileCount?: number;
  quota?: StorageQuotaResponse;
}

export interface ConversationDeleteResponseDoc {
  data: ConversationDeleteResponse;
  errorMsg: string;
}

export interface ConversationErrorDoc {
  data: any;
  details?: any;
  errorCode?: string;
  errorMsg: string;
  requestId?: string;
}

export interface ConversationEventListResponseDoc {
  data: {
    results: ConversationEventResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface ConversationEventResponse {
  contentMarkdown: string;
  conversationID: number;
  createdAt: string;
  endedAt: string | null;
  errorJSON: string;
  eventID: string;
  eventScope: string;
  eventType: string;
  id: number;
  inputJSON: string;
  latencyMS: number;
  messageID: number;
  outputJSON: string;
  parentEventID: string;
  payloadJSON: string;
  phase: string;
  platformModelName: string;
  providerProtocol: string;
  roundID: string;
  routedBindingCode: string;
  runID: string;
  seq: number;
  stage: string;
  startedAt: string;
  status: string;
  summary: string;
  title: string;
  toolCallID: string;
  toolName: string;
  updatedAt: string;
  upstreamModelName: string;
  upstreamName: string;
  userDisplayName: string;
  userID: number;
  userLabel: string;
  username: string;
}

export interface ConversationExportCompatibilityResponse {
  format: string;
  notes: string;
}

export interface ConversationExportResponse {
  compatibility: ConversationExportCompatibilityResponse;
  conversation: ConversationResponse;
  defaultMessagePublicIDs: string[];
  exportScope: string;
  exportedAt: string;
  messages: MessageResponse[];
  runs: RunResponse[];
  totalMessages: number;
  totalRuns: number;
  version: number;
}

export interface ConversationExportResponseDoc {
  data: ConversationExportResponse;
  errorMsg: string;
}

export interface ConversationListResponseDoc {
  data: {
    results: ConversationResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface ConversationProjectListResponseDoc {
  data: ConversationProjectResponse[];
  errorMsg: string;
}

export interface ConversationProjectResponse {
  color: string;
  createdAt: string;
  defaultMCPToolIDs: number[];
  defaultSkillIDs: number[];
  description: string;
  icon: string;
  mcpDefaultMode: string;
  name: string;
  publicID: string;
  sortOrder: number;
  status: string;
  systemPrompt: string;
  updatedAt: string;
}

export interface ConversationProjectResponseDoc {
  data: ConversationProjectResponse;
  errorMsg: string;
}

export interface ConversationResponse {
  contextPolicyJSON: string;
  createdAt: string;
  isStarred: boolean;
  labelsJSON: string;
  lastCompactedAt: string | null;
  lastResponseID: string;
  lastShareAccessedAt: string | null;
  messageCount: number;
  model: string;
  projectID: string;
  projectName: string;
  provider: string;
  publicID: string;
  sessionKey: string;
  shareID: string;
  shareStatus: string;
  sharedAt: string | null;
  starredAt: string | null;
  status: string;
  title: string;
  updatedAt: string;
  userID: number;
}

export interface ConversationRunListResponseDoc {
  data: {
    results: RunResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface ConversationShareResponse {
  createdAt: string;
  lastAccessedAt: string | null;
  messageCount: number;
  modelSnapshot: string;
  revokedAt: string | null;
  shareID: string;
  status: string;
  titleSnapshot: string;
  updatedAt: string;
}

export interface ConversationShareResponseDoc {
  data: ConversationShareResponse;
  errorMsg: string;
}

export interface ConversationUpdateResponseDoc {
  data: ConversationResponse;
  errorMsg: string;
}

export interface CreateAnnouncementRequest {
  /**
   * @minLength 1
   * @maxLength 20000
   */
  contentMarkdown: string;
  expiresAt?: string | null;
  pinned?: boolean;
  priority?: number;
  startsAt?: string | null;
  status?: "active" | "inactive";
  /**
   * @minLength 1
   * @maxLength 120
   */
  title: string;
  type?: "critical" | "warning" | "info" | "normal" | "general";
}

export interface CreateCheckoutRequest {
  /** @min 0 */
  amountMinorUnits?: number;
  /** @maxLength 512 */
  cancelURL?: string;
  /**
   * @min 1
   * @max 120
   */
  cycles?: number;
  /** @maxLength 32 */
  epayType?: string;
  orderType?: "subscription" | "topup";
  paymentProvider?: "stripe" | "epay";
  /** @min 1 */
  priceID?: number;
  /** @maxLength 512 */
  successURL?: string;
}

export interface CreateConversationProjectRequest {
  /** @maxLength 32 */
  color?: string;
  /** @maxItems 128 */
  defaultMCPToolIDs?: number[];
  /** @maxItems 128 */
  defaultSkillIDs?: number[];
  /** @maxLength 255 */
  description?: string;
  /** @maxLength 32 */
  icon?: string;
  mcpDefaultMode?: "inherit" | "custom";
  /** @maxLength 80 */
  name: string;
  /** @maxLength 12000 */
  systemPrompt?: string;
}

export interface CreateConversationRequest {
  /** @maxLength 128 */
  model?: string;
  /** @maxLength 32 */
  projectID?: string;
  /** @maxLength 255 */
  title?: string;
}

export interface CreateConversationShareRequest {
  /** @maxItems 1000 */
  defaultMessagePublicIDs?: string[];
}

export interface CreateModelRequest {
  accessScope?: "public" | "internal";
  /** @maxLength 10000 */
  capabilitiesJSON?: string;
  /** @min 0 */
  cbDurationMin?: number;
  /** @min 0 */
  cbFailureThreshold?: number;
  cbPolicyMode?: "default" | "enforced";
  /** @min 0 */
  cbWindowMin?: number;
  /** @maxLength 10000 */
  description?: string;
  /** @maxLength 128 */
  icon?: string;
  /** @maxLength 1000 */
  kindsJSON?: string;
  /**
   * @minLength 2
   * @maxLength 128
   */
  platformModelName: string;
  status?: "active" | "inactive";
  /** @maxLength 20000 */
  systemPrompt?: string;
  /** @maxLength 64 */
  vendor?: string;
}

export interface CreateModelResponseDoc {
  data: ModelDataResponse;
  errorMsg: string;
}

export interface CreatePermissionGroupRequest {
  /** @maxLength 512 */
  description?: string;
  /** @maxLength 128 */
  name: string;
  /**
   * @min 0
   * @max 10000
   */
  rateMultiplierPercent?: number;
}

export interface CreateRedemptionCodeRequest {
  /**
   * @minLength 3
   * @maxLength 64
   */
  code?: string;
  /** @min 0 */
  creditUSD?: number;
  /** @maxLength 255 */
  description?: string;
  /**
   * @min 0
   * @max 3660
   */
  durationDays?: number;
  expiresAt?: string | null;
  /** @min 1 */
  maxRedemptions?: number;
  mode: "usage" | "period";
  /**
   * @min 1
   * @max 100
   */
  perUserLimit?: number;
  /** @min 1 */
  planID?: number;
  /**
   * @min 1
   * @max 100
   */
  quantity?: number;
}

export interface CreateServerRequest {
  authToken?: string;
  baseURL: string;
  headersJSON?: string;
  name: string;
  status?: string;
}

export interface CreateUpstreamRequest {
  /**
   * @minLength 2
   * @maxLength 10000
   */
  apiKeys: string;
  /** @maxLength 512 */
  baseURL: string;
  cbDurationMin?: number;
  cbFailureThreshold?: number;
  cbModelThreshold?: number;
  cbThresholdLogic?: "or" | "and";
  cbWindowMin?: number;
  compatible?:
    | "openai"
    | "anthropic"
    | "google"
    | "xai"
    | "openrouter"
    | "custom";
  connectTimeoutMS?: number;
  /** @maxLength 10000 */
  headersJSON?: string;
  /**
   * @minLength 2
   * @maxLength 128
   */
  name: string;
  /** @maxLength 10000 */
  protocolDefaultsJSON?: string;
  readTimeoutMS?: number;
  status?: "active" | "inactive";
  streamIdleTimeoutMS?: number;
}

export interface CreateUpstreamResponseDoc {
  data: UpstreamDataResponse;
  errorMsg: string;
}

export interface CreateUserRequest {
  /** @maxLength 2048 */
  avatarURL?: string;
  /**
   * @minLength 3
   * @maxLength 16
   */
  displayName?: string;
  /** @maxLength 128 */
  email?: string;
  /** @maxLength 16 */
  locale?: string;
  /**
   * @minLength 8
   * @maxLength 128
   */
  password: string;
  /** @maxLength 32 */
  phone?: string;
  subscriptionExpiresAt?: string;
  /** @maxLength 32 */
  subscriptionTier?: string;
  /** @maxLength 64 */
  timezone?: string;
  /**
   * @minLength 3
   * @maxLength 16
   */
  username: string;
}

export interface CreateUserResponseDoc {
  data: UserDataResponse;
  errorMsg: string;
}

export interface DeleteAccountRequest {
  /**
   * @minLength 6
   * @maxLength 32
   */
  code: string;
  verificationMethod: "two_factor" | "email";
}

export interface DeleteAccountResponse {
  deleted: boolean;
}

export interface DeleteAccountResponseDoc {
  data: DeleteAccountResponse;
  errorMsg: string;
}

export interface DeleteFileResponse {
  deleted: boolean;
  fileID: string;
  quota: StorageQuotaResponse;
}

export interface DeleteFileResponseDoc {
  data: DeleteFileResponse;
  errorMsg: string;
}

export interface DeletePermissionGroupResponse {
  deleted: boolean;
  summary: PermissionGroupDeleteSummaryResponse;
}

export interface DeletePermissionGroupResponseDoc {
  data: DeletePermissionGroupResponse;
  errorMsg: string;
}

export interface DeleteServerResponse {
  deleted: boolean;
}

export interface DeleteServerResponseDoc {
  data: DeleteServerResponse;
  errorMsg: string;
}

export interface DeleteUserResponse {
  deleted: boolean;
}

export interface DeleteUserResponseDoc {
  data: DeleteUserResponse;
  errorMsg: string;
}

export interface EmailRegistrationCompleteRequest {
  code?: string;
  /** @maxLength 128 */
  email: string;
  /**
   * @minLength 8
   * @maxLength 128
   */
  password: string;
  /** @maxLength 2048 */
  turnstileToken?: string;
}

export interface EmailRegistrationStartRequest {
  /** @maxLength 128 */
  email: string;
  /** @maxLength 2048 */
  turnstileToken?: string;
}

export interface EmailRegistrationStartResponse {
  expiresAt: string;
  sent: boolean;
}

export interface EmailRegistrationStartResponseDoc {
  data: EmailRegistrationStartResponse;
  errorMsg: string;
}

export interface EmailVerificationStartResponse {
  availableMethods: string[];
  expiresAt: string;
  sent: boolean;
  verificationMethod: string;
}

export interface EmailVerificationStartResponseDoc {
  data: EmailVerificationStartResponse;
  errorMsg: string;
}

export interface Envelope {
  data: any;
  details?: any;
  errorCode?: string;
  errorMsg: string;
  requestId?: string;
}

export interface FileListResponse {
  quota: StorageQuotaResponse;
  results: FileObjectResponse[];
  total: number;
}

export interface FileListResponseDoc {
  data: FileListResponse;
  errorMsg: string;
}

export interface FileObjectResponse {
  sha256: string;
  chunkCount: number;
  createdAt: string;
  detectedMIME: string;
  embedError: string;
  embedStatus: string;
  expiresAt: string | null;
  extractStatus: string;
  fileCategory: string;
  fileID: string;
  fileName: string;
  lastAccessedAt: string | null;
  mimeType: string;
  processingErrorCode: string;
  processingErrorMessage: string;
  processingReady: boolean;
  processingStatus: string;
  purpose: string;
  ragOptOut: boolean;
  sizeBytes: number;
  status: string;
  updatedAt: string;
}

export interface FileUpdateResponseDoc {
  data: FileObjectResponse;
  errorMsg: string;
}

export interface FileUploadResponse {
  file: FileObjectResponse;
  quota: StorageQuotaResponse;
  reused: boolean;
}

export interface GroupModelsResponse {
  modelIDs: number[];
  rules: PermissionGroupModelRuleResponse[];
}

export interface GroupModelsResponseDoc {
  data: GroupModelsResponse;
  errorMsg: string;
}

export interface GroupUsersResponse {
  userIDs: number[];
}

export interface GroupUsersResponseDoc {
  data: GroupUsersResponse;
  errorMsg: string;
}

export interface IdentityProviderDeleteResponse {
  deleted: boolean;
}

export interface IdentityProviderDeleteResponseDoc {
  data: IdentityProviderDeleteResponse;
  errorMsg: string;
}

export interface IdentityProviderListResponse {
  results: IdentityProviderResponse[];
  total: number;
}

export interface IdentityProviderListResponseDoc {
  data: IdentityProviderListResponse;
  errorMsg: string;
}

export interface IdentityProviderReorderResponse {
  updated: boolean;
}

export interface IdentityProviderReorderResponseDoc {
  data: IdentityProviderReorderResponse;
  errorMsg: string;
}

export interface IdentityProviderResponse {
  authURL?: string;
  avatarField: string;
  clientID?: string;
  createdAt: string;
  defaultRole: "user" | "admin" | "superadmin";
  discoveryURL?: string;
  emailField: string;
  emailVerifiedField: string;
  issuerURL?: string;
  jwksURL?: string;
  loginEnabled: boolean;
  logoURL: string;
  name: string;
  nameField: string;
  publicID: string;
  registrationEnabled: boolean;
  scopes: string;
  slug: string;
  subjectField: string;
  tokenURL?: string;
  type: "oidc" | "oauth2";
  updatedAt: string;
  userinfoURL?: string;
}

export interface IdentityProviderResponseDoc {
  data: IdentityProviderResponse;
  errorMsg: string;
}

export interface ImportOpenWebUIUsersRequest {
  creditMultiplier: number;
  dryRun?: boolean;
  /** @maxLength 2048 */
  dsn: string;
}

export interface ImportOpenWebUIUsersResponse {
  dedupeField: string;
  dedupeRule: string;
  imported: number;
  scanned: number;
  skippedDuplicateSourceEmail: number;
  skippedExistingEmail: number;
  skippedInvalidEmail: number;
  skippedInvalidRow: number;
  source: string;
}

export interface ImportOpenWebUIUsersResponseDoc {
  data: ImportOpenWebUIUsersResponse;
  errorMsg: string;
}

export interface ImportUpstreamModelItemRequest {
  /** @maxLength 1000 */
  kindsJSON?: string;
  /**
   * @minLength 2
   * @maxLength 128
   */
  platformModelName: string;
  priority?: number;
  /** @maxLength 64 */
  protocol?: string;
  protocols?: string[];
  status?: "active" | "inactive";
  /**
   * @minLength 1
   * @maxLength 128
   */
  upstreamModelName: string;
}

export interface ImportUpstreamModelResultResponse {
  bindingCode: string;
  createdPlatform: boolean;
  createdRoute: boolean;
  createdRoutes: number;
  error?: string;
  existingRoutes: number;
  platformModelName: string;
  protocols: string[];
  status: string;
  upstreamModelName: string;
}

export interface ImportUpstreamModelsRequest {
  /** @minItems 1 */
  items: ImportUpstreamModelItemRequest[];
  permissionGroupIDs?: number[];
}

export interface ImportUpstreamModelsResponse {
  createdPlatform: number;
  createdRoutes: number;
  existingRoutes: number;
  failedCount: number;
  importedCount: number;
  results: ImportUpstreamModelResultResponse[];
  total: number;
}

export interface ImportUpstreamModelsResponseDoc {
  data: ImportUpstreamModelsResponse;
  errorMsg: string;
}

export interface LoginOptionsResponse {
  emailEnabled: boolean;
  emailRegistrationEnabled: boolean;
  emailVerificationEnabled: boolean;
  passwordResetEnabled: boolean;
  providers: IdentityProviderResponse[];
  turnstileRegistrationEnabled: boolean;
  turnstileSiteKey: string;
  usernameEnabled: boolean;
}

export interface LoginOptionsResponseDoc {
  data: LoginOptionsResponse;
  errorMsg: string;
}

export interface LoginRequest {
  /**
   * @minLength 6
   * @maxLength 128
   */
  password: string;
  /**
   * @minLength 3
   * @maxLength 128
   */
  username: string;
}

export interface LoginResponse {
  accessToken: string;
  expiresAt: string;
  refreshExpiresAt: string;
  sessionID: string;
  twoFactorChallengeToken?: string;
  twoFactorRequired: boolean;
  user: AuthUserResponse;
  verificationMethods?: string[];
}

export interface LoginResponseDoc {
  data: LoginResponse;
  errorMsg: string;
}

export interface LogoutResponse {
  revoked: boolean;
}

export interface LogoutResponseDoc {
  data: LogoutResponse;
  errorMsg: string;
}

export interface McpErrorDoc {
  errorMsg: string;
}

export interface MeResponse {
  user: AuthUserResponse;
}

export interface MeResponseDoc {
  data: MeResponse;
  errorMsg: string;
}

export interface MemoryErrorDoc {
  data: any;
  details?: any;
  errorCode?: string;
  errorMsg: string;
  requestId?: string;
}

export interface MessageBillingCostResponse {
  billedCurrency: string;
  billedNanousd: number;
  billedUSD: number;
  billingMode: string;
  pricingSnapshotJSON: string;
}

export interface MessageFeedbackResponse {
  messageID: number;
  messagePublicID: string;
  myFeedback: string;
  thumbsDownCount: number;
  thumbsUpCount: number;
}

export interface MessageFeedbackResponseDoc {
  data: MessageFeedbackResponse;
  errorMsg: string;
}

export interface MessageListResponseDoc {
  data: {
    results: MessageResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface MessageProcessTraceResponse {
  enabled: boolean;
  events?: MessageTraceEventResponse[];
  process?: MessageTraceBlockResponse;
  promptTrace?: MessagePromptTraceResponse;
  status: string;
  tools?: MessageTraceBlockResponse;
  upstreamThink?: MessageTraceBlockResponse;
}

export interface MessagePromptTraceBlockResponse {
  cacheable: boolean;
  kind: string;
  sourceCount: number;
  sourceRefs?: MessagePromptTraceSourceResponse[];
  title: string;
  tokenEstimate: number;
}

export interface MessagePromptTraceResponse {
  blocks: MessagePromptTraceBlockResponse[];
  fullMessageCount: number;
  mode: string;
  promptFingerprint: string;
  sentMessageCount: number;
  sentTokenEstimate: number;
  statefulDisabledReason: string;
  statefulSavedMessages: number;
  statefulSavedTokens: number;
  statefulUsed: boolean;
  totalTokenEstimate: number;
}

export interface MessagePromptTraceSourceResponse {
  artifactID?: number;
  sourceID: string;
  sourceType: string;
  title: string;
}

export interface MessageResponse {
  attachments: string;
  billingCost?: MessageBillingCostResponse;
  branchReason: string;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  content: string;
  contentType: string;
  conversationID: number;
  createdAt: string;
  editedAt: string | null;
  errorCode: string;
  errorMessage: string;
  id: number;
  inputTokens: number;
  latencyMS: number;
  modelIcon: string;
  modelVendor: string;
  myFeedback: string;
  outputTokens: number;
  parentMessageID: number | null;
  parentPublicID: string;
  platformModelName: string;
  processTrace?: MessageProcessTraceResponse;
  publicID: string;
  reasoningTokens: number;
  role: string;
  runID: string;
  sourceMessageID: number | null;
  sourcePublicID: string;
  status: string;
  thumbsDownCount: number;
  thumbsUpCount: number;
  tokenUsage: number;
  updatedAt: string;
  upstreamModelName: string;
  userID: number;
}

export interface MessageResponseDoc {
  data: MessageResponse;
  errorMsg: string;
}

export interface MessageTraceBlockResponse {
  contentMarkdown: string;
  parentEventID?: string;
  payloadJSON?: string;
  roundID?: string;
  stage?: string;
  status: string;
  summary: string;
  title: string;
  updatedAt: string;
}

export interface MessageTraceEventResponse {
  contentMarkdown: string;
  endedAt?: string;
  eventID: string;
  eventType: string;
  parentEventID?: string;
  payloadJSON?: string;
  phase: string;
  roundID?: string;
  seq: number;
  stage?: string;
  startedAt: string;
  status: string;
  summary: string;
  title: string;
  updatedAt: string;
}

export interface ModelDataResponse {
  model: ModelResponse;
}

export interface ModelListResponseDoc {
  data: {
    results: ModelResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface ModelPermissionGroupsResponse {
  effectiveGroupIDs: number[];
  manualGroupIDs: number[];
  matchedGroupIDs: number[];
  unassigned: boolean;
}

export interface ModelPermissionGroupsResponseDoc {
  data: ModelPermissionGroupsResponse;
  errorMsg: string;
}

export interface ModelPricingDataResponse {
  modelPricing: ModelPricingResponse;
}

export interface ModelPricingListResponseDoc {
  data: {
    results: ModelPricingResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface ModelPricingResponse {
  cacheReadNanousdPerMTokens: number;
  cacheReadUSDPerMTokens: number;
  cacheWriteNanousdPerMTokens: number;
  cacheWriteUSDPerMTokens: number;
  callNanousdPerCall: number;
  callUSDPerCall: number;
  createdAt: string;
  currency: string;
  durationNanousdPerSecond: number;
  durationUSDPerSecond: number;
  id: number;
  inputNanousdPerMTokens: number;
  inputUSDPerMTokens: number;
  isFree: boolean;
  modelIcon: string;
  modelVendor: string;
  outputNanousdPerMTokens: number;
  outputUSDPerMTokens: number;
  platformModelName: string;
  pricingMode: string;
  tieredPricingJSON: string;
  updatedAt: string;
}

export interface ModelProbeBatchResponse {
  failedCount: number;
  results: ModelProbeResponse[];
  successCount: number;
  totalCount: number;
  unsupportedCount: number;
}

export interface ModelProbeBatchResponseDoc {
  data: ModelProbeBatchResponse;
  errorMsg: string;
}

export interface ModelProbeDebugRequestResponse {
  body: string;
  headers?: Record<string, string>;
  method: string;
  path: string;
}

export interface ModelProbeDebugResponse {
  request: ModelProbeDebugRequestResponse;
  response: ModelProbeDebugResponseResponse;
}

export interface ModelProbeDebugResponseResponse {
  body: string;
  headers?: Record<string, string>;
  statusCode: number;
}

export interface ModelProbeRequest {
  taskType?: "chat" | "image_generation" | "image_edit" | "video_generation";
}

export interface ModelProbeResponse {
  bindingCode: string;
  debug?: ModelProbeDebugResponse;
  endpoint: string;
  errorCode?: string;
  errorMessage?: string;
  latencyMS: number;
  platformModelID: number;
  platformModelName: string;
  protocol: string;
  routeID: number;
  status: string;
  success: boolean;
  upstreamID: number;
  upstreamModelID: number;
  upstreamModelName: string;
  upstreamName: string;
  upstreamStatusCode?: number;
}

export interface ModelProbeResponseDoc {
  data: ModelProbeResponse;
  errorMsg: string;
}

export interface ModelResponse {
  accessScope: string;
  activeSourceCount: number;
  capabilitiesJSON: string;
  cbDurationMin: number;
  cbFailureThreshold: number;
  cbPolicyMode: string;
  cbWindowMin: number;
  createdAt: string;
  description: string;
  icon: string;
  id: number;
  kindsJSON: string;
  platformModelName: string;
  protocolsJSON: string;
  sortOrder: number;
  sourceCount: number;
  status: string;
  systemPrompt: string;
  updatedAt: string;
  upstreamNamesJSON: string;
  vendor: string;
}

export interface ModelUpstreamSourceDataResponse {
  source: ModelUpstreamSourceResponse;
}

export interface ModelUpstreamSourceListResponseDoc {
  data: {
    results: ModelUpstreamSourceResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface ModelUpstreamSourceResponse {
  baseURL: string;
  bindingCode: string;
  cbDurationMin: number;
  cbFailureThreshold: number;
  cbWindowMin: number;
  circuitOpen: boolean;
  circuitScope: string;
  circuitUntil: string;
  createdAt: string;
  headersJSON: string;
  id: number;
  priority: number;
  protocol: string;
  source: string;
  status: string;
  suggestedProtocol: string;
  updatedAt: string;
  upstreamID: number;
  upstreamModelIcon: string;
  upstreamModelKindsJSON: string;
  upstreamModelName: string;
  upstreamModelStatus: string;
  upstreamModelVendor: string;
  upstreamName: string;
  upstreamStatus: string;
  weight: number;
}

export interface NativeToolPricingRequest {
  billable?: boolean;
  priceLabel?: string;
  priceNanousd?: number;
  toolKey?: string;
  unit?: string;
}

export interface NativeToolPricingResponse {
  billable: boolean;
  description: string;
  label: string;
  priceLabel: string;
  priceNanousd: number;
  provider: string;
  toolKey: string;
  type: string;
  unit: string;
}

export interface OpenRouterOfficialPricingDataResponse {
  cached: boolean;
  fetchedAt: string;
  items: OpenRouterOfficialPricingItemResponse[];
  stale: boolean;
}

export interface OpenRouterOfficialPricingItemResponse {
  canonicalSlug: string;
  id: string;
  name: string;
  pricing: OpenRouterOfficialPricingUnitPricingResponse;
}

export interface OpenRouterOfficialPricingResponseDoc {
  data: OpenRouterOfficialPricingDataResponse;
  errorMsg: string;
}

export interface OpenRouterOfficialPricingUnitPricingResponse {
  completion: string;
  inputCacheRead: string;
  inputCacheWrite: string;
  prompt: string;
}

export interface PasswordResetCompleteRequest {
  code: string;
  /** @maxLength 128 */
  email: string;
  /**
   * @minLength 8
   * @maxLength 128
   */
  newPassword: string;
}

export interface PasswordResetCompleteResponse {
  changed: boolean;
}

export interface PasswordResetCompleteResponseDoc {
  data: PasswordResetCompleteResponse;
  errorMsg: string;
}

export interface PasswordResetStartRequest {
  /** @maxLength 128 */
  email: string;
}

export interface PasswordResetStartResponse {
  expiresAt: string;
  sent: boolean;
}

export interface PasswordResetStartResponseDoc {
  data: PasswordResetStartResponse;
  errorMsg: string;
}

export interface PatchAnnouncementRequestDoc {
  /** @maxLength 20000 */
  contentMarkdown?: string;
  expiresAt?: string | null;
  pinned?: boolean;
  priority?: number;
  startsAt?: string | null;
  status?: "active" | "inactive";
  /** @maxLength 120 */
  title?: string;
  type?: "critical" | "warning" | "info" | "normal" | "general";
}

export interface PatchItem {
  clear?: boolean;
  key: string;
  namespace: string;
  value?: string;
}

export interface PatchMeRequest {
  /** @maxLength 2048 */
  appearancePreferences?: string;
  /** @maxLength 2048 */
  avatarURL?: string;
  /**
   * @minLength 3
   * @maxLength 16
   */
  displayName?: string;
  /** @maxLength 16 */
  locale?: string;
  /** @maxLength 1024 */
  profilePreferences?: string;
  /** @maxLength 64 */
  timezone?: string;
}

export interface PatchMeResponseDoc {
  data: MeResponse;
  errorMsg: string;
}

export interface PatchPromptPresetRequest {
  /** @maxLength 10000 */
  content?: string;
  /** @maxLength 256 */
  description?: string;
  enabled?: boolean;
  sortOrder?: number;
  /** @maxLength 64 */
  title?: string;
  /** @maxLength 64 */
  trigger?: string;
}

export interface PatchRedemptionCodeRequestDoc {
  /** @maxLength 255 */
  description?: string;
  expiresAt?: string | null;
  maxRedemptions?: number | null;
  /**
   * @min 1
   * @max 100
   */
  perUserLimit?: number;
  status?: "active" | "inactive";
}

export interface PatchSkillRequest {
  /** @maxLength 256 */
  description?: string;
  enabled?: boolean;
  /** @maxLength 10000 */
  markdown?: string;
  sortOrder?: number;
  /** @maxLength 64 */
  title?: string;
  /** @maxLength 64 */
  trigger?: string;
}

export interface PatchUserRequest {
  /** @maxLength 2048 */
  avatarURL?: string;
  /**
   * @minLength 3
   * @maxLength 16
   */
  displayName?: string;
  /** @maxLength 128 */
  email?: string;
  /** @maxLength 16 */
  locale?: string;
  /** @maxLength 32 */
  phone?: string;
  /** @maxLength 1024 */
  profilePreferences?: string;
  /** @maxLength 255 */
  reason?: string;
  /** @maxLength 32 */
  role?: string;
  /** @maxLength 32 */
  status?: string;
  subscriptionExpiresAt?: string;
  /** @maxLength 32 */
  subscriptionTier?: string;
  /** @maxLength 64 */
  timezone?: string;
}

export interface PatchUsernameRequest {
  /**
   * @minLength 3
   * @maxLength 16
   */
  username: string;
}

export interface PaymentOrderListResponseDoc {
  data: {
    results: PaymentOrderResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface PaymentOrderResponse {
  baseAmountCents: number;
  baseCurrency: string;
  billingInterval: string;
  createdAt: string;
  creditNanousd: number;
  creditUSD: number;
  cycles: number;
  expiredAt: string | null;
  externalCheckoutID: string;
  externalPaymentID: string;
  fxRate: string;
  id: number;
  orderNo: string;
  orderType: string;
  paidAt: string | null;
  payAmountCents: number;
  payCurrency: string;
  planID: number;
  priceID: number;
  provider: string;
  snapshotJSON: string;
  status: string;
  updatedAt: string;
  userDisplayName: string;
  userID: number;
  userLabel: string;
  username: string;
}

export interface PaymentTypeResponse {
  name: string;
  type: string;
}

export interface PermissionGroupDataResponse {
  group: PermissionGroupResponse;
}

export interface PermissionGroupDataResponseDoc {
  data: PermissionGroupDataResponse;
  errorMsg: string;
}

export interface PermissionGroupDeleteSummaryResponse {
  manualModelCount: number;
  manualUserCount: number;
  planCount: number;
  ruleCount: number;
}

export interface PermissionGroupListResponse {
  results: PermissionGroupResponse[];
}

export interface PermissionGroupListResponseDoc {
  data: PermissionGroupListResponse;
  errorMsg: string;
}

export interface PermissionGroupModelRuleRequest {
  /** @maxLength 32 */
  type: string;
  /** @maxLength 128 */
  value?: string;
}

export interface PermissionGroupModelRuleResponse {
  type: string;
  value: string;
}

export interface PermissionGroupResponse {
  createdAt: string;
  description: string;
  id: number;
  isDefault: boolean;
  manualModelCount: number;
  manualUserCount: number;
  modelCount: number;
  name: string;
  rateMultiplierPercent: number;
  ruleModelCount: number;
  subscriptionUserCount: number;
  updatedAt: string;
  userCount: number;
}

export interface PlanListResponseDoc {
  data: BillingPlanResponse[];
  errorMsg: string;
}

export interface PromptPresetDataResponse {
  promptPreset: PromptPresetResponse;
}

export interface PromptPresetDeleteDataResponse {
  deleted: boolean;
}

export interface PromptPresetDeleteResponseDoc {
  data: PromptPresetDeleteDataResponse;
  errorMsg: string;
}

export interface PromptPresetErrorDoc {
  errorMsg: string;
}

export interface PromptPresetPageResponseDoc {
  data: {
    results: PromptPresetResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface PromptPresetResponse {
  content: string;
  createdAt: string;
  createdByUserID: number;
  description: string;
  enabled: boolean;
  id: number;
  scope: string;
  sortOrder: number;
  title: string;
  trigger: string;
  updatedAt: string;
  updatedByUserID: number;
}

export interface PromptPresetResponseDoc {
  data: PromptPresetDataResponse;
  errorMsg: string;
}

export interface PublicModelListResponseDoc {
  data: PublicModelResponse[];
  errorMsg: string;
}

export interface PublicModelPricingResponse {
  cacheReadUSDPerMTokens: number;
  cacheWriteUSDPerMTokens: number;
  callUSDPerCall: number;
  currency: string;
  durationUSDPerSecond: number;
  inputUSDPerMTokens: number;
  isFree: boolean;
  mode: string;
  outputUSDPerMTokens: number;
  tiers: PublicModelPricingTierResponse[];
}

export interface PublicModelPricingTierResponse {
  cacheReadUSDPerMTokens: number;
  cacheWriteUSDPerMTokens: number;
  fromTokens: number;
  inputUSDPerMTokens: number;
  outputUSDPerMTokens: number;
  upToTokens: number | null;
}

export interface PublicModelResponse {
  capabilitiesJSON: string;
  description: string;
  icon: string;
  kindsJSON: string;
  platformModelName: string;
  pricing: PublicModelPricingResponse | null;
  protocolsJSON: string;
  sortOrder: number;
  vendor: string;
}

export interface PublicSharedConversationResponse {
  createdAt: string;
  defaultMessagePublicIDs: string[];
  lastAccessedAt: string | null;
  messages: PublicSharedMessageResponse[];
  model: string;
  shareID: string;
  title: string;
}

export interface PublicSharedConversationResponseDoc {
  data: PublicSharedConversationResponse;
  errorMsg: string;
}

export interface PublicSharedMessageResponse {
  attachments: string;
  branchReason: string;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  content: string;
  contentType: string;
  createdAt: string;
  editedAt: string | null;
  errorCode: string;
  errorMessage: string;
  inputTokens: number;
  latencyMS: number;
  modelIcon: string;
  modelVendor: string;
  outputTokens: number;
  parentPublicID: string;
  platformModelName: string;
  processTrace?: MessageProcessTraceResponse;
  publicID: string;
  reasoningTokens: number;
  role: string;
  runID: string;
  sourcePublicID: string;
  status: string;
  tokenUsage: number;
  updatedAt: string;
  upstreamModelName: string;
}

export interface RedeemCodeRequest {
  /**
   * @minLength 3
   * @maxLength 64
   */
  code: string;
}

export interface RedemptionApplyDataResponse {
  account?: BillingAccountResponse;
  overview: BillingOverviewResponse;
  redemption: RedemptionResponse;
  subscription?: SubscriptionResponse;
}

export interface RedemptionApplyResponseDoc {
  data: RedemptionApplyDataResponse;
  errorMsg: string;
}

export interface RedemptionCodeCreateDataResponse {
  results: RedemptionCodeResponse[];
}

export interface RedemptionCodeCreateResponseDoc {
  data: RedemptionCodeCreateDataResponse;
  errorMsg: string;
}

export interface RedemptionCodeDataResponse {
  code: RedemptionCodeResponse;
}

export interface RedemptionCodeDeleteDataResponse {
  deleted: boolean;
}

export interface RedemptionCodeDeleteResponseDoc {
  data: RedemptionCodeDeleteDataResponse;
  errorMsg: string;
}

export interface RedemptionCodeListDataResponse {
  results: RedemptionCodeResponse[];
  total: number;
}

export interface RedemptionCodeListResponseDoc {
  data: RedemptionCodeListDataResponse;
  errorMsg: string;
}

export interface RedemptionCodeResponse {
  code?: string;
  codeHint: string;
  createdAt: string;
  createdByUserID: number;
  creditNanousd: number;
  creditUSD: number;
  description: string;
  durationDays: number;
  expiresAt: string | null;
  id: number;
  maxRedemptions: number | null;
  mode: string;
  perUserLimit: number;
  planID: number;
  redeemedCount: number;
  remainingRedemptions: number | null;
  rewardType: string;
  status: string;
  updatedAt: string;
}

export interface RedemptionCodeResponseDoc {
  data: RedemptionCodeDataResponse;
  errorMsg: string;
}

export interface RedemptionResponse {
  balanceTransactionID: number;
  codeID: number;
  createdAt: string;
  creditNanousd: number;
  creditUSD: number;
  id: number;
  mode: string;
  planID: number;
  rewardType: string;
  subscriptionID: number;
  userID: number;
}

export interface RefreshTokenResponseDoc {
  data: LoginResponse;
  errorMsg: string;
}

export interface RenameConversationRequest {
  /** @maxLength 255 */
  title: string;
}

export interface ReorderConversationProjectsRequest {
  /** @maxItems 200 */
  projectIDs: string[];
}

export interface ReorderIdentityProvidersRequest {
  providerIDs: string[];
}

export interface ReorderModelsRequest {
  /** @minItems 1 */
  modelIDs: number[];
}

export interface ReorderServerOrderItem {
  serverID: number;
  toolIDs: number[];
}

export interface ReorderServersRequest {
  servers: ReorderServerOrderItem[];
}

export interface ResetUpstreamCircuitResponseDoc {
  data: CircuitResetResponse;
  errorMsg: string;
}

export interface ResetUserPasswordRequest {
  mustResetPassword?: boolean;
  /**
   * @minLength 8
   * @maxLength 128
   */
  newPassword: string;
}

export interface ResetUserPasswordResponse {
  reset: boolean;
}

export interface ResetUserPasswordResponseDoc {
  data: ResetUserPasswordResponse;
  errorMsg: string;
}

export interface RevokeConversationSharesRequest {
  /** @maxItems 1000 */
  conversationPublicIDs?: string[];
}

export interface RevokeConversationSharesResponse {
  revoked: boolean;
}

export interface RevokeConversationSharesResponseDoc {
  data: RevokeConversationSharesResponse;
  errorMsg: string;
}

export interface RevokeUserSessionsResponse {
  revoked: boolean;
}

export interface RevokeUserSessionsResponseDoc {
  data: RevokeUserSessionsResponse;
  errorMsg: string;
}

export interface RunResponse {
  cacheReadTokens: number;
  cacheWriteTokens: number;
  conversationID: number;
  createdAt: string;
  endedAt: string | null;
  endpoint: string;
  errorCode: string;
  errorMessage: string;
  firstTokenLatencyMS: number;
  id: number;
  inputTokens: number;
  modelIcon: string;
  modelVendor: string;
  outputTokens: number;
  platformModelName: string;
  provider: string;
  providerProtocol: string;
  reasoningTokens: number;
  requestID: string;
  requestedModelName: string;
  routedBindingCode: string;
  runID: string;
  startedAt: string;
  status: string;
  taskType: string;
  toolCallsCount: number;
  totalLatencyMS: number;
  updatedAt: string;
  upstreamID: number;
  upstreamModelID: number;
  upstreamModelName: string;
  userID: number;
}

export interface SecurityVerificationStartRequest {
  verificationMethod?: "none" | "two_factor" | "email";
}

export interface SendMessageRequest {
  branchReason?: "default" | "retry" | "edit";
  /** @maxLength 64 */
  clientRunID?: string;
  content: string;
  contentType: "text" | "markdown" | "image" | "file" | "mixed";
  /** @maxItems 20 */
  fileIDs?: string[];
  htmlVisualPrompt?: boolean;
  /** @maxLength 128 */
  model?: string;
  options?: Record<string, any>;
  /** @maxLength 32 */
  parentMessagePublicID?: string;
  /** @maxItems 128 */
  selectedToolIDs?: number[];
  /** @maxItems 128 */
  skillIDs?: number[];
  /** @maxLength 32 */
  sourceMessagePublicID?: string;
}

export interface SendMessageResponse {
  assistantMessage: MessageResponse;
  metadataRefreshHint?: string;
  userMessage: MessageResponse;
}

export interface SendMessageResponseDoc {
  data: SendMessageResponse;
  errorMsg: string;
}

export interface ServerDataResponse {
  server: ServerResponse;
}

export interface ServerDataResponseDoc {
  data: ServerDataResponse;
  errorMsg: string;
}

export interface ServerListResponse {
  results: ServerResponse[];
}

export interface ServerListResponseDoc {
  data: ServerListResponse;
  errorMsg: string;
}

export interface ServerResponse {
  activeToolCount: number;
  baseURL: string;
  createdAt: string;
  headersJSON: string;
  id: number;
  lastError: string;
  lastSyncedAt: string | null;
  name: string;
  sortOrder: number;
  status: string;
  toolCount: number;
  updatedAt: string;
}

export interface ServerToolOrderListResponse {
  results: ServerToolOrderResponse[];
}

export interface ServerToolOrderListResponseDoc {
  data: ServerToolOrderListResponse;
  errorMsg: string;
}

export interface ServerToolOrderResponse {
  server: ServerResponse;
  tools: ToolResponse[];
}

export interface SetConversationArchiveRequest {
  archived: boolean;
}

export interface SetConversationProjectRequest {
  /** @maxLength 32 */
  projectID?: string;
}

export interface SetConversationStarRequest {
  starred: boolean;
}

export interface SetGroupModelsRequest {
  modelIDs?: number[];
  rules?: PermissionGroupModelRuleRequest[];
}

export interface SetGroupUsersRequest {
  userIDs?: number[];
}

export interface SetMessageFeedbackRequest {
  feedback?: "up" | "down";
}

export interface SetModelPermissionGroupsRequest {
  groupIDs?: number[];
}

export interface SettingsPatchSettingsRequest {
  /** @minItems 1 */
  items: PatchItem[];
}

export interface SkillDataResponse {
  skill: SkillResponse;
}

export interface SkillDeleteDataResponse {
  deleted: boolean;
}

export interface SkillDeleteResponseDoc {
  data: SkillDeleteDataResponse;
  errorMsg: string;
}

export interface SkillErrorDoc {
  errorMsg: string;
}

export interface SkillPageResponseDoc {
  data: {
    results: SkillResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface SkillResponse {
  createdAt: string;
  createdByUserID: number;
  description: string;
  enabled: boolean;
  id: number;
  markdown: string;
  scope: string;
  sortOrder: number;
  title: string;
  trigger: string;
  updatedAt: string;
  updatedByUserID: number;
}

export interface SkillResponseDoc {
  data: SkillDataResponse;
  errorMsg: string;
}

export interface SkillSummaryPageResponseDoc {
  data: {
    results: SkillSummaryResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface SkillSummaryResponse {
  createdAt: string;
  description: string;
  enabled: boolean;
  id: number;
  scope: string;
  sortOrder: number;
  title: string;
  trigger: string;
  updatedAt: string;
}

export interface StorageQuotaResponse {
  createdAt: string;
  id: number;
  quotaBytes: number;
  reservedBytes: number;
  updatedAt: string;
  usedBytes: number;
  userID: number;
}

export interface SubscribeRequest {
  /**
   * @min 1
   * @max 120
   */
  cycles?: number;
  /** @min 1 */
  priceID: number;
}

export interface SubscribeResponseDoc {
  data: SubscriptionDataResponse;
  errorMsg: string;
}

export interface SubscriptionDataResponse {
  subscription: SubscriptionResponse;
}

export interface SubscriptionEntitlementResponse {
  autoRenew: boolean;
  cancelAtPeriodEnd: boolean;
  currentPeriodEndAt: string | null;
  currentPeriodStartAt: string;
  id: number;
  isCurrent: boolean;
  plan: BillingPlanResponse;
  planID: number;
  priceID: number;
  startAt: string;
  status: string;
  userID: number;
}

export interface SubscriptionResponse {
  autoRenew: boolean;
  cancelAtPeriodEnd: boolean;
  currentPeriodEndAt: string | null;
  currentPeriodStartAt: string;
  id: number;
  planID: number;
  priceID: number;
  startAt: string;
  status: string;
  userID: number;
}

export interface SuccessDoc {
  data: any;
  details?: any;
  /** @example "" */
  errorCode?: string;
  /** @example "" */
  errorMsg: string;
  /** @example "" */
  requestId?: string;
}

export interface SyncUpstreamModelsResponse {
  createdUpstreamModels: number;
  existingUpstreamModels: number;
  inactivatedModels: number;
  skippedUpstreamModels: number;
  syncedModels: UpstreamSyncModelResponse[];
  totalUpstream: number;
}

export interface SyncUpstreamModelsResponseDoc {
  data: SyncUpstreamModelsResponse;
  errorMsg: string;
}

export interface SystemEventListResponseDoc {
  data: {
    results: SystemEventResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface SystemEventResponse {
  createdAt: string;
  detailJSON: string;
  event: string;
  id: number;
  level: string;
  message: string;
  requestID: string;
  resource: string;
  resourceID: string;
  source: string;
  traceID: string;
  updatedAt: string;
}

export interface ToolListResponse {
  results: ToolResponse[];
}

export interface ToolListResponseDoc {
  data: ToolListResponse;
  errorMsg: string;
}

export interface ToolResponse {
  createdAt: string;
  description: string;
  displayName: string;
  id: number;
  inputSchemaJSON: string;
  name: string;
  serverID: number;
  serverName: string;
  sortOrder: number;
  status: string;
  updatedAt: string;
}

export interface ToolResponseDoc {
  data: ToolResponse;
  errorMsg: string;
}

export interface UpdateBillingAccountBalanceRequest {
  /** @min 0 */
  balanceUSD: number;
  /** @maxLength 255 */
  description?: string;
}

export interface UpdateBillingPlanRequest {
  /** @min 0 */
  amountUSD: number;
  billingInterval: "month" | "year" | "lifetime";
  /** @maxLength 16 */
  currency?: string;
  /** @maxLength 255 */
  description: string;
  /**
   * @min 0
   * @max 100
   */
  discountPercent: number;
  /**
   * @minLength 1
   * @maxLength 64
   */
  name: string;
  /** @min 0 */
  periodCreditUSD: number;
  permissionGroupID?: number | null;
}

export interface UpdateConversationLabelsRequest {
  /** @maxItems 6 */
  labels: string[];
}

export interface UpdateConversationProjectRequest {
  /** @maxLength 32 */
  color?: string;
  /** @maxItems 128 */
  defaultMCPToolIDs?: number[];
  /** @maxItems 128 */
  defaultSkillIDs?: number[];
  /** @maxLength 255 */
  description?: string;
  /** @maxLength 32 */
  icon?: string;
  mcpDefaultMode?: "inherit" | "custom";
  /** @maxLength 80 */
  name?: string;
  status?: "active" | "archived";
  /** @maxLength 12000 */
  systemPrompt?: string;
}

export interface UpdateCurrentSessionLocationRequest {
  /**
   * @min 0
   * @max 1000000
   */
  accuracyMeters?: number;
  latitude: number;
  longitude: number;
  /** @maxLength 64 */
  timezone?: string;
}

export interface UpdateCurrentSessionLocationResponseDoc {
  data: ActiveSessionResponse;
  errorMsg: string;
}

export interface UpdateFileRequest {
  fileName?: string;
  ragOptOut?: boolean;
}

export interface UpdateMessageRequest {
  content: string;
}

export interface UpdateModelRequest {
  accessScope?: "public" | "internal";
  /** @maxLength 10000 */
  capabilitiesJSON?: string;
  /** @min 0 */
  cbDurationMin?: number;
  /** @min 0 */
  cbFailureThreshold?: number;
  cbPolicyMode?: "default" | "enforced";
  /** @min 0 */
  cbWindowMin?: number;
  /** @maxLength 10000 */
  description?: string;
  /** @maxLength 128 */
  icon?: string;
  /** @maxLength 1000 */
  kindsJSON?: string;
  /**
   * @minLength 2
   * @maxLength 128
   */
  platformModelName?: string;
  status?: "active" | "inactive";
  /** @maxLength 20000 */
  systemPrompt?: string;
  /** @maxLength 64 */
  vendor?: string;
}

export interface UpdateModelResponseDoc {
  data: ModelDataResponse;
  errorMsg: string;
}

export interface UpdateModelUpstreamSourceRequest {
  /** @min 0 */
  cbDurationMin?: number;
  /** @min 0 */
  cbFailureThreshold?: number;
  /** @min 0 */
  cbWindowMin?: number;
  priority?: number;
  /** @maxLength 64 */
  protocol?: string;
  status?: "active" | "inactive";
  weight?: number;
}

export interface UpdateModelUpstreamSourceResponseDoc {
  data: ModelUpstreamSourceDataResponse;
  errorMsg: string;
}

export interface UpdatePermissionGroupRequest {
  /** @maxLength 512 */
  description?: string;
  /** @maxLength 128 */
  name: string;
  /**
   * @min 0
   * @max 10000
   */
  rateMultiplierPercent?: number;
}

export interface UpdateServerToolsStatusRequest {
  status: string;
  toolIDs: number[];
}

export interface UpdateToolRequest {
  description?: string;
  displayName?: string;
  status?: string;
}

export interface UpdateUpstreamRequest {
  /**
   * @minLength 2
   * @maxLength 10000
   */
  addAPIKeys?: string;
  /**
   * @minLength 2
   * @maxLength 10000
   */
  apiKeys?: string;
  /** @maxLength 512 */
  baseURL?: string;
  cbDurationMin?: number;
  cbFailureThreshold?: number;
  cbModelThreshold?: number;
  cbThresholdLogic?: "or" | "and";
  cbWindowMin?: number;
  compatible?:
    | "openai"
    | "anthropic"
    | "google"
    | "xai"
    | "openrouter"
    | "custom";
  connectTimeoutMS?: number;
  deleteAPIKeyIDs?: string[];
  /** @maxLength 10000 */
  headersJSON?: string;
  /**
   * @minLength 2
   * @maxLength 128
   */
  name?: string;
  /** @maxLength 10000 */
  protocolDefaultsJSON?: string;
  readTimeoutMS?: number;
  status?: "active" | "inactive";
  streamIdleTimeoutMS?: number;
}

export interface UpdateUpstreamResponseDoc {
  data: UpstreamDataResponse;
  errorMsg: string;
}

export interface UpdateUserStatusRequest {
  /** @maxLength 255 */
  reason?: string;
  /** @maxLength 32 */
  status: string;
}

export interface UpdateUserStatusResponseDoc {
  data: UserDataResponse;
  errorMsg: string;
}

export interface UploadFileResponseDoc {
  data: FileUploadResponse;
  errorMsg: string;
}

export interface UpsertIdentityProviderRequest {
  /** @maxLength 512 */
  authURL?: string;
  /** @maxLength 64 */
  avatarField?: string;
  /** @maxLength 255 */
  clientID: string;
  /** @maxLength 4096 */
  clientSecret?: string;
  defaultRole?: "user" | "admin" | "superadmin";
  /** @maxLength 512 */
  discoveryURL?: string;
  /** @maxLength 64 */
  emailField?: string;
  /** @maxLength 64 */
  emailVerifiedField?: string;
  /** @maxLength 512 */
  issuerURL?: string;
  /** @maxLength 512 */
  jwksURL?: string;
  loginEnabled?: boolean;
  /** @maxLength 512 */
  logoURL?: string;
  /** @maxLength 80 */
  name: string;
  /** @maxLength 64 */
  nameField?: string;
  registrationEnabled?: boolean;
  /** @maxLength 255 */
  scopes?: string;
  /** @maxLength 64 */
  slug?: string;
  /** @maxLength 64 */
  subjectField?: string;
  /** @maxLength 512 */
  tokenURL?: string;
  type: "oidc" | "oauth2";
  /** @maxLength 512 */
  userinfoURL?: string;
}

export interface UpsertMemoryResponse {
  saved: boolean;
}

export interface UpsertModelPricingRequest {
  /** @min 0 */
  cacheReadUSDPerMTokens: number;
  /** @min 0 */
  cacheWriteUSDPerMTokens: number;
  /** @min 0 */
  callUSDPerCall: number;
  /** @maxLength 16 */
  currency?: string;
  /** @min 0 */
  durationUSDPerSecond: number;
  /** @min 0 */
  inputUSDPerMTokens: number;
  isFree: boolean;
  /** @min 0 */
  outputUSDPerMTokens: number;
  /** @maxLength 128 */
  platformModelName: string;
  pricingMode: "token" | "call" | "duration" | "tiered";
  /** @maxLength 20000 */
  tieredPricingJSON?: string;
}

export interface UpsertUpstreamModelRequest {
  cbDurationMin?: number;
  cbFailureThreshold?: number;
  cbWindowMin?: number;
  /** @maxLength 10000 */
  headersJSON?: string;
  /** @maxLength 1000 */
  kindsJSON?: string;
  /**
   * @minLength 2
   * @maxLength 128
   */
  platformModelName: string;
  priority?: number;
  /** @maxLength 64 */
  protocol?: string;
  routeID?: number;
  /** @maxLength 64 */
  source?: string;
  status?: "active" | "inactive";
  /**
   * @minLength 1
   * @maxLength 128
   */
  upstreamModelName: string;
  weight?: number;
}

export interface UpsertUpstreamModelResponseDoc {
  data: UpstreamModelDataResponse;
  errorMsg: string;
}

export interface UpsertUserMemoryRequest {
  /** @maxLength 128 */
  memoryKey: string;
  scope: "profile" | "preference" | "custom";
  /** @maxLength 10000 */
  value: string;
}

export interface UpsertUserMemoryResponseDoc {
  data: UpsertMemoryResponse;
  errorMsg: string;
}

export interface UpstreamAPIKeyResponse {
  id: string;
  index: number;
  keyMasked: string;
  note: string;
  status: string;
}

export interface UpstreamDataResponse {
  upstream: UpstreamResponse;
}

export interface UpstreamListResponseDoc {
  data: {
    results: UpstreamResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface UpstreamModelDataResponse {
  binding: UpstreamModelResponse;
}

export interface UpstreamModelListResponseDoc {
  data: {
    results: UpstreamModelResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface UpstreamModelResponse {
  bindingCode: string;
  cbDurationMin: number;
  cbFailureThreshold: number;
  cbWindowMin: number;
  circuitOpen: boolean;
  circuitUntil: string;
  createdAt: string;
  headersJSON: string;
  id: number;
  modelIcon: string;
  modelKindsJSON: string;
  modelVendor: string;
  platformModelID: number;
  platformModelName: string;
  priority: number;
  protocol: string;
  routeID: number;
  routeStatus: string;
  source: string;
  suggestedProtocol: string;
  updatedAt: string;
  upstreamID: number;
  upstreamModelIcon: string;
  upstreamModelKindsJSON: string;
  upstreamModelName: string;
  upstreamModelStatus: string;
  upstreamModelVendor: string;
  weight: number;
}

export interface UpstreamRemoteModelResponse {
  alreadyBound: boolean;
  alreadySynced: boolean;
  bindingCode: string;
  boundPlatformModels: string[];
  suggestedKindsJSON: string;
  suggestedPlatformModelName: string;
  suggestedProtocol: string;
  suggestedProtocols: string[];
  upstreamModelName: string;
  upstreamModelStatus: string;
}

export interface UpstreamRemoteModelsResponse {
  items: UpstreamRemoteModelResponse[];
  total: number;
}

export interface UpstreamRemoteModelsResponseDoc {
  data: UpstreamRemoteModelsResponse;
  errorMsg: string;
}

export interface UpstreamResponse {
  activeModelsCount: number;
  apiKeyItems: UpstreamAPIKeyResponse[];
  apiKeysMasked: string;
  baseURL: string;
  cbDurationMin: number;
  cbFailureThreshold: number;
  cbModelThreshold: number;
  cbThresholdLogic: string;
  cbWindowMin: number;
  circuitOpen: boolean;
  circuitUntil: string;
  compatible: string;
  connectTimeoutMS: number;
  createdAt: string;
  headersJSON: string;
  id: number;
  modelsCount: number;
  name: string;
  protocolDefaultsJSON: string;
  readTimeoutMS: number;
  status: string;
  streamIdleTimeoutMS: number;
  updatedAt: string;
}

export interface UpstreamSyncModelResponse {
  bindingCode: string;
  created: boolean;
  kindsJSON: string;
  status: string;
  suggestedProtocol: string;
  upstreamModelName: string;
}

export interface UsageDailyListResponseDoc {
  data: UsageDailyResponse[];
  errorMsg: string;
}

export interface UsageDailyModelResponse {
  avgLatencyMS: number;
  billedNanousd: number;
  billedUSD: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  callCount: number;
  durationSeconds: number;
  inputTokens: number;
  outputTokens: number;
  platformModelName: string;
  reasoningTokens: number;
  recordCount: number;
  totalTokens: number;
}

export interface UsageDailyResponse {
  avgLatencyMS: number;
  billedNanousd: number;
  billedUSD: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  callCount: number;
  durationSeconds: number;
  inputTokens: number;
  models: UsageDailyModelResponse[];
  outputTokens: number;
  reasoningTokens: number;
  recordCount: number;
  totalTokens: number;
  usageDate: string;
}

export interface UsageLedgerListResponseDoc {
  data: {
    results: UsageLedgerResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface UsageLedgerResponse {
  cacheWrite1hTokens: number;
  cacheWrite5mTokens: number;
  billedCurrency: string;
  billedNanousd: number;
  billedUSD: number;
  billingAt: string;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  callCount: number;
  conversationID: number;
  createdAt: string;
  durationSeconds: number;
  id: number;
  inputTokens: number;
  isFreeModel: boolean;
  latencyMS: number;
  modelIcon: string;
  modelVendor: string;
  outputTokens: number;
  platformModelName: string;
  pricingSnapshotJSON: string;
  providerProtocol: string;
  reasoningTokens: number;
  routedBindingCode: string;
  serviceTier: string;
  updatedAt: string;
  upstreamModelName: string;
  usageDate: string;
  usageSpeed: string;
  userID: number;
}

export interface UsageLogListResponseDoc {
  data: {
    results: UsageLogResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface UsageLogResponse {
  cacheWrite1hTokens: number;
  cacheWrite5mTokens: number;
  billedCurrency: string;
  billedNanousd: number;
  billedUSD: number;
  billingAt: string;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  callCount: number;
  conversationID: number;
  createdAt: string;
  durationSeconds: number;
  id: number;
  inputTokens: number;
  isFreeModel: boolean;
  latencyMS: number;
  outputTokens: number;
  platformModelName: string;
  pricingSnapshotJSON: string;
  providerProtocol: string;
  reasoningTokens: number;
  routedBindingCode: string;
  serviceTier: string;
  updatedAt: string;
  upstreamModelName: string;
  upstreamName: string;
  usageDate: string;
  usageSpeed: string;
  userDisplayName: string;
  userID: number;
  userLabel: string;
  username: string;
}

export interface UsageMonthlyListResponseDoc {
  data: UsageMonthlyResponse[];
  errorMsg: string;
}

export interface UsageMonthlyResponse {
  avgLatencyMS: number;
  billedNanousd: number;
  billedUSD: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  callCount: number;
  durationSeconds: number;
  inputTokens: number;
  monthStartAt: string;
  outputTokens: number;
  reasoningTokens: number;
  recordCount: number;
  totalTokens: number;
}

export interface UsageStatisticsMetricsResponse {
  avgLatencyMS: number;
  billedNanousd: number;
  billedUSD: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  callCount: number;
  inputTokens: number;
  outputTokens: number;
  reasoningTokens: number;
  recordCount: number;
  totalTokens: number;
}

export interface UsageStatisticsModelRankResponse {
  avgLatencyMS: number;
  billedNanousd: number;
  billedUSD: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  callCount: number;
  inputTokens: number;
  outputTokens: number;
  platformModelName: string;
  reasoningTokens: number;
  recordCount: number;
  totalTokens: number;
  trend: UsageStatisticsTrendResponse[];
}

export interface UsageStatisticsResponse {
  range: {
    endDate: string;
    granularity: string;
    startDate: string;
  };
  section: string;
  topModels: UsageStatisticsModelRankResponse[];
  topUsers: UsageStatisticsUserRankResponse[];
  totals: UsageStatisticsMetricsResponse;
  trend: UsageStatisticsTrendResponse[];
}

export interface UsageStatisticsResponseDoc {
  data: UsageStatisticsResponse;
  errorMsg: string;
}

export interface UsageStatisticsTrendResponse {
  avgLatencyMS: number;
  billedNanousd: number;
  billedUSD: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  callCount: number;
  inputTokens: number;
  outputTokens: number;
  periodStart: string;
  reasoningTokens: number;
  recordCount: number;
  totalTokens: number;
}

export interface UsageStatisticsUserRankResponse {
  avgLatencyMS: number;
  billedNanousd: number;
  billedUSD: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  callCount: number;
  inputTokens: number;
  outputTokens: number;
  reasoningTokens: number;
  recordCount: number;
  totalTokens: number;
  trend: UsageStatisticsTrendResponse[];
  userDisplayName: string;
  userID: number;
  userLabel: string;
  username: string;
}

export interface UserAuthEventListResponseDoc {
  data: {
    results: AuthEventResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface UserDataResponse {
  user: AdminUserResponse;
}

export interface UserListResponseDoc {
  data: {
    results: AdminUserResponse[];
    total: number;
  };
  errorMsg: string;
}

export interface UserMemoryListResponseDoc {
  data: UserMemoryResponse[];
  errorMsg: string;
}

export interface UserMemoryResponse {
  createdAt: string;
  id: number;
  memoryKey: string;
  scope: string;
  updatedAt: string;
  updatedBy: string;
  userID: number;
  value: string;
}

export interface UserSettingsPatchSettingsRequest {
  settings: Record<string, string>;
}

export interface UserSettingsResponse {
  settings: Record<string, string>;
}

export interface UserSettingsResponseDoc {
  data: UserSettingsResponse;
  errorMsg: string;
}

export interface WritePromptPresetRequest {
  /** @maxLength 10000 */
  content: string;
  /** @maxLength 256 */
  description?: string;
  enabled?: boolean;
  sortOrder?: number;
  /** @maxLength 64 */
  title: string;
  /** @maxLength 64 */
  trigger: string;
}

export interface WriteSkillRequest {
  /** @maxLength 256 */
  description?: string;
  enabled?: boolean;
  /** @maxLength 10000 */
  markdown: string;
  sortOrder?: number;
  /** @maxLength 64 */
  title: string;
  /** @maxLength 64 */
  trigger: string;
}

export namespace Admin {
  /**
   * @description 分页查询站点公告
   * @tags admin-announcements
   * @name AnnouncementsList
   * @summary 管理员查询公告
   * @request GET:/admin/announcements
   * @secure
   */
  export namespace AnnouncementsList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 是否置顶 */
      pinned?: boolean;
      /** 搜索关键词 */
      q?: string;
      /** 状态：active/inactive */
      status?: string;
      /** 类型：critical/warning/info/normal/general */
      type?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = AdminAnnouncementListResponseDoc;
  }

  /**
   * @description 创建一条 Markdown 站点公告
   * @tags admin-announcements
   * @name AnnouncementsCreate
   * @summary 管理员创建公告
   * @request POST:/admin/announcements
   * @secure
   */
  export namespace AnnouncementsCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = CreateAnnouncementRequest;
    export type RequestHeaders = {};
    export type ResponseBody = AnnouncementResponseDoc;
  }

  /**
   * @description 软删除公告
   * @tags admin-announcements
   * @name AnnouncementsDelete
   * @summary 管理员删除公告
   * @request DELETE:/admin/announcements/{id}
   * @secure
   */
  export namespace AnnouncementsDelete {
    export type RequestParams = {
      /** 公告ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = AnnouncementDeleteResponseDoc;
  }

  /**
   * @description 更新公告标题、内容、状态、优先级和有效期
   * @tags admin-announcements
   * @name AnnouncementsPartialUpdate
   * @summary 管理员更新公告
   * @request PATCH:/admin/announcements/{id}
   * @secure
   */
  export namespace AnnouncementsPartialUpdate {
    export type RequestParams = {
      /** 公告ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = PatchAnnouncementRequestDoc;
    export type RequestHeaders = {};
    export type ResponseBody = AnnouncementResponseDoc;
  }

  /**
   * @description 管理员分页查看全量可追溯审计日志
   * @tags admin
   * @name AuditLogsList
   * @summary 管理员查询审计日志
   * @request GET:/admin/audit-logs
   * @secure
   */
  export namespace AuditLogsList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 动作 */
      action?: string;
      /** 操作人用户ID */
      actor_user_id?: number;
      /** 创建时间起点(RFC3339) */
      created_from?: string;
      /** 创建时间终点(RFC3339) */
      created_to?: string;
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 搜索关键词 */
      query?: string;
      /** 资源类型 */
      resource?: string;
      /** 排序方式 */
      sort?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = AuditLogListResponseDoc;
  }

  /**
   * @description 管理员保存第三方身份源的展示顺序
   * @tags admin-auth
   * @name AuthProviderOrderPartialUpdate
   * @summary 调整第三方身份源顺序
   * @request PATCH:/admin/auth/provider-order
   * @secure
   */
  export namespace AuthProviderOrderPartialUpdate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = ReorderIdentityProvidersRequest;
    export type RequestHeaders = {};
    export type ResponseBody = IdentityProviderReorderResponseDoc;
  }

  /**
   * @description 管理员查看已配置的 OIDC 和 OAuth2 身份源
   * @tags admin-auth
   * @name AuthProvidersList
   * @summary 获取第三方身份源列表
   * @request GET:/admin/auth/providers
   * @secure
   */
  export namespace AuthProvidersList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = IdentityProviderListResponseDoc;
  }

  /**
   * @description 管理员创建一个 OIDC 或 OAuth2 身份源
   * @tags admin-auth
   * @name AuthProvidersCreate
   * @summary 创建第三方身份源
   * @request POST:/admin/auth/providers
   * @secure
   */
  export namespace AuthProvidersCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = UpsertIdentityProviderRequest;
    export type RequestHeaders = {};
    export type ResponseBody = IdentityProviderResponseDoc;
  }

  /**
   * @description 管理员删除第三方身份源；force=true 时允许删除仍有关联用户的身份源
   * @tags admin-auth
   * @name AuthProvidersDelete
   * @summary 删除第三方身份源
   * @request DELETE:/admin/auth/providers/{provider_id}
   * @secure
   */
  export namespace AuthProvidersDelete {
    export type RequestParams = {
      /** 身份源 ID */
      providerId: string;
    };
    export type RequestQuery = {
      /** 是否强制删除 */
      force?: boolean;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = IdentityProviderDeleteResponseDoc;
  }

  /**
   * @description 管理员更新一个 OIDC 或 OAuth2 身份源
   * @tags admin-auth
   * @name AuthProvidersPartialUpdate
   * @summary 更新第三方身份源
   * @request PATCH:/admin/auth/providers/{provider_id}
   * @secure
   */
  export namespace AuthProvidersPartialUpdate {
    export type RequestParams = {
      /** 身份源 ID */
      providerId: string;
    };
    export type RequestQuery = {};
    export type RequestBody = UpsertIdentityProviderRequest;
    export type RequestHeaders = {};
    export type ResponseBody = IdentityProviderResponseDoc;
  }

  /**
   * @description 设置指定用户的按量计费余额，金额单位为美元
   * @tags admin-billing
   * @name BillingAccountsBalancePartialUpdate
   * @summary 管理员设置用户按量余额
   * @request PATCH:/admin/billing/accounts/{user_id}/balance
   * @secure
   */
  export namespace BillingAccountsBalancePartialUpdate {
    export type RequestParams = {
      /** 用户ID */
      userId: number;
    };
    export type RequestQuery = {};
    export type RequestBody = UpdateBillingAccountBalanceRequest;
    export type RequestHeaders = {};
    export type ResponseBody = BillingAccountResponseDoc;
  }

  /**
   * @description 查询当前全局计费模式
   * @tags admin-billing
   * @name BillingConfigList
   * @summary 管理员查询计费配置
   * @request GET:/admin/billing/config
   * @secure
   */
  export namespace BillingConfigList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = BillingConfigResponseDoc;
  }

  /**
   * @description 更新当前全局计费模式
   * @tags admin-billing
   * @name BillingConfigPartialUpdate
   * @summary 管理员更新计费配置
   * @request PATCH:/admin/billing/config
   * @secure
   */
  export namespace BillingConfigPartialUpdate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = BillingConfigRequest;
    export type RequestHeaders = {};
    export type ResponseBody = BillingConfigResponseDoc;
  }

  /**
   * @description 按平台模型名查询模型按量计费配置
   * @tags admin-billing
   * @name BillingModelPricesList
   * @summary 管理员查询模型按量单价
   * @request GET:/admin/billing/model-prices
   * @secure
   */
  export namespace BillingModelPricesList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 搜索关键词 */
      q?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ModelPricingListResponseDoc;
  }

  /**
   * @description 按平台模型名创建或更新模型按量计费配置，金额单位为美元
   * @tags admin-billing
   * @name BillingModelPricesUpdate
   * @summary 管理员保存模型按量单价
   * @request PUT:/admin/billing/model-prices
   * @secure
   */
  export namespace BillingModelPricesUpdate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = UpsertModelPricingRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ModelPricingDataResponse;
  }

  /**
   * @description 从 storage 缓存读取 OpenRouter 模型定价；缓存不存在、过期或 refresh=true 时由后端刷新。
   * @tags admin-billing
   * @name BillingOfficialPricingOpenrouterList
   * @summary 管理员获取 OpenRouter 官方模型定价
   * @request GET:/admin/billing/official-pricing/openrouter
   * @secure
   */
  export namespace BillingOfficialPricingOpenrouterList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 强制刷新缓存 */
      refresh?: boolean;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = OpenRouterOfficialPricingResponseDoc;
  }

  /**
   * @description 更新周期套餐基础配置与默认价格
   * @tags admin-billing
   * @name BillingPlansPartialUpdate
   * @summary 管理员更新周期套餐
   * @request PATCH:/admin/billing/plans/{id}
   * @secure
   */
  export namespace BillingPlansPartialUpdate {
    export type RequestParams = {
      /** 套餐ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = UpdateBillingPlanRequest;
    export type RequestHeaders = {};
    export type ResponseBody = BillingPlanResponseDoc;
  }

  /**
   * @description 分页查询计费兑换码配置
   * @tags admin-billing
   * @name BillingRedemptionCodesList
   * @summary 管理员查询兑换码
   * @request GET:/admin/billing/redemption-codes
   * @secure
   */
  export namespace BillingRedemptionCodesList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 可兑换性：available/expired/exhausted */
      availability?: string;
      /** 计费模式：usage/period */
      mode?: string;
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 搜索关键词 */
      q?: string;
      /** 状态：active/inactive */
      status?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = RedemptionCodeListResponseDoc;
  }

  /**
   * @description 创建手动兑换码或随机兑换码，明文只在创建响应中返回
   * @tags admin-billing
   * @name BillingRedemptionCodesCreate
   * @summary 管理员创建兑换码
   * @request POST:/admin/billing/redemption-codes
   * @secure
   */
  export namespace BillingRedemptionCodesCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = CreateRedemptionCodeRequest;
    export type RequestHeaders = {};
    export type ResponseBody = RedemptionCodeCreateResponseDoc;
  }

  /**
   * @description 批量软删除兑换码，历史兑换记录保留，删除后不可再兑换
   * @tags admin-billing
   * @name BillingRedemptionCodesBatchDeleteCreate
   * @summary 管理员批量删除兑换码
   * @request POST:/admin/billing/redemption-codes/batch-delete
   * @secure
   */
  export namespace BillingRedemptionCodesBatchDeleteCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = BatchDeleteRedemptionCodeRequest;
    export type RequestHeaders = {};
    export type ResponseBody = BatchDeleteRedemptionCodeResponseDoc;
  }

  /**
   * @description 软删除兑换码，历史兑换记录保留，删除后不可再兑换
   * @tags admin-billing
   * @name BillingRedemptionCodesDelete
   * @summary 管理员删除兑换码
   * @request DELETE:/admin/billing/redemption-codes/{id}
   * @secure
   */
  export namespace BillingRedemptionCodesDelete {
    export type RequestParams = {
      /** 兑换码ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = RedemptionCodeDeleteResponseDoc;
  }

  /**
   * @description 更新兑换码状态、次数限制、过期时间和说明，不允许修改奖励本身
   * @tags admin-billing
   * @name BillingRedemptionCodesPartialUpdate
   * @summary 管理员更新兑换码
   * @request PATCH:/admin/billing/redemption-codes/{id}
   * @secure
   */
  export namespace BillingRedemptionCodesPartialUpdate {
    export type RequestParams = {
      /** 兑换码ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = PatchRedemptionCodeRequestDoc;
    export type RequestHeaders = {};
    export type ResponseBody = RedemptionCodeResponseDoc;
  }

  /**
   * @description 解密单个兑换码明文用于复制；列表接口不会返回明文
   * @tags admin-billing
   * @name BillingRedemptionCodesCodeList
   * @summary 管理员按需复制兑换码明文
   * @request GET:/admin/billing/redemption-codes/{id}/code
   * @secure
   */
  export namespace BillingRedemptionCodesCodeList {
    export type RequestParams = {
      /** 兑换码ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = RedemptionCodeResponseDoc;
  }

  /**
   * @description 管理员分页查看全量模型调用与计费用量账本
   * @tags admin
   * @name CallLogsList
   * @summary 管理员查询模型调用日志
   * @request GET:/admin/call-logs
   * @secure
   */
  export namespace CallLogsList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 计费模式筛选：free/token/call/duration/tiered */
      billing_mode?: string;
      /** 创建时间起点(RFC3339) */
      created_from?: string;
      /** 创建时间终点(RFC3339) */
      created_to?: string;
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 平台模型名筛选 */
      platform_model_name?: string;
      /** 搜索模型、上游、绑定编码、协议 */
      query?: string;
      /** 排序方式 */
      sort?: string;
      /** 调用人用户ID */
      user_id?: number;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = UsageLogListResponseDoc;
  }

  /**
   * @description 管理员分页查看对话运行轨迹、工具、MCP 与处理事件
   * @tags admin
   * @name ConversationEventsList
   * @summary 管理员查询对话事件
   * @request GET:/admin/conversation-events
   * @secure
   */
  export namespace ConversationEventsList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 会话ID */
      conversation_id?: number;
      /** 创建时间起点(RFC3339) */
      created_from?: string;
      /** 创建时间终点(RFC3339) */
      created_to?: string;
      /** 事件范围(trace_block/trace_event/tool_call) */
      event_scope?: string;
      /** 事件类型 */
      event_type?: string;
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 搜索运行ID、事件、阶段、标题、工具名 */
      query?: string;
      /** 排序方式 */
      sort?: string;
      /** 事件状态 */
      status?: string;
      /** 用户ID */
      user_id?: number;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationEventListResponseDoc;
  }

  /**
   * @description 流式导出全量会话及消息为 NDJSON 文件，最后一行为 export_manifest 元数据
   * @tags admin
   * @name ConversationsExportList
   * @summary 管理员导出全量对话数据
   * @request GET:/admin/conversations/export
   * @secure
   */
  export namespace ConversationsExportList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = string;
  }

  /**
   * @description 管理员分页查询平台模型目录，可按 only_active 过滤
   * @tags llm
   * @name LlmModelsList
   * @summary 管理员查询模型目录
   * @request GET:/admin/llm/models
   * @secure
   */
  export namespace LlmModelsList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 仅查询启用模型 */
      only_active?: boolean;
      /** 仅查询公开且可路由模型 */
      only_available?: boolean;
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 接口协议 */
      protocol?: string;
      /** 搜索关键词 */
      q?: string;
      /** 排序：sortOrder_asc/updated_desc/id_desc/platformModelName_asc/sourceCount_desc */
      sort?: string;
      /** 状态：active/inactive */
      status?: string;
      /** 上游 ID */
      upstream?: number;
      /** 模型厂商 */
      vendor?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ModelListResponseDoc;
  }

  /**
   * @description 管理员新增平台模型目录项
   * @tags llm
   * @name LlmModelsCreate
   * @summary 管理员创建模型
   * @request POST:/admin/llm/models
   * @secure
   */
  export namespace LlmModelsCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = CreateModelRequest;
    export type RequestHeaders = {};
    export type ResponseBody = CreateModelResponseDoc;
  }

  /**
   * @description 管理员批量删除模型目录及其关联路由绑定，保留上游
   * @tags llm
   * @name LlmModelsBatchDeleteCreate
   * @summary 管理员批量删除模型
   * @request POST:/admin/llm/models/batch-delete
   * @secure
   */
  export namespace LlmModelsBatchDeleteCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = BatchDeleteRequest;
    export type RequestHeaders = {};
    export type ResponseBody = BatchDeleteResponseDoc;
  }

  /**
   * @description 管理员调整平台模型在用户侧模型选择器中的展示顺序
   * @tags llm
   * @name LlmModelsOrderCreate
   * @summary 管理员调整模型顺序
   * @request POST:/admin/llm/models/order
   * @secure
   */
  export namespace LlmModelsOrderCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = ReorderModelsRequest;
    export type RequestHeaders = {};
    export type ResponseBody = SuccessDoc;
  }

  /**
   * @description 管理员删除平台模型目录项及其关联路由绑定
   * @tags llm
   * @name LlmModelsDelete
   * @summary 管理员删除模型
   * @request DELETE:/admin/llm/models/{id}
   * @secure
   */
  export namespace LlmModelsDelete {
    export type RequestParams = {
      /** 模型ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = SuccessDoc;
  }

  /**
   * @description 管理员更新平台模型目录项
   * @tags llm
   * @name LlmModelsPartialUpdate
   * @summary 管理员更新模型
   * @request PATCH:/admin/llm/models/{id}
   * @secure
   */
  export namespace LlmModelsPartialUpdate {
    export type RequestParams = {
      /** 模型ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = UpdateModelRequest;
    export type RequestHeaders = {};
    export type ResponseBody = UpdateModelResponseDoc;
  }

  /**
   * @description 管理员分页查询指定模型在各上游上的路由来源
   * @tags llm
   * @name LlmModelsSourcesList
   * @summary 管理员查询模型上游来源
   * @request GET:/admin/llm/models/{id}/sources
   * @secure
   */
  export namespace LlmModelsSourcesList {
    export type RequestParams = {
      /** 模型ID */
      id: number;
    };
    export type RequestQuery = {
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ModelUpstreamSourceListResponseDoc;
  }

  /**
   * @description 管理员将当前平台模型绑定到一个已存在的上游模型
   * @tags llm
   * @name LlmModelsSourcesCreate
   * @summary 管理员绑定模型上游来源
   * @request POST:/admin/llm/models/{id}/sources
   * @secure
   */
  export namespace LlmModelsSourcesCreate {
    export type RequestParams = {
      /** 模型ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = BindModelUpstreamSourceRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ModelUpstreamSourceDataResponse;
  }

  /**
   * @description 管理员快速启停指定模型在某上游上的来源
   * @tags llm
   * @name LlmModelsSourcesPartialUpdate
   * @summary 管理员更新模型上游来源
   * @request PATCH:/admin/llm/models/{id}/sources/{route_id}
   * @secure
   */
  export namespace LlmModelsSourcesPartialUpdate {
    export type RequestParams = {
      /** 模型ID */
      id: number;
      /** 路由绑定ID */
      routeId: number;
    };
    export type RequestQuery = {};
    export type RequestBody = UpdateModelUpstreamSourceRequest;
    export type RequestHeaders = {};
    export type ResponseBody = UpdateModelUpstreamSourceResponseDoc;
  }

  /**
   * @description 按平台模型当前活跃路由选择一个来源执行轻量连通性测试；返回结果内的调试信息已脱敏且不包含 Base URL 或密钥
   * @tags llm
   * @name LlmModelsTestCreate
   * @summary 管理员测试平台模型路由
   * @request POST:/admin/llm/models/{id}/test
   * @secure
   */
  export namespace LlmModelsTestCreate {
    export type RequestParams = {
      /** 模型ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = ModelProbeRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ModelProbeResponseDoc;
  }

  /**
   * @description 按平台模型当前全部匹配的活跃路由并发执行轻量连通性测试；返回结果内的调试信息已脱敏且不包含 Base URL 或密钥
   * @tags llm
   * @name LlmModelsTestAllCreate
   * @summary 管理员批量测试平台模型全部路由
   * @request POST:/admin/llm/models/{id}/test-all
   * @secure
   */
  export namespace LlmModelsTestAllCreate {
    export type RequestParams = {
      /** 模型ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = ModelProbeRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ModelProbeBatchResponseDoc;
  }

  /**
   * @description 管理员查询 LLM 全局设置列表
   * @tags llm
   * @name LlmSettingsList
   * @summary 管理员查询全局设置
   * @request GET:/admin/llm/settings
   * @secure
   */
  export namespace LlmSettingsList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = SuccessDoc;
  }

  /**
   * @description 管理员更新指定 LLM 全局设置项
   * @tags llm
   * @name LlmSettingsPartialUpdate
   * @summary 管理员更新全局设置
   * @request PATCH:/admin/llm/settings/{key}
   * @secure
   */
  export namespace LlmSettingsPartialUpdate {
    export type RequestParams = {
      /** 设置键 */
      key: string;
    };
    export type RequestQuery = {};
    export type RequestBody = Record<string, string>;
    export type RequestHeaders = {};
    export type ResponseBody = SuccessDoc;
  }

  /**
   * @description 管理员分页查询 LLM 上游配置
   * @tags llm
   * @name LlmUpstreamsList
   * @summary 管理员查询上游列表
   * @request GET:/admin/llm/upstreams
   * @secure
   */
  export namespace LlmUpstreamsList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 兼容类型 */
      compatible?: string;
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 搜索关键词 */
      q?: string;
      /** 排序：id_desc/id_asc/name_asc/updated_desc */
      sort?: string;
      /** 状态：active/inactive/circuit */
      status?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = UpstreamListResponseDoc;
  }

  /**
   * @description 管理员新增上游来源配置，内部标识自动分配
   * @tags llm
   * @name LlmUpstreamsCreate
   * @summary 管理员创建上游
   * @request POST:/admin/llm/upstreams
   * @secure
   */
  export namespace LlmUpstreamsCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = CreateUpstreamRequest;
    export type RequestHeaders = {};
    export type ResponseBody = CreateUpstreamResponseDoc;
  }

  /**
   * @description 管理员批量删除上游及其关联路由绑定，保留模型目录
   * @tags llm
   * @name LlmUpstreamsBatchDeleteCreate
   * @summary 管理员批量删除上游
   * @request POST:/admin/llm/upstreams/batch-delete
   * @secure
   */
  export namespace LlmUpstreamsBatchDeleteCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = BatchDeleteRequest;
    export type RequestHeaders = {};
    export type ResponseBody = BatchDeleteResponseDoc;
  }

  /**
   * @description 管理员删除上游配置及其关联路由绑定
   * @tags llm
   * @name LlmUpstreamsDelete
   * @summary 管理员删除上游
   * @request DELETE:/admin/llm/upstreams/{id}
   * @secure
   */
  export namespace LlmUpstreamsDelete {
    export type RequestParams = {
      /** 上游ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = SuccessDoc;
  }

  /**
   * @description 管理员更新上游配置（地址、密钥、状态等）
   * @tags llm
   * @name LlmUpstreamsPartialUpdate
   * @summary 管理员更新上游
   * @request PATCH:/admin/llm/upstreams/{id}
   * @secure
   */
  export namespace LlmUpstreamsPartialUpdate {
    export type RequestParams = {
      /** 上游ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = UpdateUpstreamRequest;
    export type RequestHeaders = {};
    export type ResponseBody = UpdateUpstreamResponseDoc;
  }

  /**
   * @description 管理员手动开启上游熔断状态
   * @tags llm
   * @name LlmUpstreamsCircuitOpenCreate
   * @summary 管理员手动触发上游熔断
   * @request POST:/admin/llm/upstreams/{id}/circuit/open
   * @secure
   */
  export namespace LlmUpstreamsCircuitOpenCreate {
    export type RequestParams = {
      /** 上游ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = SuccessDoc;
  }

  /**
   * @description 管理员手动清空上游失败计数并关闭熔断状态
   * @tags llm
   * @name LlmUpstreamsCircuitResetCreate
   * @summary 管理员重置上游熔断
   * @request POST:/admin/llm/upstreams/{id}/circuit/reset
   * @secure
   */
  export namespace LlmUpstreamsCircuitResetCreate {
    export type RequestParams = {
      /** 上游ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ResetUpstreamCircuitResponseDoc;
  }

  /**
   * @description 管理员分页查询指定上游的路由绑定列表
   * @tags llm
   * @name LlmUpstreamsModelsList
   * @summary 管理员查询上游模型路由绑定
   * @request GET:/admin/llm/upstreams/{id}/models
   * @secure
   */
  export namespace LlmUpstreamsModelsList {
    export type RequestParams = {
      /** 上游ID */
      id: number;
    };
    export type RequestQuery = {
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 接口协议 */
      protocol?: string;
      /** 搜索关键词 */
      q?: string;
      /** 路由状态：bound/active/inactive */
      route_status?: string;
      /** 排序：upstream_asc/upstream_desc/platform_asc/platform_desc/status_asc/protocol_asc */
      sort?: string;
      /** 上游模型状态：active/inactive */
      upstream_status?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = UpstreamModelListResponseDoc;
  }

  /**
   * @description 管理员配置平台模型到指定上游真实模型的路由绑定与覆盖请求头
   * @tags llm
   * @name LlmUpstreamsModelsCreate
   * @summary 管理员新增或更新上游模型路由绑定
   * @request POST:/admin/llm/upstreams/{id}/models
   * @secure
   */
  export namespace LlmUpstreamsModelsCreate {
    export type RequestParams = {
      /** 上游ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = UpsertUpstreamModelRequest;
    export type RequestHeaders = {};
    export type ResponseBody = UpsertUpstreamModelResponseDoc;
  }

  /**
   * @description 管理员批量删除指定上游下的路由绑定，保留模型目录
   * @tags llm
   * @name LlmUpstreamsModelsBatchDeleteCreate
   * @summary 管理员批量删除上游模型路由绑定
   * @request POST:/admin/llm/upstreams/{id}/models/batch-delete
   * @secure
   */
  export namespace LlmUpstreamsModelsBatchDeleteCreate {
    export type RequestParams = {
      /** 上游ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = BatchDeleteRequest;
    export type RequestHeaders = {};
    export type ResponseBody = BatchDeleteResponseDoc;
  }

  /**
   * @description 选择性导入上游模型，支持绑定平台模型与自定义条目
   * @tags llm
   * @name LlmUpstreamsModelsImportCreate
   * @summary 管理员批量导入上游模型
   * @request POST:/admin/llm/upstreams/{id}/models/import
   * @secure
   */
  export namespace LlmUpstreamsModelsImportCreate {
    export type RequestParams = {
      /** 上游ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = ImportUpstreamModelsRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ImportUpstreamModelsResponseDoc;
  }

  /**
   * @description 调用上游 models 接口，仅返回可导入预览，不直接落库
   * @tags llm
   * @name LlmUpstreamsModelsRemoteList
   * @summary 管理员预览上游远程模型
   * @request GET:/admin/llm/upstreams/{id}/models/remote
   * @secure
   */
  export namespace LlmUpstreamsModelsRemoteList {
    export type RequestParams = {
      /** 上游ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = UpstreamRemoteModelsResponseDoc;
  }

  /**
   * @description 调用上游 models 接口写入上游真实模型清单，不自动绑定平台模型
   * @tags llm
   * @name LlmUpstreamsModelsSyncCreate
   * @summary 管理员同步上游模型目录
   * @request POST:/admin/llm/upstreams/{id}/models/sync
   * @secure
   */
  export namespace LlmUpstreamsModelsSyncCreate {
    export type RequestParams = {
      /** 上游ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = SyncUpstreamModelsResponseDoc;
  }

  /**
   * @description 管理员删除指定上游的路由绑定
   * @tags llm
   * @name LlmUpstreamsModelsDelete
   * @summary 管理员删除上游模型路由绑定
   * @request DELETE:/admin/llm/upstreams/{id}/models/{route_id}
   * @secure
   */
  export namespace LlmUpstreamsModelsDelete {
    export type RequestParams = {
      /** 上游ID */
      id: number;
      /** 路由绑定ID */
      routeId: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = SuccessDoc;
  }

  /**
   * @description 管理员手动开启上游模型路由绑定熔断状态
   * @tags llm
   * @name LlmUpstreamsModelsCircuitOpenCreate
   * @summary 管理员手动触发上游模型路由熔断
   * @request POST:/admin/llm/upstreams/{id}/models/{route_id}/circuit/open
   * @secure
   */
  export namespace LlmUpstreamsModelsCircuitOpenCreate {
    export type RequestParams = {
      /** 上游ID */
      id: number;
      /** 路由绑定ID */
      routeId: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = SuccessDoc;
  }

  /**
   * @description 管理员手动清空上游模型路由绑定失败计数并关闭熔断状态
   * @tags llm
   * @name LlmUpstreamsModelsCircuitResetCreate
   * @summary 管理员重置上游模型路由熔断
   * @request POST:/admin/llm/upstreams/{id}/models/{route_id}/circuit/reset
   * @secure
   */
  export namespace LlmUpstreamsModelsCircuitResetCreate {
    export type RequestParams = {
      /** 上游ID */
      id: number;
      /** 路由绑定ID */
      routeId: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ResetUpstreamCircuitResponseDoc;
  }

  /**
   * @description 管理员停用该路由绑定，后续路由不会选中
   * @tags llm
   * @name LlmUpstreamsModelsDisablePartialUpdate
   * @summary 管理员停用上游模型路由绑定
   * @request PATCH:/admin/llm/upstreams/{id}/models/{route_id}/disable
   * @secure
   */
  export namespace LlmUpstreamsModelsDisablePartialUpdate {
    export type RequestParams = {
      /** 上游ID */
      id: number;
      /** 路由绑定ID */
      routeId: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = SuccessDoc;
  }

  /**
   * @description 管理员启用该路由绑定，使该上游模型重新参与路由
   * @tags llm
   * @name LlmUpstreamsModelsEnablePartialUpdate
   * @summary 管理员启用上游模型路由绑定
   * @request PATCH:/admin/llm/upstreams/{id}/models/{route_id}/enable
   * @secure
   */
  export namespace LlmUpstreamsModelsEnablePartialUpdate {
    export type RequestParams = {
      /** 上游ID */
      id: number;
      /** 路由绑定ID */
      routeId: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = SuccessDoc;
  }

  /**
   * @description 使用指定路由绑定的当前上游配置执行一次轻量连通性测试；返回结果内的调试信息已脱敏且不包含 Base URL 或密钥
   * @tags llm
   * @name LlmUpstreamsModelsTestCreate
   * @summary 管理员测试上游模型路由绑定
   * @request POST:/admin/llm/upstreams/{id}/models/{route_id}/test
   * @secure
   */
  export namespace LlmUpstreamsModelsTestCreate {
    export type RequestParams = {
      /** 上游ID */
      id: number;
      /** 路由绑定ID */
      routeId: number;
    };
    export type RequestQuery = {};
    export type RequestBody = ModelProbeRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ModelProbeResponseDoc;
  }

  /**
   * @description 按日志类型物理删除指定时间点之前的日志；操作不可恢复
   * @tags admin
   * @name LogsCleanupCreate
   * @summary 管理员清理日志
   * @request POST:/admin/logs/cleanup
   * @secure
   */
  export namespace LogsCleanupCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = CleanupLogsRequest;
    export type RequestHeaders = {};
    export type ResponseBody = CleanupLogsResponseDoc;
  }

  /**
   * @description 管理员查看已配置的 MCP 服务及其工具统计
   * @tags admin-mcp
   * @name McpServersList
   * @summary 获取 MCP 服务列表
   * @request GET:/admin/mcp/servers
   * @secure
   */
  export namespace McpServersList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ServerListResponseDoc;
  }

  /**
   * @description 管理员创建一个 MCP 服务配置
   * @tags admin-mcp
   * @name McpServersCreate
   * @summary 创建 MCP 服务
   * @request POST:/admin/mcp/servers
   * @secure
   */
  export namespace McpServersCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = CreateServerRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ServerDataResponseDoc;
  }

  /**
   * @description 管理员保存 MCP 服务及其工具的展示顺序
   * @tags admin-mcp
   * @name McpServersOrderPartialUpdate
   * @summary 调整 MCP 服务及工具顺序
   * @request PATCH:/admin/mcp/servers/order
   * @secure
   */
  export namespace McpServersOrderPartialUpdate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = ReorderServersRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ServerToolOrderListResponseDoc;
  }

  /**
   * @description 管理员删除一个 MCP 服务及其工具
   * @tags admin-mcp
   * @name McpServersDelete
   * @summary 删除 MCP 服务
   * @request DELETE:/admin/mcp/servers/{id}
   * @secure
   */
  export namespace McpServersDelete {
    export type RequestParams = {
      /** MCP 服务 ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = DeleteServerResponseDoc;
  }

  /**
   * @description 管理员更新一个 MCP 服务配置
   * @tags admin-mcp
   * @name McpServersPartialUpdate
   * @summary 更新 MCP 服务
   * @request PATCH:/admin/mcp/servers/{id}
   * @secure
   */
  export namespace McpServersPartialUpdate {
    export type RequestParams = {
      /** MCP 服务 ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = CreateServerRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ServerDataResponseDoc;
  }

  /**
   * @description 管理员从 MCP 服务同步工具定义
   * @tags admin-mcp
   * @name McpServersSyncCreate
   * @summary 同步 MCP 工具
   * @request POST:/admin/mcp/servers/{id}/sync
   * @secure
   */
  export namespace McpServersSyncCreate {
    export type RequestParams = {
      /** MCP 服务 ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ToolListResponseDoc;
  }

  /**
   * @description 管理员查看指定 MCP 服务已同步的工具
   * @tags admin-mcp
   * @name McpServersToolsList
   * @summary 获取 MCP 服务工具
   * @request GET:/admin/mcp/servers/{id}/tools
   * @secure
   */
  export namespace McpServersToolsList {
    export type RequestParams = {
      /** MCP 服务 ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ToolListResponseDoc;
  }

  /**
   * @description 管理员批量启用或停用指定 MCP 服务的工具
   * @tags admin-mcp
   * @name McpServersToolsStatusPartialUpdate
   * @summary 批量更新 MCP 工具状态
   * @request PATCH:/admin/mcp/servers/{id}/tools/status
   * @secure
   */
  export namespace McpServersToolsStatusPartialUpdate {
    export type RequestParams = {
      /** MCP 服务 ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = UpdateServerToolsStatusRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ToolListResponseDoc;
  }

  /**
   * @description 管理员更新 MCP 工具的展示信息或状态
   * @tags admin-mcp
   * @name McpToolsPartialUpdate
   * @summary 更新 MCP 工具
   * @request PATCH:/admin/mcp/tools/{id}
   * @secure
   */
  export namespace McpToolsPartialUpdate {
    export type RequestParams = {
      /** MCP 工具 ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = UpdateToolRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ToolResponseDoc;
  }

  /**
   * @description 返回平台模型的手动权限组与动态规则命中的有效权限组
   * @tags admin
   * @name ModelsPermissionGroupsList
   * @summary 管理员列出模型权限组
   * @request GET:/admin/models/{modelID}/permission-groups
   * @secure
   */
  export namespace ModelsPermissionGroupsList {
    export type RequestParams = {
      /** 平台模型ID */
      modelId: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ModelPermissionGroupsResponseDoc;
  }

  /**
   * @description 全量替换平台模型的手动权限组，不影响权限组动态规则
   * @tags admin
   * @name ModelsPermissionGroupsUpdate
   * @summary 管理员设置模型手动权限组
   * @request PUT:/admin/models/{modelID}/permission-groups
   * @secure
   */
  export namespace ModelsPermissionGroupsUpdate {
    export type RequestParams = {
      /** 平台模型ID */
      modelId: number;
    };
    export type RequestQuery = {};
    export type RequestBody = SetModelPermissionGroupsRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ModelPermissionGroupsResponseDoc;
  }

  /**
   * @description 管理员分页查看订阅和充值支付单
   * @tags admin
   * @name PaymentOrdersList
   * @summary 管理员查询支付订单记录
   * @request GET:/admin/payment-orders
   * @secure
   */
  export namespace PaymentOrdersList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 创建时间起点(RFC3339) */
      created_from?: string;
      /** 创建时间终点(RFC3339) */
      created_to?: string;
      /** 订单类型(subscription/topup) */
      order_type?: string;
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 支付渠道 */
      provider?: string;
      /** 搜索订单号、支付渠道、外部支付ID */
      query?: string;
      /** 排序方式 */
      sort?: string;
      /** 支付状态 */
      status?: string;
      /** 用户ID */
      user_id?: number;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = PaymentOrderListResponseDoc;
  }

  /**
   * @description 返回全部模型访问权限组，默认组优先
   * @tags admin
   * @name PermissionGroupsList
   * @summary 管理员列出权限组
   * @request GET:/admin/permission-groups
   * @secure
   */
  export namespace PermissionGroupsList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = PermissionGroupListResponseDoc;
  }

  /**
   * @description 创建模型访问权限组并设置计费倍率
   * @tags admin
   * @name PermissionGroupsCreate
   * @summary 管理员创建权限组
   * @request POST:/admin/permission-groups
   * @secure
   */
  export namespace PermissionGroupsCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = CreatePermissionGroupRequest;
    export type RequestHeaders = {};
    export type ResponseBody = PermissionGroupDataResponseDoc;
  }

  /**
   * @description 删除权限组及其模型、用户关联；默认组和被套餐引用的权限组不可删除
   * @tags admin
   * @name PermissionGroupsDelete
   * @summary 管理员删除权限组
   * @request DELETE:/admin/permission-groups/{id}
   * @secure
   */
  export namespace PermissionGroupsDelete {
    export type RequestParams = {
      /** 权限组ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = DeletePermissionGroupResponseDoc;
  }

  /**
   * @description 更新权限组名称、说明与计费倍率
   * @tags admin
   * @name PermissionGroupsPartialUpdate
   * @summary 管理员更新权限组
   * @request PATCH:/admin/permission-groups/{id}
   * @secure
   */
  export namespace PermissionGroupsPartialUpdate {
    export type RequestParams = {
      /** 权限组ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = UpdatePermissionGroupRequest;
    export type RequestHeaders = {};
    export type ResponseBody = PermissionGroupDataResponseDoc;
  }

  /**
   * @description 返回权限组授权的平台模型 ID 集合
   * @tags admin
   * @name PermissionGroupsModelsList
   * @summary 管理员列出权限组模型
   * @request GET:/admin/permission-groups/{id}/models
   * @secure
   */
  export namespace PermissionGroupsModelsList {
    export type RequestParams = {
      /** 权限组ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = GroupModelsResponseDoc;
  }

  /**
   * @description 全量替换权限组授权的平台模型 ID 集合与动态访问规则
   * @tags admin
   * @name PermissionGroupsModelsUpdate
   * @summary 管理员设置权限组模型
   * @request PUT:/admin/permission-groups/{id}/models
   * @secure
   */
  export namespace PermissionGroupsModelsUpdate {
    export type RequestParams = {
      /** 权限组ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = SetGroupModelsRequest;
    export type RequestHeaders = {};
    export type ResponseBody = GroupModelsResponseDoc;
  }

  /**
   * @description 返回权限组内的用户 ID 集合
   * @tags admin
   * @name PermissionGroupsUsersList
   * @summary 管理员列出权限组用户
   * @request GET:/admin/permission-groups/{id}/users
   * @secure
   */
  export namespace PermissionGroupsUsersList {
    export type RequestParams = {
      /** 权限组ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = GroupUsersResponseDoc;
  }

  /**
   * @description 全量替换权限组内的用户 ID 集合
   * @tags admin
   * @name PermissionGroupsUsersUpdate
   * @summary 管理员设置权限组用户
   * @request PUT:/admin/permission-groups/{id}/users
   * @secure
   */
  export namespace PermissionGroupsUsersUpdate {
    export type RequestParams = {
      /** 权限组ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = SetGroupUsersRequest;
    export type RequestHeaders = {};
    export type ResponseBody = GroupUsersResponseDoc;
  }

  /**
   * No description
   * @tags admin-prompt-presets
   * @name PromptPresetsList
   * @summary 管理员查询内置提示词
   * @request GET:/admin/prompt-presets
   * @secure
   */
  export namespace PromptPresetsList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 是否启用 */
      enabled?: boolean;
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 搜索关键词 */
      q?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = PromptPresetPageResponseDoc;
  }

  /**
   * No description
   * @tags admin-prompt-presets
   * @name PromptPresetsCreate
   * @summary 管理员创建内置提示词
   * @request POST:/admin/prompt-presets
   * @secure
   */
  export namespace PromptPresetsCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = WritePromptPresetRequest;
    export type RequestHeaders = {};
    export type ResponseBody = PromptPresetResponseDoc;
  }

  /**
   * No description
   * @tags admin-prompt-presets
   * @name PromptPresetsDelete
   * @summary 管理员删除内置提示词
   * @request DELETE:/admin/prompt-presets/{id}
   * @secure
   */
  export namespace PromptPresetsDelete {
    export type RequestParams = {
      /** 提示词ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = PromptPresetDeleteResponseDoc;
  }

  /**
   * No description
   * @tags admin-prompt-presets
   * @name PromptPresetsPartialUpdate
   * @summary 管理员更新内置提示词
   * @request PATCH:/admin/prompt-presets/{id}
   * @secure
   */
  export namespace PromptPresetsPartialUpdate {
    export type RequestParams = {
      /** 提示词ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = PatchPromptPresetRequest;
    export type RequestHeaders = {};
    export type ResponseBody = PromptPresetResponseDoc;
  }

  /**
   * @description 按 namespace 分组返回全部动态配置项
   * @tags admin/settings
   * @name SettingsList
   * @summary 查询全部动态配置
   * @request GET:/admin/settings
   * @secure
   */
  export namespace SettingsList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * @description 批量更新动态配置并清除缓存，下次读取自动刷新
   * @tags admin/settings
   * @name SettingsPartialUpdate
   * @summary 批量更新配置项
   * @request PATCH:/admin/settings
   * @secure
   */
  export namespace SettingsPartialUpdate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = SettingsPatchSettingsRequest;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * No description
   * @tags admin/settings
   * @name SettingsDoclingRuntimeList
   * @summary 查询 Docling 运行状态
   * @request GET:/admin/settings/docling/runtime
   * @secure
   */
  export namespace SettingsDoclingRuntimeList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * No description
   * @tags admin/settings
   * @name SettingsEmbeddingReindexCreate
   * @summary 触发向量重建（重索引所有 stale/failed 文件）
   * @request POST:/admin/settings/embedding/reindex
   * @secure
   */
  export namespace SettingsEmbeddingReindexCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * No description
   * @tags admin/settings
   * @name SettingsEmbeddingRuntimeList
   * @summary 查询 Embedding 服务运行状态
   * @request GET:/admin/settings/embedding/runtime
   * @secure
   */
  export namespace SettingsEmbeddingRuntimeList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * No description
   * @tags admin/settings
   * @name SettingsEmbeddingStatusList
   * @summary 查询向量索引健康状态
   * @request GET:/admin/settings/embedding/status
   * @secure
   */
  export namespace SettingsEmbeddingStatusList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * No description
   * @tags admin/settings
   * @name SettingsMineruRuntimeList
   * @summary 查询 MinerU 运行状态
   * @request GET:/admin/settings/mineru/runtime
   * @secure
   */
  export namespace SettingsMineruRuntimeList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * No description
   * @tags admin/settings
   * @name SettingsRapidocrRuntimeList
   * @summary 查询 RapidOCR 运行状态
   * @request GET:/admin/settings/rapidocr/runtime
   * @secure
   */
  export namespace SettingsRapidocrRuntimeList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * No description
   * @tags admin/settings
   * @name SettingsRapidocrRuntimeRestartCreate
   * @summary 重启托管 RapidOCR
   * @request POST:/admin/settings/rapidocr/runtime/restart
   * @secure
   */
  export namespace SettingsRapidocrRuntimeRestartCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * No description
   * @tags admin/settings
   * @name SettingsRapidocrRuntimeStartCreate
   * @summary 启动托管 RapidOCR
   * @request POST:/admin/settings/rapidocr/runtime/start
   * @secure
   */
  export namespace SettingsRapidocrRuntimeStartCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * No description
   * @tags admin/settings
   * @name SettingsRapidocrRuntimeStopCreate
   * @summary 停止托管 RapidOCR
   * @request POST:/admin/settings/rapidocr/runtime/stop
   * @secure
   */
  export namespace SettingsRapidocrRuntimeStopCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * No description
   * @tags admin/settings
   * @name SettingsTesseractRuntimeList
   * @summary 查询 Tesseract OCR 运行状态
   * @request GET:/admin/settings/tesseract/runtime
   * @secure
   */
  export namespace SettingsTesseractRuntimeList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * No description
   * @tags admin/settings
   * @name SettingsTikaRuntimeList
   * @summary 查询 Tika 运行状态
   * @request GET:/admin/settings/tika/runtime
   * @secure
   */
  export namespace SettingsTikaRuntimeList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * No description
   * @tags admin/settings
   * @name SettingsTikaRuntimeRestartCreate
   * @summary 重启托管 Tika
   * @request POST:/admin/settings/tika/runtime/restart
   * @secure
   */
  export namespace SettingsTikaRuntimeRestartCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * No description
   * @tags admin/settings
   * @name SettingsTikaRuntimeStartCreate
   * @summary 启动托管 Tika
   * @request POST:/admin/settings/tika/runtime/start
   * @secure
   */
  export namespace SettingsTikaRuntimeStartCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * No description
   * @tags admin/settings
   * @name SettingsTikaRuntimeStopCreate
   * @summary 停止托管 Tika
   * @request POST:/admin/settings/tika/runtime/stop
   * @secure
   */
  export namespace SettingsTikaRuntimeStopCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * @description 查询指定 namespace 下的全部配置项
   * @tags admin/settings
   * @name SettingsDetail
   * @summary 查询指定 namespace 的配置
   * @request GET:/admin/settings/{namespace}
   * @secure
   */
  export namespace SettingsDetail {
    export type RequestParams = {
      /** 命名空间 */
      namespace: string;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * No description
   * @tags admin-skills
   * @name SkillsList
   * @summary 管理员查询内置技能
   * @request GET:/admin/skills
   * @secure
   */
  export namespace SkillsList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 是否启用 */
      enabled?: boolean;
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 搜索关键词 */
      q?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = SkillPageResponseDoc;
  }

  /**
   * No description
   * @tags admin-skills
   * @name SkillsCreate
   * @summary 管理员创建内置技能
   * @request POST:/admin/skills
   * @secure
   */
  export namespace SkillsCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = WriteSkillRequest;
    export type RequestHeaders = {};
    export type ResponseBody = SkillResponseDoc;
  }

  /**
   * No description
   * @tags admin-skills
   * @name SkillsDelete
   * @summary 管理员删除内置技能
   * @request DELETE:/admin/skills/{id}
   * @secure
   */
  export namespace SkillsDelete {
    export type RequestParams = {
      /** 技能ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = SkillDeleteResponseDoc;
  }

  /**
   * No description
   * @tags admin-skills
   * @name SkillsPartialUpdate
   * @summary 管理员更新内置技能
   * @request PATCH:/admin/skills/{id}
   * @secure
   */
  export namespace SkillsPartialUpdate {
    export type RequestParams = {
      /** 技能ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = PatchSkillRequest;
    export type RequestHeaders = {};
    export type ResponseBody = SkillResponseDoc;
  }

  /**
   * @description 管理员分页查看后台结构化系统事件
   * @tags admin
   * @name SystemEventsList
   * @summary 管理员查询系统事件
   * @request GET:/admin/system-events
   * @secure
   */
  export namespace SystemEventsList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 创建时间起点(RFC3339) */
      created_from?: string;
      /** 创建时间终点(RFC3339) */
      created_to?: string;
      /** 事件 */
      event?: string;
      /** 级别 */
      level?: string;
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 搜索关键词 */
      query?: string;
      /** 排序方式 */
      sort?: string;
      /** 来源 */
      source?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = SystemEventListResponseDoc;
  }

  /**
   * @description 管理员按日期、统计对象、平台模型和计费范围查看全局费用、Token、调用次数及排名；用户与权限组筛选互斥
   * @tags admin
   * @name UsageStatisticsList
   * @summary 管理员查询全局用量统计
   * @request GET:/admin/usage-statistics
   * @secure
   */
  export namespace UsageStatisticsList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 计费范围：all/free/billable */
      billing_scope?: string;
      /** 结束日期(YYYY-MM-DD，包含当日) */
      end_date?: string;
      /** 模型排名指标：cost/tokens/calls */
      model_rank_by?: string;
      /** 权限组ID，与 user_id 互斥 */
      permission_group_id?: number;
      /** 平台模型名 */
      platform_model_name?: string;
      /** 返回范围：all/models/users */
      section?: string;
      /** 开始日期(YYYY-MM-DD)，默认近30天 */
      start_date?: string;
      /** 用户ID */
      user_id?: number;
      /** 用户排名指标：cost/tokens/calls */
      user_rank_by?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = UsageStatisticsResponseDoc;
  }

  /**
   * @description 管理员分页查询认证事件，支持 user_id/event_type/result 过滤
   * @tags admin
   * @name UserAuthEventsList
   * @summary 管理员查询用户认证事件
   * @request GET:/admin/user-auth-events
   * @secure
   */
  export namespace UserAuthEventsList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 事件类型过滤 */
      event_type?: string;
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 结果过滤(success/failure/blocked) */
      result?: string;
      /** 用户ID过滤 */
      user_id?: number;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = UserAuthEventListResponseDoc;
  }

  /**
   * @description 管理员分页查看所有用户，实现账户隔离管理
   * @tags admin
   * @name UsersList
   * @summary 管理员查询用户
   * @request GET:/admin/users
   * @secure
   */
  export namespace UsersList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 身份源 slug 过滤 */
      identity_provider?: string;
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 搜索用户名、昵称、邮箱或公开ID */
      q?: string;
      /** 订阅状态过滤(active/free) */
      subscription_status?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = UserListResponseDoc;
  }

  /**
   * @description 创建普通用户账号；需要授予管理员权限时，可在账户编辑中调整角色
   * @tags admin
   * @name UsersCreate
   * @summary 管理员创建用户
   * @request POST:/admin/users
   * @secure
   */
  export namespace UsersCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = CreateUserRequest;
    export type RequestHeaders = {};
    export type ResponseBody = CreateUserResponseDoc;
  }

  /**
   * @description 从 OpenWebUI SQLite 或 PostgreSQL 数据库读取用户，按 email 去重导入；已存在用户不会修改
   * @tags admin
   * @name UsersImportOpenwebuiCreate
   * @summary 管理员导入 OpenWebUI 用户
   * @request POST:/admin/users/import/openwebui
   * @secure
   */
  export namespace UsersImportOpenwebuiCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = ImportOpenWebUIUsersRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ImportOpenWebUIUsersResponseDoc;
  }

  /**
   * @description 管理员硬删除指定普通用户及其主要用户域数据
   * @tags admin
   * @name UsersDelete
   * @summary 管理员删除用户
   * @request DELETE:/admin/users/{id}
   * @secure
   */
  export namespace UsersDelete {
    export type RequestParams = {
      /** 用户ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = DeleteUserResponseDoc;
  }

  /**
   * @description 管理员统一维护角色、状态、时区等可编辑字段
   * @tags admin
   * @name UsersPartialUpdate
   * @summary 管理员更新用户可编辑字段
   * @request PATCH:/admin/users/{id}
   * @secure
   */
  export namespace UsersPartialUpdate {
    export type RequestParams = {
      /** 用户ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = PatchUserRequest;
    export type RequestHeaders = {};
    export type ResponseBody = UpdateUserStatusResponseDoc;
  }

  /**
   * @description 管理员重置指定用户密码并吊销其全部会话
   * @tags admin
   * @name UsersResetPasswordCreate
   * @summary 管理员重置用户密码
   * @request POST:/admin/users/{id}/reset-password
   * @secure
   */
  export namespace UsersResetPasswordCreate {
    export type RequestParams = {
      /** 用户ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = ResetUserPasswordRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ResetUserPasswordResponseDoc;
  }

  /**
   * @description 管理员吊销指定用户全部活跃会话，用于安全治理和风险控制
   * @tags admin
   * @name UsersRevokeSessionsCreate
   * @summary 管理员吊销用户全部会话
   * @request POST:/admin/users/{id}/revoke-sessions
   * @secure
   */
  export namespace UsersRevokeSessionsCreate {
    export type RequestParams = {
      /** 用户ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = RevokeUserSessionsResponseDoc;
  }

  /**
   * @description 管理员维护用户状态（active/locked/suspended/deactivated），并联动会话治理
   * @tags admin
   * @name UsersStatusPartialUpdate
   * @summary 管理员更新用户状态
   * @request PATCH:/admin/users/{id}/status
   * @secure
   */
  export namespace UsersStatusPartialUpdate {
    export type RequestParams = {
      /** 用户ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = UpdateUserStatusRequest;
    export type RequestHeaders = {};
    export type ResponseBody = UpdateUserStatusResponseDoc;
  }
}

export namespace Announcements {
  /**
   * @description 登录用户获取当前可展示的站点公告列表
   * @tags announcements
   * @name AnnouncementsList
   * @summary 获取当前公告
   * @request GET:/announcements
   * @secure
   */
  export namespace AnnouncementsList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 是否包含今日不再显示的公告 */
      include_dismissed?: boolean;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = AnnouncementListResponseDoc;
  }

  /**
   * @description 登录用户关闭当前公告版本；公告更新后会重新展示
   * @tags announcements
   * @name CloseCreate
   * @summary 关闭公告
   * @request POST:/announcements/{id}/close
   * @secure
   */
  export namespace CloseCreate {
    export type RequestParams = {
      /** 公告ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = AnnouncementStateRequest;
    export type RequestHeaders = {};
    export type ResponseBody = AnnouncementCloseResponseDoc;
  }

  /**
   * @description 登录用户对当前公告版本记录今日不再显示
   * @tags announcements
   * @name DismissTodayCreate
   * @summary 今日不再显示公告
   * @request POST:/announcements/{id}/dismiss-today
   * @secure
   */
  export namespace DismissTodayCreate {
    export type RequestParams = {
      /** 公告ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = AnnouncementStateRequest;
    export type RequestHeaders = {};
    export type ResponseBody = AnnouncementDismissResponseDoc;
  }
}

export namespace Auth {
  /**
   * @description 登录后返回JWT访问令牌
   * @tags auth
   * @name LoginCreate
   * @summary 用户登录
   * @request POST:/auth/login
   */
  export namespace LoginCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = LoginRequest;
    export type RequestHeaders = {};
    export type ResponseBody = LoginResponseDoc;
  }

  /**
   * @description 获取用户名、邮箱、OAuth/OIDC 登录入口，以及邮箱注册 Turnstile 公共配置
   * @tags auth
   * @name LoginOptionsList
   * @summary 获取登录入口配置
   * @request GET:/auth/login-options
   */
  export namespace LoginOptionsList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = LoginOptionsResponseDoc;
  }

  /**
   * @description 吊销当前 access token 对应会话
   * @tags auth
   * @name LogoutCreate
   * @summary 登出当前会话
   * @request POST:/auth/logout
   * @secure
   */
  export namespace LogoutCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = LogoutResponseDoc;
  }

  /**
   * @description 吊销当前用户所有活跃会话
   * @tags auth
   * @name LogoutAllCreate
   * @summary 登出全部会话
   * @request POST:/auth/logout-all
   * @secure
   */
  export namespace LogoutAllCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = LogoutResponseDoc;
  }

  /**
   * @description 使用邮箱、验证码和新密码完成密码重置；失败时返回通用错误，避免暴露账号状态
   * @tags auth
   * @name PasswordResetCompleteCreate
   * @summary 完成密码重置
   * @request POST:/auth/password/reset/complete
   */
  export namespace PasswordResetCompleteCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = PasswordResetCompleteRequest;
    export type RequestHeaders = {};
    export type ResponseBody = PasswordResetCompleteResponseDoc;
  }

  /**
   * @description SMTP 配置可用时，向已验证邮箱发送密码重置验证码；失败时返回通用错误，避免暴露账号状态
   * @tags auth
   * @name PasswordResetStartCreate
   * @summary 发送密码重置验证码
   * @request POST:/auth/password/reset/start
   */
  export namespace PasswordResetStartCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = PasswordResetStartRequest;
    export type RequestHeaders = {};
    export type ResponseBody = PasswordResetStartResponseDoc;
  }

  /**
   * @description 使用 HttpOnly refresh cookie 轮换并签发新的 access token
   * @tags auth
   * @name RefreshCreate
   * @summary 刷新访问令牌
   * @request POST:/auth/refresh
   */
  export namespace RefreshCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = RefreshTokenResponseDoc;
  }

  /**
   * @description 使用邮箱、密码和验证码完成注册；未开启邮箱验证码但启用 Turnstile 时需要提交 turnstileToken
   * @tags auth
   * @name RegisterEmailCompleteCreate
   * @summary 完成邮箱注册
   * @request POST:/auth/register/email/complete
   */
  export namespace RegisterEmailCompleteCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = EmailRegistrationCompleteRequest;
    export type RequestHeaders = {};
    export type ResponseBody = LoginResponseDoc;
  }

  /**
   * @description 邮箱验证码注册开启时发送验证码；启用 Turnstile 后需要提交 turnstileToken
   * @tags auth
   * @name RegisterEmailStartCreate
   * @summary 发送邮箱注册验证码
   * @request POST:/auth/register/email/start
   */
  export namespace RegisterEmailStartCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = EmailRegistrationStartRequest;
    export type RequestHeaders = {};
    export type ResponseBody = EmailRegistrationStartResponseDoc;
  }

  /**
   * @description 查询当前登录用户仍然有效的活跃会话列表
   * @tags auth
   * @name SessionsList
   * @summary 当前活跃会话
   * @request GET:/auth/sessions
   * @secure
   */
  export namespace SessionsList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ActiveSessionListResponseDoc;
  }

  /**
   * @description 用户授权后，用浏览器定位能力补充当前登录会话的精确位置
   * @tags auth
   * @name SessionsCurrentLocationUpdate
   * @summary 更新当前会话精确位置
   * @request PUT:/auth/sessions/current/location
   * @secure
   */
  export namespace SessionsCurrentLocationUpdate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = UpdateCurrentSessionLocationRequest;
    export type RequestHeaders = {};
    export type ResponseBody = UpdateCurrentSessionLocationResponseDoc;
  }

  /**
   * @description 吊销当前用户指定 session_id 对应的活跃会话
   * @tags auth
   * @name SessionsLogoutCreate
   * @summary 登出指定会话
   * @request POST:/auth/sessions/{session_id}/logout
   * @secure
   */
  export namespace SessionsLogoutCreate {
    export type RequestParams = {
      /** 会话ID */
      sessionId: string;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = LogoutResponseDoc;
  }
}

export namespace Billing {
  /**
   * @description 查询当前用户按量余额
   * @tags billing
   * @name AccountList
   * @summary 获取按量计费账户
   * @request GET:/billing/account
   * @secure
   */
  export namespace AccountList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = BillingAccountResponseDoc;
  }

  /**
   * @description 查询当前计费方式、周期额度或按量余额
   * @tags billing
   * @name OverviewList
   * @summary 获取当前用户计费概览
   * @request GET:/billing/overview
   * @secure
   */
  export namespace OverviewList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = BillingOverviewResponseDoc;
  }

  /**
   * @description 为当前用户创建套餐支付单，并返回支付跳转地址
   * @tags billing
   * @name PaymentsCheckoutCreate
   * @summary 创建支付收银台
   * @request POST:/billing/payments/checkout
   * @secure
   */
  export namespace PaymentsCheckoutCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = CreateCheckoutRequest;
    export type RequestHeaders = {};
    export type ResponseBody = CheckoutResponseDoc;
  }

  /**
   * No description
   * @tags billing
   * @name PaymentsEpayNotifyCreate
   * @summary 易支付异步通知
   * @request POST:/billing/payments/epay/notify
   */
  export namespace PaymentsEpayNotifyCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = string;
  }

  /**
   * No description
   * @tags billing
   * @name PaymentsStripeWebhookCreate
   * @summary Stripe 支付回调
   * @request POST:/billing/payments/stripe/webhook
   */
  export namespace PaymentsStripeWebhookCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * @description 查询所有启用的订阅套餐及价格
   * @tags billing
   * @name PlansList
   * @summary 获取订阅套餐
   * @request GET:/billing/plans
   * @secure
   */
  export namespace PlansList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = PlanListResponseDoc;
  }

  /**
   * @description 当前用户兑换余额或订阅权益
   * @tags billing
   * @name RedemptionsCreate
   * @summary 兑换计费权益码
   * @request POST:/billing/redemptions
   * @secure
   */
  export namespace RedemptionsCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = RedeemCodeRequest;
    export type RequestHeaders = {};
    export type ResponseBody = RedemptionApplyResponseDoc;
  }

  /**
   * @description 为当前用户创建或替换订阅
   * @tags billing
   * @name SubscriptionsCreate
   * @summary 创建订阅
   * @request POST:/billing/subscriptions
   * @secure
   */
  export namespace SubscriptionsCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = SubscribeRequest;
    export type RequestHeaders = {};
    export type ResponseBody = SubscribeResponseDoc;
  }

  /**
   * @description 查询当前用户的每日用量与费用
   * @tags billing
   * @name UsageList
   * @summary 查询用量账单
   * @request GET:/billing/usage
   * @secure
   */
  export namespace UsageList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 搜索模型 */
      query?: string;
      /** 排序：newest/oldest/tokens_desc/cost_desc/latency_desc */
      sort?: string;
      /** 状态筛选：free/billable */
      status?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = UsageLedgerListResponseDoc;
  }

  /**
   * @description 查询当前用户按日期聚合的用量与费用
   * @tags billing
   * @name UsageDailyList
   * @summary 查询每日用量
   * @request GET:/billing/usage/daily
   * @secure
   */
  export namespace UsageDailyList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 天数 */
      days?: number;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = UsageDailyListResponseDoc;
  }

  /**
   * @description 查询当前用户按月份聚合的用量与费用
   * @tags billing
   * @name UsageMonthlyList
   * @summary 查询月度用量
   * @request GET:/billing/usage/monthly
   * @secure
   */
  export namespace UsageMonthlyList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 月份数量，默认近 12 个月 */
      months?: number;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = UsageMonthlyListResponseDoc;
  }
}

export namespace Branding {
  /**
   * No description
   * @tags settings
   * @name BrandingList
   * @summary 查询公开品牌配置
   * @request GET:/branding
   */
  export namespace BrandingList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = BrandingResponseDoc;
  }

  /**
   * No description
   * @tags settings
   * @name ManifestWebmanifestList
   * @summary 查询品牌 Web App Manifest
   * @request GET:/branding/manifest.webmanifest
   */
  export namespace ManifestWebmanifestList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = BrandingManifestResponse;
  }
}

export namespace ContextArtifacts {
  /**
   * @description 查询当前用户可访问的上下文证据详情，用于 Prompt Trace 来源查看
   * @tags chat
   * @name ContextArtifactsDetail
   * @summary 查询上下文证据详情
   * @request GET:/context-artifacts/{id}
   * @secure
   */
  export namespace ContextArtifactsDetail {
    export type RequestParams = {
      /** 上下文证据 ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ContextArtifactResponseDoc;
  }
}

export namespace ConversationProjects {
  /**
   * @description 查询当前用户的会话项目分组
   * @tags chat
   * @name ConversationProjectsList
   * @summary 会话项目列表
   * @request GET:/conversation-projects
   * @secure
   */
  export namespace ConversationProjectsList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 状态筛选: active|archived|all */
      status?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationProjectListResponseDoc;
  }

  /**
   * @description 创建当前用户的会话项目分组
   * @tags chat
   * @name ConversationProjectsCreate
   * @summary 创建会话项目
   * @request POST:/conversation-projects
   * @secure
   */
  export namespace ConversationProjectsCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = CreateConversationProjectRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationProjectResponseDoc;
  }

  /**
   * @description 更新当前用户项目分组展示顺序
   * @tags chat
   * @name ReorderCreate
   * @summary 调整会话项目顺序
   * @request POST:/conversation-projects/reorder
   * @secure
   */
  export namespace ReorderCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = ReorderConversationProjectsRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationProjectListResponseDoc;
  }

  /**
   * @description 删除当前用户项目分组。默认仅解除其下会话归属；delete_conversations=true 时同时软删除项目内会话；delete_files=true 时同步删除不再被其他会话引用的文件。
   * @tags chat
   * @name ConversationProjectsDelete
   * @summary 删除会话项目
   * @request DELETE:/conversation-projects/{id}
   * @secure
   */
  export namespace ConversationProjectsDelete {
    export type RequestParams = {
      /** 项目 public_id */
      id: string;
    };
    export type RequestQuery = {
      /** 是否同时删除项目内会话 */
      delete_conversations?: boolean;
      /** 是否同步删除不再被其他会话引用的会话文件 */
      delete_files?: boolean;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationDeleteResponseDoc;
  }

  /**
   * @description 更新当前用户的会话项目分组
   * @tags chat
   * @name ConversationProjectsPartialUpdate
   * @summary 更新会话项目
   * @request PATCH:/conversation-projects/{id}
   * @secure
   */
  export namespace ConversationProjectsPartialUpdate {
    export type RequestParams = {
      /** 项目 public_id */
      id: string;
    };
    export type RequestQuery = {};
    export type RequestBody = UpdateConversationProjectRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationProjectResponseDoc;
  }
}

export namespace ConversationRuns {
  /**
   * @description 仅在用户显式点击暂停时取消对应 run；浏览器刷新或断开连接不会调用此接口
   * @tags chat
   * @name CancelCreate
   * @summary 取消流式生成
   * @request POST:/conversation-runs/{run_id}/cancel
   * @secure
   */
  export namespace CancelCreate {
    export type RequestParams = {
      /** 运行 ID */
      runId: string;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = SuccessDoc;
  }

  /**
   * @description 页面刷新后按 run_id 重新订阅仍在运行的生成流，返回 NDJSON 事件
   * @tags chat
   * @name StreamList
   * @summary 恢复流式生成订阅
   * @request GET:/conversation-runs/{run_id}/stream
   * @secure
   */
  export namespace StreamList {
    export type RequestParams = {
      /** 运行 ID */
      runId: string;
    };
    export type RequestQuery = {
      /** 已接收的最后事件序号 */
      after?: number;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = string;
  }
}

export namespace Conversations {
  /**
   * @description 查询当前用户会话列表
   * @tags chat
   * @name ConversationsList
   * @summary 会话分页列表
   * @request GET:/conversations
   * @secure
   */
  export namespace ConversationsList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 项目筛选: all|unassigned|项目 public_id */
      project?: string;
      /** 搜索关键词，匹配会话元数据、项目名称和消息正文 */
      q?: string;
      /** 分享筛选: all|shared|unshared */
      share?: string;
      /** 星标筛选: all|starred|unstarred */
      starred?: string;
      /** 状态筛选: active|archived|all */
      status?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationListResponseDoc;
  }

  /**
   * @description 创建新的聊天会话
   * @tags chat
   * @name ConversationsCreate
   * @summary 创建会话
   * @request POST:/conversations
   * @secure
   */
  export namespace ConversationsCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = CreateConversationRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationCreateResponseDoc;
  }

  /**
   * @description 返回后台配置的新会话系统推荐模型；未配置时返回空候选
   * @tags chat
   * @name DefaultModelCandidateList
   * @summary 查询新会话默认模型候选
   * @request GET:/conversations/default-model-candidate
   * @secure
   */
  export namespace DefaultModelCandidateList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationDefaultModelCandidateResponseDoc;
  }

  /**
   * @description 流式导出当前用户全部会话及消息为 NDJSON 文件
   * @tags chat
   * @name ExportList
   * @summary 导出当前用户全部对话
   * @request GET:/conversations/export
   * @secure
   */
  export namespace ExportList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = string;
  }

  /**
   * @description 批量设置当前用户会话的项目归属
   * @tags chat
   * @name ProjectCreate
   * @summary 批量设置会话项目归属
   * @request POST:/conversations/project
   * @secure
   */
  export namespace ProjectCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = BatchSetConversationProjectRequest;
    export type RequestHeaders = {};
    export type ResponseBody = BatchSetConversationProjectResponseDoc;
  }

  /**
   * @description 批量关闭当前用户会话的公开分享链接
   * @tags chat
   * @name SharesRevokeCreate
   * @summary 批量关闭会话公开分享
   * @request POST:/conversations/shares/revoke
   * @secure
   */
  export namespace SharesRevokeCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = RevokeConversationSharesRequest;
    export type RequestHeaders = {};
    export type ResponseBody = RevokeConversationSharesResponseDoc;
  }

  /**
   * @description 查询当前用户的单个会话元信息
   * @tags chat
   * @name ConversationsDetail
   * @summary 查询会话
   * @request GET:/conversations/{id}
   * @secure
   */
  export namespace ConversationsDetail {
    export type RequestParams = {
      /** 会话 public_id */
      id: string;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationUpdateResponseDoc;
  }

  /**
   * @description 删除指定会话
   * @tags chat
   * @name ConversationsDelete
   * @summary 删除会话
   * @request DELETE:/conversations/{id}
   * @secure
   */
  export namespace ConversationsDelete {
    export type RequestParams = {
      /** 会话 public_id */
      id: string;
    };
    export type RequestQuery = {
      /** 是否同步删除不再被其他会话引用的会话文件 */
      delete_files?: boolean;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationDeleteResponseDoc;
  }

  /**
   * @description 设置指定会话归档状态
   * @tags chat
   * @name ArchivePartialUpdate
   * @summary 设置会话归档
   * @request PATCH:/conversations/{id}/archive
   * @secure
   */
  export namespace ArchivePartialUpdate {
    export type RequestParams = {
      /** 会话 public_id */
      id: string;
    };
    export type RequestQuery = {};
    export type RequestBody = SetConversationArchiveRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationUpdateResponseDoc;
  }

  /**
   * @description 导出当前用户单个会话的元信息、消息、运行日志和可见处理轨迹
   * @tags chat
   * @name ExportList2
   * @summary 导出会话 JSON
   * @request GET:/conversations/{id}/export
   * @originalName exportList
   * @duplicate
   * @secure
   */
  export namespace ExportList2 {
    export type RequestParams = {
      /** 会话 public_id */
      id: string;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationExportResponseDoc;
  }

  /**
   * @description 替换指定会话的标签；传入空数组可清空标签
   * @tags chat
   * @name LabelsPartialUpdate
   * @summary 更新会话标签
   * @request PATCH:/conversations/{id}/labels
   * @secure
   */
  export namespace LabelsPartialUpdate {
    export type RequestParams = {
      /** 会话 public_id */
      id: string;
    };
    export type RequestQuery = {};
    export type RequestBody = UpdateConversationLabelsRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationUpdateResponseDoc;
  }

  /**
   * @description 查询会话内消息列表
   * @tags chat
   * @name MessagesList
   * @summary 查询会话消息
   * @request GET:/conversations/{id}/messages
   * @secure
   */
  export namespace MessagesList {
    export type RequestParams = {
      /** 会话 public_id */
      id: string;
    };
    export type RequestQuery = {
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = MessageListResponseDoc;
  }

  /**
   * @description 在会话中发送消息，支持文件/图片等多模态附件
   * @tags chat
   * @name MessagesCreate
   * @summary 发送消息
   * @request POST:/conversations/{id}/messages
   * @secure
   */
  export namespace MessagesCreate {
    export type RequestParams = {
      /** 会话 public_id */
      id: string;
    };
    export type RequestQuery = {};
    export type RequestBody = SendMessageRequest;
    export type RequestHeaders = {};
    export type ResponseBody = SendMessageResponseDoc;
  }

  /**
   * @description 在会话中发送消息并以 NDJSON 流式返回 assistant 增量文本
   * @tags chat
   * @name MessagesStreamCreate
   * @summary 流式发送消息
   * @request POST:/conversations/{id}/messages/stream
   * @secure
   */
  export namespace MessagesStreamCreate {
    export type RequestParams = {
      /** 会话 public_id */
      id: string;
    };
    export type RequestQuery = {};
    export type RequestBody = SendMessageRequest;
    export type RequestHeaders = {};
    export type ResponseBody = string;
  }

  /**
   * @description 设置当前用户单个会话的项目归属
   * @tags chat
   * @name ProjectPartialUpdate
   * @summary 设置会话项目归属
   * @request PATCH:/conversations/{id}/project
   * @secure
   */
  export namespace ProjectPartialUpdate {
    export type RequestParams = {
      /** 会话 public_id */
      id: string;
    };
    export type RequestQuery = {};
    export type RequestBody = SetConversationProjectRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationUpdateResponseDoc;
  }

  /**
   * @description 查询会话内模型调用运行日志（tokens/时长/错误）
   * @tags chat
   * @name RunsList
   * @summary 查询会话运行日志
   * @request GET:/conversations/{id}/runs
   * @secure
   */
  export namespace RunsList {
    export type RequestParams = {
      /** 会话 public_id */
      id: string;
    };
    export type RequestQuery = {
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationRunListResponseDoc;
  }

  /**
   * @description 查询当前用户指定会话的最近分享状态
   * @tags chat
   * @name ShareList
   * @summary 查询会话分享状态
   * @request GET:/conversations/{id}/share
   * @secure
   */
  export namespace ShareList {
    export type RequestParams = {
      /** 会话 public_id */
      id: string;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationShareResponseDoc;
  }

  /**
   * @description 创建当前会话全部分支的公开快照分享链接
   * @tags chat
   * @name ShareCreate
   * @summary 创建会话公开分享
   * @request POST:/conversations/{id}/share
   * @secure
   */
  export namespace ShareCreate {
    export type RequestParams = {
      /** 会话 public_id */
      id: string;
    };
    export type RequestQuery = {};
    export type RequestBody = CreateConversationShareRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationShareResponseDoc;
  }

  /**
   * @description 关闭当前会话的有效公开分享链接
   * @tags chat
   * @name ShareDelete
   * @summary 关闭会话公开分享
   * @request DELETE:/conversations/{id}/share
   * @secure
   */
  export namespace ShareDelete {
    export type RequestParams = {
      /** 会话 public_id */
      id: string;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationShareResponseDoc;
  }

  /**
   * @description 关闭当前有效分享并创建新的公开快照链接
   * @tags chat
   * @name ShareRegenerateCreate
   * @summary 重新生成会话分享链接
   * @request POST:/conversations/{id}/share/regenerate
   * @secure
   */
  export namespace ShareRegenerateCreate {
    export type RequestParams = {
      /** 会话 public_id */
      id: string;
    };
    export type RequestQuery = {};
    export type RequestBody = CreateConversationShareRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationShareResponseDoc;
  }

  /**
   * @description 设置指定会话是否星标
   * @tags chat
   * @name StarPartialUpdate
   * @summary 设置会话星标
   * @request PATCH:/conversations/{id}/star
   * @secure
   */
  export namespace StarPartialUpdate {
    export type RequestParams = {
      /** 会话 public_id */
      id: string;
    };
    export type RequestQuery = {};
    export type RequestBody = SetConversationStarRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationUpdateResponseDoc;
  }

  /**
   * @description 修改指定会话标题
   * @tags chat
   * @name TitlePartialUpdate
   * @summary 重命名会话
   * @request PATCH:/conversations/{id}/title
   * @secure
   */
  export namespace TitlePartialUpdate {
    export type RequestParams = {
      /** 会话 public_id */
      id: string;
    };
    export type RequestQuery = {};
    export type RequestBody = RenameConversationRequest;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationUpdateResponseDoc;
  }

  /**
   * @description 根据指定会话已有内容重新生成标题
   * @tags chat
   * @name TitleRegenerateCreate
   * @summary 自动重新命名会话
   * @request POST:/conversations/{id}/title/regenerate
   * @secure
   */
  export namespace TitleRegenerateCreate {
    export type RequestParams = {
      /** 会话 public_id */
      id: string;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationUpdateResponseDoc;
  }
}

export namespace Files {
  /**
   * @description 查询当前用户上传的文件
   * @tags chat
   * @name FilesList
   * @summary 文件分页列表
   * @request GET:/files
   * @secure
   */
  export namespace FilesList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 筛选，支持单值或逗号分隔多值: image,document,spreadsheet,presentation,code,pdf,audio,video */
      kind?: string;
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 搜索关键词 */
      q?: string;
      /** 排序: created|name|size|last_used */
      sort?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = FileListResponseDoc;
  }

  /**
   * @description 上传对话附件文件，统一存储并扣减用户配额（默认100MB）
   * @tags chat
   * @name FilesCreate
   * @summary 上传文件
   * @request POST:/files
   * @secure
   */
  export namespace FilesCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = {
      /**
       * 文件
       * @format binary
       */
      file: File;
      /** 文件用途 */
      purpose?: string;
    };
    export type RequestHeaders = {};
    export type ResponseBody = UploadFileResponseDoc;
  }

  /**
   * @description 删除指定文件并回收用户配额
   * @tags chat
   * @name FilesDelete
   * @summary 删除文件
   * @request DELETE:/files/{file_id}
   * @secure
   */
  export namespace FilesDelete {
    export type RequestParams = {
      /** 文件ID */
      fileId: string;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = DeleteFileResponseDoc;
  }

  /**
   * @description 修改文件名或 RAG 检索开关，file_name 和 rag_opt_out 至少填一个
   * @tags chat
   * @name FilesPartialUpdate
   * @summary 更新文件属性
   * @request PATCH:/files/{file_id}
   * @secure
   */
  export namespace FilesPartialUpdate {
    export type RequestParams = {
      /** 文件ID */
      fileId: string;
    };
    export type RequestQuery = {};
    export type RequestBody = UpdateFileRequest;
    export type RequestHeaders = {};
    export type ResponseBody = FileUpdateResponseDoc;
  }

  /**
   * @description 按当前登录用户权限读取文件内容，用于在线预览或下载
   * @tags chat
   * @name ContentList
   * @summary 获取文件内容
   * @request GET:/files/{file_id}/content
   * @secure
   */
  export namespace ContentList {
    export type RequestParams = {
      /** 文件ID */
      fileId: string;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Blob;
  }
}

export namespace Mcp {
  /**
   * @description 获取当前聊天侧可选择的 MCP 工具
   * @tags mcp
   * @name ToolsList
   * @summary 获取可用 MCP 工具
   * @request GET:/mcp/tools
   * @secure
   */
  export namespace ToolsList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ToolListResponseDoc;
  }
}

export namespace Me {
  /**
   * @description 查询当前登录用户资料
   * @tags auth
   * @name GetMe
   * @summary 当前用户信息
   * @request GET:/me
   * @secure
   */
  export namespace GetMe {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = MeResponseDoc;
  }

  /**
   * @description 删除当前登录用户账户及主要用户域数据
   * @tags auth
   * @name DeleteMe
   * @summary 删除当前用户账户
   * @request DELETE:/me
   * @secure
   */
  export namespace DeleteMe {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = DeleteAccountRequest;
    export type RequestHeaders = {};
    export type ResponseBody = DeleteAccountResponseDoc;
  }

  /**
   * @description 更新当前登录用户的头像、昵称、时区、对话偏好
   * @tags auth
   * @name PatchMe
   * @summary 更新当前用户资料
   * @request PATCH:/me
   * @secure
   */
  export namespace PatchMe {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = PatchMeRequest;
    export type RequestHeaders = {};
    export type ResponseBody = PatchMeResponseDoc;
  }

  /**
   * @description 发送删除当前账号前所需的邮箱验证码，或返回可用的两步验证方式
   * @tags auth
   * @name DeleteStartCreate
   * @summary 开始删除账号验证
   * @request POST:/me/delete/start
   * @secure
   */
  export namespace DeleteStartCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = SecurityVerificationStartRequest;
    export type RequestHeaders = {};
    export type ResponseBody = EmailVerificationStartResponseDoc;
  }

  /**
   * @description 标记当前用户已完成首次引导
   * @tags auth
   * @name OnboardingCompleteCreate
   * @summary 完成首次引导
   * @request POST:/me/onboarding/complete
   * @secure
   */
  export namespace OnboardingCompleteCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = PatchMeResponseDoc;
  }

  /**
   * @description 当前用户仅可自主修改一次登录用户名
   * @tags auth
   * @name UsernamePartialUpdate
   * @summary 修改当前用户用户名
   * @request PATCH:/me/username
   * @secure
   */
  export namespace UsernamePartialUpdate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = PatchUsernameRequest;
    export type RequestHeaders = {};
    export type ResponseBody = PatchMeResponseDoc;
  }
}

export namespace Memories {
  /**
   * @description 查询当前用户的长期个性化记忆
   * @tags memory
   * @name ProfileList
   * @summary 查询用户个性化记忆
   * @request GET:/memories/profile
   * @secure
   */
  export namespace ProfileList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = UserMemoryListResponseDoc;
  }

  /**
   * @description 新增或更新当前用户的长期个性化记忆
   * @tags memory
   * @name ProfileUpdate
   * @summary 更新用户个性化记忆
   * @request PUT:/memories/profile
   * @secure
   */
  export namespace ProfileUpdate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = UpsertUserMemoryRequest;
    export type RequestHeaders = {};
    export type ResponseBody = UpsertUserMemoryResponseDoc;
  }

  /**
   * @description 删除当前用户的指定 key 长期记忆
   * @tags memory
   * @name ProfileDelete
   * @summary 删除用户个性化记忆
   * @request DELETE:/memories/profile/{memory_key}
   * @secure
   */
  export namespace ProfileDelete {
    export type RequestParams = {
      /** 记忆 Key */
      memoryKey: string;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = UpsertUserMemoryResponseDoc;
  }
}

export namespace Messages {
  /**
   * @description 更新当前用户会话中的 assistant 消息内容，并标记为已编辑
   * @tags chat
   * @name MessagesPartialUpdate
   * @summary 更新消息内容
   * @request PATCH:/messages/{id}
   * @secure
   */
  export namespace MessagesPartialUpdate {
    export type RequestParams = {
      /** 消息 public_id */
      id: string;
    };
    export type RequestQuery = {};
    export type RequestBody = UpdateMessageRequest;
    export type RequestHeaders = {};
    export type ResponseBody = MessageResponseDoc;
  }

  /**
   * @description 对 assistant 消息设置点赞/点踩，传空 feedback 表示取消反馈
   * @tags chat
   * @name FeedbackUpdate
   * @summary 设置消息反馈
   * @request PUT:/messages/{id}/feedback
   * @secure
   */
  export namespace FeedbackUpdate {
    export type RequestParams = {
      /** 消息 public_id */
      id: string;
    };
    export type RequestQuery = {};
    export type RequestBody = SetMessageFeedbackRequest;
    export type RequestHeaders = {};
    export type ResponseBody = MessageFeedbackResponseDoc;
  }
}

export namespace Models {
  /**
   * @description 用户侧查询启用模型目录，用于聊天模型选择器
   * @tags llm
   * @name ModelsList
   * @summary 查询可用模型目录
   * @request GET:/models
   * @secure
   */
  export namespace ModelsList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = PublicModelListResponseDoc;
  }
}

export namespace PromptPresets {
  /**
   * @description 返回管理员内置和当前用户自定义的已启用提示词，用于 slash 选择器
   * @tags prompt-presets
   * @name PromptPresetsList
   * @summary 查询当前用户可用预制提示词
   * @request GET:/prompt-presets
   * @secure
   */
  export namespace PromptPresetsList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 搜索关键词 */
      q?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = PromptPresetPageResponseDoc;
  }

  /**
   * @description 分页查询当前用户自定义提示词
   * @tags prompt-presets
   * @name MineList
   * @summary 查询我的自定义提示词
   * @request GET:/prompt-presets/mine
   * @secure
   */
  export namespace MineList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 是否启用 */
      enabled?: boolean;
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 搜索关键词 */
      q?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = PromptPresetPageResponseDoc;
  }

  /**
   * No description
   * @tags prompt-presets
   * @name MineCreate
   * @summary 创建我的自定义提示词
   * @request POST:/prompt-presets/mine
   * @secure
   */
  export namespace MineCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = WritePromptPresetRequest;
    export type RequestHeaders = {};
    export type ResponseBody = PromptPresetResponseDoc;
  }

  /**
   * No description
   * @tags prompt-presets
   * @name MineDelete
   * @summary 删除我的自定义提示词
   * @request DELETE:/prompt-presets/mine/{id}
   * @secure
   */
  export namespace MineDelete {
    export type RequestParams = {
      /** 提示词ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = PromptPresetDeleteResponseDoc;
  }

  /**
   * No description
   * @tags prompt-presets
   * @name MinePartialUpdate
   * @summary 更新我的自定义提示词
   * @request PATCH:/prompt-presets/mine/{id}
   * @secure
   */
  export namespace MinePartialUpdate {
    export type RequestParams = {
      /** 提示词ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = PatchPromptPresetRequest;
    export type RequestHeaders = {};
    export type ResponseBody = PromptPresetResponseDoc;
  }
}

export namespace Settings {
  /**
   * No description
   * @tags settings
   * @name ChatContextPolicyList
   * @summary 查询聊天上下文策略
   * @request GET:/settings/chat-context-policy
   * @secure
   */
  export namespace ChatContextPolicyList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * No description
   * @tags settings
   * @name LoginPageList
   * @summary 查询公开登录页配置
   * @request GET:/settings/login-page
   */
  export namespace LoginPageList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * No description
   * @tags settings
   * @name McpPolicyList
   * @summary 查询 MCP 工具运行策略
   * @request GET:/settings/mcp-policy
   * @secure
   */
  export namespace McpPolicyList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }

  /**
   * No description
   * @tags settings
   * @name ModelOptionPolicyList
   * @summary 查询模型 options 透传策略
   * @request GET:/settings/model-option-policy
   * @secure
   */
  export namespace ModelOptionPolicyList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Envelope;
  }
}

export namespace SharedConversations {
  /**
   * @description 公开读取会话分享快照
   * @tags chat
   * @name SharedConversationsDetail
   * @summary 查询公开分享会话
   * @request GET:/shared-conversations/{share_id}
   */
  export namespace SharedConversationsDetail {
    export type RequestParams = {
      /** 分享 ID */
      shareId: string;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = PublicSharedConversationResponseDoc;
  }

  /**
   * @description 将公开分享快照克隆到当前登录用户账户，包含全部分支消息和分享内附件
   * @tags chat
   * @name CloneCreate
   * @summary 克隆公开分享会话
   * @request POST:/shared-conversations/{share_id}/clone
   * @secure
   */
  export namespace CloneCreate {
    export type RequestParams = {
      /** 分享 ID */
      shareId: string;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = ConversationUpdateResponseDoc;
  }

  /**
   * @description 只允许读取公开分享快照中实际引用的附件内容
   * @tags chat
   * @name FilesContentList
   * @summary 获取公开分享附件内容
   * @request GET:/shared-conversations/{share_id}/files/{file_id}/content
   */
  export namespace FilesContentList {
    export type RequestParams = {
      /** 文件 ID */
      fileId: string;
      /** 分享 ID */
      shareId: string;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = Blob;
  }
}

export namespace Skills {
  /**
   * @description 返回管理员内置和当前用户自定义的已启用技能摘要，用于会话按需选择 Skill 上下文
   * @tags skills
   * @name SkillsList
   * @summary 查询当前用户可用技能
   * @request GET:/skills
   * @secure
   */
  export namespace SkillsList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 按技能 ID 筛选，可重复传递 */
      id?: number[];
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 搜索关键词 */
      q?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = SkillSummaryPageResponseDoc;
  }

  /**
   * No description
   * @tags skills
   * @name MineList
   * @summary 查询我的自定义技能
   * @request GET:/skills/mine
   * @secure
   */
  export namespace MineList {
    export type RequestParams = {};
    export type RequestQuery = {
      /** 是否启用 */
      enabled?: boolean;
      /** 页码 */
      page?: number;
      /** 每页数量 */
      page_size?: number;
      /** 搜索关键词 */
      q?: string;
    };
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = SkillPageResponseDoc;
  }

  /**
   * No description
   * @tags skills
   * @name MineCreate
   * @summary 创建我的自定义技能
   * @request POST:/skills/mine
   * @secure
   */
  export namespace MineCreate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = WriteSkillRequest;
    export type RequestHeaders = {};
    export type ResponseBody = SkillResponseDoc;
  }

  /**
   * No description
   * @tags skills
   * @name MineDelete
   * @summary 删除我的自定义技能
   * @request DELETE:/skills/mine/{id}
   * @secure
   */
  export namespace MineDelete {
    export type RequestParams = {
      /** 技能ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = SkillDeleteResponseDoc;
  }

  /**
   * No description
   * @tags skills
   * @name MinePartialUpdate
   * @summary 更新我的自定义技能
   * @request PATCH:/skills/mine/{id}
   * @secure
   */
  export namespace MinePartialUpdate {
    export type RequestParams = {
      /** 技能ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = PatchSkillRequest;
    export type RequestHeaders = {};
    export type ResponseBody = SkillResponseDoc;
  }

  /**
   * @description 按需返回单个可用 Skill 的完整 SKILL.md 内容，用于用户查看详情
   * @tags skills
   * @name SkillsDetail
   * @summary 查询当前用户可用技能详情
   * @request GET:/skills/{id}
   * @secure
   */
  export namespace SkillsDetail {
    export type RequestParams = {
      /** 技能ID */
      id: number;
    };
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = SkillResponseDoc;
  }
}

export namespace User {
  /**
   * @description 返回当前用户全部个人偏好配置，缺失项以默认值填充
   * @tags user/settings
   * @name SettingsList
   * @summary 获取当前用户的配置
   * @request GET:/user/settings
   * @secure
   */
  export namespace SettingsList {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = never;
    export type RequestHeaders = {};
    export type ResponseBody = UserSettingsResponseDoc;
  }

  /**
   * @description 批量更新用户个人偏好配置，返回更新后的全量配置
   * @tags user/settings
   * @name SettingsPartialUpdate
   * @summary 更新当前用户的配置
   * @request PATCH:/user/settings
   * @secure
   */
  export namespace SettingsPartialUpdate {
    export type RequestParams = {};
    export type RequestQuery = {};
    export type RequestBody = UserSettingsPatchSettingsRequest;
    export type RequestHeaders = {};
    export type ResponseBody = UserSettingsResponseDoc;
  }
}
