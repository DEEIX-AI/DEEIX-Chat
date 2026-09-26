"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { fetchFileExtract } from "@/shared/api/file";
import type { FileExtractDTO, FileObjectDTO } from "@/shared/api/file.types";

type FileExtractState =
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
      data: FileExtractDTO;
    };

type UseFileExtractOptions = {
  file: FileObjectDTO | null;
  enabled: boolean;
  getAccessToken: () => Promise<string>;
};

export function useFileExtract({ file, enabled, getAccessToken }: UseFileExtractOptions) {
  const t = useTranslations("files.toasts");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [extract, setExtract] = React.useState<FileExtractState>({ status: "idle" });
  const extractKey = file ? `${file.fileID}:${file.extractStatus}:${file.updatedAt}` : "";

  React.useEffect(() => {
    let cancelled = false;

    if (!file) {
      setExtract({ status: "idle" });
      return undefined;
    }

    if (!enabled) {
      return undefined;
    }

    if (file.extractStatus !== "ready") {
      setExtract({
        status: "error",
        message: file.processingStatus === "failed" ? t("extractProcessingFailed") : t("extractProcessing"),
      });
      return undefined;
    }

    setExtract({ status: "loading" });

    void (async () => {
      try {
        const accessToken = await getAccessToken();
        if (!accessToken) {
          throw new Error(t("viewAfterLogin"));
        }

        const data = await fetchFileExtract(accessToken, file.fileID);
        if (cancelled) {
          return;
        }
        setExtract({ status: "ready", data });
      } catch (error) {
        if (cancelled) {
          return;
        }
        const message = resolveErrorMessage(error, t("extractLoadFailed"));
        setExtract({ status: "error", message });
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [enabled, extractKey, file, getAccessToken, resolveErrorMessage, t]);

  return extract;
}

export type { FileExtractState };
