"use client";

import { useState, useEffect } from "react";
import type {
  AdminLLMCompatible,
  AdminLLMStatus,
  AdminLLMUpstreamAPIKey,
  AdminLLMUpstreamView,
  CreateAdminLLMUpstreamRequest,
  UpdateAdminLLMUpstreamRequest,
} from "@/features/admin/api/llm.types";
import {
  createAdminLLMUpstream,
  updateAdminLLMUpstream,
} from "@/features/admin/api";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
import { Textarea } from "@/components/ui/textarea";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";
import { useTranslations } from "next-intl";
import { RotateCcw, Trash2 } from "lucide-react";
import { COMPATIBLE_OPTIONS, resolveProtocolLabel } from "@/features/admin/utils/llm-display";
import { JsonCodeEditor } from "@/shared/components/json-code-editor";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";

const PROTOCOL_DEFAULT_KINDS = [
  "chat",
  "audio",
  "image_gen",
  "image_edit",
  "video_gen",
] as const;

const NO_PROTOCOL_DEFAULT = "__system_default__";

const PROTOCOL_OPTIONS_BY_KIND: Record<(typeof PROTOCOL_DEFAULT_KINDS)[number], string[]> = {
  chat: [
    "openai_responses",
    "openai_chat_completions",
    "anthropic_messages",
    "google_generate_content",
    "xai_responses",
  ],
  audio: [
    "openai_responses",
    "openai_chat_completions",
    "anthropic_messages",
    "google_generate_content",
    "xai_responses",
  ],
  image_gen: [
    "openai_image_generations",
    "google_image_generation",
    "xai_image",
  ],
  image_edit: [
    "openai_image_edits",
    "google_image_generation",
    "xai_image_edits",
  ],
  video_gen: [
    "openai_video_generations",
  ],
};

const CODE_TEXTAREA_CLASS = "font-mono text-xs placeholder:font-sans placeholder:text-xs";

function apiKeysLinesToJson(lines: string): string {
  const keys = lines.split("\n").map((l) => l.trim()).filter(Boolean);
  if (keys.length === 0) return "";
  return JSON.stringify({
    keys: keys.map((k) => ({ key: k, status: "active" })),
    strategy: "round_robin",
  });
}

function hasKeysField(value: unknown): value is { keys: unknown } {
  return value !== null && typeof value === "object" && "keys" in value;
}

function countMaskedApiKeys(json: string): number {
  try {
    const parsed: unknown = JSON.parse(json);
    if (Array.isArray(parsed)) {
      return parsed.length;
    }
    if (hasKeysField(parsed) && Array.isArray(parsed.keys)) {
      return parsed.keys.length;
    }
  } catch {
    return 0;
  }
  return 0;
}

type MaskedAPIKeyItem = {
  id: string;
  index: number;
  keyMasked: string;
  status: string;
  note: string;
};

function normalizeAPIKeyItems(items: AdminLLMUpstreamAPIKey[] | undefined): MaskedAPIKeyItem[] {
  if (!Array.isArray(items)) return [];
  return items
    .filter((item) => item.id.trim() && Number.isInteger(item.index) && item.index >= 0 && item.keyMasked.trim())
    .map((item) => ({
      id: item.id,
      index: item.index,
      keyMasked: item.keyMasked,
      status: item.status || "active",
      note: item.note || "",
    }));
}

function maskedAPIKeyItemsFromJson(json: string): MaskedAPIKeyItem[] {
  try {
    const parsed: unknown = JSON.parse(json);
    const rawItems = Array.isArray(parsed)
      ? parsed
      : hasKeysField(parsed) && Array.isArray(parsed.keys)
        ? parsed.keys
        : [];
    return rawItems
      .map((item, index) => {
        if (item === null || typeof item !== "object") return null;
        const record = item as Record<string, unknown>;
        const keyMasked = typeof record.key === "string" ? record.key : "";
        if (!keyMasked.trim()) return null;
        return {
          id: "",
          index,
          keyMasked,
          status: typeof record.status === "string" && record.status ? record.status : "active",
          note: typeof record.note === "string" ? record.note : "",
        };
      })
      .filter((item): item is MaskedAPIKeyItem => item !== null);
  } catch {
    return [];
  }
}

