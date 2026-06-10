package conversation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
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
			TokenCount:     int(estimateTokens(userMsg.Content)),
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
			TokenCount:     int(estimateTokens(assistantMsg.Content)),
		})
		texts = append(texts, assistantMsg.Content)
	}
	if len(chunks) == 0 {
		return
	}
	embeddings, err := s.embeddingSvc.EmbedTexts(ctx, texts)
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
	if err := s.repo.UpsertMessageChunks(ctx, chunks, embeddings); err != nil {
		s.logger.Warn("upsert_message_chunks_failed", zap.Error(err))
	}
}

func reasoningPayload(delta *llm.ReasoningDelta) map[string]interface{} {
	if delta == nil {
		return nil
	}
	payload := map[string]interface{}{
		"event_type": delta.EventType,
		"item_id":    delta.ItemID,
		"status":     delta.Status,
	}
	if strings.TrimSpace(delta.Signature) != "" {
		payload["signature"] = strings.TrimSpace(delta.Signature)
	}
	if strings.TrimSpace(delta.EncryptedContent) != "" {
		payload["encrypted_content"] = strings.TrimSpace(delta.EncryptedContent)
	}
	return payload
}

// recallSemanticContext 语义召回历史消息；无结果时返回空列表。
func (s *Service) recallSemanticContext(ctx context.Context, conversationID uint, userID uint, query string) []model.MessageChunk {
	if s.embeddingSvc == nil || strings.TrimSpace(query) == "" {
		return nil
	}
	embeddings, err := s.embeddingSvc.EmbedTexts(ctx, []string{query})
	if err != nil || len(embeddings) == 0 {
		return nil
	}
	chunks, err := s.repo.SearchMessageChunks(ctx, conversationID, userID, embeddings[0], 5, 0.75)
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
	out, err := s.llmClient.Generate(ctx, routeConfig, llm.GenerateInput{
		Messages: llmMsgs,
	})
	if err != nil {
		return "", fmt.Errorf("compact llm generate: %w", err)
	}
	text := strings.TrimSpace(out.Text)
	if billingCtx, ok := ctx.Value(basicServiceBillingContextKey{}).(basicServiceBillingContext); ok {
		s.recordBasicServiceUsage(ctx, billingCtx.UserID, billingCtx.ConversationID, "compact", "上下文压缩", code, route.BindingCode, route.Protocol, route.UpstreamName, route.UpstreamModel, "5m", out.Usage, llmMsgs, text, time.Since(startedAt).Milliseconds())
	}
	return text, nil
}

func withBasicServiceBillingContext(ctx context.Context, userID uint, conversationID uint) context.Context {
	return context.WithValue(ctx, basicServiceBillingContextKey{}, basicServiceBillingContext{
		UserID:         userID,
		ConversationID: conversationID,
	})
}

func (s *Service) recordBasicServiceUsage(
	ctx context.Context,
	userID uint,
	conversationID uint,
	serviceCode string,
	serviceName string,
	platformModelName string,
	routedBindingCode string,
	providerProtocol string,
	upstreamName string,
	upstreamModelName string,
	cacheTimeout string,
	usage llm.Usage,
	fallbackMessages []llm.Message,
	fallbackOutput string,
	latencyMS int64,
) {
	if s.billingSvc == nil || userID == 0 || conversationID == 0 || strings.TrimSpace(platformModelName) == "" {
		return
	}
	billingCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	inputTokens := usage.InputTokens
	if inputTokens <= 0 {
		inputTokens = estimatePromptTokens(fallbackMessages)
	}
	outputTokens := usage.OutputTokens
	if outputTokens <= 0 {
		outputTokens = estimateTokens(fallbackOutput)
	}
	item := appbilling.ServiceUsageInput{
		ServiceCode:        strings.TrimSpace(serviceCode),
		ServiceName:        strings.TrimSpace(serviceName),
		PlatformModelName:  strings.TrimSpace(platformModelName),
		UpstreamModelName:  strings.TrimSpace(upstreamModelName),
		ProviderProtocol:   strings.TrimSpace(providerProtocol),
		CacheTimeout:       cacheTimeout,
		UsageSpeed:         strings.TrimSpace(usage.Speed),
		UsageServiceTier:   strings.TrimSpace(usage.ServiceTier),
		InputTokens:        inputTokens,
		CacheReadTokens:    usage.CacheReadTokens,
		CacheWriteTokens:   usage.CacheWriteTokens,
		CacheWrite5mTokens: usage.CacheWrite5mTokens,
		CacheWrite1hTokens: usage.CacheWrite1hTokens,
		OutputTokens:       outputTokens,
		ReasoningTokens:    usage.ReasoningTokens,
		CallCount:          1,
	}
	ledger, err := s.billingSvc.BuildUsageLedger(billingCtx, appbilling.UsagePricingInput{
		UserID:             userID,
		ConversationID:     conversationID,
		PlatformModelName:  item.PlatformModelName,
		RoutedBindingCode:  strings.TrimSpace(routedBindingCode),
		ProviderProtocol:   item.ProviderProtocol,
		UpstreamName:       strings.TrimSpace(upstreamName),
		UpstreamModelName:  strings.TrimSpace(upstreamModelName),
		CacheTimeout:       item.CacheTimeout,
		UsageSpeed:         strings.TrimSpace(usage.Speed),
		UsageServiceTier:   strings.TrimSpace(usage.ServiceTier),
		ServiceOnly:        true,
		InputTokens:        item.InputTokens,
		CacheReadTokens:    item.CacheReadTokens,
		CacheWriteTokens:   item.CacheWriteTokens,
		CacheWrite5mTokens: item.CacheWrite5mTokens,
		CacheWrite1hTokens: item.CacheWrite1hTokens,
		OutputTokens:       item.OutputTokens,
		ReasoningTokens:    item.ReasoningTokens,
		CallCount:          item.CallCount,
		LatencyMS:          latencyMS,
		ServiceItems:       []appbilling.ServiceUsageInput{item},
		RawUsageJSON:       usage.RawUsageJSON,
	})
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("basic_service_usage_build_failed",
				zap.Uint("user_id", userID),
				zap.Uint("conversation_id", conversationID),
				zap.String("service", item.ServiceCode),
				zap.String("model", item.PlatformModelName),
				zap.Error(err),
			)
		}
		return
	}
	if err := s.billingSvc.RecordUsage(billingCtx, ledger); err != nil && s.logger != nil {
		s.logger.Warn("basic_service_usage_record_failed",
			zap.Uint("user_id", userID),
			zap.Uint("conversation_id", conversationID),
			zap.String("service", item.ServiceCode),
			zap.String("model", item.PlatformModelName),
			zap.Error(err),
		)
	}
}
