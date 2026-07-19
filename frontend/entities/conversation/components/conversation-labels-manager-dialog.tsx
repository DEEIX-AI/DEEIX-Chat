"use client";

import { ConversationLabelsDialog } from "@/entities/conversation/components/conversation-labels-dialog";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { parseConversationLabelsJSON } from "@/shared/lib/conversation-labels";
import type { ConversationDTO } from "@/shared/api/conversation.types";

export type ConversationLabelsTarget = Pick<ConversationDTO, "publicID" | "labelsJSON">;

type ConversationLabelsManagerDialogProps = {
  target: ConversationLabelsTarget | null;
  onTargetChange: (target: ConversationLabelsTarget | null) => void;
  onUpdateLabels: (publicID: string, labels: string[]) => Promise<ConversationDTO | null>;
};

export function ConversationLabelsManagerDialog({
  target,
  onTargetChange,
  onUpdateLabels,
}: ConversationLabelsManagerDialogProps) {
  const stableTarget = useDialogSnapshot(target);

  if (!stableTarget) {
    return null;
  }

  return (
    <ConversationLabelsDialog
      open={Boolean(target)}
      labels={parseConversationLabelsJSON(stableTarget.labelsJSON)}
      onOpenChange={(open) => {
        if (!open) {
          onTargetChange(null);
        }
      }}
      onSave={async (labels) => {
        const updated = await onUpdateLabels(stableTarget.publicID, labels);
        if (!updated) {
          throw new Error("conversation labels were not updated");
        }
      }}
    />
  );
}
