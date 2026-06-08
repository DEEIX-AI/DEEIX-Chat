import { ApiError, apiRequest, pathParam } from "@/shared/api/http-client";
import { authedRequest } from "@/shared/api/authed-client";
import type {
  ActiveSessionListData,
  ActiveSessionDTO,
  ChangePasswordData,
  ChangePasswordPayload,
  CompleteOnboardingPayload,
  DeleteAccountPayload,
  EmailBootstrapCompletePayload,
  EmailChangeCompletePayload,
  EmailVerificationStartData,
  EmailRegistrationStartData,
  LoginData,
  LoginOptionsData,
  LoginPageSettings,
  LogoutData,
  MeData,
  PatchMePayload,
  PatchUsernamePayload,
  PasswordChangeVerificationStartData,
  SecurityVerificationMethod,
  TwoFactorDisableData,
  TwoFactorRecoveryCodesData,
  TwoFactorSetupStartData,
  TwoFactorStatusData,
  UpdateCurrentSessionLocationPayload,
  UserDTO,
  UserIdentityData,
  UserIdentityListData,
} from "@/shared/api/auth.types";

export const AUTH_ERROR_CODES = {
  passwordReuseNotAllowed: "auth.password_reuse_not_allowed",
} as const;

export function isPasswordReuseNotAllowedError(error: unknown): boolean {
  return error instanceof ApiError && error.errorCode === AUTH_ERROR_CODES.passwordReuseNotAllowed;
}

export async function login(username: string, password: string): Promise<LoginData> {
  return apiRequest<LoginData>("/api/v1/auth/login", {
    method: "POST",
    body: { username, password },
  });
}

export async function startTwoFactorEmailVerification(challengeToken: string): Promise<EmailVerificationStartData> {
  return apiRequest<EmailVerificationStartData>("/api/v1/auth/2fa/email/start", {
    method: "POST",
    body: { challengeToken },
  });
}

export async function verifyTwoFactorLogin(challengeToken: string, code: string, verificationMethod?: SecurityVerificationMethod): Promise<LoginData> {
  return apiRequest<LoginData>("/api/v1/auth/2fa/verify", {
    method: "POST",
    body: { challengeToken, verificationMethod, code },
  });
}

export async function getCurrentTwoFactorStatus(accessToken: string): Promise<TwoFactorStatusData> {
  return authedRequest<TwoFactorStatusData>(
    "/api/v1/me/2fa",
    {
      method: "GET",
      accessToken,
    },
    false,
  );
}

export async function startCurrentTwoFactorSetup(accessToken: string): Promise<TwoFactorSetupStartData> {
  return authedRequest<TwoFactorSetupStartData>(
    "/api/v1/me/2fa/setup/start",
    {
      method: "POST",
      accessToken,
    },
    false,
  );
}

export async function confirmCurrentTwoFactorSetup(accessToken: string, code: string): Promise<TwoFactorRecoveryCodesData> {
  return authedRequest<TwoFactorRecoveryCodesData>(
    "/api/v1/me/2fa/setup/confirm",
    {
      method: "POST",
      accessToken,
      body: { code },
    },
    false,
  );
}

export async function cancelCurrentTwoFactorSetup(accessToken: string): Promise<{ canceled: boolean }> {
  return authedRequest<{ canceled: boolean }>(
    "/api/v1/me/2fa/setup",
    {
      method: "DELETE",
      accessToken,
    },
    false,
  );
}

export async function disableCurrentTwoFactor(accessToken: string, code: string): Promise<TwoFactorDisableData> {
  return authedRequest<TwoFactorDisableData>(
    "/api/v1/me/2fa/disable",
    {
      method: "POST",
      accessToken,
      body: { code },
    },
    false,
  );
}

export async function regenerateCurrentTwoFactorRecoveryCodes(accessToken: string, code: string): Promise<TwoFactorRecoveryCodesData> {
  return authedRequest<TwoFactorRecoveryCodesData>(
    "/api/v1/me/2fa/recovery/regenerate",
    {
      method: "POST",
      accessToken,
      body: { code },
    },
    false,
  );
}

