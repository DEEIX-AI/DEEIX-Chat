package conversation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/tokenestimate"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// embedMessagePair 异步将用户和助手消息向量化并存入 chat_message_chunks。
func (s *Service) embedMessagePair(ctx context.Context, conversationID uint, userID uint, userMsg *model.Message, assistantMsg *model.Message) {
	if s.embeddingSvc == nil {
		return
	}
	chunks := make([]model.MessageChunk, 0, 2)
	texts := make([]string, 0, 2)
	if userMsg != nil && strings.TrimSpace(userMsg.Content) != "" {
		chunks = append(chunks, model.MessageChunk{
			ConversationID: conversationID,
			MessageID:      userMsg.ID,
			UserID:         userID,
			Role:           "user",
			ChunkIndex:     0,
			Content:        userMsg.Content,
			TokenCount:     int(tokenestimate.Estimate(userMsg.Content)),
		})
		texts = append(texts, userMsg.Content)
	}
	if assistantMsg != nil && strings.TrimSpace(assistantMsg.Content) != "" {
		chunks = append(chunks, model.MessageChunk{
			ConversationID: conversationID,
			MessageID:      assistantMsg.ID,
			UserID:         userID,
			Role:           "assistant",
			ChunkIndex:     0,
			Content:        assistantMsg.Content,
			TokenCount:     int(tokenestimate.Estimate(assistantMsg.Content)),
		})
		texts = append(texts, assistantMsg.Content)
	}
	if len(chunks) == 0 {
		return
	}
	embeddings, embeddingSignature, err := s.embeddingSvc.EmbedTextsWithSignature(ctx, texts)
	if err != nil {
		s.logger.Warn("embed_message_pair_failed", zap.Error(err))
		return
	}
	if len(embeddings) != len(chunks) {
		s.logger.Warn("embed_message_pair_length_mismatch",
			zap.Int("chunks", len(chunks)),
			zap.Int("embeddings", len(embeddings)),
		)
		return
	}
	for index := range chunks {
		chunks[index].EmbeddingSignature = embeddingSignature
	}
	if err := s.repo.UpsertMessageChunks(ctx, chunks, embeddings); err != nil {
		s.logger.Warn("upsert_message_chunks_failed", zap.Error(err))
	}
}

func reasoningPayload(delta *llm.ReasoningDelta) *tracePayload {
	if delta == nil {
		return nil
	}
	payload := &tracePayload{Reasoning: &traceReasoning{EventType: delta.EventType, ItemID: delta.ItemID, Status: delta.Status}}
	if strings.TrimSpace(delta.Signature) != "" {
		payload.Reasoning.Signature = strings.TrimSpace(delta.Signature)
	}
	if strings.TrimSpace(delta.EncryptedContent) != "" {
		payload.Reasoning.EncryptedContent = strings.TrimSpace(delta.EncryptedContent)
	}
	return payload
}

// recallSemanticContext 语义召回历史消息；无结果时返回空列表。
func (s *Service) recallSemanticContext(ctx context.Context, scope repository.HistoricalMessageScope, query string) []model.MessageChunk {
	if s.embeddingSvc == nil || !scope.Valid() || strings.TrimSpace(query) == "" {
		return nil
	}
	embeddings, embeddingSignature, err := s.embeddingSvc.EmbedTextsWithSignature(ctx, []string{query})
	if err != nil || len(embeddings) == 0 {
		return nil
	}
	chunks, err := s.repo.SearchMessageChunks(ctx, repository.MessageChunkSearchInput{
		Scope:              scope,
		QueryEmbedding:     embeddings[0],
		EmbeddingSignature: embeddingSignature,
		TopK:               5,
		MinSimilarity:      0.75,
	})
	if err != nil || len(chunks) == 0 {
		return nil
	}
	return chunks
}

