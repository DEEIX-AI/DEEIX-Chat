"use client";

import * as React from "react";

const SCREENSHOT_PREVIEW_CLOSE_DELAY_MS = 220;

/**
 * Open state of the screenshot preview dialog: opens automatically when a preview exists;
 * on close, the dialog collapses first and the preview is released later, avoiding image flicker during the exit animation.
 */
export function useChatScreenshotPreview({
  preview,
  closePreview,
}: {
  preview: unknown;
  closePreview: () => void;
}) {
  const [screenshotPreviewOpen, setScreenshotPreviewOpen] = React.useState(false);
  const screenshotPreviewCloseTimerRef = React.useRef<number | null>(null);

  const clearScreenshotPreviewCloseTimer = React.useCallback(() => {
    if (screenshotPreviewCloseTimerRef.current === null) {
      return;
    }
    window.clearTimeout(screenshotPreviewCloseTimerRef.current);
    screenshotPreviewCloseTimerRef.current = null;
  }, []);

  React.useEffect(() => {
    if (!preview) {
      setScreenshotPreviewOpen(false);
      return;
    }
    clearScreenshotPreviewCloseTimer();
    setScreenshotPreviewOpen(true);
  }, [clearScreenshotPreviewCloseTimer, preview]);

  React.useEffect(() => clearScreenshotPreviewCloseTimer, [clearScreenshotPreviewCloseTimer]);

  const closeScreenshotPreviewDialog = React.useCallback(() => {
    setScreenshotPreviewOpen(false);
    clearScreenshotPreviewCloseTimer();
    screenshotPreviewCloseTimerRef.current = window.setTimeout(() => {
      screenshotPreviewCloseTimerRef.current = null;
      closePreview();
    }, SCREENSHOT_PREVIEW_CLOSE_DELAY_MS);
  }, [clearScreenshotPreviewCloseTimer, closePreview]);

  return {
    screenshotPreviewOpen,
    closeScreenshotPreviewDialog,
  };
}
