"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { isDesktopApp } from "@/shared/platform";
import { onDesktopDownloadFinished } from "@/shared/platform/desktop-downloads";

// Desktop only: the shell saves downloads straight to the Downloads folder without
// any UI, so without this a download that worked looks exactly like one that did not.
export function DesktopDownloadNotifier(): null {
  const t = useTranslations("desktopDownloads");
  const tRef = React.useRef(t);
  tRef.current = t;

  React.useEffect(() => {
    if (!isDesktopApp()) {
      return;
    }
    return onDesktopDownloadFinished(({ fileName, success }) => {
      if (success) {
        toast.success(tRef.current("saved", { fileName }));
      } else {
        toast.error(tRef.current("failed", { fileName }));
      }
    });
  }, []);

  return null;
}
