"use client";

import * as React from "react";

import { cn } from "@/lib/utils";
import { containsMarkdownMath } from "./streamdown-content";
import {
  sanitizeHTMLStyle,
  sanitizeKatexHTMLStyle,
} from "./streamdown-style";

type MarkdownHTMLBlockProps = React.HTMLAttributes<HTMLElement> & {
  children?: React.ReactNode;
  node?: unknown;
};

type MarkdownHTMLInlineProps = React.HTMLAttributes<HTMLSpanElement> & {
  children?: React.ReactNode;
  node?: unknown;
};

type MarkdownHTMLDetailsProps = React.DetailsHTMLAttributes<HTMLDetailsElement> & {
  children?: React.ReactNode;
  node?: unknown;
};

type MarkdownHTMLInlineRenderer = (source: string) => React.ReactNode;

export const MarkdownHTMLInlineRendererContext = React.createContext<MarkdownHTMLInlineRenderer | null>(null);
export const MarkdownFootnotesContext = React.createContext(false);

const INLINE_MARKDOWN_STRONG_RE = /(\*\*|__)([^\n]+?)\1/g;
const INLINE_MARKDOWN_SOURCE_MAX_LENGTH = 64_000;

const KATEX_SPAN_CLASS_NAMES = [
  "katex",
  "katex-display",
  "katex-html",
  "katex-mathml",
  "base",
  "strut",
  "mord",
  "mop",
  "mbin",
  "mrel",
  "mopen",
  "mclose",
  "mpunct",
  "minner",
  "msupsub",
  "vlist",
  "vlist-t",
  "vlist-r",
  "vlist-s",
  "pstrut",
  "sizing",
  "mtight",
  "mspace",
  "mfrac",
  "frac-line",
  "mathrm",
  "mathnormal",
  "mathit",
  "mathbf",
  "textbf",
  "textrm",
  "mainrm",
] as const;

function isKatexSpan(className: string | undefined, style: React.CSSProperties | undefined): boolean {
  if (typeof style?.top !== "undefined") {
    return true;
  }
  const classNames = className?.trim().split(/\s+/) ?? [];
  return classNames.some((item) => (
    KATEX_SPAN_CLASS_NAMES.includes(item as (typeof KATEX_SPAN_CLASS_NAMES)[number]) ||
    /^reset-size\d+$/.test(item) ||
    /^size\d+$/.test(item)
  ));
}

function getPlainInlineText(node: React.ReactNode): string | null {
  let text = "";
  let plain = true;

  React.Children.forEach(node, (child) => {
    if (!plain || child == null || typeof child === "boolean") {
      return;
    }
    if (typeof child === "string" || typeof child === "number") {
      text += String(child);
      plain = text.length <= INLINE_MARKDOWN_SOURCE_MAX_LENGTH;
      return;
    }
    if (React.isValidElement<{ children?: React.ReactNode }>(child) && child.type === React.Fragment) {
      const fragmentText = getPlainInlineText(child.props.children);
      if (fragmentText == null) {
        plain = false;
        return;
      }
      text += fragmentText;
      plain = text.length <= INLINE_MARKDOWN_SOURCE_MAX_LENGTH;
      return;
    }
    plain = false;
  });

  return plain ? text : null;
}

function renderInlineStrongText(source: string): React.ReactNode {
  if (source.length > INLINE_MARKDOWN_SOURCE_MAX_LENGTH || (!source.includes("**") && !source.includes("__"))) {
    return source;
  }

  const nodes: React.ReactNode[] = [];
  let cursor = 0;
  INLINE_MARKDOWN_STRONG_RE.lastIndex = 0;
  for (const match of source.matchAll(INLINE_MARKDOWN_STRONG_RE)) {
    const [raw, _delimiter, content] = match;
    if (!content.trim() || match.index == null) {
      continue;
    }
    if (match.index > cursor) {
      nodes.push(source.slice(cursor, match.index));
    }
    nodes.push(
      <strong
        key={`strong-${match.index}`}
        className="font-bold text-foreground"
        style={{ fontWeight: "var(--font-chat-strong-weight)" }}
      >
        {content}
      </strong>,
    );
    cursor = match.index + raw.length;
  }

  if (cursor === 0) {
    return source;
  }
  if (cursor < source.length) {
    nodes.push(source.slice(cursor));
  }
  return nodes;
}

