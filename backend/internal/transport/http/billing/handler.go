package billing

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// Handler 封装计费 HTTP 处理。
type Handler struct {
	service  *appbilling.Service
	settings *appsettings.Service
	cfg      *config.Runtime
}

// NewHandler 创建处理器。
func NewHandler(service *appbilling.Service, settingsService *appsettings.Service, cfg *config.Runtime) *Handler {
	return &Handler{
		service:  service,
		settings: settingsService,
		cfg:      cfg,
	}
}

func (h *Handler) recordAudit(c *gin.Context, userID uint, action string, resource string, resourceID string, detail interface{}) {
	h.service.RecordAudit(c.Request.Context(), appbilling.AuditInput{
		UserID:     userID,
		RequestID:  middleware.MustRequestID(c),
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		ClientIP:   c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
		Detail:     detail,
	})
}

// GetBillingConfig godoc
// @Summary 管理员查询计费配置
// @Description 查询当前全局计费模式
// @Tags admin-billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} BillingConfigResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/billing/config [get]
func (h *Handler) GetBillingConfig(c *gin.Context) {
	config, err := h.loadBillingConfig(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "get billing config failed")
		return
	}
	response.Success(c, BillingConfigDataResponse{Config: config})
}

func (h *Handler) loadBillingConfig(ctx context.Context) (BillingConfigResponse, error) {
	mode := "self"
	prepaidAmountUSD := 0.0
	nativeToolBillingEnabled := true
	nativeToolPricingJSON := ""
	paymentProviders := []string{}
	usdToCNYRate := 7.2
	epayTypes := defaultEPayTypes()
	if h.settings != nil {
		items, err := h.settings.ListByNamespace(ctx, "billing")
		if err != nil {
			return BillingConfigResponse{}, err
		}
		for _, item := range items {
			value := strings.TrimSpace(item.Value)
			switch item.Key {
			case "mode":
				if value != "" {
					mode = value
				}
			case "prepaid_amount_usd":
				if parsed, parseErr := strconv.ParseFloat(value, 64); parseErr == nil && parsed >= 0 {
					prepaidAmountUSD = parsed
				}
			case "native_tool_billing_enabled":
				if parsed, parseErr := strconv.ParseBool(value); parseErr == nil {
					nativeToolBillingEnabled = parsed
				}
			case "native_tool_pricing_json":
				nativeToolPricingJSON = value
			case "payment_providers":
				paymentProviders = normalizePaymentProviders(value)
			case "usd_to_cny_rate":
				if parsed, parseErr := strconv.ParseFloat(value, 64); parseErr == nil && parsed > 0 {
					usdToCNYRate = parsed
				}
			case "epay_types":
				epayTypes = normalizeEPayTypes(value)
			}
		}
	}
	nativeToolPricing, err := h.service.ListNativeToolPricing(ctx, nativeToolPricingJSON)
	if err != nil {
		return BillingConfigResponse{}, err
	}
	return BillingConfigResponse{
		Mode:                     mode,
		PrepaidAmountUSD:         prepaidAmountUSD,
		PrepaidAmountNanousd:     usdToNanousd(prepaidAmountUSD),
		NativeToolBillingEnabled: nativeToolBillingEnabled,
		NativeToolPricing:        toNativeToolPricingResponses(nativeToolPricing),
		PaymentProviders:         paymentProviders,
		USDToCNYRate:             usdToCNYRate,
		EPayTypes:                epayTypes,
	}, nil
}

