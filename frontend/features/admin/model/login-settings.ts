import type { IdentityProviderDTO } from "@/shared/api/auth.types";
import type { IdentityProviderPayload } from "@/features/admin/api/auth";
import type { SettingsGrouped } from "@/shared/api/settings.types";
import { resolveLocalizedErrorMessage } from "@/i18n/resolve-error-message";

export type LoginFieldType = "int" | "bool" | "string" | "password" | "textarea" | "select" | "tabs" | "button";

export type LoginSettingsField = {
  namespace: "auth";
  key:
    | "login_page_title"
    | "login_default_next_path"
    | "username_login_enabled"
    | "email_login_enabled"
    | "third_party_login_enabled"
    | "email_registration_enabled"
    | "email_verification_enabled"
    | "smtp_host"
    | "smtp_port"
    | "smtp_username"
    | "smtp_password"
    | "smtp_from"
    | "email_registration_allowed_domains"
    | "email_registration_block_plus_alias"
    | "auto_link_verified_email"
    | "turnstile_registration_enabled"
    | "turnstile_site_key"
    | "turnstile_secret_key"
    | "token_ttl_hours"
    | "refresh_token_ttl_hours"
    | "login_max_failures"
    | "login_lock_minutes"
    | "rate_limit_enabled"
    | "rate_limit_rpm"
    | "public_auth_rate_limit_rpm";
  label: string;
  description: string;
  type: LoginFieldType;
  placeholder?: string;
};

export type LoginSettingsGroup = {
  title: string;
  description: string;
  fields: LoginSettingsField[];
};

export type ProviderTemplate = {
  label: string;
  form: Partial<IdentityProviderPayload> & Pick<IdentityProviderPayload, "type" | "name">;
};

type LoginSettingsTranslator = (key: string) => string;

