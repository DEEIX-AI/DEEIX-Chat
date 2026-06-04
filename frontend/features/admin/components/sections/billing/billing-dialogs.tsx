"use client";

import * as React from "react";
import { Plus, Trash2 } from "lucide-react";
import { motion } from "motion/react";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import {
  DIALOG_LAYOUT_TRANSITION,
  normalizePricingMode,
  parseIntValue,
  parsePrice,
  stringifyTieredPricing,
  type PlanFormState,
  type PricingMode,
  type PricingFormState,
  type TieredPricingTierForm,
} from "@/features/admin/model/billing-page";

type PricingJSONValue = Record<string, unknown>;

function pricingFormToJSON(form: PricingFormState): string {
  const pricingMode = normalizePricingMode(form.pricingMode);
  const payload = {
    platformModelName: form.platformModelName,
    currency: "USD",
    isFree: form.isFree,
    pricingMode,
    inputUSDPerMTokens: pricingMode === "token" ? parsePrice(form.input) : 0,
    cacheReadUSDPerMTokens: pricingMode === "token" ? parsePrice(form.cacheRead) : 0,
    cacheWriteUSDPerMTokens: pricingMode === "token" ? parsePrice(form.cacheWrite) : 0,
    outputUSDPerMTokens: pricingMode === "token" ? parsePrice(form.output) : 0,
    callUSDPerCall: pricingMode === "call" ? parsePrice(form.call) : 0,
    durationUSDPerSecond: pricingMode === "duration" ? parsePrice(form.duration) : 0,
    ...(pricingMode === "tiered" ? { tieredPricing: JSON.parse(stringifyTieredPricing(form.tieredTiers)) as unknown } : {}),
  };
  return JSON.stringify(payload, null, 2);
}

function readPricingNumber(payload: PricingJSONValue, key: string): string {
  const value = payload[key];
  if (value === undefined || value === null || value === "") {
    return "0";
  }
  const parsed = typeof value === "number" ? value : typeof value === "string" ? Number(value) : NaN;
  return Number.isFinite(parsed) && parsed >= 0 ? String(parsed) : "0";
}

function readPricingMode(value: unknown): PricingMode | null {
  return value === "token" || value === "call" || value === "duration" || value === "tiered" ? value : null;
}

function tieredTiersFromJSON(payload: PricingJSONValue): TieredPricingTierForm[] | null {
  const raw = typeof payload.tieredPricingJSON === "string" && payload.tieredPricingJSON.trim()
    ? JSON.parse(payload.tieredPricingJSON) as unknown
    : payload.tieredPricing;
  if (!raw || typeof raw !== "object" || Array.isArray(raw) || !Array.isArray((raw as { tiers?: unknown }).tiers)) {
    return null;
  }
  const tiers = (raw as { tiers: unknown[] }).tiers;
  if (tiers.length === 0) {
    return null;
  }
  return tiers.map((tier, index) => {
    const item = typeof tier === "object" && tier !== null && !Array.isArray(tier) ? tier as PricingJSONValue : {};
    return {
      id: `json-${index}-${String(item.upToTokens ?? 0)}`,
      upToKTokens: String(Math.ceil(Number(item.upToTokens ?? 0) / 1000) || 0),
      input: readPricingNumber(item, "inputUSDPerMTokens"),
      cacheRead: readPricingNumber(item, "cacheReadUSDPerMTokens"),
      cacheWrite: readPricingNumber(item, "cacheWriteUSDPerMTokens"),
      output: readPricingNumber(item, "outputUSDPerMTokens"),
    };
  });
}

