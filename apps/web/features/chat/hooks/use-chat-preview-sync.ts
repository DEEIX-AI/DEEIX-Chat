"use client";

import * as React from "react";

type MarkdownScrollAnchor = {
  line: number;
  top: number;
};

type MarkdownScrollDirection = "preview" | "source";

type UseChatPreviewSyncArgs = {
  enabled: boolean;
  previewRef: React.RefObject<HTMLDivElement | null>;
  source: string;
  textareaRef: React.RefObject<HTMLTextAreaElement | null>;
};

function interpolateMarkdownLine(scrollTop: number, anchors: MarkdownScrollAnchor[]): number {
  let low = 0;
  let high = anchors.length - 1;
  while (low < high) {
    const middle = Math.floor((low + high) / 2);
    if (anchors[middle].top < scrollTop) {
      low = middle + 1;
    } else {
      high = middle;
    }
  }

  const next = anchors[Math.max(low, 1)];
  const current = anchors[Math.max(low - 1, 0)];
  const span = next.top - current.top;
  if (span <= 0) {
    return next.line;
  }
  return current.line + ((scrollTop - current.top) / span) * (next.line - current.line);
}

function interpolateMarkdownScrollTop(line: number, anchors: MarkdownScrollAnchor[]): number {
  let low = 0;
  let high = anchors.length - 1;
  while (low < high) {
    const middle = Math.floor((low + high) / 2);
    if (anchors[middle].line < line) {
      low = middle + 1;
    } else {
      high = middle;
    }
  }

  const next = anchors[Math.max(low, 1)];
  const current = anchors[Math.max(low - 1, 0)];
  const span = next.line - current.line;
  if (span <= 0) {
    return next.top;
  }
  return current.top + ((line - current.line) / span) * (next.top - current.top);
}

function normalizeMarkdownAnchors(anchors: MarkdownScrollAnchor[]): MarkdownScrollAnchor[] {
  const normalized = anchors.sort((left, right) => left.line - right.line);
  let previousTop = 0;
  for (const anchor of normalized) {
    anchor.top = Math.max(anchor.top, previousTop);
    previousTop = anchor.top;
  }
  return normalized;
}

function collectMarkdownPreviewAnchors(preview: HTMLElement, source: string): MarkdownScrollAnchor[] {
  const lineCount = Math.max(source.split("\n").length, 1);
  const scrollRange = Math.max(preview.scrollHeight - preview.clientHeight, 0);
  const previewRect = preview.getBoundingClientRect();
  const lineTopMap = new Map<number, number>([[1, 0]]);

  for (const element of preview.querySelectorAll<HTMLElement>("[data-markdown-source-line]")) {
    const line = Number(element.dataset.markdownSourceLine);
    if (!Number.isFinite(line) || line < 1 || line > lineCount) {
      continue;
    }
    const top = Math.min(
      Math.max(element.getBoundingClientRect().top - previewRect.top + preview.scrollTop, 0),
      scrollRange,
    );
    lineTopMap.set(line, Math.min(lineTopMap.get(line) ?? top, top));
  }

  const anchors = Array.from(lineTopMap, ([line, top]) => ({ line, top }));
  anchors.push({ line: lineCount + 1, top: scrollRange });
  return normalizeMarkdownAnchors(anchors);
}

function createTextareaMirror(
  textarea: HTMLTextAreaElement,
  styles: CSSStyleDeclaration,
): HTMLDivElement {
  const mirror = document.createElement("div");
  mirror.style.position = "fixed";
  mirror.style.left = "-100000px";
  mirror.style.top = "0";
  mirror.style.visibility = "hidden";
  mirror.style.pointerEvents = "none";
  mirror.style.overflow = "hidden";
  mirror.style.whiteSpace = "pre-wrap";
  mirror.style.overflowWrap = styles.overflowWrap;
  mirror.style.wordBreak = styles.wordBreak;
  mirror.style.boxSizing = "border-box";
  mirror.style.width = `${
    textarea.clientWidth +
    (Number.parseFloat(styles.borderLeftWidth) || 0) +
    (Number.parseFloat(styles.borderRightWidth) || 0)
  }px`;
  mirror.style.padding = styles.padding;
  mirror.style.border = styles.border;
  mirror.style.font = styles.font;
  mirror.style.fontFamily = styles.fontFamily;
  mirror.style.fontSize = styles.fontSize;
  mirror.style.fontWeight = styles.fontWeight;
  mirror.style.letterSpacing = styles.letterSpacing;
  mirror.style.lineHeight = styles.lineHeight;
  mirror.style.tabSize = styles.tabSize;
  mirror.style.textIndent = styles.textIndent;
  mirror.style.textTransform = styles.textTransform;
  mirror.style.wordSpacing = styles.wordSpacing;
  return mirror;
}

