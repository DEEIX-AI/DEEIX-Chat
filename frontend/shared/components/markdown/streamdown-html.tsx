"use client";

import * as React from "react";

import { cn } from "@/lib/utils";
import {
  containsGFMTable,
  containsMarkdownInlineFormatting,
  containsMarkdownMath,
} from "./streamdown-content";
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

type MarkdownHTMLMarkdownRenderer = (source: string) => React.ReactNode;

export const MarkdownHTMLMarkdownRendererContext = React.createContext<MarkdownHTMLMarkdownRenderer | null>(null);

const INLINE_MARKDOWN_STRONG_RE = /(\*\*|__)([\s\S]+?)\1/g;
const HTML_MARKDOWN_SOURCE_MAX_LENGTH = 64_000;
const HTML_MARKDOWN_SOURCE_MAX_LINES = 800;
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

function getPlainReactNodeText(node: React.ReactNode): string | null {
  let text = "";
  let plain = true;

  React.Children.forEach(node, (child) => {
    if (!plain || child == null || typeof child === "boolean") {
      return;
    }

    if (typeof child === "string" || typeof child === "number") {
      text += String(child);
      if (text.length > HTML_MARKDOWN_SOURCE_MAX_LENGTH) {
        plain = false;
      }
      return;
    }

    if (React.isValidElement<{ children?: React.ReactNode }>(child) && child.type === React.Fragment) {
      const fragmentText = getPlainReactNodeText(child.props.children);
      if (fragmentText == null) {
        plain = false;
        return;
      }
      text += fragmentText;
      if (text.length > HTML_MARKDOWN_SOURCE_MAX_LENGTH) {
        plain = false;
      }
      return;
    }

    plain = false;
  });

  return plain ? text : null;
}

function normalizeHTMLMarkdownText(source: string): string {
  const trimmedSource = source.replace(/^\n+|\n+$/g, "");
  const indents = trimmedSource
    .split("\n")
    .filter((line) => line.trim())
    .map((line) => line.match(/^[ \t]*/)?.[0].length ?? 0);
  const minIndent = indents.length > 0 ? Math.min(...indents) : 0;

  if (minIndent <= 0) {
    return trimmedSource;
  }

  return trimmedSource
    .split("\n")
    .map((line) => (line.trim() ? line.slice(minIndent) : line))
    .join("\n");
}

function renderInlineStrongMarkdownText(source: string): React.ReactNode {
  if (source.length > INLINE_MARKDOWN_SOURCE_MAX_LENGTH || !containsMarkdownInlineFormatting(source)) {
    return source;
  }

  const nodes: React.ReactNode[] = [];
  let cursor = 0;
  let match: RegExpExecArray | null;
  INLINE_MARKDOWN_STRONG_RE.lastIndex = 0;

  while ((match = INLINE_MARKDOWN_STRONG_RE.exec(source)) !== null) {
    const [raw, _delimiter, content] = match;
    if (!content.trim()) {
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

function renderHTMLInlineMarkdownChildren(children: React.ReactNode): React.ReactNode {
  return React.Children.map(children, (child) => {
    if (typeof child === "string" || typeof child === "number") {
      return renderInlineStrongMarkdownText(String(child));
    }

    if (!React.isValidElement<{ children?: React.ReactNode }>(child)) {
      return child;
    }

    if (!("children" in child.props)) {
      return child;
    }

    const renderedChildren = renderHTMLInlineMarkdownChildren(child.props.children);
    return React.cloneElement(child, undefined, renderedChildren);
  });
}

function useHTMLMarkdownChildren(children: React.ReactNode): React.ReactNode {
  const renderMarkdown = React.useContext(MarkdownHTMLMarkdownRendererContext);
  const source = React.useMemo(() => getPlainReactNodeText(children), [children]);
  const normalizedSource = React.useMemo(
    () => (source == null ? "" : normalizeHTMLMarkdownText(source)),
    [source],
  );
  const renderedInlineChildren = React.useMemo(() => {
    if (source != null && !containsMarkdownInlineFormatting(source)) {
      return children;
    }
    return renderHTMLInlineMarkdownChildren(children);
  }, [children, source]);

  if (!renderMarkdown || !normalizedSource.trim()) {
    return renderedInlineChildren;
  }

  const sourceLineCount = normalizedSource.split("\n", HTML_MARKDOWN_SOURCE_MAX_LINES + 1).length;
  if (
    normalizedSource.length > HTML_MARKDOWN_SOURCE_MAX_LENGTH ||
    sourceLineCount > HTML_MARKDOWN_SOURCE_MAX_LINES ||
    (!containsGFMTable(normalizedSource) && !containsMarkdownMath(normalizedSource))
  ) {
    return renderedInlineChildren;
  }

  return renderMarkdown(normalizedSource);
}

export function MarkdownHTMLDiv({ children, className, node: _node, style }: MarkdownHTMLBlockProps) {
  const renderedChildren = useHTMLMarkdownChildren(children);
  return (
    <div className={cn("min-w-0 max-w-full", className)} style={sanitizeHTMLStyle(style)}>
      {renderedChildren}
    </div>
  );
}

export function MarkdownHTMLSection({ children, className, node: _node, style }: MarkdownHTMLBlockProps) {
  const renderedChildren = useHTMLMarkdownChildren(children);
  return (
    <section className={cn("min-w-0 max-w-full", className)} style={sanitizeHTMLStyle(style)}>
      {renderedChildren}
    </section>
  );
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
