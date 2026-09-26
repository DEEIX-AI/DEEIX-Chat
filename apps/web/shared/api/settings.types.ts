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