export function buildLoginSettingsGroups(t: LoginSettingsTranslator): LoginSettingsGroup[] {
  return [
  {
    title: t("groups.loginPage.title"),
    description: t("groups.loginPage.description"),
    fields: [
      { namespace: "auth", key: "login_page_title", label: t("fields.loginPageTitle.label"), description: t("fields.loginPageTitle.description"), type: "string", placeholder: t("fields.loginPageTitle.placeholder") },
      { namespace: "auth", key: "login_default_next_path", label: t("fields.loginDefaultNextPath.label"), description: t("fields.loginDefaultNextPath.description"), type: "string", placeholder: "/chat" },
    ],
  },
  {
    title: t("groups.loginAndRegistration.title"),
    description: t("groups.loginAndRegistration.description"),
    fields: [
      { namespace: "auth", key: "email_login_enabled", label: t("fields.emailLoginEnabled.label"), description: t("fields.emailLoginEnabled.description"), type: "bool" },
      { namespace: "auth", key: "email_registration_enabled", label: t("fields.emailRegistrationEnabled.label"), description: t("fields.emailRegistrationEnabled.description"), type: "bool" },
      { namespace: "auth", key: "username_login_enabled", label: t("fields.usernameLoginEnabled.label"), description: t("fields.usernameLoginEnabled.description"), type: "bool" },
      { namespace: "auth", key: "third_party_login_enabled", label: t("fields.thirdPartyLoginEnabled.label"), description: t("fields.thirdPartyLoginEnabled.description"), type: "bool" },
    ],
  },
  {
    title: t("groups.humanVerification.title"),
    description: t("groups.humanVerification.description"),
    fields: [
      { namespace: "auth", key: "turnstile_registration_enabled", label: t("fields.turnstileRegistrationEnabled.label"), description: t("fields.turnstileRegistrationEnabled.description"), type: "bool" },
      { namespace: "auth", key: "turnstile_site_key", label: t("fields.turnstileSiteKey.label"), description: t("fields.turnstileSiteKey.description"), type: "string", placeholder: "0x4AAAA..." },
      { namespace: "auth", key: "turnstile_secret_key", label: t("fields.turnstileSecretKey.label"), description: t("fields.turnstileSecretKey.description"), type: "password" },
    ],
  },
  {
    title: t("groups.bindingAndVerification.title"),
    description: t("groups.bindingAndVerification.description"),
    fields: [
      { namespace: "auth", key: "email_verification_enabled", label: t("fields.emailVerificationEnabled.label"), description: t("fields.emailVerificationEnabled.description"), type: "bool" },
      { namespace: "auth", key: "smtp_host", label: t("fields.smtpHost.label"), description: t("fields.smtpHost.description"), type: "string", placeholder: "smtp.example.com" },
      { namespace: "auth", key: "smtp_port", label: t("fields.smtpPort.label"), description: t("fields.smtpPort.description"), type: "int", placeholder: "587" },
      { namespace: "auth", key: "smtp_username", label: t("fields.smtpUsername.label"), description: t("fields.smtpUsername.description"), type: "string", placeholder: "noreply@example.com" },
      { namespace: "auth", key: "smtp_password", label: t("fields.smtpPassword.label"), description: t("fields.smtpPassword.description"), type: "password" },
      { namespace: "auth", key: "smtp_from", label: t("fields.smtpFrom.label"), description: t("fields.smtpFrom.description"), type: "string", placeholder: "DEEIX Chat <noreply@example.com>" },
      { namespace: "auth", key: "email_registration_allowed_domains", label: t("fields.emailRegistrationAllowedDomains.label"), description: t("fields.emailRegistrationAllowedDomains.description"), type: "textarea", placeholder: "example.com\ncompany.com" },
      { namespace: "auth", key: "email_registration_block_plus_alias", label: t("fields.emailRegistrationBlockPlusAlias.label"), description: t("fields.emailRegistrationBlockPlusAlias.description"), type: "bool" },
      { namespace: "auth", key: "auto_link_verified_email", label: t("fields.autoLinkVerifiedEmail.label"), description: t("fields.autoLinkVerifiedEmail.description"), type: "bool" },
    ],
  },
  {
    title: t("groups.loginSecurity.title"),
    description: t("groups.loginSecurity.description"),
    fields: [
      { namespace: "auth", key: "token_ttl_hours", label: t("fields.tokenTTLHours.label"), description: t("fields.tokenTTLHours.description"), type: "int", placeholder: "24" },
      { namespace: "auth", key: "refresh_token_ttl_hours", label: t("fields.refreshTokenTTLHours.label"), description: t("fields.refreshTokenTTLHours.description"), type: "int", placeholder: "720" },
      { namespace: "auth", key: "login_max_failures", label: t("fields.loginMaxFailures.label"), description: t("fields.loginMaxFailures.description"), type: "int", placeholder: "5" },
      { namespace: "auth", key: "login_lock_minutes", label: t("fields.loginLockMinutes.label"), description: t("fields.loginLockMinutes.description"), type: "int", placeholder: "15" },
      { namespace: "auth", key: "rate_limit_enabled", label: t("fields.rateLimitEnabled.label"), description: t("fields.rateLimitEnabled.description"), type: "bool" },
      { namespace: "auth", key: "rate_limit_rpm", label: t("fields.rateLimitRPM.label"), description: t("fields.rateLimitRPM.description"), type: "int", placeholder: "60" },
      { namespace: "auth", key: "public_auth_rate_limit_rpm", label: t("fields.publicAuthRateLimitRPM.label"), description: t("fields.publicAuthRateLimitRPM.description"), type: "int", placeholder: "30" },
    ],
  },
  ];
}

export const DEFAULT_PROVIDER_FORM: IdentityProviderPayload = {
  type: "oidc",
  name: "",
  slug: "",
  logoURL: "",
  loginEnabled: true,
  registrationEnabled: true,
  clientID: "",
  clientSecret: "",
  issuerURL: "",
  discoveryURL: "",
  authURL: "",
  tokenURL: "",
  userinfoURL: "",
  jwksURL: "",
  scopes: "openid profile email",
  defaultRole: "user",
  subjectField: "sub",
  emailField: "email",
  emailVerifiedField: "email_verified",
  nameField: "name",
  avatarField: "picture",
};

