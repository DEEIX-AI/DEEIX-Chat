"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import { Maximize2, Minimize2, Minus, Plus } from "lucide-react";
import { useTranslations } from "next-intl";

import { resolveLocalizedErrorMessage } from "@/i18n/resolve-error-message";
import { Button } from "@/components/ui/button";
import { useFileScale } from "@/shared/components/file-preview/file-scale";
import { PreviewLoading } from "@/shared/components/file-preview/preview-loading";

type PreviewPdfProps = {
  source: string;
  toolbarContainer?: HTMLElement | null;
  showLoading?: boolean;
  onLoadingChange?: (loading: boolean) => void;
};

type PdfModule = typeof import("pdfjs-dist/build/pdf.mjs");
type PdfDocument = Awaited<ReturnType<PdfModule["getDocument"]>["promise"]>;
type PdfPage = Awaited<ReturnType<PdfDocument["getPage"]>>;

const PDF_ZOOM = {
  min: 0.5,
  max: 1,
  step: 0.1,
  initial: 0.8,
};

function resolvePdfErrorMessage(error: unknown, t: (key: string) => string): string {
  if (!(error instanceof Error)) {
    return t("pdfLoadFailed");
  }

  const name = error.name;
  const message = error.message?.trim() || "";

  if (name === "PasswordException") {
    return t("pdfPassword");
  }

  if (name === "InvalidPDFException") {
    return t("pdfInvalid");
  }

  if (name === "ResponseException") {
    return t("pdfReadFailed");
  }

  if (message) {
    return resolveLocalizedErrorMessage(error, message);
  }

  return t("pdfLoadFailed");
}

