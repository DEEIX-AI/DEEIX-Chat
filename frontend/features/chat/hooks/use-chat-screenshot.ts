"use client";

import * as React from "react";
import { toast } from "sonner";

import {
  captureElementToPngBlob,
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
  const restoreDisplays: Array<{ element: HTMLElement; display: string }> = [];
  const restorePaddings: Array<{ element: HTMLElement; paddingLeft: string }> = [];
  const restoreMaxHeights: Array<{ element: HTMLElement; maxHeight: string }> = [];

  const collapsibles = target.querySelectorAll<HTMLElement>(".chat-user-message-collapsible");
  collapsibles.forEach((element) => {
    restoreMaxHeights.push({ element, maxHeight: element.style.maxHeight });
    element.style.maxHeight = "none";
  });

  if (selectedOnly) {
    const rows = target.querySelectorAll<HTMLElement>("[data-message-public-id]");
    rows.forEach((row) => {
      const publicID = row.dataset.messagePublicId ?? "";
      if (!selectedIDs.has(publicID)) {
        restoreDisplays.push({ element: row, display: row.style.display });
        row.style.display = "none";
      }
    });

    const contents = target.querySelectorAll<HTMLElement>(".chat-screenshot-selectable-content");
    contents.forEach((content) => {
      restorePaddings.push({ element: content, paddingLeft: content.style.paddingLeft });
      content.style.paddingLeft = "0px";
    });
  }

  return {
    restore: () => {
      restoreDisplays.forEach(({ element, display }) => {
        element.style.display = display;
      });
      restorePaddings.forEach(({ element, paddingLeft }) => {
        element.style.paddingLeft = paddingLeft;
      });
      restoreMaxHeights.forEach(({ element, maxHeight }) => {
        element.style.maxHeight = maxHeight;
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
      if (selectedOnly && selected.size === 0) {
        toast.error(messagesRef.current.emptySelection);
        return;
      }

      capturingRef.current = true;
      setCapturing(true);
      const loadingToast = toast.loading(messagesRef.current.generating);
      let preparedDom: PreparedScreenshotDom | null = null;

      try {
        if (!selectedOnly && onLoadAllMessages) {
          const loadedAll = await onLoadAllMessages({ maxPages: MAX_SCREENSHOT_LOAD_PAGES });
          if (!loadedAll) {
            toast.error(messagesRef.current.loadLimitReached, { id: loadingToast });
            return;
          }
        }

        if (startedConversationID !== conversationIDRef.current) {
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

        if (startedConversationID !== conversationIDRef.current) {
          return;
        }

        const blob = await captureElementToPngBlob(target);
        if (startedConversationID !== conversationIDRef.current) {
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
          description: error instanceof Error ? error.message : undefined,
        });
      } finally {
        preparedDom?.restore();
        capturingRef.current = false;
        setCapturing(false);
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
    startSelectionScreenshot: enterSelectionMode,
    captureFullConversation,
    captureSelectedMessages,
    closePreview,
    downloadPreview,
    copyPreviewToClipboard,
  };
}
