import type { BrandingResponse } from "@deeix/api-contract";
import { apiRequest } from "@/shared/api/http-client";

export type BrandingDTO = BrandingResponse;

export function getPublicBranding(): Promise<BrandingDTO> {
  return apiRequest<BrandingDTO>("/api/v1/branding");
}
