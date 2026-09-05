package billing

import (
	"errors"
	"net/http"
	"strings"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

// GetOpenRouterOfficialPricing godoc
// @Summary 管理员获取 OpenRouter 官方模型目录
// @Description 从 storage 缓存读取 OpenRouter 模型标识、定价和上下文限制；缓存不存在、过期或 refresh=true 时由后端刷新。
// @Tags admin-billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param refresh query bool false "强制刷新缓存"
// @Success 200 {object} OpenRouterOfficialPricingResponseDoc
// @Failure 500 {object} ErrorDoc
// @Failure 502 {object} ErrorDoc
// @Router /admin/billing/official-pricing/openrouter [get]
func (h *Handler) GetOpenRouterOfficialPricing(c *gin.Context) {
	refresh := strings.EqualFold(strings.TrimSpace(c.Query("refresh")), "true")
	if h.officialPricing == nil {
		response.InternalError(c)
		return
	}
	result, err := h.officialPricing.GetOpenRouterOfficialPricing(c.Request.Context(), refresh)
	if err != nil {
		if errors.Is(err, appbilling.ErrOfficialPricingCacheUnavailable) ||
			errors.Is(err, appbilling.ErrOfficialPricingCacheReadFailed) ||
			errors.Is(err, appbilling.ErrOfficialPricingCacheWriteFailed) {
			response.InternalError(c)
		} else {
			response.ErrorFrom(c, http.StatusBadGateway, errUpstreamServiceUnavailable)
		}
		return
	}
	response.Success(c, OpenRouterOfficialPricingDataResponse{
		FetchedAt: result.FetchedAt,
		Cached:    result.Cached,
		Stale:     result.Stale,
		Items:     toOpenRouterOfficialPricingResponses(result.Items),
	})
}

func toOpenRouterOfficialPricingResponses(items []appbilling.OfficialPricingItem) []OpenRouterOfficialPricingItemResponse {
	result := make([]OpenRouterOfficialPricingItemResponse, 0, len(items))
	for _, item := range items {
		result = append(result, OpenRouterOfficialPricingItemResponse{
			ID:                  item.ID,
			CanonicalSlug:       item.CanonicalSlug,
			Name:                item.Name,
			ContextLength:       item.ContextLength,
			MaxCompletionTokens: item.MaxCompletionTokens,
			Pricing: OpenRouterOfficialPricingUnitPricingResponse{
				Prompt:          item.Pricing.Prompt,
				Completion:      item.Pricing.Completion,
				InputCacheRead:  item.Pricing.InputCacheRead,
				InputCacheWrite: item.Pricing.InputCacheWrite,
			},
		})
	}
	return result
}
