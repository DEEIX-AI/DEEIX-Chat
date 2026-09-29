"use client";

import { useState, useEffect, useMemo, useRef } from "react";
import { Check, ChevronDownIcon, CircleHelp, Plus, ShieldAlert, Trash2 } from "lucide-react";
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
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { FeatureGate, useFeature } from "@/shared/capabilities";
import { resolveAutomaticModelContextWindow } from "@/features/admin/model/openrouter-model-catalog";
import { useAdminModelsCapabilityContext } from "@/features/admin/hooks/use-admin-models-capability-context";
import { useAdminModelsOpenrouterCatalog } from "@/features/admin/hooks/use-admin-models-openrouter-catalog";
import { useAdminModelsPermissionGroups } from "@/features/admin/hooks/use-admin-models-permission-groups";
import { useAdminModelsSheetSources } from "@/features/admin/hooks/use-admin-models-sheet-sources";
import {
  type AdminModelSheetForm,
  FOLLOW_VENDOR_GROUP,
  isSameContextWindowTarget,
  useAdminModelsSheetSubmit,
} from "@/features/admin/hooks/use-admin-models-sheet-submit";
import {
  ADMIN_LLM_ADAPTERS,
  isAdminLLMAdapter,
  isAdminLLMModelAccessScope,
  isAdminLLMModelCbPolicyMode,
  isAdminLLMStatus,
} from "@/features/admin/model/admin-unions";
import {
  ModelIcon,
  parseKindsJSON,
  parseProtocolsJSON,
  resolveModelIconURL,
  resolveModelIdentity,
} from "@/entities/model";
import type {
  AdminLLMModelDisplayGroupDTO,
  AdminLLMModelDTO,
  AdminLLMModelVendor,
  AdminLLMModelVendorDTO,
  AdminLLMAdapter,
} from "@/features/admin/api/llm-types";

import {
  ADAPTER_LABELS,
  MODEL_STATUS_OPTIONS,
  MODEL_KIND_OPTIONS,
  formatDateTime,
  resolveValue,
} from "@/features/admin/types/llm";
import { JsonCodeEditor } from "@/shared/components/json-code-editor";
import {
  imageStreamEnabledFromCapabilities,
  MODEL_CAPABILITIES_PLACEHOLDER,
  ModelCapabilitiesGuideButton,
  ModelCapabilitiesQuickConfig,
  normalizeModelCapabilitiesJSON,
  setImageStreamEnabledInCapabilities,
} from "@/features/admin/components/sections/models/models-capabilities-config";
import {
  modelContextWindowOverride,
  setModelContextWindowInCapabilities,
} from "@/features/admin/model/model-context-window";
import {
  DEFAULT_MODEL_SOURCE_BIND_DRAFT,
  createModelSourceBindDraftRow,
  modelSourceBindDraftHasSelection,
  type ModelSourceBindDraftRow,
} from "@/features/admin/model/models-source-binding";
import { PermissionGroupSelector } from "@/features/admin/components/shared/permission-group-selector";
import { ModelContextWindowField } from "@/features/admin/components/sections/models/models-context-window-field";
import { ModelIconField } from "@/features/admin/components/sections/models/models-icon-field";

// ---------------------------------------------------------------------------
// Form state
// ---------------------------------------------------------------------------

type FormState = AdminModelSheetForm;

type VendorOption = {
  value: AdminLLMModelVendor;
  label: string;
  iconUrl: string | null;
};

const UNKNOWN_VENDOR = "unknown";

const IMAGE_MEDIA_PROTOCOLS = new Set([
  "openai_image_generations",
  "openai_image_edits",
  "google_image_generation",
  "openrouter_images",
  "xai_image",
  "xai_image_edits",
]);

