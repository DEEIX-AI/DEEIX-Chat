package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	appaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const (
	usageReconciliationBuildLedger = "build_usage_ledger_failed"
	usageReconciliationSettlement  = "settle_usage_ledger_failed"
	usageBillingRetryAttempts      = 3
	usageBillingRetryBaseDelay     = 100 * time.Millisecond
	messageUsageBalanceErrorCode   = "billing.insufficient_funds"
	messageUsageBalanceErrorText   = "insufficient balance"
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

// PersistMessageUsageRejection 将需要进入对话历史的终态业务拒绝持久化。
// 可重试的预算预约失败不产生消息，保证客户端可以安全重试。
func (s *Service) PersistMessageUsageRejection(
	ctx context.Context,
	input SendMessageInput,
	authorizationErr error,
) error {
	if !shouldPersistMessageUsageRejection(authorizationErr) {
		return nil
	}
	return s.persistRejectedMessageSend(
		ctx,
		input,
		messageUsageBalanceErrorCode,
		messageUsageBalanceErrorText,
	)
}

func shouldPersistMessageUsageRejection(err error) bool {
	return errors.Is(err, appbilling.ErrUsageBalanceInsufficient)
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

// usageBudgetEstimate 描述上游调用前已确定的请求形状，用于成本预估与预算校验。
// 工具循环中再次调用时，字段是本条消息已产生的用量与下一次调用预估之和。
type usageBudgetEstimate struct {
	InputTokens      int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	OutputTokens     int64
	CallCount        int64
	DurationSeconds  int64
}

// followUpUsageBudgetEstimate 构造再次调用上游前的累计成本形状：本条消息已产生的计费用量加上
// 下一次调用的预估输入与请求限定的最大输出。billed 是按计费口径汇总的已产生用量：输入与输出侧
// 含未上报调用的预估补齐，缓存读写为观测值。
func followUpUsageBudgetEstimate(billed llm.Usage, nextInputTokens int64, options map[string]any) usageBudgetEstimate {
	return usageBudgetEstimate{
		InputTokens:      billed.InputTokens + nextInputTokens,
		CacheReadTokens:  billed.CacheReadTokens,
		CacheWriteTokens: billed.CacheWriteTokens,
		OutputTokens:     billed.OutputTokens + billed.ReasoningTokens + messageRequestMaxOutputTokens(options),
	}
}

// ensureUsageBudgetCoversEstimate 在上游调用前按请求形状预估成本并抬高预算预留，
// 让余额不足的请求在产生任何上游费用之前被拒绝。没有预留（self 模式、免费模型）直接放行。
func (s *Service) ensureUsageBudgetCoversEstimate(
	ctx context.Context,
	authorization *domainbilling.UsageAuthorization,
	route *channel.ResolvedRoute,
	options map[string]any,
	estimate usageBudgetEstimate,
) error {
	if s.billingSvc == nil || authorization == nil || authorization.Reservation == nil || route == nil {
		return nil
	}
	requiredNanousd, err := s.billingSvc.EstimateUsageNanousd(ctx, authorization.Reservation.UserID, appbilling.UsageEstimateInput{
		PlatformModelName:  route.PlatformModelName,
		ProviderProtocol:   route.Protocol,
		UpstreamModelName:  route.UpstreamModel,
		CacheTimeout:       messageCacheTimeout(options),
		RequestSpeed:       messageRequestSpeed(options),
		RequestServiceTier: messageRequestServiceTier(options),
		InputTokens:        estimate.InputTokens,
		CacheReadTokens:    estimate.CacheReadTokens,
		CacheWriteTokens:   estimate.CacheWriteTokens,
		OutputTokens:       estimate.OutputTokens,
		CallCount:          estimate.CallCount,
		DurationSeconds:    estimate.DurationSeconds,
	})
	if err != nil {
		return err
	}
	return s.billingSvc.EnsureUsageAuthorizationBudget(ctx, authorization, requiredNanousd)
}

// messageRequestMaxOutputTokens 读取请求明确限定的最大输出 token 数，未限定返回 0。
func messageRequestMaxOutputTokens(options map[string]any) int64 {
	paths := [][]string{
		{"max_output_tokens"},
		{"max_completion_tokens"},
		{"max_tokens"},
		{"maxOutputTokens"},
		{"generationConfig", "maxOutputTokens"},
		{"generation_config", "max_output_tokens"},
	}
	for _, path := range paths {
		value, ok := readModelOptionPath(options, path)
		if !ok {
			continue
		}
		if tokens := int64FromOptionValue(value); tokens > 0 {
			return tokens
		}
	}
	return 0
}

func int64FromOptionValue(value any) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case json.Number:
		parsed, err := v.Int64()
		if err != nil {
			return 0
		}
		return parsed
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
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
		s.runPostBillingTasks(ctx, input)
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
	s.runPostBillingTasks(ctx, input)
	return usageLedger, nil
}

// scheduleConversationMetadataAfterBilling 仅在主调用完成计费后安排标题与标签生成。
func (s *Service) scheduleConversationMetadataAfterBilling(ctx context.Context, input SendMessageBillingInput) {
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
	s.maybeGenerateConversationMetadataAsync(ctx, conversation, input.Result.UserMessage)
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

// recordUsageWithRetry 只在结算可幂等时重试：持有预留的结算由预留状态机回读首次结果，
// 无预留的结算靠账本运行级幂等键回读；两者都没有的账本重试可能重复入账，只执行一次。
func (s *Service) recordUsageWithRetry(ctx context.Context, usage *domainbilling.UsageLedger, authorization *domainbilling.UsageAuthorization) error {
	operation := func() error {
		return s.billingSvc.RecordUsageWithAuthorization(ctx, usage, authorization)
	}
	if !usageRecordIsIdempotent(usage, authorization) {
		return operation()
	}
	return retryUsageBillingOperation(ctx, operation)
}

func usageRecordIsIdempotent(usage *domainbilling.UsageLedger, authorization *domainbilling.UsageAuthorization) bool {
	if authorization != nil && authorization.Reservation != nil {
		return true
	}
	return usage != nil && strings.TrimSpace(usage.RefNo) != ""
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
	s.auditWriter.Write(ctx, appaudit.WriteInput{
		RequestID:   input.RequestID,
		ActorUserID: input.UserID,
		Action:      input.Action,
		Resource:    "conversation",
		ResourceID:  strconv.FormatUint(uint64(input.ConversationID), 10),
		IP:          input.ClientIP,
		UserAgent:   input.UserAgent,
		Detail: map[string]any{
			"content_type": strings.TrimSpace(input.ContentType),
			"attachments":  imageCount + fileCount,
			"file_ids":     len(input.FileIDs),
		},
	})
}

// buildSendMessageUsageLedger 根据请求开始时的授权快照构建主调用账本。
func (s *Service) buildSendMessageUsageLedger(ctx context.Context, input SendMessageBillingInput, authorization *domainbilling.UsageAuthorization) (*domainbilling.UsageLedger, error) {
	result := input.Result
	if result == nil {
		return nil, nil
	}
	isVideoGeneration := sendMessageResultIsVideoGeneration(result)
	mediaType := ""
	inputImageCount := int64(0)
	if isVideoGeneration {
		mediaType = "video"
		inputImageCount, _ = countAttachmentKinds(result.UserMessage.Attachments)
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
		BilledReason:        ModerationBlockedBilledReason(result, authorization),
		InputTokens:         sendMessageBillingInputTokens(result),
		CacheReadTokens:     sendMessageBillingCacheReadTokens(result),
		CacheWriteTokens:    sendMessageBillingCacheWriteTokens(result),
		CacheWrite5mTokens:  result.CacheWrite5mTokens,
		CacheWrite1hTokens:  result.CacheWrite1hTokens,
		OutputTokens:        result.AssistantMessage.OutputTokens,
		ReasoningTokens:     result.AssistantMessage.ReasoningTokens,
		CallCount:           sendMessageBillingCallCount(result),
		DurationBillable:    isVideoGeneration,
		DurationSeconds:     sendMessageBillingDurationSeconds(result),
		MediaType:           mediaType,
		InputImageCount:     inputImageCount,
		LatencyMS:           latencyMS,
		ServerSideToolUsage: result.ServerSideToolUsage,
		MCPToolUsage:        sendMessageMCPToolUsageInputs(result),
		RawUsageJSON:        result.RawUsageJSON,
		BillingAt:           result.StartedAt,
	})
}

// sendMessageMCPToolUsageInputs 把运行时聚合的 MCP 调用计量映射为计费入参。
func sendMessageMCPToolUsageInputs(result *SendMessageResult) []appbilling.MCPToolUsageInput {
	if result == nil || len(result.MCPToolUsage) == 0 {
		return nil
	}
	items := make([]appbilling.MCPToolUsageInput, 0, len(result.MCPToolUsage))
	for _, usage := range result.MCPToolUsage {
		items = append(items, appbilling.MCPToolUsageInput{
			ServerID:     usage.ServerID,
			ServerName:   usage.ServerName,
			ToolName:     usage.ToolName,
			CallCount:    usage.CallCount,
			PriceNanousd: usage.PriceNanousd,
		})
	}
	return items
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

func sendMessageResultIsVideoGeneration(result *SendMessageResult) bool {
	return result != nil &&
		result.Billable &&
		strings.EqualFold(strings.TrimSpace(result.AssistantMessage.Status), "success") &&
		strings.EqualFold(strings.TrimSpace(result.AssistantMessage.ContentType), "video") &&
		llm.IsVideoGenerationAdapter(result.UpstreamProtocol)
}

func sendMessageBillingDurationSeconds(result *SendMessageResult) int64 {
	if !sendMessageResultIsVideoGeneration(result) || result.DurationSeconds <= 0 {
		return 0
	}
	return result.DurationSeconds
}

// sendMessageBillingCallCount 返回按次计费的调用数：本次运行成功返回的上游 LLM 调用数，
// 与 token 用量一样覆盖工具循环的每次回灌；中断运行已产生可计费用量，至少计 1 次。
func sendMessageBillingCallCount(result *SendMessageResult) int64 {
	if result == nil || result.LLMCallCount <= 0 {
		return 1
	}
	return int64(result.LLMCallCount)
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

func messageCacheTimeout(options map[string]any) string {
	if len(options) == 0 {
		return "5m"
	}
	if value := strings.TrimSpace(stringOption(options, "cache_timeout")); value != "" {
		if strings.EqualFold(value, "1h") {
			return "1h"
		}
		return "5m"
	}
	if cacheControl, ok := options["cache_control"].(map[string]any); ok {
		if value := strings.TrimSpace(stringOption(cacheControl, "ttl")); strings.EqualFold(value, "1h") {
			return "1h"
		}
	}
	return "5m"
}

func messageRequestSpeed(options map[string]any) string {
	if len(options) == 0 {
		return ""
	}
	speed := strings.TrimSpace(stringOption(options, "speed"))
	if strings.EqualFold(speed, "fast") {
		return "fast"
	}
	return speed
}

func messageRequestServiceTier(options map[string]any) string {
	if len(options) == 0 {
		return ""
	}
	return strings.TrimSpace(stringOption(options, "service_tier"))
}

func stringOption(options map[string]any, key string) string {
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
