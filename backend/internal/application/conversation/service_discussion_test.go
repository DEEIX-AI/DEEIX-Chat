package conversation

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
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
		Summary:               domaindiscussion.RuntimeSummary{Participant: "judge", Model: "model-s"},
		SummaryPrompt:         "system:\nsummarize",
		SummaryPromptMessages: []domaindiscussion.PromptMessage{{Role: "system", Content: "summarize"}},
		Turns: []domaindiscussion.Turn{{
			Round: 1, StepID: "s1", Participant: "p1", Model: "model-a",
			Status: "completed", Content: "A1", Prompt: "system:\nanswer",
			PromptMessages: []domaindiscussion.PromptMessage{{Role: "system", Content: "answer"}},
		}},
	}
	encoded, err := json.Marshal(discussionStatePayloadOf(state))
	if err != nil {
		t.Fatalf("marshal state payload: %v", err)
	}
	text := string(encoded)
	// The web client reads camelCase keys; Go field names would silently render nothing.
	for _, key := range []string{`"stepId"`, `"summaryPrompt"`, `"summaryPromptMessages"`, `"promptMessages"`, `"templateId"`, `"placeholders"`, `"rounds"`} {
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
	text := discussionTurnUserContent("现在怎么办？", "第 1 轮\n- model-a: A1", true)
	for _, want := range []string{"现在怎么办？", "此前轮次的讨论记录（不包含本轮其他模型）", "A1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in user content, got %s", want, text)
		}
	}
	serial := discussionTurnUserContent("现在怎么办？", "第 1 轮\n- model-a: A1", false)
	if !strings.Contains(serial, "此前轮次及本轮在你之前的讨论记录") {
		t.Fatalf("expected serial context heading, got %s", serial)
	}
	// History now travels as real user/assistant messages, not as a flattened text block.
	if strings.Contains(text, "上次讨论的结果") {
		t.Fatalf("prior result must not be inlined into the current question, got %s", text)
	}
	if bare := discussionTurnUserContent("你好", "", true); strings.Contains(bare, "讨论记录") {
		t.Fatalf("an empty transcript must not add a heading, got %s", bare)
	}
}

type discussionStreamGatewayStub struct {
	temporaryLLMGatewayStub
	mu        sync.Mutex
	responses []string
}

func (g *discussionStreamGatewayStub) GenerateStream(
	ctx context.Context,
	route llm.RouteConfig,
	input llm.GenerateInput,
	onEvent func(llm.GenerateStreamEvent) error,
) (*llm.GenerateOutput, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.responses) > 0 {
		g.output = &llm.GenerateOutput{Text: g.responses[0]}
		g.responses = g.responses[1:]
	}
	if onEvent != nil {
		if err := onEvent(llm.GenerateStreamEvent{Delta: "partial"}); err != nil {
			return nil, err
		}
	}
	return g.temporaryLLMGatewayStub.GenerateStream(ctx, route, input, nil)
}

func discussionStreamTestServiceForModels(gateway *discussionStreamGatewayStub, models ...string) *Service {
	routes := make(map[string]*channel.ResolvedRoute, len(models))
	for _, modelName := range models {
		routes[modelName] = &channel.ResolvedRoute{PlatformModelName: modelName, UpstreamModel: modelName, Protocol: llm.AdapterOpenAIChatCompletions}
	}
	return &Service{
		cfg:           config.NewRuntime(config.Config{}),
		routeResolver: &textTaskRouteResolverStub{routes: routes},
		llmClient:     gateway,
	}
}

func discussionStreamTestService() *Service {
	return discussionStreamTestServiceForModels(&discussionStreamGatewayStub{}, "model-a")
}

func lastDiscussionUserContent(input llm.GenerateInput) string {
	return input.Messages[len(input.Messages)-1].Content
}

