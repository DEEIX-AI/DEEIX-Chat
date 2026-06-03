"use client";

import { useState, useEffect, useMemo, useRef } from "react";
import { Check, ChevronDownIcon, CircleHelp } from "lucide-react";
import { toast } from "sonner";
import { useLocale, useTranslations } from "next-intl";

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
  ComboboxTrigger,
  ComboboxValue,
} from "@/components/ui/combobox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { getModelOptionPolicy } from "@/shared/api/settings";
import {
  createAdminLLMModel,
  listAdminLLMModelUpstreamSources,
  updateAdminLLMModel,
} from "@/features/admin/api";
import { LobeHubIcon } from "@/shared/components/lobehub-icon";
import { KNOWN_VENDOR_OPTIONS, resolveLobeHubIconURL, resolveModelIdentity } from "@/shared/lib/model-identity";
import type {
  AdminLLMModelDTO,
  AdminLLMModelUpstreamSourceDTO,
  AdminLLMModelVendor,
  AdminLLMStatus,
  UpdateAdminLLMModelRequest,
} from "@/features/admin/api/llm.types";

import {
  MODEL_STATUS_OPTIONS,
  MODEL_KIND_OPTIONS,
  formatDateTime,
  resolveErrorMessage,
  resolveValue,
} from "@/features/admin/types/llm";
import {
  parseKindsJSON,
  stringifyKinds,
} from "@/shared/model/llm-schema";
import { parseProtocolsJSON } from "@/features/chat/model/chat-adapter-options";
import { JsonCodeEditor } from "@/shared/components/json-code-editor";
import {
  imageStreamEnabledFromCapabilities,
  MODEL_CAPABILITIES_PLACEHOLDER,
  ModelCapabilitiesGuideButton,
  ModelCapabilitiesQuickConfig,
  setImageStreamEnabledInCapabilities,
} from "@/features/admin/components/sections/models/model-capabilities-config";
import type { NativeToolDefinition } from "@/shared/lib/model-option-policy";

// ---------------------------------------------------------------------------
// Form state
// ---------------------------------------------------------------------------

type FormState = {
  platformModelName: string;
  vendor: AdminLLMModelVendor | "";
  kinds: string[];
  icon: string;
  capabilitiesJSON: string;
  systemPrompt: string;
  status: AdminLLMStatus;
  description: string;
};

type VendorOption = {
  value: AdminLLMModelVendor;
  label: string;
  iconUrl: string | null;
};

const UNKNOWN_VENDOR = "unknown";

const MODEL_SHEET_VENDOR_OPTIONS: VendorOption[] = [
  { value: UNKNOWN_VENDOR, label: "Unknown", iconUrl: null },
  ...KNOWN_VENDOR_OPTIONS.map(({ value, label }) => {
    const identity = resolveModelIdentity({ vendor: value });
    return {
      value,
      label,
      iconUrl: resolveLobeHubIconURL(identity.vendorIcon),
    };
  }),
];

const IMAGE_MEDIA_PROTOCOLS = new Set([
  "openai_image_generations",
  "openai_image_edits",
  "google_image_generation",
  "xai_image",
  "xai_image_edits",
]);

function buildInitialState(target: AdminLLMModelDTO | null): FormState {
  if (!target) {
    return {
      platformModelName: "",
      vendor: normalizeSupportedVendor(UNKNOWN_VENDOR),
      kinds: [],
      icon: "",
      capabilitiesJSON: "",
      systemPrompt: "",
      status: "active",
      description: "",
    };
  }
  let kinds: string[] = [];
  kinds = parseKindsJSON(target.kindsJSON);
  return {
    platformModelName: target.platformModelName,
    vendor: normalizeSupportedVendor(target.vendor),
    kinds,
    icon: target.icon ?? "",
    capabilitiesJSON: normalizeCapabilitiesJSON(target.capabilitiesJSON),
    systemPrompt: target.systemPrompt ?? "",
    status: target.status,
    description: target.description ?? "",
  };
}

