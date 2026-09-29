"use client";

import * as React from "react";
import dynamic from "next/dynamic";
import { Download, FileX } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeightTransition,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";
import { formatBytes, resolveFileExtension } from "@/entities/file/lib/file-display";
import { PreviewLoading } from "@/entities/file/components/file-preview/preview-loading";
import {
  type FileContentLoader,
  type PreviewDialogFile,
  useFilePreviewDialog,
} from "@/entities/file/hooks/use-file-preview-dialog";

export type { FileContentLoader, PreviewDialogFile };

type PreviewViewerLoadingProps = {
  showLoading?: boolean;
  onLoadingChange?: (loading: boolean) => void;
};

type PreviewSourceProps = PreviewViewerLoadingProps & {
  source: string;
};

type PreviewDocumentProps = PreviewSourceProps & {
  contentType: string;
};

type PreviewMediaProps = PreviewDocumentProps & {
  kind: "image" | "audio" | "video";
  alt: string;
};

type PreviewTextProps = {
  kind: "markdown" | "code" | "text";
  content: string;
};

function PreviewRendererFallback() {
  return <PreviewLoading className="min-h-[180px] sm:min-h-[320px]" />;
}

const PreviewDocx = dynamic<PreviewSourceProps>(
  () => import("@/entities/file/components/file-preview/preview-docx").then((mod) => mod.PreviewDocx),
  { ssr: false, loading: PreviewRendererFallback },
);

const PreviewDocument = dynamic<PreviewDocumentProps>(
  () => import("@/entities/file/components/file-preview/preview-document").then((mod) => mod.PreviewDocument),
  { ssr: false, loading: PreviewRendererFallback },
);

const PreviewMedia = dynamic<PreviewMediaProps>(
  () => import("@/entities/file/components/file-preview/preview-media").then((mod) => mod.PreviewMedia),
  { ssr: false, loading: PreviewRendererFallback },
);

const PreviewPdf = dynamic<PreviewSourceProps>(
  () => import("@/entities/file/components/file-preview/preview-pdf").then((mod) => mod.PreviewPdf),
  { ssr: false, loading: PreviewRendererFallback },
);

const PreviewSheet = dynamic<PreviewSourceProps>(
  () => import("@/entities/file/components/file-preview/preview-sheet").then((mod) => mod.PreviewSheet),
  { ssr: false, loading: PreviewRendererFallback },
);

const PreviewText = dynamic<PreviewTextProps>(
  () => import("@/entities/file/components/file-preview/preview-text").then((mod) => mod.PreviewText),
  { ssr: false, loading: PreviewRendererFallback },
);

function resolveFileExt(name: string): string {
  const ext = resolveFileExtension(name);
  return ext ? ext.toUpperCase().slice(0, 6) : "FILE";
}

