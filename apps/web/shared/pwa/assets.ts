import { pwaAssetManifest } from "@/shared/generated/pwa-assets";

type PWAAssetPath = keyof typeof pwaAssetManifest;

export function pwaAsset(path: string): string {
  return pwaAssetManifest[path as PWAAssetPath] ?? path;
}
