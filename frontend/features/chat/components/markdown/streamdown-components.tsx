"use client";

import * as React from "react";
import { CornerUpLeft, Download, Eye, Maximize2, WandSparkles } from "lucide-react";
import { useTranslations } from "next-intl";

import { ChevronDown } from "@/components/animate-ui/icons/chevron-down";
import { ChevronUp } from "@/components/animate-ui/icons/chevron-up";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
  downloadMarkdownImageSource,
  loadProtectedMarkdownImageBlobURL,
  resolveMarkdownImageDownloadName,
  resolveMarkdownImageSource,
  resolveProtectedMarkdownImageSource,
} from "@/features/chat/model/markdown-image-source";
import {
  resolveArtifactPreviewKind,
  type ArtifactPreviewKind,
} from "@/features/chat/model/chat-artifacts";
import { CopyActionButton } from "@/shared/components/copy-action";
import { cn } from "@/lib/utils";

const CODE_BLOCK_COLLAPSE_LINE_THRESHOLD = 16;
const DEFAULT_CODE_BLOCK_LANGUAGE = "markdown";

type ResolvedLinkKind = "same-origin" | "external" | "special" | "invalid";

type ExternalLinkSafetyDialogProps = {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => void;
  url: string;
};

type CollapsiblePreProps = React.HTMLAttributes<HTMLPreElement> & {
  children?: React.ReactNode;
};

type StreamdownCodeChildProps = {
  children?: React.ReactNode;
  className?: string;
  "data-block"?: string;
};

type MarkdownLinkProps = React.AnchorHTMLAttributes<HTMLAnchorElement> & {
  children?: React.ReactNode;
  href?: string;
};

type MarkdownImageProps = React.ImgHTMLAttributes<HTMLImageElement> & {
  alt?: string;
  src?: string;
};

export type MarkdownImageActions = {
  canEditImage?: (src: string) => boolean;
  onEditImage?: (src: string) => void;
};

export type MarkdownArtifactActions = {
  onOpenCodeArtifact: (artifact: {
    code: string;
    language: string;
    kind: ArtifactPreviewKind;
  }) => void;
};

type MarkdownParagraphProps = React.HTMLAttributes<HTMLParagraphElement> & {
  children?: React.ReactNode;
  node?: unknown;
};

type MarkdownStrongProps = React.HTMLAttributes<HTMLElement> & {
  children?: React.ReactNode;
  node?: unknown;
};

type MarkdownHeadingProps = React.HTMLAttributes<HTMLHeadingElement> & {
  children?: React.ReactNode;
};

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

const StreamdownLinkContext = React.createContext(false);
const FootnoteBackrefGroupContext = React.createContext(false);
export const MarkdownImageActionsContext = React.createContext<MarkdownImageActions | null>(null);
export const MarkdownArtifactActionsContext = React.createContext<MarkdownArtifactActions | null>(null);

