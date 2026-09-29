import type {
  ActiveSessionListResponse,
  ActiveSessionResponse,
  AuthUserIdentityProviderSummaryResponse,
  AuthUserResponse,
  DeleteAccountRequest,
  EmailRegistrationStartResponse,
  EmailVerificationStartResponse,
	IdentityProviderResponse,
  LoginOptionsResponse,
  LoginResponse,
  LogoutResponse,
  PasswordResetCompleteResponse,
  PasswordResetStartResponse,
  PatchMeRequest,
  UpdateCurrentSessionLocationRequest,
} from "@deeix/api-contract";

export type UserIdentityProviderSummaryDTO = AuthUserIdentityProviderSummaryResponse;

export type UserDTO = AuthUserResponse;

export type LoginData = Omit<LoginResponse, "user" | "verificationMethods"> & {
  user: UserDTO;
  twoFactorChallengeToken?: string;
  verificationMethods?: SecurityVerificationMethod[];
};

export type TwoFactorStatusData = {
  available: boolean;
  totpEnabled: boolean;
  required: boolean;
  recoveryCount: number;
  enabledAt: string | null;
};

export type TwoFactorSetupStartData = {
  secret: string;
  otpauthURL: string;
  expiresAt: string;
};

export type TwoFactorRecoveryCodesData = {
  recoveryCodes: string[];
  status: TwoFactorStatusData;
};

export type TwoFactorDisableData = {
  disabled: boolean;
};

export type SecurityVerificationMethod = "none" | "two_factor" | "email";

export type EmailRegistrationStartData = EmailRegistrationStartResponse & {
  debugCode?: string;
};

export type PasswordResetStartData = PasswordResetStartResponse;

export type PasswordResetCompleteData = PasswordResetCompleteResponse;

export type PasswordChangeVerificationStartData = Omit<EmailVerificationStartResponse, "availableMethods" | "verificationMethod"> & {
  verificationMethod: SecurityVerificationMethod;
  availableMethods: SecurityVerificationMethod[];
  debugCode?: string;
};

export type LoginPageSettings = {
  defaultNextPath: string;
};

export type IdentityProviderDTO = IdentityProviderResponse;

export type UserIdentityDTO = {
  id: number;
  providerID: number;
  providerType: "oidc" | "oauth2" | string;
  providerName: string;
  providerSlug: string;
  providerLogoURL?: string;
  providerDisplayName: string;
  email: string;
  emailVerified: boolean;
  linkedAt: string;
  lastLoginAt: string | null;
};

export type UserIdentityListData = {
  results: UserIdentityDTO[];
};

export type UserIdentityData = {
  identity: UserIdentityDTO;
};

export type LoginOptionsData = Omit<LoginOptionsResponse, "providers"> & {
  providers: IdentityProviderDTO[];
};

export type MeData = {
  user: UserDTO;
};

export type PatchMePayload = PatchMeRequest;

export type PatchUsernamePayload = {
  username: string;
};

export type ChangePasswordPayload = {
  currentPassword?: string;
  newPassword: string;
  verificationMethod?: SecurityVerificationMethod;
  code?: string;
};

export type ChangePasswordData = {
  changed: boolean;
};

export type CompleteOnboardingPayload = {
  newPassword?: string;
};

export type EmailVerificationStartData = Omit<EmailVerificationStartResponse, "availableMethods" | "verificationMethod"> & {
  verificationMethod: SecurityVerificationMethod;
  availableMethods: SecurityVerificationMethod[];
  debugCode?: string;
};

export type EmailBootstrapCompletePayload = {
  email: string;
  code: string;
};

export type EmailChangeCompletePayload = {
  email: string;
  currentVerificationMethod?: SecurityVerificationMethod;
  currentCode: string;
  newCode: string;
};

export type DeleteAccountPayload = Omit<DeleteAccountRequest, "verificationMethod"> & {
  verificationMethod: SecurityVerificationMethod;
};

export type LogoutData = LogoutResponse;

export type ActiveSessionDTO = ActiveSessionResponse;

export type ActiveSessionListData = Omit<ActiveSessionListResponse, "results"> & {
  results: ActiveSessionDTO[];
};

export type UpdateCurrentSessionLocationPayload = UpdateCurrentSessionLocationRequest;
