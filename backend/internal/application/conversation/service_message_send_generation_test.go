package conversation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func testRouteGenerationContext(conversation *model.Conversation) routeGenerationContext {
	return routeGenerationContext{
		input:                  SendMessageInput{UserID: 7, ConversationID: 11, Content: "second question"},
		conversation:           conversation,
		cfg:                    config.Config{},
		promptCacheSessionKey:  "session-1",
		statefulContextConfig:  "cfg",
		statefulContextState:   "state",
		normalizedBranchReason: "default",
	}
}

func testRoutePromptPlan() PromptPlan {
	return PromptPlan{Messages: []llm.Message{
		{Role: "system", Content: "stable policy"},
		{Role: "user", Content: "first question"},
		{Role: "assistant", Content: "first answer"},
		{Role: "user", Content: "second question"},
	}}
}

func TestPrepareRouteGenerationFailoverAlwaysSendsFullContext(t *testing.T) {
	service := &Service{}
	route := &channel.ResolvedRoute{Protocol: llm.AdapterOpenAIResponses, BaseURL: "https://api.openai.com/v1", UpstreamModel: "gpt-test"}
	conversation := &model.Conversation{PublicID: "conv_1", LastResponseID: "resp_123", LastPromptFingerprint: "fp_stale"}

	plan := service.prepareRouteGeneration(context.Background(), routeGenerationPreparationInput{
		Generation: testRouteGenerationContext(conversation), Route: route, PromptPlan: testRoutePromptPlan(), Mode: routeGenerationFailover,
	})

	if plan.promptMode != "route_failover" || plan.statefulDecision.DisabledReason != "route_failover" {
		t.Fatalf("failover plan must be labelled as such, got mode=%q decision=%#v", plan.promptMode, plan.statefulDecision)
	}
	if plan.statefulContinuation || plan.generateInput.PreviousResponseID != "" {
		t.Fatalf("failover must not continue a stateful response, got %#v", plan.generateInput)
	}
	if len(plan.fullLLMMessages) != 4 || len(plan.llmMessages) != 4 {
		t.Fatalf("full context must be retained, got full=%d llm=%d", len(plan.fullLLMMessages), len(plan.llmMessages))
	}
	if plan.generateInput.Instructions == "" || len(plan.generateInput.Messages) != 3 {
		t.Fatalf("Responses route must lift the system prompt into instructions, got %#v", plan.generateInput)
	}
	if plan.estimatedPromptTokens <= 0 || plan.fullContextPromptTokens < plan.estimatedPromptTokens {
		t.Fatalf("token estimates must be populated, got estimated=%d full=%d", plan.estimatedPromptTokens, plan.fullContextPromptTokens)
	}
	if plan.statefulPrefixFingerprint == "" || plan.routeConfig.Protocol != llm.AdapterOpenAIResponses {
		t.Fatalf("plan must carry the fingerprint and route config, got %#v", plan)
	}
}

func TestPrepareRouteGenerationInitialContinuesMatchingStatefulResponse(t *testing.T) {
	service := &Service{}
	route := &channel.ResolvedRoute{Protocol: llm.AdapterOpenAIResponses, BaseURL: "https://api.openai.com/v1", UpstreamModel: "gpt-test"}
	conversation := &model.Conversation{PublicID: "conv_1", LastResponseID: "resp_123"}
	gen := testRouteGenerationContext(conversation)

	// 指纹由完整请求形状推导；先按同样输入计算一次，再模拟数据库中存有匹配指纹的上一轮。
	probe := service.prepareRouteGeneration(context.Background(), routeGenerationPreparationInput{
		Generation: gen, Route: route, PromptPlan: testRoutePromptPlan(), Mode: routeGenerationFailover,
	})
	conversation.LastPromptFingerprint = probe.statefulPrefixFingerprint

	plan := service.prepareRouteGeneration(context.Background(), routeGenerationPreparationInput{
		Generation: gen, Route: route, PromptPlan: testRoutePromptPlan(), Mode: routeGenerationInitial,
	})
	if plan.promptMode != "stateful" || !plan.statefulContinuation || plan.generateInput.PreviousResponseID != "resp_123" {
		t.Fatalf("expected stateful continuation, got mode=%q continuation=%v input=%#v", plan.promptMode, plan.statefulContinuation, plan.generateInput)
	}
	if len(plan.generateInput.Messages) >= len(plan.fullLLMMessages) {
		t.Fatalf("stateful continuation must only send the new turn, sent=%d full=%d", len(plan.generateInput.Messages), len(plan.fullLLMMessages))
	}

	conversation.LastPromptFingerprint = "fp_other"
	fullPlan := service.prepareRouteGeneration(context.Background(), routeGenerationPreparationInput{
		Generation: gen, Route: route, PromptPlan: testRoutePromptPlan(), Mode: routeGenerationInitial,
	})
	if fullPlan.promptMode != "full" || fullPlan.statefulDecision.DisabledReason != "prompt_fingerprint_mismatch" {
		t.Fatalf("mismatched fingerprint must fall back to the full prompt, got mode=%q decision=%#v", fullPlan.promptMode, fullPlan.statefulDecision)
	}
}