export function FilePreviewDialog({
  file,
  open,
  onOpenChange,
  loadContent,
  allowDownload = true,
}: {
  file: PreviewDialogFile | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  loadContent?: FileContentLoader;
  allowDownload?: boolean;
}) {
  const t = useTranslations("files.previewDialog");
  const commonT = useTranslations("common.actions");
  const [dialogFile, setDialogFile] = React.useState<PreviewDialogFile | null>(file);
  const activeFile = dialogFile;
  const { state, download } = useFilePreviewDialog(activeFile, loadContent);
  const [viewerLoading, setViewerLoading] = React.useState(false);
  const previewKind = state.status === "ready" ? state.kind : null;
  const showLoadingOverlay = Boolean(activeFile) && (state.status === "loading" || viewerLoading);

  React.useEffect(() => {
    if (open && file) {
      setDialogFile(file);
    }
  }, [file, open]);

  React.useEffect(() => {
    setViewerLoading(false);
  }, [activeFile?.fileID, state.status, previewKind]);

  const previewBody = React.useMemo(() => {
    if (!activeFile) {
      return null;
    }

    if (state.status === "error") {
      return (
        <div className="flex min-h-[180px] flex-col items-center justify-center gap-3 text-center sm:min-h-[280px]">
          <FileX className="size-10 text-muted-foreground/50" />
          <p className="text-sm font-medium text-foreground">{t("cannotPreview")}</p>
          <p className="max-w-[340px] text-xs text-muted-foreground">{state.message}</p>
        </div>
      );
    }

    if (state.status !== "ready") {
      return null;
    }

    const { kind, objectURL, textContent, contentType } = state;

    if (kind === "image") {
      return (
        <div className="overflow-hidden rounded-md">
          <PreviewMedia kind="image" source={objectURL} alt={activeFile.fileName} contentType={contentType} />
        </div>
      );
    }
    if (kind === "audio" || kind === "video") {
      return <PreviewMedia kind={kind} source={objectURL} alt={activeFile.fileName} contentType={contentType} />;
    }
    if (kind === "pdf") {
      return <PreviewPdf source={objectURL} showLoading={false} onLoadingChange={setViewerLoading} />;
    }
    if (kind === "docx") {
      return <PreviewDocx source={objectURL} showLoading={false} onLoadingChange={setViewerLoading} />;
    }
    if (kind === "spreadsheet") {
      return <PreviewSheet source={objectURL} showLoading={false} onLoadingChange={setViewerLoading} />;
    }
    if (kind === "native") {
      return (
        <PreviewDocument
          source={objectURL}
          contentType={contentType}
          showLoading={false}
          onLoadingChange={setViewerLoading}
        />
      );
    }
    if (kind === "markdown" || kind === "code" || kind === "text") {
      return <PreviewText kind={kind} content={textContent ?? ""} />;
    }

    return (
      <div className="flex min-h-[180px] flex-col items-center justify-center gap-3 text-center sm:min-h-[280px]">
        <FileX className="size-10 text-muted-foreground/50" />
        <p className="text-sm font-medium text-foreground">{t("unsupported")}</p>
        {allowDownload ? (
          <>
            <p className="text-xs text-muted-foreground">{t("downloadHint")}</p>
            <Button size="sm" variant="outline" onClick={download} className="mt-2 gap-1.5">
              <Download className="size-3.5" />
              {t("downloadFile")}
            </Button>
          </>
        ) : (
          <p className="text-xs text-muted-foreground">{t("downloadUnavailableForShare")}</p>
        )}
      </div>
    );
  }, [activeFile, allowDownload, download, state, t]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="w-[calc(100vw-2rem)] gap-0 overflow-hidden p-0 sm:max-w-[720px]"
        onCloseAutoFocus={() => {
          setDialogFile(null);
          setViewerLoading(false);
        }}
      >
        <DialogHeightTransition contentClassName="max-h-[min(86svh,760px)]">
          <DialogHeader className="flex-row items-start justify-between gap-3 px-5 pb-3 pt-5 sm:gap-4">
            <div className="min-w-0 flex-1">
              <DialogTitle className="truncate">
                {activeFile?.fileName ?? t("fallbackTitle")}
              </DialogTitle>
              <DialogDescription className="sr-only">{t("description")}</DialogDescription>
              {activeFile ? (
                <p className="mt-0.5 text-[11px] text-muted-foreground">
                  {resolveFileExt(activeFile.fileName)} · {formatBytes(activeFile.sizeBytes)}
                </p>
              ) : null}
            </div>
            {allowDownload && state.status === "ready" ? (
              <Button type="button" variant="ghost" size="sm" onClick={download} className="shrink-0 gap-1.5">
                <Download className="size-3.5" />
                {t("download")}
              </Button>
            ) : null}
          </DialogHeader>

          <div className="min-h-0 flex-1 overflow-y-auto px-5 py-2">
            <div
              className={cn(
                "relative",
                showLoadingOverlay && "min-h-[180px] sm:min-h-[320px]",
              )}
            >
              {previewBody}
              {showLoadingOverlay ? (
                <PreviewLoading className="pointer-events-none absolute inset-0 z-10" />
              ) : null}
            </div>
          </div>

          <DialogFooter className="shrink-0 px-5 py-3">
            <Button type="button" variant="ghost" size="sm" onClick={() => onOpenChange(false)}>
              {commonT("close")}
            </Button>
          </DialogFooter>
        </DialogHeightTransition>
      </DialogContent>
    </Dialog>
  );
}
