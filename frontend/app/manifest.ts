import type { MetadataRoute } from "next";

import { brandAssets, brandText } from "@/shared/lib/branding";

export const dynamic = "force-static";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: brandText.title,
    short_name: brandText.shortName,
    description: "DEEIX Chat is a multi-model AI conversation workspace.",
    id: "/",
    start_url: "/chat",
    scope: "/",
    display: "standalone",
    background_color: "#ffffff",
    theme_color: "#0f172a",
    orientation: "any",
    categories: ["productivity", "business", "utilities"],
    lang: "en",
    icons: [
      {
        src: brandAssets.pwaIcon192,
        sizes: "192x192",
        type: "image/png",
        purpose: "any",
      },
      {
        src: brandAssets.pwaIcon512,
        sizes: "512x512",
        type: "image/png",
        purpose: "any",
      },
      {
        src: brandAssets.pwaMaskableIcon512,
        sizes: "512x512",
        type: "image/png",
        purpose: "maskable",
      },
    ],
  };
}
