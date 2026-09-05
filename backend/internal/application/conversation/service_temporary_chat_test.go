package conversation

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/extraction"
	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	extractport "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/extract"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type temporaryLLMGatewayStub struct {
	inputs           []llm.GenerateInput
	onGenerateStream func(llm.GenerateInput)
	// output 非空时作为上游返回，用于构造带 usage 的响应。
	output *llm.GenerateOutput
}

type temporaryBuiltinParserStub struct{}

func (temporaryBuiltinParserStub) ExtractText(data []byte) string { return string(data) }

func (temporaryBuiltinParserStub) ExtractWordText(
	context.Context,
	string,
	[]byte,
	string,
	string,
) extractport.WordTextResult {
	return extractport.WordTextResult{}
}

func (temporaryBuiltinParserStub) ExtractExcelText([]byte, string, string) string { return "" }

func (temporaryBuiltinParserStub) ExtractPDFText(string, int) (string, error) { return "", nil }

func (temporaryBuiltinParserStub) ExtractPDFPages(string, int) (extractport.PDFTextResult, error) {
	return extractport.PDFTextResult{}, nil
}

func (temporaryBuiltinParserStub) DetectPDFPageCount(string) int { return 0 }

func (s *temporaryLLMGatewayStub) Generate(context.Context, llm.RouteConfig, llm.GenerateInput) (*llm.GenerateOutput, error) {
	return nil, nil
}

func (s *temporaryLLMGatewayStub) GenerateStream(
	_ context.Context,
	_ llm.RouteConfig,
	input llm.GenerateInput,
	_ func(llm.GenerateStreamEvent) error,
) (*llm.GenerateOutput, error) {
	if s.onGenerateStream != nil {
		s.onGenerateStream(input)
	}
	s.inputs = append(s.inputs, input)
	if s.output != nil {
		return s.output, nil
	}
	return &llm.GenerateOutput{Text: "ok"}, nil
}

func (s *temporaryLLMGatewayStub) RetrieveOpenAIResponse(context.Context, llm.RouteConfig, string) (*llm.GenerateOutput, error) {
	return nil, nil
}

func (s *temporaryLLMGatewayStub) CancelOpenAIResponse(context.Context, llm.RouteConfig, string) (*llm.GenerateOutput, error) {
	return nil, nil
}

type temporaryPersistenceRepositoryStub struct {
	repository.ConversationRepository
	traceWrites      int
	traceEventWrites int
	toolCallWrites   int
}

func (s *temporaryPersistenceRepositoryStub) UpsertConversationMessageTrace(context.Context, *model.MessageTrace) error {
	s.traceWrites++
	return nil
}

func (s *temporaryPersistenceRepositoryStub) UpsertConversationMessageTraceEvent(context.Context, *model.MessageTraceEventRow) error {
	s.traceEventWrites++
	return nil
}

func (s *temporaryPersistenceRepositoryStub) CreateConversationToolCall(context.Context, *model.ToolCall) error {
	s.toolCallWrites++
	return nil
}

