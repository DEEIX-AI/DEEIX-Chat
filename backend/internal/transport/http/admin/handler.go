package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	appadmin "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/admin"
	auditapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	applogcleanup "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/logcleanup"
	systemeventapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/systemevent"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/user"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	conversationhttp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type conversationExporter interface {
	ExportConversationData(ctx context.Context, conversation *domainconversation.Conversation) (*appconversation.ConversationExportResult, error)
	ListAllConversationsAfterID(ctx context.Context, afterID uint, limit int) ([]domainconversation.Conversation, error)
}

type conversationExportManifest struct {
	Type      string `json:"_type"`
	Complete  bool   `json:"complete"`
	Exported  int64  `json:"exported"`
	Failed    int    `json:"failed"`
	FailedIDs []uint `json:"failedIDs,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Handler 封装后台管理 HTTP 处理。
type Handler struct {
	service            *appadmin.Service
	conversationExport conversationExporter
}

// NewHandler 创建处理器。
func NewHandler(service *appadmin.Service) *Handler {
	return &Handler{service: service}
}

// SetConversationExporter 注入会话导出能力。
func (h *Handler) SetConversationExporter(exporter conversationExporter) {
	h.conversationExport = exporter
}

// ListUsers godoc
// @Summary 管理员查询用户
// @Description 管理员分页查看所有用户，实现账户隔离管理
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param q query string false "搜索用户名、昵称、邮箱或公开ID"
// @Param subscription_status query string false "订阅状态过滤(active/free)"
// @Param identity_provider query string false "身份源 slug 过滤"
// @Success 200 {object} UserListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/users [get]
// ListUsers 列出用户。
func (h *Handler) ListUsers(c *gin.Context) {
	page, pageSize := pageParams(c)
	items, total, err := h.service.ListUsers(c.Request.Context(), page, pageSize, appadmin.UserListFilter{
		Query:              c.Query("q"),
		SubscriptionStatus: c.Query("subscription_status"),
		IdentityProvider:   c.Query("identity_provider"),
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list users failed")
		return
	}
	views := make([]UserResponse, 0, len(items))
	for _, v := range items {
		views = append(views, toUserResponse(v))
	}
	response.SuccessPage(c, total, views)
}

// CreateUser godoc
// @Summary 管理员创建用户
// @Description 创建普通用户账号；需要授予管理员权限时，可在账户编辑中调整角色
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateUserRequest true "用户参数"
// @Success 200 {object} CreateUserResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/users [post]
// CreateUser 创建用户。
func (h *Handler) CreateUser(c *gin.Context) {
	actorUserID := middleware.MustUserID(c)

	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.CreateUser(
		c.Request.Context(),
		req.Username,
		req.Password,
		req.AvatarURL,
		req.DisplayName,
		req.Email,
		req.Phone,
		req.Timezone,
		req.Locale,
		req.SubscriptionTier,
		req.SubscriptionExpiresAt,
	)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrUsernameTaken):
			response.Error(c, http.StatusConflict, "username already exists")
			return
		case errors.Is(err, user.ErrInvalidUsername),
			errors.Is(err, user.ErrInvalidDisplayName),
			errors.Is(err, user.ErrInvalidPassword):
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		case errors.Is(err, user.ErrInvalidAvatarURL),
			errors.Is(err, user.ErrInvalidEmail),
			errors.Is(err, user.ErrInvalidPhone),
			errors.Is(err, user.ErrInvalidTimeZone),
			errors.Is(err, user.ErrInvalidLocale),
			errors.Is(err, user.ErrInvalidSubscriptionTier),
			errors.Is(err, user.ErrSubscriptionExpiryRequired),
			errors.Is(err, user.ErrInvalidSubscriptionExpiry):
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		default:
			response.Error(c, http.StatusInternalServerError, "create user failed")
			return
		}
	}

	h.service.WriteAdminCreateUserAudit(
		c.Request.Context(),
		middleware.MustRequestID(c),
		actorUserID,
		item.ID,
		item.Username,
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	view, err := h.service.BuildUserView(c.Request.Context(), *item)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "resolve subscription failed")
		return
	}

	response.Success(c, UserDataResponse{User: toUserResponse(view)})
}

// ImportOpenWebUIUsers godoc
// @Summary 管理员导入 OpenWebUI 用户
// @Description 从 OpenWebUI SQLite 或 PostgreSQL 数据库读取用户，按 email 去重导入；已存在用户不会修改
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ImportOpenWebUIUsersRequest true "OpenWebUI 导入参数"
// @Success 200 {object} ImportOpenWebUIUsersResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 403 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/users/import/openwebui [post]
func (h *Handler) ImportOpenWebUIUsers(c *gin.Context) {
	actorUserID := middleware.MustUserID(c)

	var req ImportOpenWebUIUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	if req.CreditMultiplier == nil || *req.CreditMultiplier < 0 {
		response.ErrorFrom(c, http.StatusBadRequest, appadmin.ErrInvalidImportMultiplier)
		return
	}

	result, err := h.service.ImportOpenWebUIUsers(
		c.Request.Context(),
		middleware.MustRequestID(c),
		actorUserID,
		appadmin.OpenWebUIImportInput{
			DSN:              req.DSN,
			CreditMultiplier: *req.CreditMultiplier,
			DryRun:           req.DryRun,
		},
		c.ClientIP(),
		c.Request.UserAgent(),
	)
	if err != nil {
		switch {
		case errors.Is(err, appadmin.ErrInvalidImportDSN),
			errors.Is(err, appadmin.ErrInvalidImportMultiplier):
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		case errors.Is(err, appadmin.ErrAdminPermissionRequired):
			response.ErrorFrom(c, http.StatusForbidden, err)
			return
		case errors.Is(err, appadmin.ErrOpenWebUIImportFailed):
			response.Error(c, http.StatusInternalServerError, "openwebui import failed")
			return
		default:
			response.Error(c, http.StatusInternalServerError, "import openwebui users failed")
			return
		}
	}

	response.Success(c, toImportOpenWebUIUsersResponse(result))
}

// PatchUser godoc
// @Summary 管理员更新用户可编辑字段
// @Description 管理员统一维护角色、状态、时区等可编辑字段
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户ID"
// @Param body body PatchUserRequest true "局部更新参数"
// @Success 200 {object} UpdateUserStatusResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/users/{id} [patch]
func (h *Handler) PatchUser(c *gin.Context) {
	actorUserID := middleware.MustUserID(c)

	rawID := c.Param("id")
	parsedID, err := strconv.ParseUint(rawID, 10, strconv.IntSize)
	if err != nil || parsedID == 0 {
		response.Error(c, http.StatusBadRequest, "invalid user id")
		return
	}

	var req PatchUserRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.PatchUserByAdmin(
		c.Request.Context(),
		middleware.MustRequestID(c),
		actorUserID,
		uint(parsedID),
		toAppPatchUserInput(req),
		c.ClientIP(),
		c.Request.UserAgent(),
	)
	if err != nil {
		switch {
		case errors.Is(err, appadmin.ErrInvalidUserEmail),
			errors.Is(err, appadmin.ErrInvalidUserPhone),
			errors.Is(err, appadmin.ErrInvalidUserLocale),
			errors.Is(err, appadmin.ErrInvalidUserStatus),
			errors.Is(err, appadmin.ErrInvalidUserRole),
			errors.Is(err, appadmin.ErrInvalidUserTimeZone),
			errors.Is(err, appbilling.ErrInvalidSubscriptionTier),
			errors.Is(err, appbilling.ErrSubscriptionExpiryRequired),
			errors.Is(err, appbilling.ErrInvalidSubscriptionExpiry),
			errors.Is(err, user.ErrInvalidDisplayName),
			errors.Is(err, appadmin.ErrEmptyAdminUserPatch):
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		case errors.Is(err, appbilling.ErrPaymentRequired):
			response.ErrorFrom(c, http.StatusConflict, err)
			return
		case errors.Is(err, user.ErrUserNotFound):
			response.Error(c, http.StatusNotFound, "user not found")
			return
		case errors.Is(err, appadmin.ErrAdminPermissionRequired),
			errors.Is(err, appadmin.ErrSuperAdminManagementNotAllowed):
			response.ErrorFrom(c, http.StatusForbidden, err)
			return
		case errors.Is(err, appadmin.ErrSuperAdminStatusChangeNotAllowed),
			errors.Is(err, appadmin.ErrLastSuperAdminRoleChangeNotAllowed),
			errors.Is(err, appadmin.ErrSelfRoleChangeNotAllowed),
			errors.Is(err, appadmin.ErrSelfStatusChangeNotAllowed):
			response.ErrorFrom(c, http.StatusConflict, err)
			return
		default:
			response.Error(c, http.StatusInternalServerError, "patch user failed")
			return
		}
	}

	view, err := h.service.BuildUserView(c.Request.Context(), *item)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "resolve subscription failed")
		return
	}

	response.Success(c, UserDataResponse{User: toUserResponse(view)})
}

// ListAuditLogs godoc
// @Summary 管理员查询审计日志
// @Description 管理员分页查看全量可追溯审计日志
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param query query string false "搜索关键词"
// @Param resource query string false "资源类型"
// @Param action query string false "动作"
// @Param actor_user_id query int false "操作人用户ID"
// @Param created_from query string false "创建时间起点(RFC3339)"
// @Param created_to query string false "创建时间终点(RFC3339)"
// @Param sort query string false "排序方式"
// @Success 200 {object} AuditLogListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/audit-logs [get]
// ListAuditLogs 查询审计日志。
func (h *Handler) ListAuditLogs(c *gin.Context) {
	page, pageSize := pageParams(c)
	actorUserID, ok := parseOptionalUintQuery(c, "actor_user_id")
	if !ok {
		return
	}
	createdFrom, ok := parseOptionalTimeQuery(c, "created_from")
	if !ok {
		return
	}
	createdTo, ok := parseOptionalTimeQuery(c, "created_to")
	if !ok {
		return
	}
	items, total, err := h.service.ListAuditLogs(c.Request.Context(), page, pageSize, auditapp.ListFilter{
		Query:       c.Query("query"),
		Resource:    c.Query("resource"),
		Action:      c.Query("action"),
		ActorUserID: actorUserID,
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
		Sort:        c.Query("sort"),
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list audit logs failed")
		return
	}
	userIDs := make([]uint, 0, len(items))
	for _, item := range items {
		userIDs = append(userIDs, item.ActorUserID)
	}
	userLabels := h.service.ResolveUserLabels(c.Request.Context(), userIDs)
	logs := make([]AuditLogResponse, 0, len(items))
	for _, l := range items {
		logs = append(logs, toAuditLogResponse(l, userLabels[l.ActorUserID]))
	}
	response.SuccessPage(c, total, logs)
}

// CleanupLogs godoc
// @Summary 管理员清理日志
// @Description 按日志类型物理删除指定时间点之前的日志；操作不可恢复
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CleanupLogsRequest true "日志清理参数"
// @Success 200 {object} CleanupLogsResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/logs/cleanup [post]
// CleanupLogs 清理指定时间点之前的一类日志。
func (h *Handler) CleanupLogs(c *gin.Context) {
	var req CleanupLogsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	before, err := time.Parse(time.RFC3339, req.Before)
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, applogcleanup.ErrInvalidBefore)
		return
	}

	result, err := h.service.CleanupLogs(c.Request.Context(), applogcleanup.Input{
		Type:        req.Type,
		Before:      before,
		RequestID:   middleware.MustRequestID(c),
		ActorUserID: middleware.MustUserID(c),
		IP:          c.ClientIP(),
		UserAgent:   c.Request.UserAgent(),
	})
	if err != nil {
		switch {
		case errors.Is(err, applogcleanup.ErrInvalidType),
			errors.Is(err, applogcleanup.ErrInvalidBefore),
			errors.Is(err, applogcleanup.ErrFutureBefore):
			response.ErrorFrom(c, http.StatusBadRequest, err)
		default:
			response.Error(c, http.StatusInternalServerError, "cleanup logs failed")
		}
		return
	}

	response.Success(c, CleanupLogsResponse{
		Type:         result.Type,
		Before:       result.Before,
		DeletedCount: result.DeletedCount,
	})
}

// ListUsageLogs godoc
// @Summary 管理员查询模型调用日志
// @Description 管理员分页查看全量模型调用与计费用量账本
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param query query string false "搜索模型、上游、绑定编码、协议"
// @Param platform_model_name query string false "平台模型名筛选"
// @Param billing_mode query string false "计费模式筛选：free/token/call/duration/tiered"
// @Param user_id query int false "调用人用户ID"
// @Param created_from query string false "创建时间起点(RFC3339)"
// @Param created_to query string false "创建时间终点(RFC3339)"
// @Param sort query string false "排序方式"
// @Success 200 {object} UsageLogListResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/call-logs [get]
// ListUsageLogs 查询模型调用日志。
func (h *Handler) ListUsageLogs(c *gin.Context) {
	page, pageSize := pageParams(c)
	userID, ok := parseOptionalUintQuery(c, "user_id")
	if !ok {
		return
	}
	createdFrom, ok := parseOptionalTimeQuery(c, "created_from")
	if !ok {
		return
	}
	createdTo, ok := parseOptionalTimeQuery(c, "created_to")
	if !ok {
		return
	}
	items, total, err := h.service.ListUsageLogs(c.Request.Context(), page, pageSize, appbilling.UsageLogListFilter{
		Query:             c.Query("query"),
		PlatformModelName: c.Query("platform_model_name"),
		BillingMode:       c.Query("billing_mode"),
		UserID:            userID,
		CreatedFrom:       createdFrom,
		CreatedTo:         createdTo,
		Sort:              c.Query("sort"),
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list call logs failed")
		return
	}
	userIDs := make([]uint, 0, len(items))
	for _, item := range items {
		userIDs = append(userIDs, item.UserID)
	}
	userLabels := h.service.ResolveUserLabels(c.Request.Context(), userIDs)
	logs := make([]UsageLogResponse, 0, len(items))
	for _, item := range items {
		logs = append(logs, toUsageLogResponse(item, userLabels[item.UserID]))
	}
	response.SuccessPage(c, total, logs)
}

// GetUsageStatistics godoc
// @Summary 管理员查询全局用量统计
// @Description 管理员按日期、统计对象、平台模型和计费范围查看全局费用、Token、调用次数及排名；用户与权限组筛选互斥
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param start_date query string false "开始日期(YYYY-MM-DD)，默认近30天"
// @Param end_date query string false "结束日期(YYYY-MM-DD，包含当日)"
// @Param user_id query int false "用户ID"
// @Param permission_group_id query int false "权限组ID，与 user_id 互斥"
// @Param platform_model_name query string false "平台模型名"
// @Param billing_scope query string false "计费范围：all/free/billable"
// @Param section query string false "返回范围：all/models/users"
// @Param model_rank_by query string false "模型排名指标：cost/tokens/calls"
// @Param user_rank_by query string false "用户排名指标：cost/tokens/calls"
// @Success 200 {object} UsageStatisticsResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/usage-statistics [get]
// GetUsageStatistics 查询管理员全局用量统计。
func (h *Handler) GetUsageStatistics(c *gin.Context) {
	userID, ok := parseOptionalUintQuery(c, "user_id")
	if !ok {
		return
	}
	permissionGroupID, ok := parseOptionalUintQuery(c, "permission_group_id")
	if !ok {
		return
	}

	startDateText := strings.TrimSpace(c.Query("start_date"))
	endDateText := strings.TrimSpace(c.Query("end_date"))
	now := time.Now()
	endDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startDate := endDate.AddDate(0, 0, -29)
	if startDateText != "" || endDateText != "" {
		if startDateText == "" || endDateText == "" {
			response.ErrorWithCode(c, http.StatusBadRequest, "usage_statistics.invalid_date_range", "start_date and end_date must be provided together")
			return
		}
		parsedStartDate, startErr := time.Parse("2006-01-02", startDateText)
		parsedEndDate, endErr := time.Parse("2006-01-02", endDateText)
		if startErr != nil || endErr != nil {
			response.ErrorWithCode(c, http.StatusBadRequest, "usage_statistics.invalid_date_range", "invalid usage statistics date range")
			return
		}
		startDate = parsedStartDate
		endDate = parsedEndDate
	}
	if endDate.Before(startDate) || int(endDate.Sub(startDate).Hours()/24)+1 > 366 {
		response.ErrorWithCode(c, http.StatusBadRequest, "usage_statistics.invalid_date_range", "invalid usage statistics date range")
		return
	}

	billingScope := strings.TrimSpace(c.Query("billing_scope"))
	if billingScope == "" {
		billingScope = "all"
	}
	if billingScope != "all" && billingScope != "free" && billingScope != "billable" {
		response.ErrorWithCode(c, http.StatusBadRequest, "usage_statistics.invalid_billing_scope", "invalid billing_scope")
		return
	}
	section := strings.TrimSpace(c.Query("section"))
	if section == "" {
		section = "all"
	}
	if section != "all" && section != "models" && section != "users" {
		response.ErrorWithCode(c, http.StatusBadRequest, "usage_statistics.invalid_section", "invalid section")
		return
	}
	modelRankBy := strings.TrimSpace(c.Query("model_rank_by"))
	if modelRankBy == "" {
		modelRankBy = "cost"
	}
	if modelRankBy != "cost" && modelRankBy != "tokens" && modelRankBy != "calls" {
		response.ErrorWithCode(c, http.StatusBadRequest, "usage_statistics.invalid_rank_by", "invalid model_rank_by")
		return
	}
	userRankBy := strings.TrimSpace(c.Query("user_rank_by"))
	if userRankBy == "" {
		userRankBy = "cost"
	}
	if userRankBy != "cost" && userRankBy != "tokens" && userRankBy != "calls" {
		response.ErrorWithCode(c, http.StatusBadRequest, "usage_statistics.invalid_rank_by", "invalid user_rank_by")
		return
	}

	statistics, err := h.service.GetUsageStatistics(c.Request.Context(), appbilling.UsageStatisticsFilter{
		StartDate:         startDate,
		EndDate:           endDate,
		UserID:            userID,
		PermissionGroupID: permissionGroupID,
		PlatformModelName: c.Query("platform_model_name"),
		BillingScope:      billingScope,
		Section:           section,
		ModelRankBy:       modelRankBy,
		UserRankBy:        userRankBy,
	})
	if err != nil {
		switch {
		case errors.Is(err, appbilling.ErrInvalidUsageStatisticsSubject):
			response.ErrorWithCode(c, http.StatusBadRequest, "usage_statistics.subject_conflict", err.Error())
		case errors.Is(err, appadmin.ErrPermissionGroupNotFound):
			response.ErrorFrom(c, http.StatusNotFound, err)
		default:
			response.Error(c, http.StatusInternalServerError, "get usage statistics failed")
		}
		return
	}
	userIDs := make([]uint, 0, len(statistics.TopUsers))
	for _, rankedUser := range statistics.TopUsers {
		userIDs = append(userIDs, rankedUser.UserID)
	}
	userLabels := h.service.ResolveUserLabels(c.Request.Context(), userIDs)
	response.Success(c, toUsageStatisticsResponse(statistics, startDate, endDate, section, userLabels))
}

// ListPaymentOrders godoc
// @Summary 管理员查询支付订单记录
// @Description 管理员分页查看订阅和充值支付单
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param query query string false "搜索订单号、支付渠道、外部支付ID"
// @Param order_type query string false "订单类型(subscription/topup)"
// @Param provider query string false "支付渠道"
// @Param status query string false "支付状态"
// @Param user_id query int false "用户ID"
// @Param created_from query string false "创建时间起点(RFC3339)"
// @Param created_to query string false "创建时间终点(RFC3339)"
// @Param sort query string false "排序方式"
// @Success 200 {object} PaymentOrderListResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/payment-orders [get]
// ListPaymentOrders 查询支付订单记录。
func (h *Handler) ListPaymentOrders(c *gin.Context) {
	page, pageSize := pageParams(c)
	userID, ok := parseOptionalUintQuery(c, "user_id")
	if !ok {
		return
	}
	createdFrom, ok := parseOptionalTimeQuery(c, "created_from")
	if !ok {
		return
	}
	createdTo, ok := parseOptionalTimeQuery(c, "created_to")
	if !ok {
		return
	}
	items, total, err := h.service.ListPaymentOrders(c.Request.Context(), page, pageSize, appbilling.PaymentOrderListFilter{
		Query:       c.Query("query"),
		OrderType:   c.Query("order_type"),
		Provider:    c.Query("provider"),
		Status:      c.Query("status"),
		UserID:      userID,
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
		Sort:        c.Query("sort"),
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list payment orders failed")
		return
	}
	userIDs := make([]uint, 0, len(items))
	for _, item := range items {
		userIDs = append(userIDs, item.UserID)
	}
	userLabels := h.service.ResolveUserLabels(c.Request.Context(), userIDs)
	orders := make([]PaymentOrderResponse, 0, len(items))
	for _, item := range items {
		orders = append(orders, toPaymentOrderResponse(item, userLabels[item.UserID]))
	}
	response.SuccessPage(c, total, orders)
}

// ListConversationEvents godoc
// @Summary 管理员查询对话事件
// @Description 管理员分页查看对话运行轨迹、工具、MCP 与处理事件
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param query query string false "搜索运行ID、事件、阶段、标题、工具名"
// @Param event_scope query string false "事件范围(trace_block/trace_event/tool_call)"
// @Param event_type query string false "事件类型"
// @Param status query string false "事件状态"
// @Param user_id query int false "用户ID"
// @Param conversation_id query int false "会话ID"
// @Param created_from query string false "创建时间起点(RFC3339)"
// @Param created_to query string false "创建时间终点(RFC3339)"
// @Param sort query string false "排序方式"
// @Success 200 {object} ConversationEventListResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/conversation-events [get]
// ListConversationEvents 查询对话事件。
func (h *Handler) ListConversationEvents(c *gin.Context) {
	page, pageSize := pageParams(c)
	userID, ok := parseOptionalUintQuery(c, "user_id")
	if !ok {
		return
	}
	conversationID, ok := parseOptionalUintQuery(c, "conversation_id")
	if !ok {
		return
	}
	createdFrom, ok := parseOptionalTimeQuery(c, "created_from")
	if !ok {
		return
	}
	createdTo, ok := parseOptionalTimeQuery(c, "created_to")
	if !ok {
		return
	}
	items, total, err := h.service.ListConversationEventLogs(c.Request.Context(), page, pageSize, appconversation.EventLogListFilter{
		Query:          c.Query("query"),
		EventScope:     c.Query("event_scope"),
		EventType:      c.Query("event_type"),
		Status:         c.Query("status"),
		UserID:         userID,
		ConversationID: conversationID,
		CreatedFrom:    createdFrom,
		CreatedTo:      createdTo,
		Sort:           c.Query("sort"),
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list conversation events failed")
		return
	}
	userIDs := make([]uint, 0, len(items))
	for _, item := range items {
		userIDs = append(userIDs, item.UserID)
	}
	userLabels := h.service.ResolveUserLabels(c.Request.Context(), userIDs)
	events := make([]ConversationEventResponse, 0, len(items))
	for _, item := range items {
		events = append(events, toConversationEventResponse(item, userLabels[item.UserID]))
	}
	response.SuccessPage(c, total, events)
}

// ListSystemEvents godoc
// @Summary 管理员查询系统事件
// @Description 管理员分页查看后台结构化系统事件
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param query query string false "搜索关键词"
// @Param level query string false "级别"
// @Param source query string false "来源"
// @Param event query string false "事件"
// @Param created_from query string false "创建时间起点(RFC3339)"
// @Param created_to query string false "创建时间终点(RFC3339)"
// @Param sort query string false "排序方式"
// @Success 200 {object} SystemEventListResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/system-events [get]
func (h *Handler) ListSystemEvents(c *gin.Context) {
	page, pageSize := pageParams(c)
	createdFrom, ok := parseOptionalTimeQuery(c, "created_from")
	if !ok {
		return
	}
	createdTo, ok := parseOptionalTimeQuery(c, "created_to")
	if !ok {
		return
	}
	items, total, err := h.service.ListSystemEvents(c.Request.Context(), page, pageSize, systemeventapp.ListFilter{
		Query:       c.Query("query"),
		Level:       c.Query("level"),
		Source:      c.Query("source"),
		Event:       c.Query("event"),
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
		Sort:        c.Query("sort"),
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list system events failed")
		return
	}
	results := make([]SystemEventResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toSystemEventResponse(item))
	}
	response.SuccessPage(c, total, results)
}

func parseOptionalUintQuery(c *gin.Context, key string) (uint, bool) {
	raw := c.Query(key)
	if raw == "" {
		return 0, true
	}
	parsed, err := strconv.ParseUint(raw, 10, strconv.IntSize)
	if err != nil || parsed == 0 {
		response.Error(c, http.StatusBadRequest, "invalid "+key)
		return 0, false
	}
	return uint(parsed), true
}

func parseOptionalTimeQuery(c *gin.Context, key string) (*time.Time, bool) {
	raw := c.Query(key)
	if raw == "" {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid "+key)
		return nil, false
	}
	return &parsed, true
}

// RevokeUserSessions godoc
// @Summary 管理员吊销用户全部会话
// @Description 管理员吊销指定用户全部活跃会话，用于安全治理和风险控制
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户ID"
// @Success 200 {object} RevokeUserSessionsResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/users/{id}/revoke-sessions [post]
func (h *Handler) RevokeUserSessions(c *gin.Context) {
	actorUserID := middleware.MustUserID(c)

	rawID := c.Param("id")
	parsedID, err := strconv.ParseUint(rawID, 10, strconv.IntSize)
	if err != nil || parsedID == 0 {
		response.Error(c, http.StatusBadRequest, "invalid user id")
		return
	}

	if err = h.service.RevokeUserSessionsByAdmin(
		c.Request.Context(),
		middleware.MustRequestID(c),
		actorUserID,
		uint(parsedID),
		c.ClientIP(),
		c.Request.UserAgent(),
	); err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			response.Error(c, http.StatusNotFound, "user not found")
			return
		}
		if errors.Is(err, appadmin.ErrAdminPermissionRequired) || errors.Is(err, appadmin.ErrSuperAdminManagementNotAllowed) {
			response.ErrorFrom(c, http.StatusForbidden, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, "revoke user sessions failed")
		return
	}

	response.Success(c, RevokeUserSessionsResponse{Revoked: true})
}

// UpdateUserStatus godoc
// @Summary 管理员更新用户状态
// @Description 管理员维护用户状态（active/locked/suspended/deactivated），并联动会话治理
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户ID"
// @Param body body UpdateUserStatusRequest true "状态变更参数"
// @Success 200 {object} UpdateUserStatusResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/users/{id}/status [patch]
func (h *Handler) UpdateUserStatus(c *gin.Context) {
	actorUserID := middleware.MustUserID(c)

	rawID := c.Param("id")
	parsedID, err := strconv.ParseUint(rawID, 10, strconv.IntSize)
	if err != nil || parsedID == 0 {
		response.Error(c, http.StatusBadRequest, "invalid user id")
		return
	}

	var req UpdateUserStatusRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.UpdateUserStatusByAdmin(
		c.Request.Context(),
		middleware.MustRequestID(c),
		actorUserID,
		uint(parsedID),
		req.Status,
		req.Reason,
		c.ClientIP(),
		c.Request.UserAgent(),
	)
	if err != nil {
		if errors.Is(err, appadmin.ErrInvalidUserStatus) {
			response.Error(c, http.StatusBadRequest, "invalid user status")
			return
		}
		if errors.Is(err, user.ErrUserNotFound) {
			response.Error(c, http.StatusNotFound, "user not found")
			return
		}
		if errors.Is(err, appadmin.ErrSuperAdminStatusChangeNotAllowed) {
			response.Error(c, http.StatusConflict, "superadmin status change not allowed")
			return
		}
		if errors.Is(err, appadmin.ErrAdminPermissionRequired) || errors.Is(err, appadmin.ErrSuperAdminManagementNotAllowed) {
			response.ErrorFrom(c, http.StatusForbidden, err)
			return
		}
		response.Error(c, http.StatusInternalServerError, "update user status failed")
		return
	}

	view, err := h.service.BuildUserView(c.Request.Context(), *item)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "resolve subscription failed")
		return
	}

	response.Success(c, UserDataResponse{User: toUserResponse(view)})
}

// ResetUserPassword godoc
// @Summary 管理员重置用户密码
// @Description 管理员重置指定用户密码并吊销其全部会话
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户ID"
// @Param body body ResetUserPasswordRequest true "重置密码参数"
// @Success 200 {object} ResetUserPasswordResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/users/{id}/reset-password [post]
func (h *Handler) ResetUserPassword(c *gin.Context) {
	actorUserID := middleware.MustUserID(c)

	rawID := c.Param("id")
	parsedID, err := strconv.ParseUint(rawID, 10, strconv.IntSize)
	if err != nil || parsedID == 0 {
		response.Error(c, http.StatusBadRequest, "invalid user id")
		return
	}

	var req ResetUserPasswordRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	mustResetPassword := true
	if req.MustResetPassword != nil {
		mustResetPassword = *req.MustResetPassword
	}

	if err = h.service.ResetUserPasswordByAdmin(
		c.Request.Context(),
		middleware.MustRequestID(c),
		actorUserID,
		uint(parsedID),
		req.NewPassword,
		mustResetPassword,
		c.ClientIP(),
		c.Request.UserAgent(),
	); err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			response.Error(c, http.StatusNotFound, "user not found")
			return
		}
		if errors.Is(err, appadmin.ErrSuperAdminPasswordResetNotAllowed) {
			response.Error(c, http.StatusConflict, "superadmin password reset not allowed")
			return
		}
		if errors.Is(err, appadmin.ErrAdminPermissionRequired) || errors.Is(err, appadmin.ErrSuperAdminManagementNotAllowed) {
			response.ErrorFrom(c, http.StatusForbidden, err)
			return
		}
		if errors.Is(err, user.ErrInvalidPassword) {
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		}
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}

	response.Success(c, ResetUserPasswordResponse{Reset: true})
}

func (h *Handler) ResetUserTwoFactor(c *gin.Context) {
	actorUserID := middleware.MustUserID(c)
	rawID := c.Param("id")
	parsedID, err := strconv.ParseUint(rawID, 10, strconv.IntSize)
	if err != nil || parsedID == 0 {
		response.Error(c, http.StatusBadRequest, "invalid user id")
		return
	}
	if err = h.service.ResetUserTwoFactorByAdmin(
		c.Request.Context(),
		middleware.MustRequestID(c),
		actorUserID,
		uint(parsedID),
		c.ClientIP(),
		c.Request.UserAgent(),
	); err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			response.Error(c, http.StatusNotFound, "user not found")
			return
		}
		if errors.Is(err, appadmin.ErrSuperAdminTwoFactorResetNotAllowed) {
			response.Error(c, http.StatusConflict, "superadmin two factor reset not allowed")
			return
		}
		if errors.Is(err, appadmin.ErrAdminPermissionRequired) || errors.Is(err, appadmin.ErrSuperAdminManagementNotAllowed) {
			response.ErrorFrom(c, http.StatusForbidden, err)
			return
		}
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Success(c, ResetUserTwoFactorResponse{Reset: true})
}

// DeleteUser godoc
// @Summary 管理员删除用户
// @Description 管理员硬删除指定普通用户及其主要用户域数据
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户ID"
// @Success 200 {object} DeleteUserResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/users/{id} [delete]
func (h *Handler) DeleteUser(c *gin.Context) {
	actorUserID := middleware.MustUserID(c)

	rawID := c.Param("id")
	parsedID, err := strconv.ParseUint(rawID, 10, strconv.IntSize)
	if err != nil || parsedID == 0 {
		response.Error(c, http.StatusBadRequest, "invalid user id")
		return
	}

	if err = h.service.DeleteUserByAdmin(
		c.Request.Context(),
		middleware.MustRequestID(c),
		actorUserID,
		uint(parsedID),
		c.ClientIP(),
		c.Request.UserAgent(),
	); err != nil {
		switch {
		case errors.Is(err, user.ErrUserNotFound):
			response.Error(c, http.StatusNotFound, "user not found")
			return
		case errors.Is(err, appadmin.ErrAdminPermissionRequired),
			errors.Is(err, appadmin.ErrSuperAdminManagementNotAllowed):
			response.ErrorFrom(c, http.StatusForbidden, err)
			return
		case errors.Is(err, appadmin.ErrSuperAdminDeleteNotAllowed),
			errors.Is(err, appadmin.ErrSelfDeleteNotAllowed):
			response.ErrorFrom(c, http.StatusConflict, err)
			return
		default:
			response.Error(c, http.StatusInternalServerError, "delete user failed")
			return
		}
	}

	response.Success(c, DeleteUserResponse{Deleted: true})
}

// ListUserAuthEvents godoc
// @Summary 管理员查询用户认证事件
// @Description 管理员分页查询认证事件，支持 user_id/event_type/result 过滤
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user_id query int false "用户ID过滤"
// @Param event_type query string false "事件类型过滤"
// @Param result query string false "结果过滤(success/failure/blocked)"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} UserAuthEventListResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/user-auth-events [get]
func (h *Handler) ListUserAuthEvents(c *gin.Context) {
	var userID uint
	if raw := c.Query("user_id"); raw != "" {
		parsedID, err := strconv.ParseUint(raw, 10, strconv.IntSize)
		if err != nil || parsedID == 0 {
			response.Error(c, http.StatusBadRequest, "invalid user_id")
			return
		}
		userID = uint(parsedID)
	}

	page, pageSize := pageParams(c)
	items, total, err := h.service.ListUserAuthEventsByAdmin(
		c.Request.Context(),
		userID,
		c.Query("event_type"),
		c.Query("result"),
		page,
		pageSize,
	)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list user auth events failed")
		return
	}

	userIDs := make([]uint, 0, len(items))
	for _, item := range items {
		userIDs = append(userIDs, item.UserID)
	}
	userLabels := h.service.ResolveUserLabels(c.Request.Context(), userIDs)
	events := make([]AuthEventResponse, 0, len(items))
	for _, e := range items {
		events = append(events, toAuthEventResponse(e, userLabels[e.UserID]))
	}
	response.SuccessPage(c, total, events)
}

// ExportConversations godoc
// @Summary 管理员导出全量对话数据
// @Description 流式导出全量会话及消息为 NDJSON 文件，最后一行为 export_manifest 元数据
// @Tags admin
// @Produce application/x-ndjson
// @Security BearerAuth
// @Success 200 {string} string "NDJSON stream"
// @Failure 500 {object} ErrorDoc
// @Router /admin/conversations/export [get]
// ExportConversations 流式导出全量对话。
func (h *Handler) ExportConversations(c *gin.Context) {
	if h.conversationExport == nil {
		response.Error(c, http.StatusInternalServerError, "export not available")
		return
	}

	actorUserID := middleware.MustUserID(c)
	h.service.WriteAuditLog(c.Request.Context(), middleware.MustRequestID(c), actorUserID, "admin_export_conversations", "conversation", "", c.ClientIP(), c.Request.UserAgent(), nil)

	c.Header("Content-Type", "application/x-ndjson")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="conversations-export-%s.jsonl"`, time.Now().UTC().Format("20060102-150405")))
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)

	const batchSize = 50
	var lastID uint
	encoder := json.NewEncoder(c.Writer)
	exported := int64(0)
	var failedIDs []uint
	writeManifest := func(complete bool, exportErr string) bool {
		manifest := conversationExportManifest{
			Type:      "export_manifest",
			Complete:  complete,
			Exported:  exported,
			Failed:    len(failedIDs),
			FailedIDs: failedIDs,
			Error:     exportErr,
		}
		if err := encoder.Encode(manifest); err != nil {
			return false
		}
		c.Writer.Flush()
		return true
	}

	for {
		if c.Request.Context().Err() != nil {
			return
		}
		conversations, err := h.conversationExport.ListAllConversationsAfterID(c.Request.Context(), lastID, batchSize)
		if err != nil {
			writeManifest(false, "failed to list conversations")
			return
		}
		if len(conversations) == 0 {
			break
		}

		for i := range conversations {
			result, err := h.conversationExport.ExportConversationData(c.Request.Context(), &conversations[i])
			if err != nil {
				failedIDs = append(failedIDs, conversations[i].ID)
				continue
			}
			if err := encoder.Encode(conversationhttp.ToConversationExportResponse(result)); err != nil {
				return
			}
			exported++
		}

		c.Writer.Flush()
		lastID = conversations[len(conversations)-1].ID

		if len(conversations) < batchSize {
			break
		}
	}

	writeManifest(true, "")
}

func pageParams(c *gin.Context) (int, int) {
	page := 1
	pageSize := 20
	const maxPageSize = 1000

	if raw := c.Query("page"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if raw := c.Query("page_size"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			if parsed > maxPageSize {
				parsed = maxPageSize
			}
			pageSize = parsed
		}
	}

	return page, pageSize
}