export const PROVIDER_TEMPLATES: ProviderTemplate[] = [
  {
    label: "Apple",
    form: {
      type: "oidc",
      name: "Apple",
      issuerURL: "https://appleid.apple.com",
      scopes: "openid email name",
    },
  },
  {
    label: "GitHub",
    form: {
      type: "oauth2",
      name: "GitHub",
      authURL: "https://github.com/login/oauth/authorize",
      tokenURL: "https://github.com/login/oauth/access_token",
      userinfoURL: "https://api.github.com/user",
      scopes: "read:user user:email",
      subjectField: "id",
      emailField: "email",
      emailVerifiedField: "email_verified",
      nameField: "name",
      avatarField: "avatar_url",
    },
  },
  {
    label: "Google",
    form: {
      type: "oidc",
      name: "Google",
      discoveryURL: "https://accounts.google.com/.well-known/openid-configuration",
      scopes: "openid profile email",
    },
  },
  {
    label: "Discord",
    form: {
      type: "oauth2",
      name: "Discord",
      authURL: "https://discord.com/oauth2/authorize",
      tokenURL: "https://discord.com/api/oauth2/token",
      userinfoURL: "https://discord.com/api/users/@me",
      scopes: "identify email",
      subjectField: "id",
      emailField: "email",
      emailVerifiedField: "verified",
      nameField: "username",
      avatarField: "avatar",
    },
  },
  {
    label: "Facebook",
    form: {
      type: "oauth2",
      name: "Facebook",
      authURL: "https://www.facebook.com/dialog/oauth",
      tokenURL: "https://graph.facebook.com/oauth/access_token",
      userinfoURL: "https://graph.facebook.com/me?fields=id,name,email,picture",
      scopes: "email public_profile",
      subjectField: "id",
      emailField: "email",
      emailVerifiedField: "email_verified",
      nameField: "name",
      avatarField: "picture",
    },
  },
  {
    label: "Hugging Face",
    form: {
      type: "oidc",
      name: "Hugging Face",
      discoveryURL: "https://huggingface.co/.well-known/openid-configuration",
      scopes: "openid profile email",
    },
  },
  {
    label: "Linux Do",
    form: {
      type: "oauth2",
      name: "Linux Do",
      authURL: "https://connect.linux.do/oauth2/authorize",
      tokenURL: "https://connect.linux.do/oauth2/token",
      userinfoURL: "https://connect.linux.do/api/user",
      scopes: "profile email",
      subjectField: "id",
      emailField: "email",
      emailVerifiedField: "email_verified",
      nameField: "username",
      avatarField: "avatar_url",
    },
  },
  {
    label: "Microsoft",
    form: {
      type: "oidc",
      name: "Microsoft",
      discoveryURL: "https://login.microsoftonline.com/common/v2.0/.well-known/openid-configuration",
      scopes: "openid profile email",
    },
  },
  {
    label: "Twitter (X)",
    form: {
      type: "oauth2",
      name: "Twitter (X)",
      authURL: "https://twitter.com/i/oauth2/authorize",
      tokenURL: "https://api.twitter.com/2/oauth2/token",
      userinfoURL: "https://api.twitter.com/2/users/me?user.fields=profile_image_url",
      scopes: "users.read tweet.read offline.access",
      subjectField: "id",
      emailField: "email",
      emailVerifiedField: "email_verified",
      nameField: "name",
      avatarField: "profile_image_url",
    },
  },
  {
    label: "Vercel",
    form: {
      type: "oidc",
      name: "Vercel",
      discoveryURL: "https://vercel.com/.well-known/openid-configuration",
      scopes: "openid profile email",
    },
  },
];

export function fieldID(field: LoginSettingsField): string {
  return `${field.namespace}.${field.key}`;
}

export function flattenLoginSettings(grouped: SettingsGrouped): Record<string, string> {
  const result: Record<string, string> = {};
  for (const item of grouped.auth ?? []) {
    result[`auth.${item.key}`] = item.value ?? "";
  }
  return applyLoginDefaults(result);
}