function normalizeVendorValue(value: string): string {
  return value.trim().toLowerCase();
}

function normalizeSupportedVendor(value: string | null | undefined): AdminLLMModelVendor {
  const normalized = normalizeVendorValue(value ?? "");
  if (MODEL_SHEET_VENDOR_OPTIONS.some((item) => item.value === normalized)) {
    return normalized;
  }
  const identity = resolveModelIdentity({ vendor: normalized });
  return MODEL_SHEET_VENDOR_OPTIONS.some((item) => item.value === identity.vendorKey)
    ? identity.vendorKey
    : UNKNOWN_VENDOR;
}

function normalizeCapabilitiesJSON(value: string | null | undefined): string {
  const trimmed = value?.trim() ?? "";
  return trimmed === "{}" ? "" : trimmed;
}

function VendorOptionIcon({
  iconUrl,
  label,
  unknown = false,
}: {
  iconUrl?: string | null;
  label: string;
  unknown?: boolean;
}) {
  return (
    <span className="inline-flex size-4 shrink-0 items-center justify-center self-center text-foreground">
      {iconUrl ? (
        <LobeHubIcon iconUrl={iconUrl} label={label} />
      ) : unknown ? (
        <CircleHelp className="size-4.5" strokeWidth={1.5} />
      ) : (
        <span className="size-2 rounded-full bg-muted-foreground/35" aria-hidden="true" />
      )}
      <span className="sr-only">{label}</span>
    </span>
  );
}

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

