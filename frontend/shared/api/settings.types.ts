export type SettingItem = {
  key: string;
  value: string;
  valueType: "string" | "int" | "bool" | "json";
  description: string;
  sensitive: boolean;
  configured: boolean;
};

export type SettingsGrouped = Record<string, SettingItem[]>;

export type PatchSettingItem = {
  namespace: string;
  key: string;
  value: string;
  clear?: boolean;
};

export type PatchSettingsRequest = {
  items: PatchSettingItem[];
};
