package conversation

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	appprocessing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/processing"
	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/lifecycle"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// Handler 封装会话 HTTP 处理。
type Handler struct {
	service *appconversation.Service
	// uploads 与 processing 直接承接文件 CRUD、内容读取与处理状态查询，不经会话服务转发。
	uploads    *appupload.Service
	processing *appprocessing.Service
	cfg        *config.Runtime
	// shutdown 触发时订阅型长连接（run 对账流、run 观看流）立即退出，
	// 让优雅关停不被常驻 SSE 拖到超时；客户端依靠既有重连逻辑恢复。
	shutdown *lifecycle.Shutdown
}

func normalizeStreamEventPayload(eventType string, payload map[string]any) map[string]any {
	normalized := map[string]any{
		"type": eventType,
	}

	for key, value := range payload {
		switch typed := value.(type) {
		case *model.MessageTraceBlock:
			normalized[key] = toTraceBlockResponse(typed)
		case model.MessageTraceBlock:
			block := typed
			normalized[key] = toTraceBlockResponse(&block)
		case *model.MessageProcessTrace:
			normalized[key] = toMessageProcessTraceResponse(typed)
		case model.MessageProcessTrace:
			trace := typed
			normalized[key] = toMessageProcessTraceResponse(&trace)
		default:
			normalized[key] = value
		}
	}

	return normalized
}

// NewHandler 创建处理器。
func NewHandler(
	service *appconversation.Service,
	uploads *appupload.Service,
	processing *appprocessing.Service,
	cfg *config.Runtime,
	shutdown *lifecycle.Shutdown,
) *Handler {
	return &Handler{
		service:    service,
		uploads:    uploads,
		processing: processing,
		cfg:        cfg,
		shutdown:   shutdown,
	}
}

func (h *Handler) recordAudit(c *gin.Context, action string, resource string, resourceID string, detail any) {
	h.service.RecordAudit(c.Request.Context(), appconversation.AuditInput{
		ActorUserID: middleware.MustUserID(c),
		RequestID:   middleware.MustRequestID(c),
		Action:      action,
		Resource:    resource,
		ResourceID:  resourceID,
		IP:          c.ClientIP(),
		UserAgent:   c.Request.UserAgent(),
		Detail:      detail,
	})
}

const asyncAuditTimeout = 5 * time.Second

// sendMessageErrorStatuses 是消息发送、媒体生成与临时对话路径上哨兵错误到 HTTP 状态码的映射。
// 错误码与对外文案由哨兵自身（apperr）声明，这里只决定传输语义；HTTP 响应与 NDJSON 终态事件共用。
type sendMessageErrorStatus struct {
	err    error
	status int
}

