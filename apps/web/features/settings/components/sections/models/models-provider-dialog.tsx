"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { InputGroup, InputGroupInput } from "@/components/ui/input-group";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { SpinnerLabel } from "@/components/ui/spinner";
import { findModelProviderPreset } from "@/entities/model";
import { modelProviderProtocolLabel } from "@/features/settings/model/model-provider-protocols";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import type { CreatePersonalProviderPayload, PersonalProviderDTO } from "@/shared/api/personal-providers-types";
import { ModelsIconPicker } from "./models-icon-picker";
import { ModelsSelectDialog, ModelsSelectField } from "./models-select-dialog";

export type ModelProviderDraft = {
  name: string;
  icon: string;
  protocol: string;
  baseURL: string;
  apiKey: string;
};

/** Small catalogs are preselected; large ones (aggregators list hundreds) start empty so the user picks deliberately. */
const PRESELECT_ALL_MAX_MODELS = 12;

const FIELD_LABEL_CLASS = "text-xs font-normal text-muted-foreground";

/**
 * Add a provider: address, protocol, key, then fetch the model list and pick
 * models. A recognised address fills in the protocol and icon once, as long as
 * the user has not chosen them. The import screen opens it with the confirmed
 * address locked and its own name.
 */
export function ModelsProviderDialog({
  open,
  onOpenChange,
  protocols,
  initialDraft,
  source = "manual",
  lockEndpoint = false,
  onProbe,
  onCreate,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  protocols: string[];
  initialDraft?: Partial<ModelProviderDraft>;
  source?: "manual" | "link";
  /** Imported links fix the address so the host the user confirmed cannot change. */
  lockEndpoint?: boolean;
  onProbe: (payload: { protocol: string; baseURL: string; apiKey: string }) => Promise<string[]>;
  onCreate: (payload: CreatePersonalProviderPayload) => Promise<PersonalProviderDTO>;
}) {
  const t = useTranslations("settings.modelsPage.dialog");
  const pageT = useTranslations("settings.modelsPage");
  const commonT = useTranslations("common");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [draft, setDraft] = React.useState<ModelProviderDraft>({ name: "", icon: "", protocol: "openai_chat_completions", baseURL: "", apiKey: "" });
  // The protocol follows a recognised address until the user picks one. The icon is never
  // auto-filled into the draft: empty means "automatic", so it keeps following the address.
  const [protocolTouched, setProtocolTouched] = React.useState(false);
  const [models, setModels] = React.useState<string[] | null>(null);
  const [selected, setSelected] = React.useState<Set<string>>(() => new Set());
  const [selectOpen, setSelectOpen] = React.useState(false);
  const [probing, setProbing] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [probeError, setProbeError] = React.useState("");
  const [error, setError] = React.useState("");

  React.useEffect(() => {
    if (!open) return;
    const first = initialDraft ?? {};
    setDraft({ name: first.name ?? "", icon: first.icon ?? "", protocol: first.protocol ?? "openai_chat_completions", baseURL: first.baseURL ?? "", apiKey: first.apiKey ?? "" });
    setProtocolTouched(Boolean(first.protocol));
    setModels(null);
    setSelected(new Set());
    setSelectOpen(false);
    setProbeError("");
    setError("");
  }, [open, initialDraft]);

  const preset = findModelProviderPreset(draft.baseURL);
  const autoIcon = preset?.icon ?? "";

  const resetProbe = () => {
    setModels(null);
    setSelected(new Set());
    setProbeError("");
  };

  const setBaseURL = (value: string) => {
    const matched = findModelProviderPreset(value);
    setDraft((current) => ({
      ...current,
      baseURL: value,
      protocol: protocolTouched ? current.protocol : (matched?.protocol ?? current.protocol),
    }));
    resetProbe();
  };

  const setField = (key: "name" | "apiKey", value: string) => {
    setDraft((current) => ({ ...current, [key]: value }));
    // A different key may list different models.
    if (key === "apiKey") resetProbe();
  };

  const busy = probing || saving;
  const canProbe = Boolean(draft.baseURL.trim() && draft.apiKey.trim() && draft.protocol) && !busy;

  const handleProbe = async () => {
    setProbing(true);
    setProbeError("");
    try {
      const fetched = await onProbe({ protocol: draft.protocol, baseURL: draft.baseURL.trim(), apiKey: draft.apiKey.trim() });
      setModels(fetched);
      setSelected(new Set(fetched.length <= PRESELECT_ALL_MAX_MODELS ? fetched : []));
    } catch (failure) {
      setModels(null);
      setProbeError(resolveErrorMessage(failure, t("probeFailed")));
    } finally {
      setProbing(false);
    }
  };

  // The list is fetched on first open, so the picker shows its own loading and error states.
  const openPicker = () => {
    setSelectOpen(true);
    if (models === null && !probing) void handleProbe();
  };

  const handleSave = async () => {
    if (!models) return;
    setSaving(true);
    setError("");
    try {
      await onCreate({
        name: draft.name.trim() || preset?.name || undefined,
        icon: draft.icon || undefined,
        protocol: draft.protocol,
        baseURL: draft.baseURL.trim(),
        apiKey: draft.apiKey.trim(),
        models: models.filter((model) => selected.has(model)),
        source,
      });
      onOpenChange(false);
    } catch (saveError) {
      setError(resolveErrorMessage(saveError, t("saveFailed")));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(next) => !saving && onOpenChange(next)}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>{source === "link" ? t("importTitle") : t("title")}</DialogTitle>
          <DialogDescription>{t("description")}</DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="min-w-0 space-y-1">
            <Label className={FIELD_LABEL_CLASS} htmlFor="model-provider-name">{t("name")}</Label>
            <InputGroup>
              <ModelsIconPicker
                value={draft.icon}
                autoIcon={autoIcon}
                label={draft.name || preset?.name || t("namePlaceholder")}
                disabled={busy}
                onChange={(icon) => setDraft((current) => ({ ...current, icon }))}
              />
              <InputGroupInput
                id="model-provider-name"
                value={draft.name}
                placeholder={preset?.name ?? t("namePlaceholder")}
                maxLength={64}
                disabled={busy}
                onChange={(event) => setField("name", event.target.value)}
              />
            </InputGroup>
          </div>

          <div className="min-w-0 space-y-1">
            <Label className={FIELD_LABEL_CLASS} htmlFor="model-provider-base-url">{t("baseURL")}</Label>
            <Input
              id="model-provider-base-url"
              value={draft.baseURL}
              placeholder="https://api.example.com/v1"
              autoComplete="off"
              spellCheck={false}
              readOnly={lockEndpoint}
              disabled={busy}
              onChange={(event) => setBaseURL(event.target.value)}
            />
            <p className="text-[11px] text-muted-foreground">{t("baseURLHint")}</p>
          </div>

          <div className="min-w-0 space-y-1">
            <Label className={FIELD_LABEL_CLASS}>{t("protocol")}</Label>
            <Select
              value={draft.protocol}
              disabled={busy}
              onValueChange={(value) => {
                setProtocolTouched(true);
                setDraft((current) => ({ ...current, protocol: value }));
                resetProbe();
              }}
            >
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {protocols.map((protocol) => (
                  <SelectItem key={protocol} value={protocol}>
                    {modelProviderProtocolLabel(protocol, (key) => pageT(key))}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="min-w-0 space-y-1">
            <Label className={FIELD_LABEL_CLASS} htmlFor="model-provider-key">{t("apiKey")}</Label>
            <Input
              id="model-provider-key"
              type="password"
              value={draft.apiKey}
              placeholder={t("apiKeyPlaceholder")}
              autoComplete="off"
              spellCheck={false}
              disabled={busy}
              onChange={(event) => setField("apiKey", event.target.value)}
            />
          </div>

          <div className="min-w-0 space-y-1">
            <Label className={FIELD_LABEL_CLASS} htmlFor="model-provider-models">{t("models")}</Label>
            <ModelsSelectField
              id="model-provider-models"
              models={models ?? []}
              selected={selected}
              disabled={busy || (models === null && !canProbe)}
              onOpen={openPicker}
            />
          </div>

          {error ? (
            <p className="text-xs text-destructive" role="alert">
              {error}
            </p>
          ) : null}
          <p className="text-[11px] leading-relaxed text-muted-foreground">{t("privacyNote")}</p>
        </div>

        <DialogFooter>
          <Button type="button" variant="ghost" disabled={saving} onClick={() => onOpenChange(false)}>
            {commonT("actions.cancel")}
          </Button>
          <Button type="button" disabled={models === null || selected.size === 0 || busy} onClick={() => void handleSave()}>
            {saving ? <SpinnerLabel>{commonT("actions.saving")}</SpinnerLabel> : commonT("actions.save")}
          </Button>
        </DialogFooter>
      </DialogContent>

      <ModelsSelectDialog
        open={selectOpen}
        onOpenChange={setSelectOpen}
        models={models ?? []}
        selected={selected}
        onConfirm={setSelected}
        loading={probing}
        error={probeError}
        onRefresh={() => void handleProbe()}
      />
    </Dialog>
  );
}
