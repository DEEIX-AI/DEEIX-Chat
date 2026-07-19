import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "export",
  experimental: {
    // The Fragment-ref scroll handler in Next 16.3 Preview can crash during
    // consecutive client redirects. Keep the stable handler until upstream fixes it.
    appNewScrollHandler: false,
    useTypeScriptCli: true,
  },
  images: {
    unoptimized: true,
  },
};

export default nextConfig;