// callCompactLLM 是注入到 compact.Service 的 LLM 摘要回调。
// 通过当前路由解析选择上游，构造摘要请求并返回摘要文本。
func (s *Service) callCompactLLM(ctx context.Context, platformModelName string, messages []model.Message, prompt string) (string, error) {
	if s.routeResolver == nil || s.llmClient == nil {
		return "", errors.New("llm not configured")
	}

	code := platformModelName
	if strings.TrimSpace(code) == "" {
		return "", errors.New("compact model not configured")
	}

	route, err := s.routeResolver.ResolveRoute(ctx, channel.ResolveRouteInput{
		PlatformModelName: code,
		TaskType:          channel.TaskTypeChat,
		Scope:             channel.RouteScopeInternal,
	})
	if err != nil {
		return "", fmt.Errorf("compact route resolve: %w", err)
	}

	// 构建摘要请求：系统提示 + 历史消息（内容截断防止超长）。
	const maxContentRunes = 2000
	llmMsgs := make([]llm.Message, 0, len(messages)+1)
	llmMsgs = append(llmMsgs, llm.Message{Role: "system", Content: prompt})
	for _, m := range messages {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		content := m.Content
		runes := []rune(content)
		if len(runes) > maxContentRunes {
			content = string(runes[:maxContentRunes]) + "...[truncated]"
		}
		llmMsgs = append(llmMsgs, llm.Message{Role: m.Role, Content: content})
	}

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
	startedAt := time.Now()
	generateInput := buildTextTaskGenerateInput(route, s.cfg.Snapshot(), llmMsgs)
	var authorization *domainbilling.UsageAuthorization
	billingCtx, hasBillingContext := ctx.Value(basicServiceBillingContextKey{}).(basicServiceBillingContext)
	if hasBillingContext {
		authorization, err = s.authorizeBasicServiceUsage(ctx, billingCtx.UserID, route.PlatformModelName, "compact")
		if err != nil {
			return "", fmt.Errorf("compact usage authorization: %w", err)
		}
	}
	out, err := s.llmClient.Generate(ctx, routeConfig, generateInput)
	if err != nil {
		if releaseErr := s.releaseBasicServiceUsageAuthorization(ctx, authorization); releaseErr != nil {
			return "", errors.Join(fmt.Errorf("compact llm generate: %w", err), fmt.Errorf("release compact usage authorization: %w", releaseErr))
		}
		return "", fmt.Errorf("compact llm generate: %w", err)
	}
	text := strings.TrimSpace(out.Text)
	if hasBillingContext {
		if err = s.recordBasicServiceUsage(ctx, basicServiceUsageInput{
			Authorization:     authorization,
			UserID:            billingCtx.UserID,
			ConversationID:    billingCtx.ConversationID,
			ServiceCode:       "compact",
			ServiceName:       "上下文压缩",
			PlatformModelName: route.PlatformModelName,
			RoutedBindingCode: route.BindingCode,
			ProviderProtocol:  route.Protocol,
			UpstreamName:      route.UpstreamName,
			UpstreamModelName: route.UpstreamModel,
			CacheTimeout:      "5m",
			Usage:             out.Usage,
			FallbackMessages:  generateInput.Messages,
			FallbackOutput:    text,
			LatencyMS:         time.Since(startedAt).Milliseconds(),
		}); err != nil {
			return "", fmt.Errorf("compact usage settlement: %w", err)
		}
	}
	return text, nil
}

func withBasicServiceBillingContext(ctx context.Context, userID uint, conversationID uint) context.Context {
	return context.WithValue(ctx, basicServiceBillingContextKey{}, basicServiceBillingContext{
		UserID:         userID,
		ConversationID: conversationID,
	})
}

// authorizeBasicServiceUsage 为标题、标签和压缩等内部模型调用分配独立预算。
func (s *Service) authorizeBasicServiceUsage(ctx context.Context, userID uint, platformModelName string, serviceCode string) (*domainbilling.UsageAuthorization, error) {
	if s.billingSvc == nil {
		return &domainbilling.UsageAuthorization{Mode: "self"}, nil
	}
	refNo := "basic_" + strings.TrimSpace(serviceCode) + "_" + uuid.NewString()
	return s.billingSvc.AuthorizeUsage(ctx, userID, platformModelName, refNo)
}

// releaseBasicServiceUsageAuthorization 释放未产生可计费用量的内部调用预算。
func (s *Service) releaseBasicServiceUsageAuthorization(ctx context.Context, authorization *domainbilling.UsageAuthorization) error {
	if s.billingSvc == nil || authorization == nil {
		return nil
	}
	return s.billingSvc.ReleaseUsageAuthorization(ctx, authorization)
}

type basicServiceUsageInput struct {
	Authorization     *domainbilling.UsageAuthorization
	UserID            uint
	ConversationID    uint
	ServiceCode       string
	ServiceName       string
	PlatformModelName string
	RoutedBindingCode string
	ProviderProtocol  string
	UpstreamName      string
	UpstreamModelName string
	CacheTimeout      string
	Usage             llm.Usage
	FallbackMessages  []llm.Message
	FallbackOutput    string
	LatencyMS         int64
}

