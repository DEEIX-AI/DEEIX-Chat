"use client";

import { cjk } from "@streamdown/cjk";
import { createMathPlugin, type MathPlugin } from "@streamdown/math";
import { useTranslations } from "next-intl";
import * as React from "react";
import {
  type AllowedTags,
  type Components,
  defaultRehypePlugins,
  type IconMap,
  type PluginConfig,
  Streamdown,
  type StreamdownProps,
} from "streamdown";

import { ChevronDown } from "@/components/animate-ui/icons/chevron-down";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Marker, MarkerContent } from "@/components/ui/marker";
import { cn } from "@/lib/utils";
import { useAutoExpandDisclosure } from "@/shared/hooks/use-auto-expand-disclosure";
import {
  AdaptiveMarkdownTable,
  MarkdownTableStreamingContext,
} from "./adaptive-markdown-table";
import { StreamdownAdapterStyles } from "./streamdown-adapter-styles";
import {
  type MarkdownArtifactActions,
  MarkdownArtifactActionsContext,
  MarkdownCodePre,
  MarkdownImage,
  type MarkdownImageActions,
  MarkdownImageActionsContext,
  MarkdownLink,
  MarkdownOrderedList,
  MarkdownParagraph,
  MarkdownStrong,
  MarkdownSup,
  ThinkingHeading,
} from "./streamdown-components";
import {
  containsMarkdownMath,
  normalizeContent,
  normalizeCurrencyDollars,
  normalizeEscapedHTMLAttributeQuotes,
  normalizeHTMLBlockBlankLines,
  normalizeHTMLVisualMarkdownFences,
  normalizeLatexUnicodeSymbols,
  normalizeMathDelimiters,
  normalizeMermaidBlocks,
  parseStreamdownSegments,
  type RenderSegment,
} from "./streamdown-content";
import {
  MarkdownHTMLArticle,
  MarkdownHTMLAside,
  MarkdownHTMLDetails,
  MarkdownHTMLDiv,
  MarkdownHTMLInlineRendererContext,
  MarkdownHTMLMain,
  MarkdownHTMLSection,
  MarkdownHTMLSpan,
  MarkdownHTMLSummary,
} from "./streamdown-html";
import { renderRawHTMLMathRehypePlugin } from "./streamdown-html-math";
import {
  createStreamdownTooltipIcon,
  StreamdownCheckIcon,
  StreamdownCloseIcon,
  StreamdownCopyIcon,
  StreamdownDownloadIcon,
  StreamdownMaximizeIcon,
} from "./streamdown-icons";
import { normalizeBareURLRehypePlugin } from "./streamdown-url-normalize";
import { useMarkdownCopy } from "./use-markdown-copy";

type StreamdownRenderProps = {
  content: unknown;
  className?: string;
  streaming?: boolean;
  variant?: "default" | "thinking" | "user";
  sourcePositions?: boolean;
  autoExpandThinking?: boolean;
  imageActions?: MarkdownImageActions;
  artifactActions?: MarkdownArtifactActions;
};

type StreamdownFeatureFlags = {
  code: boolean;
  math: boolean;
  mermaid: boolean;
};

const BASE_STREAMDOWN_PLUGINS: PluginConfig = {
  cjk,
};
const STREAMDOWN_MATH_PLUGIN = (() => {
  const plugin = createMathPlugin({
    singleDollarTextMath: true,
  });
  if (!Array.isArray(plugin.rehypePlugin)) {
    return plugin;
  }

  const [rehypePlugin, options] = plugin.rehypePlugin;
  return {
    ...plugin,
    rehypePlugin: [
      rehypePlugin,
      {
        ...(typeof options === "object" && options !== null ? options : {}),
        strict: (errorCode: string) => (errorCode === "unicodeTextInMathMode" ? "ignore" : "warn"),
      },
    ] as MathPlugin["rehypePlugin"],
  };
})();
const STREAMDOWN_MATH_BASE_PLUGINS: PluginConfig = {
  ...BASE_STREAMDOWN_PLUGINS,
  math: STREAMDOWN_MATH_PLUGIN,
};

const STREAMDOWN_PLUGIN_CACHE = new Map<string, PluginConfig>();
const STREAMDOWN_PLUGIN_PROMISE_CACHE = new Map<string, Promise<PluginConfig>>();

const STREAMDOWN_CONTROLS = {
  code: {
    copy: false,
    download: false,
  },
  mermaid: {
    copy: true,
    download: true,
    fullscreen: true,
    panZoom: true,
  },
  table: false,
} as const;

function useStreamdownTranslations() {
  const t = useTranslations("chat.markdown");
  return React.useMemo(
    () => ({
      downloadDiagram: t("diagram.downloadDiagram"),
      downloadDiagramAsSvg: t("diagram.downloadDiagramAsSvg"),
      downloadDiagramAsPng: t("diagram.downloadDiagramAsPng"),
      downloadDiagramAsMmd: t("diagram.downloadDiagramAsMmd"),
      copyCode: t("diagram.copyDiagram"),
      viewFullscreen: t("diagram.viewFullscreen"),
      exitFullscreen: t("diagram.exitFullscreen"),
      zoomIn: t("diagram.zoomIn"),
      zoomOut: t("diagram.zoomOut"),
      resetView: t("diagram.resetView"),
      downloadTable: t("table.download"),
      downloadTableAsCsv: t("table.downloadAsCsv"),
      downloadTableAsMarkdown: t("table.downloadAsMarkdown"),
      tableFormatCsv: t("table.formatCsv"),
      tableFormatMarkdown: t("table.formatMarkdown"),
    }),
    [t],
  );
}

