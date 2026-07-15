"use client";

import Image from "next/image";

import { useBranding } from "@/shared/config/branding-provider";
import { useTheme } from "@/shared/components/theme-provider";

type AppLogoProps = {
  alt?: string;
  width: number;
  height: number;
  priority?: boolean;
  className?: string;
};

export function AppLogo({
  alt,
  width,
  height,
  priority,
  className,
}: AppLogoProps) {
  const branding = useBranding();
  const { resolvedTheme } = useTheme();

  return (
    <Image
      src={branding.logoURL || (resolvedTheme === "dark" ? "/logo-white.svg" : "/logo.svg")}
      alt={alt ?? branding.title}
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
