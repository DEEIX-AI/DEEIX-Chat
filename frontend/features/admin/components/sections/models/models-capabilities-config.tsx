"use client";

import { useState } from "react";
import { Check, ChevronDownIcon, CircleHelp, CopyPlus, Plus, Trash2 } from "lucide-react";
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
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { AdminLLMModelDTO } from "@/features/admin/api/llm.types";
import { ModelCapabilitiesPresetDialog } from "@/features/admin/components/sections/models/models-capabilities-presets";
import type { NativeToolDefinition } from "@/shared/lib/model-option-policy";
import { MODEL_OPTION_POLICY_PROTOCOL_LABELS, resolveModelOptionPolicyProtocol } from "@/shared/lib/model-option-policy";

export const MODEL_CAPABILITIES_PLACEHOLDER = `{
  "defaultOptions": {},
  "nativeTools": [
    {
      "key": "openai.web_search_preview",
      "protocols": ["openai_chat_completions", "openai_responses"],
      "type": "web_search_preview",
      "label": "Web Search Preview",
      "enabled": true,
      "defaultEnabled": false,
      "payload": {
        "type": "web_search_preview"
      }
    }
  ],
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

type ParameterRow = {
  id: string;
  path: string;
  label: string;
  description: string;
  type: CapabilityControlType;
  options: string;
  defaultValue: string;
};

type NativeToolOption = {
  id: string;
  toolKey: string;
  provider: string;
  label: string;
  description: string;
  type: string;
  payload: Record<string, unknown>;
  protocols: string[];
};

type CapabilityRowErrors = Record<string, Partial<Record<"path" | "type" | "options", string>>>;

type NativeToolRow = {
  id: string;
  key: string;
  provider: string;
  protocols: string;
  type: string;
  label: string;
  description: string;
  enabled: boolean;
  defaultEnabled: boolean;
  payload: string;
  catalog: boolean;
};

type NativeToolRowErrors = Record<string, Partial<Record<"key" | "protocols" | "type" | "payload", string>>>;

const CAPABILITY_CONTROL_TYPES: CapabilityControlType[] = ["text", "select", "number", "boolean"];

function nativeToolDisplayName(row: NativeToolRow): { name: string; specificName: string } {
  const specificName = row.type.trim() || row.key.trim().split(".").pop() || row.label.trim();
  const name = specificName.replace(/_\d{8}$/, "");
  const fallback = row.label || row.key || "Tool";
  return {
    name: name || fallback,
    specificName: specificName || fallback,
  };
}

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

function flattenDefaultOptions(value: unknown, prefix: string[] = []): { path: string; value: string; rawValue: unknown }[] {
  if (isPlainJSONObject(value)) {
    return Object.entries(value).flatMap(([key, child]) => flattenDefaultOptions(child, [...prefix, key]));
  }
  if (prefix.length === 0) {
    return [];
  }
  return [{
    path: prefix.join("."),
    rawValue: value,
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

function buildDefaultOptions(rows: ParameterRow[]): Record<string, unknown> {
  const options: Record<string, unknown> = {};
  rows.forEach((row) => {
    if (!row.defaultValue.trim()) {
      return;
    }
    const path = optionPathSegments(row.path);
    if (path.length === 0) {
      return;
    }
    setNestedOptionValue(options, path, parseDefaultOptionValue(row.defaultValue));
  });
  return options;
}

function normalizeControlType(value: unknown): CapabilityControlType {
  return CAPABILITY_CONTROL_TYPES.includes(value as CapabilityControlType)
    ? (value as CapabilityControlType)
    : "text";
}

function inferControlType(value: unknown): CapabilityControlType {
  if (typeof value === "boolean") {
    return "boolean";
  }
  if (typeof value === "number") {
    return "number";
  }
  return "text";
}

function parseOptionControls(value: unknown): ParameterRow[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value.flatMap((item): ParameterRow[] => {
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
      defaultValue: "",
    }];
  });
}

function parseParameterRows(defaultOptions: unknown, optionControls: unknown): ParameterRow[] {
  const defaultsByPath = new Map<string, { value: string; rawValue: unknown }>();
  flattenDefaultOptions(defaultOptions).forEach((item) => {
    defaultsByPath.set(item.path, { value: item.value, rawValue: item.rawValue });
  });
  const rows = parseOptionControls(optionControls).map((row) => {
    const defaultItem = defaultsByPath.get(row.path);
    defaultsByPath.delete(row.path);
    return {
      ...row,
      defaultValue: defaultItem?.value ?? "",
    };
  });
  defaultsByPath.forEach((item, path) => {
    rows.push({
      id: createCapabilityRowID(),
      path,
      label: "",
      description: "",
      type: inferControlType(item.rawValue),
      options: "",
      defaultValue: item.value,
    });
  });
  return rows;
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

function buildOptionControls(rows: ParameterRow[]): Record<string, unknown>[] {
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
    const options = parseControlOptions(row.options);
    if (label) {
      control.label = label;
    }
    if (description) {
      control.description = description;
    }
    if (row.type === "select" && options.length > 0) {
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

function validateParameterRows(rows: ParameterRow[], t: (key: string) => string): CapabilityRowErrors {
  const errors: CapabilityRowErrors = {};
  rows.forEach((row) => {
    if (!isValidOptionPathInput(row.path)) {
      errors[row.id] = { ...(errors[row.id] ?? {}), path: t("sheet.capabilitiesQuick.pathRequired") };
    }
    if (!CAPABILITY_CONTROL_TYPES.includes(row.type)) {
      errors[row.id] = { ...(errors[row.id] ?? {}), type: t("sheet.capabilitiesQuick.typeRequired") };
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

function nativeToolMatchesRouteProtocols(protocols: string[], routeProtocolSet: Set<string>): boolean {
  return routeProtocolSet.size === 0 || protocols.some((protocol) => routeProtocolSet.has(resolveModelOptionPolicyProtocol(protocol)));
}

function sortNativeToolOptionsByRoute(
  options: NativeToolOption[],
  routeProtocolSet: Set<string>,
): NativeToolOption[] {
  return [...options].sort((left, right) => {
    const leftMatched = nativeToolMatchesRouteProtocols(left.protocols, routeProtocolSet);
    const rightMatched = nativeToolMatchesRouteProtocols(right.protocols, routeProtocolSet);
    if (leftMatched !== rightMatched) {
      return leftMatched ? -1 : 1;
    }
    const providerOrder = left.provider.localeCompare(right.provider);
    return providerOrder || left.label.localeCompare(right.label) || left.toolKey.localeCompare(right.toolKey) || left.type.localeCompare(right.type);
  });
}

function nativeToolOptionsFromCatalog(
  nativeTools: NativeToolDefinition[],
  routeProtocols: string[] = [],
): NativeToolOption[] {
  const options = new Map<string, NativeToolOption>();
  const routeProtocolSet = new Set(routeProtocols.map((protocol) => resolveModelOptionPolicyProtocol(protocol)).filter(Boolean));
  nativeTools.forEach((tool) => {
    const toolKey = tool.toolKey.trim();
    const type = tool.type.trim();
    const id = nativeToolOptionID(toolKey, type);
    const existing = options.get(id);
    if (existing) {
      existing.protocols = Array.from(new Set([...existing.protocols, tool.protocol].filter(Boolean)));
      return;
    }
    options.set(id, {
      id,
      toolKey,
      provider: tool.provider || "Provider",
      label: tool.label || tool.type || tool.toolKey,
      description: tool.description || tool.type || tool.toolKey,
      type,
      payload: tool.payload ?? {},
      protocols: [tool.protocol].filter(Boolean),
    });
  });
  return sortNativeToolOptionsByRoute(Array.from(options.values()), routeProtocolSet);
}

function nativeToolOptionID(key: string, type: string): string {
  return [key.trim(), type.trim()].filter(Boolean).join(":");
}

function nativeToolMatchesRawTool(rawTool: Record<string, unknown>, tool: NativeToolDefinition): boolean {
  const rawType = typeof rawTool.type === "string" ? rawTool.type.trim() : "";
  if (rawType && rawType === tool.type) {
    return true;
  }
  return Boolean(
    tool.payload &&
      Object.keys(tool.payload).some((key) => key !== "type" && Object.prototype.hasOwnProperty.call(rawTool, key)),
  );
}

function nativeToolDefinitionMatchesRouteProtocols(tool: NativeToolDefinition, routeProtocolSet: Set<string>): boolean {
  if (routeProtocolSet.size === 0) {
    return true;
  }
  return routeProtocolSet.has(resolveModelOptionPolicyProtocol(tool.protocol));
}

function collectNativeToolKeysFromDefaultOptions(
  value: unknown,
  nativeTools: NativeToolDefinition[],
  routeProtocols: string[],
): { derivedKeys: string[]; matchingKeys: Set<string> } {
  const matchingKeys = new Set<string>();
  if (!isPlainJSONObject(value)) {
    return { derivedKeys: [], matchingKeys };
  }
  const routeProtocolSet = new Set(routeProtocols.map((protocol) => resolveModelOptionPolicyProtocol(protocol)).filter(Boolean));
  const tools = Array.isArray(value.tools) ? value.tools : [];
  const derivedKeys: string[] = [];
  for (const rawTool of tools) {
    if (!isPlainJSONObject(rawTool)) {
      continue;
    }
    const matchingTools = nativeTools.filter((tool) => nativeToolMatchesRawTool(rawTool, tool));
    matchingTools.forEach((tool) => {
      if (tool.toolKey) {
        matchingKeys.add(tool.toolKey);
      }
    });
    const selected = routeProtocolSet.size > 0
      ? matchingTools.find((tool) => nativeToolDefinitionMatchesRouteProtocols(tool, routeProtocolSet))
      : matchingTools[0];
    if (selected?.toolKey) {
      derivedKeys.push(selected.toolKey);
    }
  }
  return { derivedKeys: Array.from(new Set(derivedKeys)), matchingKeys };
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

function resolveNativeToolKeysFromCapabilities(
  nativeToolKeys: unknown,
  defaultOptions: unknown,
  nativeTools: NativeToolDefinition[],
  routeProtocols: string[],
): string[] {
  const explicitKeys = parseNativeToolKeys(nativeToolKeys, nativeTools);
  const { derivedKeys, matchingKeys } = collectNativeToolKeysFromDefaultOptions(defaultOptions, nativeTools, routeProtocols);
  return Array.from(
    new Set(
      explicitKeys
        .filter((key) => !matchingKeys.has(key) || derivedKeys.includes(key))
        .concat(derivedKeys),
    ),
  );
}

export function normalizeModelCapabilitiesJSON(
  value: string | null | undefined,
  nativeTools: NativeToolDefinition[],
  routeProtocols: string[],
): string {
  const trimmed = value?.trim() ?? "";
  if (!trimmed || trimmed === "{}") {
    return "";
  }
  const payload = parseCapabilitiesObject(trimmed);
  if (!payload) {
    return trimmed;
  }
  const nativeToolRows = parseNativeToolRows(payload, nativeTools, routeProtocols);
  const nextNativeTools = buildNativeTools(nativeToolRows);
  if (nextNativeTools.length > 0) {
    payload.nativeTools = nextNativeTools;
  } else {
    delete payload.nativeTools;
  }
  delete payload.nativeToolKeys;
  return Object.keys(payload).length > 0 ? JSON.stringify(payload, null, 2) : "";
}

function formatNativeToolProtocols(protocols: string[]): string {
  return protocols
    .map((protocol) => MODEL_OPTION_POLICY_PROTOCOL_LABELS[protocol as keyof typeof MODEL_OPTION_POLICY_PROTOCOL_LABELS] ?? protocol)
    .join(" / ");
}

function parseNativeToolProtocolsInput(value: string): string[] {
  return Array.from(
    new Set(
      value
        .split(",")
        .map((item) => item.trim())
        .filter(Boolean),
    ),
  );
}

function formatNativeToolProtocolsInput(protocols: string[]): string {
  return protocols.map((protocol) => protocol.trim()).filter(Boolean).join(", ");
}

function nativeToolProtocolSelectOptions(
  routeProtocols: string[],
  currentProtocols: string,
): { value: string; label: string }[] {
  const protocols = [
    ...routeProtocols,
    ...Object.keys(MODEL_OPTION_POLICY_PROTOCOL_LABELS),
    ...parseNativeToolProtocolsInput(currentProtocols),
  ];
  const seen = new Set<string>();
  return protocols.flatMap((protocol) => {
    const normalized = resolveModelOptionPolicyProtocol(protocol.trim());
    if (!normalized || seen.has(normalized)) {
      return [];
    }
    seen.add(normalized);
    return [{
      value: normalized,
      label: MODEL_OPTION_POLICY_PROTOCOL_LABELS[normalized as keyof typeof MODEL_OPTION_POLICY_PROTOCOL_LABELS] ?? normalized,
    }];
  });
}

function NativeToolProtocolsSelect({
  value,
  options,
  invalid,
  placeholder,
  onChange,
}: {
  value: string;
  options: { value: string; label: string }[];
  invalid?: boolean;
  placeholder: string;
  onChange: (value: string) => void;
}) {
  const selected = parseNativeToolProtocolsInput(value);
  const selectedSet = new Set(selected);
  const selectedLabel = options
    .filter((option) => selectedSet.has(option.value))
    .map((option) => option.label)
    .join(", ");

  function toggle(protocol: string) {
    const next = new Set(selected);
    if (next.has(protocol)) {
      next.delete(protocol);
    } else {
      next.add(protocol);
    }
    onChange(formatNativeToolProtocolsInput(Array.from(next)));
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          size="sm"
          role="combobox"
          className={cn(
            "h-8 min-w-0 w-full justify-between gap-2 border-input/40 bg-transparent px-3 py-1 text-xs font-normal text-muted-foreground shadow-none hover:bg-transparent focus-visible:border-ring/60 focus-visible:ring-[1px] focus-visible:ring-ring/40 has-[>svg]:px-3",
            invalid && "border-destructive focus-visible:ring-destructive/30",
          )}
        >
          <span className={cn("min-w-0 flex-1 truncate text-left", selectedLabel && "text-foreground/75")}>
            {selectedLabel || placeholder}
          </span>
          <ChevronDownIcon className="size-3 shrink-0 text-muted-foreground opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-72 p-1">
        {options.map((option) => (
          <button
            key={option.value}
            type="button"
            onClick={() => toggle(option.value)}
            className="relative flex w-full items-center rounded-sm py-1.5 pr-8 pl-2 text-xs font-normal hover:bg-accent"
          >
            <span className="min-w-0 flex-1 truncate text-left">{option.label}</span>
            <Check
              className={cn(
                "absolute right-2 size-4 shrink-0 text-muted-foreground",
                selectedSet.has(option.value) ? "opacity-100" : "opacity-0",
              )}
            />
          </button>
        ))}
      </PopoverContent>
    </Popover>
  );
}

function nativeToolProtocolsFromConfig(value: Record<string, unknown>): string[] {
  if (Array.isArray(value.protocols)) {
    return Array.from(
      new Set(
        value.protocols
          .map((item) => (typeof item === "string" ? item.trim() : ""))
          .filter(Boolean),
      ),
    );
  }
  return typeof value.protocol === "string" && value.protocol.trim() ? [value.protocol.trim()] : [];
}

function nativeToolPayloadType(payload: Record<string, unknown>): string {
  return typeof payload.type === "string" ? payload.type.trim() : "";
}

function nativeToolRowFromOption(option: NativeToolOption, enabled: boolean): NativeToolRow {
  return {
    id: option.id,
    key: option.toolKey,
    provider: option.provider,
    protocols: formatNativeToolProtocolsInput(option.protocols),
    type: option.type,
    label: option.label,
    description: option.description,
    enabled,
    defaultEnabled: false,
    payload: JSON.stringify(option.payload, null, 2),
    catalog: true,
  };
}

function nativeToolRowFromConfig(value: Record<string, unknown>, index: number): NativeToolRow | null {
  const payload = isPlainJSONObject(value.payload) ? value.payload : {};
  const key = typeof (value.key ?? value.toolKey) === "string" ? String(value.key ?? value.toolKey).trim() : "";
  const protocols = nativeToolProtocolsFromConfig(value);
  const type = typeof value.type === "string" ? value.type.trim() : nativeToolPayloadType(payload);
  const id = typeof value.id === "string" && value.id.trim()
    ? value.id.trim()
    : nativeToolOptionID(key, type) || createCapabilityRowID();
  if (!key && protocols.length === 0 && !type && Object.keys(payload).length === 0) {
    return null;
  }
  return {
    id,
    key,
    provider: typeof value.provider === "string" ? value.provider.trim() : "",
    protocols: formatNativeToolProtocolsInput(protocols),
    type,
    label: typeof value.label === "string" ? value.label.trim() : type || key || `Tool ${index + 1}`,
    description: typeof value.description === "string" ? value.description.trim() : "",
    enabled: value.enabled !== false,
    defaultEnabled: value.defaultEnabled === true,
    payload: JSON.stringify(payload, null, 2),
    catalog: false,
  };
}

function parseNativeToolRows(
  payload: Record<string, unknown>,
  nativeTools: NativeToolDefinition[],
  routeProtocols: string[],
): NativeToolRow[] {
  const options = nativeToolOptionsFromCatalog(nativeTools, routeProtocols);
  const routeProtocolSet = new Set(routeProtocols.map((protocol) => resolveModelOptionPolicyProtocol(protocol)).filter(Boolean));
  const rows = options.map((option) => nativeToolRowFromOption(option, false));
  const applyRow = (row: NativeToolRow) => {
    const id = nativeToolOptionID(row.key, row.type) || row.id;
    const index = rows.findIndex((item) => item.id === id);
    if (index < 0) {
      rows.unshift({ ...row, id });
      return;
    }
    rows[index] = { ...rows[index], ...row, id, catalog: rows[index].catalog };
  };

  if (Array.isArray(payload.nativeTools)) {
    payload.nativeTools.forEach((item, index) => {
      if (!isPlainJSONObject(item)) {
        return;
      }
      const row = nativeToolRowFromConfig(item, index);
      if (row) {
        applyRow(row);
      }
    });
    return rows;
  }

  resolveNativeToolKeysFromCapabilities(payload.nativeToolKeys, payload.defaultOptions, nativeTools, routeProtocols).forEach((key) => {
    const matched = options.find((option) => option.toolKey === key && option.protocols.some((protocol) => routeProtocolSet.has(resolveModelOptionPolicyProtocol(protocol))))
      ?? options.find((option) => option.toolKey === key);
    if (matched) {
      applyRow(nativeToolRowFromOption(matched, true));
    }
  });

  return rows;
}

function buildNativeTools(rows: NativeToolRow[]): Record<string, unknown>[] {
  return rows.flatMap((row): Record<string, unknown>[] => {
    if (!row.enabled) {
      return [];
    }
    let payload: unknown;
    try {
      payload = JSON.parse(row.payload.trim() || "{}") as unknown;
    } catch {
      return [];
    }
    if (!isPlainJSONObject(payload)) {
      return [];
    }
    const item: Record<string, unknown> = {
      key: row.key.trim(),
      protocols: parseNativeToolProtocolsInput(row.protocols),
      label: row.label.trim() || row.type.trim() || row.key.trim(),
      enabled: true,
      defaultEnabled: row.defaultEnabled,
      payload,
    };
    const provider = row.provider.trim();
    const type = row.type.trim() || nativeToolPayloadType(payload);
    const description = row.description.trim();
    if (provider) {
      item.provider = provider;
    }
    if (type) {
      item.type = type;
    }
    if (description) {
      item.description = description;
    }
    return [item];
  });
}

function validateNativeToolRows(rows: NativeToolRow[], t: (key: string) => string): NativeToolRowErrors {
  const errors: NativeToolRowErrors = {};
  rows.forEach((row) => {
    if (!row.enabled) {
      return;
    }
    const rowErrors: Partial<Record<"key" | "protocols" | "type" | "payload", string>> = {};
    if (!row.key.trim()) {
      rowErrors.key = t("sheet.capabilitiesQuick.nativeToolKeyRequired");
    }
    if (parseNativeToolProtocolsInput(row.protocols).length === 0) {
      rowErrors.protocols = t("sheet.capabilitiesQuick.nativeToolProtocolRequired");
    }
    let payload: unknown;
    try {
      payload = JSON.parse(row.payload.trim() || "{}") as unknown;
    } catch {
      rowErrors.payload = t("sheet.capabilitiesQuick.nativeToolPayloadInvalid");
    }
    if (payload !== undefined && !isPlainJSONObject(payload)) {
      rowErrors.payload = t("sheet.capabilitiesQuick.nativeToolPayloadInvalid");
    }
    if (!row.type.trim() && isPlainJSONObject(payload) && !nativeToolPayloadType(payload)) {
      rowErrors.type = t("sheet.capabilitiesQuick.nativeToolTypeRequired");
    }
    if (Object.keys(rowErrors).length > 0) {
      errors[row.id] = rowErrors;
    }
  });
  return errors;
}

function buildCapabilitiesJSON(
  currentJSON: string,
  parameterRows: ParameterRow[],
  nativeToolRows: NativeToolRow[],
): string | null {
  const payload = parseCapabilitiesObject(currentJSON);
  if (!payload) {
    return null;
  }
  const defaultOptions = buildDefaultOptions(parameterRows);
  const optionControls = buildOptionControls(parameterRows);
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
  const nativeTools = buildNativeTools(nativeToolRows);
  if (nativeTools.length > 0) {
    payload.nativeTools = nativeTools;
  } else {
    delete payload.nativeTools;
  }
  delete payload.nativeToolKeys;
  return Object.keys(payload).length > 0 ? JSON.stringify(payload, null, 2) : "";
}

export function ModelCapabilitiesGuideButton({ t }: { t: (key: string) => string }) {
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button type="button" variant="link" size="sm" className="h-auto gap-1 px-0 py-0 text-[11px] font-normal text-muted-foreground">
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
    "store": false,
    "reasoning": {
      "effort": "medium"
    },
    "text": {
      "verbosity": "medium"
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
  "nativeTools": [
    {
      "key": "xai.x_search",
      "protocols": ["xai_responses"],
      "type": "x_search",
      "label": "X Search",
      "enabled": true,
      "defaultEnabled": true,
      "payload": {
        "type": "x_search",
        "enable_image_understanding": true
      }
    },
    {
      "key": "xai.web_search",
      "protocols": ["xai_responses"],
      "type": "web_search",
      "label": "Web Search",
      "enabled": true,
      "defaultEnabled": true,
      "payload": {
        "type": "web_search",
        "enable_image_understanding": true,
        "enable_image_search": true
      }
    },
    {
      "key": "xai.code_interpreter",
      "protocols": ["xai_responses"],
      "type": "code_interpreter",
      "label": "Code Interpreter",
      "enabled": true,
      "defaultEnabled": false,
      "payload": {
        "type": "code_interpreter",
        "container": {
          "type": "auto"
        }
      }
    }
  ],
  "defaultOptions": {
    "store": false
  }
}`}
            </pre>
            <p className="text-xs">{t("sheet.capabilitiesGuide.toolsAutoDescription")}</p>
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

