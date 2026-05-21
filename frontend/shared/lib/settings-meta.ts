import type { SettingsGrouped } from "@/shared/api/settings.types";

export function configuredSettingsMap(grouped: SettingsGrouped): Record<string, boolean> {
  const result: Record<string, boolean> = {};
  for (const [namespace, items] of Object.entries(grouped)) {
    for (const item of items) {
      result[`${namespace}.${item.key}`] = Boolean(item.configured);
    }
  }
  return result;
}

export function settingHasValue(settings: Record<string, string>, configured: Record<string, boolean>, key: string): boolean {
  return Boolean(settings[key]?.trim()) || Boolean(configured[key]);
}
