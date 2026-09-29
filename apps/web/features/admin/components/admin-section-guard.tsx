"use client";

import { usePathname } from "next/navigation";
import type * as React from "react";

import { ADMIN_SECTIONS, DEFAULT_ADMIN_SECTION, resolveAdminSectionFromPath } from "@/features/admin/model/admin-sections";
import { useSectionGuard } from "@/shared/capabilities";

const fallbackHref = ADMIN_SECTIONS.find((item) => item.id === DEFAULT_ADMIN_SECTION)?.href ?? "";

export function AdminSectionGuard({ basePath, children }: { basePath: string; children: React.ReactNode }) {
  const pathname = usePathname();
  const blocked = useSectionGuard(resolveAdminSectionFromPath(pathname, basePath), `${basePath}${fallbackHref}`);
  return blocked ? null : children;
}