type ModelSheetProps = {
  open: boolean;
  mode: "create" | "edit";
  target: AdminLLMModelDTO | null;
  onClose: () => void;
  onSuccess: () => void;
};

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function ModelSheet({ open, mode, target, onClose, onSuccess }: ModelSheetProps) {
  const t = useTranslations("adminModels");
  const commonT = useTranslations("common");
  const locale = useLocale();
  const [form, setForm] = useState<FormState>(() => buildInitialState(target));
  const [pending, setPending] = useState(false);
  const [expandedSections, setExpandedSections] = useState<string[]>([]);
  const sheetContentRef = useRef<HTMLDivElement | null>(null);
  const [nativeTools, setNativeTools] = useState<NativeToolDefinition[]>([]);
  // Upstream sources for accordion
  const [sources, setSources] = useState<AdminLLMModelUpstreamSourceDTO[]>([]);
  const [sourcesLoading, setSourcesLoading] = useState(false);

  function setField<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  function toggleKind(kind: string) {
    setForm((prev) => ({
      ...prev,
      kinds: prev.kinds.includes(kind)
        ? prev.kinds.filter((k) => k !== kind)
        : [...prev.kinds, kind],
    }));
  }

  const selectedKindLabel = form.kinds
    .map((kind) =>
      MODEL_KIND_OPTIONS.some((option) => option.value === kind)
        ? t(`kinds.${kind}`)
        : kind,
    )
    .join(", ");
  const vendorOptions = MODEL_SHEET_VENDOR_OPTIONS.map((item) => ({
    ...item,
    label: item.value === UNKNOWN_VENDOR ? t("sheet.unknownVendor") : item.label,
  }));
  const routeProtocols = useMemo(
    () => Array.from(new Set([
      ...parseProtocolsJSON(target?.protocolsJSON ?? ""),
      ...sources.map((source) => source.protocol.trim()).filter(Boolean),
    ])),
    [sources, target?.protocolsJSON],
  );
  const imageStreamEnabled = imageStreamEnabledFromCapabilities(form.capabilitiesJSON);
  const showImageStreamControl = routeProtocols.some((protocol) => IMAGE_MEDIA_PROTOCOLS.has(protocol.trim()));

  function updateImageStreamEnabled(enabled: boolean) {
    const nextValue = setImageStreamEnabledInCapabilities(form.capabilitiesJSON, enabled);
    if (nextValue === null) {
      toast.error(t("sheet.capabilitiesQuick.invalidJSON"));
      return;
    }
    setField("capabilitiesJSON", nextValue);
  }

  function handleClose() {
    onClose();
  }

  // -------------------------------------------------------------------------
  // Load when sheet opens
  // -------------------------------------------------------------------------

  useEffect(() => {
    if (!open) {
      setNativeTools([]);
      return;
    }
    let cancelled = false;
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) {
          return;
        }
        const policy = await getModelOptionPolicy(token);
        if (!cancelled) {
          setNativeTools(policy.nativeTools);
        }
      } catch {
        if (!cancelled) {
          setNativeTools([]);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open]);

  useEffect(() => {
    if (!open || !target) {
      setSources([]);
      setForm(buildInitialState(null));
      return;
    }
    setForm(buildInitialState(target));
    setExpandedSections([]);

    setSourcesLoading(true);
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) return;
        const data = await listAdminLLMModelUpstreamSources(token, target.id, {
          page: 1,
          pageSize: 100,
        });
        setSources(data.results);
      } catch {
        setSources([]);
      } finally {
        setSourcesLoading(false);
      }
    })();
  }, [open, target]);

  // -------------------------------------------------------------------------
  // Submit
  // -------------------------------------------------------------------------

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (mode === "edit" && !target) return;
    setPending(true);
    try {
      const token = await resolveAccessToken();
      const kindsJson =
        form.kinds.length > 0 ? stringifyKinds(form.kinds) : undefined;

      if (mode === "create") {
        const data = await createAdminLLMModel(token, {
          platformModelName: form.platformModelName.trim(),
          vendor: form.vendor || undefined,
          kindsJSON: kindsJson,
          icon: form.icon.trim() || undefined,
          capabilitiesJSON: normalizeCapabilitiesJSON(form.capabilitiesJSON) || undefined,
          systemPrompt: form.systemPrompt.trim() || undefined,
          status: form.status,
          description: form.description.trim() || undefined,
        });
        setForm(buildInitialState(data.model));
        handleClose();
        onSuccess();
        toast.success(t("toast.modelCreated"));
        return;
      }

      if (!target) return;
      const payload: UpdateAdminLLMModelRequest = {
        platformModelName: form.platformModelName.trim() || undefined,
        vendor: form.vendor || undefined,
        kindsJSON: kindsJson,
        icon: form.icon.trim() || undefined,
        capabilitiesJSON: normalizeCapabilitiesJSON(form.capabilitiesJSON) || undefined,
        systemPrompt: form.systemPrompt.trim(),
        status: form.status,
        description: form.description.trim() || undefined,
      };
      await updateAdminLLMModel(token, target.id, payload);

      handleClose();
      onSuccess();
      toast.success(t("toast.modelUpdated"));
    } catch (err) {
      toast.error(mode === "create" ? t("toast.createFailed") : t("toast.updateFailed"), { description: resolveErrorMessage(err) });
    } finally {
      setPending(false);
    }
  }

  // -------------------------------------------------------------------------
  // Icon preview
  // -------------------------------------------------------------------------

  const resolvedIdentity = resolveModelIdentity({
    code: form.platformModelName,
    vendor: form.vendor,
    icon: form.icon,
  });
  const iconPreviewUrl = resolveLobeHubIconURL(form.icon || resolvedIdentity.modelIcon);
  const selectedVendorOption =
    vendorOptions.find((item) => normalizeVendorValue(item.value) === normalizeVendorValue(form.vendor)) ??
    vendorOptions[0];

  // -------------------------------------------------------------------------
  // Render
  // -------------------------------------------------------------------------

  return (
    <Sheet open={open} onOpenChange={(nextOpen) => !nextOpen && !pending && handleClose()}>
      <SheetContent
        ref={sheetContentRef}
        className="flex flex-col sm:max-w-[460px]"
      >
        <SheetHeader className="px-4 pb-4">
          <SheetTitle>{mode === "create" ? t("sheet.createTitle") : t("sheet.editTitle")}</SheetTitle>
        </SheetHeader>

        <form onSubmit={handleSubmit} className="flex flex-col flex-1 min-h-0">
          <div className="overflow-y-auto flex-1 px-6 space-y-4">

            <div>
              <Label htmlFor="model-platform-name">{t("platformModel")}</Label>
              <Input
                id="model-platform-name"
                value={form.platformModelName}
                placeholder="claude-sonnet-4.5"
                onChange={(e) => setField("platformModelName", e.target.value)}
                disabled={pending}
              />
            </div>

            <div>
              <Label htmlFor="model-vendor">{t("sheet.vendor")}</Label>
              <Combobox
                id="model-vendor"
                items={vendorOptions}
                value={selectedVendorOption}
                onValueChange={(item) => setField("vendor", item?.value ?? UNKNOWN_VENDOR)}
                itemToStringLabel={(item) => item?.label ?? ""}
                isItemEqualToValue={(item, selected) => item.value === selected.value}
                disabled={pending}
              >
                <ComboboxTrigger
                  render={
                    <Button
                      type="button"
                      variant="outline"
                      className="w-full justify-between border-input/40 bg-transparent px-3 py-1 font-normal hover:bg-transparent focus-visible:border-ring/60 focus-visible:ring-[1px] focus-visible:ring-ring/40 [&_[data-slot=combobox-trigger-icon]]:size-4 [&_[data-slot=combobox-trigger-icon]]:opacity-50"
                      disabled={pending}
                    >
                      <span className="flex min-w-0 flex-1 items-center justify-start gap-2">
                        <VendorOptionIcon
                          iconUrl={selectedVendorOption?.iconUrl}
                          label={selectedVendorOption?.label ?? ""}
                          unknown={selectedVendorOption?.value === UNKNOWN_VENDOR}
                        />
                        <span className="min-w-0 truncate text-left leading-5">
                          <ComboboxValue />
                        </span>
                      </span>
                    </Button>
                  }
                />
                <ComboboxContent
                  align="start"
                  className="min-w-[320px]"
                  portalContainer={sheetContentRef}
                >
                  <ComboboxInput placeholder={t("sheet.vendorSearchPlaceholder")} showTrigger={false} showClear={false} disabled={pending} />
                  <ComboboxEmpty>{t("sheet.noMatchedVendors")}</ComboboxEmpty>
                  <ComboboxList>
                    {(item: VendorOption) => (
                      <ComboboxItem key={item.value} value={item} className="text-left">
                        <VendorOptionIcon
                          iconUrl={item.iconUrl}
                          label={item.label}
                          unknown={item.value === UNKNOWN_VENDOR}
                        />
                        <span className="min-w-0 flex-1 truncate leading-5">{item.label}</span>
                      </ComboboxItem>
                    )}
                  </ComboboxList>
                </ComboboxContent>
              </Combobox>
            </div>

            <div>
              <Label>{t("fields.status")}</Label>
              <Select
                value={form.status}
                onValueChange={(v) => setField("status", v as AdminLLMStatus)}
                disabled={pending}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {MODEL_STATUS_OPTIONS.map((s) => (
                    <SelectItem key={s} value={s}>
                      {t(`status.${s}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div>
              <Label>{t("sheet.kind")}</Label>
              <Popover>
                <PopoverTrigger asChild>
                  <Button
                    type="button"
                    variant="outline"
                    role="combobox"
                    disabled={pending}
                    className="w-full justify-between border-input/40 bg-transparent px-3 py-1 font-normal hover:bg-transparent focus-visible:border-ring/60 focus-visible:ring-[1px] focus-visible:ring-ring/40"
                  >
                    <span className={`min-w-0 flex-1 truncate text-left ${selectedKindLabel ? "" : "text-muted-foreground"}`}>
                      {selectedKindLabel || t("sheet.selectKind")}
                    </span>
                    <ChevronDownIcon className="size-3 shrink-0 text-muted-foreground opacity-50" />
                  </Button>
                </PopoverTrigger>
                <PopoverContent align="start" className="w-48 p-1">
                  {MODEL_KIND_OPTIONS.map(({ value }) => (
                    <button
                      key={value}
                      type="button"
                      onClick={() => toggleKind(value)}
                      className="relative flex w-full items-center rounded-sm py-1.5 pr-8 pl-2 text-xs font-normal hover:bg-accent"
                    >
                      <span className="min-w-0 flex-1 truncate text-left">{t(`kinds.${value}`)}</span>
                      <Check
                        className={`absolute right-2 size-4 shrink-0 text-muted-foreground ${
                          form.kinds.includes(value) ? "opacity-100" : "opacity-0"
                        }`}
                      />
                    </button>
                  ))}
                </PopoverContent>
              </Popover>
            </div>

            <div>
              <Label htmlFor="model-icon">{t("sheet.icon")}</Label>
              <div className="flex items-center gap-3">
                <Input
                  id="model-icon"
                  value={form.icon}
                  placeholder="openai"
                  className="font-mono"
                  onChange={(e) => setField("icon", e.target.value)}
                  disabled={pending}
                />
                {iconPreviewUrl ? (
                  <LobeHubIcon key={iconPreviewUrl} iconUrl={iconPreviewUrl} label={form.icon} size={32} />
                ) : (
                  <div className="size-8 shrink-0" />
                )}
              </div>
              {form.icon.trim() === "" ? (
                <p className="mt-1 text-[11px] text-muted-foreground">
                  {t("sheet.iconAutoDescription", { vendor: resolvedIdentity.vendorLabel })}
                </p>
              ) : null}
            </div>

            <div>
              <Label htmlFor="model-desc">{t("sheet.description")}</Label>
              <Textarea
                id="model-desc"
                value={form.description}
                placeholder={t("sheet.descriptionPlaceholder")}
                className="h-20 resize-none overflow-y-auto [field-sizing:fixed]"
                onChange={(e) => setField("description", e.target.value)}
                disabled={pending}
              />
            </div>

            <div>
              <Label htmlFor="model-system-prompt">{t("sheet.systemPrompt")}</Label>
              <Textarea
                id="model-system-prompt"
                value={form.systemPrompt}
                placeholder={t("sheet.systemPromptPlaceholder")}
                className="h-28 resize-none overflow-y-auto [field-sizing:fixed]"
                onChange={(e) => setField("systemPrompt", e.target.value)}
                disabled={pending}
                maxLength={20000}
              />
              <p className="mt-1 text-[11px] text-muted-foreground">
                {t("sheet.systemPromptDescription")}
              </p>
            </div>

            <Accordion
              type="multiple"
              value={expandedSections}
              onValueChange={setExpandedSections}
              className="space-y-2"
            >
              <AccordionItem value="capabilities" className="px-1">
                <AccordionTrigger className="py-1.5 text-xs text-muted-foreground hover:no-underline">
                  {t("sheet.capabilitiesJSON")}
                </AccordionTrigger>
                <AccordionContent className="pt-3">
                  <div className="mb-2 flex min-w-0 items-center justify-between gap-2">
                    <p className="min-w-0 text-xs text-muted-foreground">
                      {t("sheet.capabilitiesDescription")}
                    </p>
                    <div className="flex shrink-0 items-center gap-1.5">
                      <ModelCapabilitiesGuideButton t={t} />
                    </div>
                  </div>
                  {showImageStreamControl ? (
                    <label
                      htmlFor="model-image-stream-enabled"
                      className="mb-2 flex min-w-0 items-center gap-2 px-1 py-1"
                    >
                      <Checkbox
                        id="model-image-stream-enabled"
                        checked={imageStreamEnabled}
                        disabled={pending}
                        onCheckedChange={(checked) => updateImageStreamEnabled(checked === true)}
                      />
                      <span className="min-w-0 truncate text-xs font-medium text-foreground">
                        {t("sheet.imageStreamEnabled")}
                      </span>
                    </label>
                  ) : null}
                  <JsonCodeEditor
                    id="model-capabilities-json"
                    value={form.capabilitiesJSON}
                    placeholder={MODEL_CAPABILITIES_PLACEHOLDER}
                    height={220}
                    onChange={(nextValue) => setField("capabilitiesJSON", nextValue)}
                    disabled={pending}
                    actions={(
                      <ModelCapabilitiesQuickConfig
                        value={form.capabilitiesJSON}
                        disabled={pending}
                        nativeTools={nativeTools}
                        routeProtocols={routeProtocols}
                        t={t}
                        commonT={commonT}
                        onApply={(nextValue) => setField("capabilitiesJSON", nextValue)}
                      />
                    )}
                  />
                </AccordionContent>
              </AccordionItem>

              <AccordionItem value="sources" className="px-1">
                <AccordionTrigger className="py-1.5 text-xs text-muted-foreground hover:no-underline">
                  {t("sheet.upstreamSources", { count: sourcesLoading ? "..." : sources.length })}
                </AccordionTrigger>
                <AccordionContent className="pt-3 space-y-2">
                  {sourcesLoading ? (
                    <div className="h-3 w-24 animate-pulse rounded-sm bg-muted/70" aria-hidden="true" />
                  ) : sources.length === 0 ? (
                    <p className="text-xs text-muted-foreground">{t("sources.empty")}</p>
                  ) : (
                    sources.map((src) => (
                      <div
                        key={src.id}
                        className="rounded-md border px-3 py-2 text-xs space-y-1.5"
                      >
                        <div className="flex items-center justify-between gap-2">
                          <div className="flex items-center gap-1.5 min-w-0">
                            <span className="font-medium truncate">
                              {resolveValue(src.upstreamName)}
                            </span>
                            <span className="text-muted-foreground">→</span>
                            <span className="font-mono text-muted-foreground truncate">
                              {resolveValue(src.upstreamModelName)}
                            </span>
                          </div>
                          <div className="flex items-center gap-1 shrink-0">
                            {src.circuitOpen ? (
                              <Badge variant="outline" className="border-destructive/60 text-destructive text-[10px]">
                                {t("status.circuitOpen")}
                              </Badge>
                            ) : src.status === "inactive" ? (
                              <Badge variant="outline" className="text-muted-foreground text-[10px]">
                                {t("status.inactive")}
                              </Badge>
                            ) : (
                              <Badge variant="secondary" className="text-[10px]">
                                {t("status.active")}
                              </Badge>
                            )}
                          </div>
                        </div>
                      </div>
                    ))
                  )}
                </AccordionContent>
              </AccordionItem>

              {target && (
                <AccordionItem value="meta" className="px-1">
                  <AccordionTrigger className="py-1.5 text-xs text-muted-foreground hover:no-underline">
                    {t("sheet.metadata")}
                  </AccordionTrigger>
                  <AccordionContent className="pt-3 space-y-2 text-xs">
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">ID</span>
                      <span className="font-mono">{target.id}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">{t("sheet.createdAt")}</span>
                      <span>{formatDateTime(target.createdAt, locale)}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">{t("sheet.updatedAt")}</span>
                      <span>{formatDateTime(target.updatedAt, locale)}</span>
                    </div>
                  </AccordionContent>
                </AccordionItem>
              )}
            </Accordion>
          </div>

          <SheetFooter className="flex flex-row justify-end px-4 py-3 gap-2">
            <Button
              type="button"
              variant="ghost"
              onClick={handleClose}
              disabled={pending}
            >
              {commonT("actions.cancel")}
            </Button>
            <Button type="submit" disabled={pending}>
              {pending ? <SpinnerLabel>{t("sheet.saving")}</SpinnerLabel> : commonT("actions.save")}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  );
}
