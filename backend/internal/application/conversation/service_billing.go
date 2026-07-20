package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const (
	usageReconciliationBuildLedger = "build_usage_ledger_failed"
	usageReconciliationSettlement  = "settle_usage_ledger_failed"
	usageBillingRetryAttempts      = 3
	usageBillingRetryBaseDelay     = 100 * time.Millisecond
)

// SendMessageBillingInput 描述一次消息发送对应的计费上下文。
type SendMessageBillingInput struct {
	UserID            uint
	ConversationID    uint
	Conversation      *model.Conversation
	PlatformModelName string
	ConversationModel string
	ClientRunID       string
	Result            *SendMessageResult
}

// SendMessageAuditInput 描述一次消息发送对应的审计上下文。
type SendMessageAuditInput struct {
	UserID         uint
	RequestID      string
	ClientIP       string
	UserAgent      string
	Action         string
	ContentType    string
	ConversationID uint
	FileIDs        []string
	Result         *SendMessageResult
}

type attachmentKindEntry struct {
	Kind     string `json:"kind"`
	MimeType string `json:"mime_type"`
}

// ApplyUsageBilling 将计费账本快照回填到消息 DTO，供流式完成事件立即返回。
func ApplyUsageBilling(message *model.Message, usage *domainbilling.UsageLedger) {
	if message == nil || usage == nil {
		return
	}
	message.BilledCurrency = usage.BilledCurrency
	message.BilledNanousd = usage.BilledNanousd
	message.PricingSnapshot = usage.PricingSnapshotJSON
}

// UpdateMessageBilling 持久化消息计费金额与计费快照。
func (s *Service) UpdateMessageBilling(ctx context.Context, messageID uint, usage *domainbilling.UsageLedger) error {
	if usage == nil || messageID == 0 {
		return nil
	}
	return s.repo.UpdateMessageBilling(ctx, messageID, usage.BilledCurrency, usage.BilledNanousd, usage.PricingSnapshotJSON)
}

// AuthorizeSendMessageUsage 在模型调用前固定计费策略并预留可用预算。
func (s *Service) AuthorizeSendMessageUsage(ctx context.Context, input SendMessageBillingInput) (*domainbilling.UsageAuthorization, error) {
	if s.billingSvc == nil {
		return &domainbilling.UsageAuthorization{Mode: "self"}, nil
	}
	return s.billingSvc.AuthorizeUsage(ctx, input.UserID, sendMessageBillingPlatformModelName(input), strings.TrimSpace(input.ClientRunID))
}

// ReleaseSendMessageUsageAuthorization 在调用未产生可计费用量时释放预留预算。
func (s *Service) ReleaseSendMessageUsageAuthorization(ctx context.Context, authorization *domainbilling.UsageAuthorization) error {
	if s.billingSvc == nil || authorization == nil {
		return nil
	}
	return s.billingSvc.ReleaseUsageAuthorization(ctx, authorization)
}

// RenewSendMessageUsageAuthorization 延长仍在运行的付费调用预算租约。
func (s *Service) RenewSendMessageUsageAuthorization(ctx context.Context, authorization *domainbilling.UsageAuthorization) error {
	if s.billingSvc == nil || authorization == nil {
		return nil
	}
	return s.billingSvc.RenewUsageAuthorization(ctx, authorization)
}

// RecordSendMessageBilling 记录发送消息产生的用量账本，并把账单快照回写到 assistant 消息。
func (s *Service) RecordSendMessageBilling(
	ctx context.Context,
	input SendMessageBillingInput,
	authorization *domainbilling.UsageAuthorization,
) (*domainbilling.UsageLedger, error) {
	if input.Result == nil {
		return nil, nil
	}
	if s.billingSvc == nil {
		s.runPostBillingTasks(input)
		return nil, nil
	}
	var usageLedger *domainbilling.UsageLedger
	err := retryUsageBillingOperation(ctx, func() error {
		var buildErr error
		usageLedger, buildErr = s.buildSendMessageUsageLedger(ctx, input, authorization)
		return buildErr
	})
	if err != nil {
		s.discardPostBillingCompaction(input.Result)
		return nil, s.markUsageAuthorizationForReconciliation(ctx, authorization, usageReconciliationBuildLedger, err)
	}
	if usageLedger == nil {
		return nil, nil
	}
	if err = s.recordUsageWithRetry(ctx, usageLedger, authorization); err != nil {
		s.discardPostBillingCompaction(input.Result)
		return nil, s.markUsageAuthorizationForReconciliation(ctx, authorization, usageReconciliationSettlement, err)
	}
	if err = retryUsageBillingOperation(ctx, func() error {
		return s.UpdateMessageBilling(ctx, input.Result.AssistantMessage.ID, usageLedger)
	}); err != nil {
		s.discardPostBillingCompaction(input.Result)
		return nil, err
	}
	s.runPostBillingTasks(input)
	return usageLedger, nil
}

