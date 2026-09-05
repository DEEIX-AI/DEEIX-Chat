"use client";

import { CornerUpLeft, Download, Eye, Maximize2, WandSparkles } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { CopyActionButton } from "@/shared/components/copy-action";
import { MediaActionBar, MediaActionButton } from "@/shared/components/media-action-bar";
import {
  type ArtifactPreviewKind,
  resolveArtifactPreviewKind,
} from "@/shared/lib/artifact-preview";
import {
  downloadMarkdownImageSource,
  loadProtectedMarkdownImageBlobURL,
  resolveMarkdownImageDownloadName,
  resolveMarkdownImageSource,
  resolveProtectedMarkdownImageSource,
} from "@/shared/lib/markdown-image-source";
import { MarkdownFootnotesContext } from "./streamdown-html";
import { StreamdownCheckIcon, StreamdownCopyIcon } from "./streamdown-icons";
import { sanitizeHTMLStyle } from "./streamdown-style";

// 未标注语言的围栏统一按纯文本处理:标签如实显示 text,避免内容被错误地按 Markdown 语法高亮。
const DEFAULT_CODE_BLOCK_LANGUAGE = "text";
const CODE_BLOCK_ACTION_BUTTON_CLASSNAME =
  "size-5 cursor-pointer rounded-none p-1 text-muted-foreground transition-all outline-none hover:bg-foreground/[0.04] hover:text-foreground focus-visible:bg-foreground/[0.04] focus-visible:text-foreground focus-visible:ring-0 disabled:cursor-not-allowed disabled:opacity-50";

type ResolvedLinkKind = "same-origin" | "external" | "special" | "invalid";

type ExternalLinkSafetyDialogProps = {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => void;
  url: string;
};

