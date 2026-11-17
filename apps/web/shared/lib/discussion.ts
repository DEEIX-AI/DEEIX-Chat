import type { DiscussionPlaceholder } from "@/shared/api/discussion-types";

/**
 * Placeholders in presentation order: the summary model first, then the discussion models.
 * The stored array keeps whatever order the editor produced, so both the admin editor and the
 * chat-time binding bar read their order from here instead of trusting the array.
 */
export function orderDiscussionPlaceholders(
  placeholders: readonly DiscussionPlaceholder[],
  summaryKey: string,
): DiscussionPlaceholder[] {
  const ordered = [...placeholders];
  const summaryIndex = ordered.findIndex((item) => item.key === summaryKey);
  if (summaryIndex <= 0) {
    return ordered;
  }
  const [summary] = ordered.splice(summaryIndex, 1);
  ordered.unshift(summary);
  return ordered;
}

/** Human name of a placeholder; the key is an internal id and never shown when a label exists. */
export function discussionPlaceholderLabel(
  placeholders: readonly DiscussionPlaceholder[],
  key: string,
): string {
  const placeholder = placeholders.find((item) => item.key === key);
  return placeholder?.label.trim() || key;
}

/** Name of a discussion speaker: the placeholder label, then the bound model, then the key. */
export function discussionSpeakerLabel(
  placeholders: readonly DiscussionPlaceholder[],
  participant: string,
  model?: string,
): string {
  const label = placeholders.find((item) => item.key === participant)?.label.trim();
  if (label) {
    return label;
  }
  return model?.trim() || participant;
}