// scheduleConversationMetadataAfterBilling 仅在主调用完成计费后安排标题与标签生成。
func (s *Service) scheduleConversationMetadataAfterBilling(input SendMessageBillingInput) {
	if input.Conversation == nil || input.Result == nil || input.Result.MetadataRefreshHint != conversationMetadataRefreshPending {
		return
	}
	if strings.TrimSpace(input.Result.UserMessage.RunID) != strings.TrimSpace(input.ClientRunID) {
		return
	}
	conversation := *input.Conversation
	if platformModelName := strings.TrimSpace(input.Result.PlatformModelName); platformModelName != "" {
		conversation.Model = platformModelName
	}
	s.maybeGenerateConversationMetadataAsync(conversation, input.Result.UserMessage)
}

// markUsageAuthorizationForReconciliation 将已产生上游费用的结算失败转为保守阻断状态。
func (s *Service) markUsageAuthorizationForReconciliation(
	ctx context.Context,
	authorization *domainbilling.UsageAuthorization,
	failureCode string,
	cause error,
) error {
	if s.billingSvc == nil || authorization == nil || authorization.Reservation == nil {
		return cause
	}
	if err := retryUsageBillingOperation(ctx, func() error {
		return s.billingSvc.MarkUsageAuthorizationForReconciliation(ctx, authorization, failureCode)
	}); err != nil {
		return errors.Join(cause, fmt.Errorf("mark usage reconciliation: %w", err))
	}
	return cause
}

// recordUsageWithRetry 仅对持有持久化 reservation 的结算执行安全重试。
func (s *Service) recordUsageWithRetry(ctx context.Context, usage *domainbilling.UsageLedger, authorization *domainbilling.UsageAuthorization) error {
	operation := func() error {
		return s.billingSvc.RecordUsageWithAuthorization(ctx, usage, authorization)
	}
	if authorization == nil || authorization.Reservation == nil {
		return operation()
	}
	return retryUsageBillingOperation(ctx, operation)
}

// retryUsageBillingOperation 对临时账务错误进行短暂退避重试，不重试业务语义错误。
func retryUsageBillingOperation(ctx context.Context, operation func() error) error {
	var err error
	for attempt := 0; attempt < usageBillingRetryAttempts; attempt++ {
		if err = operation(); err == nil || !isRetryableUsageBillingError(err) {
			return err
		}
		if attempt == usageBillingRetryAttempts-1 {
			break
		}
		delay := usageBillingRetryBaseDelay << attempt
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(err, ctx.Err())
		case <-timer.C:
		}
	}
	return err
}

// isRetryableUsageBillingError 排除重试无法改变结果的业务与状态错误。
func isRetryableUsageBillingError(err error) bool {
	if err == nil {
		return false
	}
	nonRetryable := []error{
		context.Canceled,
		context.DeadlineExceeded,
		appbilling.ErrUsageBalanceInsufficient,
		appbilling.ErrUsageReservationConflict,
		appbilling.ErrModelPricingRequired,
		repository.ErrInvalidInput,
		repository.ErrNotFound,
		repository.ErrDuplicate,
		repository.ErrConflict,
		repository.ErrInsufficientBalance,
	}
	for _, target := range nonRetryable {
		if errors.Is(err, target) {
			return false
		}
	}
	return true
}

// RecordSendMessageAudit 记录发送消息审计日志。
func (s *Service) RecordSendMessageAudit(ctx context.Context, input SendMessageAuditInput) {
	if s.auditWriter == nil || input.Result == nil {
		return
	}
	imageCount, fileCount := countAttachmentKinds(input.Result.UserMessage.Attachments)
	s.auditWriter.Write(
		ctx,
		strings.TrimSpace(input.RequestID),
		input.UserID,
		strings.TrimSpace(input.Action),
		"conversation",
		strconv.FormatUint(uint64(input.ConversationID), 10),
		strings.TrimSpace(input.ClientIP),
		strings.TrimSpace(input.UserAgent),
		map[string]interface{}{
			"content_type": strings.TrimSpace(input.ContentType),
			"attachments":  imageCount + fileCount,
			"file_ids":     len(input.FileIDs),
		},
	)
}

