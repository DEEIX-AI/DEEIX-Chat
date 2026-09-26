import type { ModelNativeToolConfig, NativeToolDefinition } from "@/shared/lib/model-option-policy";
import { resolveModelOptionPolicyProtocol } from "@/shared/lib/model-option-policy";

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

export function nativeToolPayloadSignature(value: unknown): string {
  if (Array.isArray(value)) {
    return `[${value.map((item) => nativeToolPayloadSignature(item)).join(",")}]`;
  }
  if (isPlainObject(value)) {
    return `{${Object.keys(value)
      .sort()
      .map((key) => `${JSON.stringify(key)}:${nativeToolPayloadSignature(value[key])}`)
      .join(",")}}`;
  }
  return JSON.stringify(value) ?? String(value);
}

export function nativeToolPayloadMatchesShape(
  configuredPayload: Record<string, unknown>,
  catalogPayload: Record<string, unknown>,
): boolean {
  const catalogType = typeof catalogPayload.type === "string" ? catalogPayload.type.trim() : "";
  if (catalogType) {
    return typeof configuredPayload.type === "string" && configuredPayload.type.trim() === catalogType;
  }
  const identityKeys = Object.keys(catalogPayload).filter((key) => key !== "type");
  return identityKeys.length > 0 && identityKeys.some((key) => Object.hasOwn(configuredPayload, key));
}

function nativeToolConfigPayloadType(config: ModelNativeToolConfig): string {
  return typeof config.payload.type === "string" ? config.payload.type.trim() : "";
}

function nativeToolDefinitionFromConfig(
  config: ModelNativeToolConfig,
  catalog: NativeToolDefinition[],
  modelProtocols: string[],
): NativeToolDefinition | null {
  const key = config.key.trim();
  const protocols = config.protocols.length > 0 ? config.protocols : (config.protocol.trim() ? [config.protocol.trim()] : []);
  const type = config.type.trim() || nativeToolConfigPayloadType(config);
  const configuredProtocolSet = new Set(protocols.map(resolveModelOptionPolicyProtocol).filter(Boolean));
  const modelProtocolSet = new Set(modelProtocols.map(resolveModelOptionPolicyProtocol).filter(Boolean));
  const matched = (key && configuredProtocolSet.size > 0
    ? catalog.find((tool) => tool.toolKey === key && configuredProtocolSet.has(resolveModelOptionPolicyProtocol(tool.protocol)))
    : undefined)
    ?? (key && modelProtocolSet.size > 0
      ? catalog.find((tool) => tool.toolKey === key && modelProtocolSet.has(resolveModelOptionPolicyProtocol(tool.protocol)))
      : undefined)
    ?? catalog.find((tool) => tool.toolKey === key)
    ?? (type && configuredProtocolSet.size > 0
      ? catalog.find((tool) => tool.type === type && configuredProtocolSet.has(resolveModelOptionPolicyProtocol(tool.protocol)))
      : undefined)
    ?? (type && modelProtocolSet.size > 0
      ? catalog.find((tool) => tool.type === type && modelProtocolSet.has(resolveModelOptionPolicyProtocol(tool.protocol)))
      : undefined)
    ?? (type ? catalog.find((tool) => tool.type === type) : undefined);
  if (!matched && !key && !type && Object.keys(config.payload).length === 0) {
    return null;
  }
  return {
    protocol: matched?.protocol || protocols[0] || "",
    provider: config.provider || matched?.provider || "Provider",
    type: type || matched?.type || key,
    toolKey: key || matched?.toolKey || type,
    label: config.label || matched?.label || type || key,
    description: config.description || matched?.description || type || key,
    payload: Object.keys(config.payload).length > 0 ? config.payload : (matched?.payload ?? {}),
    defaultEnabled: config.defaultEnabled,
    billable: matched?.billable ?? false,
    billingUnit: matched?.billingUnit ?? "",
    priceNanousd: matched?.priceNanousd ?? 0,
    priceLabel: matched?.priceLabel ?? "",
    riskLevel: matched?.riskLevel ?? "",
    usageAliases: matched?.usageAliases ?? [],
  };
}

export function nativeToolDefinitionVariantsFromConfig(
  config: ModelNativeToolConfig,
  catalog: NativeToolDefinition[],
  modelProtocols: string[],
): NativeToolDefinition[] {
  const base = nativeToolDefinitionFromConfig(config, catalog, modelProtocols);
  if (!base) {
    return [];
  }
  const matchingDefinitions = catalog.filter((tool) => tool.toolKey === base.toolKey);
  const explicitProtocols = config.protocols.length > 0
    ? config.protocols
    : [config.protocol].filter(Boolean);
  const modelProtocolSet = new Set(modelProtocols.map(resolveModelOptionPolicyProtocol));
  const inferredProtocols = matchingDefinitions
    .filter((tool) => modelProtocolSet.has(resolveModelOptionPolicyProtocol(tool.protocol)))
    .map((tool) => tool.protocol);
  const sourceProtocols = explicitProtocols.length > 0
    ? explicitProtocols
    : (inferredProtocols.length > 0 ? inferredProtocols : [base.protocol]);
  const protocols = Array.from(new Set(sourceProtocols.map((protocol) => protocol.trim()).filter(Boolean)));
  if (protocols.length === 0) {
    return [base];
  }
  return protocols.map((protocol) => {
    const catalogDefinition = matchingDefinitions.find((tool) =>
      resolveModelOptionPolicyProtocol(tool.protocol) === resolveModelOptionPolicyProtocol(protocol)
    );
    if (!catalogDefinition) {
      return { ...base, protocol };
    }
    const configuredPayload = config.payload;
    const payload = Object.keys(configuredPayload).length > 0
      && nativeToolPayloadMatchesShape(configuredPayload, catalogDefinition.payload)
      ? configuredPayload
      : catalogDefinition.payload;
    return {
      ...catalogDefinition,
      provider: config.provider || catalogDefinition.provider,
      label: config.label || catalogDefinition.label,
      description: config.description || catalogDefinition.description,
      payload,
      defaultEnabled: config.defaultEnabled,
    };
  });
}