var sendMessageErrorStatuses = []sendMessageErrorStatus{
	{err: appconversation.ErrConversationNotFound, status: http.StatusNotFound},
	{err: appconversation.ErrInvalidFileReference, status: http.StatusBadRequest},
	{err: appconversation.ErrFileNotFound, status: http.StatusNotFound},
	{err: appconversation.ErrFileTooLarge, status: http.StatusRequestEntityTooLarge},
	{err: appconversation.ErrInvalidMessageBranch, status: http.StatusBadRequest},
	{err: appconversation.ErrTooManyMessageFiles, status: http.StatusBadRequest},
	{err: appconversation.ErrTooManySelectedTools, status: http.StatusBadRequest},
	{err: appconversation.ErrMultipleImageAttachmentProcessors, status: http.StatusBadRequest},
	{err: appconversation.ErrImageAttachmentProcessingFailed, status: http.StatusBadGateway},
	{err: appconversation.ErrTooManySelectedSkills, status: http.StatusBadRequest},
	{err: appconversation.ErrSkillNotFound, status: http.StatusNotFound},
	{err: appconversation.ErrInvalidSkillUse, status: http.StatusBadRequest},
	{err: appconversation.ErrFileProcessingNotReady, status: http.StatusBadRequest},
	{err: appconversation.ErrFileTooLargeForFullContext, status: http.StatusBadRequest},
	{err: appconversation.ErrEmbeddingUnavailable, status: http.StatusBadRequest},
	{err: appconversation.ErrInvalidKnowledgeBaseReference, status: http.StatusBadRequest},
	{err: appconversation.ErrKnowledgeBaseUnavailable, status: http.StatusServiceUnavailable},
	{err: appconversation.ErrKnowledgeBaseNotReady, status: http.StatusConflict},
	{err: appconversation.ErrModelRouteNotConfigured, status: http.StatusServiceUnavailable},
	{err: appconversation.ErrModelAccessDenied, status: http.StatusForbidden},
	{err: appconversation.ErrStorageQuotaExceeded, status: http.StatusConflict},
	{err: appconversation.ErrGeneratedMediaArtifactUnavailable, status: http.StatusBadGateway},
	{err: appconversation.ErrUpstreamEmptyResponse, status: http.StatusBadGateway},
	{err: appconversation.ErrToolRunFinalAnswerMissing, status: http.StatusBadGateway},
	{err: appconversation.ErrMessageGenerationCanceled, status: http.StatusBadRequest},
	{err: appconversation.ErrMessageGenerationInterrupted, status: http.StatusServiceUnavailable},
	{err: appconversation.ErrMediaImagePromptRequired, status: http.StatusBadRequest},
	{err: appconversation.ErrMediaImageGenerationRejectsInputs, status: http.StatusBadRequest},
	{err: appconversation.ErrMediaImageEditInputRequired, status: http.StatusBadRequest},
	{err: appconversation.ErrMediaImageEditTooManyInputs, status: http.StatusBadRequest},
	{err: appconversation.ErrMediaImageEditInputInvalid, status: http.StatusBadRequest},
	{err: appconversation.ErrMediaVideoPromptRequired, status: http.StatusBadRequest},
	{err: appconversation.ErrMediaVideoInputInvalid, status: http.StatusBadRequest},
	{err: appconversation.ErrMediaVideoTooManyInputs, status: http.StatusBadRequest},
	{err: appconversation.ErrMediaRouteProtocolMismatch, status: http.StatusServiceUnavailable},
	{err: appconversation.ErrInvalidMediaGenerationTask, status: http.StatusBadRequest},
	{err: appconversation.ErrDuplicateMessageGenerationRun, status: http.StatusConflict},
	{err: billing.ErrUsageConcurrencyLimitExceeded, status: http.StatusTooManyRequests},
	{err: billing.ErrUsageReservationConflict, status: http.StatusConflict},
	{err: billing.ErrUsageBalanceInsufficient, status: http.StatusPaymentRequired},
	{err: billing.ErrModelPricingRequired, status: http.StatusPaymentRequired},
}

// describeSendMessageError 把消息发送 / 生成 / 计费路径上的错误映射为对外错误描述。
// 上游失败的错误码与文案取决于上游响应特征，单独判定；其余按哨兵表映射；无法归类的错误一律
// 作为内部错误返回，不把内部文案送进推断。
func describeSendMessageError(err error) response.Description {
	switch {
	case appconversation.IsUpstreamRateLimitError(err):
		return response.DescribeCode(http.StatusTooManyRequests, appconversation.MessageErrorCodeUpstreamRateLimited)
	case errors.Is(err, appconversation.ErrUpstreamRequestFailed):
		return describeUpstreamRequestFailure(err)
	}
	for _, entry := range sendMessageErrorStatuses {
		if errors.Is(err, entry.err) {
			return response.Describe(entry.status, entry.err)
		}
	}
	return response.DescribeCode(http.StatusInternalServerError, response.CodeInternal)
}

// describeUpstreamRequestFailure 描述上游请求失败。只有 application 层明确识别的上游场景
// 才使用专用错误码，其余统一返回上游不可用，避免把上游原始文案当作错误码契约。
func describeUpstreamRequestFailure(err error) response.Description {
	code := appconversation.MessageErrorCode(err)
	if code == "" {
		code = response.CodeUpstreamUnavailable
	}
	return response.DescribeCode(http.StatusBadGateway, code)
}