function resolveMarkdownLineOffsets(source: string): number[] {
  const offsets = [0];
  for (let index = 0; index < source.length; index += 1) {
    if (source[index] === "\n") {
      offsets.push(index + 1);
    }
  }
  return offsets;
}

function collectMarkdownSourceAnchors(
  textarea: HTMLTextAreaElement,
  source: string,
  previewAnchors: MarkdownScrollAnchor[],
): MarkdownScrollAnchor[] {
  const lineOffsets = resolveMarkdownLineOffsets(source);
  const lineCount = Math.max(lineOffsets.length, 1);
  const scrollRange = Math.max(textarea.scrollHeight - textarea.clientHeight, 0);
  const sourceLines = Array.from(
    new Set([1, ...previewAnchors.map((anchor) => anchor.line).filter((line) => line <= lineCount)]),
  ).sort((left, right) => left - right);
  const styles = window.getComputedStyle(textarea);
  const mirror = createTextareaMirror(textarea, styles);
  const markers: Array<{ line: number; element: HTMLSpanElement }> = [];
  let cursor = 0;

  for (const line of sourceLines) {
    const offset = lineOffsets[line - 1] ?? source.length;
    mirror.append(document.createTextNode(source.slice(cursor, offset)));
    const marker = document.createElement("span");
    marker.textContent = "\u200b";
    mirror.append(marker);
    markers.push({ line, element: marker });
    cursor = offset;
  }
  mirror.append(document.createTextNode(source.slice(cursor) || "\u200b"));
  document.body.append(mirror);

  const mirrorRect = mirror.getBoundingClientRect();
  const contentInset =
    (Number.parseFloat(styles.borderTopWidth) || 0) +
    (Number.parseFloat(styles.paddingTop) || 0);
  const anchors = markers.map(({ line, element }) => ({
    line,
    top: Math.min(
      Math.max(element.getBoundingClientRect().top - mirrorRect.top - contentInset, 0),
      scrollRange,
    ),
  }));
  mirror.remove();
  anchors.push({ line: lineCount + 1, top: scrollRange });
  return normalizeMarkdownAnchors(anchors);
}

