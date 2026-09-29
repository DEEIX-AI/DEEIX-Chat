"use client";

import * as React from "react";
import { ArrowRight, Pencil, Plus, Save, Trash2, TriangleAlert } from "lucide-react";
import { useTranslations } from "next-intl";

import { CollapsibleMotionContent } from "@/shared/components/collapsible-motion-content";
import { SettingsFieldEditor } from "../../shared/settings-runtime-panel";
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
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogHeightTransition, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { useAdminLoginSettings } from "@/features/admin/hooks/use-admin-login-settings";
import {
  AdminSortableHandle,
  AdminSortableItem,
  AdminSortableList,
} from "@/features/admin/components/shared/sortable-list";
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableLoadingRow, TableRow } from "@/components/ui/table";
import { CopyActionButton } from "@/shared/components/copy-action";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import type { IdentityProviderDTO } from "@/shared/api/auth-types";
import { IdentityProviderIcon } from "@/entities/identity-provider";
import {
  SettingsFieldInset,
  SettingsFieldItem,
  SettingsFieldList,
  SettingsPage,
  SettingsSection,
  SettingsSectionSeparator,
} from "@/shared/components/settings-layout";
import {
  buildLoginSettingsGroups,
  createProviderForm,
  DEFAULT_PROVIDER_FORM,
  type IdentityProviderForm,
  fieldID,
  isEmailSMTPField,
  isRateLimitChildField,
  isTurnstileChildField,
  normalizeProviderSlugPreview,
  providerToForm,
  PROVIDER_TEMPLATES,
  toEditorField,
  type LoginSettingsField,
  type ProviderTemplate,
  type IdentityProviderType,
  isIdentityProviderType,
  isOIDCEndpointMode,
  type OIDCEndpointMode,
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

export function AdminLoginPage() {
  const t = useTranslations("adminLogin");
  const commonT = useTranslations("common");
  const loginSettingsGroups = React.useMemo(() => buildLoginSettingsGroups(t), [t]);
  const [providerDialogOpen, setProviderDialogOpen] = React.useState(false);
  const [editingProvider, setEditingProvider] = React.useState<IdentityProviderDTO | null>(null);
  const [providerForm, setProviderForm] = React.useState<IdentityProviderForm>(DEFAULT_PROVIDER_FORM);
  const [oidcEndpointMode, setOidcEndpointMode] = React.useState<OIDCEndpointMode>("issuer");
  const {
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
    saveProvider: saveIdentityProvider,
    deleteProvider,
    updateProviderControl,
    moveProviderTo,
  } = useAdminLoginSettings(loginSettingsGroups);
  const stableDeleteProviderTarget = useDialogSnapshot(deleteProviderTarget);

  const isFieldDisabled = React.useCallback((field: LoginSettingsField) => {
    if (loading || saving) return true;
    if (field.key === "email_registration_enabled" && settingsMap["auth.email_login_enabled"] === "false") return true;
    if (field.key === "password_reset_enabled" && settingsMap["auth.email_verification_enabled"] === "false") return true;
    if (field.key === "turnstile_registration_enabled" && settingsMap["auth.email_registration_enabled"] === "false") return true;
    return false;
  }, [loading, saving, settingsMap]);

  const openCreateProvider = React.useCallback((type: IdentityProviderType) => {
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

  const saveProvider = React.useCallback(
    () => saveIdentityProvider({ editingProvider, providerForm }, () => setProviderDialogOpen(false)),
    [editingProvider, providerForm, saveIdentityProvider],
  );

  const oidcEndpointValue = oidcEndpointMode === "discovery" ? (providerForm.discoveryURL ?? "") : (providerForm.issuerURL ?? "");
  const callbackSlug = providerForm.slug?.trim() || normalizeProviderSlugPreview(providerForm.name) || "provider";
  const callbackURL = providerCallbackBaseURL ? `${providerCallbackBaseURL}/${encodeURIComponent(callbackSlug)}/callback` : "";

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
                            <CollapsibleMotionContent open={showRateLimitFields}>
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
                            </CollapsibleMotionContent>
                          ) : null}
                          {field.key === "turnstile_registration_enabled" ? (
                            <CollapsibleMotionContent open={showTurnstileFields}>
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
                            </CollapsibleMotionContent>
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

                      {!loading && !providerCallbackBaseURL ? (
                        <Alert variant="destructive" className="mb-3">
                          <TriangleAlert />
                          <AlertTitle>{t("providers.bridgeMissingTitle")}</AlertTitle>
                          <AlertDescription>{t("providers.bridgeMissingDescription")}</AlertDescription>
                        </Alert>
                      ) : null}

                      <AdminSortableList
                        items={providers.map((provider) => provider.publicID)}
                        disabled={saving || providers.length < 2}
                        onMove={moveProviderTo}
                      >
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
                            {loading && providers.length === 0 ? <TableLoadingRow colSpan={7} /> : null}
                            {providers.map((provider) => (
                              <AdminSortableItem
                                key={provider.publicID}
                                asChild
                                id={provider.publicID}
                                disabled={saving || providers.length < 2}
                              >
                                {({ attributes, isDragging, listeners }) => (
                                  <TableRow className={isDragging ? "opacity-45" : undefined}>
                                    <TableCell className="w-8 py-1.5 text-center text-muted-foreground">
                                      <div className="flex h-7 items-center justify-center">
                                        <AdminSortableHandle
                                          attributes={attributes}
                                          className="size-4 bg-transparent p-0 hover:bg-transparent hover:text-foreground"
                                          disabled={saving}
                                          hidden={providers.length < 2}
                                          label={t("providers.dragToReorder", { name: provider.name })}
                                          listeners={listeners}
                                        />
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
                                )}
                              </AdminSortableItem>
                            ))}
                            {!loading && providers.length === 0 ? (
                              <TableEmptyRow colSpan={7}>{t("providers.empty")}</TableEmptyRow>
                            ) : null}
                          </TableBody>
                        </Table>
                      </AdminSortableList>
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
        <DialogContent className="gap-0 overflow-hidden p-0 sm:max-w-[680px]">
          <DialogHeightTransition contentClassName="max-h-[min(86vh,760px)]">
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
                    if (!isIdentityProviderType(value)) return;
                    const type = value;
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
                  <Input value={callbackURL} placeholder={t("providerDialog.callbackURLUnavailable")} disabled readOnly />
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
                        if (!isOIDCEndpointMode(value)) return;
                        const mode = value;
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
          </DialogHeightTransition>
        </DialogContent>
      </Dialog>

      <AlertDialog open={Boolean(deleteProviderTarget)} onOpenChange={(open) => !open && setDeleteProviderTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteDialog.title")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("deleteDialog.description", { name: stableDeleteProviderTarget?.name ?? t("deleteDialog.thisProvider") })}
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
