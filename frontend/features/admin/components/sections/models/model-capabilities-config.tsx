"use client";

import { useState } from "react";
import { CircleHelp, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { NativeToolDefinition } from "@/shared/lib/model-option-policy";
import { MODEL_OPTION_POLICY_PROTOCOL_LABELS, resolveModelOptionPolicyProtocol } from "@/shared/lib/model-option-policy";

export const MODEL_CAPABILITIES_PLACEHOLDER = `{
  "defaultOptions": {},
  "nativeToolKeys": ["openai.web_search_preview"],
  "optionControls": [
    {
      "path": "size",
      "label": "Size",
      "description": "Image output size.",
      "type": "select",
      "options": ["1024x1024", "1024x1536", "1536x1024"]
    },
    {
      "path": "quality",
      "label": "Quality",
      "description": "Image render quality.",
      "type": "select",
      "options": ["standard", "hd"]
    }
  ]
}`;

type CapabilityControlType = "text" | "select" | "number" | "boolean";

type DefaultOptionRow = {
  id: string;
  path: string;
  value: string;
};

type OptionControlRow = {
  id: string;
  path: string;
  label: string;
  description: string;
  type: CapabilityControlType;
  options: string;
  placeholder: string;
};

type NativeToolOption = {
  toolKey: string;
  provider: string;
  label: string;
  description: string;
  protocols: string[];
};

type CapabilityRowErrors = Record<string, Partial<Record<"path" | "value" | "options", string>>>;

const CAPABILITY_CONTROL_TYPES: CapabilityControlType[] = ["text", "select", "number", "boolean"];

function createCapabilityRowID(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function isPlainJSONObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function parseCapabilitiesObject(raw: string): Record<string, unknown> | null {
  const normalized = raw.trim();
  if (!normalized) {
    return {};
  }
  try {
    const parsed = JSON.parse(normalized) as unknown;
    return isPlainJSONObject(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

export function imageStreamEnabledFromCapabilities(raw: string): boolean {
  const payload = parseCapabilitiesObject(raw);
  if (!payload) {
    return true;
  }
  const image = payload.image;
  if (!isPlainJSONObject(image)) {
    return true;
  }
  return image.stream !== false;
}

export function setImageStreamEnabledInCapabilities(raw: string, enabled: boolean): string | null {
  const payload = parseCapabilitiesObject(raw);
  if (!payload) {
    return null;
  }
  if (enabled) {
    const image = payload.image;
    if (isPlainJSONObject(image)) {
      delete image.stream;
      if (Object.keys(image).length === 0) {
        delete payload.image;
      }
    }
  } else {
    payload.image = {
      ...(isPlainJSONObject(payload.image) ? payload.image : {}),
      stream: false,
    };
  }
  return Object.keys(payload).length > 0 ? JSON.stringify(payload, null, 2) : "";
}

function optionPathSegments(path: string): string[] {
  return path
    .split(".")
    .map((segment) => segment.trim())
    .filter(Boolean);
}

function isValidOptionPathInput(path: string): boolean {
  const normalized = path.trim();
  return Boolean(normalized) && normalized.split(".").every((segment) => segment.trim());
}

function formatDefaultOptionValue(value: unknown): string {
  if (value === undefined) {
    return "";
  }
  return JSON.stringify(value);
}

function flattenDefaultOptions(value: unknown, prefix: string[] = []): DefaultOptionRow[] {
  if (isPlainJSONObject(value)) {
    return Object.entries(value).flatMap(([key, child]) => flattenDefaultOptions(child, [...prefix, key]));
  }
  if (prefix.length === 0) {
    return [];
  }
  return [{
    id: createCapabilityRowID(),
    path: prefix.join("."),
    value: formatDefaultOptionValue(value),
  }];
}

function parseDefaultOptionValue(value: string): unknown {
  const normalized = value.trim();
  if (!normalized) {
    return null;
  }
  try {
    return JSON.parse(normalized) as unknown;
  } catch {
    return normalized;
  }
}

function setNestedOptionValue(target: Record<string, unknown>, path: string[], value: unknown) {
  if (path.length === 0) {
    return;
  }
  let current = target;
  path.slice(0, -1).forEach((segment) => {
    const nextValue = current[segment];
    if (!isPlainJSONObject(nextValue)) {
      current[segment] = {};
    }
    current = current[segment] as Record<string, unknown>;
  });
  current[path[path.length - 1]] = value;
}

function buildDefaultOptions(rows: DefaultOptionRow[]): Record<string, unknown> {
  const options: Record<string, unknown> = {};
  rows.forEach((row) => {
    const path = optionPathSegments(row.path);
    if (path.length === 0) {
      return;
    }
    setNestedOptionValue(options, path, parseDefaultOptionValue(row.value));
  });
  return options;
}

function normalizeControlType(value: unknown): CapabilityControlType {
  return CAPABILITY_CONTROL_TYPES.includes(value as CapabilityControlType)
    ? (value as CapabilityControlType)
    : "text";
}

function parseOptionControls(value: unknown): OptionControlRow[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value.flatMap((item): OptionControlRow[] => {
    if (!isPlainJSONObject(item) || typeof item.path !== "string") {
      return [];
    }
    const path = optionPathSegments(item.path);
    if (path.length === 0) {
      return [];
    }
    const options = Array.isArray(item.options)
      ? item.options.map((option) => (typeof option === "string" ? option.trim() : "")).filter(Boolean).join(", ")
      : "";
    return [{
      id: createCapabilityRowID(),
      path: path.join("."),
      label: typeof item.label === "string" ? item.label : "",
      description: typeof item.description === "string" ? item.description : "",
      type: normalizeControlType(item.type),
      options,
      placeholder: typeof item.placeholder === "string" ? item.placeholder : "",
    }];
  });
}

function parseControlOptions(value: string): string[] {
  const normalized = value.trim();
  if (!normalized) {
    return [];
  }
  return Array.from(
    new Set(
      normalized
        .split(",")
        .map((item) => item.trim())
        .filter(Boolean),
    ),
  );
}

function buildOptionControls(rows: OptionControlRow[]): Record<string, unknown>[] {
  return rows.flatMap((row): Record<string, unknown>[] => {
    const path = optionPathSegments(row.path);
    if (path.length === 0) {
      return [];
    }
    const control: Record<string, unknown> = {
      path: path.join("."),
      type: row.type,
    };
    const label = row.label.trim();
    const description = row.description.trim();
    const placeholder = row.placeholder.trim();
    const options = parseControlOptions(row.options);
    if (label) {
      control.label = label;
    }
    if (description) {
      control.description = description;
    }
    if (placeholder) {
      control.placeholder = placeholder;
    }
    if (options.length > 0) {
      control.options = options;
    }
    return [control];
  });
}

function markDuplicatePathErrors<T extends { id: string; path: string }>(
  rows: T[],
  errors: CapabilityRowErrors,
  message: string,
) {
  const rowsByPath = new Map<string, T[]>();
  rows.forEach((row) => {
    if (!isValidOptionPathInput(row.path)) {
      return;
    }
    const normalizedPath = optionPathSegments(row.path).join(".");
    rowsByPath.set(normalizedPath, [...(rowsByPath.get(normalizedPath) ?? []), row]);
  });
  rowsByPath.forEach((items) => {
    if (items.length < 2) {
      return;
    }
    items.forEach((item) => {
      errors[item.id] = { ...(errors[item.id] ?? {}), path: message };
    });
  });
}

function validateDefaultRows(rows: DefaultOptionRow[], t: (key: string) => string): CapabilityRowErrors {
  const errors: CapabilityRowErrors = {};
  rows.forEach((row) => {
    if (!isValidOptionPathInput(row.path)) {
      errors[row.id] = { ...(errors[row.id] ?? {}), path: t("sheet.capabilitiesQuick.pathRequired") };
    }
    if (!row.value.trim()) {
      errors[row.id] = { ...(errors[row.id] ?? {}), value: t("sheet.capabilitiesQuick.valueRequired") };
    }
  });
  markDuplicatePathErrors(rows, errors, t("sheet.capabilitiesQuick.duplicatePath"));
  return errors;
}

function validateControlRows(rows: OptionControlRow[], t: (key: string) => string): CapabilityRowErrors {
  const errors: CapabilityRowErrors = {};
  rows.forEach((row) => {
    if (!isValidOptionPathInput(row.path)) {
      errors[row.id] = { ...(errors[row.id] ?? {}), path: t("sheet.capabilitiesQuick.pathRequired") };
    }
    if (row.type === "select" && parseControlOptions(row.options).length === 0) {
      errors[row.id] = { ...(errors[row.id] ?? {}), options: t("sheet.capabilitiesQuick.selectOptionsRequired") };
    }
  });
  markDuplicatePathErrors(rows, errors, t("sheet.capabilitiesQuick.duplicatePath"));
  return errors;
}

function hasCapabilityErrors(errors: CapabilityRowErrors): boolean {
  return Object.values(errors).some((rowErrors) => Object.keys(rowErrors).length > 0);
}

function nativeToolOptionsFromCatalog(nativeTools: NativeToolDefinition[]): NativeToolOption[] {
  const byKey = new Map<string, NativeToolOption>();
  for (const tool of nativeTools) {
    const toolKey = tool.toolKey.trim();
    if (!toolKey) {
      continue;
    }
    const existing = byKey.get(toolKey);
    if (existing) {
      if (!existing.protocols.includes(tool.protocol)) {
        existing.protocols.push(tool.protocol);
      }
      continue;
    }
    byKey.set(toolKey, {
      toolKey,
      provider: tool.provider || "Provider",
      label: tool.label || tool.type || toolKey,
      description: tool.description || tool.type || toolKey,
      protocols: [tool.protocol],
    });
  }
  return Array.from(byKey.values()).sort((left, right) => {
    const providerOrder = left.provider.localeCompare(right.provider);
    return providerOrder || left.label.localeCompare(right.label) || left.toolKey.localeCompare(right.toolKey);
  });
}

function inferNativeToolKeyFromRawTool(rawTool: unknown, nativeTools: NativeToolDefinition[]): string {
  if (!isPlainJSONObject(rawTool)) {
    return "";
  }
  const rawType = typeof rawTool.type === "string" ? rawTool.type.trim() : "";
  for (const tool of nativeTools) {
    if (rawType && rawType === tool.type) {
      return tool.toolKey;
    }
    if (tool.payload && Object.keys(tool.payload).some((key) => key !== "type" && Object.prototype.hasOwnProperty.call(rawTool, key))) {
      return tool.toolKey;
    }
  }
  return "";
}

function parseNativeToolKeys(value: unknown, nativeTools: NativeToolDefinition[]): string[] {
  if (!Array.isArray(value)) {
    return [];
  }
  const known = new Set(nativeTools.map((tool) => tool.toolKey.trim()).filter(Boolean));
  return Array.from(
    new Set(
      value
        .map((item) => (typeof item === "string" ? item.trim() : ""))
        .filter((key) => key && known.has(key)),
    ),
  );
}

function deriveNativeToolKeysFromDefaultOptions(value: unknown, nativeTools: NativeToolDefinition[]): string[] {
  if (!isPlainJSONObject(value)) {
    return [];
  }
  const tools = Array.isArray(value.tools) ? value.tools : [];
  return Array.from(
    new Set(
      tools
        .map((tool) => inferNativeToolKeyFromRawTool(tool, nativeTools))
        .filter((key) => key && nativeTools.some((tool) => tool.toolKey === key)),
    ),
  );
}

export function normalizeModelCapabilitiesJSON(value: string | null | undefined, nativeTools: NativeToolDefinition[]): string {
  const trimmed = value?.trim() ?? "";
  if (!trimmed || trimmed === "{}") {
    return "";
  }
  const payload = parseCapabilitiesObject(trimmed);
  if (!payload) {
    return trimmed;
  }
  const nativeToolKeys = Array.from(
    new Set([
      ...parseNativeToolKeys(payload.nativeToolKeys, nativeTools),
      ...deriveNativeToolKeysFromDefaultOptions(payload.defaultOptions, nativeTools),
    ]),
  );
  if (nativeToolKeys.length > 0) {
    payload.nativeToolKeys = nativeToolKeys;
  } else {
    delete payload.nativeToolKeys;
  }
  return Object.keys(payload).length > 0 ? JSON.stringify(payload, null, 2) : "";
}

function nativeToolDescriptionKey(toolKey: string): string {
  return `sheet.capabilitiesQuick.nativeToolDescriptions.${toolKey.replaceAll(".", "__")}`;
}

function resolveNativeToolDescription(tool: NativeToolOption, translate: (key: string) => string): string {
  try {
    const localized = translate(nativeToolDescriptionKey(tool.toolKey));
    if (localized.trim()) {
      return localized;
    }
  } catch {
    // Translation is optional for newly added native tools; fall back to catalog text.
  }
  return tool.description || tool.toolKey;
}

function formatNativeToolProtocols(protocols: string[]): string {
  return protocols
    .map((protocol) => MODEL_OPTION_POLICY_PROTOCOL_LABELS[protocol as keyof typeof MODEL_OPTION_POLICY_PROTOCOL_LABELS] ?? protocol)
    .join(" / ");
}

function nativeToolMatchesRouteProtocols(tool: NativeToolOption, routeProtocolSet: Set<string>): boolean {
  if (routeProtocolSet.size === 0) {
    return true;
  }
  const normalizedToolProtocols = new Set(tool.protocols.map(resolveModelOptionPolicyProtocol));
  return Array.from(routeProtocolSet).some((protocol) => normalizedToolProtocols.has(resolveModelOptionPolicyProtocol(protocol)));
}

function buildCapabilitiesJSON(
  currentJSON: string,
  defaultRows: DefaultOptionRow[],
  controlRows: OptionControlRow[],
  nativeToolKeys: string[],
): string | null {
  const payload = parseCapabilitiesObject(currentJSON);
  if (!payload) {
    return null;
  }
  const defaultOptions = buildDefaultOptions(defaultRows);
  const optionControls = buildOptionControls(controlRows);
  if (Object.keys(defaultOptions).length > 0) {
    payload.defaultOptions = defaultOptions;
  } else {
    delete payload.defaultOptions;
  }
  if (optionControls.length > 0) {
    payload.optionControls = optionControls;
  } else {
    delete payload.optionControls;
  }
  if (nativeToolKeys.length > 0) {
    payload.nativeToolKeys = nativeToolKeys;
  } else {
    delete payload.nativeToolKeys;
  }
  return Object.keys(payload).length > 0 ? JSON.stringify(payload, null, 2) : "";
}

export function ModelCapabilitiesGuideButton({ t }: { t: (key: string) => string }) {
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button type="button" variant="ghost" size="sm" className="h-6 px-2 text-xs font-normal text-muted-foreground hover:text-foreground">
          <CircleHelp className="size-3.5" />
          {t("sheet.capabilitiesGuide.button")}
        </Button>
      </DialogTrigger>
      <DialogContent className="flex max-h-[min(86vh,760px)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[760px]">
        <DialogHeader className="shrink-0 px-4 py-4">
          <DialogTitle>{t("sheet.capabilitiesGuide.title")}</DialogTitle>
          <DialogDescription>{t("sheet.capabilitiesGuide.description")}</DialogDescription>
        </DialogHeader>

        <Tabs defaultValue="defaults" className="flex min-h-0 flex-1 flex-col gap-3 overflow-hidden px-4 py-2">
          <TabsList className="shrink-0">
            <TabsTrigger value="defaults">{t("sheet.capabilitiesGuide.defaultsTab")}</TabsTrigger>
            <TabsTrigger value="controls">{t("sheet.capabilitiesGuide.controlsTab")}</TabsTrigger>
            <TabsTrigger value="tools">{t("sheet.capabilitiesGuide.toolsTab")}</TabsTrigger>
            <TabsTrigger value="policy">{t("sheet.capabilitiesGuide.policyTab")}</TabsTrigger>
          </TabsList>

          <TabsContent value="defaults" className="min-h-0 flex-1 space-y-3 overflow-y-auto text-sm text-muted-foreground">
            <p className="text-xs">{t("sheet.capabilitiesGuide.defaultsDescription")}</p>
            <pre className="max-h-72 overflow-auto rounded-md bg-muted/50 p-3 text-xs text-foreground">
{`{
  "defaultOptions": {
    "reasoning": {
      "effort": "high"
    }
  }
}`}
            </pre>
          </TabsContent>

          <TabsContent value="controls" className="min-h-0 flex-1 space-y-3 overflow-y-auto text-sm text-muted-foreground">
            <p className="text-xs">{t("sheet.capabilitiesGuide.controlsDescription")}</p>
            <pre className="max-h-72 overflow-auto rounded-md bg-muted/50 p-3 text-xs text-foreground">
{`{
  "defaultOptions": {},
  "optionControls": [
    {
      "path": "size",
      "label": "Size",
      "description": "Image output size.",
      "type": "select",
      "options": ["1024x1024", "1024x1536", "1536x1024"]
    },
    {
      "path": "quality",
      "label": "Quality",
      "description": "Image render quality.",
      "type": "select",
      "options": ["standard", "hd"]
    },
    {
      "path": "n",
      "label": "Count",
      "type": "number",
      "placeholder": "1"
    }
  ]
}`}
            </pre>
            <p className="text-xs">{t("sheet.capabilitiesGuide.controlTypes")}</p>
          </TabsContent>

          <TabsContent value="tools" className="min-h-0 flex-1 space-y-3 overflow-y-auto text-sm text-muted-foreground">
            <p className="text-xs">{t("sheet.capabilitiesGuide.toolsDescription")}</p>
            <pre className="max-h-72 overflow-auto rounded-md bg-muted/50 p-3 text-xs text-foreground">
{`{
  "nativeToolKeys": [
    "openai.web_search_preview",
    "google.google_search"
  ]
}`}
            </pre>
          </TabsContent>

          <TabsContent value="policy" className="min-h-0 flex-1 space-y-3 overflow-y-auto text-sm text-muted-foreground">
            <p className="text-xs">{t("sheet.capabilitiesGuide.policyDescription")}</p>
            <pre className="max-h-72 overflow-auto rounded-md bg-muted/50 p-3 text-xs text-foreground">
{`{
  "openai_image_generations": [
    "size",
    "quality",
    "n"
  ],
  "openai_image_edits": [
    "size",
    "quality",
    "n"
  ]
}`}
            </pre>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}

function NativeToolCheckboxGrid({
  tools,
  selectedKeys,
  warning,
  t,
  onToggle,
}: {
  tools: NativeToolOption[];
  selectedKeys: string[];
  warning: string | null;
  t: (key: string) => string;
  onToggle: (toolKey: string, checked: boolean) => void;
}) {
  if (tools.length === 0) {
    return null;
  }
  return (
    <div className="grid grid-cols-1 gap-1.5 sm:grid-cols-2">
      {tools.map((tool) => {
        const description = resolveNativeToolDescription(tool, t);
        const protocolText = formatNativeToolProtocols(tool.protocols);
        return (
          <label
            key={tool.toolKey}
            className="flex min-w-0 cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted/50"
          >
            <Checkbox
              className="shrink-0 self-center"
              checked={selectedKeys.includes(tool.toolKey)}
              onCheckedChange={(nextChecked) => onToggle(tool.toolKey, nextChecked === true)}
            />
            <span className="grid min-w-0 flex-1 grid-cols-[minmax(0,1fr)] text-xs">
              <span className="flex min-w-0 items-center gap-1.5">
                <span className="min-w-0 truncate text-foreground/85">{tool.label}</span>
                <span className="shrink-0 text-[11px] text-muted-foreground">{tool.provider}</span>
              </span>
              <span className="min-w-0 truncate text-[11px] text-muted-foreground" title={description}>
                {description}
              </span>
              <Tooltip>
                <TooltipTrigger asChild>
                  <span className="min-w-0 truncate font-mono text-[10px] text-muted-foreground/80">
                    {protocolText}
                  </span>
                </TooltipTrigger>
                <TooltipContent side="top" className="max-w-72 text-xs">
                  <p>{protocolText}</p>
                  {warning ? <p className="mt-1 text-amber-300">{warning}</p> : null}
                </TooltipContent>
              </Tooltip>
              {warning ? (
                <span className="min-w-0 truncate text-[10px] text-amber-700" title={warning}>
                  {warning}
                </span>
              ) : null}
            </span>
          </label>
        );
      })}
    </div>
  );
}

export function ModelCapabilitiesQuickConfig({
  value,
  disabled,
  nativeTools,
  routeProtocols,
  t,
  commonT,
  onApply,
}: {
  value: string;
  disabled: boolean;
  nativeTools: NativeToolDefinition[];
  routeProtocols: string[];
  t: (key: string) => string;
  commonT: (key: string) => string;
  onApply: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<"defaults" | "controls" | "tools">("defaults");
  const [defaultRows, setDefaultRows] = useState<DefaultOptionRow[]>([]);
  const [controlRows, setControlRows] = useState<OptionControlRow[]>([]);
  const [nativeToolKeys, setNativeToolKeys] = useState<string[]>([]);
  const [defaultErrors, setDefaultErrors] = useState<CapabilityRowErrors>({});
  const [controlErrors, setControlErrors] = useState<CapabilityRowErrors>({});
  const nativeToolOptions = nativeToolOptionsFromCatalog(nativeTools);
  const routeProtocolSet = new Set(routeProtocols.map((protocol) => protocol.trim()).filter(Boolean));
  const compatibleNativeToolOptions = nativeToolOptions.filter((tool) => nativeToolMatchesRouteProtocols(tool, routeProtocolSet));
  const incompatibleNativeToolOptions = nativeToolOptions.filter((tool) => !nativeToolMatchesRouteProtocols(tool, routeProtocolSet));

  function loadDraft() {
    const payload = parseCapabilitiesObject(value);
    if (!payload) {
      toast.error(t("sheet.capabilitiesQuick.invalidJSON"));
      return false;
    }
    setDefaultRows(flattenDefaultOptions(payload.defaultOptions));
    setControlRows(parseOptionControls(payload.optionControls));
    setNativeToolKeys(
      Array.from(
        new Set([
          ...parseNativeToolKeys(payload.nativeToolKeys, nativeTools),
          ...deriveNativeToolKeysFromDefaultOptions(payload.defaultOptions, nativeTools),
        ]),
      ),
    );
    setDefaultErrors({});
    setControlErrors({});
    setActiveTab("defaults");
    return true;
  }

  function openDialog() {
    if (!loadDraft()) {
      return;
    }
    setOpen(true);
  }

  function updateDefaultRow(id: string, patch: Partial<DefaultOptionRow>) {
    setDefaultErrors((prev) => {
      const { [id]: _rowErrors, ...rest } = prev;
      return rest;
    });
    setDefaultRows((prev) => prev.map((row) => (row.id === id ? { ...row, ...patch } : row)));
  }

  function updateControlRow(id: string, patch: Partial<OptionControlRow>) {
    setControlErrors((prev) => {
      const { [id]: _rowErrors, ...rest } = prev;
      return rest;
    });
    setControlRows((prev) => prev.map((row) => (row.id === id ? { ...row, ...patch } : row)));
  }

  function updateControlType(id: string, type: CapabilityControlType) {
    setControlErrors((prev) => {
      const { [id]: _rowErrors, ...rest } = prev;
      return rest;
    });
    setControlRows((prev) => prev.map((row) => (
      row.id === id ? { ...row, type, options: type === "select" ? row.options : "" } : row
    )));
  }

  function toggleNativeToolKey(toolKey: string, checked: boolean) {
    setNativeToolKeys((prev) => {
      const next = new Set(prev);
      if (checked) {
        next.add(toolKey);
      } else {
        next.delete(toolKey);
      }
      return Array.from(next);
    });
  }

  function applyDraft() {
    const nextDefaultErrors = validateDefaultRows(defaultRows, t);
    const nextControlErrors = validateControlRows(controlRows, t);
    setDefaultErrors(nextDefaultErrors);
    setControlErrors(nextControlErrors);
    if (hasCapabilityErrors(nextDefaultErrors) || hasCapabilityErrors(nextControlErrors)) {
      setActiveTab(hasCapabilityErrors(nextDefaultErrors) ? "defaults" : "controls");
      toast.error(t("sheet.capabilitiesQuick.validationFailed"));
      return;
    }
    const nextValue = buildCapabilitiesJSON(value, defaultRows, controlRows, nativeToolKeys);
    if (nextValue === null) {
      toast.error(t("sheet.capabilitiesQuick.invalidJSON"));
      return;
    }
    onApply(nextValue);
    setOpen(false);
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="h-6 px-2 text-[11px]"
        disabled={disabled}
        onClick={openDialog}
      >
        {t("sheet.capabilitiesQuick.button")}
      </Button>
      <DialogContent className="flex max-h-[min(86vh,760px)] min-w-0 flex-col gap-0 overflow-hidden p-0 sm:max-w-[760px]">
        <DialogHeader className="shrink-0 px-4 py-4">
          <DialogTitle>{t("sheet.capabilitiesQuick.title")}</DialogTitle>
          <DialogDescription>{t("sheet.capabilitiesQuick.description")}</DialogDescription>
        </DialogHeader>

        <div className="min-h-0 min-w-0 flex flex-1 flex-col overflow-hidden px-4 py-2">
          <Tabs
            value={activeTab}
            onValueChange={(value) => setActiveTab(value as "defaults" | "controls" | "tools")}
            className="flex min-h-0 min-w-0 flex-1 flex-col gap-3 overflow-hidden"
          >
            <div className="flex min-w-0 shrink-0 items-center justify-between gap-2">
              <TabsList className="h-8 min-w-0 shrink">
                <TabsTrigger value="defaults">{t("sheet.capabilitiesGuide.defaultsTab")}</TabsTrigger>
                <TabsTrigger value="controls">{t("sheet.capabilitiesGuide.controlsTab")}</TabsTrigger>
                <TabsTrigger value="tools">{t("sheet.capabilitiesGuide.toolsTab")}</TabsTrigger>
              </TabsList>
              {activeTab === "defaults" ? (
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  className="h-7 shrink-0 whitespace-nowrap px-2 text-xs"
                  onClick={() => setDefaultRows((prev) => [{ id: createCapabilityRowID(), path: "", value: "" }, ...prev])}
                >
                  <Plus className="size-3.5" />
                  {t("sheet.capabilitiesQuick.addDefault")}
                </Button>
              ) : activeTab === "controls" ? (
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  className="h-7 shrink-0 whitespace-nowrap px-2 text-xs"
                  onClick={() => setControlRows((prev) => [{
                    id: createCapabilityRowID(),
                    path: "",
                    label: "",
                    description: "",
                    type: "text",
                    options: "",
                    placeholder: "",
                  }, ...prev])}
                >
                  <Plus className="size-3.5" />
                  {t("sheet.capabilitiesQuick.addControl")}
                </Button>
              ) : (
                <Badge variant="secondary" className="h-7 shrink-0 rounded-md px-2 text-xs font-normal">
                  {nativeToolKeys.length}/{nativeToolOptions.length}
                </Badge>
              )}
            </div>
            <p className="shrink-0 text-xs leading-5 text-muted-foreground">
              {activeTab === "defaults"
                ? t("sheet.capabilitiesQuick.defaultsHelp")
                : activeTab === "controls"
                  ? t("sheet.capabilitiesQuick.controlsHelp")
                  : t("sheet.capabilitiesQuick.toolsHelp")}
            </p>

            <TabsContent value="defaults" className="min-h-0 flex-1 space-y-3 overflow-y-auto pr-1">
              {defaultRows.length === 0 ? (
                <div className="rounded-md border border-dashed px-3 py-8 text-center text-xs text-muted-foreground">
                  {t("sheet.capabilitiesQuick.emptyDefaults")}
                </div>
              ) : (
                <div className="min-w-0 space-y-1.5">
                  <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_32px] gap-2 px-1 text-[11px] text-muted-foreground">
                    <span>{t("sheet.capabilitiesQuick.pathColumn")}</span>
                    <span>{t("sheet.capabilitiesQuick.valueColumn")}</span>
                    <span />
                  </div>
                  {defaultRows.map((row) => {
                    const rowErrors = defaultErrors[row.id] ?? {};
                    return (
                      <div key={row.id} className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_32px] items-start gap-2">
                        <div className="min-w-0 space-y-1">
                          <Input
                            aria-invalid={Boolean(rowErrors.path)}
                            className={cn("h-8", rowErrors.path && "border-destructive focus-visible:ring-destructive/30")}
                            value={row.path}
                            placeholder="reasoning.effort"
                            onChange={(event) => updateDefaultRow(row.id, { path: event.target.value })}
                          />
                          {rowErrors.path ? <p className="truncate px-1 text-[10px] text-destructive">{rowErrors.path}</p> : null}
                        </div>
                        <div className="min-w-0 space-y-1">
                          <Input
                            aria-invalid={Boolean(rowErrors.value)}
                            className={cn("h-8", rowErrors.value && "border-destructive focus-visible:ring-destructive/30")}
                            value={row.value}
                            placeholder='"high"'
                            onChange={(event) => updateDefaultRow(row.id, { value: event.target.value })}
                          />
                          {rowErrors.value ? <p className="truncate px-1 text-[10px] text-destructive">{rowErrors.value}</p> : null}
                        </div>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon-sm"
                          className="justify-self-end text-muted-foreground hover:text-destructive"
                          onClick={() => {
                            setDefaultRows((prev) => prev.filter((item) => item.id !== row.id));
                            setDefaultErrors((prev) => {
                              const { [row.id]: _rowErrors, ...rest } = prev;
                              return rest;
                            });
                          }}
                          aria-label={commonT("actions.delete")}
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      </div>
                    );
                  })}
                </div>
              )}
            </TabsContent>

            <TabsContent value="controls" className="min-h-0 flex-1 space-y-3 overflow-y-auto pr-1">
              {controlRows.length === 0 ? (
                <div className="rounded-md border border-dashed px-3 py-8 text-center text-xs text-muted-foreground">
                  {t("sheet.capabilitiesQuick.emptyControls")}
                </div>
              ) : (
                <div className="min-w-0 space-y-2">
                  {controlRows.map((row) => {
                    const rowErrors = controlErrors[row.id] ?? {};
                    return (
                      <div key={row.id} className="grid min-w-0 grid-cols-[minmax(0,1fr)_32px] items-center gap-2 rounded-md bg-muted/25 px-2 py-2">
                        <div className="grid min-w-0 grid-cols-1 gap-2 sm:grid-cols-3">
                          <label className="min-w-0 space-y-1">
                            <span className="block truncate px-1 text-[11px] text-muted-foreground">
                              {t("sheet.capabilitiesQuick.pathColumn")}
                            </span>
                            <Input
                              aria-invalid={Boolean(rowErrors.path)}
                              className={cn("h-8", rowErrors.path && "border-destructive focus-visible:ring-destructive/30")}
                              value={row.path}
                              placeholder="size"
                              onChange={(event) => updateControlRow(row.id, { path: event.target.value })}
                            />
                            {rowErrors.path ? <p className="truncate px-1 text-[10px] text-destructive">{rowErrors.path}</p> : null}
                          </label>
                          <label className="min-w-0 space-y-1">
                            <span className="block truncate px-1 text-[11px] text-muted-foreground">
                              {t("sheet.capabilitiesQuick.labelColumn")}
                            </span>
                            <Input
                              className="h-8"
                              value={row.label}
                              placeholder={t("sheet.capabilitiesQuick.labelPlaceholder")}
                              onChange={(event) => updateControlRow(row.id, { label: event.target.value })}
                            />
                          </label>
                          <label className="min-w-0 space-y-1">
                            <span className="block truncate px-1 text-[11px] text-muted-foreground">
                              {t("sheet.capabilitiesQuick.descriptionColumn")}
                            </span>
                            <Input
                              className="h-8"
                              value={row.description}
                              placeholder={t("sheet.capabilitiesQuick.descriptionPlaceholder")}
                              onChange={(event) => updateControlRow(row.id, { description: event.target.value })}
                            />
                          </label>
                          <label className="min-w-0 space-y-1">
                            <span className="block truncate px-1 text-[11px] text-muted-foreground">
                              {t("sheet.capabilitiesQuick.typeColumn")}
                            </span>
                            <Select
                              value={row.type}
                              onValueChange={(type) => updateControlType(row.id, type as CapabilityControlType)}
                            >
                              <SelectTrigger className="h-8 w-full">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                {CAPABILITY_CONTROL_TYPES.map((type) => (
                                  <SelectItem key={type} value={type}>
                                    {type}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          </label>
                          <label className="min-w-0 space-y-1">
                            <span className="block truncate px-1 text-[11px] text-muted-foreground">
                              {t("sheet.capabilitiesQuick.optionsColumn")}
                            </span>
                            <Input
                              aria-invalid={Boolean(rowErrors.options)}
                              className={cn("h-8", rowErrors.options && "border-destructive focus-visible:ring-destructive/30")}
                              value={row.options}
                              disabled={row.type !== "select"}
                              placeholder={t("sheet.capabilitiesQuick.optionsPlaceholder")}
                              onChange={(event) => updateControlRow(row.id, { options: event.target.value })}
                            />
                            {rowErrors.options ? <p className="truncate px-1 text-[10px] text-destructive">{rowErrors.options}</p> : null}
                          </label>
                          <label className="min-w-0 space-y-1">
                            <span className="block truncate px-1 text-[11px] text-muted-foreground">
                              {t("sheet.capabilitiesQuick.placeholderColumn")}
                            </span>
                            <Input
                              className="h-8"
                              value={row.placeholder}
                              placeholder={t("sheet.capabilitiesQuick.placeholderPlaceholder")}
                              onChange={(event) => updateControlRow(row.id, { placeholder: event.target.value })}
                            />
                          </label>
                        </div>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon-sm"
                          className="self-center justify-self-end text-muted-foreground hover:text-destructive"
                          onClick={() => {
                            setControlRows((prev) => prev.filter((item) => item.id !== row.id));
                            setControlErrors((prev) => {
                              const { [row.id]: _rowErrors, ...rest } = prev;
                              return rest;
                            });
                          }}
                          aria-label={commonT("actions.delete")}
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      </div>
                    );
                  })}
                </div>
              )}
            </TabsContent>

            <TabsContent value="tools" className="min-h-0 flex-1 space-y-3 overflow-y-auto pr-1">
              {nativeToolOptions.length === 0 ? (
                <div className="rounded-md border border-dashed px-3 py-8 text-center text-xs text-muted-foreground">
                  {t("sheet.capabilitiesQuick.emptyTools")}
                </div>
              ) : (
                <div className="space-y-3">
                  <NativeToolCheckboxGrid
                    tools={compatibleNativeToolOptions}
                    selectedKeys={nativeToolKeys}
                    warning={null}
                    t={t}
                    onToggle={toggleNativeToolKey}
                  />
                  {incompatibleNativeToolOptions.length > 0 ? (
                    <div className="space-y-2 border-t pt-3">
                      <div className="flex min-w-0 items-center gap-1.5 text-xs text-amber-700">
                        <CircleHelp className="size-3.5 shrink-0" />
                        <p className="min-w-0">{t("sheet.capabilitiesQuick.incompatibleNativeToolsHelp")}</p>
                      </div>
                      <NativeToolCheckboxGrid
                        tools={incompatibleNativeToolOptions}
                        selectedKeys={nativeToolKeys}
                        warning={t("sheet.capabilitiesQuick.nativeToolProtocolWarning")}
                        t={t}
                        onToggle={toggleNativeToolKey}
                      />
                    </div>
                  ) : null}
                </div>
              )}
            </TabsContent>
          </Tabs>
        </div>

        <DialogFooter className="shrink-0 px-4 py-3">
          <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
            {commonT("actions.cancel")}
          </Button>
          <Button type="button" onClick={applyDraft}>
            {commonT("actions.confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