export function useChatPreviewSync({
  enabled,
  previewRef,
  source,
  textareaRef,
}: UseChatPreviewSyncArgs) {
  const sourceAnchorsRef = React.useRef<MarkdownScrollAnchor[]>([]);
  const previewAnchorsRef = React.useRef<MarkdownScrollAnchor[]>([]);
  const enabledRef = React.useRef(enabled);
  const sourceRef = React.useRef(source);
  const ignoredScrollTargetRef = React.useRef<HTMLElement | null>(null);
  const ignoredScrollResetFrameRef = React.useRef<number | null>(null);
  const refreshFrameRef = React.useRef<number | null>(null);
  const pendingRefreshDirectionRef = React.useRef<MarkdownScrollDirection>("source");
  enabledRef.current = enabled;
  sourceRef.current = source;

  const syncScroll = React.useCallback(
    (
      scrollSource: HTMLElement,
      scrollTarget: HTMLElement | null,
      sourceAnchors: MarkdownScrollAnchor[],
      targetAnchors: MarkdownScrollAnchor[],
    ) => {
      if (!scrollTarget || sourceAnchors.length < 2 || targetAnchors.length < 2) {
        return;
      }
      const sourceScrollRange = Math.max(scrollSource.scrollHeight - scrollSource.clientHeight, 0);
      const targetScrollRange = Math.max(scrollTarget.scrollHeight - scrollTarget.clientHeight, 0);
      const nextScrollTop =
        scrollSource.scrollTop <= 1
          ? 0
          : scrollSource.scrollTop >= sourceScrollRange - 1
            ? targetScrollRange
            : interpolateMarkdownScrollTop(
                interpolateMarkdownLine(scrollSource.scrollTop, sourceAnchors),
                targetAnchors,
              );
      if (Math.abs(scrollTarget.scrollTop - nextScrollTop) < 1) {
        return;
      }

      ignoredScrollTargetRef.current = scrollTarget;
      scrollTarget.scrollTop = nextScrollTop;
      if (ignoredScrollResetFrameRef.current !== null) {
        window.cancelAnimationFrame(ignoredScrollResetFrameRef.current);
      }
      ignoredScrollResetFrameRef.current = window.requestAnimationFrame(() => {
        ignoredScrollTargetRef.current = null;
        ignoredScrollResetFrameRef.current = null;
      });
    },
    [],
  );

  const refreshAnchors = React.useCallback(
    (direction: MarkdownScrollDirection) => {
      const textarea = textareaRef.current;
      const preview = previewRef.current;
      if (!enabledRef.current || !textarea || !preview) {
        return;
      }
      const currentSource = sourceRef.current;
      const previewAnchors = collectMarkdownPreviewAnchors(preview, currentSource);
      const sourceAnchors = collectMarkdownSourceAnchors(textarea, currentSource, previewAnchors);
      previewAnchorsRef.current = previewAnchors;
      sourceAnchorsRef.current = sourceAnchors;
      if (direction === "preview") {
        syncScroll(preview, textarea, previewAnchors, sourceAnchors);
      } else {
        syncScroll(textarea, preview, sourceAnchors, previewAnchors);
      }
    },
    [previewRef, syncScroll, textareaRef],
  );

  const scheduleRefresh = React.useCallback(
    (direction: MarkdownScrollDirection) => {
      if (direction === "source" || refreshFrameRef.current === null) {
        pendingRefreshDirectionRef.current = direction;
      }
      if (refreshFrameRef.current !== null) {
        return;
      }
      refreshFrameRef.current = window.requestAnimationFrame(() => {
        refreshFrameRef.current = null;
        refreshAnchors(pendingRefreshDirectionRef.current);
      });
    },
    [refreshAnchors],
  );

  const handleScroll = React.useCallback(
    (
      scrollSource: HTMLElement,
      scrollTarget: HTMLElement | null,
      sourceAnchors: MarkdownScrollAnchor[],
      targetAnchors: MarkdownScrollAnchor[],
    ) => {
      if (ignoredScrollTargetRef.current === scrollSource) {
        ignoredScrollTargetRef.current = null;
        if (ignoredScrollResetFrameRef.current !== null) {
          window.cancelAnimationFrame(ignoredScrollResetFrameRef.current);
          ignoredScrollResetFrameRef.current = null;
        }
        return;
      }
      syncScroll(scrollSource, scrollTarget, sourceAnchors, targetAnchors);
    },
    [syncScroll],
  );

  const onPreviewScroll = React.useCallback<React.UIEventHandler<HTMLDivElement>>(
    (event) => {
      handleScroll(
        event.currentTarget,
        textareaRef.current,
        previewAnchorsRef.current,
        sourceAnchorsRef.current,
      );
    },
    [handleScroll, textareaRef],
  );

  const onSourceScroll = React.useCallback<React.UIEventHandler<HTMLTextAreaElement>>(
    (event) => {
      handleScroll(
        event.currentTarget,
        previewRef.current,
        sourceAnchorsRef.current,
        previewAnchorsRef.current,
      );
    },
    [handleScroll, previewRef],
  );

  React.useEffect(() => {
    if (enabled) {
      scheduleRefresh("source");
    } else {
      if (refreshFrameRef.current !== null) {
        window.cancelAnimationFrame(refreshFrameRef.current);
        refreshFrameRef.current = null;
      }
      sourceAnchorsRef.current = [];
      previewAnchorsRef.current = [];
    }
  }, [enabled, scheduleRefresh, source]);

  React.useEffect(() => {
    const preview = previewRef.current;
    const textarea = textareaRef.current;
    if (!enabled || !preview || !textarea) {
      return;
    }

    const resizeObserver =
      typeof ResizeObserver === "undefined"
        ? null
        : new ResizeObserver((entries) => {
            scheduleRefresh(entries.some((entry) => entry.target === textarea) ? "source" : "preview");
          });
    const observePreviewContent = () => {
      if (preview.firstElementChild instanceof HTMLElement) {
        resizeObserver?.observe(preview.firstElementChild);
      }
    };
    resizeObserver?.observe(preview);
    resizeObserver?.observe(textarea);
    observePreviewContent();

    const handlePreviewLayoutChange = () => scheduleRefresh("preview");
    const mutationObserver =
      typeof MutationObserver === "undefined"
        ? null
        : new MutationObserver(() => {
            observePreviewContent();
            scheduleRefresh("preview");
          });
    mutationObserver?.observe(preview, {
      attributes: true,
      attributeFilter: ["class", "hidden", "open", "style"],
      childList: true,
      subtree: true,
    });
    preview.addEventListener("load", handlePreviewLayoutChange, true);
    preview.addEventListener("transitionend", handlePreviewLayoutChange, true);

    scheduleRefresh("source");

    return () => {
      preview.removeEventListener("load", handlePreviewLayoutChange, true);
      preview.removeEventListener("transitionend", handlePreviewLayoutChange, true);
      mutationObserver?.disconnect();
      resizeObserver?.disconnect();
    };
  }, [enabled, previewRef, scheduleRefresh, textareaRef]);

  React.useEffect(() => {
    return () => {
      if (ignoredScrollResetFrameRef.current !== null) {
        window.cancelAnimationFrame(ignoredScrollResetFrameRef.current);
      }
      if (refreshFrameRef.current !== null) {
        window.cancelAnimationFrame(refreshFrameRef.current);
      }
    };
  }, []);

  return { onPreviewScroll, onSourceScroll };
}
