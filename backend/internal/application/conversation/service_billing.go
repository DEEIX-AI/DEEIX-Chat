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
)

type messageBillingUpdateError struct {
	err error
}

func (e *messageBillingUpdateError) Error() string {
	return e.err.Error()
}

func (e *messageBillingUpdateError) Unwrap() error {
	return e.err
}

// SendMessageBillingInput 描述一次消息发送对应的计费上下文。
type SendMessageBillingInput struct {
	UserID            uint
	ConversationID    uint
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

// EnsureSendMessageBillingAccess 校验本次发送在当前计费策略下是否可用。
func (s *Service) EnsureSendMessageBillingAccess(ctx context.Context, input SendMessageBillingInput) error {
	if s.billingSvc == nil {
		return nil
	}
	return s.billingSvc.EnsureModelUsable(ctx, input.UserID, sendMessageBillingPlatformModelName(input), time.Now())
}

// ReserveSendMessageUsageBalance 在模型调用前执行按量预扣。
func (s *Service) ReserveSendMessageUsageBalance(ctx context.Context, input SendMessageBillingInput) (*domainbilling.UsageBalanceReservation, error) {
	if s.billingSvc == nil {
		return nil, nil
	}
	return s.billingSvc.ReserveUsageBalance(ctx, input.UserID, sendMessageBillingPlatformModelName(input), strings.TrimSpace(input.ClientRunID))
}

// ReleaseSendMessageUsageReservation 在调用失败或计费失败时退回预扣。
func (s *Service) ReleaseSendMessageUsageReservation(ctx context.Context, reservation *domainbilling.UsageBalanceReservation, description string) error {
	if s.billingSvc == nil || reservation == nil {
		return nil
	}
	return s.billingSvc.ReleaseUsageBalanceReservation(ctx, reservation, description)
}

// RecordSendMessageBilling 记录发送消息产生的用量账本，并把账单快照回写到 assistant 消息。
func (s *Service) RecordSendMessageBilling(
	ctx context.Context,
	input SendMessageBillingInput,
	reservation *domainbilling.UsageBalanceReservation,
) (*domainbilling.UsageLedger, error) {
	if s.billingSvc == nil || input.Result == nil {
		return nil, nil
	}
	usageLedger, err := s.buildSendMessageUsageLedger(ctx, input)
	if err != nil {
		return nil, err
	}
	if usageLedger == nil {
		return nil, nil
	}
	if err = s.billingSvc.RecordUsageWithReservation(ctx, usageLedger, reservation); err != nil {
		return nil, err
	}
	if err = s.UpdateMessageBilling(ctx, input.Result.AssistantMessage.ID, usageLedger); err != nil {
		return nil, &messageBillingUpdateError{err: err}
	}
	return usageLedger, nil
}

// ShouldReleaseSendMessageUsageReservationAfterBillingError 判断计费失败后是否应退回预扣。
func ShouldReleaseSendMessageUsageReservationAfterBillingError(err error) bool {
	var updateErr *messageBillingUpdateError
	return !errors.As(err, &updateErr)
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

func (s *Service) buildSendMessageUsageLedger(ctx context.Context, input SendMessageBillingInput) (*domainbilling.UsageLedger, error) {
	result := input.Result
	if result == nil {
		return nil, nil
	}
	latencyMS := result.LatencyMS
	if latencyMS <= 0 {
		latencyMS = result.AssistantMessage.LatencyMS
	}
	return s.billingSvc.BuildUsageLedger(ctx, appbilling.UsagePricingInput{
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
		InputTokens:         result.UserMessage.InputTokens,
		CacheReadTokens:     result.UserMessage.CacheReadTokens,
		CacheWriteTokens:    result.UserMessage.CacheWriteTokens,
		CacheWrite5mTokens:  result.CacheWrite5mTokens,
		CacheWrite1hTokens:  result.CacheWrite1hTokens,
		OutputTokens:        result.AssistantMessage.OutputTokens,
		ReasoningTokens:     result.AssistantMessage.ReasoningTokens,
		CallCount:           1,
		LatencyMS:           latencyMS,
		ServerSideToolUsage: result.ServerSideToolUsage,
		RawUsageJSON:        result.RawUsageJSON,
	})
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
