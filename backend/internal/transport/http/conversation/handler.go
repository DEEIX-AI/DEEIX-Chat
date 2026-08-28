package conversation

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/lifecycle"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// Handler 封装会话 HTTP 处理。
type Handler struct {
	service  *appconversation.Service
	cfg      *config.Runtime
	// shutdown 触发时订阅型长连接（run 对账流、run 观看流）立即退出，
	// 让优雅关停不被常驻 SSE 拖到超时；客户端依靠既有重连逻辑恢复。
	shutdown *lifecycle.Shutdown
}

func normalizeStreamEventPayload(eventType string, payload map[string]interface{}) map[string]interface{} {
	normalized := map[string]interface{}{
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
func NewHandler(service *appconversation.Service, cfg *config.Runtime, shutdown *lifecycle.Shutdown) *Handler {
	return &Handler{
		service:  service,
		cfg:      cfg,
		shutdown: shutdown,
	}
}

func (h *Handler) recordAudit(c *gin.Context, action string, resource string, resourceID string, detail interface{}) {
	h.service.RecordAudit(c.Request.Context(), appconversation.AuditInput{
		UserID:     middleware.MustUserID(c),
		RequestID:  middleware.MustRequestID(c),
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		ClientIP:   c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
		Detail:     detail,
	})
}

const (
	defaultHTTPPageSize = 20
	maxHTTPPageSize     = 1000
	maxMessagePageSize  = 1000
)

func pageParams(c *gin.Context) (int, int) {
	return pageParamsWithMax(c, maxHTTPPageSize)
}

func messagePageParams(c *gin.Context) (int, int) {
	return pageParamsWithMax(c, maxMessagePageSize)
}

func pageParamsWithMax(c *gin.Context, maxPageSize int) (int, int) {
	page := 1
	pageSize := defaultHTTPPageSize

	if raw := c.Query("page"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if raw := c.Query("page_size"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			if maxPageSize > 0 && parsed > maxPageSize {
				parsed = maxPageSize
			}
			pageSize = parsed
		}
	}

	return page, pageSize
}

type streamError struct {
	Status  int
	Code    string
	Message string
}

func mapStreamError(err error) streamError {
	status := http.StatusInternalServerError
	code := ""
	message := "send message failed"
	switch {
	case errors.Is(err, appconversation.ErrConversationNotFound):
		status = http.StatusNotFound
		message = "conversation not found"
	case errors.Is(err, appconversation.ErrInvalidFileReference):
		status = http.StatusBadRequest
		message = "invalid file reference"
	case errors.Is(err, appconversation.ErrFileNotFound):
		status = http.StatusNotFound
		message = "file not found"
	case errors.Is(err, appconversation.ErrFileTooLarge):
		status = http.StatusRequestEntityTooLarge
		message = "file too large"
	case errors.Is(err, appconversation.ErrInvalidMessageBranch):
		status = http.StatusBadRequest
		message = "invalid message branch"
	case errors.Is(err, appconversation.ErrTooManyMessageFiles):
		status = http.StatusBadRequest
		message = "too many files in one message"
	case errors.Is(err, appconversation.ErrTooManySelectedTools):
		status = http.StatusBadRequest
		message = "too many selected tools"
	case errors.Is(err, appconversation.ErrMultipleImageAttachmentProcessors):
		status = http.StatusBadRequest
		message = "multiple image attachment processors selected"
	case errors.Is(err, appconversation.ErrImageAttachmentProcessingFailed):
		status = http.StatusBadGateway
		message = "image attachment processing failed"
	case errors.Is(err, appconversation.ErrTooManySelectedSkills):
		status = http.StatusBadRequest
		message = "too many selected skills"
	case errors.Is(err, appconversation.ErrSkillNotFound):
		status = http.StatusNotFound
		message = "skill not found"
	case errors.Is(err, appconversation.ErrInvalidSkillUse):
		status = http.StatusBadRequest
		message = "invalid skill use"
	case errors.Is(err, appconversation.ErrFileProcessingNotReady):
		status = http.StatusBadRequest
		message = "file processing not ready"
	case errors.Is(err, appconversation.ErrFileTooLargeForFullContext):
		status = http.StatusBadRequest
		message = "file too large for full context"
	case errors.Is(err, appconversation.ErrEmbeddingUnavailable):
		status = http.StatusBadRequest
		message = "embedding unavailable for current file capability"
	case errors.Is(err, appconversation.ErrModelRouteNotConfigured):
		status = http.StatusServiceUnavailable
		message = "model route not configured"
	case errors.Is(err, appconversation.ErrGeneratedMediaArtifactUnavailable):
		status = http.StatusBadGateway
		code = appconversation.MessageErrorCode(err)
		message = "generated media artifact is temporarily unavailable"
	case errors.Is(err, appconversation.ErrUpstreamEmptyResponse):
		status = http.StatusBadGateway
		message = "model returned empty response"
	case errors.Is(err, appconversation.ErrMessageGenerationCanceled):
		status = http.StatusBadRequest
		message = "message generation canceled"
	case appconversation.IsUpstreamRateLimitError(err):
		status = http.StatusTooManyRequests
		code = appconversation.MessageErrorCodeUpstreamRateLimited
		message = "upstream rate limited"
	case errors.Is(err, appconversation.ErrMediaImagePromptRequired):
		status = http.StatusBadRequest
		message = "image prompt is required"
	case errors.Is(err, appconversation.ErrMediaImageGenerationRejectsInputs):
		status = http.StatusBadRequest
		message = "image generation does not accept input images"
	case errors.Is(err, appconversation.ErrMediaImageEditInputRequired):
		status = http.StatusBadRequest
		message = "image edit requires at least one input image"
	case errors.Is(err, appconversation.ErrMediaImageEditTooManyInputs):
		status = http.StatusBadRequest
		message = "too many image edit input images"
	case errors.Is(err, appconversation.ErrMediaImageEditInputInvalid):
		status = http.StatusBadRequest
		message = "image edit input image is invalid"
	case errors.Is(err, appconversation.ErrMediaVideoPromptRequired):
		status = http.StatusBadRequest
		message = "video prompt is required"
	case errors.Is(err, appconversation.ErrMediaVideoInputInvalid):
		status = http.StatusBadRequest
		message = "video generation input is invalid"
	case errors.Is(err, appconversation.ErrMediaVideoTooManyInputs):
		status = http.StatusBadRequest
		message = "too many video generation input images"
	case errors.Is(err, appconversation.ErrMediaRouteProtocolMismatch):
		status = http.StatusServiceUnavailable
		message = "media route protocol does not match task"
	case errors.Is(err, appconversation.ErrInvalidMediaGenerationTask):
		status = http.StatusBadRequest
		message = "invalid media generation task"
	case errors.Is(err, appconversation.ErrDuplicateMessageGenerationRun):
		status = http.StatusConflict
		message = "message generation run already exists"
	case errors.Is(err, appconversation.ErrUpstreamRequestFailed):
		status = http.StatusBadGateway
		code = appconversation.MessageErrorCode(err)
		message = mapClientErrorMessage(err)
	}
	if code == "" {
		code = response.InferErrorCode(status, message)
	}
	return streamError{
		Status:  status,
		Code:    code,
		Message: response.PublicErrorMessage(status, code, message),
	}
}

func streamErrorPayload(err error) map[string]interface{} {
	mapped := mapStreamError(err)
	payload := map[string]interface{}{
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

func streamErrorPayloadWithCode(code string, message string) map[string]interface{} {
	return map[string]interface{}{
		"type":      "error",
		"message":   message,
		"errorCode": code,
	}
}

// moderationBlockedStreamPayload is retained for recovery/reconnect assembly only.
// Live streams receive moderation_blocked via OnEvent after ApplyRunBlock commits.
func moderationBlockedStreamPayload(result *appconversation.SendMessageResult) map[string]interface{} {
	payload := map[string]interface{}{
		"type": "moderation_blocked",
	}
	if result == nil {
		return payload
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

func mapClientErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, appconversation.ErrUpstreamEmptyResponse) {
		return "model returned empty response"
	}
	if errors.Is(err, appconversation.ErrGeneratedMediaArtifactUnavailable) {
		return "generated media artifact is temporarily unavailable"
	}
	if appconversation.IsUpstreamRateLimitError(err) {
		return "upstream rate limited"
	}
	if errors.Is(err, appconversation.ErrUpstreamRequestFailed) {
		detail := appconversation.MessageErrorSummary(err)
		if detail != "" && detail != appconversation.ErrUpstreamRequestFailed.Error() {
			return detail
		}
		return "model request failed"
	}
	return strings.TrimSpace(err.Error())
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

func normalizeConversationProjectQuery(value string) string {
	normalized := strings.TrimSpace(value)
	switch normalized {
	case "", "all":
		return "all"
	case "unassigned":
		return "unassigned"
	default:
		return normalized
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
