import { apiRequest } from "@/shared/api/http-client";

export type BrandingDTO = {
  title: string;
  shortName: string;
  description: string;
  logoURL: string;
  faviconURL: string;
  pwaIcon192URL: string;
  pwaIcon512URL: string;
  pwaMaskableIcon512URL: string;
  appleTouchIcon180URL: string;
};

export function getPublicBranding(): Promise<BrandingDTO> {
  return apiRequest<BrandingDTO>("/api/v1/branding");
}
