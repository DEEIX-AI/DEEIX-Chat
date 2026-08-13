package contentmoderation

import (
	"strings"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type Provider = repository.ContentModerationProvider
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
