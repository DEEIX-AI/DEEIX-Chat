package contentmoderation

import (
	"errors"

	cmport "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/contentmoderation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
)

// 错误码与对外文案是前端本地化依赖的 API 契约（apps/web/i18n/messages/*/errors.json 按错误码取文案），
// 因此用 apperr 随哨兵一起声明，传输层用 response.ErrorFrom 直接读取。
// apperr.NewMasked 的第三个参数保留原有内部文本，供日志与错误链使用。
const (
	codeInvalidConfig  = "content_moderation.invalid_config"
	msgInvalidConfig   = "invalid content moderation config"
	codeConfigRequired = "content_moderation.config_required"
	codeProbeFailed    = "content_moderation.probe_failed"
	codeInvalidQuery   = "request.invalid_query"
)

var (
	ErrSuperAdminRequired    = apperr.New("auth.superadmin_required", "superadmin permission required")
	ErrAdminRequired         = apperr.New("auth.admin_required", "admin permission required")
	ErrInvalidConfig         = apperr.New(codeInvalidConfig, msgInvalidConfig)
	ErrServiceConfigRequired = apperr.New(codeConfigRequired, "content moderation service config and policy are required when enabled")
	ErrInvalidBaseURL        = cmport.ErrInvalidBaseURL
	ErrInvalidModel          = apperr.NewMasked(codeInvalidConfig, msgInvalidConfig, "invalid content moderation model")
	ErrInvalidTimeout        = apperr.NewMasked(codeInvalidConfig, msgInvalidConfig, "content moderation timeout must be between 1 and 60 seconds")
	ErrInvalidConcurrency    = apperr.NewMasked(codeInvalidConfig, msgInvalidConfig, "content moderation max concurrency must be between 1 and 64")
	ErrInvalidQueueCapacity  = apperr.NewMasked(codeInvalidConfig, msgInvalidConfig, "content moderation queue capacity must be between 1 and 4096")
	ErrInvalidCategories     = apperr.NewMasked(codeInvalidConfig, msgInvalidConfig, "invalid content moderation categories")
	ErrInvalidEventFilter    = apperr.NewMasked(codeInvalidQuery, "invalid query parameter", "invalid content moderation event filter")
	ErrImageTextOnlyCategory = apperr.NewMasked(codeInvalidConfig, msgInvalidConfig, "text-only categories cannot be selected for image policies")
	ErrEventNotFound         = apperr.New("content_moderation.event_not_found", "content moderation event not found")
	ErrProbeFailed           = apperr.New(codeProbeFailed, "content moderation probe failed")
	// 以下哨兵仅在应用内部流转（worker 队列、审核结果记录），不会直接写入 API 响应。
	ErrQueueFull             = errors.New("content moderation queue is full")
	ErrModerationTimeout     = cmport.ErrTimeout
	ErrModerationService     = cmport.ErrService
	ErrModerationRateLimited = cmport.ErrRateLimited
	ErrModerationInvalidResp = cmport.ErrInvalidResponse
	ErrModerationNetwork     = cmport.ErrNetwork
	ErrWorkerLost            = errors.New("content moderation worker lost")
	// ErrNonImageAttachment 让图片加载器跳过普通文件，避免
	// 将不适用的图片策略变成 failed-open 检查。
	ErrNonImageAttachment = errors.New("attachment is not an image")
)
