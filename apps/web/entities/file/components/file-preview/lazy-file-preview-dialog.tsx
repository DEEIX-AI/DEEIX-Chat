"use client";

import dynamic from "next/dynamic";

// FilePreviewDialog loaded on first render, so the dialog and its viewers stay out of the
// route chunk. Consumers use this instead of `dynamic(() => import("@/entities/file"))`: a
// dynamic import of the entity entry would put the whole entity into the lazy chunk.
export const LazyFilePreviewDialog = dynamic(
  () => import("@/entities/file/components/file-preview/preview-dialog").then((mod) => mod.FilePreviewDialog),
  { ssr: false },
);
