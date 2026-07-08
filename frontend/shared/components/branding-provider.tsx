"use client";

import * as React from "react";

type BrandingConfig = {
  siteName: string;
  siteTitle: string;
  faviconUrl: string;
  logoLightUrl: string;
  logoDarkUrl: string;
  themeColor: string;
};

const DEFAULT_BRANDING: BrandingConfig = {
  siteName: "DEEIX Chat",
  siteTitle: "DEEIX Chat",
  faviconUrl: "",
  logoLightUrl: "",
  logoDarkUrl: "",
  themeColor: "#0f172a",
};

const BrandingContext = React.createContext<BrandingConfig>(DEFAULT_BRANDING);

export function useBranding() {
  return React.useContext(BrandingContext);
}

export function BrandingProvider({ children }: { children: React.ReactNode }) {
  const [branding, setBranding] = React.useState<BrandingConfig>(DEFAULT_BRANDING);

  React.useEffect(() => {
    // 从 API 或 localStorage 加载品牌设置
    const loadBranding = async () => {
      try {
        const response = await fetch("/api/v1/public/branding");
        if (response.ok) {
          const data = await response.json();
          const config: BrandingConfig = {
            siteName: data.site_name || DEFAULT_BRANDING.siteName,
            siteTitle: data.site_title || DEFAULT_BRANDING.siteTitle,
            faviconUrl: data.favicon_url || DEFAULT_BRANDING.faviconUrl,
            logoLightUrl: data.logo_light_url || DEFAULT_BRANDING.logoLightUrl,
            logoDarkUrl: data.logo_dark_url || DEFAULT_BRANDING.logoDarkUrl,
            themeColor: data.theme_color || DEFAULT_BRANDING.themeColor,
          };
          setBranding(config);

          // 动态更新页面标题
          if (config.siteTitle) {
            document.title = config.siteTitle;
          }

          // 动态更新 favicon
          if (config.faviconUrl) {
            updateFavicon(config.faviconUrl);
          }

          // 动态更新主题色
          if (config.themeColor) {
            updateThemeColor(config.themeColor);
          }
        }
      } catch (error) {
        console.error("Failed to load branding config:", error);
      }
    };

    void loadBranding();
  }, []);

  return (
    <BrandingContext.Provider value={branding}>
      {children}
    </BrandingContext.Provider>
  );
}

function updateFavicon(url: string) {
  // 移除现有的 favicon
  const existingLinks = document.querySelectorAll("link[rel*='icon']");
  existingLinks.forEach(link => link.remove());

  // 添加新的 favicon
  const link = document.createElement("link");
  link.rel = "icon";
  link.href = url;
  document.head.appendChild(link);
}

function updateThemeColor(color: string) {
  // 更新或创建 theme-color meta 标签
  let metaThemeColor = document.querySelector("meta[name='theme-color']");
  if (!metaThemeColor) {
    metaThemeColor = document.createElement("meta");
    metaThemeColor.setAttribute("name", "theme-color");
    document.head.appendChild(metaThemeColor);
  }
  metaThemeColor.setAttribute("content", color);
}
