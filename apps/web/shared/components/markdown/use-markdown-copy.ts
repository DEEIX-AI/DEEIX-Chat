"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { useCopyAction } from "@/shared/components/copy-action";
import { CODE_BLOCK_PLAIN_TEXT_MIME } from "@/shared/lib/clipboard";

const LATEX_COPYABLE_SELECTOR = ".katex, .katex-display";
const LATEX_ANNOTATION_SELECTOR = "annotation[encoding='application/x-tex']";
const LATEX_INTERACTION_EXCLUSION_SELECTOR = "a, button, input, textarea, select, summary, pre, code, [contenteditable='true']";
const INLINE_CODE_COPYABLE_SELECTOR = "code:not(pre code)";
const INLINE_CODE_INTERACTION_EXCLUSION_SELECTOR = "a, button, input, textarea, select, summary, [contenteditable='true']";
const CODE_BLOCK_BODY_SELECTOR = "[data-streamdown='code-block-body']";
const MARKDOWN_COPY_POINTER_DRAG_THRESHOLD = 6;

type UseMarkdownCopyOptions = {
  contentVersion: string;
  renderVersion: unknown;
};

type UseMarkdownCopyResult = {
  rootRef: React.RefObject<HTMLDivElement | null>;
  onClickCapture: React.MouseEventHandler<HTMLDivElement>;
  onCopyCapture: React.ClipboardEventHandler<HTMLDivElement>;
  onKeyDownCapture: React.KeyboardEventHandler<HTMLDivElement>;
  onPointerDownCapture: React.PointerEventHandler<HTMLDivElement>;
};

type MarkdownCopyTarget = {
  kind: "inline-code" | "latex";
  source: string;
};

function getHTMLElementFromTarget(target: EventTarget | null): HTMLElement | null {
  if (target instanceof HTMLElement) {
    return target;
  }
  if (target instanceof Node) {
    return target.parentElement;
  }
  return null;
}

function hasNonCollapsedSelection(): boolean {
  const selection = window.getSelection();
  return Boolean(selection && !selection.isCollapsed && selection.toString().trim());
}

function getSelectedCodeText(root: HTMLElement): string {
  const selection = window.getSelection();
  if (!selection || selection.isCollapsed) {
    return "";
  }

  const anchorBlock = getHTMLElementFromTarget(selection.anchorNode)?.closest<HTMLElement>(CODE_BLOCK_BODY_SELECTOR);
  const focusBlock = getHTMLElementFromTarget(selection.focusNode)?.closest<HTMLElement>(CODE_BLOCK_BODY_SELECTOR);
  if (!anchorBlock || anchorBlock !== focusBlock || !root.contains(anchorBlock)) {
    return "";
  }

  return selection.toString();
}

function isDisplayLatexElement(element: HTMLElement): boolean {
  return element.classList.contains("katex-display") || Boolean(element.closest(".katex-display"));
}

function isDelimitedLatexSource(value: string): boolean {
  return value.startsWith("$") || value.startsWith("\\(") || value.startsWith("\\[");
}

function formatLatexSource(source: string, displayMode: boolean): string {
  const trimmedSource = source.trim();
  if (!trimmedSource || isDelimitedLatexSource(trimmedSource)) {
    return trimmedSource;
  }
  return displayMode ? `$$\n${trimmedSource}\n$$` : `$${trimmedSource}$`;
}

function getLatexSource(element: HTMLElement): string {
  const annotation = element.querySelector<HTMLElement>(LATEX_ANNOTATION_SELECTOR);
  return annotation?.textContent?.trim() ?? "";
}

function findLatexCopyElement(target: EventTarget | null, root: HTMLElement): HTMLElement | null {
  const targetElement = getHTMLElementFromTarget(target);
  if (!targetElement || !root.contains(targetElement)) {
    return null;
  }

  if (targetElement.closest(LATEX_INTERACTION_EXCLUSION_SELECTOR)) {
    return null;
  }

  const displayElement = targetElement.closest<HTMLElement>(".katex-display");
  if (displayElement && root.contains(displayElement)) {
    return displayElement;
  }

  const katexElement = targetElement.closest<HTMLElement>(".katex");
  if (katexElement && root.contains(katexElement)) {
    return katexElement;
  }

  return null;
}

function resolveLatexCopySource(target: EventTarget | null, root: HTMLElement): string {
  const copyElement = findLatexCopyElement(target, root);
  if (!copyElement) {
    return "";
  }

  return formatLatexSource(getLatexSource(copyElement), isDisplayLatexElement(copyElement));
}

function resolveInlineCodeCopySource(target: EventTarget | null, root: HTMLElement): string {
  const targetElement = getHTMLElementFromTarget(target);
  const codeElement = targetElement?.closest<HTMLElement>("[data-inline-code-copyable='true']");
  if (!codeElement || !root.contains(codeElement)) {
    return "";
  }
  return codeElement.textContent ?? "";
}

function resolveMarkdownCopyTarget(target: EventTarget | null, root: HTMLElement): MarkdownCopyTarget | null {
  const inlineCodeSource = resolveInlineCodeCopySource(target, root);
  if (inlineCodeSource.trim()) {
    return { kind: "inline-code", source: inlineCodeSource };
  }

  const latexSource = resolveLatexCopySource(target, root);
  return latexSource ? { kind: "latex", source: latexSource } : null;
}

