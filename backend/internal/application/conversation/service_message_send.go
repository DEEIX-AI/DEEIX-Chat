package conversation

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	appcompact "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/compact"
	appcm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/contentmoderation"
	apprag "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/rag"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainmemory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/memory"
	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/traceid"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	reasoningContentPassbackSettingKey = "chat.reasoning_content_passback"
	maxRequestRouteAttempts            = 3
)

// SendMessage 发送消息并调用上游渠道对话接口，支持多模态附件。
func (s *Service) SendMessage(ctx context.Context, input SendMessageInput) (result *SendMessageResult, retErr error) {
	return s.sendMessageInternal(ctx, input, nil, false)
}

// StreamMessage 发送消息并按增量回调返回 assistant 文本。
// onDelta 接收流式文本增量；input.OnEvent 接收中间事件（如 rag_search）。
func (s *Service) StreamMessage(
	ctx context.Context,
	input SendMessageInput,
	onDelta func(string) error,
) (result *SendMessageResult, retErr error) {
	input.Cancelable = true
	return s.sendMessageInternal(ctx, input, onDelta, true)
}

func (s *Service) reasoningContentPassbackEnabled(ctx context.Context, userID uint, route *channel.ResolvedRoute) bool {
	if route == nil || !route.ReasoningContentPassback {
		return false
	}
	value, err := s.getUserSettingCached(ctx, userID, reasoningContentPassbackSettingKey)
	return err == nil && value != "false"
}

func messageRouteConfig(route *channel.ResolvedRoute, attributionReferer string, attributionTitle string) llm.RouteConfig {
	return llm.RouteConfig{
		Protocol:            route.Protocol,
		BaseURL:             route.BaseURL,
		APIKey:              route.APIKey,
		HeadersJSON:         route.HeadersJSON,
		ConnectTimeoutMS:    route.ConnectTimeoutMS,
		ReadTimeoutMS:       route.ReadTimeoutMS,
		StreamIdleTimeoutMS: route.StreamIdleTimeoutMS,
		Endpoint:            llm.DefaultEndpointForAdapter(route.Protocol),
		UpstreamModel:       route.UpstreamModel,
		AttributionReferer:  attributionReferer,
		AttributionTitle:    attributionTitle,
	}
}

func canFailoverMessageRoute(attemptCount int, llmRequestCount int, maxLLMCalls int, visibleDeltaCount int, attemptHadSideEffect bool, cause error) bool {
	return cause != nil &&
		attemptCount < maxRequestRouteAttempts &&
		llmRequestCount < maxLLMCalls &&
		visibleDeltaCount == 0 &&
		!attemptHadSideEffect &&
		channel.ShouldFailoverRoute(cause)
}

// emitEvent 统一处理可选事件回调，调用方无需重复判断 nil。
func emitEvent(onEvent func(string, map[string]any) error, eventType string, payload map[string]any) {
	if onEvent == nil {
		return
	}
	_ = onEvent(eventType, payload)
}

func normalizeRAGFallbackReason(status apprag.RetrieveStatus, fallback string) string {
	value := strings.TrimSpace(string(status))
	if value == "" || value == string(apprag.RetrieveStatusHit) {
		return fallback
	}
	return value
}

func processTraceRetrievalStatus(reason string) string {
	switch strings.TrimSpace(reason) {
	case string(apprag.RetrieveStatusLowScore):
		return processTraceStatusLowScore
	case string(apprag.RetrieveStatusEmpty):
		return processTraceStatusEmpty
	default:
		return processTraceStatusIncomplete
	}
}

func processTraceFallbackMode(hasFullText bool) string {
	if hasFullText {
		return processTraceFallbackFullText
	}
	return processTraceFallbackUnavailable
}

const knowledgeBaseNoEvidenceNotice = "An explicitly selected knowledge base returned no sufficiently relevant evidence for this request. Do not claim that the answer is supported by the knowledge base. If you answer from general knowledge, state that limitation clearly."

func ragFileObjectNames(items []model.FileObject) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.FileName)
		if name == "" {
			name = strings.TrimSpace(item.FileID)
		}
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func buildRAGFallbackProcessTracePayload(
	query string,
	fileObjs []model.FileObject,
	result apprag.RetrieveResult,
	reason string,
	hasFullTextFallback bool,
	err error,

) *tracePayload {
	normalizedReason := strings.TrimSpace(textutil.FirstNonEmpty(reason, result.Reason))
	stage := traceStage{Kind: processTraceKindRetrieval, Status: processTraceRetrievalStatus(reason), Fallback: processTraceFallbackMode(hasFullTextFallback), FileCount: len(fileObjs), CandidateCount: result.CandidateCount, FilteredCount: result.FilteredCount, MaxScore: result.MaxScore, Reason: normalizedReason}
	payload := &tracePayload{Query: textutil.CompactSnippet(query, 240), FileNames: ragFileObjectNames(fileObjs), Status: strings.TrimSpace(reason), Reason: strings.TrimSpace(result.Reason), CandidateCount: result.CandidateCount, FilteredCount: result.FilteredCount, MaxScore: result.MaxScore, Stages: []traceStage{stage}}
	if err != nil {
		payload.Error = ragFallbackErrorMessage(result.Status, err)
	}
	return payload
}

