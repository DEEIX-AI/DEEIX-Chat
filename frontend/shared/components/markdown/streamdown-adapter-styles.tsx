"use client";

import * as React from "react";

/**
 * Streamdown does not currently expose class-name slots for its download menus
 * or fullscreen Mermaid toolbar. Keep the unavoidable upstream DOM adaptation
 * in one place so version upgrades have a single compatibility surface.
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
      }
    `}</style>
  );
});