function pricingFormFromJSON(current: PricingFormState, raw: string, messages: { root: string; model: string; mode: string; tiered: string }): PricingFormState {
  const parsed = JSON.parse(raw) as unknown;
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error(messages.root);
  }
  const payload = parsed as PricingJSONValue;
  const platformModelName = typeof payload.platformModelName === "string" ? payload.platformModelName.trim() : current.platformModelName;
  if (platformModelName !== current.platformModelName) {
    throw new Error(messages.model);
  }
  const pricingMode = readPricingMode(payload.pricingMode);
  if (!pricingMode) {
    throw new Error(messages.mode);
  }
  const next: PricingFormState = {
    ...current,
    pricingMode,
    isFree: typeof payload.isFree === "boolean" ? payload.isFree : current.isFree,
    input: pricingMode === "token" ? readPricingNumber(payload, "inputUSDPerMTokens") : "0",
    cacheRead: pricingMode === "token" ? readPricingNumber(payload, "cacheReadUSDPerMTokens") : "0",
    cacheWrite: pricingMode === "token" ? readPricingNumber(payload, "cacheWriteUSDPerMTokens") : "0",
    output: pricingMode === "token" ? readPricingNumber(payload, "outputUSDPerMTokens") : "0",
    call: pricingMode === "call" ? readPricingNumber(payload, "callUSDPerCall") : "0",
    duration: pricingMode === "duration" ? readPricingNumber(payload, "durationUSDPerSecond") : "0",
  };
  if (pricingMode === "tiered") {
    const tiers = tieredTiersFromJSON(payload);
    if (!tiers) {
      throw new Error(messages.tiered);
    }
    next.tieredTiers = tiers.map((tier) => ({ ...tier, upToKTokens: String(parseIntValue(tier.upToKTokens)) }));
  }
  return next;
}

type PlanBillingDialogProps = {
  open: boolean;
  saving: boolean;
  planForm: PlanFormState | null;
  setPlanForm: React.Dispatch<React.SetStateAction<PlanFormState | null>>;
  onOpenChange: (open: boolean) => void;
  onCancel: () => void;
  onSubmit: (event?: React.FormEvent<HTMLFormElement>) => void;
};

export function PlanBillingDialog({
  open,
  saving,
  planForm,
  setPlanForm,
  onOpenChange,
  onCancel,
  onSubmit,
}: PlanBillingDialogProps) {
  const t = useTranslations("adminBilling");
  const tActions = useTranslations("common.actions");
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[min(86vh,760px)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[560px]">
        <DialogHeader className="shrink-0 px-4 py-4">
          <DialogTitle>{t("plans.dialogTitle")}</DialogTitle>
          <DialogDescription>{t("plans.dialogDescription")}</DialogDescription>
        </DialogHeader>

        <motion.form layout transition={DIALOG_LAYOUT_TRANSITION} onSubmit={onSubmit} className="flex min-h-0 flex-1 flex-col">
          <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-2">
            {planForm ? (
              <>
                <div className="grid grid-cols-2 gap-5">
                  <div className="space-y-1">
                    <p className="text-xs text-muted-foreground">{t("plans.name")}</p>
                    <Input value={planForm.name} onChange={(event) => setPlanForm({ ...planForm, name: event.target.value })} required />
                  </div>
                  <div className="space-y-1">
                    <p className="text-xs text-muted-foreground">{t("plans.price")}</p>
                    <Input value={planForm.amount} type="number" min="0" step="0.01" onChange={(event) => setPlanForm({ ...planForm, amount: event.target.value })} />
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-5">
                  <div className="space-y-1">
                    <p className="text-xs text-muted-foreground">{t("plans.interval")}</p>
                    <Select value={planForm.billingInterval} onValueChange={(value) => setPlanForm({ ...planForm, billingInterval: value })}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="lifetime">{t("plans.intervals.lifetime")}</SelectItem>
                        <SelectItem value="month">{t("plans.intervals.month")}</SelectItem>
                        <SelectItem value="year">{t("plans.intervals.year")}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-1">
                    <p className="text-xs text-muted-foreground">{t("plans.periodCredit")}</p>
                    <Input value={planForm.periodCredit} type="number" min="0" step="0.01" onChange={(event) => setPlanForm({ ...planForm, periodCredit: event.target.value })} />
                  </div>
                  <div className="space-y-1">
                    <p className="text-xs text-muted-foreground">{t("plans.discount")}</p>
                    <Input value={planForm.discountPercent} type="number" min="0" max="100" step="1" onChange={(event) => setPlanForm({ ...planForm, discountPercent: event.target.value })} />
                  </div>
                  <div className="space-y-1">
                    <p className="text-xs text-muted-foreground">{t("plans.description")}</p>
                    <Input value={planForm.description} onChange={(event) => setPlanForm({ ...planForm, description: event.target.value })} />
                  </div>
                </div>
              </>
            ) : null}
          </div>

          <DialogFooter className="shrink-0 px-4 py-3">
            <Button type="button" variant="ghost" onClick={onCancel} disabled={saving}>
              {tActions("cancel")}
            </Button>
            <Button type="submit" disabled={saving || !planForm?.name.trim()}>
              {saving ? <SpinnerLabel>{tActions("saving")}</SpinnerLabel> : tActions("save")}
            </Button>
          </DialogFooter>
        </motion.form>
      </DialogContent>
    </Dialog>
  );
}

