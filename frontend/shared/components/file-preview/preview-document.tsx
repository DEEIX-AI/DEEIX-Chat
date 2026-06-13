"use client";

import * as React from "react";
import { FileText } from "lucide-react";
import { useTranslations } from "next-intl";

import { LoadingReveal } from "@/shared/components/loading-reveal";
import { PreviewStageSkeleton } from "@/shared/components/file-preview/preview-skeleton";

type PreviewDocumentProps = {
  source: string;
  contentType?: string;
  title?: string;
  description?: string;
};

function DocumentPreviewFallback({
  title,
  description,
}: {
  title?: string;
  description?: string;
}) {
  const t = useTranslations("files.previewErrors");
  const resolvedTitle = title ?? t("documentUnsupportedTitle");
  const resolvedDescription = description ?? t("documentUnsupportedDescription");

  return (
    <div className="flex h-full min-h-[360px] w-full items-center justify-center px-8 py-10">
      <div className="flex max-w-[420px] flex-col items-center text-center">
        <div className="flex size-20 items-center justify-center rounded-[24px] bg-background">
          <FileText className="size-8 text-muted-foreground" />
        </div>
        <p className="mt-5 text-base font-medium text-foreground">{resolvedTitle}</p>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">{resolvedDescription}</p>
      </div>
    </div>
  );
}

export function PreviewDocument({
  source,
  contentType,
  title,
  description,
}: PreviewDocumentProps) {
  const [status, setStatus] = React.useState<"checking" | "ready" | "fallback">("checking");

  React.useEffect(() => {
    setStatus("checking");

    const timer = window.setTimeout(() => {
      setStatus((current) => (current === "ready" ? current : "fallback"));
    }, 1200);

    return () => {
      window.clearTimeout(timer);
    };
  }, [source, contentType]);

  return (
    <div className="relative flex h-full min-h-0 flex-col bg-background">
      <div className="relative min-h-0 flex-1">
        {status === "ready" ? (
          <div className="min-h-0 h-full flex-1">
            <div className="h-full overflow-auto">
              <div className="px-1 pb-2">
                <object
                  key={`${source}:${contentType ?? ""}`}
                  data={source}
                  type={contentType}
                  className="block h-[min(70vh,720px)] w-full border-0 bg-transparent outline-none shadow-none"
                  onLoad={() => setStatus("ready")}
                />
              </div>
            </div>
          </div>
        ) : status === "fallback" ? (
          <DocumentPreviewFallback title={title} description={description} />
        ) : null}

        <LoadingReveal
          loading={status === "checking"}
          className="pointer-events-none absolute inset-0 z-10"
          contentClassName="h-full"
          skeletonClassName="h-full"
          skeleton={<PreviewStageSkeleton className="h-full" />}
        >
          <div className="h-full" />
        </LoadingReveal>
      </div>

      {status !== "ready" ? (
        <object
          key={`preload:${source}:${contentType ?? ""}`}
          data={source}
          type={contentType}
          className="pointer-events-none absolute inset-0 h-0 w-0 opacity-0"
          onLoad={() => setStatus("ready")}
        />
      ) : null}
    </div>
  );
}
