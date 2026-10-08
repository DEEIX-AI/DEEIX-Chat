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
import { resolveModelProviderIcon } from "@/entities/model";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import type { PersonalProviderDTO, UpdatePersonalProviderPayload } from "@/shared/api/personal-providers-types";
import { ModelsIconPicker } from "./models-icon-picker";
import { ModelsSelectDialog, ModelsSelectField } from "./models-select-dialog";

const FIELD_LABEL_CLASS = "text-xs font-normal text-muted-foreground";

/**
 * Edit a saved provider: rename it, replace its key, choose enabled models.
 * The live model list loads with the saved key on open; if that fails the
 * currently enabled models stay editable. The saved key is never shown.
 */
export function ModelsEditDialog({
  provider,
  onOpenChange,
  onLoadModels,
  onSave,
}: {
  provider: PersonalProviderDTO | null;
  onOpenChange: (open: boolean) => void;
  onLoadModels: (id: string) => Promise<string[]>;
  onSave: (id: string, payload: UpdatePersonalProviderPayload) => Promise<boolean>;
}) {
  const t = useTranslations("settings.modelsPage.editDialog");
  const commonT = useTranslations("common");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [name, setName] = React.useState("");
  const [icon, setIcon] = React.useState("");
  const [apiKey, setApiKey] = React.useState("");
  const [available, setAvailable] = React.useState<string[] | null>(null);
  const [selected, setSelected] = React.useState<Set<string>>(() => new Set());
  const [loadingModels, setLoadingModels] = React.useState(false);
  const [loadError, setLoadError] = React.useState("");
  const [selectOpen, setSelectOpen] = React.useState(false);
  const [saving, setSaving] = React.useState(false);

  // Resets the form and loads the live model list for the provider being opened. An effect event
  // reads the latest provider and callbacks without rerunning on every parent render.
  const openProvider = React.useEffectEvent((isCancelled: () => boolean) => {
    if (!provider) return;
    setName(provider.name);
    setIcon(provider.icon);
    setApiKey("");
    setAvailable(null);
    setSelected(new Set(provider.models));
    setLoadError("");
    setLoadingModels(true);
    onLoadModels(provider.id)
      .then((models) => {
        if (!isCancelled()) setAvailable(models);
      })
      .catch((error: unknown) => {
        if (!isCancelled()) setLoadError(resolveErrorMessage(error, t("loadFailed")));
      })
      .finally(() => {
        if (!isCancelled()) setLoadingModels(false);
      });
  });

  // Reset only when a different provider opens; list refreshes must not wipe the user's edits.
  const providerID = provider?.id ?? "";
  React.useEffect(() => {
    if (!providerID) return;
    let cancelled = false;
    openProvider(() => cancelled);
    return () => {
      cancelled = true;
    };
  }, [providerID]);

  // Keep enabled models editable even if the provider stopped listing them, so they can be unticked.
  const listed = React.useMemo(() => {
    if (!provider) return [];
    if (!available) return provider.models;
    const missing = provider.models.filter((model) => !available.includes(model));
    return [...available, ...missing];
  }, [available, provider]);

  const handleSave = async () => {
    if (!provider) return;
    const payload: UpdatePersonalProviderPayload = {};
    if (name.trim() && name.trim() !== provider.name) payload.name = name.trim();
    if (icon !== provider.icon) payload.icon = icon;
    if (apiKey.trim()) payload.apiKey = apiKey.trim();
    const models = listed.filter((model) => selected.has(model));
    if (models.join("\n") !== provider.models.join("\n")) payload.models = models;
    if (Object.keys(payload).length === 0) {
      onOpenChange(false);
      return;
    }
    setSaving(true);
    const saved = await onSave(provider.id, payload);
    setSaving(false);
    if (saved) onOpenChange(false);
  };

  return (
    <Dialog open={provider !== null} onOpenChange={(next) => !saving && onOpenChange(next)}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
          <DialogDescription className="sr-only">{provider?.host ?? ""}</DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="min-w-0 space-y-1">
            <Label className={FIELD_LABEL_CLASS} htmlFor="model-provider-edit-name">{t("name")}</Label>
            <InputGroup>
              <ModelsIconPicker
                value={icon}
                autoIcon={provider ? resolveModelProviderIcon("", provider.baseURL) : ""}
                label={name}
                disabled={saving}
                onChange={setIcon}
              />
              <InputGroupInput id="model-provider-edit-name" value={name} maxLength={64} disabled={saving} onChange={(event) => setName(event.target.value)} />
            </InputGroup>
          </div>
          <div className="min-w-0 space-y-1">
            <Label className={FIELD_LABEL_CLASS} htmlFor="model-provider-edit-base-url">{t("baseURL")}</Label>
            <Input
              id="model-provider-edit-base-url"
              className="text-muted-foreground"
              value={provider?.baseURL ?? ""}
              readOnly
              aria-describedby="model-provider-edit-base-url-hint"
            />
            <p id="model-provider-edit-base-url-hint" className="text-[11px] text-muted-foreground">{t("endpointLocked")}</p>
          </div>
          <div className="min-w-0 space-y-1">
            <Label className={FIELD_LABEL_CLASS} htmlFor="model-provider-edit-key">{t("rotateKey")}</Label>
            <Input
              id="model-provider-edit-key"
              type="password"
              value={apiKey}
              placeholder={t("rotateKeyPlaceholder")}
              autoComplete="off"
              spellCheck={false}
              disabled={saving}
              onChange={(event) => setApiKey(event.target.value)}
            />
          </div>
          <div className="min-w-0 space-y-1">
            <Label className={FIELD_LABEL_CLASS} htmlFor="model-provider-edit-models">{t("models")}</Label>
            <ModelsSelectField
              id="model-provider-edit-models"
              models={listed}
              selected={selected}
              disabled={saving}
              onOpen={() => setSelectOpen(true)}
            />
            {loadError ? <p className="text-[11px] text-amber-700 dark:text-amber-300">{t("loadFailedHint", { reason: loadError })}</p> : null}
          </div>
        </div>

        <DialogFooter>
          <Button type="button" variant="ghost" disabled={saving} onClick={() => onOpenChange(false)}>
            {commonT("actions.cancel")}
          </Button>
          <Button type="button" disabled={saving || loadingModels || selected.size === 0} onClick={() => void handleSave()}>
            {saving ? <SpinnerLabel>{commonT("actions.saving")}</SpinnerLabel> : commonT("actions.save")}
          </Button>
        </DialogFooter>
      </DialogContent>

      <ModelsSelectDialog
        open={selectOpen}
        onOpenChange={setSelectOpen}
        models={listed}
        selected={selected}
        onConfirm={setSelected}
        loading={loadingModels}
      />
    </Dialog>
  );
}
