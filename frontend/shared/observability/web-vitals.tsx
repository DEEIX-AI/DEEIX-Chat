"use client";

import * as React from "react";
import { usePathname } from "next/navigation";
import { useReportWebVitals } from "next/web-vitals";

const WEB_VITALS_DEBUG_ENABLED = process.env.NEXT_PUBLIC_WEB_VITALS_DEBUG === "true";

export function WebVitals() {
  if (!WEB_VITALS_DEBUG_ENABLED) {
    return null;
  }

  return <WebVitalsReporter />;
}

function WebVitalsReporter() {
  const pathname = usePathname();

  useReportWebVitals(
    React.useCallback(
      (metric) => {
        console.table({
          route: pathname || "/",
          name: metric.name,
          value: Number(metric.value.toFixed(2)),
          rating: metric.rating,
          delta: Number(metric.delta.toFixed(2)),
          id: metric.id,
          navigationType: metric.navigationType,
        });
      },
      [pathname],
    ),
  );

  return null;
}
