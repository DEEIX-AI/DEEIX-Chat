"use client";

import { InfoIcon, RotateCwIcon } from "lucide-react";
import { useTranslations } from "next-intl";
import type * as React from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { useDesktopNetworkProxy } from "@/features/desktop/hooks/use-desktop-network-proxy";
import { CollapsibleMotionContent } from "@/shared/components/collapsible-motion-content";
import {
  SettingsFieldInset,
  SettingsFieldItem,
  SettingsFieldList,
  SettingsFieldRow,
  SettingsPage,
  SettingsSection,
} from "@/shared/components/settings-layout";
import { isOneOf } from "@/shared/lib/type-guards";
import {
  PROXY_MODES,
  PROXY_SCHEMES,
  type ProxyErrorField,
  type ProxyScheme,
  type ProxySettings,
  type ProxyStatus,
} from "@/shared/platform/desktop-proxy";

// The Network settings tab of the desktop app, laid out like every other
// settings row (label and description left, control right; long lists below).
// Renders nothing in a browser: the platform check lives in the hook.

const SELECT_TRIGGER_CLASS =
  "text-left md:text-right *:data-[slot=select-value]:flex-1 *:data-[slot=select-value]:justify-start md:*:data-[slot=select-value]:justify-end";
const INPUT_CLASS = "min-w-0 text-left md:text-right";
const SCHEME_LABELS: Record<ProxyScheme, string> = { http: "HTTP", socks5: "SOCKS5" };
const MAX_PORT = 65535;