function annotateLatexElements(root: HTMLElement, label: string) {
  const seenElements = new Set<HTMLElement>();

  root.querySelectorAll<HTMLElement>(LATEX_COPYABLE_SELECTOR).forEach((element) => {
    const copyElement = element.closest<HTMLElement>(".katex-display") ?? element;
    if (seenElements.has(copyElement) || !getLatexSource(copyElement)) {
      return;
    }

    seenElements.add(copyElement);
    copyElement.setAttribute("data-latex-copyable", "true");
    copyElement.setAttribute("tabindex", "0");
    copyElement.setAttribute("role", "button");
    copyElement.setAttribute("aria-label", label);
    copyElement.setAttribute("title", label);
  });
}

function annotateInlineCodeElements(root: HTMLElement, label: string) {
  root.querySelectorAll<HTMLElement>(INLINE_CODE_COPYABLE_SELECTOR).forEach((element) => {
    if (!element.textContent?.trim() || element.closest(INLINE_CODE_INTERACTION_EXCLUSION_SELECTOR)) {
      return;
    }

    element.setAttribute("data-inline-code-copyable", "true");
    element.setAttribute("tabindex", "0");
    element.setAttribute("role", "button");
    element.setAttribute("aria-label", label);
    element.setAttribute("title", label);
  });
}

export function useMarkdownCopy({ contentVersion, renderVersion }: UseMarkdownCopyOptions): UseMarkdownCopyResult {
  const t = useTranslations("chat.markdown");
  const commonActions = useTranslations("common.actions");
  const commonErrors = useTranslations("common.errors");
  const { copy } = useCopyAction({
    messages: {
      copied: t("latexCopied"),
      failed: t("latexCopyFailed"),
    },
  });
  const rootRef = React.useRef<HTMLDivElement>(null);
  const pointerDownRef = React.useRef<{ x: number; y: number } | null>(null);

  React.useEffect(() => {
    const root = rootRef.current;
    if (!root) {
      return;
    }
    annotateLatexElements(root, t("copyLatex"));
    annotateInlineCodeElements(root, commonActions("copy"));
  }, [commonActions, contentVersion, renderVersion, t]);

  const copyMarkdownTarget = React.useCallback(
    (target: MarkdownCopyTarget) => {
      if (target.kind === "inline-code") {
        return copy(target.source, {
          copied: commonActions("copied"),
          failed: commonErrors("copyFailed"),
        });
      }
      return copy(target.source);
    },
    [commonActions, commonErrors, copy],
  );

  const onPointerDownCapture = React.useCallback<React.PointerEventHandler<HTMLDivElement>>((event) => {
    if (event.button !== 0) {
      pointerDownRef.current = null;
      return;
    }
    pointerDownRef.current = { x: event.clientX, y: event.clientY };
  }, []);

  const onCopyCapture = React.useCallback<React.ClipboardEventHandler<HTMLDivElement>>((event) => {
    if (event.defaultPrevented) {
      return;
    }

    const code = getSelectedCodeText(event.currentTarget);
    if (!code) {
      return;
    }

    event.clipboardData.clearData();
    event.clipboardData.setData(CODE_BLOCK_PLAIN_TEXT_MIME, "1");
    event.clipboardData.setData("text/plain", code);
    event.preventDefault();
  }, []);

  const onClickCapture = React.useCallback<React.MouseEventHandler<HTMLDivElement>>(
    (event) => {
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.altKey || event.shiftKey) {
        return;
      }

      const pointerDown = pointerDownRef.current;
      if (pointerDown) {
        const deltaX = Math.abs(event.clientX - pointerDown.x);
        const deltaY = Math.abs(event.clientY - pointerDown.y);
        if (deltaX > MARKDOWN_COPY_POINTER_DRAG_THRESHOLD || deltaY > MARKDOWN_COPY_POINTER_DRAG_THRESHOLD) {
          return;
        }
      }

      if (hasNonCollapsedSelection()) {
        return;
      }

      const root = rootRef.current;
      if (!root) {
        return;
      }
      const target = resolveMarkdownCopyTarget(event.target, root);
      if (!target) {
        return;
      }

      event.preventDefault();
      event.stopPropagation();
      void copyMarkdownTarget(target);
    },
    [copyMarkdownTarget],
  );

  const onKeyDownCapture = React.useCallback<React.KeyboardEventHandler<HTMLDivElement>>(
    (event) => {
      if (event.defaultPrevented || (event.key !== "Enter" && event.key !== " ")) {
        return;
      }

      const targetElement = getHTMLElementFromTarget(event.target);
      if (
        !targetElement?.hasAttribute("data-latex-copyable") &&
        !targetElement?.hasAttribute("data-inline-code-copyable")
      ) {
        return;
      }

      const root = rootRef.current;
      if (!root) {
        return;
      }
      const target = resolveMarkdownCopyTarget(event.target, root);
      if (!target) {
        return;
      }

      event.preventDefault();
      event.stopPropagation();
      void copyMarkdownTarget(target);
    },
    [copyMarkdownTarget],
  );

  return {
    rootRef,
    onClickCapture,
    onCopyCapture,
    onKeyDownCapture,
    onPointerDownCapture,
  };
}
