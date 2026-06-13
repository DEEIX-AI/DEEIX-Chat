"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { CenteredEmptyState } from "@/components/ui/empty-state";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { LoadingReveal } from "@/shared/components/loading-reveal";
import { ContentMeta } from "@/features/files/components/sections/content/content-meta";
import { PreviewDocument } from "@/shared/components/file-preview/preview-document";
import { PreviewDocx } from "@/shared/components/file-preview/preview-docx";
import { PreviewMedia } from "@/shared/components/file-preview/preview-media";
import { PreviewPdf } from "@/shared/components/file-preview/preview-pdf";
import { PreviewStageSkeleton } from "@/shared/components/file-preview/preview-skeleton";
import { PreviewSheet } from "@/shared/components/file-preview/preview-sheet";
import { PreviewText } from "@/shared/components/file-preview/preview-text";
import type { FileExtractState } from "@/features/files/hooks/use-file-extract";
import type { FilePreviewState } from "@/features/files/hooks/use-file-preview";
import type { FileObjectDTO } from "@/shared/api/file.types";
import { cn } from "@/lib/utils";

type ContentPreviewProps = {
  file: FileObjectDTO | null;
  preview: FilePreviewState;
  extract: FileExtractState;
  contentTab: "preview" | "extract";
  onContentTabChange: (value: "preview" | "extract") => void;
};

function PreviewEmpty({ title, description }: { title: string; description: string }) {
  return <CenteredEmptyState className="w-full" title={title} description={description} />;
}

export function ContentPreview({ file, preview, extract, contentTab, onContentTabChange }: ContentPreviewProps) {
  const t = useTranslations("files.preview");
  const [metaDrawerContainer, setMetaDrawerContainer] = React.useState<HTMLDivElement | null>(null);
  const [toolbarContainer, setToolbarContainer] = React.useState<HTMLDivElement | null>(null);
  const useInnerScrollRegion = contentTab === "preview" && preview.status === "ready" && preview.kind === "image";

  if (!file) {
    return (
      <CenteredEmptyState
        className="flex-1"
        title={t("workspaceTitle")}
        description={t("workspaceDescription")}
      />
    );
  }

  const previewContent =
    preview.status === "error" ? (
      <PreviewEmpty title={t("cannotPreview")} description={preview.message} />
    ) : preview.status === "ready" && preview.kind === "image" ? (
      <PreviewMedia
        kind="image"
        source={preview.objectURL}
        alt={file.fileName}
        contentType={preview.contentType}
        toolbarContainer={toolbarContainer}
      />
    ) : preview.status === "ready" && preview.kind === "pdf" ? (
      <PreviewPdf source={preview.objectURL} toolbarContainer={toolbarContainer} />
    ) : preview.status === "ready" && preview.kind === "docx" ? (
      <PreviewDocx source={preview.objectURL} toolbarContainer={toolbarContainer} />
    ) : preview.status === "ready" && preview.kind === "spreadsheet" ? (
      <PreviewSheet source={preview.objectURL} toolbarContainer={toolbarContainer} />
    ) : preview.status === "ready" && preview.kind === "native" ? (
      <PreviewDocument source={preview.objectURL} contentType={preview.contentType} />
    ) : preview.status === "ready" && (preview.kind === "audio" || preview.kind === "video") ? (
      <PreviewMedia
        kind={preview.kind}
        source={preview.objectURL}
        alt={file.fileName}
        contentType={preview.contentType}
        toolbarContainer={toolbarContainer}
      />
    ) : preview.status === "ready" &&
      (preview.kind === "markdown" || preview.kind === "code" || preview.kind === "text") ? (
      <div className="min-h-full rounded-[28px] bg-background/65">
        <PreviewText
          kind={preview.kind}
          content={preview.textContent ?? ""}
          className="min-h-full"
        />
      </div>
    ) : preview.status === "ready" && preview.kind === "unsupported" ? (
      <PreviewEmpty title={t("unsupportedTitle")} description={t("unsupportedDescription")} />
    ) : null;

  const extractContent =
    extract.status === "ready" && extract.data.extractText.trim() ? (
      <div className="min-h-full rounded-[28px] bg-background/65">
        <PreviewText kind="text" content={extract.data.extractText} className="min-h-full" />
      </div>
    ) : extract.status === "ready" ? (
      <PreviewEmpty title={t("noExtractTitle")} description={t("noExtractDescription")} />
    ) : extract.status === "error" ? (
      <PreviewEmpty title={t("noExtractTitle")} description={extract.message} />
    ) : null;

  return (
    <div ref={setMetaDrawerContainer} className="relative min-h-0 flex-1 overflow-hidden px-6 pt-5 pb-4">
      <div className="relative z-20 flex h-8 items-center justify-between gap-3">
        <Tabs
          value={contentTab}
          onValueChange={(value) => onContentTabChange(value as "preview" | "extract")}
          className="gap-0"
        >
          <TabsList className="h-8">
            <TabsTrigger value="preview">{t("preview")}</TabsTrigger>
            <TabsTrigger value="extract">{t("extract")}</TabsTrigger>
          </TabsList>
        </Tabs>
        <div ref={setToolbarContainer} className="flex min-h-8 shrink-0 items-center justify-end" />
      </div>

      <div
        className={cn(
          "absolute inset-x-6 top-16 bottom-16 min-h-0",
          useInnerScrollRegion ? "overflow-hidden" : "overflow-y-auto overflow-x-hidden",
        )}
      >
        {contentTab === "preview" ? previewContent : extractContent}

        <LoadingReveal
          loading={contentTab === "preview" ? preview.status === "loading" : extract.status === "loading" || extract.status === "idle"}
          className="pointer-events-none absolute inset-0 z-10"
          contentClassName="h-full"
          skeletonClassName="h-full"
          skeleton={<PreviewStageSkeleton className="h-full" />}
        >
          <div className="h-full" />
        </LoadingReveal>
      </div>

      <ContentMeta file={file} container={metaDrawerContainer} />
    </div>
  );
}
