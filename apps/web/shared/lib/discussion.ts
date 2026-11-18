import type { DiscussionPlaceholder, DiscussionRound } from "@/shared/api/discussion-types";

const SUMMARY_PLACEHOLDER_LABEL = "总结模型";

const PARALLEL_ROUND_PROMPTS = [
  "请独立回答用户问题，给出清晰、具体、可执行的答案。",
  "阅读上一轮讨论，补充遗漏、纠正错误、挑战薄弱推理。",
] as const;

const SERIAL_ROUND_PROMPTS = [
  "请先独立回答用户问题，再针对本轮排在你之前的模型发言回应具体分歧：指出其依赖的假设、逻辑跳跃或缺失证据，给出反例或替代解释，不要重复已经充分的内容。",
  "请阅读此前轮次及本轮排在你之前的模型发言，针对具体分歧回应：补充遗漏、纠正错误、指出假设与证据问题，不要重复已经充分的内容。",
] as const;

/**
 * Default task prompt for a round. The first round asks for an independent answer; later rounds
 * ask the models to build on what the discussion already established. Parallel and serial rounds
 * get separate defaults because the same-round context differs.
 */
export function defaultDiscussionRoundPrompt(roundIndex: number, parallel: boolean): string {
  const prompts = parallel ? PARALLEL_ROUND_PROMPTS : SERIAL_ROUND_PROMPTS;
  return prompts[roundIndex <= 0 ? 0 : 1];
}

/**
 * Prompt of the round's current execution mode. Older templates only store steps[].prompt, so it
 * remains the fallback; a stored mode prompt (even an empty one) wins over that fallback.
 */
export function discussionRoundPrompt(round: DiscussionRound): string {
  const stored = round.parallel ? round.parallelPrompt : round.serialPrompt;
  return stored ?? round.steps[0]?.prompt ?? "";
}

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
