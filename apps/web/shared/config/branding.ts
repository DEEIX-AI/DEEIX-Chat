import type { BrandingDTO } from "@/shared/api/branding";
import { pwaAsset } from "@/shared/pwa/assets";

export const DEFAULT_BRANDING: BrandingDTO = {
  title: "DEEIX Chat",
  shortName: "DEEIX",
  description: "DEEIX Chat is a multi-model AI conversation system.",
  logoURL: "",
  faviconURL: "/favicon.ico",
  pwaIcon192URL: pwaAsset("/pwa/icon-192.png"),
  pwaIcon512URL: pwaAsset("/pwa/icon-512.png"),
  pwaMaskableIcon512URL: pwaAsset("/pwa/icon-maskable-512.png"),
  appleTouchIcon180URL: pwaAsset("/pwa/apple-touch-icon.png"),
};

let brandingSnapshot = DEFAULT_BRANDING;

export function getBrandingSnapshot(): BrandingDTO {
  return brandingSnapshot;
}

export function setBrandingSnapshot(branding: BrandingDTO): void {
  brandingSnapshot = branding;
}

export function replaceDefaultBrandTitle(value: string, brandTitle: string): string {
  if (brandTitle === DEFAULT_BRANDING.title) {
    return value;
  }
  return value.replaceAll(DEFAULT_BRANDING.title, () => brandTitle);
}