function useStreamdownIcons(): Partial<IconMap> {
  const t = useTranslations("chat.markdown.diagram");
  return React.useMemo(
    () => ({
      CheckIcon: createStreamdownTooltipIcon(StreamdownCheckIcon, t("copyDiagram")),
      CopyIcon: createStreamdownTooltipIcon(StreamdownCopyIcon, t("copyDiagram")),
      DownloadIcon: createStreamdownTooltipIcon(StreamdownDownloadIcon, t("downloadDiagram")),
      Maximize2Icon: createStreamdownTooltipIcon(StreamdownMaximizeIcon, t("viewFullscreen")),
      XIcon: createStreamdownTooltipIcon(StreamdownCloseIcon, t("exitFullscreen")),
    }),
    [t],
  );
}

const STREAMDOWN_REMEND = {
  linkMode: "text-only",
} as const;

const STREAMDOWN_CARET = "circle" as const;
const STREAMDOWN_CODE_BLOCK_MAX_HEIGHT = "22rem";
const STREAMDOWN_LINK_SAFETY = { enabled: false } as const;
const STREAMDOWN_SANITIZED_HTML_TAGS = {
  article: ["style"],
  aside: ["style"],
  details: ["open", "style"],
  div: ["style"],
  main: ["style"],
  section: ["style"],
  a: ["href", "title", "style"],
  p: ["style"],
  span: ["style"],
  summary: ["style"],
} satisfies AllowedTags;
type RehypeSanitizeSchema = {
  tagNames?: string[];
  attributes?: Record<string, unknown>;
};
type StreamdownRehypePlugins = NonNullable<StreamdownProps["rehypePlugins"]>;
type StreamdownRehypePlugin = StreamdownRehypePlugins[number];
type RehypeSanitizePlugin = [StreamdownRehypePlugin, RehypeSanitizeSchema];
type RehypeSourceNode = {
  type?: string;
  tagName?: string;
  properties?: Record<string, unknown>;
  position?: { start?: { line?: number } };
  children?: RehypeSourceNode[];
};
const MARKDOWN_SOURCE_BLOCK_TAGS = new Set([
  "blockquote",
  "details",
  "h1",
  "h2",
  "h3",
  "h4",
  "h5",
  "h6",
  "hr",
  "li",
  "ol",
  "p",
  "pre",
  "table",
  "ul",
]);

function markdownSourcePositionRehypePlugin() {
  return (tree: RehypeSourceNode) => {
    const visit = (node: RehypeSourceNode) => {
      const sourceLine = node.position?.start?.line;
      if (node.type === "element" && node.tagName && MARKDOWN_SOURCE_BLOCK_TAGS.has(node.tagName) && sourceLine) {
        node.properties ??= {};
        node.properties["data-markdown-source-line"] = String(sourceLine);
      }
      node.children?.forEach(visit);
    };
    visit(tree);
  };
}

function buildStreamdownRehypePlugins(includeSourcePositions = false): StreamdownRehypePlugins {
  const [sanitizePlugin, sanitizeSchema] = defaultRehypePlugins.sanitize as RehypeSanitizePlugin;
  const extraTagNames = Object.keys(STREAMDOWN_SANITIZED_HTML_TAGS);
  const tagNames = Array.from(new Set([...(sanitizeSchema.tagNames ?? []), ...extraTagNames]));
  const schema = {
    ...sanitizeSchema,
    tagNames,
    attributes: {
      ...sanitizeSchema.attributes,
      ...STREAMDOWN_SANITIZED_HTML_TAGS,
    },
  };

  const sanitizeWithHTMLTags = [sanitizePlugin, schema] as StreamdownRehypePlugin;

  return [
    renderRawHTMLMathRehypePlugin,
    defaultRehypePlugins.raw,
    sanitizeWithHTMLTags,
    ...(includeSourcePositions ? [markdownSourcePositionRehypePlugin] : []),
    normalizeBareURLRehypePlugin,
  ];
}
const STREAMDOWN_REHYPE_PLUGINS = buildStreamdownRehypePlugins();
const SOURCE_POSITION_STREAMDOWN_REHYPE_PLUGINS = buildStreamdownRehypePlugins(true);
const FENCED_CODE_BLOCK_RE = /(?:^|\n)[ \t]*(?:```|~~~)(?!\s*(?:mermaid|mmd)\b)[^\n]*(?:\n|$)/i;
const MERMAID_CODE_BLOCK_RE = /(?:^|\n)[ \t]*(?:```|~~~)\s*(?:mermaid|mmd)\b/i;

