"use client";

import { useState, useEffect } from "react";
import type {
  AdminLLMCompatible,
  AdminLLMStatus,
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
import { Textarea } from "@/components/ui/textarea";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";
import { useTranslations } from "next-intl";
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
    "google_imagen",
  ],
  image_edit: [
    "openai_image_edits",
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

  useEffect(() => {
    if (open) {
      setForm(buildInitialState(target));
      setExpandedSections([]);
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
        if (apiKeysJson) payload.apiKeys = apiKeysJson;

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

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="flex flex-col">
        <SheetHeader className="px-4 pb-4">
          <SheetTitle>{mode === "create" ? t("sheet.createTitle") : t("sheet.editTitle")}</SheetTitle>
        </SheetHeader>

        <form onSubmit={handleSubmit} className="flex flex-col flex-1 min-h-0">
          <div className="overflow-y-auto flex-1 px-4 space-y-4">
            <div>
              <Label htmlFor="upstream-name">{t("sheet.name")} *</Label>
              <Input
                id="upstream-name"
                required
                placeholder={t("sheet.namePlaceholder")}
                value={form.name}
                onChange={(e) => setField("name", e.target.value)}
              />
            </div>

            <div>
              <Label htmlFor="upstream-url">{t("sheet.baseUrl")} *</Label>
              <Input
                id="upstream-url"
                required
                placeholder="https://api.example.com/v1"
                value={form.baseUrl}
                onChange={(e) => setField("baseUrl", e.target.value)}
              />
            </div>

            <div>
              <Label htmlFor="upstream-keys">{t("sheet.apiKeys")} *</Label>
              <Textarea
                id="upstream-keys"
                className={`h-24 resize-none overflow-auto whitespace-pre [field-sizing:fixed] ${CODE_TEXTAREA_CLASS}`}
                placeholder={t("sheet.apiKeysPlaceholder")}
                required={mode === "create"}
                value={form.apiKeysLines}
                wrap="off"
                onChange={(e) => setField("apiKeysLines", e.target.value)}
              />
              {mode === "edit" && target?.apiKeysMasked ? (
                <p className="text-xs text-muted-foreground">
                  {t("sheet.existingKeys", { count: countMaskedApiKeys(target.apiKeysMasked) })}
                </p>
              ) : null}
            </div>

            <div>
              <Label>{t("fields.compatibility")} *</Label>
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

            <div>
              <Label>{t("fields.status")} *</Label>
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
              <div>
                <Label>{t("sheet.models")}</Label>
                <div className="flex items-center justify-between rounded-md border px-3 py-2 text-xs">
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
              className="space-y-4"
            >
              <AccordionItem value="protocol-defaults" className="px-1">
                <AccordionTrigger className="py-1.5 text-xs text-muted-foreground hover:no-underline">
                  <span>{t("sheet.protocolDefaults")}</span>
                </AccordionTrigger>
                <AccordionContent className="space-y-4 pt-4">
                  <div className="space-y-3">
                    {PROTOCOL_DEFAULT_KINDS.map((kind) => (
                      <div key={kind} className="space-y-1.5">
                        <Label>{t(`kinds.${kind}`)}</Label>
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

              <AccordionItem value="timeouts" className="px-1">
                <AccordionTrigger className="py-1.5 text-xs text-muted-foreground hover:no-underline">
                  <span>{t("sheet.timeouts")}</span>
                </AccordionTrigger>
                <AccordionContent className="space-y-4 pt-4">
                  <div>
                    <Label htmlFor="connect-timeout">{t("sheet.connectTimeout")}</Label>
                    <Input
                      id="connect-timeout"
                      type="number"
                      placeholder="10000"
                      value={form.connectTimeoutMs}
                      onChange={(e) => setField("connectTimeoutMs", e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="read-timeout">{t("sheet.readTimeout")}</Label>
                    <Input
                      id="read-timeout"
                      type="number"
                      placeholder="120000"
                      value={form.readTimeoutMs}
                      onChange={(e) => setField("readTimeoutMs", e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="stream-timeout">{t("sheet.streamTimeout")}</Label>
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

              <AccordionItem value="circuit-break" className="px-1">
                <AccordionTrigger className="py-1.5 text-xs text-muted-foreground hover:no-underline">
                  <span>{t("sheet.circuitBreak")}</span>
                </AccordionTrigger>
                <AccordionContent className="space-y-4 pt-4">
                  <div>
                    <Label htmlFor="cb-failure-threshold">{t("sheet.failureThreshold")}</Label>
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
                  <div>
                    <Label htmlFor="cb-model-threshold">{t("sheet.modelThreshold")}</Label>
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
                  <div>
                    <Label>{t("sheet.thresholdLogic")}</Label>
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
                  <div>
                    <Label htmlFor="cb-duration">{t("sheet.circuitDuration")}</Label>
                    <Input
                      id="cb-duration"
                      type="number"
                      value={form.cbDurationMin}
                      onChange={(e) => setField("cbDurationMin", e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="cb-window">{t("sheet.circuitWindow")}</Label>
                    <Input
                      id="cb-window"
                      type="number"
                      value={form.cbWindowMin}
                      onChange={(e) => setField("cbWindowMin", e.target.value)}
                    />
                  </div>
                </AccordionContent>
              </AccordionItem>

              <AccordionItem value="headers" className="px-1">
                <AccordionTrigger className="py-1.5 text-xs text-muted-foreground hover:no-underline">
                  <span>{t("sheet.headers")}</span>
                </AccordionTrigger>
                <AccordionContent className="space-y-4 pt-4">
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
      </SheetContent>
    </Sheet>
  );
}
