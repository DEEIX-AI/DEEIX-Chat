export type SettingItem = {
  key: string;
  value: string;
  valueType: "string" | "int" | "bool" | "json";
  description: string;
  sensitive: boolean;
  configured: boolean;
};

export type SettingsGrouped = Record<string, SettingItem[]>;

export type PatchSettingItem = Omit<PatchItem, "value"> & { value: string };

export type PatchSettingsRequest = Omit<SettingsPatchSettingsRequest, "items"> & {
  items: PatchSettingItem[];
};
import type { PatchItem, SettingsPatchSettingsRequest } from "@deeix/api-contract";

// Model option policy served by /api/v1/settings/model-option-policy. The rules that
// interpret it live in the model entity (entities/model/lib/model-option-policy).
export type ModelOptionPolicyMode = "allowlist" | "denylist" | "disabled" | string;

export type ModelOptionPolicy = {
  mode: ModelOptionPolicyMode;
  allowedPathsJSON: string;
  deniedPathsJSON: string;
  nativeTools: NativeToolDefinition[];
};

export type NativeToolDefinition = {
  protocol: string;
  provider: string;
  type: string;
  toolKey: string;
  label: string;
  description: string;
  payload: Record<string, unknown>;
  defaultEnabled: boolean;
  billable: boolean;
  billingUnit: string;
  priceNanousd: number;
  priceLabel: string;
  riskLevel: string;
  usageAliases: string[];
};