func TestValidateTemporaryChatInput(t *testing.T) {
	valid := TemporaryChatInput{
		UserID:      1,
		SessionID:   "session",
		ClientRunID: "run",
		Model:       "model",
		Messages: []TemporaryChatMessage{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi"},
			{Role: "user", Content: "continue"},
		},
	}
	if err := ValidateTemporaryChatInput(valid); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	attachmentOnly := valid
	attachmentOnly.Messages = []TemporaryChatMessage{{Role: "user"}}
	attachmentOnly.Attachments = []TemporaryChatAttachment{{
		MessageIndex: 0,
		FileName:     "notes.txt",
		MimeType:     "text/plain",
		Reader:       strings.NewReader("notes"),
	}}
	if err := ValidateTemporaryChatInput(attachmentOnly); err != nil {
		t.Fatalf("attachment-only input rejected: %v", err)
	}

	tests := map[string]TemporaryChatInput{
		"assistant last": {
			UserID: 1, SessionID: "session", ClientRunID: "run", Model: "model",
			Messages: []TemporaryChatMessage{{Role: "user", Content: "hello"}, {Role: "assistant", Content: "hi"}},
		},
		"consecutive roles": {
			UserID: 1, SessionID: "session", ClientRunID: "run", Model: "model",
			Messages: []TemporaryChatMessage{{Role: "user", Content: "one"}, {Role: "user", Content: "two"}},
		},
		"system role": {
			UserID: 1, SessionID: "session", ClientRunID: "run", Model: "model",
			Messages: []TemporaryChatMessage{{Role: "system", Content: "secret"}},
		},
		"duplicate knowledge base": {
			UserID: 1, SessionID: "session", ClientRunID: "run", Model: "model",
			KnowledgeBaseIDs: []string{"kb-one", "kb-one"},
			Messages:         []TemporaryChatMessage{{Role: "user", Content: "hello"}},
		},
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateTemporaryChatInput(input); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
}

func TestStreamTemporaryChatSendsRequestScopedImageWithoutPersistence(t *testing.T) {
	gateway := &temporaryLLMGatewayStub{}
	runtimeCfg := config.NewRuntime(config.Config{
		MaxUploadFileBytes:       1024 * 1024,
		MaxMessageFiles:          10,
		FileAllowedMIMETypes:     "image/png",
		ImageMaxDimension:        1024,
		ModelOptionPolicyMode:    modelOptionPolicyAllowlist,
		ModelOptionAllowedPaths:  config.DefaultModelOptionAllowedPathsJSON(),
		ModelOptionDeniedPaths:   config.DefaultModelOptionDeniedPathsJSON(),
		FileFullContextMaxBytes:  config.DefaultFileFullContextMaxBytes,
		FileFullContextMaxTokens: 65536,
	})
	service := &Service{
		cfg: runtimeCfg,
		routeResolver: &textTaskRouteResolverStub{routes: map[string]*channel.ResolvedRoute{
			"vision": {
				PlatformModelName: "vision",
				UpstreamModel:     "vision",
				Protocol:          llm.AdapterOpenAIChatCompletions,
			},
		}},
		llmClient:  gateway,
		uploadSvc:  appupload.NewServiceWithRuntime(runtimeCfg, nil, nil, appupload.Hooks{}, appupload.ErrorSet{}, ""),
		extractSvc: extraction.NewServiceWithRuntime(runtimeCfg, extraction.EngineFactories{}),
	}
	var imageData bytes.Buffer
	sourceImage := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	sourceImage.Set(0, 0, color.NRGBA{R: 1, G: 2, B: 3, A: 255})
	if err := png.Encode(&imageData, sourceImage); err != nil {
		t.Fatalf("encode image: %v", err)
	}

	if _, err := service.StreamTemporaryChat(t.Context(), TemporaryChatInput{
		UserID:      1,
		SessionID:   "temporary-session",
		ClientRunID: "temporary-run",
		Model:       "vision",
		Messages:    []TemporaryChatMessage{{Role: "user", Content: "describe this image"}},
		Attachments: []TemporaryChatAttachment{{
			MessageIndex: 0,
			FileName:     "image.png",
			MimeType:     "image/png",
			DeclaredSize: int64(imageData.Len()),
			Reader:       bytes.NewReader(imageData.Bytes()),
		}},
	}, nil); err != nil {
		t.Fatalf("stream temporary chat: %v", err)
	}
	if len(gateway.inputs) != 1 || len(gateway.inputs[0].Messages) != 1 {
		t.Fatalf("unexpected upstream input: %#v", gateway.inputs)
	}
	message := gateway.inputs[0].Messages[0]
	if len(message.Parts) != 2 || message.Parts[0].Kind != llm.ContentPartText || message.Parts[1].Kind != llm.ContentPartImage {
		t.Fatalf("temporary image was not attached to its user message: %#v", message)
	}
}

func TestStreamTemporaryChatExtractsDocumentAndReleasesUploadSourceBeforeGeneration(t *testing.T) {
	released := false
	gateway := &temporaryLLMGatewayStub{
		onGenerateStream: func(input llm.GenerateInput) {
			if !released {
				t.Error("temporary upload source remained open during upstream generation")
			}
		},
	}
	runtimeCfg := config.NewRuntime(config.Config{
		MaxUploadFileBytes:       1024 * 1024,
		MaxMessageFiles:          10,
		FileAllowedMIMETypes:     "text/plain",
		ModelOptionPolicyMode:    modelOptionPolicyAllowlist,
		ModelOptionAllowedPaths:  config.DefaultModelOptionAllowedPathsJSON(),
		ModelOptionDeniedPaths:   config.DefaultModelOptionDeniedPathsJSON(),
		FileFullContextMaxBytes:  config.DefaultFileFullContextMaxBytes,
		FileFullContextMaxTokens: 65536,
	})
	service := &Service{
		cfg: runtimeCfg,
		routeResolver: &textTaskRouteResolverStub{routes: map[string]*channel.ResolvedRoute{
			"text": {
				PlatformModelName: "text",
				UpstreamModel:     "text",
				Protocol:          llm.AdapterOpenAIChatCompletions,
			},
		}},
		llmClient:  gateway,
		uploadSvc:  appupload.NewServiceWithRuntime(runtimeCfg, nil, nil, appupload.Hooks{}, appupload.ErrorSet{}, ""),
		extractSvc: extraction.NewServiceWithRuntime(runtimeCfg, extraction.EngineFactories{Builtin: temporaryBuiltinParserStub{}}),
	}

	if _, err := service.StreamTemporaryChat(t.Context(), TemporaryChatInput{
		UserID:      1,
		SessionID:   "temporary-session",
		ClientRunID: "temporary-run",
		Model:       "text",
		Messages:    []TemporaryChatMessage{{Role: "user", Content: "summarize the attachment"}},
		Attachments: []TemporaryChatAttachment{{
			MessageIndex: 0,
			FileName:     "notes.txt",
			MimeType:     "text/plain",
			DeclaredSize: int64(len("request-scoped document content")),
			Reader:       strings.NewReader("request-scoped document content"),
		}},
		ReleaseAttachmentSources: func() { released = true },
	}, nil); err != nil {
		t.Fatalf("stream temporary chat: %v", err)
	}
	if !released {
		t.Fatal("temporary upload source was not released")
	}
	if len(gateway.inputs) != 1 {
		t.Fatalf("expected one upstream call, got %d", len(gateway.inputs))
	}
	var contextText strings.Builder
	for _, message := range gateway.inputs[0].Messages {
		contextText.WriteString(message.Content)
		for _, part := range message.Parts {
			contextText.WriteString(part.Text)
		}
	}
	if !strings.Contains(contextText.String(), "request-scoped document content") {
		t.Fatalf("temporary document was not injected into model context: %q", contextText.String())
	}
}

func TestStripTemporaryChatProviderStateOptions(t *testing.T) {
	input := map[string]any{
		"temperature":          0.5,
		"store":                true,
		"cache_control":        map[string]any{"type": "ephemeral"},
		"cachedContent":        "cachedContents/example",
		"prompt_cache_options": map[string]any{"mode": "explicit"},
	}
	result := stripTemporaryChatProviderStateOptions(input)
	if result["temperature"] != 0.5 {
		t.Fatalf("ordinary generation option lost: %#v", result)
	}
	for _, key := range []string{"store", "cache_control", "cachedContent", "prompt_cache_options"} {
		if _, ok := result[key]; ok {
			t.Fatalf("provider state option %q was retained: %#v", key, result)
		}
	}
	if _, ok := input["store"]; !ok {
		t.Fatal("input map must not be mutated")
	}
}

func TestEnforceTemporaryGenerateInput(t *testing.T) {
	input := llm.GenerateInput{
		PreviousResponseID:  "response-1",
		PromptCacheKey:      "cache-1",
		ResponsesBackground: true,
	}

	result := enforceTemporaryGenerateInput(input)
	if !result.Ephemeral {
		t.Fatal("temporary generation must remain ephemeral")
	}
	if result.PreviousResponseID != "" || result.PromptCacheKey != "" || result.ResponsesBackground {
		t.Fatalf("temporary generation retained provider state: %#v", result)
	}
}

func TestStreamTemporaryChatPreservesProviderNativeToolsWithoutMCPTools(t *testing.T) {
	tests := []struct {
		name             string
		capabilitiesJSON string
		options          map[string]any
		expectedType     string
	}{
		{
			name: "model default",
			capabilitiesJSON: `{
				"defaultOptions": {
					"tools": [{"type": "web_search", "enable_image_understanding": true}]
				}
			}`,
			expectedType: "web_search",
		},
		{
			name:             "user option",
			capabilitiesJSON: `{"nativeToolKeys":["xai.x_search"]}`,
			options: map[string]any{
				"tools": []any{
					map[string]any{
						"type":                       "x_search",
						"enable_image_understanding": true,
						"allowed_domains":            []any{"x.com"},
					},
				},
			},
			expectedType: "x_search",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gateway := &temporaryLLMGatewayStub{}
			service := &Service{
				cfg: config.NewRuntime(config.Config{
					ModelOptionPolicyMode:   modelOptionPolicyAllowlist,
					ModelOptionAllowedPaths: config.DefaultModelOptionAllowedPathsJSON(),
					ModelOptionDeniedPaths:  config.DefaultModelOptionDeniedPathsJSON(),
				}),
				routeResolver: &textTaskRouteResolverStub{routes: map[string]*channel.ResolvedRoute{
					"grok": {
						PlatformModelName:     "grok",
						UpstreamModel:         "grok-4.20-multi-agent-0309",
						Protocol:              llm.AdapterXAIResponses,
						ModelCapabilitiesJSON: test.capabilitiesJSON,
					},
				}},
				llmClient: gateway,
			}

			if _, err := service.StreamTemporaryChat(t.Context(), TemporaryChatInput{
				UserID:      1,
				SessionID:   "temporary-session",
				ClientRunID: "temporary-run",
				Model:       "grok",
				Options:     test.options,
				Messages: []TemporaryChatMessage{
					{Role: "user", Content: "search the web"},
				},
			}, nil); err != nil {
				t.Fatalf("stream temporary chat: %v", err)
			}

			if len(gateway.inputs) != 1 {
				t.Fatalf("expected one upstream call, got %d", len(gateway.inputs))
			}
			generateInput := gateway.inputs[0]
			if generateInput.DisableTools {
				t.Fatal("temporary chat disabled provider-native tools when no MCP tool was selected")
			}
			if len(generateInput.Tools) != 0 {
				t.Fatalf("expected no MCP tool declarations, got %#v", generateInput.Tools)
			}
			tools, ok := generateInput.Options["tools"].([]map[string]any)
			if !ok || len(tools) != 1 || tools[0]["type"] != test.expectedType {
				t.Fatalf("expected %s provider-native tool, got %#v", test.expectedType, generateInput.Options["tools"])
			}
			if tools[0]["enable_image_understanding"] != true {
				t.Fatalf("expected provider-native tool parameters to remain, got %#v", tools[0])
			}
		})
	}
}

