"use client";

import * as React from "react";
import { CircleAlert } from "lucide-react";
import { useTranslations } from "next-intl";

import { MessageAttachmentRow } from "@/features/chat/components/message/message-attachment";
import { UserMessageMeta } from "@/features/chat/components/message/message-meta";
import type { ChatAreaMessage } from "@/features/chat/types/messages";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import type { FileContentResult } from "@/shared/api/file";
import type { PreviewDialogFile } from "@/features/files/components/preview/file-preview-dialog";

type ChatMessageUserProps = {
  item: ChatAreaMessage;
  busy: boolean;
  onRetryUserMessage: (message: ChatAreaMessage) => Promise<void> | void;
  onEditUserMessage: (message: ChatAreaMessage, content: string) => Promise<boolean> | boolean;
  onCycleMessageBranch: (parentPublicID: string | null, direction: "previous" | "next") => void;
  onCopy: () => void;
  copySucceeded?: boolean;
  readOnly?: boolean;
  attachmentContentLoader?: (file: PreviewDialogFile) => Promise<FileContentResult>;
  showBranchNavigator?: boolean;
};

export function ChatMessageUser({
  item,
  busy,
  onRetryUserMessage,
  onEditUserMessage,
  onCycleMessageBranch,
  onCopy,
  copySucceeded = false,
  readOnly = false,
  attachmentContentLoader,
  showBranchNavigator = true,
}: ChatMessageUserProps) {
  const tCommon = useTranslations("common.actions");
  const tMessages = useTranslations("chat.messages");
  const [isEditing, setIsEditing] = React.useState(false);
  const [editingValue, setEditingValue] = React.useState(item.content);

  React.useEffect(() => {
    setIsEditing(false);
  }, [item.publicID]);

  React.useEffect(() => {
    if (!isEditing) {
      setEditingValue(item.content);
    }
  }, [isEditing, item.content]);

  const onRetry = React.useCallback(() => {
    void onRetryUserMessage(item);
  }, [item, onRetryUserMessage]);

  const onEditSave = React.useCallback(async () => {
    const nextContent = editingValue.trim();
    if (!nextContent || nextContent === item.content.trim()) {
      return;
    }
    const ok = await onEditUserMessage(item, nextContent);
    if (ok !== false) {
      setIsEditing(false);
    }
  }, [editingValue, item, onEditUserMessage]);

  if (!readOnly && isEditing) {
    const nextContent = editingValue.trim();
    const unchanged = nextContent === item.content.trim();

    return (
      <div className="flex justify-end">
        <div className="w-full max-w-[640px] rounded-lg bg-muted/60 p-3 text-foreground">
          <Textarea
            autoFocus
            value={editingValue}
            className="chat-font-content min-h-[120px] resize-none rounded-lg border-border border-[0.5px] bg-background px-3 py-2 text-sm leading-7 shadow-none focus-visible:border-primary focus-visible:ring-0"
            style={{ fontFamily: "var(--font-chat)", fontWeight: "var(--font-chat-weight)" }}
            onChange={(event) => setEditingValue(event.target.value)}
          />
          <div className="flex items-center justify-between gap-4">
            <div className="flex gap-2 pt-2 text-xs text-muted-foreground">
              <CircleAlert className="mt-0.5 size-3 shrink-0" />
              <span>{tMessages("editCreatesBranch")}</span>
            </div>
            <div className="mt-3 flex items-center justify-center gap-2">
              <Button
                variant="ghost"
                className="rounded-lg text-xs font-medium"
                onClick={() => setIsEditing(false)}
              >
                {tCommon("cancel")}
              </Button>
              <Button
                variant="default"
                className="rounded-lg text-xs font-medium shadow-none hover:bg-primary/60"
                disabled={busy || nextContent.length === 0 || unchanged}
                onClick={() => void onEditSave()}
              >
                {tCommon("save")}
              </Button>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="group/user-message flex min-w-0 max-w-full flex-col items-end gap-2">
      {item.attachments && item.attachments.length > 0 ? (
        <MessageAttachmentRow
          attachments={item.attachments}
          loadContent={attachmentContentLoader}
          allowDownload={!readOnly}
        />
      ) : null}
      <div
        className="chat-font-content min-w-0 max-w-[70%] overflow-hidden rounded-xl bg-muted/60 p-3 text-[15px] leading-8 text-foreground [overflow-wrap:anywhere] max-sm:max-w-[88%]"
        style={{ fontFamily: "var(--font-chat)", fontWeight: "var(--font-chat-weight)" }}
      >
        {item.content.trim() ? (
          <p className="whitespace-pre-wrap break-words [overflow-wrap:anywhere]">{item.content}</p>
        ) : null}
      </div>
      <UserMessageMeta
        item={item}
        busy={busy}
        showRetry={!busy && !item.isPending}
        onCycleBranch={onCycleMessageBranch}
        onRetry={onRetry}
        onEdit={() => setIsEditing(true)}
        onCopy={onCopy}
        copySucceeded={copySucceeded}
        readOnly={readOnly}
        alwaysVisible={readOnly}
        showBranchNavigator={showBranchNavigator}
      />
    </div>
  );
}
