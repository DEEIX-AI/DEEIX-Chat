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
	return BillingConfigResponse{
		Mode:                 mode,
		PrepaidAmountUSD:     prepaidAmountUSD,
		PrepaidAmountNanousd: usdToNanousd(prepaidAmountUSD),
		PaymentProviders:     paymentProviders,
		USDToCNYRate:         usdToCNYRate,
		EPayTypes:            epayTypes,
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
			"mode":               mode,
			"prepaid_amount_usd": req.PrepaidAmountUSD,
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
