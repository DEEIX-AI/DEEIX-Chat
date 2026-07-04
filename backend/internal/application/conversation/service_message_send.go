package conversation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	appcompact "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/compact"
	apprag "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/rag"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainmemory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/traceid"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
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
	ctx = context.WithoutCancel(ctx)
	return s.sendMessageInternal(ctx, input, onDelta, true)
}

// emitEvent 统一处理可选事件回调，调用方无需重复判断 nil。
func emitEvent(onEvent func(string, map[string]interface{}) error, eventType string, payload map[string]interface{}) {
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
) map[string]interface{} {
	stage := map[string]interface{}{
		"kind":            processTraceKindRetrieval,
		"status":          processTraceRetrievalStatus(reason),
		"fallback":        processTraceFallbackMode(hasFullTextFallback),
		"file_count":      len(fileObjs),
		"candidate_count": result.CandidateCount,
		"filtered_count":  result.FilteredCount,
		"max_score":       result.MaxScore,
	}
	if normalizedReason := strings.TrimSpace(firstNonEmptyString(reason, result.Reason)); normalizedReason != "" {
		stage["reason"] = normalizedReason
	}
	payload := map[string]interface{}{
		"query":                  compactSnippet(query, 240),
		"file_names":             ragFileObjectNames(fileObjs),
		"status":                 strings.TrimSpace(reason),
		"reason":                 strings.TrimSpace(result.Reason),
		"candidate_count":        result.CandidateCount,
		"filtered_count":         result.FilteredCount,
		"max_score":              result.MaxScore,
		processTracePayloadStage: stage,
	}
	if err != nil {
		payload["error"] = err.Error()
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

	maxFiles := s.cfg.Snapshot().MaxMessageFiles
	if maxFiles <= 0 {
		maxFiles = 10
	}
	// application 层保留兜底校验，保证非 HTTP 调用路径也遵守同一 MCP 工具数量策略。
	if err := s.ValidateSelectedToolIDs(input.SelectedToolIDs); err != nil {
		return nil, err
	}

	startedAt := time.Now()
	runID := normalizeRunID(input.ClientRunID)
	if runID == "" {
		runID = "run_" + normalizePublicID(uuid.NewString())
	}

	conversation, err := s.repo.GetConversationByUser(ctx, input.ConversationID, input.UserID)
	if err != nil {
		return nil, ErrConversationNotFound
	}

	normalizedBranchReason := normalizeBranchReason(input.BranchReason)
	branchState, err := s.resolveMessageBranch(ctx, input.ConversationID, input.UserID, input.ParentMessagePublicID, input.SourceMessagePublicID, normalizedBranchReason)
	if err != nil {
		retErr = err
		return nil, err
	}
	reuseUserMessage := branchState.ReuseUserMessage != nil
	if reuseUserMessage {
		input.Content = branchState.ReuseUserMessage.Content
		input.FileIDs = parseAttachmentSnapshotFileIDs(branchState.ReuseUserMessage.Attachments)
	}
	if len(input.FileIDs) > maxFiles {
		return nil, ErrTooManyMessageFiles
	}
	if input.Cancelable {
		cancelCtx, cancel := context.WithCancel(ctx)
		ctx = cancelCtx
		s.generationStreams.register(ctx, runID, input.UserID, cancel)
	}

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
	var streamedText strings.Builder
	var streamUsageTotal llm.Usage
	var toolCallRows []model.ToolCall
	var persistedToolCallKeys map[string]struct{}
	var resolvedRoute *channel.ResolvedRoute
	var filteredOptions map[string]interface{}
	var totalServerSideToolUsage map[string]int64
	estimatedInputTokens := int64(0)
	upstreamCallStarted := false
	runState := newMessageSendRunState(s, input, conversation, startedAt, runID)
	run := runState.run
	runState.reuseUserMessage = reuseUserMessage
	runState.bind(&userMessage, &assistantMessage, &traceRecorder, &result, ctx)
	defer func() {
		if retErr != nil {
			if retained := s.persistInterruptedMessageGeneration(ctx, persistInterruptedMessageGenerationInput{
				SendInput:             input,
				UserMessage:           userMessage,
				AssistantMessage:      assistantMessage,
				AssistantText:         streamedText.String(),
				EstimatedInputTokens:  estimatedInputTokens,
				UpstreamCallStarted:   upstreamCallStarted,
				Usage:                 streamUsageTotal,
				AssistantLatency:      time.Since(startedAt).Milliseconds(),
				Error:                 retErr,
				ToolCallRows:          toolCallRows,
				PersistedToolCallKeys: persistedToolCallKeys,
				TraceRecorder:         traceRecorder,
				Route:                 resolvedRoute,
				EffectiveOptions:      filteredOptions,
				ServerSideToolUsage:   totalServerSideToolUsage,
				StartedAt:             startedAt,
				ReuseUserMessage:      reuseUserMessage,
			}); retained != nil {
				result = retained
				applyRetainedGenerationRunUsage(run, retained, len(toolCallRows), startedAt)
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
			if resolvedRoute != nil {
				result.UpstreamID = resolvedRoute.UpstreamID
				result.UpstreamName = resolvedRoute.UpstreamName
				result.PlatformModelName = resolvedRoute.PlatformModelName
				result.RoutedBindingCode = resolvedRoute.BindingCode
				result.UpstreamModelName = resolvedRoute.UpstreamModel
				result.UpstreamProtocol = resolvedRoute.Protocol
			}
		}
	}()

	resolvedAttachments, err := s.resolveAttachments(ctx, input.UserID, input.FileIDs)
	if err != nil {
		retErr = err
		return nil, err
	}

	estimatedInputTokens = estimateTokens(input.Content)
	assistantMessage = &model.Message{
		ConversationID:   input.ConversationID,
		UserID:           input.UserID,
		PublicID:         normalizePublicID(uuid.NewString()),
		RunID:            runID,
		Role:             "assistant",
		ContentType:      "text",
		Content:          "",
		BranchReason:     normalizedBranchReason,
		TokenUsage:       0,
		InputTokens:      0,
		OutputTokens:     0,
		CacheReadTokens:  0,
		CacheWriteTokens: 0,
		ReasoningTokens:  0,
		LatencyMS:        0,
		Status:           "pending",
		ErrorCode:        "",
		ErrorMessage:     "",
		Attachments:      "[]",
	}
	if reuseUserMessage {
		reused := *branchState.ReuseUserMessage
		userMessage = &reused
		assistantMessage.ParentMessageID = &userMessage.ID
		assistantMessage.SourceMessageID = branchState.SourceMessageID
		if err = s.repo.CreateAssistantBranchMessage(ctx, assistantMessage); err != nil {
			retErr = err
			return nil, err
		}
		assistantMessage.ParentPublicID = userMessage.PublicID
		assistantMessage.SourcePublicID = branchState.SourcePublicID
	} else {
		attachmentsJSON := []byte(marshalAttachmentSnapshots(resolvedAttachments))
		userMessage = &model.Message{
			ConversationID:   input.ConversationID,
			UserID:           input.UserID,
			PublicID:         normalizePublicID(uuid.NewString()),
			ParentMessageID:  branchState.ParentMessageID,
			RunID:            runID,
			Role:             "user",
			ContentType:      fallbackContentType(input.ContentType),
			Content:          input.Content,
			BranchReason:     normalizedBranchReason,
			SourceMessageID:  branchState.SourceMessageID,
			TokenUsage:       estimatedInputTokens,
			InputTokens:      estimatedInputTokens,
			OutputTokens:     0,
			CacheReadTokens:  0,
			CacheWriteTokens: 0,
			ReasoningTokens:  0,
			LatencyMS:        0,
			Status:           "pending",
			ErrorCode:        "",
			ErrorMessage:     "",
			Attachments:      string(attachmentsJSON),
		}
		attachmentRows := make([]model.Attachment, 0, len(resolvedAttachments))
		now := time.Now()
		for _, item := range resolvedAttachments {
			attachmentRows = append(attachmentRows, model.Attachment{
				ConversationID: input.ConversationID,
				UserID:         input.UserID,
				FileID:         strings.TrimSpace(item.FileID),
				Kind:           normalizeAttachmentKind(item.Kind, item.MimeType),
				FileName:       strings.TrimSpace(item.FileName),
				MimeType:       strings.TrimSpace(item.MimeType),
				FileSize:       item.FileSize,
				SHA256:         strings.TrimSpace(item.SHA256),
				StoragePath:    strings.TrimSpace(item.StoragePath),
				Status:         "active",
				MetaJSON:       strings.TrimSpace(item.MetaJSON),
				UploadedAt:     now,
			})
		}

		// 用户消息、助手占位、用户附件与消息计数必须一起提交，避免失败时留下半个回合。
		if err = s.repo.CreateMessagePairWithUserAttachments(ctx, userMessage, assistantMessage, attachmentRows); err != nil {
			retErr = err
			return nil, err
		}
		userMessage.ParentPublicID = branchState.ParentPublicID
		userMessage.SourcePublicID = branchState.SourcePublicID
		assistantMessage.ParentPublicID = userMessage.PublicID
	}
	traceRecorder = newMessageTraceRecorder(s, ctx, assistantMessage, input.OnEvent)

	if s.routeResolver == nil || s.llmClient == nil {
		retErr = ErrModelRouteNotConfigured
		return nil, retErr
	}

	route, err := s.routeResolver.ResolveRoute(ctx, channel.ResolveRouteInput{
		PlatformModelName: conversation.Model,
		TaskType:          channel.TaskTypeChat,
		Scope:             channel.RouteScopeUser,
		UserID:            input.UserID,
		ConversationID:    input.ConversationID,
		RequestID:         strings.TrimSpace(input.RequestID),
	})
	if err != nil {
		if errors.Is(err, channel.ErrModelAccessDenied) {
			retErr = ErrModelAccessDenied
			return nil, retErr
		}
		if errors.Is(err, channel.ErrRouteNotFound) || errors.Is(err, channel.ErrModelNotFound) {
			retErr = ErrModelRouteNotConfigured
			return nil, retErr
		}
		if errors.Is(err, channel.ErrAllRoutesUnavailable) {
			retErr = wrapUpstreamRequestError(err)
			return nil, retErr
		}
		retErr = err
		return nil, err
	}
	resolvedRoute = route
	if modelChanged || strings.TrimSpace(conversation.Model) != strings.TrimSpace(route.PlatformModelName) {
		conversation.Model = strings.TrimSpace(route.PlatformModelName)
		conversation.Provider = inferProvider(conversation.Model)
		if err = s.repo.UpdateConversationModel(ctx, input.ConversationID, conversation.Model, conversation.Provider); err != nil {
			retErr = err
			return nil, err
		}
	}
	if !reuseUserMessage {
		s.maybeGenerateConversationMetadataAsync(*conversation, *userMessage)
	}
	run.Endpoint = llm.DefaultEndpointForAdapter(route.Protocol)
	run.ProviderProtocol = route.Protocol
	run.UpstreamID = route.UpstreamID
	run.UpstreamModelID = route.UpstreamModelID
	run.UpstreamName = route.UpstreamName
	run.PlatformModelName = route.PlatformModelName
	run.RoutedBindingCode = route.BindingCode
	run.ModelVendor = route.ModelVendor
	run.ModelIcon = route.ModelIcon
	run.UpstreamModelName = route.UpstreamModel
	if strings.TrimSpace(run.Provider) == "" {
		run.Provider = inferProvider(conversation.Model)
	}

	// 构建完整活跃分支路径；压缩裁剪先于模型预算截断，避免摘要和全量历史重复发送。
	contextMessages := buildBranchMessagePath(branchState, userMessage)
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

	// 异步语义召回：200ms 截止时限，不阻塞 LLM 关键路径。
	// 超时后优雅跳过；召回依赖 Embedding 服务。
	// 召回结果稍后作为用户上下文 XML 注入，避免把历史片段提升为 system 指令。
	var recallCh chan []model.MessageChunk
	if cfg.EmbeddingEnabled && cfg.SemanticContextEnabled {
		recallCh = make(chan []model.MessageChunk, 1)
		go func() {
			recallCtx, cancel := context.WithTimeout(ctx, semanticRecallDeadline)
			defer cancel()
			recallCh <- s.recallSemanticContext(recallCtx, input.ConversationID, input.UserID, input.Content)
		}()
	}

	// 读取用户的文件处理模式偏好（auto / full_context / rag）。
	fileMode := "auto"
	capability := s.resolveChatFileCapability(ctx)
	if fm, fmErr := s.getUserSettingCached(ctx, input.UserID, "chat.file_mode"); fmErr == nil && fm != "" {
		fileMode = fm
	}

	// 收集并行预取结果，再规划本轮可发送的 PromptScope。
	prefetch := <-prefetchCh
	contextMessages = s.expandContextMessagesToSnapshotBoundary(ctx, input.ConversationID, userMessage.ID, contextMessages, prefetch.snapshot, compactPolicy)
	promptScope := buildPromptScope(contextMessages, prefetch.snapshot, compactPolicy)
	promptMessages := s.applyContextTokenBudget(promptScope.activeMessages(), route.UpstreamModel, route.ModelCapabilitiesJSON)
	ragQuery := buildRAGQuery(promptMessages, input.Content, cfg.RAGQueryHistoryTurns)

	conversationFileIDs := collectConversationFileIDs(promptMessages, input.FileIDs)
	conversationAttachments, err := s.resolveConversationFileContext(ctx, input.UserID, conversationFileIDs, input.FileIDs)
	if err != nil {
		retErr = err
		return nil, err
	}
	conversationAttachments, err = s.hydrateAttachmentsForSend(ctx, input.UserID, conversationAttachments, input.OnEvent)
	if err != nil {
		retErr = err
		return nil, err
	}
	currentAttachments := filterCurrentAttachments(conversationAttachments)
	userMessage.Attachments = marshalAttachmentSnapshots(currentAttachments)

	fileContextPlan := buildConversationFileContextPlan(conversationAttachments, fileMode, cfg, route.UpstreamModel, route.ModelCapabilitiesJSON, capability.RAGAvailable)

	// 构建历史消息序列（不含系统注入）
	historyMsgs := historyMessagesFromDomain(promptMessages)
	if len(historyMsgs) == 0 {
		historyMsgs = append(historyMsgs, llm.Message{
			Role:    "user",
			Content: input.Content,
		})
	}

	// ContextAssembler 只承载真正的系统级行为指令；资料型上下文稍后进入用户 XML。
	assembler := NewContextAssembler(int64(cfg.ContextMaxInputTokens))
	systemPrompt := resolveMessageSystemPromptInjection(cfg, route, conversation.ProjectSystemPrompt, input.HTMLVisualPromptEnabled, input.HTMLVisualColorMode)
	if systemPrompt.Content != "" {
		if systemPrompt.InlineToUser {
			historyMsgs = inlineSystemPromptIntoLatestUserMessage(historyMsgs, systemPrompt.Content)
		} else {
			assembler.Add(ContextSlot{Kind: SlotSystemPrompt, Content: systemPrompt.Content, Required: true})
		}
	}
	userCtx := userContextInput{}
	var prefixMemories []domainmemory.UserMemory
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
			if prefContent := buildPreferencePrompt(prefMems, 400); prefContent != "" {
				assembler.Add(ContextSlot{Kind: SlotPreference, Content: prefContent})
			}
		}
		otherMems := filterMemoriesByScope(prefetch.userMemories, "profile", "custom")
		if len(otherMems) > 0 {
			userCtx.Memory = s.selectRelevantUserMemories(ctx, input.UserID, input.Content, otherMems, 5)
		}
	}
	llmMessages, _ := assembler.Assemble(historyMsgs)
	if traceRecorder != nil && shouldShowAttachmentProcessTrace(fileContextPlan.Attachments) {
		summary, markdown, payload := buildAttachmentProcessTrace(fileMode, fileContextPlan.Attachments)
		traceRecorder.appendProcessSection(summary, markdown, payload, messageTraceStatusStreaming)
	}

	ragFallbacks := ragFallbackEvidencesFromAttachments(
		filterAttachmentsByContextMode(fileContextPlan.FullAttachments, fileContextModeRAGFallback),
		"rag_unavailable",
		"",
	)
	retrievalRAGFallbacks := make([]ragFallbackEvidence, 0)
	ragContextChunks := make([]model.RAGChunk, 0)
	if cfg.RAGEnabled && len(fileContextPlan.RAGAttachments) > 0 {
		readyObjs := fileContextPlanRAGObjects(fileContextPlan.RAGAttachments)
		emitEvent(input.OnEvent, "rag_search", map[string]interface{}{
			"message": "正在检索相关内容…",
		})
		ragCtx, ragSpan := platformtracing.Start(ctx, "conversation.rag.retrieve",
			trace.WithAttributes(
				attribute.Int64("conversation.id", int64(input.ConversationID)),
				attribute.Int64("user.id", int64(input.UserID)),
				attribute.Int("conversation.rag.file_count", len(readyObjs)),
			),
		)
		ragCallCtx := ragCtx
		ragCancel := func() {}
		if cfg.RAGWaitReadyMS > 0 {
			ragCallCtx, ragCancel = context.WithTimeout(ragCtx, time.Duration(cfg.RAGWaitReadyMS)*time.Millisecond)
		}
		ragResult, ragErr := s.ragSvc.RetrieveWithStatus(ragCallCtx, apprag.RetrieveInput{
			UserID:   input.UserID,
			Query:    ragQuery,
			FileObjs: readyObjs,
		})
		ragCancel()
		platformtracing.RecordError(ragSpan, ragErr)
		ragSpan.SetAttributes(
			attribute.String("conversation.rag.status", string(ragResult.Status)),
			attribute.String("conversation.rag.reason", strings.TrimSpace(ragResult.Reason)),
			attribute.Int("conversation.rag.candidate_count", ragResult.CandidateCount),
			attribute.Int("conversation.rag.filtered_count", ragResult.FilteredCount),
			attribute.Float64("conversation.rag.max_score", float64(ragResult.MaxScore)),
			attribute.Bool("conversation.rag.cached", ragResult.Cached),
		)
		ragSpan.End()
		ragChunksRaw := ragResult.Chunks
		ragChunks := assembler.DeduplicateRAGChunks(ragChunksRaw)
		if ragErr != nil {
			s.logger.Warn("rag_retrieval_failed",
				zap.String("trace_id", traceid.FromContext(ctx)),
				zap.Uint("user_id", input.UserID),
				zap.Error(ragErr),
			)
			fallbacks, skipped := splitRetrievalFallbackAttachments(fileContextPlan.RAGAttachments, cfg)
			fallbackLabel := "已改用全文"
			if len(fallbacks) == 0 {
				fallbackLabel = "没有可用全文"
			}
			if traceRecorder != nil {
				traceRecorder.appendProcessSection(
					"内容检索未完成，"+fallbackLabel,
					formatTraceStep(
						"内容检索",
						fmt.Sprintf("文件已检索，检索未完成，%s。", fallbackLabel),
					),
					buildRAGFallbackProcessTracePayload(ragQuery, readyObjs, ragResult, normalizeRAGFallbackReason(ragResult.Status, "rag_error"), len(fallbacks) > 0, ragErr),
					messageTraceStatusStreaming,
				)
			}
			fallbackReason := normalizeRAGFallbackReason(ragResult.Status, "rag_error")
			evidences := ragFallbackEvidencesFromAttachments(fallbacks, fallbackReason, strings.TrimSpace(ragErr.Error()))
			ragFallbacks = append(ragFallbacks, evidences...)
			retrievalRAGFallbacks = append(retrievalRAGFallbacks, evidences...)
			appendRAGFallbackSkippedTrace(traceRecorder, skipped, fallbackReason)
		} else if len(ragChunks) == 0 {
			fallbacks, skipped := splitRetrievalFallbackAttachments(fileContextPlan.RAGAttachments, cfg)
			fallbackLabel := "已改用全文"
			if len(fallbacks) == 0 {
				fallbackLabel = "没有可用全文"
			}
			ragStatus := normalizeRAGFallbackReason(ragResult.Status, "rag_empty")
			missLabel := "未检索到相关片段"
			if ragResult.Status == apprag.RetrieveStatusLowScore {
				missLabel = "检索结果低于相似度阈值"
			}
			if traceRecorder != nil {
				traceRecorder.appendProcessSection(
					"未检索到相关片段，"+fallbackLabel,
					formatTraceStep("内容检索", fmt.Sprintf("文件已检索，%s，%s。", missLabel, fallbackLabel)),
					buildRAGFallbackProcessTracePayload(ragQuery, readyObjs, ragResult, ragStatus, len(fallbacks) > 0, nil),
					messageTraceStatusStreaming,
				)
			}
			evidences := ragFallbackEvidencesFromAttachments(fallbacks, ragStatus, "")
			ragFallbacks = append(ragFallbacks, evidences...)
			retrievalRAGFallbacks = append(retrievalRAGFallbacks, evidences...)
			appendRAGFallbackSkippedTrace(traceRecorder, skipped, ragStatus)
		} else {
			if traceRecorder != nil {
				summary, markdown, payload := buildRAGProcessTrace(ragQuery, readyObjs, ragChunks)
				traceRecorder.appendProcessSection(summary, markdown, payload, messageTraceStatusStreaming)
			}
			ragContextChunks = append(ragContextChunks, ragChunks...)
		}
	}
	stableFullContextAttachments := append([]AttachmentInput{}, fileContextPlan.FullAttachments...)
	stableFullContextAttachments = append(stableFullContextAttachments, ragFallbackEvidenceAttachments(retrievalRAGFallbacks)...)
	userCtx.Attachments = imageAttachmentsForCurrentUser(stableFullContextAttachments)
	userCtx.RAGChunks = ragContextChunks
	// 语义召回注入：收集异步结果（与 RAG 解耦，独立运行）。
	// recallCh 为 nil 时（SemanticContextEnabled=false）直接跳过。
	//
	// 必须阻塞等待（不用 select default），原因：
	//   - 无附件时 hydrateAttachmentsForSend 几乎瞬间返回（~5ms），
	//     非阻塞会在 goroutine 完成前（~50-200ms）直接跳过，导致召回永远触发不了。
	//   - goroutine 持有 200ms context deadline，recallSemanticContext 失败时返回空列表，
	//     因此 <-recallCh 最多阻塞 semanticRecallDeadline（200ms），不会死锁。
	//   - 有附件时 goroutine 早已完成（附件处理 >1s >> 200ms），等待开销为零。
	if recallCh != nil {
		recalled := <-recallCh // 阻塞等待，最多 semanticRecallDeadline（200ms）
		userCtx.RecallChunks = promptScope.filterRecallChunks(recalled)
	}
	userCtx.HistoricalArtifacts = s.recallHistoricalContextArtifacts(
		ctx,
		input.ConversationID,
		userMessage.ID,
		promptScope.Snapshot != nil,
		promptScope.CoveredUntilID,
		promptScope.retainedMessageIDSet(),
		input.Content,
		ragContextChunks,
		ragFallbackEvidenceAttachments(ragFallbacks),
		userCtx.RecallChunks,
	)
	userCtx.CurrentArtifacts = s.persistPromptContextArtifacts(ctx, promptContextArtifactInput{
		ConversationID: input.ConversationID,
		UserID:         input.UserID,
		MessageID:      userMessage.ID,
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
	if traceRecorder != nil && skillPrompts != nil {
		skillTitles := skillPromptTitles(skillPrompts.Skills)
		traceRecorder.appendProcessSection(
			fmt.Sprintf("已提供 %d 个 Skill 上下文", len(skillPrompts.Skills)),
			formatTraceStep("Skill", fmt.Sprintf("本轮已加载 Skill：%s。包含 SKILL.md 内容，相关时使用。", strings.Join(skillTitles, "、"))),
			map[string]interface{}{
				processTracePayloadStage: map[string]interface{}{
					"kind":   "skill_context",
					"status": messageTraceStatusStreaming,
				},
				"skill_count":    len(skillPrompts.Skills),
				"skill_ids":      skillPromptIDs(skillPrompts.Skills),
				"skill_titles":   skillTitles,
				"skill_triggers": skillPromptTriggers(skillPrompts.Skills),
			},
			messageTraceStatusStreaming,
		)
	}
	toolRuntime := s.resolveSelectedToolRuntime(ctx, input.SelectedToolIDs)
	promptPlan := buildPromptPlan(ctx, promptPlanInput{
		BaseMessages:      llmMessages,
		StableAttachments: stableFullContextAttachments,
		DynamicContext:    userCtx,
		SkillPrompts:      skillPrompts,
		ToolRuntime:       toolRuntime,
		Config:            cfg,
		StoreProvider:     s.storeProvider,
	})
	llmMessages = promptPlan.Messages
	estimatedPromptTokens := estimatePromptTokens(llmMessages)

	attributionReferer, attributionTitle := s.llmAttribution()
	routeConfig := llm.RouteConfig{
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
	filteredOptions = filterModelOptions(input.Options, route.Protocol, modelOptionPolicyConfig{
		Mode:                  cfg.ModelOptionPolicyMode,
		AllowedPathsJSON:      cfg.ModelOptionAllowedPaths,
		DeniedPathsJSON:       cfg.ModelOptionDeniedPaths,
		ModelCapabilitiesJSON: route.ModelCapabilitiesJSON,
	})
	generateInput := llm.GenerateInput{
		RequestID:      strings.TrimSpace(input.RequestID),
		ConversationID: input.ConversationID,
		Messages:       llmMessages,
		Tools:          toolRuntime.definitions,
		Options:        filteredOptions,
	}
	fullLLMMessages := llmMessages
	applyOpenAIResponsesInstructions(route, routeConfig.Endpoint, &generateInput)
	statefulContextConfig := buildPromptContextConfigSignature(cfg)
	statefulContextState := buildPromptContextStateSignature(stableFullContextAttachments, prefixMemories)
	statefulPrefixFingerprint := buildPromptStateFingerprint(promptStateFingerprintInput{
		Protocol:          route.Protocol,
		Endpoint:          routeConfig.Endpoint,
		UpstreamID:        route.UpstreamID,
		UpstreamModel:     route.UpstreamModel,
		PlatformModelName: conversation.Model,
		ContextConfig:     statefulContextConfig,
		ContextState:      statefulContextState,
		Messages:          promptStatePrefixMessages(llmMessages),
		Tools:             toolRuntime.definitions,
		Options:           filteredOptions,
	})
	statefulDecision := resolveStatefulPreviousResponseID(
		route,
		normalizedBranchReason,
		conversation.LastResponseID,
		conversation.LastPromptFingerprint,
		statefulPrefixFingerprint,
	)
	if routeConfig.Endpoint == llm.EndpointResponses && statefulDecision.PreviousResponseID != "" {
		statefulMessages := buildStatefulResponseMessages(llmMessages)
		if len(statefulMessages) > 0 && len(statefulMessages) < len(llmMessages) {
			generateInput.Messages = statefulMessages
			generateInput.PreviousResponseID = statefulDecision.PreviousResponseID
			estimatedPromptTokens = estimatePromptTokens(statefulMessages)
			sendSpan.SetAttributes(
				attribute.Bool("conversation.stateful_response", true),
				attribute.Int("conversation.stateful_full_messages", len(llmMessages)),
				attribute.Int("conversation.stateful_sent_messages", len(statefulMessages)),
			)
		}
	} else if strings.TrimSpace(statefulDecision.DisabledReason) != "" {
		sendSpan.SetAttributes(attribute.String("conversation.stateful_disabled_reason", statefulDecision.DisabledReason))
	}
	promptMode := "full"
	if strings.TrimSpace(generateInput.PreviousResponseID) != "" {
		promptMode = "stateful"
	}
	initialPromptShape := summarizePromptShape(promptMode, generateInput.Messages, fullLLMMessages, generateInput.PreviousResponseID)
	if traceRecorder != nil {
		traceRecorder.recordPromptTrace(buildMessagePromptTrace(messagePromptTraceInput{
			Plan:               promptPlan.Trace,
			Mode:               promptMode,
			PromptFingerprint:  statefulPrefixFingerprint,
			StatefulDecision:   statefulDecision,
			SentMessages:       generateInput.Messages,
			FullMessages:       fullLLMMessages,
			PreviousResponseID: generateInput.PreviousResponseID,
		}))
	}
	sendSpan.SetAttributes(promptShapeTraceAttributes("conversation.prompt", initialPromptShape)...)

	firstVisibleDeltaLatencyMS := int64(0)
	visibleDeltaCount := 0
	emitVisibleDelta := func(delta string) error {
		if delta == "" {
			return nil
		}
		visibleDeltaCount++
		if firstVisibleDeltaLatencyMS == 0 {
			firstVisibleDeltaLatencyMS = time.Since(startedAt).Milliseconds()
			if firstVisibleDeltaLatencyMS < 0 {
				firstVisibleDeltaLatencyMS = 0
			}
		}
		if traceRecorder != nil {
			traceRecorder.completeProcess()
			traceRecorder.completeUpstreamThink()
		}
		if err := onDelta(delta); err != nil {
			return err
		}
		streamedText.WriteString(delta)
		return nil
	}
	runGenerate := func(currentInput llm.GenerateInput) (*llm.GenerateOutput, error) {
		callPromptMode := "full"
		if strings.TrimSpace(currentInput.PreviousResponseID) != "" {
			callPromptMode = "stateful"
		}
		streamRequested := preferStream && onDelta != nil
		streamSupported := llm.SupportsStreamingAdapter(routeConfig.Protocol)
		var callVisibleText strings.Builder
		emitCallVisibleDelta := func(delta string) error {
			if err := emitVisibleDelta(delta); err != nil {
				return err
			}
			callVisibleText.WriteString(delta)
			return nil
		}
		callPromptShape := summarizePromptShape(callPromptMode, currentInput.Messages, currentInput.Messages, currentInput.PreviousResponseID)
		generationCtx, generationSpan := platformtracing.Start(ctx, "conversation.llm.generate",
			trace.WithAttributes(append([]attribute.KeyValue{
				attribute.Int64("conversation.id", int64(input.ConversationID)),
				attribute.String("llm.model", routeConfig.UpstreamModel),
				attribute.String("llm.protocol", routeConfig.Protocol),
				attribute.String("llm.endpoint", routeConfig.Endpoint),
				attribute.Bool("llm.stream", streamRequested && streamSupported),
				attribute.Bool("llm.tools_disabled", currentInput.DisableTools),
				attribute.Int("llm.message_count", len(currentInput.Messages)),
				attribute.Int("llm.tool_count", len(currentInput.Tools)),
			}, promptShapeTraceAttributes("llm.prompt", callPromptShape)...)...),
		)
		var generateErr error
		defer func() {
			platformtracing.RecordError(generationSpan, generateErr)
			generationSpan.End()
		}()

		emitNonStreamingOutput := func(output *llm.GenerateOutput) error {
			if output == nil || (strings.TrimSpace(output.Text) == "" && output.Reasoning == nil) {
				return nil
			}
			cleanText, thinkText := splitAssistantOutputThinkingContent(output.Text)
			if traceRecorder != nil && output.Reasoning != nil {
				traceRecorder.syncStructuredThink(
					output.Reasoning.Text,
					output.Reasoning.Summary,
					reasoningPayload(&llm.ReasoningDelta{
						EventType:        "response.completed",
						ItemID:           output.Reasoning.ItemID,
						Status:           output.Reasoning.Status,
						Kind:             messageTraceThinkKindContent,
						EncryptedContent: output.Reasoning.EncryptedContent,
					}),
				)
			} else if traceRecorder != nil && strings.TrimSpace(thinkText) != "" {
				traceRecorder.syncStructuredThink(thinkText, "", nil)
			}
			if traceRecorder != nil {
				traceRecorder.completeUpstreamThink()
			}
			if cleanText == "" && strings.TrimSpace(thinkText) == "" {
				cleanText = strings.TrimSpace(output.Text)
			}
			if streamErr := emitCallVisibleDelta(cleanText); streamErr != nil {
				return streamErr
			}
			output.Text = cleanText
			return nil
		}

		if !streamRequested || !streamSupported {
			upstreamCallStarted = true
			output, err := s.llmClient.Generate(generationCtx, routeConfig, currentInput)
			generateErr = err
			if err == nil && streamRequested {
				generateErr = emitNonStreamingOutput(output)
				if generateErr != nil {
					return output, generateErr
				}
			}
			return output, err
		}
		thinkingRouter := &thinkingDeltaRouter{}
		callStreamUsage := llm.Usage{}
		upstreamCallStarted = true
		output, streamErr := s.llmClient.GenerateStream(generationCtx, routeConfig, currentInput, func(event llm.GenerateStreamEvent) error {
			if s.isMessageGenerationCanceled(generationCtx, runID) {
				return ErrMessageGenerationCanceled
			}
			if event.Usage != (llm.Usage{}) && input.OnEvent != nil {
				// 上游流式 usage 通常是“本次 LLM 调用累计值”，但一条消息可能包含多轮 LLM 调用。
				// 这里先换算成本次调用内增量，再累加成本轮消息总量，保证实时展示和最终账单口径一致。
				usageDelta := diffLLMUsage(event.Usage, callStreamUsage)
				callStreamUsage = event.Usage
				streamUsageTotal = addLLMUsage(streamUsageTotal, usageDelta)
				if err := emitLLMUsageEvent(input.OnEvent, streamUsageTotal); err != nil {
					return err
				}
			}
			if traceRecorder != nil && event.Reasoning != nil && event.Reasoning.Text != "" {
				traceRecorder.appendUpstreamReasoning(event.Reasoning.Kind, event.Reasoning.Text, reasoningPayload(event.Reasoning))
				if strings.EqualFold(strings.TrimSpace(event.Reasoning.Status), "completed") {
					traceRecorder.completeUpstreamThink()
				}
			}
			if traceRecorder != nil && event.ServerToolCall != nil {
				toolStatus := normalizeStreamServerToolStatus(event.ServerToolCall.Status)
				summary, markdown, payload := buildToolTrace([]model.ToolCall{{
					RunID:      runID,
					ToolCallID: strings.TrimSpace(event.ServerToolCall.ToolCallID),
					ToolType:   strings.TrimSpace(event.ServerToolCall.ToolType),
					ToolName:   strings.TrimSpace(event.ServerToolCall.ToolName),
					Status:     toolStatus,
					InputJSON:  strings.TrimSpace(event.ServerToolCall.ArgumentsJSON),
					OutputJSON: strings.TrimSpace(event.ServerToolCall.OutputJSON),
					ErrorJSON:  strings.TrimSpace(event.ServerToolCall.ErrorJSON),
				}})
				traceRecorder.syncToolSection(summary, markdown, payload, traceStatusFromToolStatus(toolStatus))
			}
			if onDelta == nil || event.Delta == "" {
				return nil
			}
			visibleDelta, thinkDelta := thinkingRouter.consume(event.Delta)
			if traceRecorder != nil && thinkDelta != "" {
				traceRecorder.appendUpstreamReasoning(messageTraceThinkKindContent, thinkDelta, nil)
			}
			if visibleDelta == "" {
				return nil
			}
			return emitCallVisibleDelta(visibleDelta)
		})
		generateErr = streamErr
		if generateErr == nil {
			visibleTail, thinkTail := thinkingRouter.flush()
			if traceRecorder != nil && thinkTail != "" {
				traceRecorder.appendUpstreamReasoning(messageTraceThinkKindContent, thinkTail, nil)
			}
			if traceRecorder != nil && output != nil && output.Reasoning != nil {
				traceRecorder.syncStructuredThink(
					output.Reasoning.Text,
					output.Reasoning.Summary,
					reasoningPayload(&llm.ReasoningDelta{
						EventType:        "response.completed",
						ItemID:           output.Reasoning.ItemID,
						Status:           output.Reasoning.Status,
						Kind:             messageTraceThinkKindContent,
						EncryptedContent: output.Reasoning.EncryptedContent,
					}),
				)
			}
			if traceRecorder != nil {
				traceRecorder.completeUpstreamThink()
			}
			if visibleTail != "" {
				if tailErr := emitCallVisibleDelta(visibleTail); tailErr != nil {
					generateErr = tailErr
				}
			}
			if output != nil {
				output.Text = callVisibleText.String()
			}
		}
		if generateErr != nil && shouldFallbackToNonStreaming(generateErr) {
			output, generateErr = s.llmClient.Generate(generationCtx, routeConfig, currentInput)
			if generateErr == nil {
				generateErr = emitNonStreamingOutput(output)
			}
		}
		return output, generateErr
	}

	handleCanceledGeneration := func(generateErr error) bool {
		if generateErr == nil || (ctx.Err() == nil && !isMessageGenerationCanceledError(generateErr)) {
			return false
		}
		retErr = ErrMessageGenerationCanceled
		return true
	}

	var upstreamOutput *llm.GenerateOutput
	upstreamOutput, err = runGenerate(generateInput)
	if handleCanceledGeneration(err) {
		return nil, retErr
	}
	if err != nil && strings.TrimSpace(generateInput.PreviousResponseID) != "" &&
		strings.TrimSpace(streamedText.String()) == "" &&
		shouldRetryWithoutPreviousResponseID(err) {
		if s.logger != nil {
			s.logger.Warn("previous_response_id_rejected_retry_full_context",
				zap.String("trace_id", traceid.FromContext(ctx)),
				zap.Uint("conversation_id", input.ConversationID),
				zap.String("protocol", route.Protocol),
				zap.String("upstream_name", route.UpstreamName),
				zap.Error(err),
			)
		}
		_ = s.repo.UpdateConversationLastResponseID(ctx, input.ConversationID, "")
		generateInput.PreviousResponseID = ""
		generateInput.Messages = fullLLMMessages
		applyOpenAIResponsesInstructions(route, routeConfig.Endpoint, &generateInput)
		estimatedPromptTokens = estimatePromptTokens(fullLLMMessages)
		initialPromptShape = summarizePromptShape("full_retry", generateInput.Messages, fullLLMMessages, "")
		if traceRecorder != nil {
			traceRecorder.recordPromptTrace(buildMessagePromptTrace(messagePromptTraceInput{
				Plan:              promptPlan.Trace,
				Mode:              "full_retry",
				PromptFingerprint: statefulPrefixFingerprint,
				StatefulDecision: statefulResponseDecision{
					DisabledReason: "previous_response_rejected",
				},
				SentMessages: generateInput.Messages,
				FullMessages: fullLLMMessages,
			}))
		}
		sendSpan.SetAttributes(promptShapeTraceAttributes("conversation.prompt_retry", initialPromptShape)...)
		streamedText.Reset()
		upstreamOutput, err = runGenerate(generateInput)
		if handleCanceledGeneration(err) {
			return nil, retErr
		}
	}
	if err != nil {
		s.routeResolver.MarkRouteFailure(ctx, route, err)
		retErr = wrapUpstreamRequestError(err)
		return nil, retErr
	}
	s.routeResolver.MarkRouteSuccess(ctx, route)

	toolCallRows = make([]model.ToolCall, 0)
	assistantText, nativeToolRows := syncUpstreamOutputTrace(traceRecorder, upstreamOutput, runID)
	toolCallRows = append(toolCallRows, nativeToolRows...)
	totalUsage := upstreamOutput.Usage
	if totalUsage == (llm.Usage{}) {
		totalUsage = streamUsageTotal
	} else {
		streamUsageTotal = totalUsage
	}
	totalServerSideToolUsage = addServerSideToolUsage(nil, upstreamOutput.ServerSideToolUsage)
	remainingToolCalls := s.resolveMaxToolCallsPerRun()
	maxLLMCalls := s.resolveMaxLLMCallsPerRun()
	if maxLLMCalls <= 0 {
		maxLLMCalls = 1
	}
	llmCallCount := 1
	toolLedger := newToolExecutionLedger()

	for len(upstreamOutput.ToolCalls) > 0 && llmCallCount < maxLLMCalls && remainingToolCalls > 0 {
		toolCtx, toolSpan := platformtracing.Start(ctx, "conversation.tool.execute",
			trace.WithAttributes(
				attribute.Int64("conversation.id", int64(input.ConversationID)),
				attribute.Int64("user.id", int64(input.UserID)),
				attribute.Int("conversation.tool.request_count", len(upstreamOutput.ToolCalls)),
				attribute.Int("conversation.tool.remaining_count", remainingToolCalls),
			),
		)
		toolResult := s.executeAssistantToolCalls(toolCtx, executeAssistantToolCallsInput{
			UserID:         input.UserID,
			ConversationID: input.ConversationID,
			MessageID:      assistantMessage.ID,
			RequestID:      input.RequestID,
			RunID:          runID,
			ToolCalls:      upstreamOutput.ToolCalls,
			ToolCallLimit:  remainingToolCalls,
			TraceRecorder:  traceRecorder,
			ToolNameMap:    toolRuntime.nameMap,
			MCPConfigs:     toolRuntime.mcpConfigs,
			ToolSchemas:    toolRuntime.schemas,
			Ledger:         toolLedger,
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
		remainingToolCalls -= len(toolResult.Rows)
		if toolResult.FatalErr != nil {
			retErr = wrapUpstreamRequestError(toolResult.FatalErr)
			return nil, retErr
		}
		if len(toolResult.ToolResults) == 0 {
			break
		}
		reasoningContent := ""
		if route.ReasoningContentPassback {
			reasoningContent = outputReasoningContent(upstreamOutput)
		}
		llmMessages = append(llmMessages,
			llm.Message{
				Role:             "assistant",
				Content:          assistantText,
				ReasoningContent: reasoningContent,
				ToolCalls:        toolResult.ExecutedToolCalls,
			},
			llm.Message{
				Role:        "tool",
				ToolResults: toolResult.ToolResults,
			},
		)

		followUpInput := generateInput
		if llmCallCount+1 >= maxLLMCalls {
			followUpInput.Messages = buildFinalToolSynthesisMessages(llmMessages, "The maximum number of LLM calls for this run has been reached. Stop calling tools and produce the final answer based on the tool results already available. If the information is insufficient, state the missing information directly.")
			followUpInput.Tools = nil
			followUpInput.DisableTools = true
			followUpInput.PreviousResponseID = ""
			applyOpenAIResponsesInstructions(route, routeConfig.Endpoint, &followUpInput)
		} else if routeConfig.Endpoint == llm.EndpointResponses && supportsPreviousResponseIDRoute(route) && strings.TrimSpace(upstreamOutput.ResponseID) != "" {
			followUpInput.PreviousResponseID = strings.TrimSpace(upstreamOutput.ResponseID)
			followUpInput.Messages = []llm.Message{{Role: "tool", ToolResults: toolResult.ToolResults}}
		} else {
			followUpInput.Messages = llmMessages
			followUpInput.PreviousResponseID = ""
			applyOpenAIResponsesInstructions(route, routeConfig.Endpoint, &followUpInput)
		}

		nextOutput, nextErr := runGenerate(followUpInput)
		if handleCanceledGeneration(nextErr) {
			return nil, retErr
		}
		if nextErr != nil {
			s.routeResolver.MarkRouteFailure(ctx, route, nextErr)
			retErr = wrapUpstreamRequestError(nextErr)
			return nil, retErr
		}
		s.routeResolver.MarkRouteSuccess(ctx, route)
		totalUsage = addLLMUsage(totalUsage, nextOutput.Usage)
		if nextOutput.Usage != (llm.Usage{}) {
			streamUsageTotal = totalUsage
		} else if streamUsageTotal != (llm.Usage{}) {
			totalUsage = streamUsageTotal
		}
		totalServerSideToolUsage = addServerSideToolUsage(totalServerSideToolUsage, nextOutput.ServerSideToolUsage)
		upstreamOutput = nextOutput
		llmCallCount++
		var nextNativeToolRows []model.ToolCall
		assistantText, nextNativeToolRows = syncUpstreamOutputTrace(traceRecorder, upstreamOutput, runID)
		toolCallRows = append(toolCallRows, nextNativeToolRows...)
	}
	if len(upstreamOutput.ToolCalls) > 0 && remainingToolCalls <= 0 && llmCallCount < maxLLMCalls {
		finalInput := generateInput
		finalInput.Messages = buildFinalToolSynthesisMessages(llmMessages, "The maximum number of tool calls for this run has been reached. Stop calling tools and produce the final answer based on the tool results already available. If the information is insufficient, state the missing information directly.")
		finalInput.Tools = nil
		finalInput.DisableTools = true
		finalInput.PreviousResponseID = ""
		applyOpenAIResponsesInstructions(route, routeConfig.Endpoint, &finalInput)
		nextOutput, nextErr := runGenerate(finalInput)
		if handleCanceledGeneration(nextErr) {
			return nil, retErr
		}
		if nextErr != nil {
			s.routeResolver.MarkRouteFailure(ctx, route, nextErr)
			retErr = wrapUpstreamRequestError(nextErr)
			return nil, retErr
		}
		s.routeResolver.MarkRouteSuccess(ctx, route)
		totalUsage = addLLMUsage(totalUsage, nextOutput.Usage)
		if nextOutput.Usage != (llm.Usage{}) {
			streamUsageTotal = totalUsage
		} else if streamUsageTotal != (llm.Usage{}) {
			totalUsage = streamUsageTotal
		}
		totalServerSideToolUsage = addServerSideToolUsage(totalServerSideToolUsage, nextOutput.ServerSideToolUsage)
		upstreamOutput = nextOutput
		llmCallCount++
		var nextNativeToolRows []model.ToolCall
		assistantText, nextNativeToolRows = syncUpstreamOutputTrace(traceRecorder, upstreamOutput, runID)
		toolCallRows = append(toolCallRows, nextNativeToolRows...)
	}

	effectiveInputTokens := totalUsage.InputTokens
	if effectiveInputTokens <= 0 {
		effectiveInputTokens = estimatedPromptTokens
	}
	effectiveOutputTokens := totalUsage.OutputTokens
	if effectiveOutputTokens <= 0 {
		effectiveOutputTokens = estimateTokens(assistantText)
	}

	if toolRunFinalAnswerMissing(upstreamOutput, len(toolCallRows) > 0, llmCallCount, maxLLMCalls, remainingToolCalls) {
		retErr = ErrToolRunFinalAnswerMissing
		return nil, retErr
	}
	if strings.TrimSpace(assistantText) == "" {
		retErr = ErrUpstreamEmptyResponse
		return nil, retErr
	}
	finalUsageEvent := totalUsage
	finalUsageEvent.InputTokens = effectiveInputTokens
	finalUsageEvent.OutputTokens = effectiveOutputTokens
	if err := emitLLMUsageEvent(input.OnEvent, finalUsageEvent); err != nil {
		retErr = err
		return nil, err
	}
	statefulPromptFingerprint := buildPromptStateFingerprint(promptStateFingerprintInput{
		Protocol:          route.Protocol,
		Endpoint:          routeConfig.Endpoint,
		UpstreamID:        route.UpstreamID,
		UpstreamModel:     route.UpstreamModel,
		PlatformModelName: conversation.Model,
		ContextConfig:     statefulContextConfig,
		ContextState:      statefulContextState,
		Messages:          buildNextStatefulPrefixMessages(fullLLMMessages, input.Content, assistantText),
		Tools:             toolRuntime.definitions,
		Options:           filteredOptions,
	})

	run.InputTokens = effectiveInputTokens
	run.OutputTokens = effectiveOutputTokens
	run.CacheReadTokens = totalUsage.CacheReadTokens
	run.CacheWriteTokens = totalUsage.CacheWriteTokens
	run.ReasoningTokens = totalUsage.ReasoningTokens
	run.ToolCallsCount = len(toolCallRows)
	run.FirstTokenLatencyMS = firstVisibleDeltaLatencyMS
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
			zap.Int("visible_delta_count", visibleDeltaCount),
			zap.Int64("first_visible_delta_latency_ms", firstVisibleDeltaLatencyMS),
		}
		fields = append(fields, promptShapeLogFields(initialPromptShape)...)
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
		InputTokens:               effectiveInputTokens,
		CacheReadTokens:           totalUsage.CacheReadTokens,
		CacheWriteTokens:          totalUsage.CacheWriteTokens,
		OutputTokens:              effectiveOutputTokens,
		ReasoningTokens:           totalUsage.ReasoningTokens,
		AssistantLatency:          assistantLatencyMS,
		ResponseID:                upstreamOutput.ResponseID,
		StatefulPromptFingerprint: statefulPromptFingerprint,
		ToolCallRows:              toolCallRows,
		PersistedToolCallKeys:     persistedToolCallKeys,
		ReuseUserMessage:          reuseUserMessage,
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
		PromptTokenEstimate: estimatedPromptTokens,
	}
	if !compactPolicy.EffectiveEnabled() {
		// 用户已关闭自动压缩，仅完成 trace 记录
		if traceRecorder != nil {
			traceRecorder.complete()
			traceRecorder.attachToMessage(assistantMessage)
		}
	} else if compactCfg.CompactAsyncEnabled {
		// 异步压缩：移出响应关键路径，不阻塞流式返回
		compactPlatformModelName := s.resolveTextTaskModel(ctx, compactCfg.CompactTaskModel, conversation.Model, input.UserID, input.ConversationID, strings.TrimSpace(input.RequestID))
		compactInput.PlatformModelName = compactPlatformModelName
		go func() {
			asyncCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			asyncCtx = withBasicServiceBillingContext(asyncCtx, input.UserID, input.ConversationID)
			if compactSnapshot, compactErr := s.compactSvc.MaybeCompactConversation(asyncCtx, compactInput); compactErr == nil && compactSnapshot != nil {
				// 压缩后清空 LastResponseID：Responses API 有状态会话链已失效，需重传
				s.invalidateSnapshotCache(input.ConversationID) // 新 Snapshot 已生成，失效缓存
				_ = s.repo.UpdateConversationLastResponseID(asyncCtx, input.ConversationID, "")
				s.persistSnapshotContextArtifact(asyncCtx, snapshotContextArtifactInput{
					ConversationID: input.ConversationID,
					UserID:         input.UserID,
					MessageID:      assistantMessage.ID,
					RunID:          runID,
					Snapshot:       compactSnapshot,
				})
			}
		}()
		if traceRecorder != nil {
			traceRecorder.complete()
			traceRecorder.attachToMessage(assistantMessage)
		}
	} else {
		compactPlatformModelName := s.resolveTextTaskModel(ctx, compactCfg.CompactTaskModel, conversation.Model, input.UserID, input.ConversationID, strings.TrimSpace(input.RequestID))
		compactInput.PlatformModelName = compactPlatformModelName
		compactCtx := withBasicServiceBillingContext(ctx, input.UserID, input.ConversationID)
		if snapshot, snapshotErr := s.compactSvc.MaybeCompactConversation(compactCtx, compactInput); snapshotErr == nil && snapshot != nil {
			// 压缩后清空 LastResponseID：Responses API 有状态会话链已失效，需重传
			s.invalidateSnapshotCache(input.ConversationID) // 新 Snapshot 已生成，失效缓存
			_ = s.repo.UpdateConversationLastResponseID(compactCtx, input.ConversationID, "")
			s.persistSnapshotContextArtifact(compactCtx, snapshotContextArtifactInput{
				ConversationID: input.ConversationID,
				UserID:         input.UserID,
				MessageID:      assistantMessage.ID,
				RunID:          runID,
				Snapshot:       snapshot,
			})
			if traceRecorder != nil {
				summary, markdown, payload := buildCompactionProcessTrace(snapshot)
				traceRecorder.appendProcessSection(summary, markdown, payload, messageTraceStatusStreaming)
			}
			// 通知前端压缩完成（同步路径仍在 SSE 流中，可发送事件）
			previewLen := len([]rune(snapshot.SummaryText))
			if previewLen > 80 {
				previewLen = 80
			}
			emitEvent(input.OnEvent, "compact_done", map[string]interface{}{
				"method":          snapshot.Strategy,
				"freed_tokens":    snapshot.SourceTokens - snapshot.SummaryTokens,
				"kept_turns":      compactCfg.ContextCompactPreserve,
				"summary_preview": string([]rune(snapshot.SummaryText)[:previewLen]),
			})
		}
		if traceRecorder != nil {
			traceRecorder.complete()
			traceRecorder.attachToMessage(assistantMessage)
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

	return &SendMessageResult{
		UserMessage:         *userMessage,
		AssistantMessage:    *assistantMessage,
		MetadataRefreshHint: conversationMetadataRefreshHint(*conversation, *userMessage),
		Billable:            true,
		UpstreamID:          run.UpstreamID,
		UpstreamName:        run.UpstreamName,
		PlatformModelName:   route.PlatformModelName,
		RoutedBindingCode:   route.BindingCode,
		UpstreamModelName:   route.UpstreamModel,
		UpstreamProtocol:    route.Protocol,
		EffectiveOptions:    filteredOptions,
		UsageSpeed:          totalUsage.Speed,
		UsageServiceTier:    totalUsage.ServiceTier,
		RawUsageJSON:        totalUsage.RawUsageJSON,
		CacheWrite5mTokens:  totalUsage.CacheWrite5mTokens,
		CacheWrite1hTokens:  totalUsage.CacheWrite1hTokens,
		ServerSideToolUsage: totalServerSideToolUsage,
		LatencyMS:           time.Since(startedAt).Milliseconds(),
		StartedAt:           startedAt,
	}, nil
}