func (s *Service) sendMessageInternal(
	ctx context.Context,
	input SendMessageInput,
	onDelta func(string) error,
	preferStream bool,
) (result *SendMessageResult, retErr error) {
	ctx, sendSpan := platformtracing.Start(ctx, "conversation.send",
		trace.WithAttributes(
			attribute.Int64("conversation.id", int64(input.ConversationID)),
			attribute.Int64("user.id", int64(input.UserID)),
			attribute.String("conversation.model", strings.TrimSpace(input.PlatformModelName)),
			attribute.Bool("conversation.stream", preferStream),
			attribute.Int("conversation.file_count", len(input.FileIDs)),
			attribute.Int("conversation.tool_count", len(input.SelectedToolIDs)),
		),
	)
	defer func() {
		platformtracing.RecordError(sendSpan, retErr)
		sendSpan.End()
	}()

	// application 层保留兜底校验，保证非 HTTP 调用路径也遵守同一 MCP 工具数量策略。
	if err := s.ValidateSelectedToolIDs(input.SelectedToolIDs); err != nil {
		return nil, err
	}

	startedAt := time.Now()
	runID := normalizeRunID(input.ClientRunID)
	if runID == "" {
		runID = "run_" + normalizePublicID(uuid.NewString())
	}
	var moderationCoord *appcm.RunCoordinator

	conversation, err := s.repo.GetConversationByUser(ctx, input.ConversationID, input.UserID)
	if err != nil {
		return nil, ErrConversationNotFound
	}

	branchPreparation, err := s.prepareMessageSendBranch(ctx, &input)
	if err != nil {
		retErr = err
		return nil, err
	}
	branchState := branchPreparation.branchState
	normalizedBranchReason := branchPreparation.normalizedBranchReason
	reuseUserMessage := branchPreparation.reuseUserMessage

	currentPlatformModelName := strings.TrimSpace(conversation.Model)
	requestedPlatformModelName := strings.TrimSpace(input.PlatformModelName)
	targetPlatformModelName := currentPlatformModelName
	if requestedPlatformModelName != "" {
		targetPlatformModelName = requestedPlatformModelName
	}
	modelChanged := targetPlatformModelName != "" && targetPlatformModelName != currentPlatformModelName
	if targetPlatformModelName != "" {
		conversation.Model = targetPlatformModelName
		conversation.Provider = inferProvider(targetPlatformModelName)
	}

	var userMessage *model.Message
	var assistantMessage *model.Message
	var traceRecorder *messageTraceRecorder
	var toolCallRows []model.ToolCall
	var persistedToolCallKeys map[string]struct{}
	var totalServerSideToolUsage map[string]int64
	var totalMCPToolUsage []MCPToolUsageItem
	// plan 在路由解析后固化本轮请求形状；在此之前中断时其零值即“尚无有效请求参数”。
	var plan routeGenerationPlan
	runner := &messageGenerationRunner{
		service:      s,
		input:        input,
		runID:        runID,
		startedAt:    startedAt,
		preferStream: preferStream,
		onDelta:      onDelta,
		maxLLMCalls:  s.resolveMaxLLMCallsPerRun(),
		usage:        &messageUsageAccumulator{},
	}
	runState := newMessageSendRunState(s, input, conversation, startedAt, runID)
	run := runState.run
	runState.reuseUserMessage = reuseUserMessage
	runState.bind(&userMessage, &assistantMessage, &traceRecorder, &result, ctx)
	if err = s.claimConversationRun(ctx, run); err != nil {
		retErr = err
		return nil, err
	}
	defer func() {
		if retErr != nil {
			assistantReasoningText := ""
			if traceRecorder != nil {
				assistantReasoningText = traceRecorder.upstreamThinkContent()
			}
			retainedOutput := false
			usageRecovered := false
			if errors.Is(retErr, ErrMessageGenerationCanceled) || llm.RequestWasAccepted(retErr) {
				if usage, ok := s.recoverOpenAIResponsesBackgroundUsage(ctx, runner.routeConfig, runner.responsesBackgroundRecovery); ok {
					usageRecovered = true
					if delta := diffLLMUsage(usage, runner.responsesBackgroundRecovery.ObservedUsage); delta != (llm.Usage{}) {
						runner.usage.addObservedUsage(delta)
					}
				}
			}
			estimatedOutputTokens, estimatedReasoningTokens := runner.usage.interruptedOutputTokens()
			if retained := s.persistInterruptedMessageGeneration(ctx, persistInterruptedMessageGenerationInput{
				SendInput:                input,
				UserMessage:              userMessage,
				AssistantMessage:         assistantMessage,
				AssistantText:            runner.streamedText.String(),
				AssistantReasoningText:   assistantReasoningText,
				EstimatedInputTokens:     runner.usage.interruptedInputTokens(),
				EstimatedOutputTokens:    estimatedOutputTokens,
				EstimatedReasoningTokens: estimatedReasoningTokens,
				UpstreamCallStarted:      runner.upstreamCallStarted,
				Usage:                    runner.usage.usage(),
				UsageRecovered:           usageRecovered,
				LLMCallCount:             runner.completedLLMCallCount,
				AssistantLatency:         time.Since(startedAt).Milliseconds(),
				Error:                    retErr,
				ToolCallRows:             toolCallRows,
				PersistedToolCallKeys:    persistedToolCallKeys,
				TraceRecorder:            traceRecorder,
				Route:                    runState.route,
				EffectiveOptions:         plan.filteredOptions,
				ServerSideToolUsage:      totalServerSideToolUsage,
				MCPToolUsage:             totalMCPToolUsage,
				StartedAt:                startedAt,
				ReuseUserMessage:         reuseUserMessage,
			}); retained != nil {
				result = retained
				retainedOutput = true
				applyRetainedGenerationRunUsage(run, retained, len(toolCallRows), startedAt)
			}
			// Input checks and any retained visible output continue after
			// cancel/interrupt/error; either surface may still block the turn.
			if moderationCoord != nil {
				if result == nil && userMessage != nil && assistantMessage != nil {
					result = &SendMessageResult{
						UserMessage:      *userMessage,
						AssistantMessage: *assistantMessage,
						Billable:         false,
						StartedAt:        startedAt,
					}
				}
				moderationCtx, cancelModeration := background.WithTimeout(ctx, moderationFinalizationTimeout)
				if result != nil && retainedOutput {
					s.completeModerationAfterInterruption(
						moderationCtx,
						moderationCoord,
						result,
						moderationOutputText(runner.streamedText.String(), assistantReasoningText),
					)
				} else {
					s.completeModerationAfterFailure(moderationCtx, moderationCoord, result)
				}
				cancelModeration()
			}
		}
		runState.finalize(ctx, retErr)
		if retErr != nil && result == nil && userMessage != nil && assistantMessage != nil {
			latencyMS := time.Since(startedAt).Milliseconds()
			if latencyMS < 0 {
				latencyMS = 0
			}
			result = &SendMessageResult{
				UserMessage:      *userMessage,
				AssistantMessage: *assistantMessage,
				Billable:         false,
				LatencyMS:        latencyMS,
				StartedAt:        startedAt,
			}
			if failedRoute := runState.route; failedRoute != nil {
				result.UpstreamID = failedRoute.UpstreamID
				result.UpstreamName = failedRoute.UpstreamName
				result.PlatformModelName = failedRoute.PlatformModelName
				result.RoutedBindingCode = failedRoute.BindingCode
				result.UpstreamModelName = failedRoute.UpstreamModel
				result.UpstreamProtocol = failedRoute.Protocol
			}
		}
	}()
	if input.Cancelable {
		cancelCtx, cancel := context.WithCancel(ctx)
		ctx = cancelCtx
		if err = s.generationStreams.register(ctx, runID, input.UserID, conversation.PublicID, cancel); err != nil {
			retErr = err
			return nil, err
		}
		if len(input.FileIDs) > 0 {
			emitEvent(input.OnEvent, "file_proc", map[string]any{"message": "正在处理附件…"})
		}
	}

	resolvedAttachments, err := s.resolveAttachments(ctx, input.UserID, input.FileIDs)
	if err != nil {
		retErr = err
		return nil, err
	}

	pair, err := s.createMessagePair(ctx, input, runID, branchPreparation, resolvedAttachments, nil)
	if err != nil {
		retErr = err
		return nil, err
	}
	userMessage = pair.user
	assistantMessage = pair.assistant
	s.persistInitialConversationFallbackTitle(ctx, *conversation, *userMessage)
	traceRecorder = newMessageTraceRecorder(s, ctx, assistantMessage, input.OnEvent)
	runner.traceRecorder = traceRecorder
	moderationCoord = s.startModerationRun(ctx, input, runID, userMessage, assistantMessage)

	if s.routeResolver == nil || s.llmClient == nil {
		retErr = ErrModelRouteNotConfigured
		return nil, retErr
	}

	routeResolveInput := channel.ResolveRouteInput{
		PlatformModelName: conversation.Model,
		TaskType:          channel.TaskTypeChat,
		Scope:             channel.RouteScopeUser,
		UserID:            input.UserID,
		ConversationID:    input.ConversationID,
		RequestID:         strings.TrimSpace(input.RequestID),
	}
	route, err := s.routeResolver.ResolveRoute(ctx, routeResolveInput)
	if err != nil {
		retErr = mapRouteResolutionError(err)
		return nil, retErr
	}
	runState.route = route
	reasoningContentPassback := s.reasoningContentPassbackEnabled(ctx, input.UserID, route)
	if modelChanged || strings.TrimSpace(conversation.Model) != strings.TrimSpace(route.PlatformModelName) {
		conversation.Model = strings.TrimSpace(route.PlatformModelName)
		conversation.Provider = inferProvider(conversation.Model)
		if err = s.repo.UpdateConversationModel(ctx, input.ConversationID, conversation.Model, conversation.Provider); err != nil {
			retErr = err
			return nil, err
		}
	}
	runState.applyRoute(route)
	if strings.TrimSpace(run.Provider) == "" {
		run.Provider = inferProvider(conversation.Model)
	}

	cfg := s.cfg.Snapshot()
	compactPolicy := s.resolveContextCompactionPolicy(ctx, cfg, input.UserID)

	// 并行预取：Snapshot + UserMemory 提前加载，隐藏 DB 延迟。
	type prefetchData struct {
		snapshot     *model.ContextSnapshot
		userMemories []domainmemory.UserMemory
	}
	prefetchCh := make(chan prefetchData, 1)
	go func() {
		var r prefetchData
		if compactPolicy.EffectiveEnabled() {
			r.snapshot, _ = s.getCachedSnapshot(ctx, input.ConversationID)
		}
		if s.memoryRecorder != nil {
			r.userMemories, _ = s.getCachedUserMemories(ctx, input.UserID)
		}
		prefetchCh <- r
	}()

	// 读取用户的文件处理模式偏好（auto / full_context / rag）。
	fileMode := "auto"
	capability := s.resolveChatFileCapability(ctx)
	if fm, fmErr := s.getUserSettingCached(ctx, input.UserID, "chat.file_mode"); fmErr == nil && fm != "" {
		fileMode = fm
	}

	// 收集并行预取结果，再规划本轮可发送的 PromptScope。
	prefetch := <-prefetchCh
	if err = s.loadMessageBranchContext(
		ctx,
		input.ConversationID,
		branchState,
		prefetch.snapshot,
		normalizedBranchReason,
	); err != nil {
		if s.logger != nil {
			s.logger.Warn("conversation_context_load_failed",
				zap.String("trace_id", traceid.FromContext(ctx)),
				zap.Uint("conversation_id", input.ConversationID),
				zap.String("request_id", strings.TrimSpace(input.RequestID)),
				zap.Error(err),
			)
		}
		retErr = err
		return nil, err
	}

	// 构建完整活跃分支路径。完整消息仅在模型路由与滚动快照已解析后按需加载，
	// 避免默认分支定位和 Prompt 规划分别水合同一批附件与引用。
	contextMessages := filterBlockedMessages(buildBranchMessagePath(branchState, userMessage))
	contextMessages = recoverAssistantRetryUserStates(contextMessages)

	// 软阈值压缩仍可按配置在响应后异步执行；只有当前请求已经越过所选模型的
	// 有效输入预算时，才同步生成滚动快照，避免本轮先被静默截断、下一轮才补摘要。
	preflightCompactInput := appcompact.MaybeCompactConversationInput{
		ConversationID:   input.ConversationID,
		UserID:           input.UserID,
		RunID:            runID,
		Messages:         contextMessages,
		ExistingSnapshot: prefetch.snapshot,
		PromptTokenEstimate: estimatePromptScopeTokens(
			contextMessages,
			prefetch.snapshot,
			compactPolicy,
			reasoningContentPassback,
		),
		ContextModelName: route.UpstreamModel,
		CapabilitiesJSON: route.ModelCapabilitiesJSON,
		PlatformModelName: s.resolveTextTaskModel(ctx, textTaskRouteInput{
			ConfiguredModel:   cfg.CompactTaskModel,
			ConversationModel: conversation.Model,
			UserID:            input.UserID,
			ConversationID:    input.ConversationID,
			RequestID:         input.RequestID,
		}),
		Force: true,
	}
	if compactPolicy.EffectiveEnabled() && s.compactSvc.ContextBudgetExceeded(preflightCompactInput) {
		preflightSnapshot, compactErr := s.compactSvc.MaybeCompactConversation(ctx, preflightCompactInput)
		if compactErr != nil {
			retErr = compactErr
			return nil, compactErr
		}
		if preflightSnapshot != nil {
			prefetch.snapshot = preflightSnapshot
			s.invalidateSnapshotCache(input.ConversationID)
			_ = s.repo.UpdateConversationLastResponseID(ctx, input.ConversationID, "")
			s.persistSnapshotContextArtifact(ctx, snapshotContextArtifactInput{
				ConversationID: input.ConversationID,
				UserID:         input.UserID,
				MessageID:      assistantMessage.ID,
				RunID:          runID,
				Snapshot:       preflightSnapshot,
			})
			if traceRecorder != nil {
				summary, markdown, payload := buildCompactionProcessTrace(preflightSnapshot)
				traceRecorder.appendProcessSection(summary, markdown, payload, messageTraceStatusStreaming)
			}
		}
	}
	promptScope := buildPromptScope(contextMessages, prefetch.snapshot, compactPolicy)
	promptMessages := promptScope.activeMessages()
	ragQuery := buildRAGQuery(promptMessages, input.Content, cfg.RAGQueryHistoryTurns)
	historicalScope := promptScope.historicalMessageScope(input.ConversationID, input.UserID, userMessage.ID)

	// 语义召回必须先限定到当前活跃分支，再由向量存储执行 Top-K，避免 sibling 分支占用名额。
	// 召回仍与附件和 RAG 处理并行，200ms 超时后按原行为优雅跳过。
	var recallCh chan []model.MessageChunk
	if cfg.EmbeddingEnabled && cfg.SemanticContextEnabled && historicalScope.Valid() {
		recallCh = make(chan []model.MessageChunk, 1)
		go func() {
			recallCtx, cancel := context.WithTimeout(ctx, semanticRecallDeadline)
			defer cancel()
			recallCh <- s.recallSemanticContext(recallCtx, historicalScope, input.Content)
		}()
	}

	conversationFileIDs := collectConversationFileIDs(promptMessages, input.FileIDs)
	conversationAttachments, err := s.resolveConversationFileContext(ctx, input.UserID, conversationFileIDs, input.FileIDs)
	if err != nil {
		retErr = err
		return nil, err
	}
	conversationAttachments = bindAttachmentMessageRoles(conversationAttachments, promptMessages)
	conversationAttachments, err = s.hydrateAttachmentsForSend(ctx, input.UserID, conversationAttachments, input.OnEvent)
	if err != nil {
		retErr = err
		return nil, err
	}
	currentAttachments := filterCurrentAttachments(conversationAttachments)
	userMessage.Attachments = marshalAttachmentSnapshots(currentAttachments)

	toolRuntime, err := s.resolveSelectedToolRuntime(ctx, input.SelectedToolIDs)
	if err != nil {
		retErr = err
		return nil, err
	}
	imageAttachmentRoutingActive := toolRuntime.attachmentProcessor != nil
	imageProcessing, err := s.processImageAttachments(ctx, imageAttachmentProcessingInput{
		UserID:         input.UserID,
		ConversationID: input.ConversationID,
		MessageID:      assistantMessage.ID,
		RequestID:      input.RequestID,
		RunID:          runID,
		UserPrompt:     input.Content,
		Attachments:    currentAttachments,
		Runtime:        toolRuntime,
		TraceRecorder:  traceRecorder,
	})
	toolCallRows = append(toolCallRows, imageProcessing.Rows...)
	mergeToolCallPersistenceKeys(&persistedToolCallKeys, imageProcessing.PersistedToolCallKeys)
	totalMCPToolUsage = mergeMCPToolUsage(totalMCPToolUsage, imageProcessing.MCPToolUsage)
	if err != nil {
		retErr = err
		return nil, err
	}
	if imageProcessing.Routed {
		toolRuntime = toolRuntime.withoutAttachmentProcessor()
		if len(toolCallRows) >= s.resolveMaxToolCallsPerRun() {
			toolRuntime = toolRuntime.withoutDefinitions()
		}
	}

	fileContextPlan := buildConversationFileContextPlan(conversationAttachments, fileMode, cfg, route.UpstreamModel, route.ModelCapabilitiesJSON, capability.RAGAvailable)
	if imageProcessing.Routed {
		fileContextPlan = withoutCurrentImageAttachments(fileContextPlan)
	}
	if !cfg.KnowledgeBaseEnabled {
		// 知识库功能已被后台关闭：视同未选择知识库，检索与后续“知识库未命中/不可用”的
		// 判定、提示一并跳过，避免存量引用阻塞发送或注入误导性提示。
		input.KnowledgeBaseIDs = nil
	}
	knowledgeBaseFiles, err := s.resolveKnowledgeBaseRAGFiles(
		ctx,
		input.UserID,
		input.KnowledgeBaseIDs,
		cfg.RAGEnabled && cfg.EmbeddingEnabled && capability.RAGAvailable,
	)
	if err != nil {
		retErr = err
		return nil, err
	}

	contextAssembler := NewContextAssembler(0)
	userCtx := userContextInput{ImageAnalyses: imageProcessing.Analyses}
	var prefixMemories []domainmemory.UserMemory
	preferencePrompt := ""
	if promptScope.Snapshot != nil {
		if snapshotSummary := strings.TrimSpace(promptScope.Snapshot.SummaryText); snapshotSummary != "" {
			userCtx.Snapshot = &snapshotContext{
				Summary:  snapshotSummary,
				FromTurn: promptScope.Snapshot.FromTurn,
				ToTurn:   promptScope.Snapshot.ToTurn,
				Strategy: promptScope.Snapshot.Strategy,
			}
		}
	}
	if len(prefetch.userMemories) > 0 {
		prefMems := filterMemoriesByScope(prefetch.userMemories, "preference")
		if len(prefMems) > 0 {
			prefixMemories = prefMems
			preferencePrompt = buildPreferencePrompt(prefMems, 400)
		}
		otherMems := filterMemoriesByScope(prefetch.userMemories, "profile", "custom")
		if len(otherMems) > 0 {
			userCtx.Memory = s.selectRelevantUserMemories(ctx, input.UserID, input.Content, otherMems, 5)
		}
	}
	processTraceAttachments := attachmentProcessTraceItems(fileContextPlan.Attachments)
	if traceRecorder != nil && shouldShowAttachmentProcessTrace(processTraceAttachments) {
		summary, markdown, payload := buildAttachmentProcessTrace(fileMode, processTraceAttachments)
		traceRecorder.appendProcessSection(summary, markdown, payload, messageTraceStatusStreaming)
	}

	rag, err := s.retrieveMessageRAGContext(ctx, messageRAGRetrievalInput{
		input:              input,
		cfg:                cfg,
		query:              ragQuery,
		fileContextPlan:    fileContextPlan,
		knowledgeBaseFiles: knowledgeBaseFiles,
		contextAssembler:   contextAssembler,
		traceRecorder:      traceRecorder,
	})
	if err != nil {
		retErr = err
		return nil, err
	}
	ragFallbacks := rag.fallbacks
	ragContextChunks := rag.chunks
	userCtx.RAGNotice = rag.notice
	stableFullContextAttachments := append([]AttachmentInput{}, fileContextPlan.FullAttachments...)
	stableFullContextAttachments = append(stableFullContextAttachments, ragFallbackEvidenceAttachments(rag.retrievalFallbacks)...)
	userCtx.Attachments = imageAttachmentsForCurrentUser(stableFullContextAttachments)
	userCtx.RAGChunks = ragContextChunks
	assistantMessage.KnowledgeSources = messageKnowledgeSourcesFromRAGChunks(ragContextChunks)
	// 语义召回注入：收集异步结果（与 RAG 解耦，独立运行）。
	// recallCh 为 nil 时（未启用语义召回或当前分支没有历史消息）直接跳过。
	//
	// 必须阻塞等待（不用 select default），原因：
	//   - 无附件时 hydrateAttachmentsForSend 几乎瞬间返回（~5ms），
	//     非阻塞会在 goroutine 完成前（~50-200ms）直接跳过，导致召回永远触发不了。
	//   - goroutine 持有 200ms context deadline，recallSemanticContext 失败时返回空列表，
	//     因此 <-recallCh 最多阻塞 semanticRecallDeadline（200ms），不会死锁。
	//   - 有附件时 goroutine 早已完成（附件处理 >1s >> 200ms），等待开销为零。
	if recallCh != nil {
		userCtx.RecallChunks = <-recallCh // 阻塞等待，最多 semanticRecallDeadline（200ms）
	}
	userCtx.HistoricalArtifacts = s.recallHistoricalContextArtifacts(ctx, historicalContextRecallInput{
		Scope:              historicalScope,
		HasCurrentSnapshot: promptScope.Snapshot != nil,
		Query:              input.Content,
		CurrentRAGChunks:   ragContextChunks,
		CurrentFallbacks:   ragFallbackEvidenceAttachments(ragFallbacks),
		CurrentRecall:      userCtx.RecallChunks,
	})
	userCtx.CurrentArtifacts = s.persistPromptContextArtifacts(ctx, promptContextArtifactInput{
		ConversationID: input.ConversationID,
		UserID:         input.UserID,
		MessageID:      assistantMessage.ID,
		RunID:          run.RunID,
		Query:          ragQuery,
		RAGChunks:      ragContextChunks,
		RAGFallbacks:   ragFallbacks,
		RecallChunks:   userCtx.RecallChunks,
		Memories:       userCtx.Memory,
	})
	skillPrompts, err := s.resolveSkillPrompts(ctx, input)
	if err != nil {
		retErr = err
		return nil, err
	}
	recordSkillPromptTrace(traceRecorder, skillPrompts)
	routePromptInput := messageRoutePromptInput{
		UserContent:             input.Content,
		ProjectSystemPrompt:     conversation.ProjectSystemPrompt,
		HTMLVisualPromptEnabled: input.HTMLVisualPromptEnabled,
		DomainMessages:          promptScope.activeMessages(),
		StableAttachments:       stableFullContextAttachments,
		DynamicContext:          userCtx,
		PreferencePrompt:        preferencePrompt,
		SkillPrompts:            skillPrompts,
		ToolRuntime:             toolRuntime,
		SkipImageAttachments:    imageAttachmentRoutingActive,
		Config:                  cfg,
	}
	promptPlan, reasoningContentPassback, err := s.planRoutePrompt(ctx, input.UserID, routePromptInput, route)
	if err != nil {
		retErr = err
		return nil, err
	}

	attributionReferer, attributionTitle := s.llmAttribution()
	promptCacheSessionKey := strings.TrimSpace(conversation.SessionKey)
	if promptCacheSessionKey == "" {
		promptCacheSessionKey = strings.TrimSpace(conversation.PublicID)
	}
	gen := routeGenerationContext{
		input:                  input,
		conversation:           conversation,
		cfg:                    cfg,
		tools:                  toolRuntime.definitions,
		promptCacheSessionKey:  promptCacheSessionKey,
		statefulContextConfig:  buildPromptContextConfigSignature(cfg),
		statefulContextState:   buildPromptContextStateSignature(stableFullContextAttachments, prefixMemories),
		normalizedBranchReason: normalizedBranchReason,
		attributionReferer:     attributionReferer,
		attributionTitle:       attributionTitle,
	}
	plan = s.prepareRouteGeneration(ctx, routeGenerationPreparationInput{
		Generation:               gen,
		Route:                    route,
		PromptPlan:               promptPlan,
		ReasoningContentPassback: reasoningContentPassback,
		Mode:                     routeGenerationInitial,
		TraceRecorder:            traceRecorder,
	})
	runner.routeConfig = plan.routeConfig
	if plan.generateInput.ResponsesBackground {
		sendSpan.SetAttributes(attribute.Bool("conversation.responses_background", true))
	}
	if plan.statefulContinuation {
		sendSpan.SetAttributes(
			attribute.Bool("conversation.stateful_response", true),
			attribute.Int("conversation.stateful_full_messages", len(plan.llmMessages)),
			attribute.Int("conversation.stateful_sent_messages", len(plan.generateInput.Messages)),
		)
	} else if strings.TrimSpace(plan.statefulDecision.DisabledReason) != "" {
		sendSpan.SetAttributes(attribute.String("conversation.stateful_disabled_reason", plan.statefulDecision.DisabledReason))
	}
	sendSpan.SetAttributes(promptShapeTraceAttributes("conversation.prompt", plan.promptShape)...)
	// 提示词形状已确定但尚未调用上游：按预估成本抬高预算预留，余额不足在此终止，不产生任何上游费用。
	if err := s.ensureUsageBudgetCoversEstimate(ctx, input.UsageAuthorization, route, plan.filteredOptions, usageBudgetEstimate{
		InputTokens:  plan.estimatedPromptTokens,
		OutputTokens: messageRequestMaxOutputTokens(plan.filteredOptions),
	}); err != nil {
		retErr = err
		return nil, err
	}

	upstreamOutput, err := runner.runRouteAttempt(ctx, &plan, sendSpan)
	if generationCanceled(ctx, err) {
		retErr = ErrMessageGenerationCanceled
		return nil, retErr
	}
	attemptedRouteIDs := []uint{route.RouteID}
	routeFailureRecorded := false
	for canFailoverMessageRoute(len(attemptedRouteIDs), runner.llmRequestCount, runner.maxLLMCalls, runner.visibleDeltaCount, runner.attemptHadSideEffect, err) {
		failedRoute := route
		failedErr := err
		s.routeResolver.MarkRouteFailure(ctx, failedRoute, failedErr)
		routeFailureRecorded = true

		routeResolveInput.ExcludedRouteIDs = append([]uint(nil), attemptedRouteIDs...)
		nextRoute, resolveErr := s.routeResolver.ResolveRoute(ctx, routeResolveInput)
		if resolveErr != nil {
			if s.logger != nil {
				s.logger.Warn("upstream_route_failover_unavailable",
					zap.String("trace_id", traceid.FromContext(ctx)),
					zap.Uint("conversation_id", input.ConversationID),
					zap.Uint("failed_route_id", failedRoute.RouteID),
					zap.Error(resolveErr),
				)
			}
			err = failedErr
			break
		}

		route = nextRoute
		attemptedRouteIDs = append(attemptedRouteIDs, route.RouteID)
		routeFailureRecorded = false
		nextPromptPlan, nextReasoningContentPassback, buildErr := s.planRoutePrompt(ctx, input.UserID, routePromptInput, route)
		if buildErr != nil {
			retErr = buildErr
			return nil, buildErr
		}
		runState.applyRoute(route)
		plan = s.prepareRouteGeneration(ctx, routeGenerationPreparationInput{
			Generation:               gen,
			Route:                    route,
			PromptPlan:               nextPromptPlan,
			ReasoningContentPassback: nextReasoningContentPassback,
			Mode:                     routeGenerationFailover,
			TraceRecorder:            traceRecorder,
		})
		runner.beginRouteFailover(plan.routeConfig)
		sendSpan.SetAttributes(
			attribute.Bool("conversation.route_failover", true),
			attribute.Int("conversation.route_attempt", len(attemptedRouteIDs)),
		)
		if s.logger != nil {
			s.logger.Warn("upstream_route_failover",
				zap.String("trace_id", traceid.FromContext(ctx)),
				zap.Uint("conversation_id", input.ConversationID),
				zap.Uint("failed_route_id", failedRoute.RouteID),
				zap.Uint("next_route_id", route.RouteID),
				zap.Int("attempt", len(attemptedRouteIDs)),
				zap.Error(failedErr),
			)
		}
		upstreamOutput, err = runner.runRouteAttempt(ctx, &plan, sendSpan)
		if generationCanceled(ctx, err) {
			retErr = ErrMessageGenerationCanceled
			return nil, retErr
		}
	}
	if err != nil {
		if !routeFailureRecorded {
			s.routeResolver.MarkRouteFailure(ctx, route, err)
		}
		retErr = wrapUpstreamRequestError(err)
		return nil, retErr
	}
	s.routeResolver.MarkRouteSuccess(ctx, route)

	// 路由已最终确定，后续工具回灌沿用该路由的请求形状。
	routeConfig := plan.routeConfig
	generateInput := plan.generateInput
	llmMessages := plan.llmMessages
	fullLLMMessages := plan.fullLLMMessages
	filteredOptions := plan.filteredOptions
	reasoningContentPassback = plan.reasoningContentPassback

	assistantText := upstreamOutput.Text
	nativeToolRows := upstreamServerToolCallRows(upstreamOutput, runID)
	toolCallRows = append(toolCallRows, nativeToolRows...)
	totalUsage := upstreamOutput.Usage
	if totalUsage == (llm.Usage{}) {
		totalUsage = runner.usage.usage()
	} else {
		runner.usage.setObservedUsage(totalUsage)
	}
	totalServerSideToolUsage = addServerSideToolUsage(nil, upstreamOutput.ServerSideToolUsage)
	remainingToolCalls := max(s.resolveMaxToolCallsPerRun()-len(imageProcessing.Rows), 0)
	llmCallCount := runner.llmRequestCount
	toolLedger := newToolExecutionLedger()
	toolHistoryTrimmedForRun := false
	// 工具回灌的每次上游调用都独立计费：按本条消息已产生的用量加本次调用的预估成本校验预留，
	// 余额不足时在发起调用前终止，已产生的用量走中断结算。
	ensureFollowUpBudget := func(nextInput llm.GenerateInput) error {
		return s.ensureUsageBudgetCoversEstimate(ctx, input.UsageAuthorization, route, filteredOptions, followUpUsageBudgetEstimate(
			runner.usage.billedUsage(),
			estimateBillableInputTokens(nextInput, llmMessages),
			filteredOptions,
		))
	}

	for len(upstreamOutput.ToolCalls) > 0 && llmCallCount < runner.maxLLMCalls && remainingToolCalls > 0 {
		pendingToolCalls := upstreamOutput.ToolCalls
		if len(pendingToolCalls) > remainingToolCalls {
			pendingToolCalls = pendingToolCalls[:remainingToolCalls]
		}
		reasoningContent := ""
		if reasoningContentPassback {
			reasoningContent = outputReasoningContent(upstreamOutput)
		}
		assistantToolMessage := llm.Message{
			Role:             "assistant",
			Content:          assistantText,
			ReasoningContent: reasoningContent,
			ToolCalls:        pendingToolCalls,
		}
		toolResultTokenBudget := resolveToolResultTokenBudget(
			generateInput,
			llmMessages,
			assistantToolMessage,
			route.UpstreamModel,
			route.ModelCapabilitiesJSON,
			cfg.ContextWindowFallbackTokens,
		)
		toolCtx, toolSpan := platformtracing.Start(ctx, "conversation.tool.execute",
			trace.WithAttributes(
				attribute.Int64("conversation.id", int64(input.ConversationID)),
				attribute.Int64("user.id", int64(input.UserID)),
				attribute.Int("conversation.tool.request_count", len(upstreamOutput.ToolCalls)),
				attribute.Int("conversation.tool.remaining_count", remainingToolCalls),
				attribute.Int64("conversation.tool.result_token_budget", toolResultTokenBudget),
			),
		)
		toolResult := s.executeAssistantToolCalls(toolCtx, executeAssistantToolCallsInput{
			UserID:            input.UserID,
			ConversationID:    input.ConversationID,
			MessageID:         assistantMessage.ID,
			RequestID:         input.RequestID,
			RunID:             runID,
			ToolCalls:         pendingToolCalls,
			ToolCallLimit:     remainingToolCalls,
			TraceRecorder:     traceRecorder,
			ToolNameMap:       toolRuntime.nameMap,
			MCPBindings:       toolRuntime.mcpBindings,
			ToolSchemas:       toolRuntime.schemas,
			Ledger:            toolLedger,
			ResultTokenBudget: toolResultTokenBudget,
		})
		toolSpan.SetAttributes(
			attribute.Int("conversation.tool.executed_count", len(toolResult.Rows)),
			attribute.Int("conversation.tool.result_count", len(toolResult.ToolResults)),
		)
		if toolExecutionHasError(toolResult.Rows) {
			toolSpan.SetStatus(codes.Error, "tool execution failed")
		}
		toolSpan.End()
		toolCallRows = append(toolCallRows, toolResult.Rows...)
		mergeToolCallPersistenceKeys(&persistedToolCallKeys, toolResult.PersistedToolCallKeys)
		totalMCPToolUsage = mergeMCPToolUsage(totalMCPToolUsage, toolResult.MCPToolUsage)
		remainingToolCalls -= len(toolResult.Rows)
		if toolResult.FatalErr != nil {
			retErr = wrapUpstreamRequestError(toolResult.FatalErr)
			return nil, retErr
		}
		if len(toolResult.ToolResults) == 0 {
			break
		}
		assistantToolMessage.ToolCalls = toolResult.ExecutedToolCalls
		llmMessages = append(llmMessages,
			assistantToolMessage,
			llm.Message{
				Role:        "tool",
				ToolResults: toolResult.ToolResults,
			},
		)
		var toolHistoryTrimmed bool
		llmMessages, toolHistoryTrimmed = trimToolFollowUpHistory(
			generateInput,
			llmMessages,
			route.UpstreamModel,
			route.ModelCapabilitiesJSON,
			cfg.ContextWindowFallbackTokens,
		)
		if toolHistoryTrimmed {
			toolHistoryTrimmedForRun = true
			sendSpan.SetAttributes(attribute.Bool("conversation.tool.history_trimmed", true))
		}
		var toolResultsRebalanced bool
		llmMessages, toolResultsRebalanced = rebalanceToolFollowUpResults(
			generateInput,
			llmMessages,
			route.UpstreamModel,
			route.ModelCapabilitiesJSON,
			cfg.ContextWindowFallbackTokens,
		)
		if toolResultsRebalanced {
			sendSpan.SetAttributes(attribute.Bool("conversation.tool.results_rebalanced", true))
		}

		followUpInput := generateInput
		if llmCallCount+1 >= runner.maxLLMCalls {
			followUpInput.Messages = buildFinalToolSynthesisMessages(llmMessages, "The maximum number of LLM calls for this run has been reached. Stop calling tools and produce the final answer based on the tool results already available. If the information is insufficient, state the missing information directly.")
			followUpInput.Tools = nil
			followUpInput.DisableTools = true
			followUpInput.PreviousResponseID = ""
			applyOpenAIResponsesInstructions(route, routeConfig.Endpoint, &followUpInput)
		} else if !toolHistoryTrimmed && !toolResultsRebalanced && routeConfig.Endpoint == llm.EndpointResponses && supportsPreviousResponseIDRoute(route) && strings.TrimSpace(upstreamOutput.ResponseID) != "" {
			followUpInput.PreviousResponseID = strings.TrimSpace(upstreamOutput.ResponseID)
			followUpInput.Messages = []llm.Message{{Role: "tool", ToolResults: toolResult.ToolResults}}
		} else {
			followUpInput.Messages = llmMessages
			followUpInput.PreviousResponseID = ""
			applyOpenAIResponsesInstructions(route, routeConfig.Endpoint, &followUpInput)
		}

		if budgetErr := ensureFollowUpBudget(followUpInput); budgetErr != nil {
			retErr = budgetErr
			return nil, retErr
		}
		nextOutput, nextErr := runner.generate(ctx, followUpInput, llmMessages)
		if generationCanceled(ctx, nextErr) {
			retErr = ErrMessageGenerationCanceled
			return nil, retErr
		}
		if nextErr != nil {
			s.routeResolver.MarkRouteFailure(ctx, route, nextErr)
			retErr = wrapUpstreamRequestError(nextErr)
			return nil, retErr
		}
		s.routeResolver.MarkRouteSuccess(ctx, route)
		totalUsage = mergeFollowUpUsage(totalUsage, nextOutput, runner.usage)
		totalServerSideToolUsage = addServerSideToolUsage(totalServerSideToolUsage, nextOutput.ServerSideToolUsage)
		upstreamOutput = nextOutput
		llmCallCount = runner.llmRequestCount
		assistantText = upstreamOutput.Text
		nextNativeToolRows := upstreamServerToolCallRows(upstreamOutput, runID)
		toolCallRows = append(toolCallRows, nextNativeToolRows...)
	}
	if len(upstreamOutput.ToolCalls) > 0 && remainingToolCalls <= 0 && llmCallCount < runner.maxLLMCalls {
		finalInput := generateInput
		finalInput.Messages = buildFinalToolSynthesisMessages(llmMessages, "The maximum number of tool calls for this run has been reached. Stop calling tools and produce the final answer based on the tool results already available. If the information is insufficient, state the missing information directly.")
		finalInput.Tools = nil
		finalInput.DisableTools = true
		finalInput.PreviousResponseID = ""
		applyOpenAIResponsesInstructions(route, routeConfig.Endpoint, &finalInput)
		if budgetErr := ensureFollowUpBudget(finalInput); budgetErr != nil {
			retErr = budgetErr
			return nil, retErr
		}
		nextOutput, nextErr := runner.generate(ctx, finalInput, llmMessages)
		if generationCanceled(ctx, nextErr) {
			retErr = ErrMessageGenerationCanceled
			return nil, retErr
		}
		if nextErr != nil {
			s.routeResolver.MarkRouteFailure(ctx, route, nextErr)
			retErr = wrapUpstreamRequestError(nextErr)
			return nil, retErr
		}
		s.routeResolver.MarkRouteSuccess(ctx, route)
		totalUsage = mergeFollowUpUsage(totalUsage, nextOutput, runner.usage)
		totalServerSideToolUsage = addServerSideToolUsage(totalServerSideToolUsage, nextOutput.ServerSideToolUsage)
		upstreamOutput = nextOutput
		llmCallCount++
		assistantText = upstreamOutput.Text
		nextNativeToolRows := upstreamServerToolCallRows(upstreamOutput, runID)
		toolCallRows = append(toolCallRows, nextNativeToolRows...)
	}

	effectiveInputTokens := runner.usage.effectiveInputTokens(plan.estimatedPromptTokens)
	effectiveOutputTokens, effectiveReasoningTokens := runner.usage.effectiveOutputTokens()

	if toolRunFinalAnswerMissing(upstreamOutput, len(toolCallRows) > 0, llmCallCount, runner.maxLLMCalls, remainingToolCalls) {
		retErr = ErrToolRunFinalAnswerMissing
		return nil, retErr
	}
	if strings.TrimSpace(assistantText) == "" && len(upstreamOutput.GeneratedImages) == 0 {
		retErr = ErrUpstreamEmptyResponse
		return nil, retErr
	}
	finalUsageEvent := totalUsage
	finalUsageEvent.InputTokens = effectiveInputTokens
	finalUsageEvent.OutputTokens = effectiveOutputTokens
	finalUsageEvent.ReasoningTokens = effectiveReasoningTokens
	if err := emitLLMUsageEvent(input.OnEvent, finalUsageEvent); err != nil {
		retErr = err
		return nil, err
	}
	assistantReasoningContent := ""
	if reasoningContentPassback {
		assistantReasoningContent = outputReasoningContent(upstreamOutput)
	}
	statefulPromptFingerprint := buildPromptStateFingerprint(promptStateFingerprintInput{
		Protocol:          route.Protocol,
		Endpoint:          routeConfig.Endpoint,
		UpstreamID:        route.UpstreamID,
		UpstreamModel:     route.UpstreamModel,
		PlatformModelName: conversation.Model,
		ContextConfig:     gen.statefulContextConfig,
		ContextState:      gen.statefulContextState,
		Messages:          buildNextStatefulPrefixMessages(fullLLMMessages, input.Content, assistantText, assistantReasoningContent),
		Tools:             toolRuntime.definitions,
		Options:           filteredOptions,
	})
	responseIDForPersistence := upstreamOutput.ResponseID
	// 历史裁剪后的上游 response 不再代表数据库可重建的完整历史，禁止跨轮复用。
	if toolHistoryTrimmedForRun {
		responseIDForPersistence = ""
		statefulPromptFingerprint = ""
	}

	run.InputTokens = effectiveInputTokens
	run.OutputTokens = effectiveOutputTokens
	run.CacheReadTokens = totalUsage.CacheReadTokens
	run.CacheWriteTokens = totalUsage.CacheWriteTokens
	run.ReasoningTokens = effectiveReasoningTokens
	run.ToolCallsCount = len(toolCallRows)
	run.FirstTokenLatencyMS = runner.firstVisibleDeltaLatencyMS
	if run.FirstTokenLatencyMS == 0 {
		run.FirstTokenLatencyMS = time.Since(startedAt).Milliseconds()
	}
	if run.FirstTokenLatencyMS < 0 {
		run.FirstTokenLatencyMS = 0
	}
	if s.logger != nil {
		fields := []zap.Field{
			zap.String("trace_id", traceid.FromContext(ctx)),
			zap.Uint("conversation_id", input.ConversationID),
			zap.String("protocol", route.Protocol),
			zap.String("upstream_name", route.UpstreamName),
			zap.Int64("input_tokens", totalUsage.InputTokens),
			zap.Int64("cache_read_tokens", totalUsage.CacheReadTokens),
			zap.Int64("cache_write_tokens", totalUsage.CacheWriteTokens),
			zap.Int64("output_tokens", totalUsage.OutputTokens),
			zap.Int("visible_delta_count", runner.visibleDeltaCount),
			zap.Int64("first_visible_delta_latency_ms", runner.firstVisibleDeltaLatencyMS),
		}
		fields = append(fields, promptShapeLogFields(plan.promptShape)...)
		s.logger.Debug("conversation_prompt_shape", fields...)
	}

	assistantLatencyMS := time.Since(startedAt).Milliseconds()
	if assistantLatencyMS < 0 {
		assistantLatencyMS = 0
	}
	persistCtx, persistSpan := platformtracing.Start(ctx, "conversation.persist",
		trace.WithAttributes(
			attribute.Int64("conversation.id", int64(input.ConversationID)),
			attribute.Int64("user.message_id", int64(userMessage.ID)),
			attribute.Int64("assistant.message_id", int64(assistantMessage.ID)),
			attribute.Int("conversation.tool_count", len(toolCallRows)),
		),
	)
	err = s.persistSuccessfulMessageGeneration(persistCtx, persistMessageGenerationInput{
		SendInput:                 input,
		Conversation:              conversation,
		UserMessage:               userMessage,
		AssistantMessage:          assistantMessage,
		AssistantText:             assistantText,
		AssistantReasoningContent: assistantReasoningContent,
		GeneratedImages:           upstreamOutput.GeneratedImages,
		InputTokens:               effectiveInputTokens,
		CacheReadTokens:           totalUsage.CacheReadTokens,
		CacheWriteTokens:          totalUsage.CacheWriteTokens,
		OutputTokens:              effectiveOutputTokens,
		ReasoningTokens:           effectiveReasoningTokens,
		AssistantLatency:          assistantLatencyMS,
		ResponseID:                responseIDForPersistence,
		StatefulPromptFingerprint: statefulPromptFingerprint,
		ToolCallRows:              toolCallRows,
		PersistedToolCallKeys:     persistedToolCallKeys,
		Route:                     runState.route,
		ReuseUserMessage:          reuseUserMessage,
		SkipEmbed:                 moderationCoord != nil,
	})
	platformtracing.RecordError(persistSpan, err)
	persistSpan.End()
	if err != nil {
		retErr = err
		return nil, err
	}

	compactMessages := append([]model.Message(nil), contextMessages...)
	compactMessages[len(compactMessages)-1] = *userMessage
	compactMessages = append(compactMessages, *assistantMessage)
	compactCfg := s.cfg.Snapshot()
	compactPolicy = s.resolveContextCompactionPolicy(ctx, compactCfg, input.UserID)
	compactInput := appcompact.MaybeCompactConversationInput{
		ConversationID:      input.ConversationID,
		UserID:              input.UserID,
		RunID:               runID,
		Messages:            compactMessages,
		ExistingSnapshot:    prefetch.snapshot,
		PromptTokenEstimate: plan.fullContextPromptTokens + effectiveOutputTokens,
		ContextModelName:    route.UpstreamModel,
		CapabilitiesJSON:    route.ModelCapabilitiesJSON,
	}
	var postBillingCompaction *postBillingCompactionTask
	if !compactPolicy.EffectiveEnabled() || !s.compactSvc.ShouldCompactConversation(compactInput) {
		// 用户已关闭自动压缩，仅完成 trace 记录
		if traceRecorder != nil {
			traceRecorder.complete()
			traceRecorder.attachToMessage(assistantMessage)
		}
	} else {
		compactPlatformModelName := s.resolveTextTaskModel(ctx, textTaskRouteInput{
			ConfiguredModel:   compactCfg.CompactTaskModel,
			ConversationModel: conversation.Model,
			UserID:            input.UserID,
			ConversationID:    input.ConversationID,
			RequestID:         input.RequestID,
		})
		compactInput.PlatformModelName = compactPlatformModelName
		postBillingCompaction = &postBillingCompactionTask{
			Async:          compactCfg.CompactAsyncEnabled,
			Input:          compactInput,
			ConversationID: input.ConversationID,
			UserID:         input.UserID,
			MessageID:      assistantMessage.ID,
			RunID:          runID,
			PreserveTurns:  compactCfg.ContextCompactPreserve,
			OnEvent:        input.OnEvent,
			TraceRecorder:  traceRecorder,
		}
		if compactCfg.CompactAsyncEnabled && traceRecorder != nil {
			summary, payload := buildPendingCompactionProcessTrace()
			traceRecorder.setCompactionProcessStage(summary, "", payload)
			traceRecorder.completeForBackgroundContinuation()
			traceRecorder.attachToMessage(assistantMessage)
			postBillingCompaction.OnEvent = nil
		}
	}

	// 流式路径：trace 已由 traceRecorder.attachToMessage 从内存填充；
	// 新消息 feedback 必为 0，两次 DB 读无意义，跳过以消除 completed 事件前的最后阻塞。
	if !preferStream {
		feedbackMessages := []model.Message{*userMessage, *assistantMessage}
		if err = s.hydrateMessageFeedback(ctx, input.UserID, feedbackMessages); err == nil {
			_ = s.hydrateMessageProcessTraces(ctx, feedbackMessages)
			*userMessage = feedbackMessages[0]
			*assistantMessage = feedbackMessages[1]
		}
	}

	result = &SendMessageResult{
		UserMessage:           *userMessage,
		AssistantMessage:      *assistantMessage,
		MetadataRefreshHint:   s.resolveConversationMetadataRefreshHint(ctx, *conversation, *userMessage),
		Billable:              true,
		UpstreamID:            run.UpstreamID,
		UpstreamName:          run.UpstreamName,
		PlatformModelName:     route.PlatformModelName,
		RoutedBindingCode:     route.BindingCode,
		UpstreamModelName:     route.UpstreamModel,
		UpstreamProtocol:      route.Protocol,
		EffectiveOptions:      filteredOptions,
		UsageSpeed:            totalUsage.Speed,
		UsageServiceTier:      totalUsage.ServiceTier,
		RawUsageJSON:          totalUsage.RawUsageJSON,
		CacheWrite5mTokens:    totalUsage.CacheWrite5mTokens,
		CacheWrite1hTokens:    totalUsage.CacheWrite1hTokens,
		ServerSideToolUsage:   totalServerSideToolUsage,
		MCPToolUsage:          totalMCPToolUsage,
		LLMCallCount:          runner.completedLLMCallCount,
		LatencyMS:             time.Since(startedAt).Milliseconds(),
		StartedAt:             startedAt,
		postBillingCompaction: postBillingCompaction,
	}
	// Soft moderation barrier: show checking, then block or pass.
	if moderationCoord != nil {
		outputImages := s.loadOutputImagesForModeration(ctx, moderationCoord, input.UserID, assistantMessage.Attachments)
		s.completeModerationAfterSuccess(ctx, completeModerationAfterSuccessInput{
			Coordinator:      moderationCoord,
			Result:           result,
			OutputText:       moderationOutputText(assistantText, assistantReasoningContent, traceRecorder.upstreamThinkContent()),
			OutputImages:     outputImages,
			EmbedInput:       input,
			ReuseUserMessage: reuseUserMessage,
		})
	}
	return result, nil
}

func messageKnowledgeSourcesFromRAGChunks(chunks []model.RAGChunk) []model.MessageKnowledgeSource {
	if len(chunks) == 0 {
		return nil
	}
	sources := make([]model.MessageKnowledgeSource, 0, len(chunks))
	for _, chunk := range chunks {
		sources = append(sources, model.MessageKnowledgeSource{
			FileName:   strings.TrimSpace(chunk.FileName),
			FileID:     strings.TrimSpace(chunk.FileID),
			ChunkIndex: chunk.ChunkIndex,
			Score:      chunk.Score,
			Preview:    textutil.CompactSnippet(chunk.Content, 100),
		})
	}
	return sources
}
