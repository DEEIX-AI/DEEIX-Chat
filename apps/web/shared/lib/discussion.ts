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

// Labels created before numbering used A/B/C ("模型 A"). They read as 模型 1, 模型 2, … so the
// discussion is ordered by position instead of by letter.
const LEGACY_LETTER_LABEL = /^模型\s*([A-Z])$/u;

/** Display label of a placeholder: legacy letter defaults become the 1-based position. */
export function normalizeDiscussionLabel(label: string, index: number): string {
  const trimmed = label.trim();
  if (LEGACY_LETTER_LABEL.test(trimmed)) {
    return `模型 ${index}`;
  }
  return trimmed;
}

/**
 * Label of every placeholder, keyed by placeholder key. The summary keeps its own label; the
 * discussion models are numbered from 1 in their presentation order.
 */
export function discussionPlaceholderLabels(
  placeholders: readonly DiscussionPlaceholder[],
  summaryKey: string,
): Map<string, string> {
  const labels = new Map<string, string>();
  let index = 0;
  for (const placeholder of orderDiscussionPlaceholders(placeholders, summaryKey)) {
    if (placeholder.key === summaryKey) {
      labels.set(placeholder.key, placeholder.label.trim() || placeholder.key);
      continue;
    }
    index += 1;
    labels.set(placeholder.key, normalizeDiscussionLabel(placeholder.label, index) || placeholder.key);
  }
  return labels;
}

/** Human name of a placeholder; the key is an internal id and never shown when a label exists. */
export function discussionPlaceholderLabel(
  placeholders: readonly DiscussionPlaceholder[],
  key: string,
): string {
  const placeholder = placeholders.find((item) => item.key === key);
  return placeholder?.label.trim() || key;
}