func TestDiscussionSerialRoundBuildsIncrementalContext(t *testing.T) {
	gateway := &discussionStreamGatewayStub{responses: []string{"first answer", "second answer", "third answer"}}
	service := discussionStreamTestServiceForModels(gateway, "model-a", "model-b", "model-c")
	serial := false
	state := domaindiscussion.State{Turns: []domaindiscussion.Turn{{Round: 1, Status: "completed", Model: "previous", Content: "previous answer"}}}
	round := domaindiscussion.RuntimeRound{Index: 2, Parallel: &serial, Steps: []domaindiscussion.RuntimeStep{
		{ID: "s1", Participant: "p1", Model: "model-a"},
		{ID: "s2", Participant: "p2", Model: "model-b"},
		{ID: "s3", Participant: "p3", Model: "model-c"},
	}}
	results := service.runDiscussionRound(
		context.Background(),
		DiscussionSendInput{Content: "question"},
		&model.Conversation{ID: 1},
		round,
		&state,
		nil,
		func(string, map[string]any) {},
	)
	if len(results) != 3 || results[0].Model != "model-a" || results[1].Model != "model-b" || results[2].Model != "model-c" {
		t.Fatalf("unexpected result order: %#v", results)
	}
	if len(gateway.inputs) != 3 {
		t.Fatalf("upstream calls = %d", len(gateway.inputs))
	}
	first := lastDiscussionUserContent(gateway.inputs[0])
	second := lastDiscussionUserContent(gateway.inputs[1])
	third := lastDiscussionUserContent(gateway.inputs[2])
	if !strings.Contains(first, "previous answer") || strings.Contains(first, "first answer") {
		t.Fatalf("unexpected first context: %s", first)
	}
	if !strings.Contains(second, "first answer") || strings.Contains(second, "second answer") {
		t.Fatalf("unexpected second context: %s", second)
	}
	if !strings.Contains(third, "first answer") || !strings.Contains(third, "second answer") || !strings.Contains(third, "此前轮次及本轮在你之前") {
		t.Fatalf("unexpected third context: %s", third)
	}
	for _, input := range gateway.inputs {
		if !strings.Contains(input.Messages[0].Content, discussionSerialPrompt) {
			t.Fatalf("missing serial instruction: %s", input.Messages[0].Content)
		}
	}
}

func TestDiscussionParallelRoundSharesPreviousContextOnly(t *testing.T) {
	gateway := &discussionStreamGatewayStub{responses: []string{"first answer", "second answer"}}
	service := discussionStreamTestServiceForModels(gateway, "model-a", "model-b")
	state := domaindiscussion.State{Turns: []domaindiscussion.Turn{{Round: 1, Status: "completed", Model: "previous", Content: "previous answer"}}}
	round := domaindiscussion.RuntimeRound{Index: 2, Steps: []domaindiscussion.RuntimeStep{
		{ID: "s1", Participant: "p1", Model: "model-a"},
		{ID: "s2", Participant: "p2", Model: "model-b"},
	}}
	results := service.runDiscussionRound(
		context.Background(),
		DiscussionSendInput{Content: "question"},
		&model.Conversation{ID: 1},
		round,
		&state,
		nil,
		func(string, map[string]any) {},
	)
	if len(results) != 2 || len(gateway.inputs) != 2 {
		t.Fatalf("results = %d, calls = %d", len(results), len(gateway.inputs))
	}
	contexts := []string{lastDiscussionUserContent(gateway.inputs[0]), lastDiscussionUserContent(gateway.inputs[1])}
	if contexts[0] != contexts[1] || !strings.Contains(contexts[0], "previous answer") || strings.Contains(contexts[0], "first answer") || strings.Contains(contexts[0], "second answer") {
		t.Fatalf("parallel contexts differ or include same-round answers: %#v", contexts)
	}
	for _, input := range gateway.inputs {
		if !strings.Contains(input.Messages[0].Content, discussionParallelPrompt) {
			t.Fatalf("missing parallel instruction: %s", input.Messages[0].Content)
		}
	}
}