func (s *Service) recordBasicServiceUsage(ctx context.Context, input basicServiceUsageInput) error {
	if s.billingSvc == nil || input.UserID == 0 || input.ConversationID == 0 || strings.TrimSpace(input.PlatformModelName) == "" {
		return nil
	}
	billingCtx, cancel := background.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// 基础服务常与主对话共享系统前缀，提示词全部命中缓存时非缓存输入为 0 是合法观测值，
	// 只有上游完全没上报输入侧用量才用预估补齐。
	inputTokens := input.Usage.InputTokens
	if !input.Usage.HasObservedInput() {
		inputTokens = estimatePromptTokens(input.FallbackMessages)
	}
	outputTokens := input.Usage.OutputTokens
	if outputTokens <= 0 {
		outputTokens = tokenestimate.Estimate(input.FallbackOutput)
	}
	item := appbilling.ServiceUsageInput{
		ServiceCode:        strings.TrimSpace(input.ServiceCode),
		ServiceName:        strings.TrimSpace(input.ServiceName),
		PlatformModelName:  strings.TrimSpace(input.PlatformModelName),
		UpstreamModelName:  strings.TrimSpace(input.UpstreamModelName),
		ProviderProtocol:   strings.TrimSpace(input.ProviderProtocol),
		CacheTimeout:       input.CacheTimeout,
		UsageSpeed:         strings.TrimSpace(input.Usage.Speed),
		UsageServiceTier:   strings.TrimSpace(input.Usage.ServiceTier),
		InputTokens:        inputTokens,
		CacheReadTokens:    input.Usage.CacheReadTokens,
		CacheWriteTokens:   input.Usage.CacheWriteTokens,
		CacheWrite5mTokens: input.Usage.CacheWrite5mTokens,
		CacheWrite1hTokens: input.Usage.CacheWrite1hTokens,
		OutputTokens:       outputTokens,
		ReasoningTokens:    input.Usage.ReasoningTokens,
		CallCount:          1,
	}
	pricingInput := appbilling.UsagePricingInput{
		Authorization:      input.Authorization,
		UserID:             input.UserID,
		ConversationID:     input.ConversationID,
		PlatformModelName:  item.PlatformModelName,
		RoutedBindingCode:  strings.TrimSpace(input.RoutedBindingCode),
		ProviderProtocol:   item.ProviderProtocol,
		UpstreamName:       strings.TrimSpace(input.UpstreamName),
		UpstreamModelName:  strings.TrimSpace(input.UpstreamModelName),
		CacheTimeout:       item.CacheTimeout,
		UsageSpeed:         strings.TrimSpace(input.Usage.Speed),
		UsageServiceTier:   strings.TrimSpace(input.Usage.ServiceTier),
		ServiceOnly:        true,
		InputTokens:        item.InputTokens,
		CacheReadTokens:    item.CacheReadTokens,
		CacheWriteTokens:   item.CacheWriteTokens,
		CacheWrite5mTokens: item.CacheWrite5mTokens,
		CacheWrite1hTokens: item.CacheWrite1hTokens,
		OutputTokens:       item.OutputTokens,
		ReasoningTokens:    item.ReasoningTokens,
		CallCount:          item.CallCount,
		LatencyMS:          input.LatencyMS,
		ServiceItems:       []appbilling.ServiceUsageInput{item},
		RawUsageJSON:       input.Usage.RawUsageJSON,
	}
	var ledger *domainbilling.UsageLedger
	err := retryUsageBillingOperation(billingCtx, func() error {
		var buildErr error
		ledger, buildErr = s.billingSvc.BuildUsageLedger(billingCtx, pricingInput)
		return buildErr
	})
	if err != nil {
		err = s.markUsageAuthorizationForReconciliation(billingCtx, input.Authorization, "basic_build_usage_failed", err)
		if s.logger != nil {
			s.logger.Warn("basic_service_usage_build_failed",
				zap.Uint("user_id", input.UserID),
				zap.Uint("conversation_id", input.ConversationID),
				zap.String("service", item.ServiceCode),
				zap.String("model", item.PlatformModelName),
				zap.Error(err),
			)
		}
		return err
	}
	if err := s.recordUsageWithRetry(billingCtx, ledger, input.Authorization); err != nil {
		err = s.markUsageAuthorizationForReconciliation(billingCtx, input.Authorization, "basic_settle_usage_failed", err)
		if s.logger != nil {
			s.logger.Warn("basic_service_usage_record_failed",
				zap.Uint("user_id", input.UserID),
				zap.Uint("conversation_id", input.ConversationID),
				zap.String("service", item.ServiceCode),
				zap.String("model", item.PlatformModelName),
				zap.Error(err),
			)
		}
		return err
	}
	return nil
}
