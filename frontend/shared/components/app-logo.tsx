"use client";

import Image from "next/image";

import { useTheme } from "@/shared/components/theme-provider";
import { brandAssets, brandText } from "@/shared/lib/branding";

type AppLogoProps = {
  alt?: string;
  width: number;
  height: number;
  priority?: boolean;
  className?: string;
};

export function AppLogo({
  alt = brandText.title,
  width,
  height,
  priority,
  className,
}: AppLogoProps) {
  const { resolvedTheme } = useTheme();

  return (
    <Image
      src={brandAssets.logo ?? (resolvedTheme === "dark" ? "/logo-white.svg" : "/logo.svg")}
      alt={alt}
      width={width}
      height={height}
      priority={priority}
      className={className}
    />
  );
}

export function DeeixLogo({
  alt = "DEEIX Chat",
  width,
  height,
  priority,
  className,
}: AppLogoProps) {
  const { resolvedTheme } = useTheme();

  return (
    <Image
      src={resolvedTheme === "dark" ? "/logo-white.svg" : "/logo.svg"}
      alt={alt}
      width={width}
      height={height}
      priority={priority}
      className={className}
    />
  );
}
