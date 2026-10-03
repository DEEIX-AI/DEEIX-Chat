import {
  type ModelNativeToolConfig,
  type NativeToolDefinition,
  nativeToolDefinitionVariantsFromConfig,
  nativeToolPayloadSignature,
  resolveModelOptionPolicyProtocol,
} from "@/entities/model";
import type { ConversationOptions } from "@/shared/api/conversation-types";

// Provider-native tools (web search, code execution…) enabled per model by the administrator. The
// user's choice travels in `options.tools`; the backend re-validates it against the model config.

export type NativeToolVisualOption = {
  primary: NativeToolDefinition;
  variants: NativeToolDefinition[];
  protocols: string[];
};

export function isPlainOptionObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function providerToolObjectsFromOptions(options: ConversationOptions): Record<string, unknown>[] {
  const rawTools = options.tools;
  if (!Array.isArray(rawTools)) {
    return [];
  }
  return rawTools.filter(isPlainOptionObject);
}

function providerToolMatchesDefinition(tool: Record<string, unknown>, definition: NativeToolDefinition): boolean {
  const toolType = typeof tool.type === "string" ? tool.type.trim() : "";
  if (toolType) {
    return toolType === definition.type;
  }
  return Object.keys(definition.payload ?? {}).some((key) => key !== "type" && Object.hasOwn(tool, key));
}

function nativeToolDefinitionsFromKeys(
  toolKeys: string[],
  catalog: NativeToolDefinition[],
): NativeToolDefinition[] {
  const allowedKeys = new Set(toolKeys.map((key) => key.trim()).filter(Boolean));
  return catalog.filter((tool) => allowedKeys.has(tool.toolKey.trim()));
}

export function nativeToolVisualOptionsFromConfigs(
  configs: ModelNativeToolConfig[],
  fallbackToolKeys: string[],
  catalog: NativeToolDefinition[],
  modelProtocols: string[],
): NativeToolVisualOption[] {
  const sourceConfigs = configs.length > 0
    ? configs
    : nativeToolDefinitionsFromKeys(fallbackToolKeys, catalog).map((tool): ModelNativeToolConfig => ({
      id: `${tool.protocol}:${tool.toolKey}:${tool.type}`,
      key: tool.toolKey,
      protocol: tool.protocol,
      protocols: [tool.protocol],
      provider: tool.provider,
      type: tool.type,
      label: tool.label,
      description: tool.description,
      enabled: true,
      defaultEnabled: false,
      payload: tool.payload,
    }));
  const visualOptions = new Map<string, NativeToolVisualOption>();
  sourceConfigs.forEach((config) => {
    if (!config.enabled) {
      return;
    }
    const definitions = nativeToolDefinitionVariantsFromConfig(config, catalog, modelProtocols);
    if (definitions.length === 0) {
      return;
    }
    definitions.forEach((definition) => {
      const visualKey = definition.type.trim()
        || definition.toolKey.trim()
        || definition.provider.trim()
        || `payload:${nativeToolPayloadSignature(definition.payload)}`;
      const existing = visualOptions.get(visualKey);
      if (!existing) {
        visualOptions.set(visualKey, {
          primary: definition,
          variants: [definition],
          protocols: [definition.protocol].filter(Boolean),
        });
        return;
      }
      const variantSignature = `${resolveModelOptionPolicyProtocol(definition.protocol)}:${nativeToolPayloadSignature(definition.payload)}`;
      const hasVariant = existing.variants.some((candidate) =>
        `${resolveModelOptionPolicyProtocol(candidate.protocol)}:${nativeToolPayloadSignature(candidate.payload)}` === variantSignature
      );
      if (!hasVariant) {
        existing.variants.push(definition);
      }
      existing.protocols = Array.from(new Set([...existing.protocols, definition.protocol].filter(Boolean)));
    });
  });
  return Array.from(visualOptions.values());
}

function providerToolMatchesAnyDefinition(
  value: unknown,
  definitions: NativeToolDefinition[],
): boolean {
  if (!isPlainOptionObject(value)) {
    return false;
  }
  return definitions.some((definition) => providerToolMatchesDefinition(value, definition));
}

export function ignoredProviderToolValues(
  value: unknown,
  definitions: NativeToolDefinition[],
): unknown[] {
  if (value === undefined) {
    return [];
  }
  if (!Array.isArray(value)) {
    return [value];
  }
  return value.filter((item) => !providerToolMatchesAnyDefinition(item, definitions));
}

export function hasProviderTool(options: ConversationOptions, definitions: NativeToolDefinition[]): boolean {
  return providerToolObjectsFromOptions(options).some((tool) =>
    definitions.some((definition) => providerToolMatchesDefinition(tool, definition))
  );
}

export function setProviderToolEnabled(
  options: ConversationOptions,
  definitions: NativeToolDefinition[],
  enabled: boolean,
): ConversationOptions {
  const tools = providerToolObjectsFromOptions(options);
  const matchesTool = (tool: Record<string, unknown>) =>
    definitions.some((definition) => providerToolMatchesDefinition(tool, definition));
  const nextTools = tools.filter((tool) => !matchesTool(tool));
  if (enabled) {
    const seenPayloads = new Set<string>();
    for (const definition of definitions) {
      const payload = Object.keys(definition.payload).length > 0
        ? definition.payload
        : { type: definition.type };
      const signature = nativeToolPayloadSignature(payload);
      if (seenPayloads.has(signature)) {
        continue;
      }
      seenPayloads.add(signature);
      nextTools.push({ ...payload });
    }
  }

  if (nextTools.length === 0) {
    const { tools: _tools, ...rest } = options;
    return rest;
  }

  return { ...options, tools: nextTools };
}