func TestDiscussionSerialTurnUsesSerialDefaultPrompt(t *testing.T) {
	service := discussionStreamTestService()
	cases := []struct {
		roundIndex int
		want       string
	}{
		{1, discussionSerialDefaultStepPrompt},
		{2, discussionSerialFollowupPrompt},
	}
	for _, item := range cases {
		turn := service.runDiscussionTurn(
			context.Background(),
			DiscussionSendInput{Content: "question"},
			&model.Conversation{ID: 1},
			item.roundIndex,
			domaindiscussion.RuntimeStep{ID: "s1", Participant: "p1", Model: "model-a"},
			"",
			nil,
			false,
		)
		if len(turn.PromptMessages) == 0 {
			t.Fatalf("round %d has no prompt messages", item.roundIndex)
		}
		system := turn.PromptMessages[0].Content
		if !strings.Contains(system, item.want) {
			t.Fatalf("round %d serial prompt = %q, want %q", item.roundIndex, system, item.want)
		}
		if strings.Contains(system, discussionDefaultStepPrompt) || strings.Contains(system, discussionFollowupPrompt) {
			t.Fatalf("round %d serial turn used a parallel default: %s", item.roundIndex, system)
		}
	}
}

func TestDiscussionTurnEmitsStartWithPromptBeforeDelta(t *testing.T) {
	events := make([]string, 0, 2)
	startPrompt := ""
	var startMessages []discussionPromptMessagePayload
	turn := discussionStreamTestService().runDiscussionTurn(
		context.Background(),
		DiscussionSendInput{Content: "question", OnEvent: func(eventType string, payload map[string]any) error {
			events = append(events, eventType)
			if eventType == "discussion_turn_start" {
				startPrompt, _ = payload["prompt"].(string)
				startMessages, _ = payload["promptMessages"].([]discussionPromptMessagePayload)
			}
			return nil
		}},
		&model.Conversation{ID: 1},
		1,
		domaindiscussion.RuntimeStep{ID: "s1", Participant: "p1", Model: "model-a"},
		"",
		nil,
		true,
	)
	if strings.Join(events, ",") != "discussion_turn_start,discussion_turn_delta" {
		t.Fatalf("events = %v", events)
	}
	if !strings.Contains(startPrompt, discussionDefaultStepPrompt) || !strings.Contains(startPrompt, "user:\nquestion") {
		t.Fatalf("start prompt = %q", startPrompt)
	}
	if len(startMessages) != 2 || startMessages[0].Role != "system" || startMessages[1].Role != "user" || startMessages[1].Content != "question" {
		t.Fatalf("start messages = %#v", startMessages)
	}
	if turn.Status != "completed" || turn.Content != "ok" || len(turn.PromptMessages) != 2 {
		t.Fatalf("turn = %#v", turn)
	}
}

func TestDiscussionSummaryEmitsStartWithPromptBeforeDelta(t *testing.T) {
	events := make([]string, 0, 2)
	startPrompt := ""
	var startMessages []discussionPromptMessagePayload
	turn := discussionStreamTestService().runDiscussionSummary(
		context.Background(),
		DiscussionSendInput{Content: "question"},
		&model.Conversation{ID: 1},
		domaindiscussion.RuntimePlan{Summary: domaindiscussion.RuntimeSummary{Participant: "judge", Model: "model-a"}},
		nil,
		nil,
		func(eventType string, payload map[string]any) {
			events = append(events, eventType)
			if eventType == "discussion_summary_start" {
				startPrompt, _ = payload["prompt"].(string)
				startMessages, _ = payload["promptMessages"].([]discussionPromptMessagePayload)
			}
		},
	)
	if strings.Join(events, ",") != "discussion_summary_start,discussion_summary_delta" {
		t.Fatalf("events = %v", events)
	}
	if !strings.Contains(startPrompt, discussionSummaryPrompt) || !strings.Contains(startPrompt, "user:\nquestion") {
		t.Fatalf("start prompt = %q", startPrompt)
	}
	if len(startMessages) != 2 || startMessages[0].Role != "system" || startMessages[1].Role != "user" {
		t.Fatalf("start messages = %#v", startMessages)
	}
	if turn.Status != "completed" || turn.Content != "ok" || len(turn.PromptMessages) != 2 {
		t.Fatalf("turn = %#v", turn)
	}
}
