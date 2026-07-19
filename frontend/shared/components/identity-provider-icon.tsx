"use client";

import * as React from "react";

import {
  resolveIdentityProviderIconScale,
  resolveIdentityProviderIconURL,
  shouldInvertIdentityProviderIcon,
} from "@/shared/lib/identity-provider-icons";
import { cn } from "@/lib/utils";

export function IdentityProviderIcon({
  name,
  slug,
  logoURL,
  className,
  iconClassName,
  fallbackClassName,
}: {
  name: string;
  slug: string;
  logoURL?: string;
  className?: string;
  iconClassName?: string;
  fallbackClassName?: string;
}) {
  const customLogoURL = logoURL?.trim() ?? "";
  const defaultIconURL = resolveIdentityProviderIconURL(name, slug);
  const iconCandidates = React.useMemo(
    () => [customLogoURL, defaultIconURL].filter((value): value is string => Boolean(value)),
    [customLogoURL, defaultIconURL],
  );
  const iconCandidatesKey = iconCandidates.join("|");
  const [iconIndex, setIconIndex] = React.useState(0);
  React.useEffect(() => {
    setIconIndex(0);
  }, [iconCandidatesKey]);

  const iconUrl = iconCandidates[iconIndex] ?? "";
  const iconScale = resolveIdentityProviderIconScale(name, slug);
  const rootClassName = cn("grid size-4 shrink-0 place-items-center", className);
  const isDefaultIcon = iconUrl === defaultIconURL;
  const invertInDarkMode = isDefaultIcon && shouldInvertIdentityProviderIcon(name, slug);
  if (iconUrl) {
    return (
      <span aria-hidden="true" className={cn(rootClassName, isDefaultIcon && "text-foreground")}>
        <img
          alt=""
          className={cn("block size-4 object-contain", invertInDarkMode && "dark:invert", iconClassName)}
          src={iconUrl}
          style={{ transform: isDefaultIcon ? `scale(${iconScale})` : undefined }}
          onError={() => {
            setIconIndex((current) => Math.min(current + 1, iconCandidates.length));
          }}
        />
      </span>
    );
  }
  return (
    <span className={cn(rootClassName, "text-xs font-medium leading-none text-muted-foreground", fallbackClassName)} aria-hidden="true">
      {name.trim().slice(0, 1) || "S"}
    </span>
  );
}
