"use client";

import * as React from "react";
import dynamic from "next/dynamic";
import { Download, FileX } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { formatBytes, resolveFileExtension, resolveFilePreviewKind } from "@/features/files/utils/file-display";
import { fetchFileContent, type FileContentResult } from "@/shared/api/file";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export type PreviewDialogFile = {
  fileID: string;
  fileName: string;
  mimeType: string;
  sizeBytes: number;
};

type PreviewState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "error"; message: string }
  | {
      status: "ready";
      kind: ReturnType<typeof resolveFilePreviewKind>;
      objectURL: string;
      textContent: string | null;
      contentType: string;
    };

type PreviewSourceProps = {
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
  return (
    <div className="flex min-h-[320px] flex-col gap-3 px-1 py-4">
      <Skeleton className="h-4 w-2/3 rounded-md" />
      <Skeleton className="h-4 w-full rounded-md" />
      <Skeleton className="h-4 w-4/5 rounded-md" />
      <Skeleton className="h-4 w-3/5 rounded-md" />
    </div>
  );
}

const PreviewDocx = dynamic<PreviewSourceProps>(
  () => import("@/features/files/components/preview/preview-docx").then((mod) => mod.PreviewDocx),
  { ssr: false, loading: PreviewRendererFallback },
);

const PreviewDocument = dynamic<PreviewDocumentProps>(
  () => import("@/features/files/components/preview/preview-document").then((mod) => mod.PreviewDocument),
  { ssr: false, loading: PreviewRendererFallback },
);

const PreviewMedia = dynamic<PreviewMediaProps>(
  () => import("@/features/files/components/preview/preview-media").then((mod) => mod.PreviewMedia),
  { ssr: false, loading: PreviewRendererFallback },
);

const PreviewPdf = dynamic<PreviewSourceProps>(
  () => import("@/features/files/components/preview/preview-pdf").then((mod) => mod.PreviewPdf),
  { ssr: false, loading: PreviewRendererFallback },
);

const PreviewSheet = dynamic<PreviewSourceProps>(
  () => import("@/features/files/components/preview/preview-sheet").then((mod) => mod.PreviewSheet),
  { ssr: false, loading: PreviewRendererFallback },
);

const PreviewText = dynamic<PreviewTextProps>(
  () => import("@/features/files/components/preview/preview-text").then((mod) => mod.PreviewText),
  { ssr: false, loading: PreviewRendererFallback },
);

function resolveFileExt(name: string): string {
  const ext = resolveFileExtension(name);
  return ext ? ext.toUpperCase().slice(0, 6) : "FILE";
}

function isReadableTextContent(content: string): boolean {
  const sample = content.slice(0, 4000);
  if (sample.length === 0) {
    return true;
  }

  let replacements = 0;
  let controls = 0;
  for (const char of sample) {
    const code = char.charCodeAt(0);
    if (char === "\uFFFD") {
      replacements += 1;
      continue;
    }
    if (code < 32 && code !== 9 && code !== 10 && code !== 13) {
      controls += 1;
    }
  }

  return replacements / sample.length < 0.08 && controls / sample.length < 0.02;
}

type FileContentLoader = (file: PreviewDialogFile) => Promise<FileContentResult>;

