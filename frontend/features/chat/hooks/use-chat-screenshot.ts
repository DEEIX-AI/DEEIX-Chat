"use client";

import * as React from "react";
import { toast } from "sonner";

import {
  captureElementToPngBlob,
  ConversationScreenshotTooLargeError,
  copyPngBlobToClipboard,
  downloadPngBlob,
  isClipboardImageWriteSupported,
  resolveConversationScreenshotFileName,
} from "@/features/chat/model/conversation-screenshot";

export type ChatScreenshotMessages = {
  emptySelection: string;
  generating: string;
  ready: string;
  failed: string;
  loadLimitReached: string;
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
  onLoadAllMessages?: (options?: { maxPages?: number }) => Promise<boolean>;
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
  let screenshotRoots = [target];

  target.dataset.screenshotCapturing = "true";

  if (selectedOnly) {
    const selectedRows: HTMLElement[] = [];
    const rows = target.querySelectorAll<HTMLElement>("[data-message-public-id]");
    rows.forEach((row) => {
      const publicID = row.dataset.messagePublicId ?? "";
      if (selectedIDs.has(publicID)) {
        selectedRows.push(row);
        return;
      }

      restoreDisplays.push({ element: row, display: row.style.display });
      restoreExcludeAttributes.push({ element: row, value: row.getAttribute("data-screenshot-exclude") });
      row.style.display = "none";
      row.setAttribute("data-screenshot-exclude", "true");
    });
    screenshotRoots = selectedRows;
  }

  forEachElementInRoots(screenshotRoots, ".chat-user-message-collapsible", (element) => {
    restoreMaxHeights.push({ element, maxHeight: element.style.maxHeight });
    element.style.maxHeight = "none";
  });

  forEachElementInRoots(screenshotRoots, ".chat-message-meta", (element) => {
    restoreMetaDisplays.push({ element, display: element.style.display });
    element.style.display = "none";
  });

  forEachElementInRoots(screenshotRoots, "[data-screenshot-only='true']", (element) => {
    restoreScreenshotOnlyDisplays.push({ element, display: element.style.display });
    element.style.display = "flex";
  });

  if (selectedOnly) {
    target.querySelectorAll<HTMLElement>(".chat-screenshot-brand").forEach((element) => {
      restoreScreenshotOnlyDisplays.push({ element, display: element.style.display });
      element.style.display = "flex";
    });
  }

  if (selectedOnly) {
    forEachElementInRoots(screenshotRoots, ".chat-screenshot-selectable-content", (content) => {
      restorePaddings.push({ element: content, paddingLeft: content.style.paddingLeft });
      content.style.paddingLeft = "0px";
    });
  }

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

const MAX_SCREENSHOT_LOAD_PAGES = 50;

export function useChatScreenshot({
  conversationID,
  messageContentRef,
  conversationTitle,
  onLoadAllMessages,
  messages,
}: UseChatScreenshotOptions) {
  const [selectionMode, setSelectionMode] = React.useState(false);
  const [selectedIDs, setSelectedIDs] = React.useState<Set<string>>(() => new Set());
  const [capturing, setCapturing] = React.useState(false);
  const capturingRef = React.useRef(false);
  const captureRunIDRef = React.useRef(0);
  const [preview, setPreview] = React.useState<ChatScreenshotPreview | null>(null);
  const previewRef = React.useRef<ChatScreenshotPreview | null>(null);

  const messagesRef = React.useRef(messages);
  messagesRef.current = messages;
  const titleRef = React.useRef(conversationTitle);
  titleRef.current = conversationTitle;
  const conversationIDRef = React.useRef(conversationID);
  conversationIDRef.current = conversationID;

  React.useEffect(() => {
    previewRef.current = preview;
  }, [preview]);

  React.useEffect(() => {
    captureRunIDRef.current += 1;
    setSelectionMode(false);
    setSelectedIDs(new Set());
    capturingRef.current = false;
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
        next.add(publicID);
      }
      return next;
    });
  }, []);

  const selectMany = React.useCallback((publicIDs: string[]) => {
    setSelectedIDs(new Set(publicIDs.filter(Boolean)));
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
      if (capturingRef.current) {
        return;
      }
      const selected = selectedIDs;
      const startedConversationID = conversationIDRef.current;
      const captureRunID = captureRunIDRef.current + 1;
      if (selectedOnly && selected.size === 0) {
        toast.error(messagesRef.current.emptySelection);
        return;
      }

      captureRunIDRef.current = captureRunID;
      capturingRef.current = true;
      setCapturing(true);
      const loadingToast = toast.loading(messagesRef.current.generating);
      let preparedDom: PreparedScreenshotDom | null = null;
      const cancelCapture = () => {
        toast.dismiss(loadingToast);
      };
      const isCurrentCapture = () =>
        captureRunIDRef.current === captureRunID && startedConversationID === conversationIDRef.current;

      try {
        if (!selectedOnly && onLoadAllMessages) {
          const loadedAll = await onLoadAllMessages({ maxPages: MAX_SCREENSHOT_LOAD_PAGES });
          if (!isCurrentCapture()) {
            cancelCapture();
            return;
          }
          if (!loadedAll) {
            toast.error(messagesRef.current.loadLimitReached, { id: loadingToast });
            return;
          }
        }

        if (!isCurrentCapture()) {
          cancelCapture();
          return;
        }

        await nextAnimationFrame();

        const target = messageContentRef.current;
        if (!target) {
          throw new Error("Message content is not available");
        }

        preparedDom = prepareConversationScreenshotDom(target, {
          selectedOnly,
          selectedIDs: selected,
        });

        await nextAnimationFrame();

        if (!isCurrentCapture()) {
          cancelCapture();
          return;
        }

        const blob = await captureElementToPngBlob(target);
        if (!isCurrentCapture()) {
          cancelCapture();
          return;
        }
        setPreviewBlob(blob);
        toast.success(messagesRef.current.ready, { id: loadingToast });
        if (selectedOnly) {
          exitSelectionMode();
        }
      } catch (error) {
        toast.error(messagesRef.current.failed, {
          id: loadingToast,
          description:
            error instanceof ConversationScreenshotTooLargeError
              ? messagesRef.current.tooLarge
              : error instanceof Error
                ? error.message
                : undefined,
        });
      } finally {
        preparedDom?.restore();
        if (captureRunIDRef.current === captureRunID) {
          capturingRef.current = false;
          setCapturing(false);
        }
      }
    },
    [exitSelectionMode, messageContentRef, onLoadAllMessages, selectedIDs, setPreviewBlob],
  );

  const captureFullConversation = React.useCallback(() => {
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
    captureFullConversation,
    captureSelectedMessages,
    closePreview,
    downloadPreview,
    copyPreviewToClipboard,
  };
}