const BASE_MARKDOWN_CLASSNAME = cn(
  "chat-font-content min-w-0 max-w-full overflow-hidden leading-6 text-foreground [overflow-wrap:anywhere]",
  "[&>*:last-child]:after:text-muted-foreground/55",
  "[&_p]:min-w-0 [&_p]:max-w-full [&_p]:break-words [&_p]:[overflow-wrap:anywhere]",
  "[&_li]:min-w-0 [&_li]:max-w-full [&_li]:break-words [&_li]:[overflow-wrap:anywhere]",
  "[&_blockquote]:min-w-0 [&_blockquote]:max-w-full [&_blockquote]:break-words [&_blockquote]:[overflow-wrap:anywhere]",
  "[&_[data-streamdown='mermaid-block']]:my-4 [&_[data-streamdown='mermaid-block']]:flex [&_[data-streamdown='mermaid-block']]:!w-full [&_[data-streamdown='mermaid-block']]:min-w-0 [&_[data-streamdown='mermaid-block']]:gap-2 [&_[data-streamdown='mermaid-block']]:rounded-none [&_[data-streamdown='mermaid-block']]:border-0 [&_[data-streamdown='mermaid-block']]:bg-transparent [&_[data-streamdown='mermaid-block']]:p-0 [&_[data-streamdown='mermaid-block']]:shadow-none",
  "[&_[data-streamdown='mermaid-block']>div:last-child]:!w-full [&_[data-streamdown='mermaid-block']>div:last-child]:min-w-0 [&_[data-streamdown='mermaid-block']>div:last-child]:rounded-none [&_[data-streamdown='mermaid-block']>div:last-child]:border-0 [&_[data-streamdown='mermaid-block']>div:last-child]:bg-transparent [&_[data-streamdown='mermaid-block']>div:last-child]:p-0 [&_[data-streamdown='mermaid-block']>div:last-child]:shadow-none",
  "[&_[data-streamdown='mermaid']]:my-0 [&_[data-streamdown='mermaid']]:block [&_[data-streamdown='mermaid']]:!w-full [&_[data-streamdown='mermaid']]:max-h-[280px] [&_[data-streamdown='mermaid']]:min-w-0 [&_[data-streamdown='mermaid']]:overflow-hidden [&_[data-streamdown='mermaid']]:rounded-none [&_[data-streamdown='mermaid']]:border-0 [&_[data-streamdown='mermaid']]:bg-transparent [&_[data-streamdown='mermaid']]:shadow-none",
  "[&_[data-streamdown='mermaid']>div]:!w-full [&_[data-streamdown='mermaid']>div]:max-w-none [&_[data-streamdown='mermaid']>div]:min-w-0",
  "[&_[data-streamdown='mermaid']_svg]:mx-auto [&_[data-streamdown='mermaid']_svg]:block [&_[data-streamdown='mermaid']_svg]:h-auto [&_[data-streamdown='mermaid']_svg]:max-h-[280px] [&_[data-streamdown='mermaid']_svg]:max-w-full [&_[data-streamdown='mermaid']_svg]:bg-transparent",
  "[&_[data-streamdown='mermaid']>div>div:first-child]:!left-0 [&_[data-streamdown='mermaid']>div>div:first-child]:rounded-none [&_[data-streamdown='mermaid']>div>div:first-child]:border-0 [&_[data-streamdown='mermaid']>div>div:first-child]:bg-transparent [&_[data-streamdown='mermaid']>div>div:first-child]:p-0 [&_[data-streamdown='mermaid']>div>div:first-child]:shadow-none [&_[data-streamdown='mermaid']>div>div:first-child]:backdrop-blur-none",
  "[&_[data-streamdown='mermaid-block-actions']]:gap-2 [&_[data-streamdown='mermaid-block-actions']]:border-0 [&_[data-streamdown='mermaid-block-actions']]:rounded-none [&_[data-streamdown='mermaid-block-actions']]:bg-transparent [&_[data-streamdown='mermaid-block-actions']]:p-0 [&_[data-streamdown='mermaid-block-actions']]:shadow-none [&_[data-streamdown='mermaid-block-actions']]:backdrop-blur-none",
  "[&_[data-streamdown='mermaid-block-actions']>button]:border-0 [&_[data-streamdown='mermaid-block-actions']>button]:bg-transparent [&_[data-streamdown='mermaid-block-actions']>button]:shadow-none [&_[data-streamdown='mermaid-block-actions']>button:hover]:bg-foreground/[0.04] [&_[data-streamdown='mermaid-block-actions']>button:hover]:text-foreground",
  "[&_[data-streamdown='mermaid-block-actions']>div>button]:border-0 [&_[data-streamdown='mermaid-block-actions']>div>button]:bg-transparent [&_[data-streamdown='mermaid-block-actions']>div>button]:shadow-none [&_[data-streamdown='mermaid-block-actions']>div>button:hover]:bg-foreground/[0.04] [&_[data-streamdown='mermaid-block-actions']>div>button:hover]:text-foreground",
  "[&_[data-streamdown='mermaid-block-actions']_button:focus-visible]:outline-none [&_[data-streamdown='mermaid-block-actions']_button:focus-visible]:ring-0 [&_[data-streamdown='mermaid-block-actions']_button:focus-visible]:bg-foreground/[0.04] [&_[data-streamdown='mermaid-block-actions']_button:focus-visible]:text-foreground",
  "[&_[data-streamdown='mermaid-block-actions']_svg]:size-3",
  "[&_[data-streamdown='mermaid-block']_button>svg]:size-3",
  "[&_code:not(pre_code)]:rounded-md [&_code:not(pre_code)]:bg-foreground/[0.05] [&_code:not(pre_code)]:px-1.5 [&_code:not(pre_code)]:py-0.5 [&_code:not(pre_code)]:font-mono [&_code:not(pre_code)]:text-[0.85em] [&_code:not(pre_code)]:text-primary [&_code:not(pre_code)]:whitespace-pre-wrap [&_code:not(pre_code)]:break-words [&_code:not(pre_code)]:[overflow-wrap:anywhere]",
  "[&_[data-streamdown='code-block']]:my-4 [&_[data-streamdown='code-block']]:!w-full [&_[data-streamdown='code-block']]:min-w-0 [&_[data-streamdown='code-block']]:gap-0 [&_[data-streamdown='code-block']]:border-0 [&_[data-streamdown='code-block']]:rounded-none [&_[data-streamdown='code-block']]:bg-transparent [&_[data-streamdown='code-block']]:p-0 [&_[data-streamdown='code-block']]:shadow-none [&_[data-streamdown='code-block']]:outline-none [&_[data-streamdown='code-block']]:ring-0",
  // Streamdown's fixed offscreen placeholder changes height when a code block
  // enters the viewport, which can move the surrounding conversation.
  "[&_[data-streamdown='code-block']]:![content-visibility:visible] [&_[data-streamdown='code-block']]:![contain-intrinsic-size:none]",
  "[&_[data-streamdown='code-block']>div:first-child]:min-h-0 [&_[data-streamdown='code-block']>div:first-child]:justify-between [&_[data-streamdown='code-block']>div:first-child]:gap-2 [&_[data-streamdown='code-block']>div:first-child]:border-0 [&_[data-streamdown='code-block']>div:first-child]:bg-transparent [&_[data-streamdown='code-block']>div:first-child]:mt-2 [&_[data-streamdown='code-block']>div:first-child]:pb-6 [&_[data-streamdown='code-block']>div:first-child]:text-[11px] [&_[data-streamdown='code-block']>div:first-child]:font-medium [&_[data-streamdown='code-block']>div:first-child]:tracking-[0.06em] [&_[data-streamdown='code-block']>div:first-child]:text-muted-foreground/85 [&_[data-streamdown='code-block']>div:first-child]:shadow-none",
  "[&_[data-streamdown='code-block']>div:last-child]:!w-full [&_[data-streamdown='code-block']>div:last-child]:min-w-0 [&_[data-streamdown='code-block']>div:last-child]:border-0 [&_[data-streamdown='code-block']>div:last-child]:rounded-none [&_[data-streamdown='code-block']>div:last-child]:bg-transparent [&_[data-streamdown='code-block']>div:last-child]:p-0 [&_[data-streamdown='code-block']>div:last-child]:shadow-none",
  "[&_[data-streamdown='code-block-body']]:!rounded-xl [&_[data-streamdown='code-block-body']]:!border-[0.75rem] [&_[data-streamdown='code-block-body']]:!border-transparent [&_[data-streamdown='code-block-body']]:!bg-muted/40 [&_[data-streamdown='code-block-body']]:!p-0",
  "[&_pre]:group [&_pre]:my-0 [&_pre]:block [&_pre]:!w-full [&_pre]:!min-w-0 [&_pre]:max-w-full [&_pre]:overflow-visible [&_pre]:border-0 [&_pre]:bg-transparent [&_pre]:p-0 [&_pre]:shadow-none [&_pre]:outline-none [&_pre]:ring-0",
  "[&_pre>code]:block [&_pre>code]:w-max [&_pre>code]:min-w-full [&_pre>code]:max-w-none [&_pre>code]:border-0 [&_pre>code]:bg-transparent [&_pre>code]:font-mono [&_pre>code]:text-[12px] [&_pre>code]:leading-5 [&_pre>code]:text-foreground/92 [&_pre>code]:shadow-none [&_pre>code]:outline-none [&_pre>code]:ring-0",
  "[&_pre>code>span]:before:text-[11px]",
  "[&_[data-streamdown='code-block-actions']]:gap-2 [&_[data-streamdown='code-block-actions']]:!opacity-100 [&_[data-streamdown='code-block-actions']]:border-0 [&_[data-streamdown='code-block-actions']]:rounded-none [&_[data-streamdown='code-block-actions']]:bg-transparent [&_[data-streamdown='code-block-actions']]:p-0 [&_[data-streamdown='code-block-actions']]:shadow-none [&_[data-streamdown='code-block-actions']]:backdrop-blur-none",
  "[&_[data-streamdown='code-block-actions']_button]:inline-flex [&_[data-streamdown='code-block-actions']_button]:items-center [&_[data-streamdown='code-block-actions']_button]:justify-center [&_[data-streamdown='code-block-actions']_button]:rounded-none [&_[data-streamdown='code-block-actions']_button]:border-0 [&_[data-streamdown='code-block-actions']_button]:bg-transparent [&_[data-streamdown='code-block-actions']_button]:p-1 [&_[data-streamdown='code-block-actions']_button]:text-muted-foreground [&_[data-streamdown='code-block-actions']_button]:shadow-none [&_[data-streamdown='code-block-actions']_button:hover]:bg-foreground/[0.04] [&_[data-streamdown='code-block-actions']_button:hover]:text-foreground",
  "[&_[data-streamdown='code-block-actions']_svg]:size-3",
  "[&_[data-footnotes]]:mt-6 [&_[data-footnotes]]:border-t [&_[data-footnotes]]:border-foreground/15 [&_[data-footnotes]]:pt-3 [&_[data-footnotes]]:text-[11px] [&_[data-footnotes]]:leading-5 [&_[data-footnotes]]:text-muted-foreground/82",
  "[&_[data-footnotes]_h2]:sr-only",
  "[&_[data-footnotes]_ol]:my-0 [&_[data-footnotes]_ol]:pl-4 [&_[data-footnotes]_ol]:text-[11px] [&_[data-footnotes]_ol]:leading-5",
  "[&_[data-footnotes]_li]:my-0.5 [&_[data-footnotes]_li]:!py-0.5 [&_[data-footnotes]_li]:pl-1 [&_[data-footnotes]_li]:text-[11px] [&_[data-footnotes]_li]:leading-5 [&_[data-footnotes]_li]:text-muted-foreground/82",
  "[&_[data-footnotes]_p]:my-0 [&_[data-footnotes]_p]:text-[11px] [&_[data-footnotes]_p]:leading-5 [&_[data-footnotes]_p]:text-muted-foreground/82",
  "[&_.katex]:text-[1.04em]",
  "[&_.katex-display]:my-3.5 [&_.katex-display]:block [&_.katex-display]:max-w-full [&_.katex-display]:overflow-x-auto [&_.katex-display]:overflow-y-hidden [&_.katex-display]:px-1 [&_.katex-display]:py-1.5 [&_.katex-display]:text-center",
  "[&_.katex-display>.katex]:inline-block [&_.katex-display>.katex]:min-w-fit [&_.katex-display>.katex]:max-w-none [&_.katex-display>.katex]:text-center",
  "[&_.katex_.mfrac_.frac-line]:!inline-block [&_.katex_.mfrac_.frac-line]:!w-full [&_.katex_.mfrac_.frac-line]:min-h-px [&_.katex_.mfrac_.frac-line]:![border-bottom:0.04em_solid_currentColor]",
  "[&_[data-latex-copyable='true']]:cursor-copy [&_[data-latex-copyable='true']]:rounded-sm [&_[data-latex-copyable='true']]:outline-none [&_[data-latex-copyable='true']]:transition-colors",
  "[&_[data-latex-copyable='true']:hover]:bg-foreground/[0.035] [&_[data-latex-copyable='true']:focus-visible]:bg-foreground/[0.045] [&_[data-latex-copyable='true']:focus-visible]:ring-2 [&_[data-latex-copyable='true']:focus-visible]:ring-ring/25",
  "[&_[data-inline-code-copyable='true']]:cursor-copy [&_[data-inline-code-copyable='true']]:outline-none [&_[data-inline-code-copyable='true']]:transition-colors",
  "[&_[data-inline-code-copyable='true']:hover]:bg-foreground/[0.08] [&_[data-inline-code-copyable='true']:focus-visible]:bg-foreground/[0.08] [&_[data-inline-code-copyable='true']:focus-visible]:ring-2 [&_[data-inline-code-copyable='true']:focus-visible]:ring-ring/25",
  "[&_strong]:font-semibold",
);