func TestStreamTemporaryChatKeepsFullyCachedInputTokensObserved(t *testing.T) {
	gateway := &temporaryLLMGatewayStub{output: &llm.GenerateOutput{
		Text:  "ok",
		Usage: llm.Usage{CacheReadTokens: 3355, OutputTokens: 23, ReasoningTokens: 49},
	}}
	service := &Service{
		cfg: config.NewRuntime(config.Config{
			ModelOptionPolicyMode:   modelOptionPolicyAllowlist,
			ModelOptionAllowedPaths: config.DefaultModelOptionAllowedPathsJSON(),
			ModelOptionDeniedPaths:  config.DefaultModelOptionDeniedPathsJSON(),
		}),
		routeResolver: &textTaskRouteResolverStub{routes: map[string]*channel.ResolvedRoute{
			"chat": {
				PlatformModelName: "chat",
				UpstreamModel:     "chat",
				Protocol:          llm.AdapterOpenAIChatCompletions,
			},
		}},
		llmClient: gateway,
	}

	result, err := service.StreamTemporaryChat(t.Context(), TemporaryChatInput{
		UserID:      1,
		SessionID:   "temporary-session",
		ClientRunID: "temporary-run",
		Model:       "chat",
		Messages:    []TemporaryChatMessage{{Role: "user", Content: strings.Repeat("cached prompt ", 200)}},
	}, nil)
	if err != nil {
		t.Fatalf("stream temporary chat: %v", err)
	}

	if result.UserMessage.InputTokens != 0 || result.UserMessage.TokenUsage != 0 {
		t.Fatalf("expected fully cached prompt to keep zero non-cached input, got input=%d usage=%d", result.UserMessage.InputTokens, result.UserMessage.TokenUsage)
	}
	if result.UserMessage.CacheReadTokens != 3355 {
		t.Fatalf("expected cache read tokens to be preserved, got %d", result.UserMessage.CacheReadTokens)
	}
	if result.AssistantMessage.OutputTokens != 23 || result.AssistantMessage.ReasoningTokens != 49 {
		t.Fatalf("unexpected output usage: %#v", result.AssistantMessage)
	}
	if result.UsageSource != "observed" {
		t.Fatalf("expected fully cached usage to be observed, got %q", result.UsageSource)
	}
}

