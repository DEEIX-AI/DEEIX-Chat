"use client";

import * as React from "react";

const SCREENSHOT_PREVIEW_CLOSE_DELAY_MS = 220;

/**
 * 截图预览对话框的开合状态：有预览时自动打开；
 * 关闭时先收起对话框，延迟释放预览资源，避免退场动画期间图片闪烁。
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