const SAFE_HTML_STYLE_PROPERTIES: ReadonlySet<string> = new Set([
  "alignContent",
  "alignItems",
  "alignSelf",
  "background",
  "backgroundColor",
  "border",
  "borderBlock",
  "borderBlockEnd",
  "borderBlockStart",
  "borderBottom",
  "borderColor",
  "borderInline",
  "borderInlineEnd",
  "borderInlineStart",
  "borderLeft",
  "borderBottomWidth",
  "borderRadius",
  "borderRight",
  "borderStyle",
  "borderTop",
  "borderWidth",
  "boxShadow",
  "boxSizing",
  "color",
  "columnGap",
  "display",
  "flex",
  "flexBasis",
  "flexDirection",
  "flexGrow",
  "flexShrink",
  "flexWrap",
  "fontSize",
  "fontStyle",
  "fontWeight",
  "gap",
  "gridAutoColumns",
  "gridAutoFlow",
  "gridAutoRows",
  "gridColumn",
  "gridColumnEnd",
  "gridColumnStart",
  "gridRow",
  "gridRowEnd",
  "gridRowStart",
  "gridTemplateColumns",
  "gridTemplateRows",
  "height",
  "justifyItems",
  "justifyContent",
  "justifySelf",
  "lineHeight",
  "margin",
  "marginBlock",
  "marginBlockEnd",
  "marginBlockStart",
  "marginBottom",
  "marginInline",
  "marginInlineEnd",
  "marginInlineStart",
  "marginLeft",
  "marginRight",
  "marginTop",
  "maxHeight",
  "maxWidth",
  "minHeight",
  "minWidth",
  "opacity",
  "order",
  "overflow",
  "overflowX",
  "overflowY",
  "padding",
  "paddingBlock",
  "paddingBlockEnd",
  "paddingBlockStart",
  "paddingBottom",
  "paddingInline",
  "paddingInlineEnd",
  "paddingInlineStart",
  "paddingLeft",
  "paddingRight",
  "paddingTop",
  "placeContent",
  "placeItems",
  "placeSelf",
  "position",
  "rowGap",
  "textAlign",
  "top",
  "right",
  "bottom",
  "left",
  "transform",
  "verticalAlign",
  "whiteSpace",
  "width",
  "zIndex",
]);
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
const KATEX_SAFE_HTML_STYLE_PROPERTIES: ReadonlySet<string> = new Set([
  ...SAFE_HTML_STYLE_PROPERTIES,
  "top",
]);
const UNSAFE_STYLE_VALUE_RE = /(?:url\s*\(|expression\s*\(|javascript:|@import|[<>{}])/i;

function isSafeHTMLStyleValue(value: string | number): boolean {
  if (typeof value === "number") {
    return Number.isFinite(value);
  }
  const normalizedValue = value.trim();
  return Boolean(normalizedValue) && normalizedValue.length <= 120 && !UNSAFE_STYLE_VALUE_RE.test(normalizedValue);
}

function sanitizeHTMLStyle(
  style: React.CSSProperties | undefined,
  safeProperties: ReadonlySet<string> = SAFE_HTML_STYLE_PROPERTIES,
): React.CSSProperties | undefined {
  if (!style) {
    return undefined;
  }

  const safeStyle: Record<string, string | number> = {};
  for (const [property, value] of Object.entries(style)) {
    if (!safeProperties.has(property)) {
      continue;
    }
    if (typeof value !== "string" && typeof value !== "number") {
      continue;
    }
    if (!isSafeHTMLStyleValue(value)) {
      continue;
    }
    safeStyle[property] = value;
  }

  return Object.keys(safeStyle).length > 0 ? safeStyle : undefined;
}

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

function resolveLinkKind(href: string): ResolvedLinkKind {
  if (href.startsWith("#")) {
    return "same-origin";
  }

  const currentOrigin = typeof window === "undefined" ? "http://localhost" : window.location.origin;

  try {
    const targetURL = new URL(href, currentOrigin);
    if (targetURL.protocol === "javascript:") {
      return "invalid";
    }
    if (targetURL.origin === currentOrigin) {
      return "same-origin";
    }
    if (targetURL.protocol === "http:" || targetURL.protocol === "https:") {
      return "external";
    }
    return "special";
  } catch {
    return "invalid";
  }
}

function isFootnoteBackref(props: React.AnchorHTMLAttributes<HTMLAnchorElement>): boolean {
  return "data-footnote-backref" in props;
}

function isFootnoteReference(props: React.AnchorHTMLAttributes<HTMLAnchorElement>): boolean {
  return "data-footnote-ref" in props;
}

function resolveHashTarget(href: string, scope: HTMLElement | null): HTMLElement | null {
  if (typeof window === "undefined") {
    return null;
  }

  const url = new URL(href, window.location.href);
  if (url.origin !== window.location.origin || url.pathname !== window.location.pathname || !url.hash) {
    return null;
  }

  const rawID = url.hash.slice(1);
  const decodedID = decodeURIComponent(rawID);
  const candidateIDs = [
    rawID,
    decodedID,
    `user-content-${rawID}`,
    `user-content-${decodedID}`,
  ];

  const findInScope = (root: ParentNode): HTMLElement | null => {
    const elements = Array.from(root.querySelectorAll<HTMLElement>("[id]"));
    return (
      elements.find((element) => candidateIDs.includes(element.id)) ??
      elements.find((element) => element.id.endsWith(rawID) || element.id.endsWith(decodedID)) ??
      null
    );
  };

  return (scope ? findInScope(scope) : null) ?? findInScope(document);
}

function scrollToHashTarget(href: string, scope: HTMLElement | null): boolean {
  const target = resolveHashTarget(href, scope);
  if (!target) {
    return false;
  }

  const targetRect = target.getBoundingClientRect();
  const visible =
    targetRect.top >= 0 &&
    targetRect.left >= 0 &&
    targetRect.bottom <= window.innerHeight &&
    targetRect.right <= window.innerWidth;

  if (!visible) {
    target.scrollIntoView({ block: "nearest", inline: "nearest", behavior: "smooth" });
  }
  if (!target.hasAttribute("tabindex")) {
    target.setAttribute("tabindex", "-1");
  }
  target.focus({ preventScroll: true });
  return true;
}

function getReactNodeText(node: React.ReactNode): string {
  return React.Children.toArray(node)
    .map((child) => {
      if (typeof child === "string" || typeof child === "number") {
        return String(child);
      }
      if (React.isValidElement<{ children?: React.ReactNode }>(child)) {
        return getReactNodeText(child.props.children);
      }
      return "";
    })
    .join("");
}

function resolveFootnoteBackrefIndex(children: React.ReactNode, ariaLabel?: string): string {
  const ariaMatch = ariaLabel?.trim().match(/(\d+)(?:-(\d+))?$/);
  if (ariaMatch) {
    return ariaMatch[2] ?? "1";
  }

  const childIndex = getReactNodeText(children).replace("↩", "").trim();
  return childIndex || "1";
}

function FootnoteBackrefContent({
  children,
  ariaLabel,
}: {
  children: React.ReactNode;
  ariaLabel?: string;
}) {
  const t = useTranslations("chat.markdown");
  const shouldShowIndex = React.useContext(FootnoteBackrefGroupContext);
  const backrefIndex = resolveFootnoteBackrefIndex(children, ariaLabel);

  return (
    <>
      <CornerUpLeft className="size-3.5" strokeWidth={1.8} />
      {shouldShowIndex ? <span className="ml-0.5 text-[10px] leading-none">{backrefIndex}</span> : null}
      <span className="sr-only">{t("back")}</span>
    </>
  );
}

function getCodeTextFromChild(child: React.ReactElement<StreamdownCodeChildProps>): string {
  const raw = child.props.children;
  if (typeof raw === "string") {
    return raw;
  }

  if (Array.isArray(raw)) {
    return raw.filter((item): item is string => typeof item === "string").join("");
  }

  return "";
}

function getCodeLanguage(className?: string): string {
  if (!className) {
    return "";
  }

  const match = className.match(/language-([^\s]+)/);
  return match?.[1] ?? "";
}

function ensureCodeBlockLanguage(
  child: React.ReactElement<StreamdownCodeChildProps>,
): React.ReactElement<StreamdownCodeChildProps> {
  if (getCodeLanguage(child.props.className)) {
    return child;
  }

  return React.cloneElement(child, {
    className: cn(child.props.className, `language-${DEFAULT_CODE_BLOCK_LANGUAGE}`),
  });
}

function getLineCount(value: string): number {
  if (!value) {
    return 0;
  }

  return value.replace(/\n$/, "").split("\n").length;
}

function isMermaidLanguage(language: string): boolean {
  return language === "mermaid" || language === "mmd";
}

function CodeBlockActionButton({
  label,
  children,
  onClick,
}: {
  label: string;
  children: React.ReactNode;
  onClick: () => void;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          aria-label={label}
          className="inline-flex size-6 items-center justify-center rounded-md p-1 text-muted-foreground transition-colors hover:bg-foreground/[0.04] hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/35"
          onClick={onClick}
        >
          {children}
        </button>
      </TooltipTrigger>
      <TooltipContent side="top">{label}</TooltipContent>
    </Tooltip>
  );
}

function CodeBlockActions({
  code,
  language,
  previewable,
}: {
  code: string;
  language: string;
  previewable: boolean;
}) {
  const commonActions = useTranslations("common.actions");
  const commonErrors = useTranslations("common.errors");
  const artifactCopy = useTranslations("chat.markdown.artifact");
  const artifactActions = React.useContext(MarkdownArtifactActionsContext);
  const artifactKind = React.useMemo(() => resolveArtifactPreviewKind(language, code), [code, language]);

  const handleOpenArtifact = React.useCallback(() => {
    if (!artifactActions || !artifactKind) {
      return;
    }
    artifactActions.onOpenCodeArtifact({ code, language, kind: artifactKind });
  }, [artifactActions, artifactKind, code, language]);

  const canOpenArtifact = Boolean(previewable && artifactActions && artifactKind && code.trim());

  return (
    <div className="pointer-events-none absolute right-0 top-0 z-20 flex h-8 items-center justify-end">
      <div className="pointer-events-auto flex shrink-0 items-center gap-2 rounded-md bg-background/80 px-1.5 py-1 backdrop-blur">
        {canOpenArtifact ? (
          <CodeBlockActionButton label={artifactCopy("openPreview")} onClick={handleOpenArtifact}>
            <Eye className="size-4" strokeWidth={1.8} />
          </CodeBlockActionButton>
        ) : null}
        <Tooltip>
          <TooltipTrigger asChild>
            <CopyActionButton
              key={`${language}:${code}`}
              type="button"
              variant="ghost"
              size="icon-xs"
              className="inline-flex size-6 items-center justify-center rounded-md p-1 text-muted-foreground transition-colors hover:bg-foreground/[0.04] hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/35"
              value={code}
              messages={{ copied: commonActions("copied"), failed: commonErrors("copyFailed") }}
              iconClassName="size-3.5"
              aria-label={commonActions("copy")}
            />
          </TooltipTrigger>
          <TooltipContent side="top">{commonActions("copy")}</TooltipContent>
        </Tooltip>
      </div>
    </div>
  );
}

function ExternalLinkSafetyDialog({ isOpen, onClose, onConfirm, url }: ExternalLinkSafetyDialogProps) {
  const t = useTranslations("chat.markdown.externalLink");
  const common = useTranslations("common.actions");

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
          <DialogDescription>{t("description")}</DialogDescription>
        </DialogHeader>

        <div className="space-y-1">
          <p className="text-xs text-muted-foreground">{t("linkAddress")}</p>
          <div className="flex items-center gap-2">
            <Input readOnly value={url} className="font-mono" />
            <CopyActionButton
              key={isOpen ? url : "closed"}
              type="button"
              variant="ghost"
              size="icon"
              value={url}
              messages={{ copied: t("copied"), failed: t("copyFailed") }}
              iconClassName="size-4"
              aria-label={t("copy")}
            />
          </div>
        </div>

        <DialogFooter>
          <Button type="button" variant="ghost" onClick={onClose}>
            {common("cancel")}
          </Button>
          <Button type="button" onClick={onConfirm}>
            {common("open")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function openExternalURL(url: string) {
  window.open(url, "_blank", "noreferrer");
}

function isEmptyReactNode(node: React.ReactNode): boolean {
  return node == null || node === "";
}

function isCodeBlockElement(node: React.ReactNode): boolean {
  if (!React.isValidElement<{ "data-block"?: string }>(node)) {
    return false;
  }
  return node.props["data-block"] === "true";
}

function isImageElement(node: React.ReactNode): boolean {
  return React.isValidElement(node) && node.type === MarkdownImage;
}

function isImageLinkElement(node: React.ReactNode): boolean {
  if (!React.isValidElement<{ children?: React.ReactNode }>(node) || node.type !== MarkdownLink) {
    return false;
  }
  const children = React.Children.toArray(node.props.children).filter((child) => !isEmptyReactNode(child));
  return children.length === 1 && isImageElement(children[0]);
}

function isFootnoteBackrefElement(node: React.ReactNode): boolean {
  return React.isValidElement<React.AnchorHTMLAttributes<HTMLAnchorElement>>(node) && isFootnoteBackref(node.props);
}

export function CollapsibleCodePre({ children }: CollapsiblePreProps) {
  const t = useTranslations("chat.markdown.codeBlock");
  const childElement = React.isValidElement<StreamdownCodeChildProps>(children) ? ensureCodeBlockLanguage(children) : null;
  const codeContent = childElement ? getCodeTextFromChild(childElement) : "";
  const lineCount = getLineCount(codeContent);
  const language = childElement ? getCodeLanguage(childElement.props.className) : "";
  const mermaid = isMermaidLanguage(language);
  const artifactPreviewable = Boolean(resolveArtifactPreviewKind(language, codeContent));
  const isCollapsible =
    childElement != null && !mermaid && lineCount > CODE_BLOCK_COLLAPSE_LINE_THRESHOLD;
  const [expanded, setExpanded] = React.useState(false);
  const [isToggleHovered, setIsToggleHovered] = React.useState(false);

  if (!childElement) {
    return children;
  }

  const codeBlock = React.cloneElement(childElement, { "data-block": "true" });

  if (!isCollapsible) {
    return (
      <div className="relative w-full">
        {!mermaid ? <CodeBlockActions code={codeContent} language={language} previewable={artifactPreviewable} /> : null}
        {codeBlock}
      </div>
    );
  }

  return (
    <div className="relative w-full">
      <CodeBlockActions code={codeContent} language={language} previewable={artifactPreviewable} />
      <div
        className={cn(
          "w-full",
          "[&_[data-streamdown='code-block-body']]:transition-[max-height] [&_[data-streamdown='code-block-body']]:duration-300 [&_[data-streamdown='code-block-body']]:ease-out",
          !expanded && "[&_[data-streamdown='code-block-body']]:max-h-[22rem] [&_[data-streamdown='code-block-body']]:overflow-hidden",
        )}
      >
        {codeBlock}
      </div>
      {!expanded ? (
        <div className="pointer-events-none absolute inset-x-0 bottom-9 h-20 bg-gradient-to-b from-transparent via-background/70 to-background" />
      ) : null}
      <div className="flex justify-center">
        <button
          type="button"
          onClick={() => setExpanded((current) => !current)}
          onMouseEnter={() => setIsToggleHovered(true)}
          onMouseLeave={() => setIsToggleHovered(false)}
          className="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-[11px] font-medium text-muted-foreground transition-colors hover:bg-foreground/[0.04] hover:text-foreground"
        >
          {expanded ? (
            <ChevronUp className="size-3.5" animate={isToggleHovered ? "default" : undefined} />
          ) : (
            <ChevronDown className="size-3.5" animate={isToggleHovered ? "default" : undefined} />
          )}
          <span>{expanded ? t("collapse") : t("expand", { count: lineCount })}</span>
        </button>
      </div>
    </div>
  );
}

export function MarkdownLink({ children, className, href, onClick, ...props }: MarkdownLinkProps) {
  const t = useTranslations("chat.markdown");
  const [modalOpen, setModalOpen] = React.useState(false);
  const [pendingURL, setPendingURL] = React.useState("");
  const incomplete = href === "streamdown:incomplete-link";
  const linkKind = React.useMemo(() => (href ? resolveLinkKind(href) : "invalid"), [href]);
  const footnoteBackref = isFootnoteBackref(props);
  const footnoteReference = isFootnoteReference(props);
  const normalizedChildren = React.useMemo(
    () => React.Children.toArray(children).filter((child) => !isEmptyReactNode(child)),
    [children],
  );
  const hasBlockChild = React.useMemo(
    () =>
      normalizedChildren.some(
        (child) => isImageElement(child) || isImageLinkElement(child) || isCodeBlockElement(child),
      ),
    [normalizedChildren],
  );

  const handleClick = React.useCallback(
    (event: React.MouseEvent<HTMLAnchorElement>) => {
      onClick?.(event);
      if (event.defaultPrevented) {
        return;
      }

      if (!href || incomplete || linkKind === "invalid") {
        event.preventDefault();
        return;
      }

      if (linkKind === "same-origin" && href.includes("#")) {
        event.preventDefault();
        const scope = event.currentTarget.closest<HTMLElement>("[data-chat-markdown-scope]");
        scrollToHashTarget(href, scope);
        return;
      }

      if (linkKind !== "external") {
        return;
      }

      event.preventDefault();
      setPendingURL(href);
      setModalOpen(true);
    },
    [href, incomplete, linkKind, onClick],
  );

  const handleConfirm = React.useCallback(() => {
    if (!pendingURL) {
      return;
    }
    openExternalURL(pendingURL);
    setModalOpen(false);
  }, [pendingURL]);

  const handleClose = React.useCallback(() => {
    setModalOpen(false);
  }, []);

  if (!href || incomplete || linkKind === "invalid") {
    if (hasBlockChild) {
      return <StreamdownLinkContext.Provider value={true}>{children}</StreamdownLinkContext.Provider>;
    }
    return (
      <span
        className={cn("wrap-anywhere font-medium text-primary underline", className)}
        data-incomplete={incomplete || undefined}
        data-streamdown="link"
      >
        {children}
      </span>
    );
  }

  return (
    <>
      <a
        {...props}
        className={cn(
          "wrap-anywhere font-medium text-primary underline",
          footnoteReference && "text-[0.72em] no-underline",
          footnoteBackref && "ml-1 inline-flex align-baseline text-muted-foreground/75 no-underline hover:text-muted-foreground",
          className,
        )}
        aria-label={footnoteBackref ? t("footnoteBackref") : props["aria-label"]}
        data-streamdown="link"
        href={href}
        rel={linkKind === "external" ? "noreferrer" : props.rel}
        target={linkKind === "external" ? "_blank" : undefined}
        onClick={(event) => void handleClick(event)}
      >
        <StreamdownLinkContext.Provider value={true}>
          {footnoteBackref ? (
            <FootnoteBackrefContent ariaLabel={props["aria-label"]}>{children}</FootnoteBackrefContent>
          ) : (
            children
          )}
        </StreamdownLinkContext.Provider>
      </a>
      <ExternalLinkSafetyDialog
        isOpen={modalOpen}
        onClose={handleClose}
        onConfirm={handleConfirm}
        url={pendingURL}
      />
    </>
  );
}

export function MarkdownImage({ alt, className, onError, onLoad, src, ...props }: MarkdownImageProps) {
  const t = useTranslations("chat.markdown");
  const insideLink = React.useContext(StreamdownLinkContext);
  const imageActions = React.useContext(MarkdownImageActionsContext);
  const [loaded, setLoaded] = React.useState(false);
  const [failed, setFailed] = React.useState(false);
  const [previewOpen, setPreviewOpen] = React.useState(false);
  const resolvedSrc = React.useMemo(() => (src ? resolveMarkdownImageSource(src) : ""), [src]);
  const protectedSrc = React.useMemo(() => (src ? resolveProtectedMarkdownImageSource(src) : null), [src]);
  const [displaySrc, setDisplaySrc] = React.useState(() => (protectedSrc ? "" : resolvedSrc));

  React.useEffect(() => {
    setLoaded(false);
    setFailed(false);
    if (!src) {
      setDisplaySrc("");
      return undefined;
    }

    if (!protectedSrc) {
      setDisplaySrc(resolvedSrc);
      return undefined;
    }
    setDisplaySrc("");

    const controller = new AbortController();
    let blobURL = "";
    let active = true;
    void loadProtectedMarkdownImageBlobURL(protectedSrc, controller.signal)
      .then((nextBlobURL) => {
        blobURL = nextBlobURL;
        if (active) {
          setFailed(false);
          setDisplaySrc(blobURL);
        } else {
          URL.revokeObjectURL(blobURL);
        }
      })
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") {
          return;
        }
        setDisplaySrc("");
        setFailed(true);
      });

    return () => {
      active = false;
      controller.abort();
      if (blobURL) {
        URL.revokeObjectURL(blobURL);
      }
    };
  }, [protectedSrc, resolvedSrc, src]);

  const handleLoad = React.useCallback(
    (event: React.SyntheticEvent<HTMLImageElement>) => {
      setLoaded(true);
      setFailed(false);
      onLoad?.(event);
    },
    [onLoad],
  );

  const handleError = React.useCallback(
    (event: React.SyntheticEvent<HTMLImageElement>) => {
      setLoaded(false);
      setFailed(true);
      onError?.(event);
    },
    [onError],
  );

  const handleDownload = React.useCallback(async () => {
    if (!src) {
      return;
    }
    try {
      await downloadMarkdownImageSource(src, resolveMarkdownImageDownloadName(src, alt));
    } catch {
      openExternalURL(resolvedSrc);
    }
  }, [alt, resolvedSrc, src]);

  const canUseImageActions = !insideLink && !failed && Boolean(displaySrc);
  const canEditImage = Boolean(src && imageActions?.onEditImage && (imageActions.canEditImage?.(src) ?? true));

  if (!src) {
    return null;
  }

  return (
    <span
      className={cn("group relative my-4 block w-fit max-w-full sm:max-w-[32rem]", className)}
      data-streamdown="image-wrapper"
    >
      {failed ? (
        <span className="flex min-h-28 min-w-48 items-center justify-center rounded-xl border border-border bg-muted/25 px-4 py-6 text-sm text-muted-foreground sm:min-w-80">
          {alt?.trim() || t("imageUnavailable")}
        </span>
      ) : !displaySrc ? (
        <span className="block min-h-28 min-w-48 animate-pulse rounded-xl border border-border/60 bg-muted/20 sm:min-w-80" />
      ) : (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          {...props}
          alt={alt}
          className="block h-auto max-h-[34rem] w-auto max-w-full rounded-xl border border-border/60 bg-muted/10 object-contain"
          loading="lazy"
          src={displaySrc}
          onError={handleError}
          onLoad={handleLoad}
        />
      )}
      {canUseImageActions ? (
        <span
          className={cn(
            "absolute bottom-2 right-2 inline-flex items-center gap-1 rounded-full border border-border/70 bg-background/88 p-1 text-muted-foreground shadow-sm transition-opacity",
            loaded ? "opacity-100" : "opacity-0",
          )}
        >
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                aria-label={t("previewImage")}
                className="inline-flex size-7 items-center justify-center rounded-full transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
                type="button"
                onClick={() => setPreviewOpen(true)}
              >
                <Maximize2 className="size-3.5" />
              </button>
            </TooltipTrigger>
            <TooltipContent>{t("previewImage")}</TooltipContent>
          </Tooltip>
          {canEditImage ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <button
                  aria-label={t("editImage")}
                  className="inline-flex size-7 items-center justify-center rounded-full transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
                  type="button"
                  onClick={() => imageActions?.onEditImage?.(src)}
                >
                  <WandSparkles className="size-3.5" />
                </button>
              </TooltipTrigger>
              <TooltipContent>{t("editImage")}</TooltipContent>
            </Tooltip>
          ) : null}
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                aria-label={t("downloadImage")}
                className="inline-flex size-7 items-center justify-center rounded-full transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
                type="button"
                onClick={() => void handleDownload()}
              >
                <Download className="size-3.5" />
              </button>
            </TooltipTrigger>
            <TooltipContent>{t("downloadImage")}</TooltipContent>
          </Tooltip>
        </span>
      ) : null}
      <Dialog open={previewOpen} onOpenChange={setPreviewOpen}>
        <DialogContent className="w-fit max-w-[96vw] border-0 bg-transparent p-0 shadow-none sm:max-w-[96vw] [&>button]:border [&>button]:border-border/70 [&>button]:bg-background/90 [&>button]:text-foreground [&>button]:shadow-sm">
          <DialogTitle className="sr-only">{alt?.trim() || t("previewImage")}</DialogTitle>
          <DialogDescription className="sr-only">{t("previewImage")}</DialogDescription>
          {displaySrc ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              alt={alt}
              className="block max-h-[92vh] max-w-[96vw] rounded-lg border border-border/50 bg-background/5 object-contain shadow-2xl"
              src={displaySrc}
            />
          ) : null}
        </DialogContent>
      </Dialog>
    </span>
  );
}

