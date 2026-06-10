"use client";

import * as React from "react";
import { ArrowRight, ChevronDown, GripVertical, Pencil, Plus, Save, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { SettingsCollapsibleContent } from "../shared/settings-collapsible-content";
import { SettingsFieldEditor } from "../shared/settings-runtime-panel";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { createAdminIdentityProvider, deleteAdminIdentityProvider, listAdminIdentityProviders, listAdminSettings, patchAdminSettings, reorderAdminIdentityProviders, updateAdminIdentityProvider } from "@/features/admin/api";
import type { IdentityProviderPayload } from "@/features/admin/api/auth";
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableRow, TableSkeletonRows } from "@/components/ui/table";
import { ApiError } from "@/shared/api/http-client";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { CopyActionButton } from "@/shared/components/copy-action";
import { configuredSettingsMap } from "@/shared/lib/settings-meta";
import type { IdentityProviderDTO } from "@/shared/api/auth.types";
import type { PatchSettingItem } from "@/shared/api/settings.types";
import { IdentityProviderIcon } from "@/shared/components/identity-provider-icon";
import {
  SettingsFieldInset,
  SettingsFieldItem,
  SettingsFieldList,
  SettingsPage,
  SettingsSection,
  SettingsSectionSeparator,
} from "@/shared/components/settings-layout";
import {
  applyLoginDefaults,
  buildLoginSettingsGroups,
  createProviderForm,
  DEFAULT_PROVIDER_FORM,
  fieldID,
  flattenLoginSettings,
  includesEmailVerificationSettings,
  includesPasswordLoginSettings,
  includesTurnstileSettings,
  isEmailSMTPField,
  isRateLimitChildField,
  isTurnstileChildField,
  normalizeProviderSlugPreview,
  providerToForm,
  PROVIDER_TEMPLATES,
  reorderProviders,
  resolveErrorMessage,
  toEditorField,
  validateEmailVerificationSettings,
  validatePasswordLoginSettings,
  validateTurnstileSettings,
  type LoginSettingsField,
  type LoginSettingsGroup,
  type ProviderTemplate,
} from "@/features/admin/model/login-settings";

function RequiredMark() {
  return <span className="ml-0.5 text-destructive">*</span>;
}

function FieldMappingArrow() {
  return (
    <span aria-hidden="true" className="flex h-9 w-4 items-center justify-center self-end text-muted-foreground">
      <ArrowRight className="size-3.5" />
    </span>
  );
}