export function PreviewPdf({ source, toolbarContainer, showLoading = true, onLoadingChange }: PreviewPdfProps) {
  const t = useTranslations("files.previewErrors");
  const tPreview = useTranslations("files.preview");
  const canvasRefs = React.useRef<Array<HTMLCanvasElement | null>>([]);
  const containerRef = React.useRef<HTMLDivElement | null>(null);
  const scrollRegionRef = React.useRef<HTMLDivElement | null>(null);
  const renderTokenRef = React.useRef(0);
  const [pdfModule, setPdfModule] = React.useState<PdfModule | null>(null);
  const [documentProxy, setDocumentProxy] = React.useState<PdfDocument | null>(null);
  const [pageCount, setPageCount] = React.useState(0);
  const {
    scale: zoom,
    zoomOut,
    zoomIn,
    canZoomOut,
    canZoomIn,
  } = useFileScale(PDF_ZOOM);
  const [availableWidth, setAvailableWidth] = React.useState(0);
  const [isFullscreen, setIsFullscreen] = React.useState(false);
  const [status, setStatus] = React.useState<"loading" | "ready" | "error">("loading");
  const [errorMessage, setErrorMessage] = React.useState(t("pdfLoadFailed"));
  const measureKey = status === "ready" ? pageCount : 0;

  React.useEffect(() => {
    onLoadingChange?.(status === "loading");
  }, [onLoadingChange, status]);

  React.useEffect(() => {
    canvasRefs.current.length = pageCount;
  }, [pageCount]);

  React.useEffect(() => {
    const handleFullscreenChange = () => {
      setIsFullscreen(document.fullscreenElement === containerRef.current);
    };

    document.addEventListener("fullscreenchange", handleFullscreenChange);
    return () => {
      document.removeEventListener("fullscreenchange", handleFullscreenChange);
    };
  }, []);

  React.useEffect(() => {
    if (measureKey === 0) {
      setAvailableWidth(0);
      return undefined;
    }

    const node = scrollRegionRef.current;
    if (!node) {
      return undefined;
    }

    const updateWidth = () => {
      const nextWidth = Math.max(node.clientWidth - 8, 0);
      setAvailableWidth(nextWidth);
    };

    updateWidth();

    const observer = new ResizeObserver(() => {
      updateWidth();
    });
    observer.observe(node);

    return () => {
      observer.disconnect();
    };
  }, [measureKey]);

  React.useEffect(() => {
    let cancelled = false;

    void (async () => {
      try {
        setStatus("loading");
        const mod = await import("pdfjs-dist/build/pdf.mjs");
        mod.GlobalWorkerOptions.workerSrc = new URL("pdfjs-dist/build/pdf.worker.min.mjs", import.meta.url).toString();

        if (!cancelled) {
          setPdfModule(mod);
        }
      } catch (error) {
        if (cancelled) {
          return;
        }
        setStatus("error");
        setErrorMessage(resolvePdfErrorMessage(error, t));
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [t]);

  React.useEffect(() => {
    if (!pdfModule) {
      return undefined;
    }

    let cancelled = false;
    let loadingTask: ReturnType<PdfModule["getDocument"]> | null = null;

    void (async () => {
      try {
        setStatus("loading");
        const response = await fetch(source);
        const arrayBuffer = await response.arrayBuffer();
        loadingTask = pdfModule.getDocument({
          data: new Uint8Array(arrayBuffer),
          cMapUrl: "/pdfjs/cmaps/",
          cMapPacked: true,
          standardFontDataUrl: "/pdfjs/standard_fonts/",
          useSystemFonts: true,
          enableXfa: true,
          stopAtErrors: false,
        });
        const pdf = await loadingTask.promise;

        if (cancelled) {
          await pdf.destroy();
          return;
        }

        setDocumentProxy((current) => {
          if (current) {
            void current.destroy();
          }
          return pdf;
        });
        setPageCount(pdf.numPages);
        setStatus("ready");
      } catch (error) {
        if (cancelled) {
          return;
        }
        setStatus("error");
        setErrorMessage(resolvePdfErrorMessage(error, t));
      }
    })();

    return () => {
      cancelled = true;
      if (loadingTask) {
        void loadingTask.destroy();
      }
    };
  }, [pdfModule, source, t]);

  React.useEffect(() => {
    if (!documentProxy || status !== "ready") {
      return undefined;
    }

    if (!availableWidth) {
      return undefined;
    }

    const currentToken = renderTokenRef.current + 1;
    renderTokenRef.current = currentToken;
    const renderTasks: Array<{ cancel: () => void }> = [];
    const renderedPages: PdfPage[] = [];

    void (async () => {
      for (let pageNumber = 1; pageNumber <= documentProxy.numPages; pageNumber += 1) {
        if (renderTokenRef.current !== currentToken) {
          break;
        }

        const canvas = canvasRefs.current[pageNumber - 1];
        if (!canvas) {
          continue;
        }

        const page = await documentProxy.getPage(pageNumber);
        if (renderTokenRef.current !== currentToken) {
          page.cleanup();
          break;
        }
        renderedPages.push(page);
        const naturalViewport = page.getViewport({ scale: 1 });
        const fitScale = naturalViewport.width > 0 ? availableWidth / naturalViewport.width : 1;
        const viewport = page.getViewport({ scale: fitScale * zoom });
        const context = canvas.getContext("2d");
        if (!context) {
          page.cleanup();
          renderedPages.splice(renderedPages.indexOf(page), 1);
          continue;
        }

        const ratio = window.devicePixelRatio || 1;
        canvas.width = Math.floor(viewport.width * ratio);
        canvas.height = Math.floor(viewport.height * ratio);
        canvas.style.width = `${viewport.width}px`;
        canvas.style.height = `${viewport.height}px`;
        context.setTransform(ratio, 0, 0, ratio, 0, 0);

        const renderTask = page.render({
          canvasContext: context,
          viewport,
        });

        renderTasks.push(renderTask);

        try {
          await renderTask.promise;
        } catch {
          if (renderTokenRef.current !== currentToken) {
            break;
          }
        } finally {
          page.cleanup();
          renderedPages.splice(renderedPages.indexOf(page), 1);
        }
      }
    })();

    return () => {
      renderTokenRef.current += 1;
      renderTasks.forEach((task) => task.cancel());
      renderedPages.forEach((page) => page.cleanup());
    };
  }, [availableWidth, documentProxy, status, zoom]);

  React.useEffect(() => {
    return () => {
      if (documentProxy) {
        void documentProxy.destroy();
      }
    };
  }, [documentProxy]);

  const toggleFullscreen = React.useCallback(async () => {
    const element = containerRef.current;
    if (!element) {
      return;
    }

    if (document.fullscreenElement === element) {
      await document.exitFullscreen();
      return;
    }

    await element.requestFullscreen();
  }, []);

  const toolbar = (
    <div className="flex items-center gap-1.5">
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="size-7 rounded-full"
        aria-label={tPreview("zoomOut")}
        onClick={zoomOut}
        disabled={status !== "ready" || !canZoomOut}
      >
        <Minus className="size-3.5" />
      </Button>
      <span className="min-w-11 text-center text-[11px] text-muted-foreground">{Math.round(zoom * 100)}%</span>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="size-7 rounded-full"
        aria-label={tPreview("zoomIn")}
        onClick={zoomIn}
        disabled={status !== "ready" || !canZoomIn}
      >
        <Plus className="size-3.5" />
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="size-7 rounded-full"
        aria-label={isFullscreen ? tPreview("exitFullscreen") : tPreview("enterFullscreen")}
        onClick={() => void toggleFullscreen()}
        disabled={status !== "ready"}
      >
        {isFullscreen ? <Minimize2 className="size-3.5" /> : <Maximize2 className="size-3.5" />}
      </Button>
    </div>
  );

  return (
    <div ref={containerRef} className="flex h-full min-h-0 flex-col bg-background">
      {toolbarContainer ? createPortal(toolbar, toolbarContainer) : (
        <div className="flex shrink-0 items-center justify-end gap-1.5 px-1 py-2">{toolbar}</div>
      )}

      <div className="relative min-h-0 flex-1">
        {status === "error" ? (
          <div className="flex min-h-0 h-full items-center justify-center px-6">
            <div className="flex max-w-[420px] flex-col items-center gap-3 text-center">
              <p className="text-sm text-muted-foreground">{errorMessage}</p>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-8 rounded-full px-3 text-xs"
                onClick={() => window.open(source, "_blank", "noopener,noreferrer")}
              >
                {t("pdfOpenOriginal")}
              </Button>
            </div>
          </div>
        ) : null}

        {status === "ready" ? (
          <div ref={scrollRegionRef} className="min-h-0 h-full flex-1 overflow-auto">
              <div className="px-1 pb-2">
                <div className="mx-auto flex min-w-full w-max flex-col gap-4">
                  {Array.from({ length: pageCount }, (_, index) => (
                    <div key={index} className="flex justify-center">
                      <canvas
                        ref={(node) => {
                          canvasRefs.current[index] = node;
                        }}
                        className="block shrink-0"
                      />
                    </div>
                  ))}
                </div>
              </div>
          </div>
        ) : null}

        {showLoading && status === "loading" ? (
          <PreviewLoading className="pointer-events-none absolute inset-0 z-10" />
        ) : null}
      </div>
    </div>
  );
}