type MarkdownCodePreProps = React.HTMLAttributes<HTMLPreElement> & {
  children?: React.ReactNode;
  node?: unknown;
  "data-markdown-source-line"?: string | number;
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

type MarkdownImageProps = React.ImgHTMLAttributes<HTMLImageElement>;

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

type MarkdownOrderedListProps = React.OlHTMLAttributes<HTMLOListElement> & {
  children?: React.ReactNode;
  node?: unknown;
};

type MarkdownStrongProps = React.HTMLAttributes<HTMLElement> & {
  children?: React.ReactNode;
  node?: unknown;
};

type MarkdownSupProps = React.HTMLAttributes<HTMLElement> & {
  children?: React.ReactNode;
  node?: unknown;
};

type MarkdownHeadingProps = React.HTMLAttributes<HTMLHeadingElement> & {
  children?: React.ReactNode;
};

const StreamdownLinkContext = React.createContext(false);
const FootnoteBackrefGroupContext = React.createContext(false);
export const MarkdownImageActionsContext = React.createContext<MarkdownImageActions | null>(null);
export const MarkdownArtifactActionsContext = React.createContext<MarkdownArtifactActions | null>(null);

function resolveLinkKind(href: string): ResolvedLinkKind {
  if (href.startsWith("#")) {
    return "same-origin";
  }

  const currentOrigin = typeof window === "undefined" ? "http://localhost" : window.location.origin;

  try {
    const targetURL = new URL(href, currentOrigin);
    if (
      targetURL.protocol === "javascript:" ||
      targetURL.protocol === "data:" ||
      targetURL.protocol === "vbscript:"
    ) {
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

function isFootnoteBackref(
  props: React.AnchorHTMLAttributes<HTMLAnchorElement>,
  children?: React.ReactNode,
): boolean {
  const href = props.href?.trim() ?? "";
  const childText = children == null ? "" : getReactNodeText(children);
  return (
    "data-footnote-backref" in props ||
    /^#(?:user-content-)?fnref(?:[-\d]|$)/i.test(href) ||
    (href.includes("#") && childText.includes("↩"))
  );
}

function isFootnoteReference(props: React.AnchorHTMLAttributes<HTMLAnchorElement>): boolean {
  return "data-footnote-ref" in props;
}

function isSuperscriptReferenceElement(node: React.ReactNode): boolean {
  return (
    React.isValidElement<React.AnchorHTMLAttributes<HTMLAnchorElement>>(node) &&
    (isFootnoteReference(node.props) || typeof node.props.href === "string")
  );
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

function containsFootnoteBackref(node: React.ReactNode): boolean {
  return React.Children.toArray(node).some((child) => {
    if (!React.isValidElement<React.AnchorHTMLAttributes<HTMLAnchorElement>>(child)) {
      return false;
    }
    if (isFootnoteBackref(child.props, child.props.children)) {
      return true;
    }
    return containsFootnoteBackref(child.props.children);
  });
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
      <CornerUpLeft className="size-3" strokeWidth={2} />
      {shouldShowIndex ? <span className="ml-0.5 text-[10px] leading-none">{backrefIndex}</span> : null}
      <span className="sr-only">{t("back")}</span>
    </>
  );
}

export function MarkdownOrderedList({
  children,
  className,
  node: _node,
  style,
  ...props
}: MarkdownOrderedListProps) {
  const footnoteList = containsFootnoteBackref(children);

  const list = (
    <ol
      {...props}
      className={cn(
        "list-inside list-decimal whitespace-normal [li_&]:pl-6",
        footnoteList &&
          "mt-6 border-foreground/15 border-t pt-3 pl-4 text-[11px] leading-5 text-muted-foreground/82 [&_li]:py-0.5 [&_p]:my-0 [&_p]:text-[11px] [&_p]:leading-5",
        className,
      )}
      data-streamdown={footnoteList ? "footnote-list" : "ordered-list"}
      style={sanitizeHTMLStyle(style)}
    >
      {children}
    </ol>
  );

  return footnoteList ? <MarkdownFootnotesContext.Provider value>{list}</MarkdownFootnotesContext.Provider> : list;
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
          className={CODE_BLOCK_ACTION_BUTTON_CLASSNAME}
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
  const copyCode = code.replace(/\n$/, "");

  const handleOpenArtifact = React.useCallback(() => {
    if (!artifactActions || !artifactKind) {
      return;
    }
    artifactActions.onOpenCodeArtifact({ code, language, kind: artifactKind });
  }, [artifactActions, artifactKind, code, language]);

  const canOpenArtifact = Boolean(previewable && artifactActions && artifactKind && code.trim());

  return (
    <div className="pointer-events-none absolute right-0 top-0 z-20 flex h-8 items-center justify-end">
      <div
        className="pointer-events-auto flex shrink-0 items-center gap-2"
        data-streamdown="code-block-actions"
      >
        {canOpenArtifact ? (
          <CodeBlockActionButton label={artifactCopy("openPreview")} onClick={handleOpenArtifact}>
            <Eye className="size-3" strokeWidth={2.25} />
          </CodeBlockActionButton>
        ) : null}
        <Tooltip>
          <TooltipTrigger asChild>
            <CopyActionButton
              key={`${language}:${code}`}
              type="button"
              variant="ghost"
              size="icon-xs"
              className={CODE_BLOCK_ACTION_BUTTON_CLASSNAME}
              value={copyCode}
              messages={{ copied: commonActions("copied"), failed: commonErrors("copyFailed") }}
              copyIcon={<StreamdownCopyIcon className="size-3" />}
              copiedIcon={<StreamdownCheckIcon className="size-3" />}
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
  return (
    React.isValidElement<React.AnchorHTMLAttributes<HTMLAnchorElement>>(node) &&
    isFootnoteBackref(node.props, node.props.children)
  );
}

export function MarkdownCodePre({ children, node: _node, "data-markdown-source-line": sourceLine }: MarkdownCodePreProps) {
  const childElement = React.isValidElement<StreamdownCodeChildProps>(children) ? ensureCodeBlockLanguage(children) : null;
  const codeContent = childElement ? getCodeTextFromChild(childElement) : "";
  const language = childElement ? getCodeLanguage(childElement.props.className) : "";
  const mermaid = isMermaidLanguage(language);
  const artifactPreviewable = Boolean(resolveArtifactPreviewKind(language, codeContent));

  if (!childElement) {
    return children;
  }

  const codeBlock = React.cloneElement(childElement, { "data-block": "true" });

  return (
    <div className="relative w-full" data-markdown-source-line={sourceLine}>
      {!mermaid ? <CodeBlockActions code={codeContent} language={language} previewable={artifactPreviewable} /> : null}
      {codeBlock}
    </div>
  );
}

export function MarkdownLink({ children, className, href, onClick, style, ...props }: MarkdownLinkProps) {
  const t = useTranslations("chat.markdown");
  const insideFootnotes = React.useContext(MarkdownFootnotesContext);
  const [modalOpen, setModalOpen] = React.useState(false);
  const [pendingURL, setPendingURL] = React.useState("");
  const incomplete = href === "streamdown:incomplete-link";
  const linkKind = React.useMemo(() => (href ? resolveLinkKind(href) : "invalid"), [href]);
  const footnoteBackref =
    isFootnoteBackref(props, children) || (insideFootnotes && getReactNodeText(children).includes("↩"));
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
          footnoteReference &&
            "inline-block whitespace-nowrap font-normal leading-none no-underline",
          footnoteBackref &&
            "ml-1 inline-flex size-4 items-center justify-center align-middle text-muted-foreground/75 no-underline hover:text-foreground",
          className,
        )}
        aria-label={footnoteBackref ? t("footnoteBackref") : props["aria-label"]}
        data-streamdown="link"
        href={href}
        rel={linkKind === "external" ? "noreferrer" : props.rel}
        style={sanitizeHTMLStyle(style)}
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

export function MarkdownSup({ children, className, node: _node, style, ...props }: MarkdownSupProps) {
  const reference = React.Children.toArray(children).some(isSuperscriptReferenceElement);

  return (
    <sup
      {...props}
      className={cn(
        reference
          ? "relative -top-[0.42em] ml-1 align-baseline text-[11px] font-normal leading-none [&_a]:!font-normal [&_a]:no-underline"
          : "text-sm",
        className,
      )}
      data-streamdown={reference ? "footnote-reference" : "superscript"}
      style={sanitizeHTMLStyle(style)}
    >
      {children}
    </sup>
  );
}

export function MarkdownImage({ alt, className, onError, onLoad, src: srcProp, ...props }: MarkdownImageProps) {
  // Markdown 渲染只产生字符串 src；Blob 形态的 src 不在渲染范围内。
  const src = typeof srcProp === "string" ? srcProp : undefined;
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
        <MediaActionBar
          className={cn(
            "absolute bottom-2 right-2 transition-opacity",
            loaded ? "opacity-100" : "opacity-0",
          )}
        >
          <MediaActionButton label={t("previewImage")} onClick={() => setPreviewOpen(true)}>
            <Maximize2 className="size-3.5" />
          </MediaActionButton>
          {canEditImage ? (
            <MediaActionButton label={t("editImage")} onClick={() => imageActions?.onEditImage?.(src)}>
              <WandSparkles className="size-3.5" />
            </MediaActionButton>
          ) : null}
          <MediaActionButton label={t("downloadImage")} onClick={() => void handleDownload()}>
            <Download className="size-3.5" />
          </MediaActionButton>
        </MediaActionBar>
      ) : null}
      <Dialog open={previewOpen} onOpenChange={setPreviewOpen}>
        <DialogContent className="w-fit max-w-[96vw] border-0 bg-transparent p-0 shadow-none sm:max-w-[96vw] [&>button]:border [&>button]:border-border/70 [&>button]:bg-background/90 [&>button]:text-foreground [&>button]:shadow-sm">
          <DialogTitle className="sr-only">{alt?.trim() || t("previewImage")}</DialogTitle>
          <DialogDescription className="sr-only">{t("previewImage")}</DialogDescription>
          {displaySrc ? (
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

export function ThinkingHeading({ children, ...props }: MarkdownHeadingProps) {
  return <p {...props}>{children}</p>;
}
