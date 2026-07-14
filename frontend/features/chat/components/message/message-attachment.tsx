"use client";

import * as React from "react";
import dynamic from "next/dynamic";

import {
  Attachment,
  AttachmentContent,
  AttachmentDescription,
  AttachmentMedia,
  AttachmentTitle,
  AttachmentTrigger,
} from "@/components/ui/attachment";
import type { MessageAttachment } from "@/features/chat/types/messages";
import type { PreviewDialogFile } from "@/shared/components/file-preview/preview-dialog";
import { formatBytes, resolveFileExtension, resolveFileIcon } from "@/shared/lib/file-display";
import type { FileContentResult } from "@/shared/api/file";

const FilePreviewDialog = dynamic(
  () => import("@/shared/components/file-preview/preview-dialog").then((module) => module.FilePreviewDialog),
  { ssr: false },
);

// ─── helpers ──────────────────────────────────────────────────────────────────

function resolveFileExt(name: string): string {
  const ext = resolveFileExtension(name);
  return ext ? ext.toUpperCase().slice(0, 6) : "FILE";
}

function resolveCardMeta(att: MessageAttachment): string {
  return `${resolveFileExt(att.fileName)} · ${formatBytes(att.sizeBytes)}`;
}

// ─── single card ─────────────────────────────────────────────────────────────

function AttachmentCard({
  att,
  onClick,
}: {
  att: MessageAttachment;
  onClick: () => void;
}) {
  const meta = resolveCardMeta(att);
  const fileIcon = resolveFileIcon(att);

  return (
    <Attachment
      size="sm"
      className="h-12 w-56 border-0 bg-muted/35 text-left hover:bg-muted/50 dark:bg-white/[0.06] dark:hover:bg-white/[0.09]"
    >
      <AttachmentMedia className="size-6 bg-transparent text-muted-foreground">
        {React.createElement(fileIcon, { className: "size-5", strokeWidth: 1.6 })}
      </AttachmentMedia>
      <AttachmentContent className="flex min-w-0 flex-1 flex-col justify-center px-0 py-0">
        <AttachmentTitle className="text-[12px] leading-4 text-foreground/90" title={att.fileName}>
          {att.fileName}
        </AttachmentTitle>
        <AttachmentDescription className="mt-1 text-[11px] leading-none">
          {meta}
        </AttachmentDescription>
      </AttachmentContent>
      <AttachmentTrigger
        onClick={onClick}
        aria-label={att.fileName}
        className="rounded-lg focus-visible:ring-2 focus-visible:ring-ring"
      />
    </Attachment>
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
