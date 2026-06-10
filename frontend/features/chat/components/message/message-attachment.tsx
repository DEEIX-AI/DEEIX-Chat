"use client";

import * as React from "react";
import dynamic from "next/dynamic";

import type { MessageAttachment } from "@/features/chat/types/messages";
import type { PreviewDialogFile } from "@/features/files/components/preview/file-preview-dialog";
import { formatBytes, resolveFileExtension, resolveFileIcon } from "@/features/files/utils/file-display";
import type { FileContentResult } from "@/shared/api/file";

const FilePreviewDialog = dynamic(
  () => import("@/features/files/components/preview/file-preview-dialog").then((module) => module.FilePreviewDialog),
  { ssr: false },
);

// ─── helpers ──────────────────────────────────────────────────────────────────

function resolveFileExt(name: string): string {
  const ext = resolveFileExtension(name);
  return ext ? ext.toUpperCase().slice(0, 6) : "FILE";
}

function resolveCardMeta(att: MessageAttachment): string {
  return formatBytes(att.sizeBytes);
}

// ─── single card ─────────────────────────────────────────────────────────────

function AttachmentCard({
  att,
  onClick,
}: {
  att: MessageAttachment;
  onClick: () => void;
}) {
  const ext = resolveFileExt(att.fileName);
  const meta = resolveCardMeta(att);
  const fileIcon = resolveFileIcon(att);

  return (
    <div
      className="group relative h-14 w-56 shrink-0 rounded-lg bg-muted/35 text-left transition-colors hover:bg-muted/50 dark:bg-white/[0.06] dark:hover:bg-white/[0.09]"
    >
      <button
        type="button"
        onClick={onClick}
        className="flex h-full w-full items-center gap-2.5 rounded-lg px-2.5 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <div className="flex size-6 shrink-0 items-center justify-center">
          {React.createElement(fileIcon, { className: "size-5 text-muted-foreground", strokeWidth: 1.6 })}
        </div>
        <div className="flex min-w-0 flex-1 flex-col justify-center">
          <p className="truncate text-[12px] font-medium leading-4 text-foreground/90" title={att.fileName}>
            {att.fileName}
          </p>
          <div className="mt-1 flex min-w-0 items-center gap-1.5">
            <span className="min-w-0 shrink truncate text-[10px] leading-none text-muted-foreground">
              {meta}
            </span>
            <span className="shrink-0 rounded-md bg-muted/60 px-1.5 py-0.5 text-[10px] font-medium leading-none text-muted-foreground/65">
              {ext}
            </span>
          </div>
        </div>
      </button>
    </div>
  );
}

// ─── public export ────────────────────────────────────────────────────────────

export function MessageAttachmentRow({
  attachments,
  loadContent,
  allowDownload = true,
  align = "end",
}: {
  attachments: MessageAttachment[];
  loadContent?: (file: PreviewDialogFile) => Promise<FileContentResult>;
  allowDownload?: boolean;
  align?: "start" | "end";
}) {
  const [activeAtt, setActiveAtt] = React.useState<MessageAttachment | null>(null);
  const [dialogOpen, setDialogOpen] = React.useState(false);

  const handleClick = React.useCallback((att: MessageAttachment) => {
    setActiveAtt(att);
    setDialogOpen(true);
  }, []);

  const handleOpenChange = React.useCallback((v: boolean) => {
    setDialogOpen(v);
    if (!v) setActiveAtt(null);
  }, []);

  return (
    <>
      <div className={`flex max-w-full flex-wrap gap-2 sm:max-w-[70%] ${align === "start" ? "justify-start" : "justify-end"}`}>
        {attachments.map((att) => (
          <AttachmentCard key={att.fileID} att={att} onClick={() => handleClick(att)} />
        ))}
      </div>
      {activeAtt ? (
        <FilePreviewDialog
          file={activeAtt}
          open={dialogOpen}
          onOpenChange={handleOpenChange}
          loadContent={loadContent}
          allowDownload={allowDownload}
        />
      ) : null}
    </>
  );
}
