import { authedRequest } from "@/shared/api/authed-client";
import { pathParam } from "@/shared/api/http-client";
import type { IdentityProviderDTO } from "@/shared/api/auth.types";

export type IdentityProviderPayload = {
  type: "oidc" | "oauth2";
  name: string;
  slug?: string;
  logoURL?: string;
  loginEnabled: boolean;
  registrationEnabled: boolean;
  clientID: string;
  clientSecret?: string;
  issuerURL?: string;
  discoveryURL?: string;
  authURL?: string;
  tokenURL?: string;
  userinfoURL?: string;
  jwksURL?: string;
  scopes?: string;
  defaultRole?: "user" | "superadmin";
  subjectField?: string;
  emailField?: string;
  nameField?: string;
  avatarField?: string;
};

export async function listAdminIdentityProviders(accessToken: string): Promise<{ total: number; results: IdentityProviderDTO[] }> {
  return authedRequest<{ total: number; results: IdentityProviderDTO[] }>(
    "/api/v1/admin/auth/providers",
    { accessToken },
    true,
  );
}

export async function createAdminIdentityProvider(accessToken: string, payload: IdentityProviderPayload): Promise<IdentityProviderDTO> {
  return authedRequest<IdentityProviderDTO>(
    "/api/v1/admin/auth/providers",
    { method: "POST", accessToken, body: payload },
    true,
  );
}

export async function updateAdminIdentityProvider(accessToken: string, providerID: string, payload: IdentityProviderPayload): Promise<IdentityProviderDTO> {
  return authedRequest<IdentityProviderDTO>(
    `/api/v1/admin/auth/providers/${pathParam(providerID)}`,
    { method: "PATCH", accessToken, body: payload },
    true,
  );
}

export async function reorderAdminIdentityProviders(accessToken: string, providerIDs: string[]): Promise<{ updated: boolean }> {
  return authedRequest<{ updated: boolean }>(
    "/api/v1/admin/auth/provider-order",
    { method: "PATCH", accessToken, body: { providerIDs } },
    true,
  );
}

export async function deleteAdminIdentityProvider(accessToken: string, providerID: string, options: { force?: boolean } = {}): Promise<{ deleted: boolean }> {
  const query = options.force ? "?force=true" : "";
  return authedRequest<{ deleted: boolean }>(
    `/api/v1/admin/auth/providers/${pathParam(providerID)}${query}`,
    { method: "DELETE", accessToken },
    true,
  );
}
