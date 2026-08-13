package repository

import (
	"context"
	"errors"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
)

var (
	ErrContentModerationInvalidBaseURL = errors.New("invalid content moderation base url")
	ErrContentModerationTimeout        = errors.New("content moderation timed out")
	ErrContentModerationService        = errors.New("content moderation service error")
	ErrContentModerationRateLimited    = errors.New("content moderation rate limited")
	ErrContentModerationInvalidResp    = errors.New("content moderation invalid response")
	ErrContentModerationNetwork        = errors.New("content moderation network error")
)

// ContentModerationProvider is the outbound moderation port implemented by infrastructure.
type ContentModerationProvider interface {
	ValidateBaseURL(raw string) error
	ModerateText(
		ctx context.Context,
		config domaincm.ProviderConfig,
		text string,
		selected []string,
		modality string,
	) (*domaincm.ProviderResponse, error)
	ModerateImages(
		ctx context.Context,
		config domaincm.ProviderConfig,
		images []domaincm.ProviderImage,
		selected []string,
		modality string,
	) (*domaincm.ProviderResponse, error)
}