func TestStripTemporaryMessageCacheControls(t *testing.T) {
	cacheControl := &llm.CacheControl{Type: "ephemeral", TTL: "1h"}
	input := []llm.Message{{
		Role:         "system",
		Content:      "temporary context",
		CacheControl: cacheControl,
		Parts: []llm.ContentPart{{
			Kind:         "text",
			Text:         "temporary context",
			CacheControl: cacheControl,
		}},
	}}
	result := stripTemporaryMessageCacheControls(input)
	if result[0].CacheControl != nil || result[0].Parts[0].CacheControl != nil {
		t.Fatalf("temporary message retained a provider cache marker: %#v", result[0])
	}
	if input[0].CacheControl == nil || input[0].Parts[0].CacheControl == nil {
		t.Fatal("input messages must not be mutated")
	}
}

func TestEphemeralTraceEmitsWithoutPersistence(t *testing.T) {
	repo := &temporaryPersistenceRepositoryStub{}
	service := &Service{
		cfg: config.NewRuntime(config.Config{
			ProcessTraceEnabled:         true,
			ProcessTraceVisibleToUser:   true,
			ProcessTracePersistInflight: true,
		}),
		repo: repo,
	}
	eventCount := 0
	promptTraceVisible := false
	recorder := newEphemeralMessageTraceRecorder(service, t.Context(), &model.Message{
		PublicID: "temporary-message",
		UserID:   1,
		RunID:    "temporary-run",
	}, func(eventType string, payload map[string]any) error {
		eventCount++
		if eventType == "process_update" {
			if trace, ok := payload["trace"].(*model.MessageProcessTrace); ok && trace.PromptTrace != nil {
				promptTraceVisible = true
			}
		}
		return nil
	})
	recorder.recordPromptTrace(&model.MessagePromptTrace{
		Mode:               "full",
		SentTokenEstimate:  12,
		FullMessageCount:   1,
		SentMessageCount:   1,
		TotalTokenEstimate: 12,
		Blocks: []model.MessagePromptTraceBlock{{
			Kind:          string(PromptBlockTranscript),
			Title:         "历史对话",
			TokenEstimate: 12,
			SourceCount:   1,
		}},
	})
	recorder.appendProcessSection("检索", "完成", nil, messageTraceStatusCompleted)
	recorder.complete()
	recorder.waitForPendingPersistence(t.Context())

	if eventCount == 0 {
		t.Fatal("expected ephemeral trace to remain visible to the current browser")
	}
	if !promptTraceVisible {
		t.Fatal("expected ephemeral prompt context to be emitted to the current browser")
	}
	if repo.traceWrites != 0 || repo.traceEventWrites != 0 {
		t.Fatalf("ephemeral trace was persisted: trace=%d event=%d", repo.traceWrites, repo.traceEventWrites)
	}
}
