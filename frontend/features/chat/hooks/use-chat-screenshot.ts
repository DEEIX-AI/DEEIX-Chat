"use client";

import * as React from "react";
import { toast } from "sonner";

import {
  ConversationScreenshotTooLargeError,
  captureElementToPngBlob,
  isScreenshotCaptureAbort,
  loadConversationScreenshotRenderer,
  MAX_SCREENSHOT_MESSAGES,
} from "@/features/chat/model/conversation-screenshot";
import {
  copyPngBlobToClipboard,
  downloadPngBlob,
  isClipboardImageWriteSupported,
  resolveConversationScreenshotFileName,
} from "@/features/chat/model/conversation-screenshot-output";

export type ChatScreenshotMessages = {
  emptySelection: string;
  selectionLimitReached: string;
  generating: string;
  ready: string;
  failed: string;
  tooLarge: string;
  downloaded: string;
  copied: string;
  copyFailed: string;
  copyUnsupported: string;
};

type ChatScreenshotPreview = {
  url: string;
  blob: Blob;
  fileName: string;
};

type UseChatScreenshotOptions = {
  conversationID: string | null;
  messageContentRef: React.RefObject<HTMLDivElement | null>;
  conversationTitle: string;
  messages: ChatScreenshotMessages;
};

function nextAnimationFrame() {
  return new Promise<void>((resolve) => {
    window.requestAnimationFrame(() => {
      window.requestAnimationFrame(() => resolve());
    });
  });
}

type PreparedScreenshotDom = {
  restore: () => void;
};

function forEachElementInRoots(
  roots: HTMLElement[],
  selector: string,
  callback: (element: HTMLElement) => void,
) {
  roots.forEach((root) => {
    if (root.matches(selector)) {
      callback(root);
    }
    root.querySelectorAll<HTMLElement>(selector).forEach(callback);
  });
}

function prepareConversationScreenshotDom(
  target: HTMLElement,
  {
    selectedOnly,
    selectedIDs,
  }: {
    selectedOnly: boolean;
    selectedIDs: Set<string>;
  },
): PreparedScreenshotDom {
  const previousCapturing = target.dataset.screenshotCapturing;
  const restoreDisplays: Array<{ element: HTMLElement; display: string }> = [];
  const restoreExcludeAttributes: Array<{ element: HTMLElement; value: string | null }> = [];
  const restorePaddings: Array<{ element: HTMLElement; paddingLeft: string }> = [];
  const restoreMaxHeights: Array<{ element: HTMLElement; maxHeight: string }> = [];
  const restoreMetaDisplays: Array<{ element: HTMLElement; display: string }> = [];
  const restoreScreenshotOnlyDisplays: Array<{ element: HTMLElement; display: string }> = [];
  target.dataset.screenshotCapturing = "true";
  const rows = Array.from(target.querySelectorAll<HTMLElement>("[data-screenshot-message-row='true']"));
  const includedRows = selectedOnly
    ? rows.filter((row) => selectedIDs.has(row.dataset.messagePublicId ?? "")).slice(-MAX_SCREENSHOT_MESSAGES)
    : rows.slice(-MAX_SCREENSHOT_MESSAGES);
  const includedRowSet = new Set(includedRows);

  rows.forEach((row) => {
    if (includedRowSet.has(row)) {
      return;
    }
    restoreDisplays.push({ element: row, display: row.style.display });
    restoreExcludeAttributes.push({ element: row, value: row.getAttribute("data-screenshot-exclude") });
    row.style.display = "none";
    row.setAttribute("data-screenshot-exclude", "true");
  });

  const screenshotRoots = includedRows;

  const mutableElementSelector = [
    ".chat-user-message-collapsible",
    ".chat-message-meta",
    "[data-screenshot-only='true']",
    selectedOnly ? ".chat-screenshot-selectable-content" : "",
  ]
    .filter(Boolean)
    .join(",");
  forEachElementInRoots(screenshotRoots, mutableElementSelector, (element) => {
    if (element.matches(".chat-user-message-collapsible")) {
      restoreMaxHeights.push({ element, maxHeight: element.style.maxHeight });
      element.style.maxHeight = "none";
    }
    if (element.matches(".chat-message-meta")) {
      restoreMetaDisplays.push({ element, display: element.style.display });
      element.style.display = "none";
    }
    if (element.matches("[data-screenshot-only='true']")) {
      restoreScreenshotOnlyDisplays.push({ element, display: element.style.display });
      element.style.display = "flex";
    }
    if (selectedOnly && element.matches(".chat-screenshot-selectable-content")) {
      restorePaddings.push({ element, paddingLeft: element.style.paddingLeft });
      element.style.paddingLeft = "0px";
    }
  });

  target.querySelectorAll<HTMLElement>(".chat-screenshot-brand").forEach((element) => {
    restoreScreenshotOnlyDisplays.push({ element, display: element.style.display });
    element.style.display = "flex";
  });

  return {
    restore: () => {
      if (previousCapturing === undefined) {
        delete target.dataset.screenshotCapturing;
      } else {
        target.dataset.screenshotCapturing = previousCapturing;
      }
      restoreDisplays.forEach(({ element, display }) => {
        element.style.display = display;
      });
      restoreExcludeAttributes.forEach(({ element, value }) => {
        if (value === null) {
          element.removeAttribute("data-screenshot-exclude");
        } else {
          element.setAttribute("data-screenshot-exclude", value);
        }
      });
      restorePaddings.forEach(({ element, paddingLeft }) => {
        element.style.paddingLeft = paddingLeft;
      });
      restoreMaxHeights.forEach(({ element, maxHeight }) => {
        element.style.maxHeight = maxHeight;
      });
      restoreMetaDisplays.forEach(({ element, display }) => {
        element.style.display = display;
      });
      restoreScreenshotOnlyDisplays.forEach(({ element, display }) => {
        element.style.display = display;
      });
    },
  };
}