export function ModelCapabilitiesQuickConfig({
  value,
  disabled,
  presetModels = [],
  currentModelID,
  nativeTools,
  routeProtocols,
  t,
  commonT,
  triggerVariant = "ghost",
  triggerClassName,
  triggerLabel,
  onApply,
}: {
  value: string;
  disabled: boolean;
  presetModels?: AdminLLMModelDTO[];
  currentModelID?: number | null;
  nativeTools: NativeToolDefinition[];
  routeProtocols: string[];
  t: (key: string, values?: Record<string, string | number>) => string;
  commonT: (key: string) => string;
  triggerVariant?: "default" | "secondary" | "ghost" | "outline" | "link";
  triggerClassName?: string;
  triggerLabel?: string;
  onApply: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [presetOpen, setPresetOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<"parameters" | "tools">("parameters");
  const [draftBaseJSON, setDraftBaseJSON] = useState("");
  const [parameterRows, setParameterRows] = useState<ParameterRow[]>([]);
  const [nativeToolRows, setNativeToolRows] = useState<NativeToolRow[]>([]);
  const [expandedNativeToolID, setExpandedNativeToolID] = useState("");
  const [parameterErrors, setParameterErrors] = useState<CapabilityRowErrors>({});
  const [nativeToolErrors, setNativeToolErrors] = useState<NativeToolRowErrors>({});
  const routeProtocolSet = new Set(routeProtocols.map((protocol) => resolveModelOptionPolicyProtocol(protocol)).filter(Boolean));

  function loadDraft() {
    const payload = parseCapabilitiesObject(value);
    if (!payload) {
      toast.error(t("sheet.capabilitiesQuick.invalidJSON"));
      return false;
    }
    setParameterRows(parseParameterRows(payload.defaultOptions, payload.optionControls));
    const nextNativeToolRows = parseNativeToolRows(payload, nativeTools, routeProtocols);
    setNativeToolRows(nextNativeToolRows);
    setExpandedNativeToolID("");
    setParameterErrors({});
    setNativeToolErrors({});
    setActiveTab("parameters");
    setDraftBaseJSON(value);
    return true;
  }

  function openDialog() {
    if (!loadDraft()) {
      return;
    }
    setOpen(true);
  }

  function updateParameterRow(id: string, patch: Partial<ParameterRow>) {
    setParameterErrors((prev) => {
      const { [id]: _rowErrors, ...rest } = prev;
      return rest;
    });
    setParameterRows((prev) => prev.map((row) => (row.id === id ? { ...row, ...patch } : row)));
  }

  function updateParameterType(id: string, type: CapabilityControlType) {
    setParameterErrors((prev) => {
      const { [id]: _rowErrors, ...rest } = prev;
      return rest;
    });
    setParameterRows((prev) => prev.map((row) => (
      row.id === id ? { ...row, type, options: type === "select" ? row.options : "" } : row
    )));
  }

  function updateNativeToolRow(id: string, patch: Partial<NativeToolRow>) {
    setNativeToolErrors((prev) => {
      const { [id]: _rowErrors, ...rest } = prev;
      return rest;
    });
    setNativeToolRows((prev) => prev.map((row) => (row.id === id ? { ...row, ...patch } : row)));
  }

  function addNativeToolRow() {
    const protocol = routeProtocols[0]?.trim() || "openai_responses";
    const id = createCapabilityRowID();
    setNativeToolRows((prev) => [{
      id,
      key: "",
      provider: "",
      protocols: protocol,
      type: "",
      label: "",
      description: "",
      enabled: true,
      defaultEnabled: true,
      payload: "{\n  \"type\": \"\"\n}",
      catalog: false,
    }, ...prev]);
    setExpandedNativeToolID(id);
  }

  function removeNativeToolRow(id: string) {
    const nextRows = nativeToolRows.filter((item) => item.id !== id);
    setNativeToolRows(nextRows);
    setExpandedNativeToolID((current) => (current === id ? "" : current));
    setNativeToolErrors((prev) => {
      const { [id]: _rowErrors, ...rest } = prev;
      return rest;
    });
  }

  function addParameterRow() {
    setParameterRows((prev) => [{
      id: createCapabilityRowID(),
      path: "",
      label: "",
      description: "",
      type: "text",
      options: "",
      defaultValue: "",
    }, ...prev]);
  }

  function applyDraft() {
    const nextParameterErrors = validateParameterRows(parameterRows, t);
    const nextNativeToolErrors = validateNativeToolRows(nativeToolRows, t);
    setParameterErrors(nextParameterErrors);
    setNativeToolErrors(nextNativeToolErrors);
    if (hasCapabilityErrors(nextParameterErrors) || Object.keys(nextNativeToolErrors).length > 0) {
      setActiveTab(hasCapabilityErrors(nextParameterErrors) ? "parameters" : "tools");
      toast.error(t("sheet.capabilitiesQuick.validationFailed"));
      return;
    }
    const nextValue = buildCapabilitiesJSON(draftBaseJSON, parameterRows, nativeToolRows);
    if (nextValue === null) {
      toast.error(t("sheet.capabilitiesQuick.invalidJSON"));
      return;
    }
    onApply(nextValue);
    setOpen(false);
  }

  function applyPresetValue(nextValue: string) {
    const payload = parseCapabilitiesObject(nextValue);
    if (!payload) {
      toast.error(t("sheet.capabilitiesQuick.invalidJSON"));
      return;
    }
    setParameterRows(parseParameterRows(payload.defaultOptions, payload.optionControls));
    setNativeToolRows(parseNativeToolRows(payload, nativeTools, routeProtocols));
    setExpandedNativeToolID("");
    setParameterErrors({});
    setNativeToolErrors({});
    setActiveTab("parameters");
    setDraftBaseJSON(nextValue);
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <Button
        type="button"
        variant={triggerVariant}
        size="sm"
        className={cn("h-6 px-2 text-[11px]", triggerClassName)}
        disabled={disabled}
        onClick={openDialog}
      >
        {triggerLabel ?? t("sheet.capabilitiesQuick.button")}
      </Button>
      <DialogContent className="flex h-[min(86vh,760px)] min-w-0 flex-col gap-0 overflow-hidden p-0 sm:max-w-[760px]">
        <DialogHeader className="shrink-0 px-4 py-4">
          <div className="flex min-w-0 items-start justify-between gap-3">
            <div className="min-w-0 space-y-1.5">
              <DialogTitle>{t("sheet.capabilitiesQuick.title")}</DialogTitle>
              <DialogDescription>{t("sheet.capabilitiesQuick.description")}</DialogDescription>
            </div>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              className="h-7 shrink-0 gap-1 px-2 text-xs font-normal shadow-none"
              onClick={() => setPresetOpen(true)}
            >
              <CopyPlus className="size-3.5" />
              {t("sheet.capabilitiesPreset.button")}
            </Button>
          </div>
        </DialogHeader>

        <ModelCapabilitiesPresetDialog
          open={presetOpen}
          onOpenChange={setPresetOpen}
          models={presetModels}
          currentModelID={currentModelID}
          routeProtocols={routeProtocols}
          t={t}
          commonT={commonT}
          onApply={applyPresetValue}
        />

        <div className="min-h-0 min-w-0 flex flex-1 flex-col overflow-hidden px-4 py-2">
          <Tabs
            value={activeTab}
            onValueChange={(value) => setActiveTab(value as "parameters" | "tools")}
            className="flex min-h-0 min-w-0 flex-1 flex-col gap-3 overflow-hidden"
          >
            <div className="min-w-0 shrink-0">
              <TabsList className="grid h-8 w-full min-w-0 grid-cols-2">
                <TabsTrigger value="parameters" className="min-w-0">
                  <span className="min-w-0 truncate">{t("sheet.capabilitiesQuick.parametersTab")}</span>
                </TabsTrigger>
                <TabsTrigger value="tools" className="min-w-0">
                  <span className="min-w-0 truncate">{t("sheet.capabilitiesGuide.toolsTab")}</span>
                </TabsTrigger>
              </TabsList>
            </div>

            <TabsContent value="parameters" className="min-h-0 flex flex-1 flex-col gap-3 overflow-hidden pr-1">
              <div className="flex min-w-0 shrink-0 items-start justify-between gap-3">
                <p className="min-w-0 flex-1 text-xs leading-5 text-muted-foreground">
                  {t("sheet.capabilitiesQuick.parametersIntro")}
                </p>
                <Button
                  type="button"
                  variant="default"
                  size="sm"
                  className="h-7 shrink-0 whitespace-nowrap px-2 text-xs"
                  onClick={addParameterRow}
                >
                  <Plus className="size-3.5" />
                  {t("sheet.capabilitiesQuick.addParameter")}
                </Button>
              </div>
              {parameterRows.length === 0 ? (
                <div className="flex min-h-0 flex-1 flex-col items-center justify-center rounded-md border border-dashed px-3 py-8 text-center">
                  <p className="text-xs text-muted-foreground">{t("sheet.capabilitiesQuick.emptyParameters")}</p>
                </div>
              ) : (
                <div className="min-h-0 flex-1 overflow-y-auto rounded-md border border-dashed p-3">
                  <div className="min-w-0 space-y-2">
                    {parameterRows.map((row) => {
                      const rowErrors = parameterErrors[row.id] ?? {};
                      return (
                        <div key={row.id} className="grid min-w-0 grid-cols-[minmax(0,1fr)_32px] items-start gap-2 rounded-md bg-muted/40 px-2 py-2">
                          <div className="grid min-w-0 grid-cols-1 gap-2 sm:grid-cols-3">
                            <label className="min-w-0 space-y-1">
                              <span className="block truncate px-1 text-[11px] text-muted-foreground">
                                {t("sheet.capabilitiesQuick.pathColumn")} *
                              </span>
                              <Input
                                aria-invalid={Boolean(rowErrors.path)}
                                className={cn("h-8", rowErrors.path && "border-destructive focus-visible:ring-destructive/30")}
                                value={row.path}
                                placeholder="path.to.option"
                                onChange={(event) => updateParameterRow(row.id, { path: event.target.value })}
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
                                onChange={(event) => updateParameterRow(row.id, { label: event.target.value })}
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
                                onChange={(event) => updateParameterRow(row.id, { description: event.target.value })}
                              />
                            </label>
                            <label className="min-w-0 space-y-1">
                              <span className="block truncate px-1 text-[11px] text-muted-foreground">
                                {t("sheet.capabilitiesQuick.typeColumn")} *
                              </span>
                              <Select
                                value={row.type}
                                onValueChange={(type) => updateParameterType(row.id, type as CapabilityControlType)}
                              >
                                <SelectTrigger
                                  aria-invalid={Boolean(rowErrors.type)}
                                  className={cn("h-8 w-full", rowErrors.type && "border-destructive focus-visible:ring-destructive/30")}
                                >
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
                              {rowErrors.type ? <p className="truncate px-1 text-[10px] text-destructive">{rowErrors.type}</p> : null}
                            </label>
                            <label className="min-w-0 space-y-1">
                              <span className="block truncate px-1 text-[11px] text-muted-foreground">
                                {t("sheet.capabilitiesQuick.optionsColumn")}{row.type === "select" ? " *" : ""}
                              </span>
                              <Input
                                aria-invalid={Boolean(rowErrors.options)}
                                className={cn("h-8", rowErrors.options && "border-destructive focus-visible:ring-destructive/30")}
                                value={row.options}
                                disabled={row.type !== "select"}
                                placeholder={t("sheet.capabilitiesQuick.optionsPlaceholder")}
                                onChange={(event) => updateParameterRow(row.id, { options: event.target.value })}
                              />
                              {rowErrors.options ? <p className="truncate px-1 text-[10px] text-destructive">{rowErrors.options}</p> : null}
                            </label>
                            <label className="min-w-0 space-y-1">
                              <span className="block truncate px-1 text-[11px] text-muted-foreground">
                                {t("sheet.capabilitiesQuick.defaultValueColumn")}
                              </span>
                              <Input
                                className="h-8"
                                value={row.defaultValue}
                                placeholder={'"high", 0.7, true, null, {"key":"value"}'}
                                onChange={(event) => updateParameterRow(row.id, { defaultValue: event.target.value })}
                              />
                            </label>
                          </div>
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            className="mt-5 size-8 justify-self-end text-muted-foreground hover:text-destructive"
                            onClick={() => {
                              setParameterRows((prev) => prev.filter((item) => item.id !== row.id));
                              setParameterErrors((prev) => {
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
                </div>
              )}
            </TabsContent>

            <TabsContent value="tools" className="min-h-0 flex flex-1 flex-col gap-3 overflow-hidden pr-1">
              <div className="flex min-w-0 shrink-0 items-start justify-between gap-3">
                <p className="min-w-0 flex-1 text-xs leading-5 text-muted-foreground">
                  {t("sheet.capabilitiesQuick.toolsIntro")}
                </p>
                <Button
                  type="button"
                  variant="default"
                  size="sm"
                  className="h-7 shrink-0 whitespace-nowrap px-2 text-xs"
                  onClick={addNativeToolRow}
                >
                  <Plus className="size-3.5" />
                  {t("sheet.capabilitiesQuick.addNativeTool")}
                </Button>
              </div>
              {nativeToolRows.length === 0 ? (
                <div className="flex min-h-0 flex-1 flex-col items-center justify-center rounded-md border border-dashed px-3 py-8 text-center">
                  <p className="text-xs text-muted-foreground">{t("sheet.capabilitiesQuick.emptyTools")}</p>
                </div>
              ) : (
                <div className="min-h-0 flex-1 overflow-y-auto rounded-md border border-dashed p-3">
                  <div className="min-w-0 space-y-2">
                    {nativeToolRows.map((row) => {
                      const rowErrors = nativeToolErrors[row.id] ?? {};
                      const protocols = parseNativeToolProtocolsInput(row.protocols);
                      const protocolText = formatNativeToolProtocols(protocols);
                      const protocolMatched = nativeToolMatchesRouteProtocols(protocols, routeProtocolSet);
                      const protocolOptions = nativeToolProtocolSelectOptions(routeProtocols, row.protocols);
                      const displayName = nativeToolDisplayName(row);
                      const expanded = expandedNativeToolID === row.id;
                      return (
                        <div
                          key={row.id}
                          className={cn(
                            "min-w-0 rounded-md border border-l-2 px-2 py-2",
                            protocolMatched ? "border-l-muted-foreground/30 bg-muted/40" : "border-l-transparent bg-muted/20",
                            expanded ? "border-y-border/70 border-r-border/70" : "border-y-transparent border-r-transparent",
                            expanded && "space-y-3",
                            !row.enabled && "text-muted-foreground",
                          )}
                        >
                          <div className="flex min-h-9 min-w-0 items-center justify-between gap-2">
                            <label className="grid min-w-0 flex-1 grid-cols-[auto_minmax(0,1fr)] items-center gap-2">
                              <Checkbox
                                checked={row.enabled}
                                onCheckedChange={(checked) => updateNativeToolRow(row.id, { enabled: checked === true })}
                              />
                              <span className="min-w-0 space-y-0.5">
                                <span className="flex min-w-0 items-center gap-1.5">
                                  <span className="min-w-0 truncate text-xs text-foreground/85">
                                    {displayName.name}
                                  </span>
                                  {row.catalog ? (
                                    <Badge variant="secondary" className="h-5 shrink-0 rounded-md px-1.5 text-[10px] font-normal">
                                      {row.provider || row.key}
                                    </Badge>
                                  ) : null}
                                  {!protocolMatched ? (
                                    <Badge variant="outline" className="h-5 shrink-0 rounded-md px-1.5 text-[10px] font-normal text-amber-700">
                                      {t("sheet.capabilitiesQuick.nativeToolMayNotApply")}
                                    </Badge>
                                  ) : null}
                                  {Object.keys(rowErrors).length > 0 ? (
                                    <span className="size-1.5 shrink-0 rounded-full bg-destructive" aria-hidden="true" />
                                  ) : null}
                                </span>
                                <span className="block min-w-0 truncate font-mono text-[10px] text-muted-foreground">
                                  {displayName.specificName} · {protocolText || "-"}
                                </span>
                              </span>
                            </label>

                            <div className="flex shrink-0 items-center gap-2">
                              <label className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
                                <Switch
                                  size="sm"
                                  checked={row.defaultEnabled}
                                  disabled={!row.enabled}
                                  onCheckedChange={(checked) => updateNativeToolRow(row.id, { defaultEnabled: checked === true })}
                                />
                                {t("sheet.capabilitiesQuick.nativeToolDefaultEnabled")}
                              </label>
                              <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                className="h-7 px-2 text-xs"
                                onClick={() => setExpandedNativeToolID((current) => (current === row.id ? "" : row.id))}
                              >
                                {expanded ? t("sheet.capabilitiesQuick.nativeToolCollapse") : t("sheet.capabilitiesQuick.nativeToolConfigure")}
                              </Button>
                              {!row.catalog ? (
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="icon"
                                  className="size-8 text-muted-foreground hover:text-destructive"
                                  onClick={() => removeNativeToolRow(row.id)}
                                  aria-label={commonT("actions.delete")}
                                >
                                  <Trash2 className="size-3.5" />
                                </Button>
                              ) : null}
                            </div>
                          </div>

                          {expanded ? (
                          <div className="grid min-w-0 grid-cols-1 gap-2 border-t pt-3 sm:grid-cols-3">
                            <label className="min-w-0 space-y-1">
                              <span className="block truncate px-1 text-[11px] text-muted-foreground">
                                {t("sheet.capabilitiesQuick.nativeToolKey")} *
                              </span>
                              <Input
                                className={cn("h-8 bg-transparent", rowErrors.key && "border-destructive focus-visible:ring-destructive/30")}
                                value={row.key}
                                disabled={row.catalog}
                                placeholder="anthropic.web_search_20260209"
                                onChange={(event) => updateNativeToolRow(row.id, { key: event.target.value })}
                              />
                              {rowErrors.key ? <p className="truncate px-1 text-[10px] text-destructive">{rowErrors.key}</p> : null}
                            </label>
                            <label className="min-w-0 space-y-1">
                              <span className="block truncate px-1 text-[11px] text-muted-foreground">
                                {t("sheet.capabilitiesQuick.nativeToolType")} *
                              </span>
                              <Input
                                className={cn("h-8 bg-transparent", rowErrors.type && "border-destructive focus-visible:ring-destructive/30")}
                                value={row.type}
                                disabled={row.catalog}
                                placeholder="web_search_20260209"
                                onChange={(event) => updateNativeToolRow(row.id, { type: event.target.value })}
                              />
                              {rowErrors.type ? <p className="truncate px-1 text-[10px] text-destructive">{rowErrors.type}</p> : null}
                            </label>
                            <label className="min-w-0 space-y-1">
                              <span className="block truncate px-1 text-[11px] text-muted-foreground">
                                {t("sheet.capabilitiesQuick.nativeToolProtocols")} *
                              </span>
                              <NativeToolProtocolsSelect
                                value={row.protocols}
                                options={protocolOptions}
                                invalid={Boolean(rowErrors.protocols)}
                                placeholder={t("sheet.capabilitiesQuick.nativeToolProtocolsPlaceholder")}
                                onChange={(protocols) => updateNativeToolRow(row.id, { protocols })}
                              />
                              {rowErrors.protocols ? <p className="truncate px-1 text-[10px] text-destructive">{rowErrors.protocols}</p> : null}
                            </label>
                            <label className="min-w-0 space-y-1">
                              <span className="block truncate px-1 text-[11px] text-muted-foreground">
                                {t("sheet.capabilitiesQuick.labelColumn")}
                              </span>
                              <Input
                                className="h-8 bg-transparent"
                                value={row.label}
                                placeholder={t("sheet.capabilitiesQuick.labelPlaceholder")}
                                onChange={(event) => updateNativeToolRow(row.id, { label: event.target.value })}
                              />
                            </label>
                            <label className="min-w-0 space-y-1">
                              <span className="block truncate px-1 text-[11px] text-muted-foreground">
                                {t("sheet.capabilitiesQuick.descriptionColumn")}
                              </span>
                              <Input
                                className="h-8 bg-transparent"
                                value={row.description}
                                placeholder={t("sheet.capabilitiesQuick.descriptionPlaceholder")}
                                onChange={(event) => updateNativeToolRow(row.id, { description: event.target.value })}
                              />
                            </label>
                            <label className="min-w-0 space-y-1 sm:col-span-3">
                              <span className="block truncate px-1 text-[11px] text-muted-foreground">
                                {t("sheet.capabilitiesQuick.nativeToolPayload")} *
                              </span>
                              <textarea
                                className={cn(
                                  "min-h-20 w-full resize-y rounded-md border bg-transparent px-2 py-2 font-mono text-xs outline-none focus-visible:ring-2 focus-visible:ring-ring/40",
                                  rowErrors.payload && "border-destructive focus-visible:ring-destructive/30",
                                )}
                                value={row.payload}
                                spellCheck={false}
                                onChange={(event) => updateNativeToolRow(row.id, { payload: event.target.value })}
                              />
                              {rowErrors.payload ? <p className="truncate px-1 text-[10px] text-destructive">{rowErrors.payload}</p> : null}
                            </label>
                          </div>
                          ) : null}
                        </div>
                      );
                    })}
                  </div>
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
