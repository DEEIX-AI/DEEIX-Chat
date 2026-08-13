package contentmoderation

import (
	"errors"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

var (
	ErrSuperAdminRequired    = errors.New("superadmin permission required")
	ErrAdminRequired         = errors.New("admin permission required")
	ErrInvalidConfig         = errors.New("invalid content moderation config")
	ErrServiceConfigRequired = errors.New("content moderation service config and policy are required when enabled")
	ErrInvalidBaseURL        = repository.ErrContentModerationInvalidBaseURL
	ErrInvalidModel          = errors.New("invalid content moderation model")
	ErrInvalidTimeout        = errors.New("content moderation timeout must be between 1 and 60 seconds")
	ErrInvalidConcurrency    = errors.New("content moderation max concurrency must be between 1 and 64")
	ErrInvalidQueueCapacity  = errors.New("content moderation queue capacity must be between 1 and 4096")
	ErrInvalidCategories     = errors.New("invalid content moderation categories")
	ErrInvalidEventFilter    = errors.New("invalid content moderation event filter")
	ErrImageTextOnlyCategory = errors.New("text-only categories cannot be selected for image policies")
	ErrEventNotFound         = errors.New("content moderation event not found")
	ErrProbeFailed           = errors.New("content moderation probe failed")
	ErrQueueFull             = errors.New("content moderation queue is full")
	ErrModerationTimeout     = repository.ErrContentModerationTimeout
	ErrModerationService     = repository.ErrContentModerationService
	ErrModerationRateLimited = repository.ErrContentModerationRateLimited
	ErrModerationInvalidResp = repository.ErrContentModerationInvalidResp
	ErrModerationNetwork     = repository.ErrContentModerationNetwork
	ErrWorkerLost            = errors.New("content moderation worker lost")
	// ErrNonImageAttachment lets image loaders skip ordinary files without
	// turning an inapplicable image policy into a failed-open check.
	ErrNonImageAttachment = errors.New("attachment is not an image")
)
