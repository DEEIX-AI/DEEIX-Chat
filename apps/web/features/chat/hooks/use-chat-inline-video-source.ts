"use client";

import { useTranslations } from "next-intl";
import * as React from "react";

import type { MessageAttachment } from "@/features/chat/types/messages";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { fetchFileContent } from "@/shared/api/file";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import type { FileContentLoader } from "@/entities/file";

export type InlineVideoPreviewState =
  | { status: "loading" }
  | { status: "error"; message: string }
  | { status: "ready"; source: string; contentType: string };

/**
 * Resolves a playable source for an inline video attachment: the server preview URL
 * when present, otherwise the file bytes as an object URL revoked on change/unmount.
 */
export function useChatInlineVideoSource(
  attachment: MessageAttachment,
  loadContent?: FileContentLoader,
): InlineVideoPreviewState {
  const tPreview = useTranslations("files.previewDialog");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const objectURLRef = React.useRef<string | null>(null);
  const fileID = attachment.fileID;
  const fileName = attachment.fileName;
  const mimeType = attachment.mimeType;
  const detectedMime = attachment.detectedMime;
  const previewURL = attachment.previewURL;
  const sizeBytes = attachment.sizeBytes;
  const [state, setState] = React.useState<InlineVideoPreviewState>(() =>
    previewURL
      ? {
          status: "ready",
          source: previewURL,
          contentType: detectedMime || mimeType,
        }
      : { status: "loading" },
  );
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

    if (previewURL) {
      setState({
        status: "ready",
        source: previewURL,
        contentType: detectedMime || mimeType,
      });
      return undefined;
    }

    setState({ status: "loading" });
    void (async () => {
      try {
        const file = {
          fileID,
          fileName,
          mimeType,
          sizeBytes,
        };
        const result = loadContent
          ? await loadContent(file, controller.signal)
          : await (async () => {
              const token = await resolveAccessToken();
              if (!token) {
                throw new Error(tPreview("sessionExpired"));
              }
              return fetchFileContent(token, fileID, controller.signal);
            })();
        const objectURL = URL.createObjectURL(result.blob);
        objectURLRef.current = objectURL;

        if (cancelled || controller.signal.aborted) {
          URL.revokeObjectURL(objectURL);
          if (objectURLRef.current === objectURL) {
            objectURLRef.current = null;
          }
          return;
        }

        setState({
          status: "ready",
          source: objectURL,
          contentType: result.contentType || detectedMime || mimeType,
        });
      } catch (error) {
        if (cancelled || controller.signal.aborted) {
          return;
        }
        setState({ status: "error", message: resolveErrorMessage(error, tPreview("loadFailed")) });
      }
    })();

    return () => {
      cancelled = true;
      controller.abort();
      revokeObjectURL();
    };
  }, [
    detectedMime,
    fileID,
    fileName,
    loadContent,
    mimeType,
    previewURL,
    resolveErrorMessage,
    revokeObjectURL,
    sizeBytes,
    tPreview,
  ]);

  return state;
}