// PatchBillingConfig godoc
// @Summary 管理员更新计费配置
// @Description 更新当前全局计费模式
// @Tags admin-billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body BillingConfigRequest true "计费配置"
// @Success 200 {object} BillingConfigResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/billing/config [patch]
func (h *Handler) PatchBillingConfig(c *gin.Context) {
	var req BillingConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	mode := strings.TrimSpace(req.Mode)
	if h.settings == nil {
		response.Error(c, http.StatusInternalServerError, "settings service unavailable")
		return
	}
	patches := []appsettings.PatchItem{
		{Namespace: "billing", Key: "mode", Value: mode},
	}
	if req.PrepaidAmountUSD != nil {
		patches = append(patches, appsettings.PatchItem{
			Namespace: "billing",
			Key:       "prepaid_amount_usd",
			Value:     strconv.FormatFloat(*req.PrepaidAmountUSD, 'f', -1, 64),
		})
	}
	if req.NativeToolBillingEnabled != nil {
		patches = append(patches, appsettings.PatchItem{
			Namespace: "billing",
			Key:       "native_tool_billing_enabled",
			Value:     strconv.FormatBool(*req.NativeToolBillingEnabled),
		})
	}
	if req.NativeToolPricing != nil {
		value, err := h.service.NormalizeNativeToolPricingJSON(c.Request.Context(), nativeToolPricingOverridesFromRequests(req.NativeToolPricing))
		if err != nil {
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		}
		patches = append(patches, appsettings.PatchItem{
			Namespace: "billing",
			Key:       "native_tool_pricing_json",
			Value:     value,
		})
	}
	if _, err := h.settings.BatchUpdate(c.Request.Context(), patches); err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}

	userID := middleware.MustUserID(c)
	h.recordAudit(
		c,
		userID,
		"update_billing_config",
		"billing_config",
		"mode",
		map[string]interface{}{
			"mode":                        mode,
			"prepaid_amount_usd":          req.PrepaidAmountUSD,
			"native_tool_billing_enabled": req.NativeToolBillingEnabled,
			"native_tool_pricing_updated": req.NativeToolPricing != nil,
		},
	)

	config, err := h.loadBillingConfig(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "get billing config failed")
		return
	}
	response.Success(c, BillingConfigDataResponse{Config: config})
}

// ListPlans godoc
// @Summary 获取订阅套餐
// @Description 查询所有启用的订阅套餐及价格
// @Tags billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} PlanListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /billing/plans [get]
func (h *Handler) ListPlans(c *gin.Context) {
	items, err := h.service.ListPlans(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list plans failed")
		return
	}
	response.Success(c, toPlanListResponse(items))
}

// GetBillingAccount godoc
// @Summary 获取按量计费账户
// @Description 查询当前用户按量余额
// @Tags billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} BillingAccountResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /billing/account [get]
func (h *Handler) GetBillingAccount(c *gin.Context) {
	account, err := h.service.GetBillingAccount(c.Request.Context(), middleware.MustUserID(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "get billing account failed")
		return
	}
	response.Success(c, BillingAccountDataResponse{Account: toBillingAccountResponse(account)})
}

// UpdateBillingAccountBalance godoc
// @Summary 管理员设置用户按量余额
// @Description 设置指定用户的按量计费余额，金额单位为美元
// @Tags admin-billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user_id path int true "用户ID"
// @Param body body UpdateBillingAccountBalanceRequest true "余额"
// @Success 200 {object} BillingAccountResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/billing/accounts/{user_id}/balance [patch]
func (h *Handler) UpdateBillingAccountBalance(c *gin.Context) {
	targetUserID, err := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if err != nil || targetUserID == 0 {
		response.Error(c, http.StatusBadRequest, "invalid user id")
		return
	}
	var req UpdateBillingAccountBalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	actorUserID := middleware.MustUserID(c)
	account, err := h.service.SetBillingAccountBalance(c.Request.Context(), appbilling.BillingAccountBalanceInput{
		UserID:      uint(targetUserID),
		BalanceUSD:  req.BalanceUSD,
		RefNo:       middleware.MustRequestID(c),
		Description: req.Description,
	})
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	h.recordAudit(
		c,
		actorUserID,
		"update_billing_balance",
		"billing_account",
		strconv.FormatUint(targetUserID, 10),
		map[string]interface{}{
			"user_id":     targetUserID,
			"balance_usd": req.BalanceUSD,
		},
	)
	response.Success(c, BillingAccountDataResponse{Account: toBillingAccountResponse(account)})
}