function renderInlineStrongChildren(children: React.ReactNode): React.ReactNode {
  return React.Children.map(children, (child) => {
    if (typeof child === "string" || typeof child === "number") {
      return renderInlineStrongText(String(child));
    }
    if (!React.isValidElement<{ children?: React.ReactNode }>(child) || !("children" in child.props)) {
      return child;
    }
    return React.cloneElement(child, undefined, renderInlineStrongChildren(child.props.children));
  });
}

function useHTMLMarkdownChildren(children: React.ReactNode): React.ReactNode {
  const renderInlineMarkdown = React.useContext(MarkdownHTMLInlineRendererContext);
  const source = React.useMemo(() => getPlainInlineText(children), [children]);

  if (source != null && !source.includes("\n") && containsMarkdownMath(source)) {
    return renderInlineMarkdown?.(source) ?? children;
  }
  return renderInlineStrongChildren(children);
}

export function MarkdownHTMLDiv({ children, className, node: _node, style }: MarkdownHTMLBlockProps) {
  const renderedChildren = useHTMLMarkdownChildren(children);
  return (
    <div className={cn("min-w-0 max-w-full", className)} style={sanitizeHTMLStyle(style)}>
      {renderedChildren}
    </div>
  );
}

export function MarkdownHTMLSection({ children, className, node: _node, style, ...props }: MarkdownHTMLBlockProps) {
  const renderedChildren = useHTMLMarkdownChildren(children);
  const footnotes = "data-footnotes" in props || className?.split(/\s+/).includes("footnotes") === true;
  const section = (
    <section {...props} className={cn("min-w-0 max-w-full", className)} style={sanitizeHTMLStyle(style)}>
      {renderedChildren}
    </section>
  );

  return footnotes ? <MarkdownFootnotesContext.Provider value>{section}</MarkdownFootnotesContext.Provider> : section;
}

export function MarkdownHTMLArticle({ children, className, node: _node, style }: MarkdownHTMLBlockProps) {
  const renderedChildren = useHTMLMarkdownChildren(children);
  return (
    <article className={cn("min-w-0 max-w-full", className)} style={sanitizeHTMLStyle(style)}>
      {renderedChildren}
    </article>
  );
}

export function MarkdownHTMLAside({ children, className, node: _node, style }: MarkdownHTMLBlockProps) {
  const renderedChildren = useHTMLMarkdownChildren(children);
  return (
    <aside className={cn("min-w-0 max-w-full", className)} style={sanitizeHTMLStyle(style)}>
      {renderedChildren}
    </aside>
  );
}

export function MarkdownHTMLMain({ children, className, node: _node, style }: MarkdownHTMLBlockProps) {
  const renderedChildren = useHTMLMarkdownChildren(children);
  return (
    <main className={cn("min-w-0 max-w-full", className)} style={sanitizeHTMLStyle(style)}>
      {renderedChildren}
    </main>
  );
}

export function MarkdownHTMLDetails({ children, className, node: _node, open, style }: MarkdownHTMLDetailsProps) {
  const renderedChildren = useHTMLMarkdownChildren(children);
  return (
    <details className={cn("min-w-0 max-w-full", className)} open={open} style={sanitizeHTMLStyle(style)}>
      {renderedChildren}
    </details>
  );
}

export function MarkdownHTMLSummary({ children, className, node: _node, style }: MarkdownHTMLBlockProps) {
  return (
    <summary className={cn("min-w-0 max-w-full", className)} style={sanitizeHTMLStyle(style)}>
      {children}
    </summary>
  );
}

export function MarkdownHTMLSpan({ children, className, node: _node, style }: MarkdownHTMLInlineProps) {
  if (isKatexSpan(className, style)) {
    return (
      <span className={className} style={sanitizeKatexHTMLStyle(style)}>
        {children}
      </span>
    );
  }

  return (
    <span className={cn("min-w-0 max-w-full", className)} style={sanitizeHTMLStyle(style)}>
      {children}
    </span>
  );
}
