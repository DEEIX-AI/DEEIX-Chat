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
import { SpinnerLabel } from "@/components/ui/spinner";
import { findModelProviderPreset } from "@/entities/model";
import { modelSelectionPayload } from "@/features/settings/model/model-protocol-choices";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import type {
  CreatePersonalProviderPayload,
  PersonalProviderAvailableModelDTO,
  PersonalProviderDTO,
} from "@/shared/api/personal-providers-types";
import { ModelsIconPicker } from "./models-icon-picker";
import { type ModelOption, ModelsSelectDialog, ModelsSelectField } from "./models-select-dialog";

export type ModelProviderDraft = {
  name: string;
  icon: string;
  /** Set by an import link only; otherwise the protocol follows the address. */
  protocol: string;
  baseURL: string;
  apiKey: string;
};

/** How the model list is fetched when the address is not a known provider: the format relays speak. */
const DEFAULT_PROVIDER_PROTOCOL = "openai_chat_completions";

/** Small catalogs are preselected; large ones (aggregators list hundreds) start empty so the user picks deliberately. */
const PRESELECT_ALL_MAX_MODELS = 12;

const FIELD_LABEL_CLASS = "text-xs font-normal text-muted-foreground";

/**
 * Add a provider: address and key, then fetch the model list and pick models,
 * each with its own protocol. The provider's protocol only decides how the list
 * is fetched, so it is not asked for: a known address brings its own, anything
 * else is treated as OpenAI-compatible. The import screen opens the dialog with
 * the confirmed address locked and its own name.
 */
export function ModelsProviderDialog({
  open,
  onOpenChange,
  modelProtocols,
  initialDraft,
  source = "manual",
  lockEndpoint = false,
  onProbe,
  onCreate,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Protocols a single model may run on, image and video ones included. */
  modelProtocols: string[];
  initialDraft?: Partial<ModelProviderDraft>;
  source?: "manual" | "link";
  /** Imported links fix the address so the host the user confirmed cannot change. */
  lockEndpoint?: boolean;
  onProbe: (payload: { protocol: string; baseURL: string; apiKey: string }) => Promise<PersonalProviderAvailableModelDTO[]>;
  onCreate: (payload: CreatePersonalProviderPayload) => Promise<PersonalProviderDTO>;
}) {
  const t = useTranslations("settings.modelsPage.dialog");
  const commonT = useTranslations("common");
  const resolveErrorMessage = useLocalizedErrorMessage();
  // The icon is never auto-filled into the draft: empty means "automatic", so it keeps following the address.
  const [draft, setDraft] = React.useState<ModelProviderDraft>({ name: "", icon: "", protocol: "", baseURL: "", apiKey: "" });
  const [models, setModels] = React.useState<ModelOption[] | null>(null);
  const [selected, setSelected] = React.useState<Map<string, string[]>>(() => new Map());
  const [selectOpen, setSelectOpen] = React.useState(false);
  const [probing, setProbing] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [probeError, setProbeError] = React.useState("");
  const [error, setError] = React.useState("");

  React.useEffect(() => {
    if (!open) return;
    const first = initialDraft ?? {};
    setDraft({ name: first.name ?? "", icon: first.icon ?? "", protocol: first.protocol ?? "", baseURL: first.baseURL ?? "", apiKey: first.apiKey ?? "" });
    setModels(null);
    setSelected(new Map());
    setSelectOpen(false);
    setProbeError("");
    setError("");
  }, [open, initialDraft]);

  const preset = findModelProviderPreset(draft.baseURL);
  const autoIcon = preset?.icon ?? "";
  const protocol = draft.protocol || preset?.protocol || DEFAULT_PROVIDER_PROTOCOL;

  const resetProbe = () => {
    setModels(null);
    setSelected(new Map());
    setProbeError("");
  };

  const setBaseURL = (value: string) => {
    setDraft((current) => ({ ...current, baseURL: value }));
    resetProbe();
  };

  const setField = (key: "name" | "apiKey", value: string) => {
    setDraft((current) => ({ ...current, [key]: value }));
    // A different key may list different models.
    if (key === "apiKey") resetProbe();
  };

  const busy = probing || saving;
  const canProbe = Boolean(draft.baseURL.trim() && draft.apiKey.trim()) && !busy;

  const handleProbe = async () => {
    setProbing(true);
    setProbeError("");
    try {
      const fetched = await onProbe({ protocol, baseURL: draft.baseURL.trim(), apiKey: draft.apiKey.trim() });
      setModels(fetched);
      setSelected(new Map(
        (fetched.length <= PRESELECT_ALL_MAX_MODELS ? fetched : []).map((model) => [model.name, [...model.suggestedProtocols]]),
      ));
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
        protocol,
        baseURL: draft.baseURL.trim(),
        apiKey: draft.apiKey.trim(),
        models: modelSelectionPayload(models, selected, modelProtocols),
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
        protocols={modelProtocols}
        onConfirm={setSelected}
        loading={probing}
        error={probeError}
        onRefresh={() => void handleProbe()}
      />
    </Dialog>
  );
}
