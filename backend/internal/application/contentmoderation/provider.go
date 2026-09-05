package contentmoderation

import (
	"context"
	"strings"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
)

// Provider 是审核服务依赖的出站审核端口，由 infra/contentmoderation 以相同签名实现。
type Provider interface {
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

type ProviderConfig = domaincm.ProviderConfig
type ProviderImage = domaincm.ProviderImage
type CategoryResult = domaincm.CategoryResult
type Response = domaincm.ProviderResponse
type HitEvaluation = domaincm.HitEvaluation

func EvaluateHit(response *Response, selected []string, expectedModality string) HitEvaluation {
	return domaincm.EvaluateHit(response, selected, expectedModality)
}

func maskAPIKey(key string) string {
	value := strings.TrimSpace(key)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "****"
	}
	return value[:4] + "..." + value[len(value)-4:]
}
