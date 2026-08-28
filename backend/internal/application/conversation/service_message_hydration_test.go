package conversation

import (
	"fmt"
	"testing"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

func TestNormalizeLegacyThinkReplayEventsCollapsesStreamAndRepeatedFinalSnapshots(t *testing.T) {
	startedAt := time.Now().UTC().Add(-time.Minute)
	firstEndedAt := startedAt.Add(10 * time.Second)
	finalEndedAt := startedAt.Add(12 * time.Second)
	finalPayload := persistedReasoningTestPayload("response.completed", "reasoning_1")
	rows := []model.MessageTraceEventRow{
		persistedThinkTestRow("upstream_think_1", "round_1", 2, "相同思考", persistedReasoningTestPayload("chat.completion.chunk", ""), startedAt, &firstEndedAt),
		{
			RunID:     "run_1",
			EventID:   "tools_1",
			EventType: "tool",
			Phase:     messageTraceTypeTools,
			Stage:     messageTraceStageTool,
			Status:    messageTraceStatusCompleted,
			Seq:       3,
			CreatedAt: startedAt.Add(11 * time.Second),
		},
		persistedThinkTestRow("upstream_think_2", "round_2", 4, "相同思考", finalPayload, startedAt.Add(12*time.Second), &finalEndedAt),
		persistedThinkTestRow("upstream_think_3", "round_3", 5, "相同思考", finalPayload, startedAt.Add(12*time.Second+20*time.Millisecond), &finalEndedAt),
	}
	originalFirstPayload := rows[0].PayloadJSON

	normalized := normalizeLegacyThinkReplayEvents(rows)

	if len(normalized) != 2 {
		t.Fatalf("normalized events = %#v, want canonical think plus tool", normalized)
	}
	think := normalized[0]
	if think.EventID != "upstream_think_1" || think.RoundID != "round_1" || think.Seq != 2 {
		t.Fatalf("canonical think identity changed: %#v", think)
	}
	if think.PayloadJSON != finalPayload || think.EndedAt == nil || !think.EndedAt.Equal(finalEndedAt) {
		t.Fatalf("canonical think did not receive final snapshot: %#v", think)
	}
	if normalized[1].EventID != "tools_1" {
		t.Fatalf("tool event order changed: %#v", normalized)
	}
	if rows[0].PayloadJSON != originalFirstPayload {
		t.Fatalf("normalization mutated input rows: got %q want %q", rows[0].PayloadJSON, originalFirstPayload)
	}
}

func TestNormalizeLegacyThinkReplayEventsCollapsesRepeatedCompletedSnapshots(t *testing.T) {
	createdAt := time.Now().UTC()
	endedAt := createdAt.Add(time.Second)
	payload := persistedReasoningTestPayload("response.completed", "")
	rows := []model.MessageTraceEventRow{
		persistedThinkTestRow("upstream_think_1", "round_1", 2, "重复完成内容", payload, createdAt, &endedAt),
		persistedThinkTestRow("upstream_think_2", "round_2", 3, "重复完成内容", payload, createdAt.Add(25*time.Millisecond), &endedAt),
		persistedThinkTestRow("upstream_think_3", "round_3", 4, "重复完成内容", payload, createdAt.Add(50*time.Millisecond), &endedAt),
	}

	normalized := normalizeLegacyThinkReplayEvents(rows)

	if len(normalized) != 1 || normalized[0].EventID != "upstream_think_1" {
		t.Fatalf("normalized events = %#v, want first completed event only", normalized)
	}
}

func TestNormalizeLegacyThinkReplayEventsCollapsesSingleFinalReconciliation(t *testing.T) {
	createdAt := time.Now().UTC()
	endedAt := createdAt.Add(time.Second)
	finalPayload := persistedReasoningTestPayload("response.completed", "")
	rows := []model.MessageTraceEventRow{
		persistedThinkTestRow("upstream_think_1", "round_1", 2, "相同文本", persistedReasoningTestPayload("chat.completion.chunk", ""), createdAt, &endedAt),
		persistedThinkTestRow("upstream_think_2", "round_2", 3, "相同文本", finalPayload, createdAt.Add(time.Second), &endedAt),
	}

	normalized := normalizeLegacyThinkReplayEvents(rows)

	if len(normalized) != 1 || normalized[0].EventID != "upstream_think_1" {
		t.Fatalf("normalized events = %#v, want canonical live event only", normalized)
	}
	if normalized[0].PayloadJSON != finalPayload {
		t.Fatalf("canonical event payload = %q, want final payload %q", normalized[0].PayloadJSON, finalPayload)
	}
}

func TestNormalizeLegacyThinkReplayEventsCollapsesReasoningDoneAndFinalSnapshot(t *testing.T) {
	createdAt := time.Now().UTC()
	endedAt := createdAt.Add(time.Second)
	finalPayload := persistedReasoningTestPayload("response.completed", "reasoning_1")
	rows := []model.MessageTraceEventRow{
		persistedThinkTestRow("upstream_think_1", "round_1", 2, "最终思考", persistedReasoningTestPayload("response.reasoning_text.done", "reasoning_1"), createdAt, &endedAt),
		persistedThinkTestRow("upstream_think_2", "round_2", 3, "最终思考", finalPayload, createdAt.Add(20*time.Millisecond), &endedAt),
	}

	normalized := normalizeLegacyThinkReplayEvents(rows)

	if len(normalized) != 1 || normalized[0].EventID != "upstream_think_1" {
		t.Fatalf("normalized events = %#v, want canonical reasoning done event only", normalized)
	}
	if normalized[0].PayloadJSON != finalPayload {
		t.Fatalf("canonical event payload = %q, want final payload %q", normalized[0].PayloadJSON, finalPayload)
	}
}

func TestNormalizeLegacyThinkReplayEventsCollapsesCompletedSnapshotsWithoutMetadata(t *testing.T) {
	createdAt := time.Now().UTC()
	endedAt := createdAt.Add(time.Second)
	rows := []model.MessageTraceEventRow{
		persistedThinkTestRow("upstream_think_1", "round_1", 2, "旧思考内容", "", createdAt, &endedAt),
		persistedThinkTestRow("upstream_think_2", "round_2", 3, "旧思考内容", "{}", createdAt.Add(25*time.Millisecond), &endedAt),
		persistedThinkTestRow("upstream_think_3", "round_3", 4, "旧思考内容", `{"reasoning":{"kind":"content"}}`, createdAt.Add(50*time.Millisecond), &endedAt),
	}

	normalized := normalizeLegacyThinkReplayEvents(rows)

	if len(normalized) != 1 || normalized[0].EventID != "upstream_think_1" {
		t.Fatalf("normalized events = %#v, want one canonical legacy event", normalized)
	}
}

func TestNormalizeLegacyThinkReplayEventsPreservesUnknownRoundsSeparatedByTool(t *testing.T) {
	createdAt := time.Now().UTC()
	endedAt := createdAt.Add(time.Second)
	rows := []model.MessageTraceEventRow{
		persistedThinkTestRow("upstream_think_1", "round_1", 2, "相同文本", "", createdAt, &endedAt),
		{
			RunID:     "run_1",
			EventID:   "tools_1",
			EventType: "tool",
			Phase:     messageTraceTypeTools,
			Stage:     messageTraceStageTool,
			Status:    messageTraceStatusCompleted,
			Seq:       3,
			CreatedAt: createdAt.Add(10 * time.Millisecond),
		},
		persistedThinkTestRow("upstream_think_2", "round_2", 4, "相同文本", "", createdAt.Add(25*time.Millisecond), &endedAt),
	}

	normalized := normalizeLegacyThinkReplayEvents(rows)

	if len(normalized) != len(rows) {
		t.Fatalf("normalized events = %#v, want tool-separated rounds preserved", normalized)
	}
}

func TestNormalizeLegacyThinkReplayEventsPreservesUnconfirmedRounds(t *testing.T) {
	createdAt := time.Now().UTC()
	endedAt := createdAt.Add(time.Second)
	tests := []struct {
		name string
		rows []model.MessageTraceEventRow
	}{
		{
			name: "different reasoning items",
			rows: []model.MessageTraceEventRow{
				persistedThinkTestRow("upstream_think_1", "round_1", 2, "相同文本", persistedReasoningTestPayload("response.completed", "reasoning_1"), createdAt, &endedAt),
				persistedThinkTestRow("upstream_think_2", "round_2", 3, "相同文本", persistedReasoningTestPayload("response.completed", "reasoning_2"), createdAt.Add(time.Millisecond), &endedAt),
			},
		},
		{
			name: "completed snapshots outside replay window",
			rows: []model.MessageTraceEventRow{
				persistedThinkTestRow("upstream_think_1", "round_1", 2, "相同文本", persistedReasoningTestPayload("response.completed", ""), createdAt, &endedAt),
				persistedThinkTestRow("upstream_think_2", "round_2", 3, "相同文本", persistedReasoningTestPayload("response.completed", ""), createdAt.Add(legacyReasoningReplayMaxGap+time.Millisecond), &endedAt),
			},
		},
		{
			name: "different final payloads",
			rows: []model.MessageTraceEventRow{
				persistedThinkTestRow("upstream_think_1", "round_1", 2, "相同文本", `{"reasoning":{"event_type":"response.completed","status":"first"}}`, createdAt, &endedAt),
				persistedThinkTestRow("upstream_think_2", "round_2", 3, "相同文本", `{"reasoning":{"event_type":"response.completed","status":"second"}}`, createdAt.Add(time.Millisecond), &endedAt),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			normalized := normalizeLegacyThinkReplayEvents(test.rows)
			if len(normalized) != len(test.rows) {
				t.Fatalf("normalized events = %#v, want all %d rounds preserved", normalized, len(test.rows))
			}
		})
	}
}

func persistedThinkTestRow(
	eventID string,
	roundID string,
	seq int,
	content string,
	payloadJSON string,
	createdAt time.Time,
	endedAt *time.Time,
) model.MessageTraceEventRow {
	return model.MessageTraceEventRow{
		MessageID:       1,
		ConversationID:  2,
		UserID:          3,
		RunID:           "run_1",
		EventID:         eventID,
		EventType:       "think",
		Phase:           messageTraceTypeUpstreamThink,
		Stage:           messageTraceStageThink,
		RoundID:         roundID,
		Status:          messageTraceStatusCompleted,
		Title:           "模型思考",
		Summary:         content,
		ContentMarkdown: content,
		PayloadJSON:     payloadJSON,
		Seq:             seq,
		StartedAt:       createdAt,
		EndedAt:         endedAt,
		CreatedAt:       createdAt,
		UpdatedAt:       createdAt,
	}
}

func persistedReasoningTestPayload(eventType string, itemID string) string {
	return fmt.Sprintf(`{"reasoning":{"event_type":%q,"item_id":%q,"status":"completed"}}`, eventType, itemID)
}