// ListRedemptionCodes godoc
// @Summary 管理员查询兑换码
// @Description 分页查询计费兑换码配置
// @Tags admin-billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param mode query string false "计费模式：usage/period"
// @Param status query string false "状态：active/inactive"
// @Param availability query string false "可兑换性：available/expired/exhausted"
// @Param q query string false "搜索关键词"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} RedemptionCodeListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/billing/redemption-codes [get]
func (h *Handler) ListRedemptionCodes(c *gin.Context) {
	page, pageSize := pageParams(c)
	items, total, err := h.service.ListRedemptionCodes(c.Request.Context(), appbilling.RedemptionCodeListInput{
		Mode:         c.Query("mode"),
		Status:       c.Query("status"),
		Availability: c.Query("availability"),
		Query:        c.Query("q"),
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list redemption codes failed")
		return
	}
	response.SuccessPage(c, total, toRedemptionCodeResponses(items))
}

// CreateRedemptionCodes godoc
// @Summary 管理员创建兑换码
// @Description 创建手动兑换码或随机兑换码，明文只在创建响应中返回
// @Tags admin-billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateRedemptionCodeRequest true "兑换码配置"
// @Success 200 {object} RedemptionCodeCreateResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/billing/redemption-codes [post]
func (h *Handler) CreateRedemptionCodes(c *gin.Context) {
	var req CreateRedemptionCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	actorUserID := middleware.MustUserID(c)
	items, err := h.service.CreateRedemptionCodes(c.Request.Context(), actorUserID, appbilling.RedemptionCodeInput{
		Code:           req.Code,
		Quantity:       req.Quantity,
		Mode:           req.Mode,
		CreditUSD:      req.CreditUSD,
		PlanID:         req.PlanID,
		DurationDays:   req.DurationDays,
		MaxRedemptions: req.MaxRedemptions,
		PerUserLimit:   req.PerUserLimit,
		ExpiresAt:      req.ExpiresAt,
		Description:    req.Description,
	})
	if err != nil {
		writeRedemptionCodeError(c, err)
		return
	}
	h.recordAudit(
		c,
		actorUserID,
		"create_redemption_code",
		"billing_redemption_code",
		req.Mode,
		map[string]interface{}{
			"mode":            req.Mode,
			"quantity":        req.Quantity,
			"credit_usd":      req.CreditUSD,
			"plan_id":         req.PlanID,
			"duration_days":   req.DurationDays,
			"max_redemptions": req.MaxRedemptions,
			"per_user_limit":  req.PerUserLimit,
		},
	)
	response.Success(c, RedemptionCodeCreateDataResponse{Results: toRedemptionCodeResponses(items)})
}

func writeRedemptionCodeError(c *gin.Context, err error) {
	var validationErr appbilling.RedemptionCodeValidationError
	if errors.As(err, &validationErr) {
		response.ErrorWithDetails(c, http.StatusBadRequest, "billing.invalid_redemption_code", err.Error(), validationErr)
		return
	}
	if errors.Is(err, appbilling.ErrRedemptionCodeConflict) {
		response.ErrorFrom(c, http.StatusConflict, err)
		return
	}
	if errors.Is(err, appbilling.ErrRedemptionCodeUnavailable) {
		response.ErrorFrom(c, http.StatusNotFound, err)
		return
	}
	if errors.Is(err, appbilling.ErrInvalidRedemptionCode) {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	if errors.Is(err, appbilling.ErrRedemptionCodeHashUnavailable) {
		response.ErrorFrom(c, http.StatusInternalServerError, err)
		return
	}
	response.Error(c, http.StatusInternalServerError, "redemption code operation failed")
}

// RevealRedemptionCode godoc
// @Summary 管理员按需复制兑换码明文
// @Description 解密单个兑换码明文用于复制；列表接口不会返回明文
// @Tags admin-billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "兑换码ID"
// @Success 200 {object} RedemptionCodeResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/billing/redemption-codes/{id}/code [get]
func (h *Handler) RevealRedemptionCode(c *gin.Context) {
	codeID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || codeID == 0 {
		response.Error(c, http.StatusBadRequest, "invalid redemption code id")
		return
	}
	item, err := h.service.RevealRedemptionCode(c.Request.Context(), uint(codeID))
	if err != nil {
		if errors.Is(err, appbilling.ErrRedemptionCodeUnavailable) {
			response.Error(c, http.StatusNotFound, "redemption code not found")
			return
		}
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	actorUserID := middleware.MustUserID(c)
	h.recordAudit(
		c,
		actorUserID,
		"reveal_redemption_code",
		"billing_redemption_code",
		strconv.FormatUint(codeID, 10),
		map[string]interface{}{"code_hint": item.CodeHint},
	)
	c.Header("Cache-Control", "no-store")
	response.Success(c, RedemptionCodeDataResponse{Code: toRedemptionCodeResponse(*item)})
}

// PatchRedemptionCode godoc
// @Summary 管理员更新兑换码
// @Description 更新兑换码状态、次数限制、过期时间和说明，不允许修改奖励本身
// @Tags admin-billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "兑换码ID"
// @Param body body PatchRedemptionCodeRequestDoc true "兑换码更新字段"
// @Success 200 {object} RedemptionCodeResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/billing/redemption-codes/{id} [patch]
func (h *Handler) PatchRedemptionCode(c *gin.Context) {
	codeID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || codeID == 0 {
		response.Error(c, http.StatusBadRequest, "invalid redemption code id")
		return
	}
	var req PatchRedemptionCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.UpdateRedemptionCode(c.Request.Context(), uint(codeID), appbilling.RedemptionCodeUpdateInput{
		Status:            req.Status,
		MaxRedemptionsSet: req.MaxRedemptions.Set,
		MaxRedemptions:    req.MaxRedemptions.Value,
		PerUserLimit:      req.PerUserLimit,
		ExpiresAtSet:      req.ExpiresAt.Set,
		ExpiresAt:         req.ExpiresAt.Value,
		Description:       req.Description,
	})
	if err != nil {
		writeRedemptionCodeError(c, err)
		return
	}
	actorUserID := middleware.MustUserID(c)
	h.recordAudit(
		c,
		actorUserID,
		"update_redemption_code",
		"billing_redemption_code",
		strconv.FormatUint(codeID, 10),
		map[string]interface{}{
			"status":          req.Status,
			"max_redemptions": req.MaxRedemptions.Value,
			"per_user_limit":  req.PerUserLimit,
		},
	)
	response.Success(c, RedemptionCodeDataResponse{Code: toRedemptionCodeResponse(*item)})
}

// DeleteRedemptionCode godoc
// @Summary 管理员删除兑换码
// @Description 软删除兑换码，历史兑换记录保留，删除后不可再兑换
// @Tags admin-billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "兑换码ID"
// @Success 200 {object} RedemptionCodeDeleteResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/billing/redemption-codes/{id} [delete]
func (h *Handler) DeleteRedemptionCode(c *gin.Context) {
	codeID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || codeID == 0 {
		response.Error(c, http.StatusBadRequest, "invalid redemption code id")
		return
	}
	if err := h.service.DeleteRedemptionCode(c.Request.Context(), uint(codeID)); err != nil {
		if errors.Is(err, appbilling.ErrRedemptionCodeUnavailable) {
			response.Error(c, http.StatusNotFound, "redemption code not found")
			return
		}
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	actorUserID := middleware.MustUserID(c)
	h.recordAudit(
		c,
		actorUserID,
		"delete_redemption_code",
		"billing_redemption_code",
		strconv.FormatUint(codeID, 10),
		map[string]interface{}{"deleted": true},
	)
	response.Success(c, RedemptionCodeDeleteDataResponse{Deleted: true})
}

// BatchDeleteRedemptionCodes godoc
// @Summary 管理员批量删除兑换码
// @Description 批量软删除兑换码，历史兑换记录保留，删除后不可再兑换
// @Tags admin-billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body BatchDeleteRedemptionCodeRequest true "批量删除请求"
// @Success 200 {object} BatchDeleteRedemptionCodeResponseDoc
// @Failure 400 {object} ErrorDoc
// @Router /admin/billing/redemption-codes/batch-delete [post]
func (h *Handler) BatchDeleteRedemptionCodes(c *gin.Context) {
	var req BatchDeleteRedemptionCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	result := h.service.BatchDeleteRedemptionCodes(c.Request.Context(), req.IDs)
	actorUserID := middleware.MustUserID(c)
	h.recordAudit(
		c,
		actorUserID,
		"batch_delete_redemption_codes",
		"billing_redemption_code",
		"batch",
		map[string]interface{}{
			"ids":           req.IDs,
			"success_count": result.SuccessCount,
			"not_found":     result.NotFoundCount,
			"failed":        result.FailedCount,
		},
	)
	response.Success(c, toBatchDeleteRedemptionCodeResponse(*result))
}

// RedeemCode godoc
// @Summary 兑换计费权益码
// @Description 当前用户兑换余额或订阅权益
// @Tags billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body RedeemCodeRequest true "兑换码"
// @Success 200 {object} RedemptionApplyResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /billing/redemptions [post]
func (h *Handler) RedeemCode(c *gin.Context) {
	var req RedeemCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	userID := middleware.MustUserID(c)
	result, err := h.service.RedeemCode(c.Request.Context(), userID, req.Code)
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	overview, err := h.service.GetBillingOverview(c.Request.Context(), userID, time.Now())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "get billing overview failed")
		return
	}
	var account *BillingAccountResponse
	if result.Account != nil {
		value := toBillingAccountResponse(result.Account)
		account = &value
	}
	var subscription *SubscriptionResponse
	if result.Subscription != nil {
		value := toSubscriptionResponse(result.Subscription)
		subscription = &value
	}
	response.Success(c, RedemptionApplyDataResponse{
		Redemption:   toRedemptionResponse(*result),
		Account:      account,
		Subscription: subscription,
		Overview:     toBillingOverviewResponse(overview),
	})
}

// GetBillingOverview godoc
// @Summary 获取当前用户计费概览
// @Description 查询当前计费方式、周期额度或按量余额
// @Tags billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} BillingOverviewResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /billing/overview [get]
func (h *Handler) GetBillingOverview(c *gin.Context) {
	overview, err := h.service.GetBillingOverview(c.Request.Context(), middleware.MustUserID(c), time.Now())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "get billing overview failed")
		return
	}
	response.Success(c, BillingOverviewDataResponse{Overview: toBillingOverviewResponse(overview)})
}

