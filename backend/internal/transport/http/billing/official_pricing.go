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
// @Description 从 storage 缓存读取 OpenRouter 模型标识、基础定价、输入 token 阶梯覆盖和上下文限制；无法映射到当前 token 计费模型的附加字段会在 unsupportedFields 中标记，快速配置会忽略这些字段并继续导入可识别的 token 价格。由原生工具计费负责的按次字段（例如 web_search）会被忽略。
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
				Prompt:               item.Pricing.Prompt,
				Completion:           item.Pricing.Completion,
				InputCacheRead:       item.Pricing.InputCacheRead,
				InputCacheWrite:      item.Pricing.InputCacheWrite,
				CacheWritePriceBasis: item.Pricing.CacheWritePriceBasis,
				Overrides:            toOpenRouterOfficialPricingOverrides(item.Pricing.Overrides),
				UnsupportedFields:    append([]string(nil), item.Pricing.UnsupportedFields...),
			},
		})
	}
	return result
}

func toOpenRouterOfficialPricingOverrides(overrides []appbilling.OfficialPricingOverride) []OpenRouterOfficialPricingOverrideResponse {
	if len(overrides) == 0 {
		return nil
	}
	result := make([]OpenRouterOfficialPricingOverrideResponse, 0, len(overrides))
	for _, override := range overrides {
		result = append(result, OpenRouterOfficialPricingOverrideResponse{
			MinPromptTokens: override.MinPromptTokens,
			Prompt:          override.Prompt,
			Completion:      override.Completion,
			InputCacheRead:  override.InputCacheRead,
			InputCacheWrite: override.InputCacheWrite,
		})
	}
	return result
}