function maskedAPIKeyItems(target: AdminLLMUpstreamView | null): MaskedAPIKeyItem[] {
  if (!target) return [];
  const typedItems = normalizeAPIKeyItems(target.apiKeyItems);
  if (typedItems.length > 0) return typedItems;
  return maskedAPIKeyItemsFromJson(target.apiKeysMasked);
}

function isActiveAPIKeyItem(item: MaskedAPIKeyItem): boolean {
  return item.status === "" || item.status === "active";
}

function parseProtocolDefaults(raw: string): Record<string, string> {
  try {
    const parsed: unknown = JSON.parse(raw);
    if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
      return {};
    }
    return Object.fromEntries(
      Object.entries(parsed)
        .filter(([, value]) => typeof value === "string" && value.trim())
        .map(([kind, value]) => [kind, String(value).trim()]),
    );
  } catch {
    return {};
  }
}

function stringifyProtocolDefaults(defaults: Record<string, string>): string {
  const normalized = Object.fromEntries(
    PROTOCOL_DEFAULT_KINDS
      .map((kind) => [kind, defaults[kind]?.trim() ?? ""] as const)
      .filter(([, protocol]) => protocol),
  );
  return Object.keys(normalized).length > 0 ? JSON.stringify(normalized) : "";
}

function protocolDefaultValue(raw: string, kind: string): string {
  return parseProtocolDefaults(raw)[kind] || NO_PROTOCOL_DEFAULT;
}

type UpstreamSheetProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  mode: "create" | "edit";
  target: AdminLLMUpstreamView | null;
  onSuccess: (item: AdminLLMUpstreamView) => void;
  onManageModels?: (item: AdminLLMUpstreamView) => void;
};

type FormState = {
  name: string;
  apiKeysLines: string;
  baseUrl: string;
  compatible: AdminLLMCompatible | "";
  protocolDefaultsJson: string;
  status: AdminLLMStatus;
  connectTimeoutMs: string;
  readTimeoutMs: string;
  streamIdleTimeoutMs: string;
  cbFailureThreshold: string;
  cbModelThreshold: string;
  cbThresholdLogic: "or" | "and";
  cbDurationMin: string;
  cbWindowMin: string;
  headersJson: string;
};

function buildInitialState(target: AdminLLMUpstreamView | null): FormState {
  if (!target) {
    return {
      name: "",
      apiKeysLines: "",
      baseUrl: "",
      compatible: "openai",
      protocolDefaultsJson: "",
      status: "active",
      connectTimeoutMs: "",
      readTimeoutMs: "",
      streamIdleTimeoutMs: "",
      cbFailureThreshold: "",
      cbModelThreshold: "",
      cbThresholdLogic: "or",
      cbDurationMin: "",
      cbWindowMin: "",
      headersJson: "",
    };
  }
  return {
    name: target.name,
    apiKeysLines: "",
    baseUrl: target.baseURL,
    compatible: target.compatible,
    protocolDefaultsJson: target.protocolDefaultsJSON || "",
    status: target.status,
    connectTimeoutMs: target.connectTimeoutMS ? String(target.connectTimeoutMS) : "",
    readTimeoutMs: target.readTimeoutMS ? String(target.readTimeoutMS) : "",
    streamIdleTimeoutMs: target.streamIdleTimeoutMS
      ? String(target.streamIdleTimeoutMS)
      : "",
    cbFailureThreshold:
      target.cbFailureThreshold != null ? String(target.cbFailureThreshold) : "",
    cbModelThreshold:
      target.cbModelThreshold != null ? String(target.cbModelThreshold) : "",
    cbThresholdLogic: target.cbThresholdLogic ?? "or",
    cbDurationMin: target.cbDurationMin ? String(target.cbDurationMin) : "",
    cbWindowMin: target.cbWindowMin ? String(target.cbWindowMin) : "",
    headersJson: target.headersJSON || "",
  };
}

