"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { fetchFileContent, type FileContentResult } from "@/shared/api/file";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { isReadableTextContent, resolveFilePreviewKind } from "@/entities/file/lib/file-display";

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

export type FileContentLoader = (
  file: PreviewDialogFile,
  signal: AbortSignal,
) => Promise<FileContentResult>;

export function useFilePreviewDialog(file: PreviewDialogFile | null, loadContent?: FileContentLoader) {
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
    const controller = new AbortController();
    revoke();
    setState({ status: "loading" });

    void (async () => {
      try {
        const result = loadContent
          ? await loadContent(file, controller.signal)
          : await (async () => {
              const token = await resolveAccessToken();
              if (!token) {
                throw new Error(t("sessionExpired"));
              }
              return fetchFileContent(token, file.fileID, controller.signal);
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

        if (cancelled || controller.signal.aborted) {
          URL.revokeObjectURL(objectURL);
          return;
        }

        setState({ status: "ready", kind, objectURL, textContent, contentType: result.contentType });
      } catch (error) {
        if (cancelled || controller.signal.aborted) {
          return;
        }
        setState({ status: "error", message: resolveErrorMessage(error, t("loadFailed")) });
      }
    })();

    return () => {
      cancelled = true;
      controller.abort();
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