// UpdatePlan godoc
// @Summary 管理员更新周期套餐
// @Description 更新周期套餐基础配置与默认价格
// @Tags admin-billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "套餐ID"
// @Param body body UpdateBillingPlanRequest true "套餐配置"
// @Success 200 {object} BillingPlanResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/billing/plans/{id} [patch]
func (h *Handler) UpdatePlan(c *gin.Context) {
	planID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || planID == 0 {
		response.Error(c, http.StatusBadRequest, "invalid plan id")
		return
	}

	var req UpdateBillingPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.UpdatePlan(c.Request.Context(), uint(planID), planUpdateInputFromRequest(req))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "update billing plan failed")
		return
	}

	userID := middleware.MustUserID(c)
	h.recordAudit(
		c,
		userID,
		"update_billing_plan",
		"billing_plan",
		strconv.FormatUint(planID, 10),
		map[string]interface{}{
			"plan_id":           planID,
			"name":              req.Name,
			"period_credit_usd": req.PeriodCreditUSD,
			"discount_percent":  req.DiscountPercent,
			"amount_usd":        req.AmountUSD,
			"billing_interval":  req.BillingInterval,
		},
	)

	response.Success(c, BillingPlanDataResponse{Plan: toPlanListResponse([]appbilling.BillingPlanView{*item})[0]})
}

