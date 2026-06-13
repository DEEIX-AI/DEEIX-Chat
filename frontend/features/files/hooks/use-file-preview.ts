"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { fetchFileContent } from "@/shared/api/file";
import type { FileObjectDTO } from "@/shared/api/file.types";

import type { FilePreviewKind } from "@/features/files/types/files";
import { isFileReady, isImageFile, resolveFileExtension, resolveFilePreviewKind } from "@/shared/lib/file-display";

function isReadableTextContent(content: string): boolean {
  if (!content) {
    return true;
  }

  const sample = content.slice(0, 4000);
  if (!sample) {
    return true;
  }

  let replacementCount = 0;
  let controlCount = 0;

  for (const char of sample) {
    const code = char.charCodeAt(0);
    if (char === "\uFFFD") {
      replacementCount += 1;
      continue;
    }

    const isAllowedWhitespace = code === 9 || code === 10 || code === 13;
    const isControl = code < 32 && !isAllowedWhitespace;
    if (isControl || code === 127) {
      controlCount += 1;
    }
  }

  const replacementRatio = replacementCount / sample.length;
  const controlRatio = controlCount / sample.length;
  return replacementRatio < 0.08 && controlRatio < 0.02;
}

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

        const result = await fetchFileContent(accessToken, file.fileID);
        let kind = resolveFilePreviewKind(file, result.contentType);
        const objectURL = URL.createObjectURL(result.blob);
        objectURLRef.current = objectURL;

        let textContent: string | null = null;
        if (["markdown", "code", "text"].includes(kind)) {
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

        if (cancelled) {
          URL.revokeObjectURL(objectURL);
          if (objectURLRef.current === objectURL) {
            objectURLRef.current = null;
          }
          return;
        }

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
        if (cancelled) {
          return;
        }

        const message = resolveErrorMessage(error, t("previewLoadFailed"));
        setPreview({ status: "error", message });
        toast.error(t("previewLoadFailed"), { description: message });
      }
    })();

    return () => {
      cancelled = true;
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