export function UpstreamSheet({
  open,
  onOpenChange,
  mode,
  target,
  onSuccess,
  onManageModels,
}: UpstreamSheetProps) {
  const t = useTranslations("adminChannels");
  const commonT = useTranslations("common");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [form, setForm] = useState<FormState>(() => buildInitialState(target));
  const [pending, setPending] = useState(false);
  const [expandedSections, setExpandedSections] = useState<string[]>([]);
  const [pendingDeleteAPIKeyIDs, setPendingDeleteAPIKeyIDs] = useState<Set<string>>(() => new Set());
  const [deleteAPIKeyTarget, setDeleteAPIKeyTarget] = useState<MaskedAPIKeyItem | null>(null);

  useEffect(() => {
    if (open) {
      setForm(buildInitialState(target));
      setExpandedSections([]);
      setPendingDeleteAPIKeyIDs(new Set());
      setDeleteAPIKeyTarget(null);
    }
  }, [open, target]);

  function setField<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  function setProtocolDefault(kind: string, protocol: string) {
    setForm((prev) => {
      const defaults = parseProtocolDefaults(prev.protocolDefaultsJson);
      if (protocol === NO_PROTOCOL_DEFAULT) {
        delete defaults[kind];
      } else {
        defaults[kind] = protocol;
      }
      return {
        ...prev,
        protocolDefaultsJson: stringifyProtocolDefaults(defaults),
      };
    });
  }

  function markAPIKeyForDeletion(id: string) {
    setPendingDeleteAPIKeyIDs((prev) => new Set(prev).add(id));
    setDeleteAPIKeyTarget(null);
  }

  function restoreAPIKey(id: string) {
    setPendingDeleteAPIKeyIDs((prev) => {
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
  }

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setPending(true);
    try {
      const token = await resolveAccessToken();
      const apiKeysJson = apiKeysLinesToJson(form.apiKeysLines);

      if (mode === "create") {
        const payload: CreateAdminLLMUpstreamRequest = {
          name: form.name.trim(),
          baseURL: form.baseUrl.trim(),
          apiKeys: apiKeysJson,
          compatible: form.compatible || undefined,
          protocolDefaultsJSON: form.protocolDefaultsJson.trim() || undefined,
          status: form.status,
          connectTimeoutMS: form.connectTimeoutMs
            ? Number(form.connectTimeoutMs)
            : undefined,
          readTimeoutMS: form.readTimeoutMs ? Number(form.readTimeoutMs) : undefined,
          streamIdleTimeoutMS: form.streamIdleTimeoutMs
            ? Number(form.streamIdleTimeoutMs)
            : undefined,
          cbFailureThreshold:
            form.cbFailureThreshold !== "" ? Number(form.cbFailureThreshold) : undefined,
          cbModelThreshold:
            form.cbModelThreshold !== "" ? Number(form.cbModelThreshold) : undefined,
          cbThresholdLogic: form.cbThresholdLogic,
          cbDurationMin: form.cbDurationMin ? Number(form.cbDurationMin) : undefined,
          cbWindowMin: form.cbWindowMin ? Number(form.cbWindowMin) : undefined,
          headersJSON: form.headersJson.trim() || undefined,
        };
        const data = await createAdminLLMUpstream(token, payload);
        onSuccess(data.upstream);
        onOpenChange(false);
        onManageModels?.(data.upstream);
        toast.success(t("toast.upstreamCreated"));
      } else if (target) {
        const payload: UpdateAdminLLMUpstreamRequest = {};
        const nextName = form.name.trim();
        const nextBaseURL = form.baseUrl.trim();
        const deleteAPIKeyIDs = Array.from(pendingDeleteAPIKeyIDs).sort();
        const existingAPIKeys = maskedAPIKeyItems(target);
        if (nextName !== target.name) payload.name = nextName;
        if (nextBaseURL !== target.baseURL) payload.baseURL = nextBaseURL;
        if (form.compatible !== target.compatible) payload.compatible = form.compatible;
        if (form.protocolDefaultsJson !== (target.protocolDefaultsJSON || ""))
          payload.protocolDefaultsJSON = form.protocolDefaultsJson.trim() || undefined;
        if (form.status !== target.status) payload.status = form.status;
        if (form.connectTimeoutMs !== String(target.connectTimeoutMS ?? ""))
          payload.connectTimeoutMS = form.connectTimeoutMs
            ? Number(form.connectTimeoutMs)
            : undefined;
        if (form.readTimeoutMs !== String(target.readTimeoutMS ?? ""))
          payload.readTimeoutMS = form.readTimeoutMs
            ? Number(form.readTimeoutMs)
            : undefined;
        if (form.streamIdleTimeoutMs !== String(target.streamIdleTimeoutMS ?? ""))
          payload.streamIdleTimeoutMS = form.streamIdleTimeoutMs
            ? Number(form.streamIdleTimeoutMs)
            : undefined;
        if (form.cbFailureThreshold !== String(target.cbFailureThreshold ?? ""))
          payload.cbFailureThreshold =
            form.cbFailureThreshold !== "" ? Number(form.cbFailureThreshold) : undefined;
        if (form.cbModelThreshold !== String(target.cbModelThreshold ?? ""))
          payload.cbModelThreshold =
            form.cbModelThreshold !== "" ? Number(form.cbModelThreshold) : undefined;
        if (form.cbThresholdLogic !== (target.cbThresholdLogic ?? "or"))
          payload.cbThresholdLogic = form.cbThresholdLogic;
        if (form.cbDurationMin !== String(target.cbDurationMin ?? ""))
          payload.cbDurationMin = form.cbDurationMin
            ? Number(form.cbDurationMin)
            : undefined;
        if (form.cbWindowMin !== String(target.cbWindowMin ?? ""))
          payload.cbWindowMin = form.cbWindowMin ? Number(form.cbWindowMin) : undefined;
        if (form.headersJson.trim() !== (target.headersJSON || ""))
          payload.headersJSON = form.headersJson.trim() || undefined;
        if (apiKeysJson) payload.addAPIKeys = apiKeysJson;
        if (deleteAPIKeyIDs.length > 0) {
          const remainingActiveKeyCount = existingAPIKeys.filter(
            (item) => !pendingDeleteAPIKeyIDs.has(item.id) && isActiveAPIKeyItem(item),
          ).length;
          if (!apiKeysJson && remainingActiveKeyCount === 0) {
            toast.error(t("toast.updateFailed"), {
              description: t("sheet.apiKeyDeleteRequiresReplacement"),
            });
            return;
          }
          payload.deleteAPIKeyIDs = deleteAPIKeyIDs;
        }

        const data = await updateAdminLLMUpstream(token, target.id, payload);
        onSuccess(data.upstream);
        onOpenChange(false);
        toast.success(t("toast.upstreamUpdated"));
      }
    } catch (error) {
      toast.error(mode === "create" ? t("toast.createFailed") : t("toast.updateFailed"), {
        description: resolveErrorMessage(error),
      });
    } finally {
      setPending(false);
    }
  }

  const existingAPIKeyItems = mode === "edit" ? maskedAPIKeyItems(target) : [];
  const pendingDeleteCount = existingAPIKeyItems.filter((item) => pendingDeleteAPIKeyIDs.has(item.id)).length;

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="flex flex-col gap-0 sm:max-w-[460px]">
        <SheetHeader className="px-4 pb-4">
          <SheetTitle>{mode === "create" ? t("sheet.createTitle") : t("sheet.editTitle")}</SheetTitle>
        </SheetHeader>

        <form onSubmit={handleSubmit} className="flex flex-col flex-1 min-h-0">
          <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-2">
            <div className="min-w-0 space-y-1">
              <Label className="text-xs font-normal text-muted-foreground" htmlFor="upstream-name">{t("sheet.name")} *</Label>
              <Input
                id="upstream-name"
                required
                placeholder={t("sheet.namePlaceholder")}
                value={form.name}
                onChange={(e) => setField("name", e.target.value)}
              />
            </div>

            <div className="min-w-0 space-y-1">
              <Label className="text-xs font-normal text-muted-foreground" htmlFor="upstream-url">{t("sheet.baseUrl")} *</Label>
              <Input
                id="upstream-url"
                required
                placeholder="https://api.example.com/v1"
                value={form.baseUrl}
                onChange={(e) => setField("baseUrl", e.target.value)}
              />
            </div>

            <div className="min-w-0 space-y-1">
              <Label className="text-xs font-normal text-muted-foreground" htmlFor="upstream-keys">
                {mode === "create" ? `${t("sheet.apiKeys")} *` : t("sheet.apiKeysAdd")}
              </Label>
              <Textarea
                id="upstream-keys"
                className={`h-24 resize-none overflow-auto whitespace-pre [field-sizing:fixed] ${CODE_TEXTAREA_CLASS}`}
                placeholder={mode === "create" ? t("sheet.apiKeysPlaceholder") : t("sheet.apiKeysAddPlaceholder")}
                required={mode === "create"}
                spellCheck={false}
                value={form.apiKeysLines}
                wrap="off"
                onChange={(e) => setField("apiKeysLines", e.target.value)}
              />
              {mode === "edit" && existingAPIKeyItems.length > 0 ? (
                <div className="mt-2 space-y-1.5">
                  <div className="flex items-center justify-between gap-3 text-[11px] leading-4 text-muted-foreground">
                    <span className="truncate">{t("sheet.existingKeys", { count: existingAPIKeyItems.length })}</span>
                    {pendingDeleteCount > 0 ? (
                      <span className="shrink-0 text-destructive">
                        {t("sheet.apiKeyDeletePending", { count: pendingDeleteCount })}
                      </span>
                    ) : null}
                  </div>
                  <div className="overflow-hidden rounded-md bg-muted/35">
                    {existingAPIKeyItems.map((item) => {
                      const markedForDeletion = pendingDeleteAPIKeyIDs.has(item.id);
                      return (
                        <div
                          key={item.id || item.index}
                          className="group/key flex h-8 items-center gap-3 border-b border-border/40 px-2.5 text-xs last:border-b-0"
                        >
                          <div className="min-w-0 flex-1">
                            <div className="flex min-h-6 min-w-0 items-center gap-2 leading-none">
                              <span className={`flex min-h-6 items-center truncate font-mono ${markedForDeletion ? "text-muted-foreground line-through" : "text-foreground"}`}>
                                {item.keyMasked}
                              </span>
                              {markedForDeletion ? (
                                <span className="shrink-0 text-[11px] text-destructive">
                                  {t("sheet.apiKeyDeleteAfterSave")}
                                </span>
                              ) : null}
                            </div>
                            {item.note ? (
                              <p className="mt-1 truncate text-muted-foreground">{item.note}</p>
                            ) : null}
                          </div>
                          {item.id ? (
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="icon-sm"
                                  className={markedForDeletion
                                    ? "size-6 text-muted-foreground hover:text-foreground"
                                    : "size-6 text-muted-foreground opacity-70 hover:text-destructive group-hover/key:opacity-100"}
                                  aria-label={markedForDeletion ? t("sheet.apiKeyRestore") : t("sheet.apiKeyDelete")}
                                  onClick={() => {
                                    if (markedForDeletion) {
                                      restoreAPIKey(item.id);
                                    } else {
                                      setDeleteAPIKeyTarget(item);
                                    }
                                  }}
                                >
                                  {markedForDeletion ? (
                                    <RotateCcw className="size-3.5 stroke-1" />
                                  ) : (
                                    <Trash2 className="size-3.5 stroke-1" />
                                  )}
                                </Button>
                              </TooltipTrigger>
                              <TooltipContent side="top">
                                {markedForDeletion ? t("sheet.apiKeyRestore") : t("sheet.apiKeyDelete")}
                              </TooltipContent>
                            </Tooltip>
                          ) : null}
                        </div>
                      );
                    })}
                  </div>
                </div>
              ) : mode === "edit" && target?.apiKeysMasked ? (
                <p className="text-xs text-muted-foreground">
                  {t("sheet.existingKeys", { count: countMaskedApiKeys(target.apiKeysMasked) })}
                </p>
              ) : null}
            </div>

            <div className="min-w-0 space-y-1">
              <Label className="text-xs font-normal text-muted-foreground">{t("fields.compatibility")} *</Label>
              <Select
                value={form.compatible}
                onValueChange={(v) => setField("compatible", v as AdminLLMCompatible)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {COMPATIBLE_OPTIONS.map((opt) => (
                    <SelectItem key={opt.value} value={opt.value}>
                      {opt.value === "custom" ? t("compatible.custom") : opt.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="min-w-0 space-y-1">
              <Label className="text-xs font-normal text-muted-foreground">{t("fields.status")} *</Label>
              <Select
                value={form.status}
                onValueChange={(v) => setField("status", v as AdminLLMStatus)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="active">{t("status.active")}</SelectItem>
                  <SelectItem value="inactive">{t("status.inactive")}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {mode === "edit" && target && onManageModels && (
              <div className="min-w-0 space-y-1">
                <Label className="text-xs font-normal text-muted-foreground">{t("sheet.models")}</Label>
                <div className="flex h-8 items-center justify-between rounded-md bg-muted/35 px-2.5 text-xs">
                  <span className="text-muted-foreground">
                    {t("table.modelCountSummary", {
                      active: target.activeModelsCount,
                      total: target.modelsCount,
                    })}
                  </span>
                  <Button
                    type="button"
                    variant="ghost"
                    size="xs"
                    onClick={() => {
                      onOpenChange(false);
                      onManageModels(target);
                    }}
                  >
                    {t("actions.manageModels")}
                  </Button>
                </div>
              </div>
            )}

            <Accordion
              type="multiple"
              value={expandedSections}
              onValueChange={setExpandedSections}
              className="border-y border-border/60"
            >
              <AccordionItem value="protocol-defaults" className="border-border/60">
                <AccordionTrigger className="h-11 items-center py-0 text-xs font-normal text-muted-foreground hover:text-foreground hover:no-underline data-[state=open]:font-medium data-[state=open]:text-foreground [&_.accordion-trigger-icon]:translate-y-0">
                  <span>{t("sheet.protocolDefaults")}</span>
                </AccordionTrigger>
                <AccordionContent className="space-y-3 pb-4 pt-0">
                  <div className="space-y-3">
                    {PROTOCOL_DEFAULT_KINDS.map((kind) => (
                      <div key={kind} className="min-w-0 space-y-1">
                        <Label className="text-xs font-normal text-muted-foreground">{t(`kinds.${kind}`)}</Label>
                        <Select
                          value={protocolDefaultValue(form.protocolDefaultsJson, kind)}
                          onValueChange={(value) => setProtocolDefault(kind, value)}
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value={NO_PROTOCOL_DEFAULT}>{t("sheet.systemDefault")}</SelectItem>
                            {PROTOCOL_OPTIONS_BY_KIND[kind].map((protocol) => (
                              <SelectItem key={protocol} value={protocol}>
                                {resolveProtocolLabel(protocol)}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    ))}
                    <p className="text-xs text-muted-foreground">
                      {t("sheet.protocolDefaultsDescription")}
                    </p>
                  </div>
                </AccordionContent>
              </AccordionItem>

              <AccordionItem value="timeouts" className="border-border/60">
                <AccordionTrigger className="h-11 items-center py-0 text-xs font-normal text-muted-foreground hover:text-foreground hover:no-underline data-[state=open]:font-medium data-[state=open]:text-foreground [&_.accordion-trigger-icon]:translate-y-0">
                  <span>{t("sheet.timeouts")}</span>
                </AccordionTrigger>
                <AccordionContent className="space-y-4 pb-4 pt-0">
                  <div className="min-w-0 space-y-1">
                    <Label className="text-xs font-normal text-muted-foreground" htmlFor="connect-timeout">{t("sheet.connectTimeout")}</Label>
                    <Input
                      id="connect-timeout"
                      type="number"
                      placeholder="10000"
                      value={form.connectTimeoutMs}
                      onChange={(e) => setField("connectTimeoutMs", e.target.value)}
                    />
                  </div>
                  <div className="min-w-0 space-y-1">
                    <Label className="text-xs font-normal text-muted-foreground" htmlFor="read-timeout">{t("sheet.readTimeout")}</Label>
                    <Input
                      id="read-timeout"
                      type="number"
                      placeholder="120000"
                      value={form.readTimeoutMs}
                      onChange={(e) => setField("readTimeoutMs", e.target.value)}
                    />
                  </div>
                  <div className="min-w-0 space-y-1">
                    <Label className="text-xs font-normal text-muted-foreground" htmlFor="stream-timeout">{t("sheet.streamTimeout")}</Label>
                    <Input
                      id="stream-timeout"
                      type="number"
                      placeholder="60000"
                      value={form.streamIdleTimeoutMs}
                      onChange={(e) => setField("streamIdleTimeoutMs", e.target.value)}
                    />
                  </div>
                </AccordionContent>
              </AccordionItem>

              <AccordionItem value="circuit-break" className="border-border/60">
                <AccordionTrigger className="h-11 items-center py-0 text-xs font-normal text-muted-foreground hover:text-foreground hover:no-underline data-[state=open]:font-medium data-[state=open]:text-foreground [&_.accordion-trigger-icon]:translate-y-0">
                  <span>{t("sheet.circuitBreak")}</span>
                </AccordionTrigger>
                <AccordionContent className="space-y-4 pb-4 pt-0">
                  <div className="min-w-0 space-y-1">
                    <Label className="text-xs font-normal text-muted-foreground" htmlFor="cb-failure-threshold">{t("sheet.failureThreshold")}</Label>
                    <Input
                      id="cb-failure-threshold"
                      type="number"
                      placeholder="0"
                      value={form.cbFailureThreshold}
                      onChange={(e) => setField("cbFailureThreshold", e.target.value)}
                    />
                    <p className="text-xs text-muted-foreground">
                      {t("sheet.failureThresholdDescription")}
                    </p>
                  </div>
                  <div className="min-w-0 space-y-1">
                    <Label className="text-xs font-normal text-muted-foreground" htmlFor="cb-model-threshold">{t("sheet.modelThreshold")}</Label>
                    <Input
                      id="cb-model-threshold"
                      type="number"
                      placeholder="0"
                      value={form.cbModelThreshold}
                      onChange={(e) => setField("cbModelThreshold", e.target.value)}
                    />
                    <p className="text-xs text-muted-foreground">
                      {t("sheet.modelThresholdDescription")}
                    </p>
                  </div>
                  <div className="min-w-0 space-y-1">
                    <Label className="text-xs font-normal text-muted-foreground">{t("sheet.thresholdLogic")}</Label>
                    <Select
                      value={form.cbThresholdLogic}
                      onValueChange={(v) => setField("cbThresholdLogic", v as "or" | "and")}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="or">{t("sheet.thresholdLogicOr")}</SelectItem>
                        <SelectItem value="and">{t("sheet.thresholdLogicAnd")}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="min-w-0 space-y-1">
                    <Label className="text-xs font-normal text-muted-foreground" htmlFor="cb-duration">{t("sheet.circuitDuration")}</Label>
                    <Input
                      id="cb-duration"
                      type="number"
                      value={form.cbDurationMin}
                      onChange={(e) => setField("cbDurationMin", e.target.value)}
                    />
                  </div>
                  <div className="min-w-0 space-y-1">
                    <Label className="text-xs font-normal text-muted-foreground" htmlFor="cb-window">{t("sheet.circuitWindow")}</Label>
                    <Input
                      id="cb-window"
                      type="number"
                      value={form.cbWindowMin}
                      onChange={(e) => setField("cbWindowMin", e.target.value)}
                    />
                  </div>
                </AccordionContent>
              </AccordionItem>

              <AccordionItem value="headers" className="border-border/60">
                <AccordionTrigger className="h-11 items-center py-0 text-xs font-normal text-muted-foreground hover:text-foreground hover:no-underline data-[state=open]:font-medium data-[state=open]:text-foreground [&_.accordion-trigger-icon]:translate-y-0">
                  <span>{t("sheet.headers")}</span>
                </AccordionTrigger>
                <AccordionContent className="space-y-4 pb-4 pt-0">
                  <JsonCodeEditor
                    placeholder={`{"X-Custom-Header": "value"}`}
                    value={form.headersJson}
                    height={220}
                    onChange={(nextValue) => setField("headersJson", nextValue)}
                  />
                </AccordionContent>
              </AccordionItem>
            </Accordion>
          </div>

          <SheetFooter className="flex flex-row justify-end px-4 py-3 gap-2">
            <Button
              type="button"
              variant="ghost"
              onClick={() => onOpenChange(false)}
              disabled={pending}
            >
              {commonT("actions.cancel")}
            </Button>
            <Button type="submit" disabled={pending}>
              {pending ? (
                <SpinnerLabel>
                  {mode === "create" ? t("sheet.creating") : t("sheet.saving")}
                </SpinnerLabel>
              ) : mode === "create" ? (
                commonT("actions.create")
              ) : (
                commonT("actions.save")
              )}
            </Button>
          </SheetFooter>
        </form>

        <AlertDialog
          open={deleteAPIKeyTarget !== null}
          onOpenChange={(nextOpen) => {
            if (!nextOpen) setDeleteAPIKeyTarget(null);
          }}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>{t("sheet.apiKeyDeleteTitle")}</AlertDialogTitle>
              <AlertDialogDescription>
                {t("sheet.apiKeyDeleteDescription", {
                  key: deleteAPIKeyTarget?.keyMasked ?? "",
                })}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{commonT("actions.cancel")}</AlertDialogCancel>
              <AlertDialogAction
                variant="destructive"
                onClick={() => {
                  if (deleteAPIKeyTarget) markAPIKeyForDeletion(deleteAPIKeyTarget.id);
                }}
              >
                {t("sheet.apiKeyDeleteConfirm")}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </SheetContent>
    </Sheet>
  );
}