const THINKING_MARKDOWN_CLASSNAME = cn(
  BASE_MARKDOWN_CLASSNAME,
  "leading-6 text-muted-foreground/84",
  "[&_p]:my-0.25 [&_p]:text-[12px] [&_p]:leading-5 [&_p]:text-muted-foreground/84",
  "[&_li]:text-[12px] [&_li]:leading-5 [&_li]:text-muted-foreground/84",
  "[&_ul]:my-0.5 [&_ul]:pl-4",
  "[&_ol]:my-0.5 [&_ol]:pl-4",
  "[&_h1]:mt-0.5 [&_h1]:mb-0 [&_h1]:text-[12px] [&_h1]:font-medium [&_h1]:leading-5 [&_h1]:text-muted-foreground/88",
  "[&_h2]:mt-0.5 [&_h2]:mb-0 [&_h2]:text-[12px] [&_h2]:font-medium [&_h2]:leading-5 [&_h2]:text-muted-foreground/88",
  "[&_h3]:mt-0.5 [&_h3]:mb-0 [&_h3]:text-[12px] [&_h3]:font-medium [&_h3]:leading-5 [&_h3]:text-muted-foreground/88",
  "[&_h4]:mt-0.5 [&_h4]:mb-0 [&_h4]:text-[12px] [&_h4]:font-medium [&_h4]:leading-5 [&_h4]:text-muted-foreground/88",
  "[&_strong]:font-semibold [&_strong]:text-foreground",
  "[&_em]:italic [&_em]:text-foreground/92",
  "[&_blockquote]:my-0.5 [&_blockquote]:border-l-0 [&_blockquote]:pl-0 [&_blockquote]:text-[12px] [&_blockquote]:text-muted-foreground/78",
  "[&_code:not(pre_code)]:bg-foreground/[0.03] [&_code:not(pre_code)]:text-[11px] [&_code:not(pre_code)]:text-muted-foreground/88",
  "[&_[data-streamdown='code-block-body']]:!bg-muted/20",
  "[&_pre]:pb-0",
  "[&_pre>code]:py-2 [&_pre>code]:text-[11px] [&_pre>code]:leading-5 [&_pre>code]:text-muted-foreground/82",
  "[&_th]:py-0.5 [&_th]:text-[11px] [&_th]:text-muted-foreground/86",
  "[&_td]:py-0.5 [&_td]:text-[11px] [&_td]:text-muted-foreground/78",
);

