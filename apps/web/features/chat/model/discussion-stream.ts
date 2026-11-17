import type { DiscussionStreamEvent } from "@/shared/api/conversation-types";
import type { DiscussionState, DiscussionTurn } from "@/shared/api/discussion-types";

function upsertTurn(turns: readonly DiscussionTurn[], next: DiscussionTurn): DiscussionTurn[] {
  const index = turns.findIndex((turn) => turn.round === next.round && turn.stepId === next.stepId);
  if (index < 0) {
    return [...turns, next];
  }
  const merged = [...turns];
  merged[index] = { ...merged[index], ...next };
  return merged;
}

/**
 * Folds one discussion stream event into the live state shown in the transcript.
 * Rounds stream model by model, so a turn is created on its first delta and replaced when it ends.
 */
export function applyDiscussionEvent(
  state: DiscussionState | undefined,
  event: DiscussionStreamEvent,
): DiscussionState | undefined {
  const now = Math.floor(Date.now() / 1000);
  switch (event.type) {
    case "discussion_start":
      return {
        enabled: true,
        status: "running",
        templateId: event.template,
        templateName: event.templateName,
        placeholders: event.placeholders ?? [],
        rounds: event.rounds,
        summary: event.summary,
        // A resumed stream replays from the start, so keep what the first pass already collected.
        turns: state?.templateId === event.template ? state.turns : [],
        createdAt: state?.createdAt ?? now,
        updatedAt: now,
      };
    case "discussion_turn_delta": {
      if (!state) {
        return state;
      }
      const existing = state.turns.find(
        (turn) => turn.round === event.round && turn.stepId === event.stepId,
      );
      return {
        ...state,
        turns: upsertTurn(state.turns, {
          round: event.round,
          stepId: event.stepId,
          participant: event.participant,
          model: event.model,
          status: "running",
          content: `${existing?.content ?? ""}${event.delta}`,
        }),
        updatedAt: now,
      };
    }
    case "discussion_turn_done": {
      if (!state) {
        return state;
      }
      return { ...state, turns: upsertTurn(state.turns, event.turn), updatedAt: now };
    }
    case "discussion_round_done": {
      if (!state) {
        return state;
      }
      const others = state.turns.filter((turn) => turn.round !== event.round);
      return { ...state, turns: [...others, ...event.turns], updatedAt: now };
    }
    case "discussion_summary_delta":
      return state ? { ...state, status: "summarizing", updatedAt: now } : state;
    case "discussion_done":
      return event.state;
    default:
      return state;
  }
}
