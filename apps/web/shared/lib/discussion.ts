import type { DiscussionPlaceholder } from "@/shared/api/discussion-types";

const SUMMARY_PLACEHOLDER_LABEL = "总结模型";

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

/**
 * Label of every placeholder, keyed by placeholder key. Discussion models are numbered from 1 in
 * their presentation order; the summary keeps its own name. The editor no longer exposes labels,
 * so the position is the single source of truth and stored labels are only carried for the API.
 */
export function discussionPlaceholderLabels(
  placeholders: readonly DiscussionPlaceholder[],
  summaryKey: string,
): Map<string, string> {
  const labels = new Map<string, string>();
  let index = 0;
  for (const placeholder of orderDiscussionPlaceholders(placeholders, summaryKey)) {
    if (placeholder.key === summaryKey) {
      labels.set(placeholder.key, SUMMARY_PLACEHOLDER_LABEL);
      continue;
    }
    index += 1;
    labels.set(placeholder.key, `模型 ${index}`);
  }
  return labels;
}