const USER_MARKDOWN_CLASSNAME = cn(
  BASE_MARKDOWN_CLASSNAME,
  "leading-8",
  "[&_p]:whitespace-pre-wrap [&_p]:leading-8",
  "[&_li]:leading-7",
  "[&_ul]:my-1 [&_ul]:pl-5",
  "[&_ol]:my-1 [&_ol]:pl-5",
  "[&_h1]:my-1 [&_h1]:text-[17px] [&_h1]:font-semibold [&_h1]:leading-7",
  "[&_h2]:my-1 [&_h2]:text-base [&_h2]:font-semibold [&_h2]:leading-7",
  "[&_h3]:my-1 [&_h3]:text-[15px] [&_h3]:font-semibold [&_h3]:leading-7",
  "[&_h4]:my-1 [&_h4]:text-[15px] [&_h4]:font-medium [&_h4]:leading-7",
  "[&_h5]:my-1 [&_h5]:text-[15px] [&_h5]:font-medium [&_h5]:leading-7",
  "[&_h6]:my-1 [&_h6]:text-[15px] [&_h6]:font-medium [&_h6]:leading-7",
  "[&_blockquote]:my-1 [&_blockquote]:border-l-2 [&_blockquote]:pl-3",
  "[&_[data-streamdown='code-block']]:my-2",
  "[&_[data-streamdown='table-wrapper']]:my-2",
);