// Subscribe godoc
// @Summary 创建订阅
// @Description 为当前用户创建或替换订阅
// @Tags billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body SubscribeRequest true "订阅参数"
// @Success 200 {object} SubscribeResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /billing/subscriptions [post]
func (h *Handler) Subscribe(c *gin.Context) {
	userID := middleware.MustUserID(c)

	var req SubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.Subscribe(c.Request.Context(), userID, req.PriceID, req.Cycles)
	if err != nil {
		if errors.Is(err, appbilling.ErrPaymentRequired) {
			response.Error(c, http.StatusBadRequest, "payment is required")
			return
		}
		response.Error(c, http.StatusInternalServerError, "subscribe failed")
		return
	}

	h.recordAudit(
		c,
		userID,
		"subscribe",
		"billing",
		strconv.FormatUint(uint64(item.ID), 10),
		map[string]interface{}{
			"price_id": req.PriceID,
			"cycles":   req.Cycles,
		},
	)

	response.Success(c, SubscriptionDataResponse{
		Subscription: toSubscriptionResponse(item),
	})
}

// ListUsage godoc
// @Summary 查询用量账单
// @Description 查询当前用户的每日用量与费用
// @Tags billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param query query string false "搜索模型"
// @Param status query string false "状态筛选：free/billable"
// @Param sort query string false "排序：newest/oldest/tokens_desc/cost_desc/latency_desc"
// @Success 200 {object} UsageLedgerListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /billing/usage [get]
func (h *Handler) ListUsage(c *gin.Context) {
	userID := middleware.MustUserID(c)
	page, pageSize := pageParams(c)
	filter := appbilling.UsageListFilter{
		Query:  c.Query("query"),
		Status: c.Query("status"),
		Sort:   c.Query("sort"),
	}

	items, total, err := h.service.ListUsage(c.Request.Context(), userID, page, pageSize, filter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list usage failed")
		return
	}
	usages := make([]UsageLedgerResponse, 0, len(items))
	for _, u := range items {
		usages = append(usages, toUsageLedgerResponse(u))
	}
	response.SuccessPage(c, total, usages)
}

