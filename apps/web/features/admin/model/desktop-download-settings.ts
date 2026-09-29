import { DEFAULT_DESKTOP_DOWNLOAD_URL } from "@/shared/api/settings";
import type { SettingItem } from "@/shared/api/settings-types";

export const DESKTOP_DOWNLOAD_NAMESPACE = "desktop";

export type DesktopDownloadSettingsKey = "download_enabled" | "download_url";

export type DesktopDownloadSettingsField = {
  namespace: typeof DESKTOP_DOWNLOAD_NAMESPACE;
  key: DesktopDownloadSettingsKey;
  label: string;
  description: string;
  type: "bool" | "string";
  placeholder?: string;
};

type DesktopDownloadSettingsTranslator = (key: string) => string;

export function buildDesktopDownloadSettingsFields(t: DesktopDownloadSettingsTranslator): DesktopDownloadSettingsField[] {
  return [
    {
      namespace: DESKTOP_DOWNLOAD_NAMESPACE,
      key: "download_enabled",
      label: t("desktopDownload.enabledLabel"),
      description: t("desktopDownload.enabledDescription"),
      type: "bool",
    },
    {
      namespace: DESKTOP_DOWNLOAD_NAMESPACE,
      key: "download_url",
      label: t("desktopDownload.urlLabel"),
      description: t("desktopDownload.urlDescription"),
      type: "string",
      placeholder: DEFAULT_DESKTOP_DOWNLOAD_URL,
    },
  ];
}

export function desktopDownloadFieldID(field: DesktopDownloadSettingsField): string {
  return `${field.namespace}.${field.key}`;
}

/** Flattens the `desktop` namespace to `desktop.<key>` values, filling registry defaults. */
export function flattenDesktopDownloadSettings(items: SettingItem[] | undefined): Record<string, string> {
  const result: Record<string, string> = {};
  for (const item of items ?? []) {
    result[`${DESKTOP_DOWNLOAD_NAMESPACE}.${item.key}`] = item.value ?? "";
  }
  return {
    ...result,
    "desktop.download_enabled": result["desktop.download_enabled"] === "false" ? "false" : "true",
    "desktop.download_url": result["desktop.download_url"]?.trim() || DEFAULT_DESKTOP_DOWNLOAD_URL,
  };
}

export function toDesktopDownloadEditorField(field: DesktopDownloadSettingsField) {
  return {
    id: desktopDownloadFieldID(field),
    label: field.label,
    description: field.description,
    type: field.type,
    placeholder: field.placeholder,
  } as const;
}