export function AdminLoginSettingsPage() {
  const t = useTranslations("adminLogin");
  const commonT = useTranslations("common");
  const loginSettingsGroups = React.useMemo(() => buildLoginSettingsGroups(t), [t]);
  const [settingsMap, setSettingsMap] = React.useState<Record<string, string>>(() => applyLoginDefaults({}));
  const [savedMap, setSavedMap] = React.useState<Record<string, string>>(() => applyLoginDefaults({}));
  const [configuredMap, setConfiguredMap] = React.useState<Record<string, boolean>>({});
  const [providers, setProviders] = React.useState<IdentityProviderDTO[]>([]);
  const [providerDialogOpen, setProviderDialogOpen] = React.useState(false);
  const [editingProvider, setEditingProvider] = React.useState<IdentityProviderDTO | null>(null);
  const [deleteProviderTarget, setDeleteProviderTarget] = React.useState<IdentityProviderDTO | null>(null);
  const [forceDeleteProviderTarget, setForceDeleteProviderTarget] = React.useState<IdentityProviderDTO | null>(null);
  const [forceDeleteProviderMessage, setForceDeleteProviderMessage] = React.useState("");
  const [providerForm, setProviderForm] = React.useState<IdentityProviderPayload>(DEFAULT_PROVIDER_FORM);
  const [oidcEndpointMode, setOidcEndpointMode] = React.useState<"issuer" | "discovery">("issuer");
  const [draggedProviderID, setDraggedProviderID] = React.useState<string | null>(null);
  const [dragOverProviderID, setDragOverProviderID] = React.useState<string | null>(null);
  const [frontendOrigin, setFrontendOrigin] = React.useState("");
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
      const [grouped, providerPage] = await Promise.all([listAdminSettings(token), listAdminIdentityProviders(token)]);
      const flattened = flattenLoginSettings(grouped);
      setConfiguredMap(configuredSettingsMap(grouped));
      setSettingsMap(flattened);
      setSavedMap(flattened);
      setProviders(providerPage.results);
    } catch (error) {
      toast.error(t("toast.loadFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setLoading(false);
    }
  }, [t]);

  React.useEffect(() => {
    void loadData();
  }, [loadData]);

  React.useEffect(() => {
    setFrontendOrigin(window.location.origin);
  }, []);

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
      return next;
    });
  }, [t]);

  const isFieldDisabled = React.useCallback((field: LoginSettingsField) => {
    if (loading || saving) return true;
    if (field.key === "email_registration_enabled" && settingsMap["auth.email_login_enabled"] === "false") return true;
    if (field.key === "turnstile_registration_enabled" && settingsMap["auth.email_registration_enabled"] === "false") return true;
    return false;
  }, [loading, saving, settingsMap]);

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
        toast.error(t("toast.saveFailed"), { description: resolveErrorMessage(error) });
      } finally {
        setSaving(false);
      }
    },
    [configuredMap, savedMap, settingsMap, t],
  );

  const openCreateProvider = React.useCallback((type: "oidc" | "oauth2") => {
    setEditingProvider(null);
    setProviderForm(createProviderForm({ type, scopes: type === "oidc" ? "openid profile email" : "profile email" }));
    setOidcEndpointMode("issuer");
    setProviderDialogOpen(true);
  }, []);

  const openCreateProviderFromTemplate = React.useCallback((template: ProviderTemplate) => {
    setEditingProvider(null);
    setProviderForm(createProviderForm(template.form));
    setOidcEndpointMode(template.form.discoveryURL ? "discovery" : "issuer");
    setProviderDialogOpen(true);
  }, []);

  const openEditProvider = React.useCallback((provider: IdentityProviderDTO) => {
    setEditingProvider(provider);
    setProviderForm(providerToForm(provider));
    setOidcEndpointMode(provider.discoveryURL ? "discovery" : "issuer");
    setProviderDialogOpen(true);
  }, []);

  const saveProvider = React.useCallback(async () => {
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      const payload = {
        ...providerForm,
        registrationEnabled: providerForm.loginEnabled && providerForm.registrationEnabled,
      };
      if (editingProvider) {
        await updateAdminIdentityProvider(token, editingProvider.publicID, payload);
      } else {
        await createAdminIdentityProvider(token, payload);
      }
      toast.success(t("toast.providerSaved"));
      setProviderDialogOpen(false);
      const page = await listAdminIdentityProviders(token);
      setProviders(page.results);
    } catch (error) {
      toast.error(t("toast.providerSaveFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }, [editingProvider, providerForm, t]);

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
        setForceDeleteProviderMessage(resolveErrorMessage(error));
        return;
      }
      toast.error(t("toast.providerDeleteFailed"), { description: resolveErrorMessage(error) });
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
      toast.error(t("toast.providerControlSaveFailed"), { description: resolveErrorMessage(error) });
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
      toast.error(t("toast.providerOrderSaveFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }, [t]);

  const dropProvider = React.useCallback((targetProvider: IdentityProviderDTO) => {
    if (!draggedProviderID || draggedProviderID === targetProvider.publicID) {
      setDraggedProviderID(null);
      setDragOverProviderID(null);
      return;
    }
    const previousProviders = providers;
    const orderedProviders = reorderProviders(providers, draggedProviderID, targetProvider.publicID);
    setDraggedProviderID(null);
    setDragOverProviderID(null);
    if (orderedProviders === providers) return;
    setProviders(orderedProviders);
    void saveProviderOrder(orderedProviders, previousProviders);
  }, [draggedProviderID, providers, saveProviderOrder]);

  const oidcEndpointValue = oidcEndpointMode === "discovery" ? (providerForm.discoveryURL ?? "") : (providerForm.issuerURL ?? "");
  const callbackSlug = providerForm.slug?.trim() || normalizeProviderSlugPreview(providerForm.name) || "provider";
  const callbackURL = `${frontendOrigin || "http://localhost:3000"}/auth/callback?provider=${encodeURIComponent(callbackSlug)}`;

  return (
    <SettingsPage>
      <div className="space-y-8">
        {loginSettingsGroups.map((group, index) => {
          const groupDirty = group.fields.some((field) => dirtyFieldIDs.has(fieldID(field)));
          const thirdPartyEnabled = (settingsMap["auth.third_party_login_enabled"] ?? "true") === "true";
          const visibleFields = group.fields.filter((field) => !isEmailSMTPField(field) || settingsMap["auth.email_verification_enabled"] !== "false");
          const mainFields = visibleFields.filter((field) => !isEmailSMTPField(field) && !isRateLimitChildField(field) && !isTurnstileChildField(field));
          const smtpFields = visibleFields.filter(isEmailSMTPField);
          const turnstileFields = visibleFields.filter(isTurnstileChildField);
          const rateLimitFields = visibleFields.filter(isRateLimitChildField);
          return (
            <React.Fragment key={group.title}>
              <SettingsSection
                title={group.title}
                actions={
                  groupDirty ? (
                    <Button type="button" size="sm" disabled={loading || saving} onClick={() => void handleSaveGroup(group)}>
                      <Save className="size-3.5" />
                      {commonT("actions.save")}
                    </Button>
                  ) : null
                }
              >

                <div className="space-y-4">
                  <SettingsFieldList>
                    {mainFields.map((field, fieldIndex) => {
                      const id = fieldID(field);
                      const showSMTPFields = field.key === "email_verification_enabled" && settingsMap["auth.email_verification_enabled"] !== "false";
                      const showTurnstileFields = field.key === "turnstile_registration_enabled" && settingsMap["auth.turnstile_registration_enabled"] === "true" && turnstileFields.length > 0;
                      const showRateLimitFields = field.key === "rate_limit_enabled" && settingsMap["auth.rate_limit_enabled"] === "true" && rateLimitFields.length > 0;
                      return (
                        <React.Fragment key={id}>
                          <SettingsFieldItem index={fieldIndex}>
                            <SettingsFieldEditor
                              field={toEditorField(field)}
                              value={settingsMap[id] ?? ""}
                              configured={configuredMap[id]}
                              dirty={(settingsMap[id] ?? "") !== (savedMap[id] ?? "")}
                              disabled={isFieldDisabled(field)}
                              onChange={(value) => updateSettingValue(field, value)}
                            />
                          </SettingsFieldItem>
                          {showSMTPFields ? (
                            <SettingsFieldInset className="mt-3 md:mt-4">
                              <SettingsFieldList className="gap-3 md:gap-4">
                                {smtpFields.map((smtpField) => {
                                  const smtpFieldID = fieldID(smtpField);
                                  return (
                                    <SettingsFieldEditor
                                      key={smtpFieldID}
                                      field={toEditorField(smtpField)}
                                      value={settingsMap[smtpFieldID] ?? ""}
                                      configured={configuredMap[smtpFieldID]}
                                      dirty={(settingsMap[smtpFieldID] ?? "") !== (savedMap[smtpFieldID] ?? "")}
                                      disabled={isFieldDisabled(smtpField)}
                                      onChange={(value) => updateSettingValue(smtpField, value)}
                                    />
                                  );
                                })}
                              </SettingsFieldList>
                            </SettingsFieldInset>
                          ) : null}
                          {field.key === "rate_limit_enabled" ? (
                            <SettingsCollapsibleContent open={showRateLimitFields}>
                              <SettingsFieldInset className="mt-3 md:mt-4">
                                <SettingsFieldList className="gap-3 md:gap-4">
                                  {rateLimitFields.map((rateLimitField) => {
                                    const rateLimitFieldID = fieldID(rateLimitField);
                                    return (
                                      <SettingsFieldEditor
                                        key={rateLimitFieldID}
                                        field={toEditorField(rateLimitField)}
                                        value={settingsMap[rateLimitFieldID] ?? ""}
                                        configured={configuredMap[rateLimitFieldID]}
                                        dirty={(settingsMap[rateLimitFieldID] ?? "") !== (savedMap[rateLimitFieldID] ?? "")}
                                        disabled={isFieldDisabled(rateLimitField)}
                                        onChange={(value) => updateSettingValue(rateLimitField, value)}
                                      />
                                    );
                                  })}
                                </SettingsFieldList>
                              </SettingsFieldInset>
                            </SettingsCollapsibleContent>
                          ) : null}
                          {field.key === "turnstile_registration_enabled" ? (
                            <SettingsCollapsibleContent open={showTurnstileFields}>
                              <SettingsFieldInset className="mt-3 md:mt-4">
                                <SettingsFieldList className="gap-3 md:gap-4">
                                  {turnstileFields.map((turnstileField) => {
                                    const turnstileFieldID = fieldID(turnstileField);
                                    return (
                                      <SettingsFieldEditor
                                        key={turnstileFieldID}
                                        field={toEditorField(turnstileField)}
                                        value={settingsMap[turnstileFieldID] ?? ""}
                                        configured={configuredMap[turnstileFieldID]}
                                        dirty={(settingsMap[turnstileFieldID] ?? "") !== (savedMap[turnstileFieldID] ?? "")}
                                        disabled={isFieldDisabled(turnstileField)}
                                        onChange={(value) => updateSettingValue(turnstileField, value)}
                                      />
                                    );
                                  })}
                                </SettingsFieldList>
                              </SettingsFieldInset>
                            </SettingsCollapsibleContent>
                          ) : null}
                        </React.Fragment>
                      );
                    })}
                  </SettingsFieldList>
                  {group.fields.some((field) => field.key === "third_party_login_enabled") && thirdPartyEnabled ? (
                    <Field>
                      <div className="flex flex-col gap-2 md:flex-row md:items-start md:gap-4 xl:gap-6">
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <FieldLabel>{t("providers.title")}</FieldLabel>
                          </div>
                        </div>

                        <div className="w-full min-w-0 md:w-44 md:shrink-0 xl:w-52">
                          <div className="flex items-end justify-start md:justify-end">
                            <DropdownMenu modal={false}>
                              <DropdownMenuTrigger asChild>
                                <Button type="button" size="sm" disabled={loading || saving}>
                                  <Plus className="size-3.5" />
                                  {t("providers.create")}
                                </Button>
                              </DropdownMenuTrigger>
                              <DropdownMenuContent align="end" className="w-44">
                                {PROVIDER_TEMPLATES.map((template) => (
                                  <DropdownMenuItem key={template.label} onClick={() => openCreateProviderFromTemplate(template)}>
                                    <IdentityProviderIcon name={template.form.name} slug="" />
                                    {template.label}
                                  </DropdownMenuItem>
                                ))}
                                <DropdownMenuSeparator />
                                <DropdownMenuItem onClick={() => openCreateProvider("oidc")}>{t("providers.customOIDC")}</DropdownMenuItem>
                                <DropdownMenuItem onClick={() => openCreateProvider("oauth2")}>{t("providers.customOAuth2")}</DropdownMenuItem>
                              </DropdownMenuContent>
                            </DropdownMenu>
                          </div>
                        </div>
                      </div>

                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead className="w-8" />
                            <TableHead className="w-[220px]">{t("providers.name")}</TableHead>
                            <TableHead>{t("providers.type")}</TableHead>
                            <TableHead className="text-center">{t("providers.loginControl")}</TableHead>
                            <TableHead className="text-center">{t("providers.registrationControl")}</TableHead>
                            <TableHead>{t("providerDialog.clientID")}</TableHead>
                            <TableHead className="w-[68px]" stickyEnd />
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {loading && providers.length === 0 ? <TableSkeletonRows colSpan={7} rowCount={6} /> : null}
                          {providers.map((provider) => (
                            <TableRow
                              key={provider.publicID}
                              className={dragOverProviderID === provider.publicID && draggedProviderID !== provider.publicID ? "bg-muted/40" : undefined}
                              onDragOver={(event) => {
                                if (!draggedProviderID || draggedProviderID === provider.publicID) return;
                                event.preventDefault();
                                event.dataTransfer.dropEffect = "move";
                                setDragOverProviderID(provider.publicID);
                              }}
                              onDragLeave={() => {
                                setDragOverProviderID((current) => (current === provider.publicID ? null : current));
                              }}
                              onDrop={(event) => {
                                event.preventDefault();
                                dropProvider(provider);
                              }}
                            >
                              <TableCell className="w-8 py-1.5 text-center text-muted-foreground">
                                <div className="flex h-7 items-center justify-center">
                                  <button
                                    type="button"
                                    draggable={!saving && providers.length > 1}
                                    className="inline-flex size-4 cursor-grab items-center justify-center text-muted-foreground transition-colors hover:text-foreground active:cursor-grabbing disabled:cursor-not-allowed disabled:opacity-50"
                                    disabled={saving || providers.length < 2}
                                    aria-label={t("providers.dragToReorder", { name: provider.name })}
                                    onDragStart={(event) => {
                                      setDraggedProviderID(provider.publicID);
                                      event.dataTransfer.effectAllowed = "move";
                                      event.dataTransfer.setData("text/plain", provider.publicID);
                                    }}
                                    onDragEnd={() => {
                                      setDraggedProviderID(null);
                                      setDragOverProviderID(null);
                                    }}
                                  >
                                    <GripVertical className="size-3.5" />
                                  </button>
                                </div>
                              </TableCell>
                              <TableCell className="w-[220px] max-w-[220px] py-1.5">
                                <div className="flex h-7 max-w-[220px] min-w-0 items-center gap-2">
                                  <IdentityProviderIcon name={provider.name} slug={provider.slug} logoURL={provider.logoURL} />
                                  <button
                                    type="button"
                                    className="inline-flex min-w-0 max-w-full items-baseline gap-1.5 text-left font-medium hover:underline"
                                    title={`${provider.name} (${provider.slug})`}
                                    onClick={() => openEditProvider(provider)}
                                  >
                                    <span className="min-w-0 truncate">{provider.name}</span>
                                    <span className="shrink-0 text-xs font-normal text-muted-foreground">({provider.slug})</span>
                                  </button>
                                </div>
                              </TableCell>
                              <TableCell className="py-1.5 uppercase">
                                <span className="flex h-7 items-center">{provider.type}</span>
                              </TableCell>
                              <TableCell className="py-1.5 text-center">
                                <div className="flex h-7 items-center justify-center">
                                  <Switch
                                    size="sm"
                                    checked={provider.loginEnabled}
                                    disabled={saving}
                                    aria-label={t("providers.loginControlFor", { name: provider.name })}
                                    onCheckedChange={(checked) => void updateProviderControl(provider, "loginEnabled", checked)}
                                  />
                                </div>
                              </TableCell>
                              <TableCell className="py-1.5 text-center">
                                <div className="flex h-7 items-center justify-center">
                                  <Switch
                                    size="sm"
                                    checked={provider.loginEnabled && provider.registrationEnabled}
                                    disabled={saving || !provider.loginEnabled}
                                    aria-label={t("providers.registrationControlFor", { name: provider.name })}
                                    onCheckedChange={(checked) => void updateProviderControl(provider, "registrationEnabled", checked)}
                                  />
                                </div>
                              </TableCell>
                              <TableCell className="max-w-52 py-1.5 font-mono text-xs">
                                <span className="flex h-7 items-center truncate">{provider.clientID || "-"}</span>
                              </TableCell>
                              <TableCell className="w-[68px] py-1.5 whitespace-nowrap" stickyEnd onClick={(event) => event.stopPropagation()}>
                                <div className="flex h-7 items-center justify-start gap-1 md:justify-end">
                                  <Button type="button" size="icon-xs" variant="ghost" className="text-muted-foreground shadow-none" onClick={() => openEditProvider(provider)} disabled={saving} title={t("providers.edit")} aria-label={t("providers.edit")}>
                                    <Pencil className="size-3.5 stroke-1" />
                                  </Button>
                                  <Button type="button" size="icon-xs" variant="ghost" className="text-muted-foreground shadow-none" onClick={() => setDeleteProviderTarget(provider)} disabled={saving} title={t("providers.delete")} aria-label={t("providers.delete")}>
                                    <Trash2 className="size-3.5 stroke-1" />
                                  </Button>
                                </div>
                              </TableCell>
                            </TableRow>
                          ))}
                          {!loading && providers.length === 0 ? (
                            <TableEmptyRow colSpan={7}>{t("providers.empty")}</TableEmptyRow>
                          ) : null}
                        </TableBody>
                      </Table>
                    </Field>
                  ) : null}
                </div>
              </SettingsSection>
              {index < loginSettingsGroups.length - 1 ? <SettingsSectionSeparator /> : null}
            </React.Fragment>
          );
        })}
      </div>

      <Dialog open={providerDialogOpen} onOpenChange={setProviderDialogOpen}>
        <DialogContent className="flex max-h-[min(86vh,760px)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[680px]">
          <DialogHeader className="shrink-0 px-4 py-4">
            <DialogTitle>{editingProvider ? t("providerDialog.editTitle") : t("providerDialog.createTitle")}</DialogTitle>
            <DialogDescription>{t("providerDialog.description")}</DialogDescription>
          </DialogHeader>
          <div className="grid min-h-0 flex-1 grid-cols-2 gap-3 overflow-y-auto px-4 py-2">
            <div className="flex items-center justify-between rounded-md bg-muted/50 px-3 py-2">
              <div className="space-y-0.5">
                <div className="text-xs font-medium">{t("providers.loginControl")}</div>
                <div className="text-[11px] text-muted-foreground">{t("providerDialog.loginControlDescription")}</div>
              </div>
              <Switch
                checked={providerForm.loginEnabled}
                onCheckedChange={(checked) =>
                  setProviderForm((prev) => ({
                    ...prev,
                    loginEnabled: checked,
                    registrationEnabled: checked ? prev.registrationEnabled : false,
                  }))
                }
              />
            </div>
            <div className="flex items-center justify-between rounded-md bg-muted/50 px-3 py-2">
              <div className="space-y-0.5">
                <div className="text-xs font-medium">{t("providers.registrationControl")}</div>
                <div className="text-[11px] text-muted-foreground">{t("providerDialog.registrationControlDescription")}</div>
              </div>
              <Switch
                checked={providerForm.loginEnabled && providerForm.registrationEnabled}
                disabled={!providerForm.loginEnabled}
                onCheckedChange={(checked) => setProviderForm((prev) => ({ ...prev, registrationEnabled: checked }))}
              />
            </div>
            <label className="space-y-1 text-sm">
              <span className="text-xs text-muted-foreground">{t("providers.type")}<RequiredMark /></span>
              <Select
                value={providerForm.type}
                onValueChange={(value) => {
                  const type = value as "oidc" | "oauth2";
                  setProviderForm((prev) => ({ ...prev, type }));
                  if (type === "oidc") setOidcEndpointMode(providerForm.discoveryURL ? "discovery" : "issuer");
                }}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="oidc">OIDC</SelectItem>
                  <SelectItem value="oauth2">OAuth2</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label className="space-y-1 text-sm">
              <span className="text-xs text-muted-foreground">{t("providers.name")}<RequiredMark /></span>
              <Input value={providerForm.name} onChange={(event) => setProviderForm((prev) => ({ ...prev, name: event.target.value }))} />
            </label>
            <label className="col-span-2 space-y-1 text-sm">
              <span className="text-xs text-muted-foreground">{t("providerDialog.callbackURL")}</span>
              <div className="grid grid-cols-[minmax(0,1fr)_auto] gap-2">
                <Input value={callbackURL} disabled readOnly />
                <CopyActionButton
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="text-muted-foreground shadow-none"
                  value={callbackURL}
                  messages={{ copied: t("toast.callbackCopied"), failed: commonT("errors.copyFailed") }}
                  aria-label={t("providerDialog.copyCallbackURL")}
                  title={t("providerDialog.copyCallbackURL")}
                />
              </div>
            </label>
            <label className="col-span-2 space-y-1 text-sm">
              <span className="text-xs text-muted-foreground">{t("providerDialog.logoURL")}</span>
              <div className="grid grid-cols-[minmax(0,1fr)_auto] gap-2">
                <Input value={providerForm.logoURL ?? ""} onChange={(event) => setProviderForm((prev) => ({ ...prev, logoURL: event.target.value }))} placeholder="https://example.com/logo.svg" />
                <div className="grid h-8 w-8 place-items-center rounded-md border border-input/40 bg-transparent">
                  <IdentityProviderIcon
                    name={providerForm.name || "Provider"}
                    slug={providerForm.slug || normalizeProviderSlugPreview(providerForm.name)}
                    logoURL={providerForm.logoURL}
                    className="size-5"
                    iconClassName="size-5"
                    fallbackClassName="text-sm font-semibold uppercase"
                  />
                </div>
              </div>
            </label>
            <label className="col-span-2 space-y-1 text-sm">
              <span className="text-xs text-muted-foreground">{t("providerDialog.clientID")}<RequiredMark /></span>
              <Input value={providerForm.clientID} onChange={(event) => setProviderForm((prev) => ({ ...prev, clientID: event.target.value }))} />
            </label>
            <label className="col-span-2 space-y-1 text-sm">
              <span className="text-xs text-muted-foreground">{t("providerDialog.clientSecret")}{editingProvider ? null : <RequiredMark />}</span>
              <Input type="password" value={providerForm.clientSecret ?? ""} onChange={(event) => setProviderForm((prev) => ({ ...prev, clientSecret: event.target.value }))} placeholder={editingProvider ? commonT("input.configuredPasswordPlaceholder") : ""} />
            </label>
            {providerForm.type === "oidc" ? (
              <label className="col-span-2 space-y-1 text-sm">
                <span className="text-xs text-muted-foreground">{t("providerDialog.oidcEndpoint")}<RequiredMark /></span>
                <div className="grid grid-cols-[140px_minmax(0,1fr)] gap-2">
                  <Select
                    value={oidcEndpointMode}
                    onValueChange={(value) => {
                      const mode = value as "issuer" | "discovery";
                      setOidcEndpointMode(mode);
                      setProviderForm((prev) =>
                        mode === "discovery"
                          ? { ...prev, discoveryURL: prev.discoveryURL || prev.issuerURL || "", issuerURL: "" }
                          : { ...prev, issuerURL: prev.issuerURL || prev.discoveryURL || "", discoveryURL: "" },
                      );
                    }}
                  >
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="issuer">{t("providerDialog.issuerURL")}</SelectItem>
                      <SelectItem value="discovery">{t("providerDialog.discoveryURL")}</SelectItem>
                    </SelectContent>
                  </Select>
                  <Input
                    value={oidcEndpointValue}
                    onChange={(event) =>
                      setProviderForm((prev) =>
                        oidcEndpointMode === "discovery"
                          ? { ...prev, discoveryURL: event.target.value, issuerURL: "" }
                          : { ...prev, issuerURL: event.target.value, discoveryURL: "" },
                      )
                    }
                    placeholder={oidcEndpointMode === "discovery" ? "https://example.com/.well-known/openid-configuration" : "https://example.com"}
                  />
                </div>
              </label>
            ) : (
              <>
                <label className="col-span-2 space-y-1 text-sm"><span className="text-xs text-muted-foreground">{t("providerDialog.authURL")}<RequiredMark /></span><Input value={providerForm.authURL ?? ""} onChange={(event) => setProviderForm((prev) => ({ ...prev, authURL: event.target.value }))} /></label>
                <label className="col-span-2 space-y-1 text-sm"><span className="text-xs text-muted-foreground">{t("providerDialog.tokenURL")}<RequiredMark /></span><Input value={providerForm.tokenURL ?? ""} onChange={(event) => setProviderForm((prev) => ({ ...prev, tokenURL: event.target.value }))} /></label>
                <label className="col-span-2 space-y-1 text-sm"><span className="text-xs text-muted-foreground">{t("providerDialog.userinfoURL")}<RequiredMark /></span><Input value={providerForm.userinfoURL ?? ""} onChange={(event) => setProviderForm((prev) => ({ ...prev, userinfoURL: event.target.value }))} /></label>
              </>
            )}
            <label className="col-span-2 space-y-1 text-sm"><span className="text-xs text-muted-foreground">{t("providerDialog.scopes")}</span><Input value={providerForm.scopes ?? ""} onChange={(event) => setProviderForm((prev) => ({ ...prev, scopes: event.target.value }))} /></label>
            <Separator className="col-span-2 my-2" />
            <Accordion type="single" collapsible className="col-span-2 -mt-1">
              <AccordionItem value="claim-mapping" className="border-b-0">
                <AccordionTrigger className="py-1 text-xs hover:no-underline">{t("providerDialog.advancedSettings")}</AccordionTrigger>
                <AccordionContent className="space-y-3 pb-0 pt-2">
                  <div className="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-end gap-3">
                    <label className="space-y-1 text-sm">
                      <span className="text-xs text-muted-foreground">{t("providerDialog.sourceField")}</span>
                      <Input value={providerForm.subjectField ?? ""} onChange={(event) => setProviderForm((prev) => ({ ...prev, subjectField: event.target.value }))} placeholder="sub" />
                    </label>
                    <FieldMappingArrow />
                    <label className="space-y-1 text-sm">
                      <span className="text-xs text-muted-foreground">{t("providerDialog.systemField")}</span>
                      <Input value={t("providerDialog.systemFields.userID")} disabled readOnly />
                    </label>
                  </div>
                  <div className="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-end gap-3">
                    <label className="space-y-1 text-sm">
                      <span className="text-xs text-muted-foreground">{t("providerDialog.sourceField")}</span>
                      <Input value={providerForm.emailField ?? ""} onChange={(event) => setProviderForm((prev) => ({ ...prev, emailField: event.target.value }))} placeholder="email" />
                    </label>
                    <FieldMappingArrow />
                    <label className="space-y-1 text-sm">
                      <span className="text-xs text-muted-foreground">{t("providerDialog.systemField")}</span>
                      <Input value={t("providerDialog.systemFields.email")} disabled readOnly />
                    </label>
                  </div>
                  <div className="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-end gap-3">
                    <label className="space-y-1 text-sm">
                      <span className="text-xs text-muted-foreground">{t("providerDialog.sourceField")}</span>
                      <Input value={providerForm.emailVerifiedField ?? ""} onChange={(event) => setProviderForm((prev) => ({ ...prev, emailVerifiedField: event.target.value }))} placeholder="email_verified" />
                    </label>
                    <FieldMappingArrow />
                    <label className="space-y-1 text-sm">
                      <span className="text-xs text-muted-foreground">{t("providerDialog.systemField")}</span>
                      <Input value={t("providerDialog.systemFields.emailVerified")} disabled readOnly />
                    </label>
                  </div>
                  <div className="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-end gap-3">
                    <label className="space-y-1 text-sm">
                      <span className="text-xs text-muted-foreground">{t("providerDialog.sourceField")}</span>
                      <Input value={providerForm.nameField ?? ""} onChange={(event) => setProviderForm((prev) => ({ ...prev, nameField: event.target.value }))} placeholder="name" />
                    </label>
                    <FieldMappingArrow />
                    <label className="space-y-1 text-sm">
                      <span className="text-xs text-muted-foreground">{t("providerDialog.systemField")}</span>
                      <Input value={t("providerDialog.systemFields.displayName")} disabled readOnly />
                    </label>
                  </div>
                  <div className="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-end gap-3">
                    <label className="space-y-1 text-sm">
                      <span className="text-xs text-muted-foreground">{t("providerDialog.sourceField")}</span>
                      <Input value={providerForm.avatarField ?? ""} onChange={(event) => setProviderForm((prev) => ({ ...prev, avatarField: event.target.value }))} placeholder="picture" />
                    </label>
                    <FieldMappingArrow />
                    <label className="space-y-1 text-sm">
                      <span className="text-xs text-muted-foreground">{t("providerDialog.systemField")}</span>
                      <Input value={t("providerDialog.systemFields.avatar")} disabled readOnly />
                    </label>
                  </div>
                </AccordionContent>
              </AccordionItem>
            </Accordion>
          </div>
          <DialogFooter className="shrink-0 px-4 py-3">
            <Button variant="ghost" onClick={() => setProviderDialogOpen(false)}>{commonT("actions.cancel")}</Button>
            <Button type="button" onClick={() => void saveProvider()} disabled={saving}>{commonT("actions.save")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={Boolean(deleteProviderTarget)} onOpenChange={(open) => !open && setDeleteProviderTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteDialog.title")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("deleteDialog.description", { name: deleteProviderTarget?.name ?? t("deleteDialog.thisProvider") })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={saving}>{commonT("actions.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={saving || !deleteProviderTarget}
              onClick={() => deleteProviderTarget && void deleteProvider(deleteProviderTarget)}
            >
              {commonT("actions.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={Boolean(forceDeleteProviderTarget)} onOpenChange={(open) => {
        if (!open) {
          setForceDeleteProviderTarget(null);
          setForceDeleteProviderMessage("");
        }
      }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("forceDeleteDialog.title")}</AlertDialogTitle>
            <AlertDialogDescription>
              {forceDeleteProviderMessage || t("forceDeleteDialog.description")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={saving}>{commonT("actions.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={saving || !forceDeleteProviderTarget}
              onClick={() => forceDeleteProviderTarget && void deleteProvider(forceDeleteProviderTarget, true)}
            >
              {t("forceDeleteDialog.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsPage>
  );
}
