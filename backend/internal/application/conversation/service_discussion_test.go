package conversation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domaindiscussion "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/discussion"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
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
	if !strings.Contains(text, "第 1 轮") || !strings.Contains(text, "第 2 轮") {
		t.Fatalf("expected round headings, got %s", text)
	}
	// The speaker is named by its model, never by the internal placeholder key.
	if !strings.Contains(text, "- model-a: A1") || strings.Contains(text, "p1") {
		t.Fatalf("expected model-named speakers, got %s", text)
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

func TestDiscussionEventPayloadsUseStateShape(t *testing.T) {
	state := domaindiscussion.State{
		Enabled:      true,
		Status:       "completed",
		TemplateID:   "tpl",
		TemplateName: "demo",
		Placeholders: []domaindiscussion.Placeholder{{Key: "p1", Label: "模型 1"}},
		Rounds: []domaindiscussion.RuntimeRound{{
			Index: 1,
			Steps: []domaindiscussion.RuntimeStep{{ID: "s1", Participant: "p1", Model: "model-a", Prompt: "answer"}},
		}},
		Summary:       domaindiscussion.RuntimeSummary{Participant: "judge", Model: "model-s"},
		SummaryPrompt: "system:\nsummarize",
		Turns: []domaindiscussion.Turn{{
			Round: 1, StepID: "s1", Participant: "p1", Model: "model-a",
			Status: "completed", Content: "A1", Prompt: "system:\nanswer",
		}},
	}
	encoded, err := json.Marshal(discussionStatePayloadOf(state))
	if err != nil {
		t.Fatalf("marshal state payload: %v", err)
	}
	text := string(encoded)
	// The web client reads camelCase keys; Go field names would silently render nothing.
	for _, key := range []string{`"stepId"`, `"summaryPrompt"`, `"templateId"`, `"placeholders"`, `"rounds"`} {
		if !strings.Contains(text, key) {
			t.Fatalf("expected %s in payload, got %s", key, text)
		}
	}
	for _, key := range []string{`"StepID"`, `"SummaryPrompt"`, `"TemplateID"`} {
		if strings.Contains(text, key) {
			t.Fatalf("domain field name %s leaked into payload: %s", key, text)
		}
	}
	turnEncoded, err := json.Marshal(discussionTurnPayloadOf(state.Turns[0]))
	if err != nil {
		t.Fatalf("marshal turn payload: %v", err)
	}
	if !strings.Contains(string(turnEncoded), `"prompt":"system:\nanswer"`) {
		t.Fatalf("expected prompt in turn payload, got %s", turnEncoded)
	}
}

func TestDiscussionTurnUserContentKeepsQuestionAndTranscript(t *testing.T) {
	text := discussionTurnUserContent("现在怎么办？", "第 1 轮\n- model-a: A1")
	for _, want := range []string{"现在怎么办？", "上一轮及更早的讨论记录", "A1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in user content, got %s", want, text)
		}
	}
	// History now travels as real user/assistant messages, not as a flattened text block.
	if strings.Contains(text, "上次讨论的结果") {
		t.Fatalf("prior result must not be inlined into the current question, got %s", text)
	}
	if bare := discussionTurnUserContent("你好", ""); strings.Contains(bare, "讨论记录") {
		t.Fatalf("an empty transcript must not add a heading, got %s", bare)
	}
}

type discussionStreamGatewayStub struct {
	temporaryLLMGatewayStub
}

func (g *discussionStreamGatewayStub) GenerateStream(
	ctx context.Context,
	route llm.RouteConfig,
	input llm.GenerateInput,
	onEvent func(llm.GenerateStreamEvent) error,
) (*llm.GenerateOutput, error) {
	if onEvent != nil {
		if err := onEvent(llm.GenerateStreamEvent{Delta: "partial"}); err != nil {
			return nil, err
		}
	}
	return g.temporaryLLMGatewayStub.GenerateStream(ctx, route, input, nil)
}

func discussionStreamTestService() *Service {
	return &Service{
		cfg: config.NewRuntime(config.Config{}),
		routeResolver: &textTaskRouteResolverStub{routes: map[string]*channel.ResolvedRoute{
			"model-a": {PlatformModelName: "model-a", UpstreamModel: "model-a", Protocol: llm.AdapterOpenAIChatCompletions},
		}},
		llmClient: &discussionStreamGatewayStub{},
	}
}

func TestDiscussionTurnEmitsStartBeforeDelta(t *testing.T) {
	events := make([]string, 0, 2)
	turn := discussionStreamTestService().runDiscussionTurn(
		context.Background(),
		DiscussionSendInput{Content: "question", OnEvent: func(eventType string, _ map[string]any) error {
			events = append(events, eventType)
			return nil
		}},
		&model.Conversation{ID: 1},
		1,
		domaindiscussion.RuntimeStep{ID: "s1", Participant: "p1", Model: "model-a"},
		"",
		nil,
	)
	if strings.Join(events, ",") != "discussion_turn_start,discussion_turn_delta" {
		t.Fatalf("events = %v", events)
	}
	if turn.Status != "completed" || turn.Content != "ok" {
		t.Fatalf("turn = %#v", turn)
	}
}

func TestDiscussionSummaryEmitsStartBeforeDelta(t *testing.T) {
	events := make([]string, 0, 2)
	turn := discussionStreamTestService().runDiscussionSummary(
		context.Background(),
		DiscussionSendInput{Content: "question"},
		&model.Conversation{ID: 1},
		domaindiscussion.RuntimePlan{Summary: domaindiscussion.RuntimeSummary{Participant: "judge", Model: "model-a"}},
		nil,
		nil,
		func(eventType string, _ map[string]any) {
			events = append(events, eventType)
		},
	)
	if strings.Join(events, ",") != "discussion_summary_start,discussion_summary_delta" {
		t.Fatalf("events = %v", events)
	}
	if turn.Status != "completed" || turn.Content != "ok" {
		t.Fatalf("turn = %#v", turn)
	}
}