function formatCircuitUntil(until: string, locale: string): string {
  const raw = until.trim();
  if (!raw) {
    return "-";
  }
  const timestamp = Number(raw);
  const date = Number.isFinite(timestamp) && timestamp > 0 ? new Date(timestamp * 1000) : new Date(raw);
  if (Number.isNaN(date.getTime())) {
    return raw;
  }
  return new Intl.DateTimeFormat(locale, {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function buildInitialState(target: AdminLLMModelDTO | null): FormState {
  if (!target) {
    return {
      platformModelName: "",
      vendor: UNKNOWN_VENDOR,
      displayGroupID: FOLLOW_VENDOR_GROUP,
      kinds: [],
      icon: "",
      capabilitiesJSON: "",
      systemPrompt: "",
      accessScope: "public",
      status: "active",
      description: "",
      cbPolicyMode: "default",
      cbFailureThreshold: "0",
      cbDurationMin: "0",
      cbWindowMin: "0",
    };
  }
  let kinds: string[] = [];
  kinds = parseKindsJSON(target.kindsJSON);
  return {
    platformModelName: target.platformModelName,
    vendor: target.vendor,
    displayGroupID: target.displayGroupID ? String(target.displayGroupID) : FOLLOW_VENDOR_GROUP,
    kinds,
    icon: target.icon ?? "",
    capabilitiesJSON: normalizeCapabilitiesText(target.capabilitiesJSON),
    systemPrompt: target.systemPrompt ?? "",
    accessScope: target.accessScope === "internal" ? "internal" : "public",
    status: target.status,
    description: target.description ?? "",
    cbPolicyMode: target.cbPolicyMode === "enforced" ? "enforced" : "default",
    cbFailureThreshold: String(target.cbFailureThreshold ?? 0),
    cbDurationMin: String(target.cbDurationMin ?? 0),
    cbWindowMin: String(target.cbWindowMin ?? 0),
  };
}

function normalizeVendorValue(value: string): string {
  return value.trim().toLowerCase();
}

function normalizeCapabilitiesText(value: string | null | undefined): string {
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
        <ModelIcon iconUrl={iconUrl} label={label} />
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
  models: AdminLLMModelDTO[];
  vendors: AdminLLMModelVendorDTO[];
  displayGroups: AdminLLMModelDisplayGroupDTO[];
  onClose: () => void;
  onSuccess: () => void;
};

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function ModelSheet({ open, mode, target, models, vendors, displayGroups, onClose, onSuccess }: ModelSheetProps) {
  const t = useTranslations("adminModels");
  const commonT = useTranslations("common");
  const locale = useLocale();
  const [form, setForm] = useState<FormState>(() => buildInitialState(target));
  const [iconUploading, setIconUploading] = useState(false);
  const [expandedSections, setExpandedSections] = useState<string[]>([]);
  const [showCapabilitiesJSONAdvanced, setShowCapabilitiesJSONAdvanced] = useState(false);
  const sheetContentRef = useRef<HTMLDivElement | null>(null);
  const { nativeTools, capabilitySourceModels, contextWindowFallbackTokens } = useAdminModelsCapabilityContext({ open, models });
  const { openRouterCatalog, loadOpenRouterCatalog } = useAdminModelsOpenrouterCatalog(open);
  const [bindRows, setBindRows] = useState<ModelSourceBindDraftRow[]>(() => [createModelSourceBindDraftRow()]);
  const {
    sources,
    sourcesLoading,
    upstreams,
    upstreamsLoading,
    upstreamModelsByID,
    upstreamModelsLoadingByID,
    loadUpstreamModels,
  } = useAdminModelsSheetSources({ open, mode, target });
  const multiUser = useFeature("multiUser");
  const {
    permissionGroups,
    manualPermissionGroupIDs,
    setManualPermissionGroupIDs,
    matchedPermissionGroupIDs,
    effectivePermissionGroupIDs,
    permissionGroupsUnassigned,
    permissionGroupsLoading,
    saveModelPermissionGroups,
  } = useAdminModelsPermissionGroups({ open, mode, target, multiUser });
  const { pending, submit } = useAdminModelsSheetSubmit({
    mode,
    target,
    openRouterCatalog,
    loadOpenRouterCatalog,
    manualPermissionGroupIDs,
    saveModelPermissionGroups,
    onClose,
    onSuccess,
  });

  function setField<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  const contextWindowOverride = useMemo(
    () => modelContextWindowOverride(form.capabilitiesJSON),
    [form.capabilitiesJSON],
  );
  const effectiveContextWindow = useMemo(() => {
    if (contextWindowOverride !== null) {
      return contextWindowOverride;
    }
    const targetUnchanged = isSameContextWindowTarget(target, form.platformModelName, form.vendor);
    if (targetUnchanged && target?.contextWindow) {
      return target.contextWindow;
    }
    if (openRouterCatalog.status === "loaded") {
      const resolved = resolveAutomaticModelContextWindow(
        openRouterCatalog.items,
        form.platformModelName,
        form.vendor,
      );
      if (resolved !== null) {
        return resolved;
      }
    }
    return contextWindowFallbackTokens;
  }, [
    contextWindowFallbackTokens,
    contextWindowOverride,
    form.platformModelName,
    form.vendor,
    openRouterCatalog,
    target,
  ]);
  function updateContextWindowOverride(value: number | null): boolean {
    const nextValue = setModelContextWindowInCapabilities(form.capabilitiesJSON, value);
    if (nextValue === null) {
      toast.error(t("sheet.capabilitiesQuick.invalidJSON"));
      return false;
    }
    setField("capabilitiesJSON", nextValue);
    return true;
  }

  function toggleKind(kind: string) {
    setForm((prev) => ({
      ...prev,
      kinds: prev.kinds.includes(kind)
        ? prev.kinds.filter((k) => k !== kind)
        : [...prev.kinds, kind],
    }));
  }

  function addBindRow() {
    setBindRows((current) => [createModelSourceBindDraftRow(), ...current]);
  }

  function removeBindRow(rowID: string) {
    setBindRows((current) => {
      if (current.length <= 1) {
        return [createModelSourceBindDraftRow()];
      }
      return current.filter((row) => row.id !== rowID);
    });
  }

  function handleBindRowUpstreamChange(rowID: string, upstreamID: string) {
    setBindRows((current) =>
      current.map((row) =>
        row.id === rowID
          ? {
              ...row,
              draft: {
                ...DEFAULT_MODEL_SOURCE_BIND_DRAFT,
                upstreamID,
              },
            }
          : row,
      ),
    );
    void loadUpstreamModels(upstreamID);
  }

  function handleBindRowModelChange(rowID: string, upstreamModelID: string) {
    const targetRow = bindRows.find((row) => row.id === rowID);
    const selected = targetRow
      ? (upstreamModelsByID[targetRow.draft.upstreamID] ?? []).find(
          (item) => String(item.id) === upstreamModelID,
        )
      : undefined;
    if (selected?.suggestedProtocol === "xai_video") {
      setForm((current) => ({
        ...current,
        kinds: Array.from(new Set([...current.kinds, "video_gen", "video_extension"])),
      }));
    }
    setBindRows((current) => {
      const currentTargetRow = current.find((row) => row.id === rowID);
      if (!currentTargetRow) {
        return current;
      }
      const protocols: AdminLLMAdapter[] = selected?.suggestedProtocol === "xai_video"
        ? ["xai_video", "xai_video_extensions"]
        : selected?.suggestedProtocol
          ? [selected.suggestedProtocol]
          : [];
      const existingProtocols = new Set(
        current
          .filter(
            (row) =>
              row.id !== rowID &&
              row.draft.upstreamID === currentTargetRow.draft.upstreamID &&
              row.draft.upstreamModelID === upstreamModelID,
          )
          .map((row) => row.draft.protocol)
          .filter(Boolean),
      );
      const missingProtocols = protocols.filter((protocol) => !existingProtocols.has(protocol));
      const primaryProtocol = missingProtocols[0] ?? "";
      const companionRows = missingProtocols.slice(1).map((protocol) =>
        createModelSourceBindDraftRow({
          ...currentTargetRow.draft,
          upstreamModelID,
          protocol,
        }),
      );

      return current.flatMap((row) =>
        row.id === rowID
          ? [
              {
                ...row,
                draft: {
                  ...row.draft,
                  upstreamModelID,
                  protocol: primaryProtocol,
                },
              },
              ...companionRows,
            ]
          : [row],
      );
    });
  }

  function setBindRowField<K extends keyof ModelSourceBindDraftRow["draft"]>(
    rowID: string,
    key: K,
    value: ModelSourceBindDraftRow["draft"][K],
  ) {
    setBindRows((current) =>
      current.map((row) =>
        row.id === rowID
          ? {
              ...row,
              draft: {
                ...row.draft,
                [key]: value,
              },
            }
          : row,
      ),
    );
  }

  const selectedKindLabel = form.kinds
    .map((kind) =>
      MODEL_KIND_OPTIONS.some((option) => option.value === kind)
        ? t(`kinds.${kind}`)
        : kind,
    )
    .join(", ");
  const vendorOptions = vendors.map((item) => ({
    value: item.key,
    label: item.name,
    iconUrl: resolveModelIconURL(item.icon),
  }));
  const routeProtocols = useMemo(
    () => Array.from(new Set([
      ...parseProtocolsJSON(target?.protocolsJSON ?? ""),
      ...sources.map((source) => source.protocol.trim()).filter(Boolean),
      ...bindRows.map((row) => row.draft.protocol).filter(Boolean),
    ])),
    [bindRows, sources, target?.protocolsJSON],
  );
  function getBindProtocolOptions(row: ModelSourceBindDraftRow): AdminLLMAdapter[] {
    const upstreamModels = upstreamModelsByID[row.draft.upstreamID] ?? [];
    const selectedUpstreamModel = upstreamModels.find((item) => String(item.id) === row.draft.upstreamModelID);
    const values = new Set<AdminLLMAdapter>(ADMIN_LLM_ADAPTERS);
    if (selectedUpstreamModel?.suggestedProtocol) {
      values.add(selectedUpstreamModel.suggestedProtocol);
    }
    if (selectedUpstreamModel?.protocol) {
      values.add(selectedUpstreamModel.protocol);
    }
    return Array.from(values).sort((a, b) => {
      const labelA = ADAPTER_LABELS[a] ?? a;
      const labelB = ADAPTER_LABELS[b] ?? b;
      return labelA.localeCompare(labelB);
    });
  }
  const imageStreamEnabled = imageStreamEnabledFromCapabilities(form.capabilitiesJSON);
  const showImageStreamControl = routeProtocols.some((protocol) => IMAGE_MEDIA_PROTOCOLS.has(protocol.trim()));
  const showPermissionGroupUnassigned =
    !permissionGroupsLoading && permissionGroupsUnassigned && effectivePermissionGroupIDs.length === 0;

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

  // Route data resets live in useAdminModelsSheetSources; this resets the form UI.
  useEffect(() => {
    if (!open) {
      setForm(buildInitialState(null));
      setBindRows([createModelSourceBindDraftRow()]);
      setExpandedSections([]);
      setShowCapabilitiesJSONAdvanced(false);
      return;
    }

    if (mode === "create" || !target) {
      setForm(buildInitialState(null));
      setBindRows([createModelSourceBindDraftRow()]);
      setExpandedSections(["capabilities", "sources"]);
      setShowCapabilitiesJSONAdvanced(false);
      return;
    }

    setForm(buildInitialState(target));
    setBindRows([createModelSourceBindDraftRow()]);
    setExpandedSections(["capabilities"]);
    setShowCapabilitiesJSONAdvanced(false);
  }, [mode, open, target]);

  // -------------------------------------------------------------------------
  // Submit
  // -------------------------------------------------------------------------

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (iconUploading) return;
    await submit({
      form,
      contextWindowOverride,
      bindRows,
      normalizeCapabilities: (capabilitiesJSON) => normalizeModelCapabilitiesJSON(capabilitiesJSON, nativeTools, routeProtocols),
      onCreated: (model) => {
        setForm(buildInitialState(model));
        setBindRows([createModelSourceBindDraftRow()]);
      },
    });
  }

  // -------------------------------------------------------------------------
  // Icon preview
  // -------------------------------------------------------------------------

  const resolvedIdentity = resolveModelIdentity({
    code: form.platformModelName,
    vendor: form.vendor,
    icon: form.icon,
  });
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
        className="flex flex-col gap-0 sm:max-w-[460px]"
      >
        <SheetHeader className="shrink-0 px-4 py-4">
          <SheetTitle>{mode === "create" ? t("sheet.createTitle") : t("sheet.editTitle")}</SheetTitle>
        </SheetHeader>

        <form onSubmit={handleSubmit} className="flex flex-col flex-1 min-h-0">
          <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-2">

            <div className="space-y-4">
              <div className="min-w-0 space-y-1">
                <Label className="text-xs font-normal text-muted-foreground" htmlFor="model-platform-name">{t("platformModel")}</Label>
                <Input
                  id="model-platform-name"
                  value={form.platformModelName}
                  placeholder="claude-sonnet-4.5"
                  onChange={(e) => setField("platformModelName", e.target.value)}
                  disabled={pending}
                />
              </div>

              <div className="min-w-0 space-y-1">
                <Label className="text-xs font-normal text-muted-foreground" htmlFor="model-vendor">{t("sheet.vendor")}</Label>
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

              <div className="min-w-0 space-y-1">
                <Label className="text-xs font-normal text-muted-foreground" htmlFor="model-display-group">
                  {t("sheet.displayGroup")}
                </Label>
                <Select
                  value={form.displayGroupID}
                  onValueChange={(value) => setField("displayGroupID", value)}
                  disabled={pending}
                >
                  <SelectTrigger id="model-display-group">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={FOLLOW_VENDOR_GROUP}>
                      {t("sheet.followVendor", { vendor: selectedVendorOption?.label ?? form.vendor })}
                    </SelectItem>
                    {displayGroups.map((group) => (
                      <SelectItem key={group.id} value={String(group.id)}>
                        {group.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-xs leading-5 text-muted-foreground">{t("sheet.displayGroupDescription")}</p>
              </div>

              <div className="min-w-0 space-y-1">
                <Label className="text-xs font-normal text-muted-foreground">{t("fields.status")}</Label>
                <Select
                  value={form.status}
                  onValueChange={(v) => {
                    if (isAdminLLMStatus(v)) setField("status", v);
                  }}
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

              <div className="min-w-0 space-y-1">
                <Label className="text-xs font-normal text-muted-foreground">{t("sheet.kind")}</Label>
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

              <div className="min-w-0 space-y-1">
                <Label className="text-xs font-normal text-muted-foreground" htmlFor="model-icon">{t("sheet.icon")}</Label>
                <ModelIconField
                  id="model-icon"
                  value={form.icon}
                  placeholder="openai"
                  help={t("sheet.iconHelp")}
                  disabled={pending}
                  onChange={(value) => setField("icon", value)}
                  onUploadingChange={setIconUploading}
                />
                {form.icon.trim() === "" ? (
                  <p className="text-[11px] text-muted-foreground">
                    {t("sheet.iconAutoDescription", { vendor: resolvedIdentity.vendorLabel })}
                  </p>
                ) : null}
              </div>
            </div>

            <Accordion
              type="multiple"
              value={expandedSections}
              onValueChange={setExpandedSections}
              className="border-y border-border/60"
            >
              <AccordionItem value="capabilities" className="border-border/60">
                <AccordionTrigger className="h-11 items-center py-0 text-xs font-normal text-muted-foreground hover:text-foreground hover:no-underline data-[state=open]:font-medium data-[state=open]:text-foreground [&_.accordion-trigger-icon]:translate-y-0">
                  {t("sheet.capabilities")}
                </AccordionTrigger>
                <AccordionContent className="space-y-3 pb-4 pt-0">
                  <p className="text-xs leading-5 text-muted-foreground">
                    {t("sheet.capabilitiesDescription")}
                  </p>
                  <ModelContextWindowField
                    value={contextWindowOverride}
                    effectiveValue={effectiveContextWindow}
                    disabled={pending}
                    onChange={updateContextWindowOverride}
                  />
                  {showImageStreamControl ? (
                    <div className="pb-1">
                      <label
                        htmlFor="model-image-stream-enabled"
                        className="flex min-w-0 items-center gap-2 text-xs font-normal text-muted-foreground"
                      >
                        <Checkbox
                          id="model-image-stream-enabled"
                          checked={imageStreamEnabled}
                          disabled={pending}
                          className="size-3.5"
                          onCheckedChange={(checked) => updateImageStreamEnabled(checked === true)}
                        />
                        <span className="min-w-0 truncate">
                          {t("sheet.imageStreamEnabled")}
                        </span>
                      </label>
                    </div>
                  ) : null}
                  <div className="grid min-w-0 grid-cols-2 gap-2">
                    <ModelCapabilitiesQuickConfig
                      value={form.capabilitiesJSON}
                      disabled={pending}
                      presetModels={capabilitySourceModels}
                      currentModelID={target?.id ?? null}
                      nativeTools={nativeTools}
                      routeProtocols={routeProtocols}
                      t={t}
                      commonT={commonT}
                      triggerVariant="secondary"
                      triggerClassName="h-8 w-full justify-start px-2 text-xs font-normal shadow-none"
                      triggerLabel={t("sheet.capabilitiesVisualButton")}
                      onApply={(nextValue) => setField("capabilitiesJSON", nextValue)}
                    />
                    <Button
                      type="button"
                      variant="secondary"
                      size="sm"
                      className="h-8 justify-between px-2 text-xs font-normal shadow-none"
                      onClick={() => setShowCapabilitiesJSONAdvanced((prev) => !prev)}
                    >
                      {t("sheet.capabilitiesAdvancedJSON")}
                      <ChevronDownIcon
                        className={cn(
                          "size-3 transition-transform",
                          showCapabilitiesJSONAdvanced && "rotate-180",
                        )}
                      />
                    </Button>
                  </div>
                  {showCapabilitiesJSONAdvanced ? (
                    <div className="space-y-1.5 pt-1">
                      <div className="flex min-w-0 items-center justify-between gap-2">
                        <p className="truncate text-[11px] text-muted-foreground">
                          {t("sheet.capabilitiesJSON")}
                        </p>
                        <ModelCapabilitiesGuideButton t={t} />
                      </div>
                      <JsonCodeEditor
                        id="model-capabilities-json"
                        value={form.capabilitiesJSON}
                        placeholder={MODEL_CAPABILITIES_PLACEHOLDER}
                        height={220}
                        onChange={(nextValue) => setField("capabilitiesJSON", nextValue)}
                        disabled={pending}
                      />
                    </div>
                  ) : null}
                </AccordionContent>
              </AccordionItem>

              <AccordionItem value="circuit-breaker" className="border-border/60">
                <AccordionTrigger className="h-11 items-center py-0 text-xs font-normal text-muted-foreground hover:text-foreground hover:no-underline data-[state=open]:font-medium data-[state=open]:text-foreground [&_.accordion-trigger-icon]:translate-y-0">
                  {t("sheet.circuitBreak")}
                </AccordionTrigger>
                <AccordionContent className="space-y-3 pb-4 pt-0">
                  <p className="text-xs leading-5 text-muted-foreground">
                    {t("sheet.circuitBreakDescription")}
                  </p>
                  <div className="grid min-w-0 grid-cols-1 gap-3">
                    <div className="space-y-1">
                      <Label className="text-xs font-normal text-muted-foreground" htmlFor="model-cb-policy-mode">
                        {t("sheet.circuitPolicyMode")}
                      </Label>
                      <Select
                        value={form.cbPolicyMode}
                        onValueChange={(v) => {
                          if (isAdminLLMModelCbPolicyMode(v)) setField("cbPolicyMode", v);
                        }}
                        disabled={pending}
                      >
                        <SelectTrigger id="model-cb-policy-mode">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="default">{t("sheet.circuitPolicyDefault")}</SelectItem>
                          <SelectItem value="enforced">{t("sheet.circuitPolicyEnforced")}</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-1">
                      <Label className="text-xs font-normal text-muted-foreground" htmlFor="model-cb-failure-threshold">
                        {t("sheet.failureThreshold")}
                      </Label>
                      <Input
                        id="model-cb-failure-threshold"
                        type="number"
                        min={0}
                        step={1}
                        value={form.cbFailureThreshold}
                        onChange={(e) => setField("cbFailureThreshold", e.target.value)}
                        disabled={pending}
                      />
                    </div>
                    <div className="space-y-1">
                      <Label className="text-xs font-normal text-muted-foreground" htmlFor="model-cb-duration-min">
                        {t("sheet.circuitDuration")}
                      </Label>
                      <Input
                        id="model-cb-duration-min"
                        type="number"
                        min={0}
                        step={1}
                        value={form.cbDurationMin}
                        onChange={(e) => setField("cbDurationMin", e.target.value)}
                        disabled={pending}
                      />
                    </div>
                    <div className="space-y-1">
                      <Label className="text-xs font-normal text-muted-foreground" htmlFor="model-cb-window-min">
                        {t("sheet.circuitWindow")}
                      </Label>
                      <Input
                        id="model-cb-window-min"
                        type="number"
                        min={0}
                        step={1}
                        value={form.cbWindowMin}
                        onChange={(e) => setField("cbWindowMin", e.target.value)}
                        disabled={pending}
                      />
                    </div>
                  </div>
                </AccordionContent>
              </AccordionItem>

              <AccordionItem value="other" className="border-border/60">
                <AccordionTrigger className="h-11 items-center py-0 text-xs font-normal text-muted-foreground hover:text-foreground hover:no-underline data-[state=open]:font-medium data-[state=open]:text-foreground [&_.accordion-trigger-icon]:translate-y-0">
                  {t("sheet.otherInfo")}
                </AccordionTrigger>
                <AccordionContent className="space-y-4 pb-4 pt-0">
                  <div className="space-y-1">
                    <Label className="text-xs font-normal text-muted-foreground">{t("sheet.accessScope")}</Label>
                    <Select
                      value={form.accessScope}
                      onValueChange={(v) => {
                        if (isAdminLLMModelAccessScope(v)) setField("accessScope", v);
                      }}
                      disabled={pending}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="public">{t("accessScope.public")}</SelectItem>
                        <SelectItem value="internal">{t("accessScope.internal")}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>

                  <FeatureGate feature="multiUser">
                    <div className="space-y-1.5">
                      <Label className="text-xs font-normal text-muted-foreground">
                        {t("sheet.permissionGroups")}
                      </Label>
                      <PermissionGroupSelector
                        groups={permissionGroups}
                        selectedIDs={manualPermissionGroupIDs}
                        matchedIDs={matchedPermissionGroupIDs}
                        disabled={pending}
                        loading={permissionGroupsLoading}
                        placeholder={t("sheet.permissionGroupsPlaceholder")}
                        emptyLabel={t("sheet.permissionGroupsEmpty")}
                        autoBadgeLabel={t("sheet.permissionGroupsAutoBadge")}
                        onSelectedIDsChange={setManualPermissionGroupIDs}
                      />
                      <p className={cn("text-[11px] leading-4", showPermissionGroupUnassigned ? "text-destructive" : "text-muted-foreground")}>
                        {showPermissionGroupUnassigned
                          ? t("sheet.permissionGroupsUnassigned")
                          : t("sheet.permissionGroupsDescription")}
                      </p>
                    </div>
                  </FeatureGate>

                  <div className="space-y-1">
                    <Label className="text-xs font-normal text-muted-foreground" htmlFor="model-desc">{t("sheet.description")}</Label>
                    <Textarea
                      id="model-desc"
                      value={form.description}
                      placeholder={t("sheet.descriptionPlaceholder")}
                      className="h-20 resize-none overflow-y-auto [field-sizing:fixed]"
                      onChange={(e) => setField("description", e.target.value)}
                      disabled={pending}
                    />
                  </div>

                  <div className="space-y-1">
                    <Label className="text-xs font-normal text-muted-foreground" htmlFor="model-system-prompt">{t("sheet.systemPrompt")}</Label>
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
                </AccordionContent>
              </AccordionItem>

              <AccordionItem value="sources" className="border-border/60">
                <AccordionTrigger className="h-11 items-center py-0 text-xs font-normal text-muted-foreground hover:text-foreground hover:no-underline data-[state=open]:font-medium data-[state=open]:text-foreground [&_.accordion-trigger-icon]:translate-y-0">
                  {mode === "create"
                    ? t("sheet.bindInitialSource")
                    : t("sheet.upstreamSources", { count: sourcesLoading ? "..." : sources.length })}
                </AccordionTrigger>
                <AccordionContent className="space-y-3 pb-4 pt-0">
                  {mode === "create" ? (
                    <div className="space-y-2">
                      <div className="flex items-center justify-between gap-2">
                        <p className="min-w-0 text-xs text-muted-foreground">
                          {t("sources.initialSourcesHelp")}
                        </p>
                        <Button
                          type="button"
                          size="sm"
                          variant="secondary"
                          className="h-8 shrink-0 px-2 text-xs font-normal shadow-none"
                          disabled={pending}
                          onClick={addBindRow}
                        >
                          <Plus className="size-3.5 stroke-1" />
                          {t("sources.addSource")}
                        </Button>
                      </div>

                      <div className="space-y-2.5">
                        {bindRows.map((row, index) => {
                          const draft = row.draft;
                          const rowModels = upstreamModelsByID[draft.upstreamID] ?? [];
                          const rowModelsLoading = upstreamModelsLoadingByID[draft.upstreamID] === true;
                          const rowProtocolOptions = getBindProtocolOptions(row);
                          const rowHasSelection = modelSourceBindDraftHasSelection(draft);

                          return (
                            <div
                              key={row.id}
                              className="space-y-2.5 rounded-md border border-border/60 bg-muted/15 p-3"
                            >
                              <div className="flex h-6 items-center justify-between gap-2">
                                <span className="text-[11px] font-medium text-muted-foreground">
                                  {t("sources.sourceDraft", { index: index + 1 })}
                                </span>
                                <Button
                                  type="button"
                                  size="icon-sm"
                                  variant="ghost"
                                  className="size-6 text-muted-foreground shadow-none"
                                  disabled={pending}
                                  onClick={() => removeBindRow(row.id)}
                                  aria-label={t("sources.removeSource")}
                                >
                                  <Trash2 className="size-3.5 stroke-1" />
                                </Button>
                              </div>

                              <div className="grid grid-cols-2 gap-2">
                                <div className="min-w-0 space-y-1">
                                  <Label className="text-xs font-normal text-muted-foreground">{t("sources.upstream")}</Label>
                                  <Select
                                    value={draft.upstreamID}
                                    onValueChange={(value) => handleBindRowUpstreamChange(row.id, value)}
                                    disabled={pending || upstreamsLoading}
                                  >
                                    <SelectTrigger className="h-8 bg-background text-xs">
                                      <SelectValue placeholder={upstreamsLoading ? t("sources.loadingUpstreams") : t("sources.selectUpstream")} />
                                    </SelectTrigger>
                                    <SelectContent>
                                      {upstreams.map((item) => (
                                        <SelectItem key={item.id} value={String(item.id)}>
                                          {item.name}
                                        </SelectItem>
                                      ))}
                                    </SelectContent>
                                  </Select>
                                </div>

                                <div className="min-w-0 space-y-1">
                                  <Label className="text-xs font-normal text-muted-foreground">{t("sources.upstreamModel")}</Label>
                                  <Select
                                    value={draft.upstreamModelID}
                                    onValueChange={(value) => handleBindRowModelChange(row.id, value)}
                                    disabled={pending || !draft.upstreamID || rowModelsLoading}
                                  >
                                    <SelectTrigger className="h-8 bg-background font-mono text-xs">
                                      <SelectValue placeholder={rowModelsLoading ? t("sources.loadingUpstreamModels") : t("sources.selectUpstreamModel")} />
                                    </SelectTrigger>
                                    <SelectContent>
                                      {rowModels.map((item) => (
                                        <SelectItem key={item.id} value={String(item.id)}>
                                          {item.upstreamModelName}
                                        </SelectItem>
                                      ))}
                                    </SelectContent>
                                  </Select>
                                </div>
                              </div>

                              <div className="grid grid-cols-2 gap-2">
                                <div className="min-w-0 space-y-1">
                                  <Label className="text-xs font-normal text-muted-foreground">{t("sources.protocol")}</Label>
                                  <Select
                                    value={draft.protocol}
                                    onValueChange={(value) => {
                                      if (isAdminLLMAdapter(value)) setBindRowField(row.id, "protocol", value);
                                    }}
                                    disabled={pending || !draft.upstreamModelID}
                                  >
                                    <SelectTrigger className="h-8 bg-background text-xs">
                                      <SelectValue placeholder={t("sources.selectProtocol")} />
                                    </SelectTrigger>
                                    <SelectContent>
                                      {rowProtocolOptions.map((protocol) => (
                                        <SelectItem key={protocol} value={protocol}>
                                          {ADAPTER_LABELS[protocol] ?? protocol}
                                        </SelectItem>
                                      ))}
                                    </SelectContent>
                                  </Select>
                                </div>

                                <div className="min-w-0 space-y-1">
                                  <Label className="text-xs font-normal text-muted-foreground">{t("sources.status")}</Label>
                                  <Select
                                    value={draft.status}
                                    onValueChange={(value) => {
                                      if (isAdminLLMStatus(value)) setBindRowField(row.id, "status", value);
                                    }}
                                    disabled={pending || !rowHasSelection}
                                  >
                                    <SelectTrigger className="h-8 bg-background text-xs">
                                      <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                      <SelectItem value="active">{t("status.active")}</SelectItem>
                                      <SelectItem value="inactive">{t("status.inactive")}</SelectItem>
                                    </SelectContent>
                                  </Select>
                                </div>
                              </div>

                              <div className="grid grid-cols-2 gap-2">
                                <div className="min-w-0 space-y-1">
                                  <div className="inline-flex items-center gap-1">
                                    <Label className="text-xs font-normal leading-4 text-muted-foreground" htmlFor={`model-source-priority-${row.id}`}>
                                      {t("sources.priority")}
                                    </Label>
                                    <Popover>
                                      <Tooltip>
                                        <TooltipTrigger asChild>
                                          <PopoverTrigger asChild>
                                            <Button
                                              type="button"
                                              variant="ghost"
                                              size="icon-xs"
                                              className="-translate-y-0.5 text-muted-foreground hover:bg-transparent hover:text-foreground"
                                              aria-label={t("sources.priorityDesc")}
                                            >
                                              <CircleHelp className="size-3" />
                                            </Button>
                                          </PopoverTrigger>
                                        </TooltipTrigger>
                                        <TooltipContent side="top" className="max-w-xs text-xs">
                                          {t("sources.priorityDesc")}
                                        </TooltipContent>
                                      </Tooltip>
                                      <PopoverContent className="w-72 max-w-[calc(100vw-2rem)] p-3 text-xs leading-5">
                                        {t("sources.priorityDesc")}
                                      </PopoverContent>
                                    </Popover>
                                  </div>
                                  <Input
                                    id={`model-source-priority-${row.id}`}
                                    value={draft.priority}
                                    inputMode="numeric"
                                    disabled={pending || !rowHasSelection}
                                    onChange={(event) => setBindRowField(row.id, "priority", event.target.value)}
                                    className="h-8 bg-background font-mono text-xs tabular-nums"
                                  />
                                </div>
                                <div className="min-w-0 space-y-1">
                                  <div className="inline-flex items-center gap-1">
                                    <Label className="text-xs font-normal leading-4 text-muted-foreground" htmlFor={`model-source-weight-${row.id}`}>
                                      {t("sources.weight")}
                                    </Label>
                                    <Popover>
                                      <Tooltip>
                                        <TooltipTrigger asChild>
                                          <PopoverTrigger asChild>
                                            <Button
                                              type="button"
                                              variant="ghost"
                                              size="icon-xs"
                                              className="-translate-y-0.5 text-muted-foreground hover:bg-transparent hover:text-foreground"
                                              aria-label={t("sources.weightDesc")}
                                            >
                                              <CircleHelp className="size-3" />
                                            </Button>
                                          </PopoverTrigger>
                                        </TooltipTrigger>
                                        <TooltipContent side="top" className="max-w-xs text-xs">
                                          {t("sources.weightDesc")}
                                        </TooltipContent>
                                      </Tooltip>
                                      <PopoverContent className="w-72 max-w-[calc(100vw-2rem)] p-3 text-xs leading-5">
                                        {t("sources.weightDesc")}
                                      </PopoverContent>
                                    </Popover>
                                  </div>
                                  <Input
                                    id={`model-source-weight-${row.id}`}
                                    value={draft.weight}
                                    inputMode="numeric"
                                    disabled={pending || !rowHasSelection}
                                    onChange={(event) => setBindRowField(row.id, "weight", event.target.value)}
                                    className="h-8 bg-background font-mono text-xs tabular-nums"
                                  />
                                </div>
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    </div>
                  ) : sourcesLoading ? (
                    <div className="h-3 w-24 animate-pulse rounded-sm bg-muted/70" aria-hidden="true" />
                  ) : sources.length === 0 ? (
                    <p className="text-xs text-muted-foreground">{t("sources.empty")}</p>
                  ) : (
                    <div className="space-y-1.5">
                      {sources.map((src) => (
                        <div
                          key={src.id}
                          className="flex h-8 items-center rounded-md bg-secondary px-2.5 text-xs"
                        >
                          <div className="flex min-w-0 flex-1 items-center justify-between gap-1.5">
                            <div className="flex min-w-0 items-center gap-1">
                              <span className="truncate font-medium">
                                {resolveValue(src.upstreamName)}
                              </span>
                              <span className="text-muted-foreground">→</span>
                              <span className="truncate font-mono text-muted-foreground">
                                {resolveValue(src.upstreamModelName)}
                              </span>
                            </div>
                            <div className="flex shrink-0 items-center">
                              {src.circuitOpen ? (
                                <Tooltip>
                                  <TooltipTrigger asChild>
                                    <ShieldAlert
                                      className="size-4 text-destructive"
                                      aria-label={t("status.circuitOpen")}
                                    />
                                  </TooltipTrigger>
                                  <TooltipContent side="top" className="text-xs">
                                    <div className="space-y-1">
                                      <div>{t("status.circuitOpen")}</div>
                                      <div>{t("sources.circuitUntil", { time: formatCircuitUntil(src.circuitUntil, locale) })}</div>
                                    </div>
                                  </TooltipContent>
                                </Tooltip>
                              ) : src.status === "inactive" || src.upstreamStatus === "inactive" || src.upstreamModelStatus === "inactive" ? (
                                <Badge variant="ghost" className="h-5 rounded-md px-1.5 text-[10px] font-normal text-muted-foreground">
                                  {t("status.inactive")}
                                </Badge>
                              ) : (
                                <Badge variant="ghost" className="h-5 rounded-md px-1.5 text-[10px] font-normal text-foreground/75">
                                  {t("status.active")}
                                </Badge>
                              )}
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </AccordionContent>
              </AccordionItem>

              {target && (
                <AccordionItem value="meta" className="border-border/60">
                  <AccordionTrigger className="h-11 items-center py-0 text-xs font-normal text-muted-foreground hover:text-foreground hover:no-underline data-[state=open]:font-medium data-[state=open]:text-foreground [&_.accordion-trigger-icon]:translate-y-0">
                    {t("sheet.metadata")}
                  </AccordionTrigger>
                  <AccordionContent className="space-y-2 pb-4 pt-0 text-xs">
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
            <Button type="submit" disabled={pending || iconUploading}>
              {pending || iconUploading ? <SpinnerLabel>{t("sheet.saving")}</SpinnerLabel> : commonT("actions.save")}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  );
}