const DEFAULT_STREAMDOWN_COMPONENTS = {
  a: MarkdownLink,
  article: MarkdownHTMLArticle,
  aside: MarkdownHTMLAside,
  b: MarkdownStrong,
  details: MarkdownHTMLDetails,
  div: MarkdownHTMLDiv,
  img: MarkdownImage,
  main: MarkdownHTMLMain,
  ol: MarkdownOrderedList,
  p: MarkdownParagraph,
  pre: MarkdownCodePre,
  section: MarkdownHTMLSection,
  span: MarkdownHTMLSpan,
  strong: MarkdownStrong,
  sup: MarkdownSup,
  summary: MarkdownHTMLSummary,
  table: AdaptiveMarkdownTable,
} as const;

const THINKING_STREAMDOWN_COMPONENTS = {
  ...DEFAULT_STREAMDOWN_COMPONENTS,
  h1: ThinkingHeading,
  h2: ThinkingHeading,
  h3: ThinkingHeading,
  h4: ThinkingHeading,
  h5: ThinkingHeading,
  h6: ThinkingHeading,
} as const;

function normalizeStreamdownContent(
  content: unknown,
  preserveSourceLines = false,
  streaming = false,
): string {
  const escapedContent = normalizeCurrencyDollars(
    normalizeEscapedHTMLAttributeQuotes(normalizeContent(content)),
  );
  const normalizedContent = normalizeMermaidBlocks(
    normalizeLatexUnicodeSymbols(
      preserveSourceLines ? escapedContent : normalizeMathDelimiters(escapedContent),
    ),
  );
  return preserveSourceLines
    ? normalizedContent
    : normalizeHTMLBlockBlankLines(normalizeHTMLVisualMarkdownFences(normalizedContent), streaming);
}

function detectStreamdownFeatures(content: string): StreamdownFeatureFlags {
  return {
    code: FENCED_CODE_BLOCK_RE.test(content),
    math: containsMarkdownMath(content),
    mermaid: MERMAID_CODE_BLOCK_RE.test(content),
  };
}

function getStreamdownPluginKey(features: StreamdownFeatureFlags): string {
  return [features.code ? "code" : "", features.math ? "math" : "", features.mermaid ? "mermaid" : ""]
    .filter(Boolean)
    .join(":");
}

function getInitialStreamdownPlugins(features: StreamdownFeatureFlags): PluginConfig {
  if (!features.math) {
    return BASE_STREAMDOWN_PLUGINS;
  }

  return STREAMDOWN_MATH_BASE_PLUGINS;
}

async function loadStreamdownPlugins(features: StreamdownFeatureFlags): Promise<PluginConfig> {
  const key = getStreamdownPluginKey(features);

  if (!key) {
    return BASE_STREAMDOWN_PLUGINS;
  }

  const cachedPlugins = STREAMDOWN_PLUGIN_CACHE.get(key);
  if (cachedPlugins) {
    return cachedPlugins;
  }

  const cachedPromise = STREAMDOWN_PLUGIN_PROMISE_CACHE.get(key);
  if (cachedPromise) {
    return cachedPromise;
  }

  const promise = (async () => {
    const plugins: PluginConfig = { ...BASE_STREAMDOWN_PLUGINS };

    if (features.code) {
      const { code } = await import("@streamdown/code");
      plugins.code = code;
    }

    if (features.math) {
      plugins.math = STREAMDOWN_MATH_PLUGIN;
    }

    if (features.mermaid) {
      const { createMermaidPlugin } = await import("@streamdown/mermaid");
      plugins.mermaid = createMermaidPlugin({
        config: {
          flowchart: {
            htmlLabels: false,
          },
        },
      });
    }

    STREAMDOWN_PLUGIN_CACHE.set(key, plugins);
    STREAMDOWN_PLUGIN_PROMISE_CACHE.delete(key);

    return plugins;
  })();

  STREAMDOWN_PLUGIN_PROMISE_CACHE.set(key, promise);
  void promise.catch(() => {
    STREAMDOWN_PLUGIN_PROMISE_CACHE.delete(key);
  });

  return promise;
}