export function useChatScreenshot({
  conversationID,
  messageContentRef,
  conversationTitle,
  messages,
}: UseChatScreenshotOptions) {
  const [selectionMode, setSelectionMode] = React.useState(false);
  const [selectedIDs, setSelectedIDs] = React.useState<Set<string>>(() => new Set());
  const [capturing, setCapturing] = React.useState(false);
  const captureControllerRef = React.useRef<AbortController | null>(null);
  const [preview, setPreview] = React.useState<ChatScreenshotPreview | null>(null);
  const previewRef = React.useRef<ChatScreenshotPreview | null>(null);

  const messagesRef = React.useRef(messages);
  messagesRef.current = messages;
  const titleRef = React.useRef(conversationTitle);
  titleRef.current = conversationTitle;
  React.useEffect(() => {
    previewRef.current = preview;
  }, [preview]);

  React.useEffect(() => {
    captureControllerRef.current?.abort();
    setSelectionMode(false);
    setSelectedIDs(new Set());
    setCapturing(false);
    setPreview((current) => {
      if (current) {
        URL.revokeObjectURL(current.url);
      }
      return null;
    });
  }, [conversationID]);

  React.useEffect(() => {
    return () => {
      captureControllerRef.current?.abort();
      if (previewRef.current) {
        URL.revokeObjectURL(previewRef.current.url);
      }
    };
  }, []);

  const enterSelectionMode = React.useCallback(() => {
    setSelectedIDs(new Set());
    setSelectionMode(true);
  }, []);

  const exitSelectionMode = React.useCallback(() => {
    setSelectionMode(false);
    setSelectedIDs(new Set());
  }, []);

  const toggleSelection = React.useCallback((publicID: string) => {
    if (!publicID) {
      return;
    }
    setSelectedIDs((previous) => {
      const next = new Set(previous);
      if (next.has(publicID)) {
        next.delete(publicID);
      } else {
        if (next.size >= MAX_SCREENSHOT_MESSAGES) {
          toast.error(messagesRef.current.selectionLimitReached, {
            id: "chat-screenshot-selection-limit",
          });
          return previous;
        }
        next.add(publicID);
      }
      return next;
    });
  }, []);

  const selectMany = React.useCallback((publicIDs: string[]) => {
    setSelectedIDs(new Set(publicIDs.filter(Boolean).slice(-MAX_SCREENSHOT_MESSAGES)));
  }, []);

  const clearSelection = React.useCallback(() => {
    setSelectedIDs(new Set());
  }, []);

  const pruneSelection = React.useCallback((publicIDs: string[]) => {
    const availableIDs = new Set(publicIDs.filter(Boolean));
    setSelectedIDs((previous) => {
      let changed = false;
      const next = new Set<string>();
      previous.forEach((publicID) => {
        if (availableIDs.has(publicID)) {
          next.add(publicID);
        } else {
          changed = true;
        }
      });
      return changed ? next : previous;
    });
  }, []);

  const setPreviewBlob = React.useCallback((blob: Blob) => {
    const fileName = resolveConversationScreenshotFileName(titleRef.current);
    const url = URL.createObjectURL(blob);
    setPreview((current) => {
      if (current) {
        URL.revokeObjectURL(current.url);
      }
      return { url, blob, fileName };
    });
  }, []);

  const runCapture = React.useCallback(
    async (selectedOnly: boolean) => {
      if (captureControllerRef.current) {
        return;
      }
      const selected = selectedIDs;
      if (selectedOnly && selected.size === 0) {
        toast.error(messagesRef.current.emptySelection);
        return;
      }

      const controller = new AbortController();
      captureControllerRef.current = controller;
      setCapturing(true);
      const loadingToast = toast.loading(messagesRef.current.generating);
      let preparedDom: PreparedScreenshotDom | null = null;
      try {
        const target = messageContentRef.current;
        if (!target) {
          throw new Error("Message content is not available");
        }

        const rendererReady = loadConversationScreenshotRenderer();
        preparedDom = prepareConversationScreenshotDom(target, {
          selectedOnly,
          selectedIDs: selected,
        });

        await Promise.all([rendererReady, nextAnimationFrame()]);
        controller.signal.throwIfAborted();

        const blob = await captureElementToPngBlob(target, { signal: controller.signal });
        controller.signal.throwIfAborted();
        setPreviewBlob(blob);
        toast.success(messagesRef.current.ready, { id: loadingToast });
        if (selectedOnly) {
          exitSelectionMode();
        }
      } catch (error) {
        if (isScreenshotCaptureAbort(error)) {
          toast.dismiss(loadingToast);
        } else {
          toast.error(messagesRef.current.failed, {
            id: loadingToast,
            description:
              error instanceof ConversationScreenshotTooLargeError
                ? messagesRef.current.tooLarge
                : error instanceof Error
                  ? error.message
                  : undefined,
          });
        }
      } finally {
        preparedDom?.restore();
        if (captureControllerRef.current === controller) {
          captureControllerRef.current = null;
          setCapturing(false);
        }
      }
    },
    [exitSelectionMode, messageContentRef, selectedIDs, setPreviewBlob],
  );

  const captureLatestMessages = React.useCallback(() => {
    void runCapture(false);
  }, [runCapture]);

  const captureSelectedMessages = React.useCallback(() => {
    void runCapture(true);
  }, [runCapture]);

  const closePreview = React.useCallback(() => {
    setPreview((current) => {
      if (current) {
        URL.revokeObjectURL(current.url);
      }
      return null;
    });
  }, []);

  const downloadPreview = React.useCallback(() => {
    if (!preview) {
      return;
    }
    downloadPngBlob(preview.blob, preview.fileName);
    toast.success(messagesRef.current.downloaded);
  }, [preview]);

  const copyPreviewToClipboard = React.useCallback(async () => {
    if (!preview) {
      return;
    }
    if (!isClipboardImageWriteSupported()) {
      toast.error(messagesRef.current.copyUnsupported);
      return;
    }
    try {
      await copyPngBlobToClipboard(preview.blob);
      toast.success(messagesRef.current.copied);
    } catch (error) {
      toast.error(messagesRef.current.copyFailed, {
        description: error instanceof Error ? error.message : undefined,
      });
    }
  }, [preview]);

  return {
    selectionMode,
    selectedIDs,
    selectedCount: selectedIDs.size,
    capturing,
    preview,
    clipboardSupported: isClipboardImageWriteSupported(),
    exitSelectionMode,
    toggleSelection,
    selectMany,
    clearSelection,
    pruneSelection,
    startSelectionScreenshot: enterSelectionMode,
    captureLatestMessages,
    captureSelectedMessages,
    closePreview,
    downloadPreview,
    copyPreviewToClipboard,
  };
}
