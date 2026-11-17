package conversation

import (
	"strings"
	"testing"

	domaindiscussion "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/discussion"
)

func TestDiscussionTranscriptSkipsFailedTurns(t *testing.T) {
	turns := []domaindiscussion.Turn{
		{Round: 1, Participant: "p1", Model: "model-a", Status: "completed", Content: "A1"},
		{Round: 1, Participant: "p2", Model: "model-b", Status: "error", Error: "boom"},
		{Round: 2, Participant: "p1", Model: "model-a", Status: "completed", Content: "A2"},
	}
	text := discussionTranscript(turns)
	if strings.Contains(text, "model-b") {
		t.Fatalf("failed turn leaked into transcript: %s", text)
	}
	if !strings.Contains(text, "Round 1") || !strings.Contains(text, "Round 2") {
		t.Fatalf("expected round headings, got %s", text)
	}
	if !strings.Contains(text, "A1") || !strings.Contains(text, "A2") {
		t.Fatalf("expected successful content, got %s", text)
	}
}

func TestDiscussionFallbackSummaryWithoutSuccess(t *testing.T) {
	text := discussionFallbackSummary([]domaindiscussion.Turn{
		{Round: 1, Participant: "p1", Model: "model-a", Status: "error", Error: "boom"},
	}, "summary failed")
	if !strings.Contains(text, "没有成功的讨论发言") {
		t.Fatalf("expected empty discussion note, got %s", text)
	}
	if !strings.Contains(text, "summary failed") {
		t.Fatalf("expected failure note, got %s", text)
	}
}