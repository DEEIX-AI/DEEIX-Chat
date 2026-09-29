import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import type { UpsertIdentityProviderRequest } from "@deeix/api-contract";
import {
  createAdminIdentityProvider,
  deleteAdminIdentityProvider,
  listAdminIdentityProviders,
  listAdminSettings,
  patchAdminSettings,
  reorderAdminIdentityProviders,
  updateAdminIdentityProvider,
} from "@/features/admin/api";
import { moveSortableItem } from "@/features/admin/components/shared/sortable-list";
import {
  applyLoginDefaults,
  fieldID,
  flattenLoginSettings,
  type IdentityProviderForm,
  includesEmailVerificationSettings,
  includesPasswordLoginSettings,
  includesTurnstileSettings,
  type LoginSettingsField,
  type LoginSettingsGroup,
  providerToForm,
  validateEmailVerificationSettings,
  validatePasswordLoginSettings,
  validateTurnstileSettings,
} from "@/features/admin/model/login-settings";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { getLoginOptions } from "@/shared/api/auth";
import type { IdentityProviderDTO } from "@/shared/api/auth-types";
import { ApiError } from "@/shared/api/http-client";
import type { PatchSettingItem } from "@/shared/api/settings-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { configuredSettingsMap } from "@/features/admin/utils/settings-meta";

/**
 * Login settings plus identity providers. Both share one `saving` flag so the
 * page never runs two writes at once. Provider toggles and reordering are
 * optimistic and roll back on failure; a 409 on delete escalates to the
 * force-delete confirmation instead of failing.
 */
