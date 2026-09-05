package user

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	appuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// Handler 预留用户域 HTTP 处理器（当前用户管理在 admin 模块暴露）。
type Handler struct {
	service *appuser.Service
}

// NewHandler 创建处理器。
func NewHandler(service *appuser.Service) *Handler {
	return &Handler{service: service}
}

// GetAvatar 获取用户当前上传头像内容。
func (h *Handler) GetAvatar(c *gin.Context) {
	publicID := strings.TrimSpace(c.Param("public_id"))
	if publicID == "" {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidUserID)
		return
	}
	result, err := h.service.OpenAvatarContent(c.Request.Context(), publicID)
	if err != nil {
		switch {
		case errors.Is(err, appuser.ErrUserNotFound),
			errors.Is(err, appuser.ErrAvatarNotFound):
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		default:
			response.InternalError(c)
			return
		}
	}
	defer result.Reader.Close() //nolint:errcheck

	contentType := strings.TrimSpace(result.ContentType)
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		response.ErrorFrom(c, http.StatusNotFound, errAvatarNotFound)
		return
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "inline")
	c.Header("Cache-Control", "public, max-age=300")
	c.Header("X-Content-Type-Options", "nosniff")
	if result.SizeBytes > 0 {
		c.Header("Content-Length", strconv.FormatInt(result.SizeBytes, 10))
	}
	if !result.ModTime.IsZero() {
		c.Header("Last-Modified", result.ModTime.UTC().Format(http.TimeFormat))
	}
	if _, err = io.Copy(c.Writer, result.Reader); err != nil {
		c.Abort()
		return
	}
}

// GetDailyActivity godoc
// @Summary 查询每日活跃度
// @Description 查询当前用户按计费归属日聚合的模型请求数与 token 消耗，逐日补零
// @Tags user
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param days query int false "统计天数(默认365，最大366)"
// @Success 200 {object} UserDailyActivityListResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /user/stats/activity [get]
func (h *Handler) GetDailyActivity(c *gin.Context) {
	days := 0
	if daysText := strings.TrimSpace(c.Query("days")); daysText != "" {
		parsed, err := strconv.Atoi(daysText)
		if err != nil || parsed <= 0 {
			response.ErrorFrom(c, http.StatusBadRequest, errInvalidDailyActivityDays)
			return
		}
		days = parsed
	}
	items, err := h.service.GetDailyActivity(c.Request.Context(), middleware.MustUserID(c), days, time.Now())
	if err != nil {
		response.InternalError(c)
		return
	}
	results := make([]UserDailyActivityItem, 0, len(items))
	for _, item := range items {
		results = append(results, UserDailyActivityItem{
			Date:         item.Date,
			RequestCount: item.RequestCount,
			TokenUsage:   item.TokenUsage,
		})
	}
	response.Success(c, results)
}
