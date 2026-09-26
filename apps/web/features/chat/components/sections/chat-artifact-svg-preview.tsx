"use client";

import * as React from "react";

import type { HTMLVisualThemeSnapshot } from "@/shared/lib/html-visual-theme";

type ChatArtifactSVGPreviewProps = {
  complete: boolean;
  invalidMessage: string;
  source: string;
  theme: HTMLVisualThemeSnapshot;
  title: string;
};

type SVGPreview = {
  source: string;
  theme: HTMLVisualThemeSnapshot;
} & (
  | { status: "invalid" }
  | {
      status: "ready";
      url: string;
    }
);

const SVG_NAMESPACE = "http://www.w3.org/2000/svg";

function applyPreviewTheme(source: string, theme: HTMLVisualThemeSnapshot): string | null {
  const parser = new DOMParser();
  const xmlDocument = parser.parseFromString(source, "image/svg+xml");
  let root: Element = xmlDocument.documentElement;
  if (root.localName !== "svg") return null;

  if (root.namespaceURI === null) {
    const htmlDocument = parser.parseFromString(source, "text/html");
    const [htmlRoot] = Array.from(htmlDocument.body.children);
    if (htmlDocument.body.children.length !== 1 || htmlRoot?.localName !== "svg") return null;
    root = htmlRoot;
  } else if (root.namespaceURI !== SVG_NAMESPACE) {
    return null;
  }

  const style = root.ownerDocument.createElementNS(SVG_NAMESPACE, "style");
  const variables = theme.variables.map(([name, value]) => `${name}:${value}`).join(";");
  style.setAttribute("data-deeix-artifact-theme", "");
  style.textContent = `:root{color-scheme:${theme.colorScheme};${variables}}`;
  root.prepend(style);

  return new XMLSerializer().serializeToString(root);
}

export function ChatArtifactSVGPreview({
  complete,
  invalidMessage,
  source,
  theme,
  title,
}: ChatArtifactSVGPreviewProps) {
  const [preview, setPreview] = React.useState<SVGPreview | null>(null);
  const [failedURL, setFailedURL] = React.useState<string | null>(null);

  React.useEffect(() => {
    setFailedURL(null);
    if (!complete) {
      setPreview(null);
      return;
    }

    const previewSource = applyPreviewTheme(source, theme);
    if (!previewSource) {
      setPreview({ source, status: "invalid", theme });
      return;
    }

    // SVG image documents do not expose scripts, event handlers, or links as active page DOM.
    const url = URL.createObjectURL(
      new Blob([previewSource], { type: "image/svg+xml;charset=utf-8" }),
    );
    setPreview({ source, status: "ready", theme, url });

    return () => URL.revokeObjectURL(url);
  }, [complete, source, theme]);

  const currentPreview =
    complete && preview?.source === source && preview.theme === theme ? preview : null;
  const previewURL = currentPreview?.status === "ready" ? currentPreview.url : null;
  const failed =
    currentPreview?.status === "invalid" || Boolean(previewURL && failedURL === previewURL);

  return (
    <div className="flex h-full min-h-[320px] w-full items-center justify-center overflow-auto bg-background p-3">
      {previewURL && !failed ? (
        <img
          src={previewURL}
          alt={title}
          className="block max-h-full max-w-full select-none object-contain"
          decoding="async"
          draggable={false}
          referrerPolicy="no-referrer"
          onError={() => setFailedURL(previewURL)}
        />
      ) : complete && failed ? (
        <p className="px-6 text-center text-sm text-muted-foreground">{invalidMessage}</p>
      ) : null}
    </div>
  );
}
