import { pwaAssetManifest } from "@/shared/generated/pwa-assets";

// Widened to a string lookup so arbitrary paths fall back to themselves without a key assertion.
const PWA_ASSET_LOOKUP: Readonly<Record<string, string | undefined>> = pwaAssetManifest;

export function pwaAsset(path: string): string {
  return PWA_ASSET_LOOKUP[path] ?? path;
}