export function MarkdownParagraph({ children, className, node: _node, style, ...props }: MarkdownParagraphProps) {
  const normalizedChildren = React.Children.toArray(children).filter((child) => !isEmptyReactNode(child));
  if (normalizedChildren.length === 1) {
    const onlyChild = normalizedChildren[0];
    if (isImageElement(onlyChild) || isImageLinkElement(onlyChild) || isCodeBlockElement(onlyChild)) {
      return <>{children}</>;
    }
  }
  const footnoteBackrefCount = normalizedChildren.filter(isFootnoteBackrefElement).length;
  const paragraphChildren =
    footnoteBackrefCount > 1 ? (
      <FootnoteBackrefGroupContext.Provider value={true}>{children}</FootnoteBackrefGroupContext.Provider>
    ) : (
      children
    );

  return (
    <p
      {...props}
      className={cn("min-w-0 max-w-full break-words [overflow-wrap:anywhere]", className)}
      style={sanitizeHTMLStyle(style)}
    >
      {paragraphChildren}
    </p>
  );
}

export function MarkdownStrong({ children, className, node: _node, style, ...props }: MarkdownStrongProps) {
  return (
    <strong
      {...props}
      className={cn("font-bold text-foreground", className)}
      style={{
        ...sanitizeHTMLStyle(style),
        fontWeight: "var(--font-chat-strong-weight)",
      }}
    >
      {children}
    </strong>
  );
}