// ListMonthlyUsage godoc
// @Summary 查询月度用量
// @Description 查询当前用户按月份聚合的用量与费用
// @Tags billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param months query int false "月份数量，默认近 12 个月"
// @Success 200 {object} UsageMonthlyListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /billing/usage/monthly [get]
func (h *Handler) ListMonthlyUsage(c *gin.Context) {
	userID := middleware.MustUserID(c)
	months, err := strconv.Atoi(c.DefaultQuery("months", "12"))
	if err != nil || months <= 0 {
		months = 12
	}

	items, err := h.service.ListMonthlyUsage(c.Request.Context(), userID, months)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list monthly usage failed")
		return
	}
	results := make([]UsageMonthlyResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toUsageMonthlyResponse(item))
	}
	response.Success(c, results)
}

// ListDailyUsage godoc
// @Summary 查询每日用量
// @Description 查询当前用户按日期聚合的用量与费用
// @Tags billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param days query int false "天数"
// @Success 200 {object} UsageDailyListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /billing/usage/daily [get]
func (h *Handler) ListDailyUsage(c *gin.Context) {
	userID := middleware.MustUserID(c)
	startDateText := strings.TrimSpace(c.Query("start_date"))
	endDateText := strings.TrimSpace(c.Query("end_date"))
	if startDateText != "" && endDateText != "" {
		startDate, startErr := time.Parse("2006-01-02", startDateText)
		endDate, endErr := time.Parse("2006-01-02", endDateText)
		if startErr != nil || endErr != nil {
			response.Error(c, http.StatusBadRequest, "invalid daily usage date range")
			return
		}
		items, err := h.service.ListDailyUsageRange(c.Request.Context(), userID, startDate, endDate)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "list daily usage failed")
			return
		}
		results := make([]UsageDailyResponse, 0, len(items))
		for _, item := range items {
			results = append(results, toUsageDailyResponse(item))
		}
		response.Success(c, results)
		return
	}

	daysText := strings.TrimSpace(c.Query("days"))
	if daysText == "" {
		items, err := h.service.ListCurrentCycleDailyUsage(c.Request.Context(), userID, time.Now())
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "list daily usage failed")
			return
		}
		results := make([]UsageDailyResponse, 0, len(items))
		for _, item := range items {
			results = append(results, toUsageDailyResponse(item))
		}
		response.Success(c, results)
		return
	}

	days, err := strconv.Atoi(daysText)
	if err != nil || days <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid daily usage days")
		return
	}
	items, err := h.service.ListDailyUsage(c.Request.Context(), userID, days, time.Now())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list daily usage failed")
		return
	}
	results := make([]UsageDailyResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toUsageDailyResponse(item))
	}
	response.Success(c, results)
}