function useStreamdownPlugins(content: string): PluginConfig {
  const features = React.useMemo(() => detectStreamdownFeatures(content), [content]);
  const pluginKey = React.useMemo(() => getStreamdownPluginKey(features), [features]);
  const [plugins, setPlugins] = React.useState<PluginConfig>(() => STREAMDOWN_PLUGIN_CACHE.get(pluginKey) ?? getInitialStreamdownPlugins(features));

  React.useEffect(() => {
    let cancelled = false;
    const cachedPlugins = STREAMDOWN_PLUGIN_CACHE.get(pluginKey);

    if (cachedPlugins) {
      setPlugins(cachedPlugins);
      return;
    }

    setPlugins(getInitialStreamdownPlugins(features));

    void loadStreamdownPlugins(features)
      .then((loadedPlugins) => {
        if (!cancelled) {
          setPlugins(loadedPlugins);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setPlugins(getInitialStreamdownPlugins(features));
        }
      });

    return () => {
      cancelled = true;
    };
  }, [features, pluginKey]);

  return plugins;
}

function ThinkingSegmentBlock({
  content,
  incomplete,
  plugins,
  streaming,
  autoExpand,
}: {
  content: string;
  incomplete: boolean;
  plugins: PluginConfig;
  streaming: boolean;
  autoExpand: boolean;
}) {
  const t = useTranslations("chat.markdown.thinking");
  const translations = useStreamdownTranslations();
  const icons = useStreamdownIcons();
  const active = streaming && incomplete;
  const { open, onOpenChange } = useAutoExpandDisclosure({ active, autoExpand });

  const isActive = active;

  return (
    <Accordion
      type="single"
      collapsible
      value={open ? "thinking" : ""}
      onValueChange={(value) => onOpenChange(value === "thinking")}
      className="w-full"
    >
      <AccordionItem value="thinking" className="border-b-0">
        <AccordionTrigger
          iconPosition="none"
          className="group items-start justify-between gap-1.5 py-0 text-left no-underline hover:no-underline"
        >
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-1.5">
              <Marker
                render={<span />}
                className={cn(
                  "inline-flex min-h-0 w-auto text-[13px] font-medium transition-colors",
                  !isActive && "text-muted-foreground group-hover:text-foreground",
                )}
              >
                <MarkerContent className={cn("min-w-0", isActive && "shimmer")}>
                  {isActive ? t("active") : t("done")}
                </MarkerContent>
              </Marker>
            </div>
          </div>
          <ChevronDown
            className={cn(
              "mt-0.5 size-3.5 shrink-0 text-muted-foreground transition-transform duration-200 group-hover:text-foreground",
              open && "rotate-180",
            )}
          />
        </AccordionTrigger>
        <AccordionContent className="px-0 pb-0 pt-1.5 duration-[350ms] ease-in-out">
          <HTMLInlineMarkdownProvider
            className={cn(THINKING_MARKDOWN_CLASSNAME, "text-[12px] leading-6 text-muted-foreground/84")}
            components={THINKING_STREAMDOWN_COMPONENTS}
            plugins={plugins}
          >
            <Streamdown
              className={cn(THINKING_MARKDOWN_CLASSNAME, "text-[12px] leading-6 text-muted-foreground/84")}
              codeBlockMaxHeight={STREAMDOWN_CODE_BLOCK_MAX_HEIGHT}
              components={THINKING_STREAMDOWN_COMPONENTS}
              controls={STREAMDOWN_CONTROLS}
              icons={icons}
              plugins={plugins}
              rehypePlugins={STREAMDOWN_REHYPE_PLUGINS}
              remend={STREAMDOWN_REMEND}
              mode={active ? "streaming" : "static"}
              normalizeHtmlIndentation
              parseIncompleteMarkdown={active}
              shikiTheme={["github-light", "github-dark"]}
              animated={false}
              isAnimating={active}
              translations={translations}
            >
              {content}
            </Streamdown>
          </HTMLInlineMarkdownProvider>
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  );
}

function HTMLInlineMarkdownProvider({
  children,
  className,
  components,
  plugins,
}: {
  children: React.ReactNode;
  className: string;
  components: Components;
  plugins: PluginConfig;
}) {
  const translations = useStreamdownTranslations();
  const renderInlineMarkdown = React.useCallback(
    (source: string) => (
      <MarkdownHTMLInlineRendererContext.Provider value={null}>
        <Streamdown
          className={className}
          codeBlockMaxHeight={STREAMDOWN_CODE_BLOCK_MAX_HEIGHT}
          components={components}
          controls={false}
          plugins={plugins}
          rehypePlugins={STREAMDOWN_REHYPE_PLUGINS}
          remend={STREAMDOWN_REMEND}
          linkSafety={STREAMDOWN_LINK_SAFETY}
          mode="static"
          parseIncompleteMarkdown={false}
          shikiTheme={["github-light", "github-dark"]}
          animated={false}
          isAnimating={false}
          translations={translations}
        >
          {source}
        </Streamdown>
      </MarkdownHTMLInlineRendererContext.Provider>
    ),
    [className, components, plugins, translations],
  );

  return (
    <MarkdownHTMLInlineRendererContext.Provider value={renderInlineMarkdown}>
      {children}
    </MarkdownHTMLInlineRendererContext.Provider>
  );
}

