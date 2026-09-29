"use client";

import { usePathname } from "next/navigation";
import type * as React from "react";

import {
  DEFAULT_SETTINGS_SECTION,
  resolveSettingsSectionFromPath,
  SETTINGS_SECTIONS,
} from "@/features/settings/model/settings-sections";
import { useSectionGuard } from "@/shared/capabilities";

const fallbackHref = SETTINGS_SECTIONS.find((item) => item.id === DEFAULT_SETTINGS_SECTION)?.href ?? "";

export function SettingsSectionGuard({ basePath, children }: { basePath: string; children: React.ReactNode }) {
  const pathname = usePathname();
  const blocked = useSectionGuard(resolveSettingsSectionFromPath(pathname, basePath), `${basePath}${fallbackHref}`);
  return blocked ? null : children;
}