export async function startEmailRegistration(email: string, turnstileToken?: string): Promise<EmailRegistrationStartData> {
  return apiRequest<EmailRegistrationStartData>("/api/v1/auth/register/email/start", {
    method: "POST",
    body: { email, turnstileToken },
  });
}

export async function completeEmailRegistration(email: string, password: string, code: string, turnstileToken?: string): Promise<LoginData> {
  return apiRequest<LoginData>("/api/v1/auth/register/email/complete", {
    method: "POST",
    body: { email, password, code, turnstileToken },
  });
}

export async function startPasswordChangeVerification(accessToken: string, verificationMethod?: SecurityVerificationMethod): Promise<PasswordChangeVerificationStartData> {
  return authedRequest<PasswordChangeVerificationStartData>(
    "/api/v1/auth/password/change/start",
    {
      method: "POST",
      accessToken,
      body: verificationMethod ? { verificationMethod } : undefined,
    },
    false,
  );
}

export async function changePassword(accessToken: string, payload: ChangePasswordPayload): Promise<ChangePasswordData> {
  return authedRequest<ChangePasswordData>(
    "/api/v1/auth/password/change/complete",
    {
      method: "POST",
      accessToken,
      body: payload,
    },
    false,
  );
}

export async function startEmailBootstrap(accessToken: string, email: string): Promise<EmailVerificationStartData> {
  return authedRequest<EmailVerificationStartData>(
    "/api/v1/me/email/bootstrap/start",
    {
      method: "POST",
      accessToken,
      body: { email },
    },
    false,
  );
}

export async function completeEmailBootstrap(accessToken: string, payload: EmailBootstrapCompletePayload): Promise<UserDTO> {
  const data = await authedRequest<MeData>(
    "/api/v1/me/email/bootstrap/complete",
    {
      method: "POST",
      accessToken,
      body: payload,
    },
    true,
  );
  return data.user;
}

export async function startCurrentEmailVerification(accessToken: string): Promise<EmailVerificationStartData> {
  return authedRequest<EmailVerificationStartData>(
    "/api/v1/me/email/verify-current/start",
    {
      method: "POST",
      accessToken,
    },
    false,
  );
}

export async function completeCurrentEmailVerification(accessToken: string, code: string): Promise<UserDTO> {
  const data = await authedRequest<MeData>(
    "/api/v1/me/email/verify-current/complete",
    {
      method: "POST",
      accessToken,
      body: { code },
    },
    true,
  );
  return data.user;
}

export async function startCurrentEmailChange(accessToken: string, verificationMethod?: SecurityVerificationMethod): Promise<EmailVerificationStartData> {
  return authedRequest<EmailVerificationStartData>(
    "/api/v1/me/email/change/start-current",
    {
      method: "POST",
      accessToken,
      body: verificationMethod ? { verificationMethod } : undefined,
    },
    false,
  );
}

export async function startNewEmailChange(accessToken: string, email: string): Promise<EmailVerificationStartData> {
  return authedRequest<EmailVerificationStartData>(
    "/api/v1/me/email/change/start-new",
    {
      method: "POST",
      accessToken,
      body: { email },
    },
    false,
  );
}

export async function completeEmailChange(accessToken: string, payload: EmailChangeCompletePayload): Promise<UserDTO> {
  const data = await authedRequest<MeData>(
    "/api/v1/me/email/change/complete",
    {
      method: "POST",
      accessToken,
      body: payload,
    },
    true,
  );
  return data.user;
}

export async function completeOnboarding(accessToken: string, payload?: CompleteOnboardingPayload): Promise<UserDTO> {
  const data = await authedRequest<MeData>(
    "/api/v1/me/onboarding/complete",
    {
      method: "POST",
      accessToken,
      body: payload,
    },
    true,
  );
  return data.user;
}

export async function startAccountDeleteVerification(accessToken: string, verificationMethod?: SecurityVerificationMethod): Promise<EmailVerificationStartData> {
  return authedRequest<EmailVerificationStartData>(
    "/api/v1/me/delete/start",
    {
      method: "POST",
      accessToken,
      body: verificationMethod ? { verificationMethod } : undefined,
    },
    false,
  );
}

export async function getLoginPageSettings(): Promise<LoginPageSettings> {
  return apiRequest<LoginPageSettings>("/api/v1/settings/login-page");
}

