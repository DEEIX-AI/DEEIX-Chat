"use client";

import * as React from "react";

/**
 * Streamdown does not currently expose class-name slots for its download menus,
 * fullscreen Mermaid toolbar or Mermaid loading placeholders. Keep the unavoidable
 * upstream DOM adaptation in one place so version upgrades have a single
 * compatibility surface.
 */
export const StreamdownAdapterStyles = React.memo(function StreamdownAdapterStyles() {
  return (
    <style jsx global>{`
      :is(
          [data-streamdown="mermaid-block-actions"],
          [data-streamdown="table-download-actions"]
        )
        > div
        > div {
        z-index: 50;
        margin-top: 0.375rem;
        min-width: 8rem;
        overflow: hidden;
        border: 0.5px solid var(--border);
        border-radius: 0.75rem;
        background: var(--popover);
        padding: 0.375rem;
        color: var(--popover-foreground);
        font-family: var(--font-sans);
        box-shadow: var(--shadow-xs);
        animation: streamdown-action-menu-enter 150ms ease-out;
      }

      :is(
          [data-streamdown="mermaid-block-actions"],
          [data-streamdown="table-download-actions"]
        )
        > div
        > div
        > button {
        width: 100%;
        border-radius: 0.375rem;
        padding: 0.375rem 0.5rem;
        outline: none;
        font-size: 0.75rem;
        line-height: 1.25rem;
        text-align: left;
      }

      :is(
          [data-streamdown="mermaid-block-actions"],
          [data-streamdown="table-download-actions"]
        )
        > div
        > div
        > button:hover,
      :is(
          [data-streamdown="mermaid-block-actions"],
          [data-streamdown="table-download-actions"]
        )
        > div
        > div
        > button:focus-visible {
        background: color-mix(in oklch, var(--accent) 40%, transparent);
        color: var(--accent-foreground);
      }

      [data-streamdown="mermaid-fullscreen"] > div:first-child {
        gap: 0.5rem;
      }

      [data-streamdown="mermaid-fullscreen"] > div:first-child > button,
      [data-streamdown="mermaid-fullscreen"] > div:first-child > div > button {
        display: inline-flex;
        width: 1.25rem;
        height: 1.25rem;
        align-items: center;
        justify-content: center;
        padding: 0.25rem;
        border: 0;
        border-radius: 0;
        background: transparent;
        color: var(--muted-foreground);
        box-shadow: none;
        transition: color 150ms, background-color 150ms;
      }

      [data-streamdown="mermaid-fullscreen"] > div:first-child > button:hover,
      [data-streamdown="mermaid-fullscreen"] > div:first-child > button:focus-visible,
      [data-streamdown="mermaid-fullscreen"] > div:first-child > div > button:hover,
      [data-streamdown="mermaid-fullscreen"] > div:first-child > div > button:focus-visible {
        background: color-mix(in oklch, var(--foreground) 4%, transparent);
        color: var(--foreground);
      }

      [data-streamdown="mermaid-fullscreen"] > div:first-child button svg {
        width: 0.75rem;
        height: 0.75rem;
      }

      /*
       * Mermaid loading. Before a diagram exists Streamdown cycles through several
       * placeholders (lazy-chunk skeleton, deferred-render spacer, "Loading diagram..."),
       * and shows a parse error while a partial chart streams. They stay laid out (the
       * deferred render waits on an IntersectionObserver) but are hidden behind
       * MarkdownMermaidLoading until the diagram svg, the final error card or the
       * plain-code fallback appears. Without :has() the upstream placeholders show instead.
       */
      [data-markdown-mermaid="pending"] > [data-markdown-mermaid-loading] {
        display: flex;
      }

      @supports selector(:has(*)) {
        [data-markdown-mermaid]:not(
            :has(
              [data-streamdown="mermaid"] [role="img"] > svg,
              [data-markdown-mermaid-error],
              [data-streamdown="code-block"]
            )
          )
          > [data-markdown-mermaid-loading] {
          display: flex;
        }

        [data-markdown-mermaid]:not(
            :has(
              [data-streamdown="mermaid"] [role="img"] > svg,
              [data-markdown-mermaid-error],
              [data-streamdown="code-block"]
            )
          )
          > :not([data-markdown-mermaid-loading]) {
          position: absolute;
          inset: 0;
          overflow: hidden;
          visibility: hidden;
          pointer-events: none;
        }
      }

      [data-markdown-mermaid] [data-streamdown="mermaid"] [role="img"],
      [data-markdown-mermaid-error] {
        animation: markdown-mermaid-enter 180ms ease-out;
      }

      @keyframes markdown-mermaid-enter {
        from {
          opacity: 0;
        }
      }

      @keyframes streamdown-action-menu-enter {
        from {
          opacity: 0;
          transform: translateY(-0.5rem) scale(0.95);
        }
        to {
          opacity: 1;
          transform: translateY(0) scale(1);
        }
      }

      @media (prefers-reduced-motion: reduce) {
        :is(
            [data-streamdown="mermaid-block-actions"],
            [data-streamdown="table-download-actions"]
          )
          > div
          > div {
          animation: none;
        }

        [data-markdown-mermaid] [data-streamdown="mermaid"] [role="img"],
        [data-markdown-mermaid-error] {
          animation: none;
        }
      }
    `}</style>
  );
});
