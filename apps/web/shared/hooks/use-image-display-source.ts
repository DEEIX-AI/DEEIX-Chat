import * as React from "react";

import {
  loadProtectedMarkdownImageBlobURL,
  resolveMarkdownImageSource,
  resolveProtectedMarkdownImageSource,
} from "@/shared/lib/markdown-image-source";

export type ImageDisplaySource = {
  // What an <img> can load: the resolved URL, or a blob URL once a protected
  // file has been fetched with credentials. Empty while that fetch runs.
  displaySrc: string;
  resolvedSrc: string;
  // Set for conversation files, which need the bearer token and so cannot be an <img> src.
  protectedSrc: string | null;
  // The credentialed fetch failed. Plain URLs report failures through the <img> error event.
  fetchFailed: boolean;
};

/** Resolves an image reference from model output (absolute URL or `/api/v1/...` path) to a loadable src. */
export function useImageDisplaySource(src: string | undefined): ImageDisplaySource {
  const resolvedSrc = React.useMemo(() => (src ? resolveMarkdownImageSource(src) : ""), [src]);
  const protectedSrc = React.useMemo(() => (src ? resolveProtectedMarkdownImageSource(src) : null), [src]);
  const [blob, setBlob] = React.useState<{ source: string; url: string } | null>(null);
  const [failedSource, setFailedSource] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!protectedSrc) {
      return undefined;
    }
    const controller = new AbortController();
    let blobURL = "";
    void loadProtectedMarkdownImageBlobURL(protectedSrc, controller.signal)
      .then((nextBlobURL) => {
        blobURL = nextBlobURL;
        if (controller.signal.aborted) {
          URL.revokeObjectURL(blobURL);
          return;
        }
        setBlob({ source: protectedSrc, url: blobURL });
      })
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") {
          return;
        }
        setFailedSource(protectedSrc);
      });
    return () => {
      controller.abort();
      if (blobURL) {
        URL.revokeObjectURL(blobURL);
      }
    };
  }, [protectedSrc]);

  // Results are keyed by the source they belong to, so a changed src never shows the previous image.
  const displaySrc = protectedSrc ? (blob?.source === protectedSrc ? blob.url : "") : resolvedSrc;
  return { displaySrc, resolvedSrc, protectedSrc, fetchFailed: protectedSrc !== null && failedSource === protectedSrc };
}
