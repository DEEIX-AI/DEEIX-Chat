"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { CenteredEmptyState } from "@/components/ui/empty-state";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ContentMeta } from "@/features/files/components/sections/content/content-meta";
import { PreviewDocument } from "@/shared/components/file-preview/preview-document";
import { PreviewDocx } from "@/shared/components/file-preview/preview-docx";
import { PreviewLoading } from "@/shared/components/file-preview/preview-loading";
import { PreviewMedia } from "@/shared/components/file-preview/preview-media";
import { PreviewPdf } from "@/shared/components/file-preview/preview-pdf";
import { PreviewSheet } from "@/shared/components/file-preview/preview-sheet";
import { PreviewText } from "@/shared/components/file-preview/preview-text";
import type { FileExtractState } from "@/features/files/hooks/use-file-extract";
import type { FilePreviewState } from "@/features/files/hooks/use-file-preview";
import type { FileObjectDTO } from "@/shared/api/file.types";
import { cn } from "@/lib/utils";

type ContentPreviewProps = {
  file: FileObjectDTO | null;
  deferEmptyState: boolean;
  preview: FilePreviewState;
  extract: FileExtractState;
  contentTab: "preview" | "extract";
  onContentTabChange: (value: "preview" | "extract") => void;
};

function PreviewEmpty({ title, description }: { title: string; description: string }) {
  return <CenteredEmptyState className="w-full" title={title} description={description} />;
}

export function ContentPreview({ file, deferEmptyState, preview, extract, contentTab, onContentTabChange }: ContentPreviewProps) {
  const t = useTranslations("files.preview");
  const [metaDrawerContainer, setMetaDrawerContainer] = React.useState<HTMLDivElement | null>(null);
  const [toolbarContainer, setToolbarContainer] = React.useState<HTMLDivElement | null>(null);
  const [viewerLoading, setViewerLoading] = React.useState(false);
  const useInnerScrollRegion = contentTab === "preview" && preview.status === "ready" && preview.kind === "image";
  const previewKind = preview.status === "ready" ? preview.kind : null;
  const previewLoading = contentTab === "preview" ? preview.status === "loading" || viewerLoading : extract.status === "loading" || extract.status === "idle";

  React.useEffect(() => {
    setViewerLoading(false);
  }, [contentTab, file?.fileID, preview.status, previewKind]);

  if (!file) {
    if (deferEmptyState) {
      return <div className="min-h-0 flex-1" />;
    }

    return (
      <CenteredEmptyState
        className="flex-1"
        title={t("workspaceTitle")}
        description={t("workspaceDescription")}
      />
    );
  }

  let previewContent: React.ReactNode = null;
  if (preview.status === "error") {
    previewContent = <PreviewEmpty title={t("cannotPreview")} description={preview.message} />;
  } else if (preview.status === "ready") {
    if (preview.kind === "image") {
      previewContent = (
        <PreviewMedia
          kind="image"
          source={preview.objectURL}
          alt={file.fileName}
          contentType={preview.contentType}
          toolbarContainer={toolbarContainer}
        />
      );
    } else if (preview.kind === "pdf") {
      previewContent = (
        <PreviewPdf
          source={preview.objectURL}
          toolbarContainer={toolbarContainer}
          showLoading={false}
          onLoadingChange={setViewerLoading}
        />
      );
    } else if (preview.kind === "docx") {
      previewContent = (
        <PreviewDocx
          source={preview.objectURL}
          toolbarContainer={toolbarContainer}
          showLoading={false}
          onLoadingChange={setViewerLoading}
        />
      );
    } else if (preview.kind === "spreadsheet") {
      previewContent = (
        <PreviewSheet
          source={preview.objectURL}
          toolbarContainer={toolbarContainer}
          showLoading={false}
          onLoadingChange={setViewerLoading}
        />
      );
    } else if (preview.kind === "native") {
      previewContent = (
        <PreviewDocument
          source={preview.objectURL}
          contentType={preview.contentType}
          showLoading={false}
          onLoadingChange={setViewerLoading}
        />
      );
    } else if (preview.kind === "audio" || preview.kind === "video") {
      previewContent = (
        <PreviewMedia
          kind={preview.kind}
          source={preview.objectURL}
          alt={file.fileName}
          contentType={preview.contentType}
          toolbarContainer={toolbarContainer}
        />
      );
    } else if (preview.kind === "markdown" || preview.kind === "code" || preview.kind === "text") {
      previewContent = (
        <div className="min-h-full rounded-md bg-background/65">
          <PreviewText
            kind={preview.kind}
            content={preview.textContent ?? ""}
            className="min-h-full"
          />
        </div>
      );
    } else if (preview.kind === "unsupported") {
      previewContent = <PreviewEmpty title={t("unsupportedTitle")} description={t("unsupportedDescription")} />;
    }
  }

  let extractContent: React.ReactNode = null;
  if (extract.status === "ready" && extract.data.extractText.trim()) {
    extractContent = (
      <div className="min-h-full rounded-md bg-background/65">
        <PreviewText kind="text" content={extract.data.extractText} className="min-h-full" />
      </div>
    );
  } else if (extract.status === "ready") {
    extractContent = <PreviewEmpty title={t("noExtractTitle")} description={t("noExtractDescription")} />;
  } else if (extract.status === "error") {
    extractContent = <PreviewEmpty title={t("noExtractTitle")} description={extract.message} />;
  }

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

        {previewLoading ? (
          <PreviewLoading
            className="pointer-events-none absolute inset-0 z-10"
          />
        ) : null}
      </div>

      <ContentMeta file={file} container={metaDrawerContainer} />
    </div>
  );
}
