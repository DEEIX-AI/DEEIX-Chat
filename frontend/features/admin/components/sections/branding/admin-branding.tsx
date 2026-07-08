"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { Upload, X } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { SpinnerLabel } from "@/components/ui/spinner";
import { listAdminSettingsByNamespace, patchAdminSettings } from "@/features/admin/api/settings";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import type { SettingItem } from "@/shared/api/settings.types";

type BrandingSettings = {
  site_name: string;
  site_title: string;
  favicon_url: string;
  logo_light_url: string;
  logo_dark_url: string;
  theme_color: string;
};

const DEFAULT_SETTINGS: BrandingSettings = {
  site_name: "DEEIX Chat",
  site_title: "DEEIX Chat",
  favicon_url: "",
  logo_light_url: "",
  logo_dark_url: "",
  theme_color: "#0f172a",
};

function settingsToForm(items: SettingItem[]): BrandingSettings {
  const result = { ...DEFAULT_SETTINGS };
  for (const item of items) {
    if (item.key in result) {
      result[item.key as keyof BrandingSettings] = item.value || DEFAULT_SETTINGS[item.key as keyof BrandingSettings];
    }
  }
  return result;
}

export function AdminBrandingPage() {
  const t = useTranslations("adminBranding");
  const tActions = useTranslations("common.actions");
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [form, setForm] = React.useState<BrandingSettings>(DEFAULT_SETTINGS);
  const [faviconPreview, setFaviconPreview] = React.useState<string | null>(null);
  const [logoLightPreview, setLogoLightPreview] = React.useState<string | null>(null);
  const [logoDarkPreview, setLogoDarkPreview] = React.useState<string | null>(null);

  React.useEffect(() => {
    void loadSettings();
  }, []);

  async function loadSettings() {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"));
        return;
      }
      const items = await listAdminSettingsByNamespace(token, "branding");
      setForm(settingsToForm(items));
    } catch (error) {
      toast.error(t("toast.loadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setLoading(false);
    }
  }

  async function handleSave(event: React.FormEvent) {
    event.preventDefault();
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"));
        return;
      }

      await patchAdminSettings(token, {
        items: Object.entries(form).map(([key, value]) => ({
          namespace: "branding",
          key,
          value: value.trim(),
        })),
      });

      toast.success(t("toast.saved"));

      // 刷新页面以应用新的品牌设置
      setTimeout(() => {
        window.location.reload();
      }, 1000);
    } catch (error) {
      toast.error(t("toast.saveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }

  function setField(key: keyof BrandingSettings, value: string) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  function handleImageUpload(key: keyof BrandingSettings, file: File) {
    const reader = new FileReader();
    reader.onloadend = () => {
      const dataUrl = reader.result as string;
      setField(key, dataUrl);

      // 设置预览
      if (key === "favicon_url") {
        setFaviconPreview(dataUrl);
      } else if (key === "logo_light_url") {
        setLogoLightPreview(dataUrl);
      } else if (key === "logo_dark_url") {
        setLogoDarkPreview(dataUrl);
      }
    };
    reader.readAsDataURL(file);
  }

  function clearImage(key: keyof BrandingSettings) {
    setField(key, "");
    if (key === "favicon_url") {
      setFaviconPreview(null);
    } else if (key === "logo_light_url") {
      setLogoLightPreview(null);
    } else if (key === "logo_dark_url") {
      setLogoDarkPreview(null);
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <SpinnerLabel>{t("loading")}</SpinnerLabel>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">{t("title")}</h2>
        <p className="text-sm text-muted-foreground">{t("description")}</p>
      </div>

      <form onSubmit={handleSave} className="space-y-6">
        {/* 网站名称 */}
        <div className="space-y-2">
          <Label htmlFor="site_name">{t("siteName")}</Label>
          <Input
            id="site_name"
            value={form.site_name}
            onChange={(e) => setField("site_name", e.target.value)}
            placeholder="DEEIX Chat"
            disabled={saving}
          />
          <p className="text-xs text-muted-foreground">{t("siteNameHint")}</p>
        </div>

        {/* 网站标题 */}
        <div className="space-y-2">
          <Label htmlFor="site_title">{t("siteTitle")}</Label>
          <Input
            id="site_title"
            value={form.site_title}
            onChange={(e) => setField("site_title", e.target.value)}
            placeholder="DEEIX Chat"
            disabled={saving}
          />
          <p className="text-xs text-muted-foreground">{t("siteTitleHint")}</p>
        </div>

        {/* Favicon */}
        <div className="space-y-2">
          <Label>{t("favicon")}</Label>
          <div className="flex items-center gap-3">
            {(faviconPreview || form.favicon_url) ? (
              <div className="relative">
                <img
                  src={faviconPreview || form.favicon_url}
                  alt="Favicon"
                  className="size-12 rounded border"
                />
                <button
                  type="button"
                  onClick={() => clearImage("favicon_url")}
                  className="absolute -right-2 -top-2 rounded-full bg-destructive p-1 text-destructive-foreground shadow-sm hover:bg-destructive/90"
                  disabled={saving}
                >
                  <X className="size-3" />
                </button>
              </div>
            ) : (
              <div className="flex size-12 items-center justify-center rounded border border-dashed">
                <Upload className="size-5 text-muted-foreground" />
              </div>
            )}
            <div className="flex-1">
              <Input
                type="file"
                accept="image/*"
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) handleImageUpload("favicon_url", file);
                }}
                disabled={saving}
                className="text-sm"
              />
              <p className="mt-1 text-xs text-muted-foreground">{t("faviconHint")}</p>
            </div>
          </div>
        </div>

        {/* Logo 浅色 */}
        <div className="space-y-2">
          <Label>{t("logoLight")}</Label>
          <div className="flex items-center gap-3">
            {(logoLightPreview || form.logo_light_url) ? (
              <div className="relative">
                <img
                  src={logoLightPreview || form.logo_light_url}
                  alt="Logo Light"
                  className="h-12 w-auto rounded border bg-white p-2"
                />
                <button
                  type="button"
                  onClick={() => clearImage("logo_light_url")}
                  className="absolute -right-2 -top-2 rounded-full bg-destructive p-1 text-destructive-foreground shadow-sm hover:bg-destructive/90"
                  disabled={saving}
                >
                  <X className="size-3" />
                </button>
              </div>
            ) : (
              <div className="flex h-12 w-24 items-center justify-center rounded border border-dashed">
                <Upload className="size-5 text-muted-foreground" />
              </div>
            )}
            <div className="flex-1">
              <Input
                type="file"
                accept="image/*"
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) handleImageUpload("logo_light_url", file);
                }}
                disabled={saving}
                className="text-sm"
              />
              <p className="mt-1 text-xs text-muted-foreground">{t("logoLightHint")}</p>
            </div>
          </div>
        </div>

        {/* Logo 深色 */}
        <div className="space-y-2">
          <Label>{t("logoDark")}</Label>
          <div className="flex items-center gap-3">
            {(logoDarkPreview || form.logo_dark_url) ? (
              <div className="relative">
                <img
                  src={logoDarkPreview || form.logo_dark_url}
                  alt="Logo Dark"
                  className="h-12 w-auto rounded border bg-slate-900 p-2"
                />
                <button
                  type="button"
                  onClick={() => clearImage("logo_dark_url")}
                  className="absolute -right-2 -top-2 rounded-full bg-destructive p-1 text-destructive-foreground shadow-sm hover:bg-destructive/90"
                  disabled={saving}
                >
                  <X className="size-3" />
                </button>
              </div>
            ) : (
              <div className="flex h-12 w-24 items-center justify-center rounded border border-dashed">
                <Upload className="size-5 text-muted-foreground" />
              </div>
            )}
            <div className="flex-1">
              <Input
                type="file"
                accept="image/*"
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) handleImageUpload("logo_dark_url", file);
                }}
                disabled={saving}
                className="text-sm"
              />
              <p className="mt-1 text-xs text-muted-foreground">{t("logoDarkHint")}</p>
            </div>
          </div>
        </div>

        {/* 主题色 */}
        <div className="space-y-2">
          <Label htmlFor="theme_color">{t("themeColor")}</Label>
          <div className="flex items-center gap-3">
            <Input
              id="theme_color"
              type="color"
              value={form.theme_color}
              onChange={(e) => setField("theme_color", e.target.value)}
              disabled={saving}
              className="h-10 w-20 cursor-pointer"
            />
            <Input
              value={form.theme_color}
              onChange={(e) => setField("theme_color", e.target.value)}
              placeholder="#0f172a"
              disabled={saving}
              className="flex-1"
            />
          </div>
          <p className="text-xs text-muted-foreground">{t("themeColorHint")}</p>
        </div>

        {/* 提交按钮 */}
        <div className="flex justify-end gap-3">
          <Button
            type="button"
            variant="outline"
            onClick={() => void loadSettings()}
            disabled={saving}
          >
            {tActions("reset")}
          </Button>
          <Button type="submit" disabled={saving}>
            {saving ? <SpinnerLabel>{tActions("saving")}</SpinnerLabel> : tActions("save")}
          </Button>
        </div>
      </form>
    </div>
  );
}
