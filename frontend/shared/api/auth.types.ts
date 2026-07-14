export type UserIdentityProviderSummaryDTO = {
  id: number;
  type: string;
  name: string;
  slug: string;
  logoURL: string;
};

export type UserDTO = {
  id: number;
  publicID: string;
  username: string;
  displayName: string;
  avatarURL: string;
  email: string;
  phone: string;
  role: string;
  status: string;
  timezone: string;
  locale: string;
  profilePreferences: string;
  appearancePreferences: string;
  onboardingCompletedAt: string | null;
  emailVerifiedAt: string | null;
  emailSource: string;
  emailBootstrapUsedAt: string | null;
  phoneVerifiedAt: string | null;
  usernameChangedAt: string | null;
  passwordEnabled: boolean;
  passwordSetAt: string | null;
  passwordOrigin: string;
  mustResetPassword: boolean;
  initialUsernameRequired: boolean;
  initialSecurityRequired: boolean;
  twoFactorAvailable: boolean;
  twoFactorEnabled: boolean;
  twoFactorRequired: boolean;
  twoFactorRecoveryCount: number;
  lastLoginAt: string | null;
  lastActiveAt: string | null;
  createdAt: string;
  updatedAt: string;
  subscriptionTier: string;
  subscriptionPlanID: number | null;
  subscriptionPlanName: string;
  subscriptionStatus: string;
  subscriptionExpiresAt: string | null;
  billingAccountCurrency: string;
  billingBalanceNanousd: number;
  billingBalanceUSD: number;
  billingAccountStatus: string;
  identityProviders: UserIdentityProviderSummaryDTO[];
};

export type LoginData = {
  accessToken: string;
  sessionID: string;
  expiresAt: string;
  refreshExpiresAt: string;
  user: UserDTO;
  twoFactorRequired: boolean;
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

export type EmailRegistrationStartData = {
  sent: boolean;
  expiresAt: string;
  debugCode?: string;
};

export type PasswordResetStartData = {
  sent: boolean;
  expiresAt: string;
};

export type PasswordResetCompleteData = {
  changed: boolean;
};

export type PasswordChangeVerificationStartData = {
  sent: boolean;
  expiresAt: string;
  verificationMethod: SecurityVerificationMethod;
  availableMethods: SecurityVerificationMethod[];
  debugCode?: string;
};

export type LoginPageSettings = {
  defaultNextPath: string;
};

export type IdentityProviderDTO = {
  publicID: string;
  type: "oidc" | "oauth2";
  name: string;
  slug: string;
  logoURL?: string;
  loginEnabled: boolean;
  registrationEnabled: boolean;
  clientID?: string;
  issuerURL?: string;
  discoveryURL?: string;
  authURL?: string;
  tokenURL?: string;
  userinfoURL?: string;
  jwksURL?: string;
  scopes: string;
  defaultRole: "user" | "admin" | "superadmin";
  subjectField: string;
  emailField: string;
  emailVerifiedField: string;
  nameField: string;
  avatarField: string;
  createdAt: string;
  updatedAt: string;
};

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

export type LoginOptionsData = {
  usernameEnabled: boolean;
  emailEnabled: boolean;
  emailRegistrationEnabled: boolean;
  emailVerificationEnabled: boolean;
  passwordResetEnabled: boolean;
  turnstileRegistrationEnabled: boolean;
  turnstileSiteKey: string;
  providers: IdentityProviderDTO[];
};

export type MeData = {
  user: UserDTO;
};

export type PatchMePayload = {
  avatarURL?: string;
  displayName?: string;
  timezone?: string;
  locale?: string;
  profilePreferences?: string;
  appearancePreferences?: string;
};

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

export type EmailVerificationStartData = {
  sent: boolean;
  expiresAt: string;
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

export type DeleteAccountPayload = {
  verificationMethod: SecurityVerificationMethod;
  code: string;
};

export type LogoutData = {
  revoked: boolean;
};

export type ActiveSessionDTO = {
  sessionID: string;
  current: boolean;
  deviceLabel: string;
  deviceName: string;
  browserName: string;
  osName: string;
  deviceType: string;
  clientIP: string;
  locationLabel: string;
  geoSource: string;
  geoAccuracy: string;
  countryCode: string;
  regionName: string;
  cityName: string;
  timezoneName: string;
  ipLatitude: number | null;
  ipLongitude: number | null;
  preciseLatitude: number | null;
  preciseLongitude: number | null;
  preciseAccuracyMeters: number | null;
  preciseLocatedAt: string | null;
  createdAt: string;
  updatedAt: string;
  lastSeenAt: string | null;
  expiresAt: string;
};

export type ActiveSessionListData = {
  total: number;
  results: ActiveSessionDTO[];
};

export type UpdateCurrentSessionLocationPayload = {
  latitude: number;
  longitude: number;
  accuracyMeters?: number;
  timezone?: string;
};