export function useAdminLoginSettings(loginSettingsGroups: LoginSettingsGroup[]) {
  const t = useTranslations("adminLogin");
  const [settingsMap, setSettingsMap] = React.useState<Record<string, string>>(() => applyLoginDefaults({}));
  const [savedMap, setSavedMap] = React.useState<Record<string, string>>(() => applyLoginDefaults({}));
  const [configuredMap, setConfiguredMap] = React.useState<Record<string, boolean>>({});
  const [providers, setProviders] = React.useState<IdentityProviderDTO[]>([]);
  const [deleteProviderTarget, setDeleteProviderTarget] = React.useState<IdentityProviderDTO | null>(null);
  const [forceDeleteProviderTarget, setForceDeleteProviderTarget] = React.useState<IdentityProviderDTO | null>(null);
  const [forceDeleteProviderMessage, setForceDeleteProviderMessage] = React.useState("");
  const [providerCallbackBaseURL, setProviderCallbackBaseURL] = React.useState("");
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);

  const loadData = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      const [grouped, providerPage, loginOptions] = await Promise.all([listAdminSettings(token), listAdminIdentityProviders(token), getLoginOptions()]);
      const flattened = flattenLoginSettings(grouped);
      setConfiguredMap(configuredSettingsMap(grouped));
      setSettingsMap(flattened);
      setSavedMap(flattened);
      setProviders(providerPage.results);
      setProviderCallbackBaseURL(loginOptions.providerAuthBridge.callbackBaseURL);
    } catch (error) {
      toast.error(t("toast.loadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setLoading(false);
    }
  }, [t]);

  React.useEffect(() => {
    void loadData();
  }, [loadData]);

  const dirtyFieldIDs = React.useMemo(() => {
    const result = new Set<string>();
    for (const group of loginSettingsGroups) {
      for (const field of group.fields) {
        const id = fieldID(field);
        if ((settingsMap[id] ?? "") !== (savedMap[id] ?? "")) result.add(id);
      }
    }
    return result;
  }, [loginSettingsGroups, savedMap, settingsMap]);

  const updateSettingValue = React.useCallback((field: LoginSettingsField, value: string) => {
    setSettingsMap((prev) => {
      const next = { ...prev, [fieldID(field)]: value };
      if (
        (field.key === "username_login_enabled" || field.key === "email_login_enabled") &&
        validatePasswordLoginSettings(next, t("validation.passwordLoginRequired"))
      ) {
        toast.error(t("toast.cannotDisableLoginMethod"), { description: t("toast.thirdPartyOnlyRequiresAdminBinding") });
        return prev;
      }
      if (field.key === "email_login_enabled" && value !== "true") {
        next["auth.email_registration_enabled"] = "false";
        next["auth.turnstile_registration_enabled"] = "false";
      }
      if (field.key === "email_registration_enabled" && value !== "true") {
        next["auth.turnstile_registration_enabled"] = "false";
      }
      if (field.key === "email_verification_enabled" && value !== "true") {
        next["auth.password_reset_enabled"] = "false";
      }
      return next;
    });
  }, [t]);

  const handleSaveGroup = React.useCallback(
    async (group: LoginSettingsGroup) => {
      const nextSettingsMap = applyLoginDefaults(settingsMap);
      const nextPath = nextSettingsMap["auth.login_default_next_path"] ?? "";
      if (!nextPath.startsWith("/") || nextPath.startsWith("//")) {
        toast.error(t("toast.saveFailed"), { description: t("validation.defaultNextPath") });
        return;
      }
      if (includesPasswordLoginSettings(group)) {
        const validationError = validatePasswordLoginSettings(nextSettingsMap, t("validation.passwordLoginRequired"));
        if (validationError) {
          toast.error(t("toast.saveFailed"), { description: validationError });
          return;
        }
      }
      if (includesEmailVerificationSettings(group)) {
        const validationError = validateEmailVerificationSettings(nextSettingsMap, configuredMap, {
          smtpHost: t("fields.smtpHost.label"),
          smtpPort: t("fields.smtpPort.label"),
          smtpUsername: t("fields.smtpUsername.label"),
          smtpPassword: t("fields.smtpPassword.label"),
          missingSMTP: (labels) => t("validation.missingSMTP", { fields: labels.join(t("punctuation.listSeparator")) }),
          invalidSMTPPort: t("validation.invalidSMTPPort"),
        });
        if (validationError) {
          toast.error(t("toast.saveFailed"), { description: validationError });
          return;
        }
      }
      if (includesTurnstileSettings(group)) {
        const validationError = validateTurnstileSettings(nextSettingsMap, configuredMap, {
          siteKey: t("fields.turnstileSiteKey.label"),
          secretKey: t("fields.turnstileSecretKey.label"),
          registrationRequired: t("validation.turnstileRegistrationRequired"),
          missing: (labels) => t("validation.missingTurnstile", { fields: labels.join(t("punctuation.listSeparator")) }),
        });
        if (validationError) {
          toast.error(t("toast.saveFailed"), { description: validationError });
          return;
        }
      }
      const items: PatchSettingItem[] = group.fields
        .map((field) => ({ namespace: field.namespace, key: field.key, value: nextSettingsMap[fieldID(field)] ?? "" }))
        .filter((item) => item.value !== (savedMap[`${item.namespace}.${item.key}`] ?? ""));
      if (items.length === 0) return;
      setSaving(true);
      try {
        const token = await resolveAccessToken();
        if (!token) {
          toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
          return;
        }
        const grouped = await patchAdminSettings(token, { items });
        const flattened = flattenLoginSettings(grouped);
        setConfiguredMap(configuredSettingsMap(grouped));
        setSettingsMap(flattened);
        setSavedMap(flattened);
        toast.success(t("toast.settingsUpdated"));
      } catch (error) {
        toast.error(t("toast.saveFailed"), { description: resolveAdminErrorMessage(error) });
      } finally {
        setSaving(false);
      }
    },
    [configuredMap, savedMap, settingsMap, t],
  );

  // `onSaved` runs right after the success toast, before the list refetch, so
  // the dialog closes at the same moment it did before the hook extraction.
  const saveProvider = React.useCallback(async (
    { editingProvider, providerForm }: { editingProvider: IdentityProviderDTO | null; providerForm: IdentityProviderForm },
    onSaved: () => void,
  ) => {
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      const payload: UpsertIdentityProviderRequest = {
        ...providerForm,
        registrationEnabled: providerForm.loginEnabled && providerForm.registrationEnabled,
      };
      if (editingProvider) {
        await updateAdminIdentityProvider(token, editingProvider.publicID, payload);
      } else {
        await createAdminIdentityProvider(token, payload);
      }
      toast.success(t("toast.providerSaved"));
      onSaved();
      const page = await listAdminIdentityProviders(token);
      setProviders(page.results);
    } catch (error) {
      toast.error(t("toast.providerSaveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }, [t]);

  const deleteProvider = React.useCallback(async (provider: IdentityProviderDTO, force = false) => {
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      await deleteAdminIdentityProvider(token, provider.publicID, { force });
      setProviders((prev) => prev.filter((item) => item.publicID !== provider.publicID));
      setDeleteProviderTarget(null);
      setForceDeleteProviderTarget(null);
      setForceDeleteProviderMessage("");
      toast.success(t("toast.providerDeleted"));
    } catch (error) {
      if (!force && error instanceof ApiError && error.status === 409) {
        setDeleteProviderTarget(null);
        setForceDeleteProviderTarget(provider);
        setForceDeleteProviderMessage(resolveAdminErrorMessage(error));
        return;
      }
      toast.error(t("toast.providerDeleteFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }, [t]);

  const updateProviderControl = React.useCallback(async (provider: IdentityProviderDTO, key: "loginEnabled" | "registrationEnabled", value: boolean) => {
    if (key === "registrationEnabled" && value && !provider.loginEnabled) {
      toast.error(t("toast.enableLoginFirst"), { description: t("toast.registrationRequiresLogin") });
      return;
    }
    const previousProviders = providers;
    const updatedProvider = {
      ...provider,
      [key]: value,
      ...(key === "loginEnabled" && !value ? { registrationEnabled: false } : {}),
    };
    const nextProviders = providers.map((item) => (item.publicID === provider.publicID ? updatedProvider : item));
    setProviders(nextProviders);
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        setProviders(previousProviders);
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      await updateAdminIdentityProvider(token, provider.publicID, providerToForm(updatedProvider));
      toast.success(t("toast.providerControlUpdated"));
    } catch (error) {
      setProviders(previousProviders);
      toast.error(t("toast.providerControlSaveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }, [providers, t]);

  const saveProviderOrder = React.useCallback(async (orderedProviders: IdentityProviderDTO[], previousProviders: IdentityProviderDTO[]) => {
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        setProviders(previousProviders);
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      await reorderAdminIdentityProviders(token, orderedProviders.map((provider) => provider.publicID));
      toast.success(t("toast.providerOrderUpdated"));
    } catch (error) {
      setProviders(previousProviders);
      toast.error(t("toast.providerOrderSaveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }, [t]);

  const moveProviderTo = React.useCallback((providerID: string, targetProviderID: string) => {
    if (providerID === targetProviderID) {
      return;
    }
    const index = providers.findIndex((provider) => provider.publicID === providerID);
    const targetIndex = providers.findIndex((provider) => provider.publicID === targetProviderID);
    const previousProviders = providers;
    const orderedProviders = moveSortableItem(providers, index, targetIndex);
    if (orderedProviders === providers) {
      return;
    }
    setProviders(orderedProviders);
    void saveProviderOrder(orderedProviders, previousProviders);
  }, [providers, saveProviderOrder]);

  return {
    settingsMap,
    savedMap,
    configuredMap,
    providers,
    deleteProviderTarget,
    setDeleteProviderTarget,
    forceDeleteProviderTarget,
    setForceDeleteProviderTarget,
    forceDeleteProviderMessage,
    setForceDeleteProviderMessage,
    providerCallbackBaseURL,
    loading,
    saving,
    dirtyFieldIDs,
    updateSettingValue,
    handleSaveGroup,
    saveProvider,
    deleteProvider,
    updateProviderControl,
    moveProviderTo,
  };
}