export function DesktopNetworkSettings() {
  const t = useTranslations("desktopNetwork");
  const common = useTranslations("common");
  const { status, proxy, draft, setDraft, password, setPassword, dirty, saving, fieldError, save, reset, restart } =
    useDesktopNetworkProxy();

  if (status !== "ready" || proxy === null) {
    return null;
  }
  const saved = proxy.settings;
  const managed = proxy.managedByPolicy;
  const errorFor = (field: ProxyErrorField) => (fieldError?.field === field ? fieldError.message : undefined);
  const changed = (key: keyof ProxySettings) => draft[key] !== saved[key];
  const update = <K extends keyof ProxySettings>(key: K, value: ProxySettings[K]) =>
    setDraft((current) => ({ ...current, [key]: value }));
  const unsaved = <Badge variant="ghost" className="relative -mt-1.5 text-[8px] font-medium text-amber-800">{common("states.unsaved")}</Badge>;

  return (
    <SettingsPage>
      <SettingsSection
        title={t("title")}
        actions={
          dirty && !managed ? (
            <>
              <Button type="button" variant="ghost" size="sm" disabled={saving} onClick={reset}>
                {common("actions.reset")}
              </Button>
              <Button type="button" size="sm" disabled={saving} onClick={() => void save()}>
                {saving ? <SpinnerLabel>{common("actions.saving")}</SpinnerLabel> : common("actions.save")}
              </Button>
            </>
          ) : null
        }
      >
        <SettingsFieldList>
          {/* Under policy the user's own choice does not apply, so it is not offered. */}
          {managed ? null : (
            <SettingsFieldItem index={0}>
              <ControlRow
                id="network-proxy-mode"
                label={t("mode.title")}
                description={t("mode.description")}
                badge={changed("mode") ? unsaved : null}
              >
                <Select
                  value={draft.mode}
                  disabled={saving}
                  onValueChange={(mode) => {
                    if (isOneOf(PROXY_MODES)(mode)) update("mode", mode);
                  }}
                >
                  <SelectTrigger id="network-proxy-mode" size="sm" className={SELECT_TRIGGER_CLASS}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent align="start">
                    {PROXY_MODES.map((mode) => (
                      <SelectItem key={mode} value={mode} className="text-left md:text-right">
                        {t(`mode.${mode}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </ControlRow>
            </SettingsFieldItem>
          )}

          <CollapsibleMotionContent open={draft.mode === "manual" && !managed} contentClassName="-mx-px px-px pb-px">
            <SettingsFieldItem index={1}>
              <SettingsFieldInset>
                <SettingsFieldList className="gap-3 md:gap-4">
                  <ControlRow
                    id="network-proxy-scheme"
                    label={t("fields.scheme")}
                    description={t("fields.schemeDescription")}
                    badge={changed("scheme") ? unsaved : null}
                  >
                    <Select
                      value={draft.scheme}
                      disabled={saving}
                      onValueChange={(scheme) => {
                        if (isOneOf(PROXY_SCHEMES)(scheme)) update("scheme", scheme);
                      }}
                    >
                      <SelectTrigger id="network-proxy-scheme" size="sm" className={SELECT_TRIGGER_CLASS}>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent align="start">
                        {PROXY_SCHEMES.map((scheme) => (
                          <SelectItem key={scheme} value={scheme} className="text-left md:text-right">
                            {SCHEME_LABELS[scheme]}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </ControlRow>

                  <ControlRow
                    id="network-proxy-host"
                    label={t("fields.host")}
                    description={t("fields.hostDescription")}
                    badge={changed("host") ? unsaved : null}
                    error={errorFor("host")}
                  >
                    <Input
                      id="network-proxy-host"
                      value={draft.host}
                      placeholder="127.0.0.1"
                      autoComplete="off"
                      spellCheck={false}
                      disabled={saving}
                      aria-invalid={errorFor("host") ? true : undefined}
                      className={INPUT_CLASS}
                      onChange={(event) => update("host", event.target.value)}
                    />
                  </ControlRow>

                  <ControlRow
                    id="network-proxy-port"
                    label={t("fields.port")}
                    description={t("fields.portDescription")}
                    badge={changed("port") ? unsaved : null}
                    error={errorFor("port")}
                  >
                    <Input
                      id="network-proxy-port"
                      value={draft.port === null ? "" : String(draft.port)}
                      placeholder="7890"
                      inputMode="numeric"
                      autoComplete="off"
                      disabled={saving}
                      aria-invalid={errorFor("port") ? true : undefined}
                      className={INPUT_CLASS}
                      onChange={(event) => update("port", parsePort(event.target.value))}
                    />
                  </ControlRow>

                  <ControlRow
                    id="network-proxy-username"
                    label={t("fields.username")}
                    description={t("fields.usernameDescription")}
                    badge={changed("username") ? unsaved : null}
                  >
                    <Input
                      id="network-proxy-username"
                      value={draft.username}
                      autoComplete="off"
                      spellCheck={false}
                      disabled={saving}
                      className={INPUT_CLASS}
                      onChange={(event) => update("username", event.target.value)}
                    />
                  </ControlRow>

                  <CollapsibleMotionContent open={draft.username.trim() !== ""} contentClassName="-mx-px px-px pb-px">
                    <ControlRow
                      id="network-proxy-password"
                      label={t("fields.password")}
                      description={
                        <>
                          {t("fields.passwordDescription")}
                          {proxy.hasPassword && password === undefined ? (
                            <>
                              {" "}
                              <button
                                type="button"
                                className="underline underline-offset-2 hover:text-foreground disabled:pointer-events-none"
                                disabled={saving}
                                onClick={() => setPassword("")}
                              >
                                {t("fields.removePassword")}
                              </button>
                            </>
                          ) : null}
                        </>
                      }
                      badge={password !== undefined ? unsaved : null}
                    >
                      <Input
                        id="network-proxy-password"
                        type="password"
                        value={password ?? ""}
                        placeholder={
                          proxy.hasPassword && password === undefined ? common("input.configuredPasswordPlaceholder") : undefined
                        }
                        autoComplete="new-password"
                        disabled={saving}
                        className={INPUT_CLASS}
                        // Clearing the input keeps the stored password; removing it is explicit.
                        onChange={(event) => setPassword(event.target.value === "" ? undefined : event.target.value)}
                      />
                    </ControlRow>
                  </CollapsibleMotionContent>

                  <Field>
                    <div className="min-w-0 space-y-2">
                      <div>
                        <div className="flex items-center gap-2">
                          <FieldLabel htmlFor="network-proxy-bypass">{t("fields.bypass")}</FieldLabel>
                          {changed("bypass") ? unsaved : null}
                        </div>
                        <FieldDescription className="text-[11px]">{t("fields.bypassDescription")}</FieldDescription>
                      </div>
                      <Textarea
                        id="network-proxy-bypass"
                        value={draft.bypass}
                        placeholder={"*.corp.example\n192.168.*"}
                        spellCheck={false}
                        disabled={saving}
                        aria-invalid={errorFor("bypass") ? true : undefined}
                        className="h-20 resize-none overflow-y-auto [field-sizing:fixed]"
                        onChange={(event) => update("bypass", event.target.value)}
                      />
                      {errorFor("bypass") ? <FieldError className="text-[11px]">{errorFor("bypass")}</FieldError> : null}
                    </div>
                  </Field>
                </SettingsFieldList>
              </SettingsFieldInset>
            </SettingsFieldItem>
          </CollapsibleMotionContent>

          <SettingsFieldItem index={managed ? 0 : 2}>
            <SettingsFieldRow
              title={t("effective.title")}
              description={t(`effective.sources.${proxy.effective.source}`)}
              controlClassName="md:w-auto xl:w-auto"
            >
              <EffectiveRoute effective={proxy.effective} directLabel={t("effective.direct")} />
            </SettingsFieldRow>
          </SettingsFieldItem>
        </SettingsFieldList>

        <Notices proxy={proxy} restart={restart} />
      </SettingsSection>
    </SettingsPage>
  );
}

/** One settings row: label, description and badge left, control right; an error under the control. */
function ControlRow({
  id,
  label,
  description,
  badge,
  error,
  children,
}: {
  id: string;
  label: string;
  description?: React.ReactNode;
  badge?: React.ReactNode;
  error?: string;
  children: React.ReactNode;
}) {
  return (
    <Field>
      <div className="flex min-w-0 flex-col gap-2 md:flex-row md:items-start md:gap-4 xl:gap-6">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <FieldLabel htmlFor={id}>{label}</FieldLabel>
            {badge}
          </div>
          {description ? <FieldDescription className="text-[11px]">{description}</FieldDescription> : null}
        </div>
        <div className="w-full min-w-0 space-y-1.5 md:w-44 md:shrink-0 xl:w-52">
          {children}
          {error ? <FieldError className="text-[11px] md:text-right">{error}</FieldError> : null}
        </div>
      </div>
    </Field>
  );
}

function EffectiveRoute({ effective, directLabel }: { effective: ProxyStatus["effective"]; directLabel: string }) {
  const { http, https } = effective;
  if (http === null && https === null) {
    return <p className="text-xs text-muted-foreground">{directLabel}</p>;
  }
  // One proxy for both schemes is the common case; show it once.
  const lines = http === https ? [http] : [http && `HTTP  ${http}`, https && `HTTPS ${https}`];
  return (
    <div className="space-y-0.5 text-left text-xs text-muted-foreground md:text-right">
      {lines
        .filter((line) => line !== null)
        .map((line) => (
          <p key={line} className="break-all">
            {line}
          </p>
        ))}
    </div>
  );
}

function Notices({ proxy, restart }: { proxy: ProxyStatus; restart: () => void }) {
  const t = useTranslations("desktopNetwork.notices");
  const { effective } = proxy;
  const restartReason = proxy.restartRequired ? t("restart") : proxy.systemChanged ? t("systemChanged") : null;
  const notes = [
    proxy.managedByPolicy ? t("managed") : null,
    effective.pacIgnored ? t("pacIgnored") : null,
    effective.webviewFollowsSystem ? t("webviewNotApplied") : null,
  ].filter((note) => note !== null);

  if (notes.length === 0 && restartReason === null) {
    return null;
  }
  return (
    <div className="space-y-1.5 text-[11px] leading-relaxed">
      {notes.map((note) => (
        <p key={note} className="flex gap-1.5 text-muted-foreground">
          <InfoIcon aria-hidden className="mt-0.5 size-3 shrink-0" />
          <span>{note}</span>
        </p>
      ))}
      {restartReason ? (
        <div className="flex flex-col gap-2 text-amber-700 sm:flex-row sm:items-center sm:justify-between dark:text-amber-300">
          <p className="flex gap-1.5">
            <InfoIcon aria-hidden className="mt-0.5 size-3 shrink-0" />
            <span>{restartReason}</span>
          </p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="h-7 shrink-0 rounded-md border-border/50 px-2.5 text-[12px] font-normal text-foreground shadow-none hover:bg-accent/40"
            onClick={restart}
          >
            <RotateCwIcon aria-hidden className="size-3.5 stroke-1" />
            {t("restartAction")}
          </Button>
        </div>
      ) : null}
    </div>
  );
}

/** Digits only; an empty field is "no port yet", and the shell rejects 0. */
function parsePort(value: string): number | null {
  const digits = value.replace(/\D/g, "").slice(0, 5);
  if (digits === "") {
    return null;
  }
  return Math.min(Number(digits), MAX_PORT);
}