// buildSendMessageUsageLedger 根据请求开始时的授权快照构建主调用账本。
func (s *Service) buildSendMessageUsageLedger(ctx context.Context, input SendMessageBillingInput, authorization *domainbilling.UsageAuthorization) (*domainbilling.UsageLedger, error) {
	result := input.Result
	if result == nil {
		return nil, nil
	}
	latencyMS := result.LatencyMS
	if latencyMS <= 0 {
		latencyMS = result.AssistantMessage.LatencyMS
	}
	return s.billingSvc.BuildUsageLedger(ctx, appbilling.UsagePricingInput{
		Authorization:       authorization,
		UserID:              input.UserID,
		ConversationID:      input.ConversationID,
		PlatformModelName:   sendMessageBillingPlatformModelName(input),
		RoutedBindingCode:   strings.TrimSpace(result.RoutedBindingCode),
		ProviderProtocol:    strings.TrimSpace(result.UpstreamProtocol),
		UpstreamName:        strings.TrimSpace(result.UpstreamName),
		UpstreamModelName:   strings.TrimSpace(result.UpstreamModelName),
		CacheTimeout:        messageCacheTimeout(result.EffectiveOptions),
		RequestSpeed:        messageRequestSpeed(result.EffectiveOptions),
		UsageSpeed:          strings.TrimSpace(result.UsageSpeed),
		RequestServiceTier:  messageRequestServiceTier(result.EffectiveOptions),
		UsageServiceTier:    strings.TrimSpace(result.UsageServiceTier),
		UsageSource:         strings.TrimSpace(result.UsageSource),
		InputTokens:         sendMessageBillingInputTokens(result),
		CacheReadTokens:     sendMessageBillingCacheReadTokens(result),
		CacheWriteTokens:    sendMessageBillingCacheWriteTokens(result),
		CacheWrite5mTokens:  result.CacheWrite5mTokens,
		CacheWrite1hTokens:  result.CacheWrite1hTokens,
		OutputTokens:        result.AssistantMessage.OutputTokens,
		ReasoningTokens:     result.AssistantMessage.ReasoningTokens,
		CallCount:           1,
		DurationSeconds:     sendMessageBillingDurationSeconds(result, latencyMS),
		LatencyMS:           latencyMS,
		ServerSideToolUsage: result.ServerSideToolUsage,
		RawUsageJSON:        result.RawUsageJSON,
		BillingAt:           result.StartedAt,
	})
}

func sendMessageBillingInputTokens(result *SendMessageResult) int64 {
	if result == nil {
		return 0
	}
	if sendMessageResultUsesAssistantSideInput(result) {
		return result.AssistantMessage.InputTokens
	}
	return result.UserMessage.InputTokens
}

func sendMessageBillingCacheReadTokens(result *SendMessageResult) int64 {
	if result == nil {
		return 0
	}
	if sendMessageResultUsesAssistantSideInput(result) {
		return result.AssistantMessage.CacheReadTokens
	}
	return result.UserMessage.CacheReadTokens
}

func sendMessageBillingCacheWriteTokens(result *SendMessageResult) int64 {
	if result == nil {
		return 0
	}
	if sendMessageResultUsesAssistantSideInput(result) {
		return result.AssistantMessage.CacheWriteTokens
	}
	return result.UserMessage.CacheWriteTokens
}

func sendMessageBillingDurationSeconds(result *SendMessageResult, latencyMS int64) int64 {
	if result != nil && result.DurationSeconds > 0 {
		return result.DurationSeconds
	}
	if latencyMS <= 0 {
		return 0
	}
	return (latencyMS + 999) / 1000
}

// sendMessageResultUsesAssistantSideInput 判断 prompt-side usage 是否归属 assistant 消息。
// assistant-only retry 会复用原用户消息，因此本轮 input/cache usage 不能回写到 user。
func sendMessageResultUsesAssistantSideInput(result *SendMessageResult) bool {
	return result != nil && result.AssistantMessage.SourceMessageID != nil
}

func sendMessageBillingPlatformModelName(input SendMessageBillingInput) string {
	if input.Result != nil {
		if value := strings.TrimSpace(input.Result.PlatformModelName); value != "" {
			return value
		}
	}
	if value := strings.TrimSpace(input.PlatformModelName); value != "" {
		return value
	}
	return strings.TrimSpace(input.ConversationModel)
}

func messageCacheTimeout(options map[string]interface{}) string {
	if len(options) == 0 {
		return "5m"
	}
	if value := strings.TrimSpace(stringOption(options, "cache_timeout")); value != "" {
		if strings.EqualFold(value, "1h") {
			return "1h"
		}
		return "5m"
	}
	if cacheControl, ok := options["cache_control"].(map[string]interface{}); ok {
		if value := strings.TrimSpace(stringOption(cacheControl, "ttl")); strings.EqualFold(value, "1h") {
			return "1h"
		}
	}
	return "5m"
}

func messageRequestSpeed(options map[string]interface{}) string {
	if len(options) == 0 {
		return ""
	}
	speed := strings.TrimSpace(stringOption(options, "speed"))
	if strings.EqualFold(speed, "fast") {
		return "fast"
	}
	return speed
}

func messageRequestServiceTier(options map[string]interface{}) string {
	if len(options) == 0 {
		return ""
	}
	return strings.TrimSpace(stringOption(options, "service_tier"))
}

func stringOption(options map[string]interface{}, key string) string {
	raw, ok := options[key]
	if !ok || raw == nil {
		return ""
	}
	switch value := raw.(type) {
	case string:
		return value
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func countAttachmentKinds(attachmentsJSON string) (int64, int64) {
	items := make([]attachmentKindEntry, 0)
	if err := json.Unmarshal([]byte(strings.TrimSpace(attachmentsJSON)), &items); err != nil {
		return 0, 0
	}

	var imageCount int64
	var fileCount int64
	for _, item := range items {
		switch NormalizeAttachmentKind(item.Kind, item.MimeType) {
		case "image":
			imageCount++
		default:
			fileCount++
		}
	}
	return imageCount, fileCount
}