function useFilePreviewDialog(file: PreviewDialogFile | null, loadContent?: FileContentLoader) {
  const t = useTranslations("files.previewDialog");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const objectURLRef = React.useRef<string | null>(null);
  const [state, setState] = React.useState<PreviewState>({ status: "idle" });

  const revoke = React.useCallback(() => {
    if (!objectURLRef.current) {
      return;
    }
    URL.revokeObjectURL(objectURLRef.current);
    objectURLRef.current = null;
  }, []);

  React.useEffect(() => {
    if (!file) {
      revoke();
      setState({ status: "idle" });
      return undefined;
    }

    let cancelled = false;
    revoke();
    setState({ status: "loading" });

    void (async () => {
      try {
        const result = loadContent
          ? await loadContent(file)
          : await (async () => {
              const token = await resolveAccessToken();
              if (!token) {
                throw new Error(t("sessionExpired"));
              }
              return fetchFileContent(token, file.fileID);
            })();
        let kind = resolveFilePreviewKind(file, result.contentType);
        const objectURL = URL.createObjectURL(result.blob);
        objectURLRef.current = objectURL;

        let textContent: string | null = null;
        if (kind === "markdown" || kind === "code" || kind === "text" || kind === "unsupported") {
          const raw = await result.blob.text();
          if (isReadableTextContent(raw)) {
            textContent = raw;
            if (kind === "unsupported") {
              kind = "text";
            }
          } else {
            kind = "unsupported";
          }
        }

        if (cancelled) {
          URL.revokeObjectURL(objectURL);
          return;
        }

        setState({ status: "ready", kind, objectURL, textContent, contentType: result.contentType });
      } catch (error) {
        if (cancelled) {
          return;
        }
        setState({ status: "error", message: resolveErrorMessage(error, t("loadFailed")) });
      }
    })();

    return () => {
      cancelled = true;
      revoke();
    };
  }, [file, loadContent, resolveErrorMessage, revoke, t]);

  const download = React.useCallback(() => {
    if (state.status !== "ready" || !file) {
      return;
    }
    const anchor = document.createElement("a");
    anchor.href = state.objectURL;
    anchor.download = file.fileName;
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
  }, [file, state]);

  return { state, download };
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
  const activeFile = open ? file : null;
  const { state, download } = useFilePreviewDialog(activeFile, loadContent);

  const previewBody = React.useMemo(() => {
    if (!open || !file) {
      return null;
    }

    if (state.status === "loading") {
      return (
        <div className="flex min-h-[180px] flex-col gap-3 px-1 py-4 sm:min-h-[320px]">
          <Skeleton className="h-4 w-2/3 rounded-md" />
          <Skeleton className="h-4 w-full rounded-md" />
          <Skeleton className="h-4 w-4/5 rounded-md" />
          <Skeleton className="h-4 w-3/5 rounded-md" />
        </div>
      );
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
        <div className="overflow-hidden rounded-xl">
          <PreviewMedia kind="image" source={objectURL} alt={file.fileName} contentType={contentType} />
        </div>
      );
    }
    if (kind === "audio" || kind === "video") {
      return <PreviewMedia kind={kind} source={objectURL} alt={file.fileName} contentType={contentType} />;
    }
    if (kind === "pdf") {
      return <PreviewPdf source={objectURL} />;
    }
    if (kind === "docx") {
      return <PreviewDocx source={objectURL} />;
    }
    if (kind === "spreadsheet") {
      return <PreviewSheet source={objectURL} />;
    }
    if (kind === "native") {
      return <PreviewDocument source={objectURL} contentType={contentType} />;
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
  }, [allowDownload, download, file, open, state, t]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="flex max-h-[68dvh] w-[calc(100vw-1.5rem)] flex-col gap-0 overflow-hidden p-0 sm:max-h-[92vh] sm:max-w-2xl"
      >
        <DialogHeader className="flex flex-row items-center justify-between gap-3 border-b border-border/50 px-4 py-3 sm:gap-4 sm:px-5 sm:py-4">
          <div className="min-w-0 flex-1">
            <DialogTitle className="truncate text-sm font-medium leading-snug">
              {file?.fileName ?? t("fallbackTitle")}
            </DialogTitle>
            <DialogDescription className="sr-only">{t("description")}</DialogDescription>
            {file ? (
              <p className="mt-0.5 text-[11px] text-muted-foreground">
                {resolveFileExt(file.fileName)} · {formatBytes(file.sizeBytes)}
              </p>
            ) : null}
          </div>
          {allowDownload && state.status === "ready" ? (
            <Button size="sm" variant="ghost" onClick={download} className="shrink-0 gap-1.5 text-xs">
              <Download className="size-3.5" />
              {t("download")}
            </Button>
          ) : null}
        </DialogHeader>

        <div className="min-h-0 flex-1 overflow-hidden">
          <div className="h-full max-h-[calc(68dvh-64px)] overflow-auto sm:max-h-[calc(92vh-72px)]">
            <div className="px-3 py-3 sm:px-5 sm:py-5">{previewBody}</div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