export function applyLoginDefaults(settings: Record<string, string>): Record<string, string> {
  const result = {
    ...settings,
    "auth.login_page_title": settings["auth.login_page_title"]?.trim() || "Sign in to DEEIX Chat",
    "auth.login_default_next_path": settings["auth.login_default_next_path"]?.trim() || "/chat",
    "auth.username_login_enabled": settings["auth.username_login_enabled"] || "true",
    "auth.email_login_enabled": settings["auth.email_login_enabled"] || "true",
    "auth.third_party_login_enabled": settings["auth.third_party_login_enabled"] || "true",
    "auth.email_registration_enabled": settings["auth.email_registration_enabled"] || "true",
    "auth.email_verification_enabled": settings["auth.email_verification_enabled"] || "false",
    "auth.smtp_host": settings["auth.smtp_host"] ?? "",
    "auth.smtp_port": settings["auth.smtp_port"]?.trim() || "587",
    "auth.smtp_username": settings["auth.smtp_username"] ?? "",
    "auth.smtp_password": settings["auth.smtp_password"] ?? "",
    "auth.smtp_from": settings["auth.smtp_from"] ?? "",
    "auth.email_registration_allowed_domains": settings["auth.email_registration_allowed_domains"] ?? "",
    "auth.email_registration_block_plus_alias": settings["auth.email_registration_block_plus_alias"] || "false",
    "auth.auto_link_verified_email": settings["auth.auto_link_verified_email"] || "true",
    "auth.turnstile_registration_enabled": settings["auth.turnstile_registration_enabled"] || "false",
    "auth.turnstile_site_key": settings["auth.turnstile_site_key"] ?? "",
    "auth.turnstile_secret_key": settings["auth.turnstile_secret_key"] ?? "",
    "auth.token_ttl_hours": settings["auth.token_ttl_hours"]?.trim() || "24",
    "auth.refresh_token_ttl_hours": settings["auth.refresh_token_ttl_hours"]?.trim() || "720",
    "auth.login_max_failures": settings["auth.login_max_failures"]?.trim() || "5",
    "auth.login_lock_minutes": settings["auth.login_lock_minutes"]?.trim() || "15",
    "auth.rate_limit_enabled": settings["auth.rate_limit_enabled"] || "false",
    "auth.rate_limit_rpm": settings["auth.rate_limit_rpm"]?.trim() || "60",
    "auth.public_auth_rate_limit_rpm": settings["auth.public_auth_rate_limit_rpm"]?.trim() || "30",
  };
  if (result["auth.email_login_enabled"] === "false") {
    result["auth.email_registration_enabled"] = "false";
  }
  if (result["auth.email_registration_enabled"] === "false") {
    result["auth.turnstile_registration_enabled"] = "false";
  }
  return result;
}

export function toEditorField(field: LoginSettingsField) {
  return {
    id: fieldID(field),
    label: field.label,
    description: field.description,
    type: field.type,
    placeholder: field.placeholder,
  } as const;
}

export function isEmailSMTPField(field: LoginSettingsField) {
  return field.key === "smtp_host" || field.key === "smtp_port" || field.key === "smtp_username" || field.key === "smtp_password" || field.key === "smtp_from";
}

export function isRateLimitChildField(field: LoginSettingsField) {
  return field.key === "rate_limit_rpm" || field.key === "public_auth_rate_limit_rpm";
}

export function includesEmailVerificationSettings(group: LoginSettingsGroup) {
  return group.fields.some((field) => field.key === "email_verification_enabled" || isEmailSMTPField(field));
}

export function includesTurnstileSettings(group: LoginSettingsGroup) {
  return group.fields.some((field) => field.key === "turnstile_registration_enabled" || field.key === "turnstile_site_key" || field.key === "turnstile_secret_key");
}

export function validateEmailVerificationSettings(
  settings: Record<string, string>,
  configured: Record<string, boolean> = {},
  labels: {
    smtpHost: string;
    smtpPort: string;
    smtpUsername: string;
    smtpPassword: string;
    missingSMTP: (labels: string[]) => string;
    invalidSMTPPort: string;
  } = {
    smtpHost: "SMTP host",
    smtpPort: "SMTP port",
    smtpUsername: "SMTP username",
    smtpPassword: "SMTP password",
    missingSMTP: (labels) => `Email verification requires ${labels.join(", ")}.`,
    invalidSMTPPort: "SMTP port must be an integer between 1 and 65535.",
  },
): string | undefined {
  if (settings["auth.email_verification_enabled"] !== "true") {
    return undefined;
  }
  const requiredFields = [
    { key: "auth.smtp_host", label: labels.smtpHost },
    { key: "auth.smtp_port", label: labels.smtpPort },
    { key: "auth.smtp_username", label: labels.smtpUsername },
    { key: "auth.smtp_password", label: labels.smtpPassword },
  ];
  const missingLabels = requiredFields
    .filter((field) => !(settings[field.key] ?? "").trim() && !configured[field.key])
    .map((field) => field.label);
  if (missingLabels.length > 0) {
    return labels.missingSMTP(missingLabels);
  }
  const port = Number(settings["auth.smtp_port"]);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    return labels.invalidSMTPPort;
  }
  return undefined;
}

