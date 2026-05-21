"use client";

import * as React from "react";
import { usePathname, useSearchParams } from "next/navigation";

import { useSidebar } from "@/components/ui/sidebar";

export function SidebarRouteCloser() {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const { isMobile, openMobile, setOpenMobile } = useSidebar();
  const routeKey = `${pathname}?${searchParams.toString()}`;
  const previousRouteKeyRef = React.useRef(routeKey);

  React.useEffect(() => {
    if (previousRouteKeyRef.current === routeKey) {
      return;
    }

    previousRouteKeyRef.current = routeKey;

    if (isMobile && openMobile) {
      setOpenMobile(false);
    }
  }, [isMobile, openMobile, routeKey, setOpenMobile]);

  return null;
}