func TestMessageGenerationRunnerEmitVisibleDeltaTracksFirstTokenAndText(t *testing.T) {
	var received []string
	runner := &messageGenerationRunner{
		startedAt: time.Now().Add(-50 * time.Millisecond),
		onDelta: func(delta string) error {
			received = append(received, delta)
			return nil
		},
	}
	if err := runner.emitVisibleDelta(""); err != nil || runner.visibleDeltaCount != 0 {
		t.Fatalf("empty delta must be ignored, err=%v count=%d", err, runner.visibleDeltaCount)
	}
	if err := runner.emitVisibleDelta("Hel"); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if err := runner.emitVisibleDelta("lo"); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if runner.visibleDeltaCount != 2 || runner.streamedText.String() != "Hello" || len(received) != 2 {
		t.Fatalf("unexpected runner state count=%d text=%q received=%v", runner.visibleDeltaCount, runner.streamedText.String(), received)
	}
	if runner.firstVisibleDeltaLatencyMS < 50 {
		t.Fatalf("first token latency must be measured from run start, got %d", runner.firstVisibleDeltaLatencyMS)
	}

	failing := &messageGenerationRunner{startedAt: time.Now(), onDelta: func(string) error { return errors.New("client gone") }}
	if err := failing.emitVisibleDelta("x"); err == nil || failing.streamedText.Len() != 0 {
		t.Fatalf("delivery failure must propagate and not record text, err=%v text=%q", err, failing.streamedText.String())
	}
}

func TestMessageGenerationRunnerBeginRouteFailoverResetsRouteLocalState(t *testing.T) {
	runner := &messageGenerationRunner{llmRequestCount: 2, completedLLMCallCount: 1, attemptHadSideEffect: true}
	runner.streamedText.WriteString("partial")

	runner.beginRouteFailover(llm.RouteConfig{Protocol: llm.AdapterOpenAIChatCompletions})

	if runner.attemptHadSideEffect || runner.streamedText.Len() != 0 {
		t.Fatalf("route-local observations must reset, got sideEffect=%v text=%q", runner.attemptHadSideEffect, runner.streamedText.String())
	}
	if runner.llmRequestCount != 2 || runner.completedLLMCallCount != 1 || runner.routeConfig.Protocol != llm.AdapterOpenAIChatCompletions {
		t.Fatalf("call counters must survive failover and route config must switch, got %#v", runner)
	}
}

func TestMergeFollowUpUsagePrefersObservedTotals(t *testing.T) {
	usage := &messageUsageAccumulator{}
	total := llm.Usage{InputTokens: 10, OutputTokens: 5}

	observed := mergeFollowUpUsage(total, &llm.GenerateOutput{Usage: llm.Usage{InputTokens: 20, OutputTokens: 7}}, usage)
	if observed.InputTokens != 30 || observed.OutputTokens != 12 {
		t.Fatalf("observed usage must accumulate, got %#v", observed)
	}
	if usage.usage() != observed {
		t.Fatalf("accumulator must be synchronised with the observed total, got %#v", usage.usage())
	}

	unreported := mergeFollowUpUsage(observed, &llm.GenerateOutput{}, usage)
	if unreported != observed {
		t.Fatalf("an unreported call must fall back to the accumulator total, got %#v", unreported)
	}
}

func TestGenerationCanceledDistinguishesUserStopFromUpstreamFailure(t *testing.T) {
	if generationCanceled(context.Background(), nil) {
		t.Fatal("nil error is never a cancellation")
	}
	if generationCanceled(context.Background(), errors.New("upstream 500")) {
		t.Fatal("plain upstream failures must not be reported as cancellations")
	}
	if !generationCanceled(context.Background(), ErrMessageGenerationCanceled) {
		t.Fatal("explicit stop must be a cancellation")
	}
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if !generationCanceled(canceledCtx, errors.New("upstream aborted")) {
		t.Fatal("any error after the request context ends must be a cancellation")
	}
}
