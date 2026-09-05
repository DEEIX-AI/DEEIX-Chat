package conversation

import (
	"context"
	"errors"
	"strings"
	"time"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	appcm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/contentmoderation"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/google/uuid"
)

const temporaryChatMaxContentChars = 1_000_000

// TemporaryChatMessage 是浏览器内临时对话的一条消息。
type TemporaryChatMessage struct {
	Role    string
	Content string
}

// TemporaryChatInput 描述不创建会话、消息和运行记录的临时推理请求。
type TemporaryChatInput struct {
	UserID                   uint
	RequestID                string
	SessionID                string
	ClientRunID              string
	Model                    string
	Options                  map[string]any
	SelectedToolIDs          []uint
	SkillIDs                 []uint
	KnowledgeBaseIDs         []string
	HTMLVisualPromptEnabled  bool
	Messages                 []TemporaryChatMessage
	Attachments              []TemporaryChatAttachment
	ReleaseAttachmentSources func()
	// UsageAuthorization 是请求级计费授权；提示词形状确定后据此把预算预留抬高到预估成本。
	UsageAuthorization *domainbilling.UsageAuthorization
	OnEvent            func(eventType string, payload map[string]any) error
}

// StreamTemporaryChat 直接以请求上下文调用上游。调用方断开连接时生成随即取消，
// 且不会注册普通会话使用的断线续传缓存。
func (s *Service) StreamTemporaryChat(
	ctx context.Context,
	input TemporaryChatInput,
	onDelta func(string) error,
) (*SendMessageResult, error) {
	if err := ValidateTemporaryChatInput(input); err != nil {
		return nil, err
	}
	if s.routeResolver == nil || s.llmClient == nil {
		return nil, ErrModelRouteNotConfigured
	}

	startedAt := time.Now()
	route, err := s.routeResolver.ResolveRoute(ctx, channel.ResolveRouteInput{
		PlatformModelName: strings.TrimSpace(input.Model),
		TaskType:          channel.TaskTypeChat,
		Scope:             channel.RouteScopeUser,
		UserID:            input.UserID,
		RequestID:         strings.TrimSpace(input.RequestID),
	})
	if err != nil {
		return nil, mapRouteResolutionError(err)
	}
	attributionReferer, attributionTitle := s.llmAttribution()
	routeConfig := messageRouteConfig(route, attributionReferer, attributionTitle)

	cfg := s.cfg.Snapshot()
	runID := strings.TrimSpace(input.ClientRunID)
	lastUser := input.Messages[len(input.Messages)-1]
	temporaryAssistant := model.Message{
		PublicID:    uuid.NewString(),
		UserID:      input.UserID,
		RunID:       runID,
		Role:        "assistant",
		ContentType: "markdown",
		Status:      "pending",
		CreatedAt:   startedAt,
		UpdatedAt:   startedAt,
	}
	traceRecorder := newEphemeralMessageTraceRecorder(s, ctx, &temporaryAssistant, input.OnEvent)
	messages := make([]llm.Message, 0, len(input.Messages)+1)
	for _, item := range input.Messages {
		messages = append(messages, llm.Message{Role: item.Role, Content: item.Content})
	}
	attachmentContext, err := s.prepareTemporaryAttachmentContext(ctx, input, messages)
	if input.ReleaseAttachmentSources != nil {
		input.ReleaseAttachmentSources()
	}
	if err != nil {
		return nil, err
	}
	messages = attachmentContext.messages
	systemPrompt := resolveMessageSystemPromptInjection(cfg, route, "", input.HTMLVisualPromptEnabled)
	if systemPrompt.Content != "" {
		if systemPrompt.InlineToUser {
			messages = inlineSystemPromptIntoLatestUserMessage(messages, systemPrompt.Content)
		} else {
			messages = append([]llm.Message{{Role: "system", Content: systemPrompt.Content}}, messages...)
		}
	}
	if err := s.ValidateSelectedToolIDs(input.SelectedToolIDs); err != nil {
		return nil, err
	}
	toolRuntime, err := s.resolveSelectedToolRuntime(ctx, input.SelectedToolIDs)
	if err != nil {
		return nil, err
	}
	// Attachment processors depend on persisted file IDs. Request-scoped temporary
	// attachments are injected directly into the model context instead.
	toolRuntime = toolRuntime.withoutAttachmentProcessor()
	skillPrompts, err := s.resolveSkillPrompts(ctx, SendMessageInput{
		UserID:   input.UserID,
		SkillIDs: input.SkillIDs,
	})
	if err != nil {
		traceRecorder.fail(err)
		return nil, err
	}
	recordSkillPromptTrace(traceRecorder, skillPrompts)
	knowledgeContext, knowledgeSources, err := s.prepareTemporaryKnowledgeContext(ctx, input, traceRecorder)
	if err != nil {
		traceRecorder.fail(err)
		return nil, err
	}
	promptPlan := buildPromptPlan(ctx, promptPlanInput{
		BaseMessages:      messages,
		StableAttachments: attachmentContext.stableAttachments,
		DynamicContext:    knowledgeContext,
		SkillPrompts:      skillPrompts,
		ToolRuntime:       toolRuntime,
		Config:            cfg,
	})
	messages = stripTemporaryMessageCacheControls(promptPlan.Messages)
	fullMessages := cloneLLMMessages(messages)
	filteredOptions := filterModelOptions(input.Options, route.Protocol, modelOptionPolicyConfig{
		Mode:                  cfg.ModelOptionPolicyMode,
		AllowedPathsJSON:      cfg.ModelOptionAllowedPaths,
		DeniedPathsJSON:       cfg.ModelOptionDeniedPaths,
		ModelCapabilitiesJSON: route.ModelCapabilitiesJSON,
	})
	filteredOptions = stripTemporaryChatProviderStateOptions(filteredOptions)
	var moderationCoord *appcm.RunCoordinator
	if s.moderationSvc != nil {
		moderationCoord = s.moderationSvc.BeginRun(ctx, appcm.RunMeta{
			UserID:          input.UserID,
			RunID:           strings.TrimSpace(input.ClientRunID),
			MessagePublicID: uuid.NewString(),
			Ephemeral:       true,
		})
		if moderationCoord != nil {
			if input.OnEvent != nil {
				moderationCoord.SetLiveEmitter(moderationLiveEmitter(input.OnEvent, input.UsageAuthorization))
			}
			moderationCoord.EnqueueInputText(lastUser.Content)
			moderationCoord.EnqueueInputImageSources(attachmentContext.moderationImages)
		}
	}
	generateInput := llm.GenerateInput{
		RequestID: strings.TrimSpace(input.RequestID),
		Messages:  messages,
		Tools:     toolRuntime.definitions,
		Options:   filteredOptions,
		Ephemeral: true,
	}
	var budgetFit promptBudgetFit
	generateInput, budgetFit = fitGenerateInputToModelBudget(
		generateInput,
		route.UpstreamModel,
		route.ModelCapabilitiesJSON,
		cfg.ContextWindowFallbackTokens,
		cfg.ContextTokenBudgetEnabled,
	)
	if budgetFit.Trimmed {
		promptPlan.applyMessages(generateInput.Messages)
	}
	applyOpenAIResponsesInstructions(route, routeConfig.Endpoint, &generateInput)
	traceRecorder.recordPromptTrace(buildMessagePromptTrace(messagePromptTraceInput{
		Plan:         promptPlan.Trace,
		Mode:         "full",
		SentMessages: generateInput.Messages,
		FullMessages: fullMessages,
	}))
	if err := s.ensureUsageBudgetCoversEstimate(ctx, input.UsageAuthorization, route, filteredOptions, usageBudgetEstimate{
		InputTokens:  estimateGenerateInputTokens(generateInput),
		OutputTokens: messageRequestMaxOutputTokens(filteredOptions),
	}); err != nil {
		if moderationCoord != nil {
			moderationCoord.WaitInputOnly(ctx)
		}
		traceRecorder.fail(err)
		return nil, err
	}

	generation, generateErr := s.runTemporaryGeneration(ctx, temporaryGenerationInput{
		Request:       input,
		Route:         route,
		RouteConfig:   routeConfig,
		InitialInput:  generateInput,
		ToolRuntime:   toolRuntime,
		TraceRecorder: traceRecorder,
		StartedAt:     startedAt,
		OnDelta:       onDelta,
	})
	output := generation.Output
	if generateErr != nil {
		if output == nil || output.Text == "" && generation.Usage == (llm.Usage{}) {
			if moderationCoord != nil {
				moderationCoord.WaitInputOnly(ctx)
			}
			traceRecorder.fail(generateErr)
			return nil, wrapTemporaryGenerationError(generateErr)
		}
	}
	if output == nil {
		if moderationCoord != nil {
			moderationCoord.WaitInputOnly(ctx)
		}
		traceRecorder.fail(ErrUpstreamEmptyResponse)
		return nil, ErrUpstreamEmptyResponse
	}
	assistantText := output.Text
	if generateErr == nil && strings.TrimSpace(assistantText) == "" {
		if moderationCoord != nil {
			moderationCoord.WaitInputOnly(ctx)
		}
		traceRecorder.fail(ErrUpstreamEmptyResponse)
		return nil, ErrUpstreamEmptyResponse
	}
	usage := generation.Usage
	// 非缓存输入为 0 是全部命中缓存时的合法观测值，只有上游完全没上报输入侧用量才用预估补齐。
	inputObserved := usage.HasObservedInput()
	outputObserved := usage.OutputTokens > 0
	inputTokens := usage.InputTokens
	if !inputObserved {
		inputTokens = estimateGenerateInputTokens(generateInput)
	}
	outputTokens := resolveObservedOrEstimatedOutputTokens(usage.OutputTokens, assistantText)
	usageSource := "observed"
	switch {
	case !inputObserved && !outputObserved:
		usageSource = "estimated"
	case !inputObserved || !outputObserved:
		usageSource = "mixed"
	}
	firstTokenLatencyMS := generation.FirstTokenLatency
	if firstTokenLatencyMS <= 0 {
		firstTokenLatencyMS = time.Since(startedAt).Milliseconds()
	}
	now := time.Now()
	temporaryAssistant.Content = assistantText
	temporaryAssistant.OutputTokens = outputTokens
	temporaryAssistant.ReasoningTokens = usage.ReasoningTokens
	temporaryAssistant.TokenUsage = outputTokens
	temporaryAssistant.LatencyMS = firstTokenLatencyMS
	temporaryAssistant.Status = "success"
	temporaryAssistant.KnowledgeSources = knowledgeSources
	temporaryAssistant.UpdatedAt = now
	if generateErr != nil {
		traceRecorder.fail(generateErr)
	} else {
		traceRecorder.complete()
	}
	traceRecorder.attachToMessage(&temporaryAssistant)
	result := &SendMessageResult{
		UserMessage: model.Message{
			PublicID:         uuid.NewString(),
			UserID:           input.UserID,
			RunID:            runID,
			Role:             "user",
			ContentType:      "text",
			Content:          lastUser.Content,
			InputTokens:      inputTokens,
			CacheReadTokens:  usage.CacheReadTokens,
			CacheWriteTokens: usage.CacheWriteTokens,
			TokenUsage:       inputTokens,
			Status:           "success",
			CreatedAt:        startedAt,
			UpdatedAt:        now,
		},
		AssistantMessage:    temporaryAssistant,
		Billable:            true,
		UpstreamID:          route.UpstreamID,
		UpstreamName:        route.UpstreamName,
		PlatformModelName:   route.PlatformModelName,
		RoutedBindingCode:   route.BindingCode,
		UpstreamModelName:   route.UpstreamModel,
		UpstreamProtocol:    route.Protocol,
		EffectiveOptions:    filteredOptions,
		UsageSpeed:          usage.Speed,
		UsageServiceTier:    usage.ServiceTier,
		UsageSource:         usageSource,
		RawUsageJSON:        usage.RawUsageJSON,
		CacheWrite5mTokens:  usage.CacheWrite5mTokens,
		CacheWrite1hTokens:  usage.CacheWrite1hTokens,
		ServerSideToolUsage: output.ServerSideToolUsage,
		MCPToolUsage:        generation.MCPToolUsage,
		LLMCallCount:        generation.LLMCallCount,
		LatencyMS:           time.Since(startedAt).Milliseconds(),
		StartedAt:           startedAt,
	}
	if moderationCoord != nil {
		applyBarrierOutcome(result, moderationCoord.AfterGeneration(ctx, assistantText, nil))
	}
	if generateErr != nil {
		return result, wrapTemporaryGenerationError(generateErr)
	}
	return result, nil
}

