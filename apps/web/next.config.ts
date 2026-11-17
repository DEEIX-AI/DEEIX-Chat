import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "export",
  // Workspace packages ship TypeScript source; compile them with the app.
  transpilePackages: ["@deeix/core"],
  experimental: {
    useTypeScriptCli: true,
  },
  images: {
    unoptimized: true,
  },
};

export default nextConfig;