export function MarkdownHTMLDiv({ children, className, node: _node, style }: MarkdownHTMLBlockProps) {
  return (
    <div className={cn("min-w-0 max-w-full", className)} style={sanitizeHTMLStyle(style)}>
      {children}
    </div>
  );
}

export function MarkdownHTMLSection({ children, className, node: _node, style }: MarkdownHTMLBlockProps) {
  return (
    <section className={cn("min-w-0 max-w-full", className)} style={sanitizeHTMLStyle(style)}>
      {children}
    </section>
  );
}

export function MarkdownHTMLArticle({ children, className, node: _node, style }: MarkdownHTMLBlockProps) {
  return (
    <article className={cn("min-w-0 max-w-full", className)} style={sanitizeHTMLStyle(style)}>
      {children}
    </article>
  );
}

export function MarkdownHTMLAside({ children, className, node: _node, style }: MarkdownHTMLBlockProps) {
  return (
    <aside className={cn("min-w-0 max-w-full", className)} style={sanitizeHTMLStyle(style)}>
      {children}
    </aside>
  );
}

export function MarkdownHTMLMain({ children, className, node: _node, style }: MarkdownHTMLBlockProps) {
  return (
    <main className={cn("min-w-0 max-w-full", className)} style={sanitizeHTMLStyle(style)}>
      {children}
    </main>
  );
}

export function MarkdownHTMLDetails({ children, className, node: _node, open, style }: MarkdownHTMLDetailsProps) {
  return (
    <details className={cn("min-w-0 max-w-full", className)} open={open} style={sanitizeHTMLStyle(style)}>
      {children}
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
      <span className={className} style={sanitizeHTMLStyle(style, KATEX_SAFE_HTML_STYLE_PROPERTIES)}>
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

export function ThinkingHeading({ children, ...props }: MarkdownHeadingProps) {
  return <p {...props}>{children}</p>;
}