type PricingBillingDialogProps = {
  open: boolean;
  saving: boolean;
  form: PricingFormState | null;
  setForm: React.Dispatch<React.SetStateAction<PricingFormState | null>>;
  onOpenChange: (open: boolean) => void;
  onCancel: () => void;
  onSubmit: (event?: React.FormEvent<HTMLFormElement>) => void;
  onAddTier: () => void;
  onRemoveTier: (index: number) => void;
  onUpdateTier: (index: number, patch: Partial<TieredPricingTierForm>) => void;
};

export function PricingBillingDialog({
  open,
  saving,
  form,
  setForm,
  onOpenChange,
  onCancel,
  onSubmit,
  onAddTier,
  onRemoveTier,
  onUpdateTier,
}: PricingBillingDialogProps) {
  const t = useTranslations("adminBilling");
  const tActions = useTranslations("common.actions");
  const [editorMode, setEditorMode] = React.useState<"form" | "json">("form");
  const [jsonDraft, setJSONDraft] = React.useState("");
  const [jsonError, setJSONError] = React.useState("");
  const lastSyncedJSONRef = React.useRef("");

  React.useEffect(() => {
    if (!open || !form) {
      setEditorMode("form");
      setJSONDraft("");
      setJSONError("");
      lastSyncedJSONRef.current = "";
      return;
    }
    const nextJSON = pricingFormToJSON(form);
    if (editorMode !== "json") {
      setJSONDraft(nextJSON);
      lastSyncedJSONRef.current = nextJSON;
      setJSONError("");
    }
  }, [editorMode, form, open]);

  const handleJSONChange = React.useCallback((value: string) => {
    setJSONDraft(value);
    if (!form) {
      return;
    }
    try {
      const nextForm = pricingFormFromJSON(form, value, {
        root: t("modelPricing.jsonErrors.root"),
        model: t("modelPricing.jsonErrors.model"),
        mode: t("modelPricing.jsonErrors.mode"),
        tiered: t("modelPricing.jsonErrors.tiered"),
      });
      setJSONError("");
      lastSyncedJSONRef.current = value;
      setForm(nextForm);
    } catch (error) {
      setJSONError(error instanceof Error ? error.message : t("modelPricing.jsonErrors.invalid"));
    }
  }, [form, setForm, t]);

  const handleJSONPricingModeChange = React.useCallback((value: string) => {
    if (!form) {
      return;
    }
    const pricingMode = normalizePricingMode(value);
    const nextForm = { ...form, pricingMode };
    const nextJSON = pricingFormToJSON(nextForm);
    setForm(nextForm);
    setJSONDraft(nextJSON);
    setJSONError("");
    lastSyncedJSONRef.current = nextJSON;
  }, [form, setForm]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[min(86vh,760px)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[620px]">
        <DialogHeader className="shrink-0 gap-3 px-4 py-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0 space-y-1.5">
            <DialogTitle>{t("modelPricing.dialogTitle")}</DialogTitle>
            <DialogDescription>{t("modelPricing.dialogDescription")}</DialogDescription>
          </div>
          <Tabs value={editorMode} onValueChange={(value) => setEditorMode(value === "json" ? "json" : "form")} className="shrink-0">
            <TabsList>
              <TabsTrigger value="form">{t("modelPricing.formMode")}</TabsTrigger>
              <TabsTrigger value="json">{t("modelPricing.jsonMode")}</TabsTrigger>
            </TabsList>
          </Tabs>
        </DialogHeader>

        <motion.form layout transition={DIALOG_LAYOUT_TRANSITION} onSubmit={onSubmit} className="flex min-h-0 flex-1 flex-col">
          <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-2">
            {form ? (
              <>
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">{t("modelPricing.platformModel")}</p>
                <div className="flex h-8 items-center rounded-md border border-input/40 bg-muted/30 px-3 text-xs">
                  <span className="truncate">{form.platformModelName}</span>
                </div>
              </div>

              {editorMode === "form" ? (
                <>
                  <div className="grid grid-cols-2 gap-5">
                    <div className="space-y-1">
                      <p className="text-xs text-muted-foreground">{t("modelPricing.pricingMode")}</p>
                      <Select value={form.pricingMode} onValueChange={(value) => setForm({ ...form, pricingMode: normalizePricingMode(value) })}>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="token">{t("pricingModes.token")}</SelectItem>
                          <SelectItem value="call">{t("pricingModes.call")}</SelectItem>
                          <SelectItem value="duration">{t("pricingModes.duration")}</SelectItem>
                          <SelectItem value="tiered">{t("pricingModes.tiered")}</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-1">
                      <p className="text-xs text-muted-foreground">{t("modelPricing.freeModel")}</p>
                      <div className="flex h-8 items-center">
                        <Switch size="sm" checked={form.isFree} onCheckedChange={(checked) => setForm({ ...form, isFree: checked })} />
                      </div>
                    </div>
                  </div>

                  {form.pricingMode === "token" ? (
                    <>
                      <div className="grid grid-cols-2 gap-5">
                        <div className="space-y-1">
                          <p className="text-xs text-muted-foreground">{t("modelPricing.inputPerM")}</p>
                          <Input value={form.input} type="number" min="0" step="0.000001" onChange={(event) => setForm({ ...form, input: event.target.value })} />
                        </div>
                        <div className="space-y-1">
                          <p className="text-xs text-muted-foreground">{t("modelPricing.outputPerM")}</p>
                          <Input value={form.output} type="number" min="0" step="0.000001" onChange={(event) => setForm({ ...form, output: event.target.value })} />
                        </div>
                      </div>

                      <div className="grid grid-cols-2 gap-5">
                        <div className="space-y-1">
                          <p className="text-xs text-muted-foreground">{t("modelPricing.cacheReadPerM")}</p>
                          <Input value={form.cacheRead} type="number" min="0" step="0.000001" onChange={(event) => setForm({ ...form, cacheRead: event.target.value })} />
                        </div>
                        <div className="space-y-1">
                          <p className="text-xs text-muted-foreground">{t("modelPricing.cacheWritePerM")}</p>
                          <Input value={form.cacheWrite} type="number" min="0" step="0.000001" onChange={(event) => setForm({ ...form, cacheWrite: event.target.value })} />
                        </div>
                      </div>
                    </>
                  ) : null}

                  {form.pricingMode === "call" ? (
                    <div className="space-y-1">
                      <p className="text-xs text-muted-foreground">{t("modelPricing.perCall")}</p>
                      <Input value={form.call} type="number" min="0" step="0.000001" onChange={(event) => setForm({ ...form, call: event.target.value })} />
                    </div>
                  ) : null}

                  {form.pricingMode === "duration" ? (
                    <div className="space-y-1">
                      <p className="text-xs text-muted-foreground">{t("modelPricing.perSecond")}</p>
                      <Input value={form.duration} type="number" min="0" step="0.000001" onChange={(event) => setForm({ ...form, duration: event.target.value })} />
                    </div>
                  ) : null}

                  {form.pricingMode === "tiered" ? (
                <div className="space-y-2">
                  <div className="flex items-center justify-between gap-3">
                    <p className="text-xs text-muted-foreground">{t("modelPricing.tieredHint")}</p>
                    <Button type="button" variant="ghost" size="xs" onClick={onAddTier}>
                      <Plus className="size-3.5" />
                      {t("modelPricing.addTier")}
                    </Button>
                  </div>

                  <div className="space-y-2">
                    {form.tieredTiers.map((tier, index) => (
                      <div key={tier.id} className="grid gap-2 rounded-md border px-3 py-2">
                        <div className="flex items-center justify-between gap-2">
                          <span className="text-xs font-medium">{t("modelPricing.tierName", { index: index + 1 })}</span>
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon-xs"
                            className="text-muted-foreground"
                            disabled={form.tieredTiers.length <= 1}
                            onClick={() => onRemoveTier(index)}
                            aria-label={t("modelPricing.deleteTier")}
                          >
                            <Trash2 className="size-3.5" />
                          </Button>
                        </div>
                        <div className="grid grid-cols-5 gap-2">
                          <div className="space-y-1">
                            <p className="text-[11px] text-muted-foreground">{t("modelPricing.tokenLimitK")}</p>
                            <Input
                              value={tier.upToKTokens}
                              type="number"
                              min="0"
                              step="1"
                              onChange={(event) => onUpdateTier(index, { upToKTokens: event.target.value })}
                            />
                          </div>
                          <div className="space-y-1">
                            <p className="text-[11px] text-muted-foreground">{t("modelPricing.inputPerM")}</p>
                            <Input
                              value={tier.input}
                              type="number"
                              min="0"
                              step="0.000001"
                              onChange={(event) => onUpdateTier(index, { input: event.target.value })}
                            />
                          </div>
                          <div className="space-y-1">
                            <p className="text-[11px] text-muted-foreground">{t("modelPricing.outputPerM")}</p>
                            <Input
                              value={tier.output}
                              type="number"
                              min="0"
                              step="0.000001"
                              onChange={(event) => onUpdateTier(index, { output: event.target.value })}
                            />
                          </div>
                          <div className="space-y-1">
                            <p className="text-[11px] text-muted-foreground">{t("modelPricing.cacheReadPerM")}</p>
                            <Input
                              value={tier.cacheRead}
                              type="number"
                              min="0"
                              step="0.000001"
                              onChange={(event) => onUpdateTier(index, { cacheRead: event.target.value })}
                            />
                          </div>
                          <div className="space-y-1">
                            <p className="text-[11px] text-muted-foreground">{t("modelPricing.cacheWritePerM")}</p>
                            <Input
                              value={tier.cacheWrite}
                              type="number"
                              min="0"
                              step="0.000001"
                              onChange={(event) => onUpdateTier(index, { cacheWrite: event.target.value })}
                            />
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                  <p className="text-[11px] text-muted-foreground">{t("modelPricing.tierNote")}</p>
                </div>
                  ) : null}
                </>
              ) : (
                <div className="space-y-3">
                  <div className="space-y-1">
                    <p className="text-xs text-muted-foreground">{t("modelPricing.pricingMode")}</p>
                    <Select value={form.pricingMode} onValueChange={handleJSONPricingModeChange}>
                      <SelectTrigger className="w-full">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="token">{t("pricingModes.token")}</SelectItem>
                        <SelectItem value="call">{t("pricingModes.call")}</SelectItem>
                        <SelectItem value="duration">{t("pricingModes.duration")}</SelectItem>
                        <SelectItem value="tiered">{t("pricingModes.tiered")}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <Textarea
                    value={jsonDraft}
                    className="h-80 resize-none overflow-y-auto font-mono text-xs [field-sizing:fixed]"
                    spellCheck={false}
                    disabled={saving}
                    onChange={(event) => handleJSONChange(event.target.value)}
                  />
                  <p className={jsonError ? "text-[11px] text-destructive" : "text-[11px] text-muted-foreground"}>
                    {jsonError || t("modelPricing.jsonHint")}
                  </p>
                </div>
              )}
              </>
            ) : null}
          </div>

          <DialogFooter className="shrink-0 px-4 py-3">
            <Button type="button" variant="ghost" onClick={onCancel} disabled={saving}>
              {tActions("cancel")}
            </Button>
            <Button type="submit" disabled={saving || Boolean(jsonError)}>
              {saving ? <SpinnerLabel>{tActions("saving")}</SpinnerLabel> : tActions("save")}
            </Button>
          </DialogFooter>
        </motion.form>
      </DialogContent>
    </Dialog>
  );
}