export function includesPasswordLoginSettings(group: LoginSettingsGroup) {
  return group.fields.some((field) => field.key === "username_login_enabled" || field.key === "email_login_enabled");
}

export function validateTurnstileSettings(
  settings: Record<string, string>,
  configured: Record<string, boolean> = {},
  labels: {
    siteKey: string;
    secretKey: string;
    registrationRequired: string;
    missing: (labels: string[]) => string;
  } = {
    siteKey: "Turnstile Site Key",
    secretKey: "Turnstile Secret Key",
    registrationRequired: "Enable email registration before Turnstile verification.",
    missing: (labels) => `Turnstile verification requires ${labels.join(", ")}.`,
  },
): string | undefined {
  if (settings["auth.turnstile_registration_enabled"] !== "true") {
    return undefined;
  }
  if (settings["auth.email_registration_enabled"] !== "true") {
    return labels.registrationRequired;
  }
  const requiredFields = [
    { key: "auth.turnstile_site_key", label: labels.siteKey },
    { key: "auth.turnstile_secret_key", label: labels.secretKey },
  ];
  const missingLabels = requiredFields
    .filter((field) => !(settings[field.key] ?? "").trim() && !configured[field.key])
    .map((field) => field.label);
  if (missingLabels.length > 0) {
    return labels.missing(missingLabels);
  }
  return undefined;
}

export function validatePasswordLoginSettings(
  settings: Record<string, string>,
  message = "Before disabling username and email login, enable third-party login and ensure at least one administrator has a usable identity provider linked.",
): string | undefined {
  if (
    settings["auth.username_login_enabled"] === "false" &&
    settings["auth.email_login_enabled"] === "false" &&
    settings["auth.third_party_login_enabled"] !== "true"
  ) {
    return message;
  }
  return undefined;
}

export function resolveErrorMessage(error: unknown): string {
  return resolveLocalizedErrorMessage(error);
}


export function createProviderForm(overrides: Partial<IdentityProviderPayload>): IdentityProviderPayload {
  const form = {
    ...DEFAULT_PROVIDER_FORM,
    ...overrides,
    clientID: "",
    clientSecret: "",
  };
  return {
    ...form,
    registrationEnabled: form.loginEnabled ? form.registrationEnabled : false,
  };
}

export function providerToForm(provider: IdentityProviderDTO): IdentityProviderPayload {
  return {
    type: provider.type,
    name: provider.name,
    slug: provider.slug,
    logoURL: provider.logoURL ?? "",
    loginEnabled: provider.loginEnabled,
    registrationEnabled: provider.loginEnabled && provider.registrationEnabled,
    clientID: provider.clientID ?? "",
    clientSecret: "",
    issuerURL: provider.issuerURL ?? "",
    discoveryURL: provider.discoveryURL ?? "",
    authURL: provider.authURL ?? "",
    tokenURL: provider.tokenURL ?? "",
    userinfoURL: provider.userinfoURL ?? "",
    jwksURL: provider.jwksURL ?? "",
    scopes: provider.scopes,
    defaultRole: provider.defaultRole,
    subjectField: provider.subjectField,
    emailField: provider.emailField,
    emailVerifiedField: provider.emailVerifiedField,
    nameField: provider.nameField,
    avatarField: provider.avatarField,
  };
}

export function normalizeProviderSlugPreview(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replaceAll(" ", "-")
    .replace(/[^a-z0-9_-]+/g, "-")
    .replace(/^[-_]+|[-_]+$/g, "");
}

export function reorderProviders(items: IdentityProviderDTO[], draggedID: string, targetID: string) {
  const fromIndex = items.findIndex((item) => item.publicID === draggedID);
  const toIndex = items.findIndex((item) => item.publicID === targetID);
  if (fromIndex < 0 || toIndex < 0 || fromIndex === toIndex) return items;
  const next = [...items];
  const [moved] = next.splice(fromIndex, 1);
  next.splice(toIndex, 0, moved);
  return next;
}