export const StreamdownRender = React.memo(function StreamdownRender({
  content,
  className,
  streaming = false,
  variant = "default",
  sourcePositions = false,
  autoExpandThinking = true,
  imageActions,
  artifactActions,
}: StreamdownRenderProps) {
  const normalizedContent = React.useMemo(
    () => normalizeStreamdownContent(content, sourcePositions, streaming),
    [content, sourcePositions, streaming],
  );
  const plugins = useStreamdownPlugins(normalizedContent);
  const segments = React.useMemo(
    () =>
      parseStreamdownSegments(normalizedContent, {
        normalizeHTMLVisualFences: !sourcePositions,
        parseThinking: variant !== "user",
      }),
    [normalizedContent, sourcePositions, variant],
  );
  const {
    rootRef: markdownCopyRootRef,
    onClickCapture: handleMarkdownCopyClickCapture,
    onCopyCapture: handleMarkdownCopyCapture,
    onKeyDownCapture: handleMarkdownCopyKeyDownCapture,
    onPointerDownCapture: handleMarkdownCopyPointerDownCapture,
  } = useMarkdownCopy({
    contentVersion: normalizedContent,
    renderVersion: plugins,
  });
  const thinkingSegments = React.useMemo(
    () => segments.filter((segment): segment is Extract<RenderSegment, { type: "thinking" }> => segment.type === "thinking"),
    [segments],
  );
  const translations = useStreamdownTranslations();
  const icons = useStreamdownIcons();
  const markdownSegments = React.useMemo(
    () => segments.filter((segment): segment is Extract<RenderSegment, { type: "markdown" }> => segment.type === "markdown"),
    [segments],
  );
  const mergedThinkingContent = React.useMemo(
    () => thinkingSegments.map((segment) => segment.content.trim()).filter(Boolean).join("\n\n"),
    [thinkingSegments],
  );
  const hasIncompleteThinking = React.useMemo(
    () => thinkingSegments.some((segment) => segment.incomplete),
    [thinkingSegments],
  );
  const contentSpacingClassName =
    variant === "thinking" ? "space-y-1.5 leading-6" : variant === "user" ? "space-y-2 leading-8" : "space-y-3 leading-8";
  const activeMarkdownClassName =
    variant === "thinking"
      ? THINKING_MARKDOWN_CLASSNAME
      : variant === "user"
        ? USER_MARKDOWN_CLASSNAME
        : BASE_MARKDOWN_CLASSNAME;
  const components = variant === "thinking" ? THINKING_STREAMDOWN_COMPONENTS : DEFAULT_STREAMDOWN_COMPONENTS;
  const rehypePlugins = sourcePositions
    ? SOURCE_POSITION_STREAMDOWN_REHYPE_PLUGINS
    : STREAMDOWN_REHYPE_PLUGINS;

  if (segments.length === 0) {
    return null;
  }

  return (
    <div
      ref={markdownCopyRootRef}
      className={cn("chat-font-content min-w-0 max-w-full overflow-hidden text-foreground [overflow-wrap:anywhere]", contentSpacingClassName, className)}
      data-chat-markdown-scope=""
      onClickCapture={handleMarkdownCopyClickCapture}
      onCopyCapture={handleMarkdownCopyCapture}
      onKeyDownCapture={handleMarkdownCopyKeyDownCapture}
      onPointerDownCapture={handleMarkdownCopyPointerDownCapture}
    >
      <StreamdownAdapterStyles />
      <MarkdownTableStreamingContext.Provider value={streaming}>
        {mergedThinkingContent ? (
          <ThinkingSegmentBlock
            content={mergedThinkingContent}
            incomplete={hasIncompleteThinking}
            plugins={plugins}
            streaming={streaming}
            autoExpand={autoExpandThinking}
          />
        ) : null}
      {markdownSegments.map((segment, index) => (
        <MarkdownArtifactActionsContext.Provider key={`markdown-${index}`} value={artifactActions ?? null}>
          <MarkdownImageActionsContext.Provider value={imageActions ?? null}>
            <HTMLInlineMarkdownProvider
              className={activeMarkdownClassName}
              components={components}
              plugins={plugins}
            >
              <Streamdown
                className={activeMarkdownClassName}
                codeBlockMaxHeight={STREAMDOWN_CODE_BLOCK_MAX_HEIGHT}
                components={components}
                controls={STREAMDOWN_CONTROLS}
                icons={icons}
                plugins={plugins}
                rehypePlugins={rehypePlugins}
                remend={STREAMDOWN_REMEND}
                linkSafety={STREAMDOWN_LINK_SAFETY}
                caret={streaming ? STREAMDOWN_CARET : undefined}
                mode={streaming ? "streaming" : "static"}
                normalizeHtmlIndentation
                parseIncompleteMarkdown={streaming}
                shikiTheme={["github-light", "github-dark"]}
                animated={false}
                isAnimating={streaming}
                translations={translations}
              >
                {segment.content}
              </Streamdown>
            </HTMLInlineMarkdownProvider>
          </MarkdownImageActionsContext.Provider>
        </MarkdownArtifactActionsContext.Provider>
        ))}
      </MarkdownTableStreamingContext.Provider>
    </div>
  );
});