// wrapTemporaryGenerationError 把上游失败包装为统一的上游错误；工具循环中的预算校验失败
// 不是上游故障，原样返回以保持余额不足的错误语义。
func wrapTemporaryGenerationError(err error) error {
	if errors.Is(err, appbilling.ErrUsageBalanceInsufficient) {
		return err
	}
	return wrapUpstreamRequestError(err)
}

// enforceTemporaryGenerateInput is the final privacy boundary before every
// upstream call, including follow-up calls after tool execution.
func enforceTemporaryGenerateInput(input llm.GenerateInput) llm.GenerateInput {
	input.Ephemeral = true
	input.PreviousResponseID = ""
	input.PromptCacheKey = ""
	input.ResponsesBackground = false
	return input
}

func stripTemporaryChatProviderStateOptions(options map[string]any) map[string]any {
	if len(options) == 0 {
		return nil
	}
	result := cloneModelOptionMap(options)
	for _, key := range []string{
		"cache_control",
		"cache_timeout",
		"cachedContent",
		"cached_content",
		"enable_cache",
		"prompt_cache",
		"prompt_cache_key",
		"prompt_cache_options",
		"prompt_cache_retention",
		"store",
	} {
		delete(result, key)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func stripTemporaryMessageCacheControls(messages []llm.Message) []llm.Message {
	result := cloneLLMMessages(messages)
	for messageIndex := range result {
		result[messageIndex].CacheControl = nil
		if len(result[messageIndex].Parts) == 0 {
			continue
		}
		result[messageIndex].Parts = append([]llm.ContentPart(nil), result[messageIndex].Parts...)
		for partIndex := range result[messageIndex].Parts {
			result[messageIndex].Parts[partIndex].CacheControl = nil
		}
	}
	return result
}

// ValidateTemporaryChatInput 在写入流式响应头及预留计费预算前校验临时上下文。
func ValidateTemporaryChatInput(input TemporaryChatInput) error {
	if input.UserID == 0 || strings.TrimSpace(input.SessionID) == "" || strings.TrimSpace(input.ClientRunID) == "" || strings.TrimSpace(input.Model) == "" {
		return ErrInvalidMessageContent
	}
	if len(input.Messages) == 0 || len(input.Messages) > 100 {
		return ErrInvalidMessageContent
	}
	if len(input.Attachments) > TemporaryChatMaxAttachments {
		return ErrTooManyMessageFiles
	}
	if len(input.KnowledgeBaseIDs) > 8 {
		return ErrInvalidMessageContent
	}
	seenKnowledgeBases := make(map[string]struct{}, len(input.KnowledgeBaseIDs))
	for _, publicID := range input.KnowledgeBaseIDs {
		normalized := strings.TrimSpace(publicID)
		if normalized == "" || len([]rune(normalized)) > 32 {
			return ErrInvalidMessageContent
		}
		if _, exists := seenKnowledgeBases[normalized]; exists {
			return ErrInvalidMessageContent
		}
		seenKnowledgeBases[normalized] = struct{}{}
	}
	attachmentCounts := make(map[int]int)
	for _, attachment := range input.Attachments {
		if attachment.MessageIndex < 0 || attachment.MessageIndex >= len(input.Messages) || attachment.Reader == nil {
			return ErrInvalidFileReference
		}
		attachmentCounts[attachment.MessageIndex]++
	}
	totalChars := 0
	previousRole := ""
	for index, item := range input.Messages {
		role := strings.TrimSpace(item.Role)
		content := strings.TrimSpace(item.Content)
		if (role != "user" && role != "assistant") || role == previousRole {
			return ErrInvalidMessageContent
		}
		if content == "" && (role != "user" || attachmentCounts[index] == 0) {
			return ErrInvalidMessageContent
		}
		totalChars += len([]rune(item.Content))
		if totalChars > temporaryChatMaxContentChars {
			return ErrInvalidMessageContent
		}
		previousRole = role
	}
	if previousRole != "user" {
		return ErrInvalidMessageContent
	}
	return nil
}
