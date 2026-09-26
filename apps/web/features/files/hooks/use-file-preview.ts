"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { fetchFileContent } from "@/shared/api/file";
import type { FileObjectDTO } from "@/shared/api/file.types";

import type { FilePreviewKind } from "@/features/files/types/files";
import { isFileReady, isImageFile, isReadableTextContent, resolveFileExtension, resolveFilePreviewKind } from "@/shared/lib/file-display";

async function tryReadTextPreview(blob: Blob): Promise<{ textContent: string | null }> {
  const textContent = await blob.text();
  if (!isReadableTextContent(textContent)) {
    return {
      textContent: null,
    };
  }

  return {
    textContent,
  };
}

type FilePreviewState =
  | {
      status: "idle";
    }
  | {
      status: "loading";
    }
  | {
      status: "error";
      message: string;
    }
  | {
      status: "ready";
      kind: FilePreviewKind;
      objectURL: string;
      textContent: string | null;
      contentType: string;
      contentLength: number | null;
      extension: string;
      isImage: boolean;
    };

type UseFilePreviewOptions = {
  file: FileObjectDTO | null;
  getAccessToken: () => Promise<string>;
};

export function useFilePreview({ file, getAccessToken }: UseFilePreviewOptions) {
  const t = useTranslations("files.toasts");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const objectURLRef = React.useRef<string | null>(null);
  const [preview, setPreview] = React.useState<FilePreviewState>({ status: "idle" });
  const previewKey = file ? `${file.fileID}:${file.status}:${file.updatedAt}` : "";

  const revokeObjectURL = React.useCallback(() => {
    if (!objectURLRef.current) {
      return;
    }
    URL.revokeObjectURL(objectURLRef.current);
    objectURLRef.current = null;
  }, []);

  React.useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();

    revokeObjectURL();

    if (!file) {
      setPreview({ status: "idle" });
      return undefined;
    }

    if (!isFileReady(file.status)) {
      setPreview({
        status: "error",
        message: file.status.trim().toLowerCase() === "failed" ? t("previewProcessingFailed") : t("previewProcessing"),
      });
      return undefined;
    }

    setPreview({ status: "loading" });

    void (async () => {
      try {
        const accessToken = await getAccessToken();
        if (!accessToken) {
          throw new Error(t("viewAfterLogin"));
        }

        const result = await fetchFileContent(accessToken, file.fileID, controller.signal);
        let kind = resolveFilePreviewKind(file, result.contentType);
        const objectURL = URL.createObjectURL(result.blob);

        let textContent: string | null = null;
        if (kind === "markdown" || kind === "code" || kind === "text") {
          const textPreview = await tryReadTextPreview(result.blob);
          textContent = textPreview.textContent;
          if (textContent === null) {
            kind = "unsupported";
          }
        }

        if (kind === "unsupported") {
          const textPreview = await tryReadTextPreview(result.blob);
          if (textPreview.textContent !== null) {
            kind = "text";
            textContent = textPreview.textContent;
          }
        }

        if (cancelled || controller.signal.aborted) {
          URL.revokeObjectURL(objectURL);
          return;
        }

        objectURLRef.current = objectURL;
        setPreview({
          status: "ready",
          kind,
          objectURL,
          textContent,
          contentType: result.contentType,
          contentLength: result.contentLength,
          extension: resolveFileExtension(file.fileName),
          isImage: isImageFile(file),
        });
      } catch (error) {
        if (cancelled || controller.signal.aborted) {
          return;
        }

        const message = resolveErrorMessage(error, t("previewLoadFailed"));
        setPreview({ status: "error", message });
        toast.error(t("previewLoadFailed"), { description: message });
      }
    })();

    return () => {
      cancelled = true;
      controller.abort();
      revokeObjectURL();
    };
  }, [file, getAccessToken, previewKey, resolveErrorMessage, revokeObjectURL, t]);

  const open = React.useCallback(() => {
    if (preview.status !== "ready") {
      return;
    }
    window.open(preview.objectURL, "_blank", "noopener,noreferrer");
  }, [preview]);

  const download = React.useCallback(() => {
    if (preview.status !== "ready" || !file) {
      return;
    }

    const anchor = document.createElement("a");
    anchor.href = preview.objectURL;
    anchor.download = file.fileName;
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
  }, [file, preview]);

  return {
    preview,
    open,
    download,
  };
}

export type { FilePreviewState };