export async function getLoginOptions(): Promise<LoginOptionsData> {
  return apiRequest<LoginOptionsData>("/api/v1/auth/login-options");
}

export async function completeProviderLogin(
  slug: string,
  code: string,
  state: string,
  redirectURI: string,
  codeVerifier: string,
  intent: "login" | "register" | "bind",
): Promise<LoginData> {
  return apiRequest<LoginData>(`/api/v1/auth/providers/${pathParam(slug)}/callback`, {
    method: "POST",
    body: { code, state, redirectURI: redirectURI, codeVerifier: codeVerifier, intent },
  });
}

export async function refresh(): Promise<LoginData> {
  return apiRequest<LoginData>("/api/v1/auth/refresh", {
    method: "POST",
  });
}

export async function logout(accessToken: string): Promise<LogoutData> {
  return authedRequest<LogoutData>(
    "/api/v1/auth/logout",
    {
      method: "POST",
      accessToken,
    },
    true,
  );
}

export async function logoutAll(accessToken: string): Promise<LogoutData> {
  return authedRequest<LogoutData>(
    "/api/v1/auth/logout-all",
    {
      method: "POST",
      accessToken,
    },
    true,
  );
}

export async function logoutSession(accessToken: string, sessionID: string): Promise<LogoutData> {
  return authedRequest<LogoutData>(
    `/api/v1/auth/sessions/${pathParam(sessionID)}/logout`,
    {
      method: "POST",
      accessToken,
    },
    true,
  );
}

export async function getMe(accessToken: string): Promise<UserDTO> {
  const data = await authedRequest<MeData>(
    "/api/v1/me",
    {
      accessToken,
    },
    true,
  );
  return data.user;
}

export async function deleteMe(accessToken: string, payload: DeleteAccountPayload): Promise<{ deleted: boolean }> {
  return authedRequest<{ deleted: boolean }>(
    "/api/v1/me",
    {
      method: "DELETE",
      accessToken,
      body: payload,
    },
    false,
  );
}

export async function listCurrentUserIdentities(accessToken: string): Promise<UserIdentityListData> {
  return authedRequest<UserIdentityListData>(
    "/api/v1/me/identities",
    {
      accessToken,
    },
    true,
  );
}

export async function completeProviderBind(
  accessToken: string,
  slug: string,
  code: string,
  state: string,
  redirectURI: string,
  codeVerifier: string,
): Promise<UserIdentityData> {
  return authedRequest<UserIdentityData>(
    `/api/v1/me/identities/providers/${pathParam(slug)}/callback`,
    {
      method: "POST",
      accessToken,
      body: { code, state, redirectURI: redirectURI, codeVerifier: codeVerifier },
    },
  );
}

export async function deleteCurrentUserIdentity(accessToken: string, identityID: number): Promise<{ deleted: boolean }> {
  return authedRequest<{ deleted: boolean }>(
    `/api/v1/me/identities/${identityID}`,
    {
      method: "DELETE",
      accessToken,
    },
    false,
  );
}

export async function patchMe(accessToken: string, payload: PatchMePayload): Promise<UserDTO> {
  const data = await authedRequest<MeData>(
    "/api/v1/me",
    {
      method: "PATCH",
      accessToken,
      body: payload,
    },
    true,
  );
  return data.user;
}

export async function patchUsername(accessToken: string, payload: PatchUsernamePayload): Promise<UserDTO> {
  const data = await authedRequest<MeData>(
    "/api/v1/me/username",
    {
      method: "PATCH",
      accessToken,
      body: payload,
    },
    true,
  );
  return data.user;
}

export async function getCurrentActiveSessions(accessToken: string): Promise<ActiveSessionListData> {
  return authedRequest<ActiveSessionListData>(
    "/api/v1/auth/sessions",
    {
      accessToken,
    },
    true,
  );
}

export async function updateCurrentSessionLocation(
  accessToken: string,
  payload: UpdateCurrentSessionLocationPayload,
): Promise<ActiveSessionDTO> {
  return authedRequest<ActiveSessionDTO>(
    "/api/v1/auth/sessions/current/location",
    {
      method: "PUT",
      accessToken,
      body: payload,
    },
    true,
  );
}