// ListModelPricing godoc
// @Summary 管理员查询模型按量单价
// @Description 按平台模型名查询模型按量计费配置
// @Tags admin-billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param q query string false "搜索关键词"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} ModelPricingListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/billing/model-prices [get]
func (h *Handler) ListModelPricing(c *gin.Context) {
	page, pageSize := pageParams(c)
	items, total, err := h.service.ListModelPricing(
		c.Request.Context(),
		c.Query("q"),
		page,
		pageSize,
	)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list model pricing failed")
		return
	}
	results := make([]ModelPricingResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toModelPricingResponse(item))
	}
	response.SuccessPage(c, total, results)
}

// UpsertModelPricing godoc
// @Summary 管理员保存模型按量单价
// @Description 按平台模型名创建或更新模型按量计费配置，金额单位为美元
// @Tags admin-billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body UpsertModelPricingRequest true "模型单价"
// @Success 200 {object} ModelPricingDataResponse
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/billing/model-prices [put]
func (h *Handler) UpsertModelPricing(c *gin.Context) {
	var req UpsertModelPricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	platformModelName := strings.TrimSpace(req.PlatformModelName)
	if platformModelName == "" {
		response.Error(c, http.StatusBadRequest, "platform model name is required")
		return
	}
	req.PlatformModelName = platformModelName

	item, err := h.service.UpsertModelPricing(c.Request.Context(), modelPricingInputFromRequest(req))
	if err != nil {
		if errors.Is(err, appbilling.ErrInvalidModelPricing) {
			response.Error(c, http.StatusBadRequest, "invalid model pricing")
			return
		}
		response.Error(c, http.StatusInternalServerError, "save model pricing failed")
		return
	}

	userID := middleware.MustUserID(c)
	h.recordAudit(
		c,
		userID,
		"upsert_model_pricing",
		"billing_model_price",
		platformModelName,
		map[string]interface{}{
			"platform_model_name": platformModelName,
			"currency":            req.Currency,
		},
	)

	response.Success(c, ModelPricingDataResponse{
		ModelPricing: toModelPricingResponse(*item),
	})
}

func pageParams(c *gin.Context) (int, int) {
	page := 1
	pageSize := 20

	if raw := c.Query("page"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if raw := c.Query("page_size"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			if parsed > 100 {
				parsed = 100
			}
			pageSize = parsed
		}
	}

	return page, pageSize
}