func streamErrorPayload(err error) map[string]any {
	mapped := describeSendMessageError(err)
	payload := map[string]any{
		"type":      "error",
		"status":    mapped.Status,
		"message":   mapped.Message,
		"errorCode": mapped.Code,
	}
	if debug := appconversation.MessageErrorDebug(err); debug != nil {
		payload["debug"] = debug
	}
	return payload
}

// streamErrorPayloadWithResult 在错误事件中保留已持久化的消息结果，供客户端完成临时消息对账。
func streamErrorPayloadWithResult(err error, result *appconversation.SendMessageResult) map[string]any {
	payload := streamErrorPayload(err)
	if result != nil {
		payload["data"] = toSendMessageResponse(result)
	}
	return payload
}

// moderationBlockedStreamPayload is retained for recovery/reconnect assembly only.
// Live streams receive moderation_blocked via OnEvent after ApplyRunBlock commits.
// 此时运行已定稿，可直接按结算结论标注"拦截后上游用量照常计费"。
func moderationBlockedStreamPayload(result *appconversation.SendMessageResult, authorization *domainbilling.UsageAuthorization) map[string]any {
	payload := map[string]any{
		"type": "moderation_blocked",
	}
	if result == nil {
		return payload
	}
	if billedReason := appconversation.ModerationBlockedBilledReason(result, authorization); billedReason != "" {
		payload["billedReason"] = billedReason
	}
	if result.Moderation != nil && result.Moderation.Blocked {
		payload["eventID"] = result.Moderation.EventID
		payload["direction"] = result.Moderation.Direction
		if len(result.Moderation.Categories) > 0 {
			payload["categories"] = result.Moderation.Categories
		}
		return payload
	}
	eventID := strings.TrimSpace(result.AssistantMessage.ModerationEventID)
	if eventID == "" {
		eventID = strings.TrimSpace(result.UserMessage.ModerationEventID)
	}
	direction := "output"
	if strings.EqualFold(strings.TrimSpace(result.UserMessage.Status), "blocked") {
		direction = "input"
	}
	categoriesJSON := result.AssistantMessage.ModerationCategoriesJSON
	if strings.TrimSpace(categoriesJSON) == "" || categoriesJSON == "[]" {
		categoriesJSON = result.UserMessage.ModerationCategoriesJSON
	}
	var categories []string
	_ = json.Unmarshal([]byte(categoriesJSON), &categories)
	payload["eventID"] = eventID
	payload["direction"] = direction
	if len(categories) > 0 {
		payload["categories"] = categories
	}
	return payload
}

func stringParam(c *gin.Context, name string) (string, error) {
	value := strings.TrimSpace(c.Param(name))
	if value == "" {
		return "", errors.New("empty param")
	}
	return value, nil
}

func normalizeConversationStatusFilter(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "archived":
		return "archived"
	case "all":
		return "all"
	default:
		return "active"
	}
}

func normalizeConversationStarredFilter(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "starred":
		return "starred"
	case "unstarred":
		return "unstarred"
	default:
		return "all"
	}
}

func normalizeConversationShareFilter(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "shared":
		return "shared"
	case "unshared":
		return "unshared"
	default:
		return "all"
	}
}

func normalizeFileSort(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "created", "recent":
		return "created"
	case "name":
		return "name"
	case "size":
		return "size"
	case "last_used":
		return "last_used"
	default:
		return "created"
	}
}

func normalizeFileKinds(value string) string {
	if strings.TrimSpace(value) == "" {
		return "all"
	}

	allowed := map[string]struct{}{
		"image":        {},
		"document":     {},
		"spreadsheet":  {},
		"presentation": {},
		"code":         {},
		"pdf":          {},
		"audio":        {},
		"video":        {},
	}

	items := strings.Split(value, ",")
	normalized := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		current := strings.ToLower(strings.TrimSpace(item))
		if current == "" || current == "all" {
			continue
		}
		if _, ok := allowed[current]; !ok {
			continue
		}
		if _, exists := seen[current]; exists {
			continue
		}
		seen[current] = struct{}{}
		normalized = append(normalized, current)
	}
	if len(normalized) == 0 {
		return "all"
	}
	return strings.Join(normalized, ",")
}
