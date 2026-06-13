"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { useCopyAction } from "@/shared/components/copy-action";

const LATEX_COPYABLE_SELECTOR = ".katex, .katex-display";
const LATEX_ANNOTATION_SELECTOR = "annotation[encoding='application/x-tex']";
const LATEX_INTERACTION_EXCLUSION_SELECTOR = "a, button, input, textarea, select, summary, pre, code, [contenteditable='true']";
const LATEX_POINTER_DRAG_THRESHOLD = 6;

type UseLatexCopyOptions = {
  contentVersion: string;
  renderVersion: unknown;
};

type UseLatexCopyResult = {
  rootRef: React.RefObject<HTMLDivElement | null>;
  onClickCapture: React.MouseEventHandler<HTMLDivElement>;
  onKeyDownCapture: React.KeyboardEventHandler<HTMLDivElement>;
  onPointerDownCapture: React.PointerEventHandler<HTMLDivElement>;
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

export function useLatexCopy({ contentVersion, renderVersion }: UseLatexCopyOptions): UseLatexCopyResult {
  const t = useTranslations("chat.markdown");
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
  }, [contentVersion, renderVersion, t]);

  const copyLatexFromTarget = React.useCallback(
    async (target: EventTarget | null): Promise<boolean> => {
      const root = rootRef.current;
      if (!root) {
        return false;
      }

      const source = resolveLatexCopySource(target, root);
      if (!source) {
        return false;
      }

      return copy(source);
    },
    [copy],
  );

  const onPointerDownCapture = React.useCallback<React.PointerEventHandler<HTMLDivElement>>((event) => {
    if (event.button !== 0) {
      pointerDownRef.current = null;
      return;
    }
    pointerDownRef.current = { x: event.clientX, y: event.clientY };
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
        if (deltaX > LATEX_POINTER_DRAG_THRESHOLD || deltaY > LATEX_POINTER_DRAG_THRESHOLD) {
          return;
        }
      }

      if (hasNonCollapsedSelection()) {
        return;
      }

      const root = rootRef.current;
      if (!root || !resolveLatexCopySource(event.target, root)) {
        return;
      }

      event.preventDefault();
      event.stopPropagation();
      void copyLatexFromTarget(event.target);
    },
    [copyLatexFromTarget],
  );

  const onKeyDownCapture = React.useCallback<React.KeyboardEventHandler<HTMLDivElement>>(
    (event) => {
      if (event.defaultPrevented || (event.key !== "Enter" && event.key !== " ")) {
        return;
      }

      const targetElement = getHTMLElementFromTarget(event.target);
      if (!targetElement?.hasAttribute("data-latex-copyable")) {
        return;
      }

      event.preventDefault();
      event.stopPropagation();
      void copyLatexFromTarget(event.target);
    },
    [copyLatexFromTarget],
  );

  return {
    rootRef,
    onClickCapture,
    onKeyDownCapture,
    onPointerDownCapture,
  };
}
