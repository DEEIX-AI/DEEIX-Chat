// Package errorlog 实现请求错误日志的持久化。
package errorlog

import (
	"context"
	"strings"

	domainerrorlog "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/errorlog"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
)

// Repo 封装错误日志数据访问。
type Repo struct {
	db *gorm.DB
}

// NewRepo 创建仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// Create 追加一条错误日志。
func (r *Repo) Create(ctx context.Context, item *domainerrorlog.Log) error {
	row := toModelErrorLog(item)
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return dberror.Translate(err)
	}
	item.ID = row.ID
	item.CreatedAt = row.CreatedAt
	return nil
}

// List 分页查询错误日志。
func (r *Repo) List(ctx context.Context, offset int, limit int, filter repository.ErrorLogListFilter) ([]domainerrorlog.Log, int64, error) {
	items := make([]models.ErrorLog, 0)
	var total int64

	query := r.db.WithContext(ctx).Model(&models.ErrorLog{})
	if requestID := strings.TrimSpace(filter.RequestID); requestID != "" {
		query = query.Where("request_id = ?", requestID)
	}
	if keyword := strings.TrimSpace(filter.Query); keyword != "" {
		// 错误 ID 走精确匹配（有索引）；其余字段模糊匹配，便于按路由、错误码或原因片段排查。
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Where(
			"request_id = ? OR trace_id = ? OR LOWER(route) LIKE ? OR LOWER(path) LIKE ? OR LOWER(error_code) LIKE ? OR LOWER(cause) LIKE ? OR LOWER(detail) LIKE ?",
			keyword, strings.ToLower(keyword), like, like, like, like, like,
		)
	}
	if errorCode := strings.TrimSpace(filter.ErrorCode); errorCode != "" {
		query = query.Where("error_code = ?", errorCode)
	}
	switch strings.TrimSpace(filter.StatusClass) {
	case "5xx":
		query = query.Where("status_code >= ?", 500)
	case "stream":
		// 写出 200 后才失败的流式请求：状态码仍是 2xx。
		query = query.Where("status_code < ?", 400)
	}
	if filter.UserID > 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.CreatedFrom != nil {
		query = query.Where("created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("created_at <= ?", *filter.CreatedTo)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, dberror.Translate(err)
	}
	order := "created_at DESC, id DESC"
	if strings.TrimSpace(filter.Sort) == "created_asc" {
		order = "created_at ASC, id ASC"
	}
	if err := query.Order(order).Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return nil, 0, dberror.Translate(err)
	}
	results := make([]domainerrorlog.Log, 0, len(items))
	for _, item := range items {
		results = append(results, toDomainErrorLog(item))
	}
	return results, total, nil
}

func toModelErrorLog(item *domainerrorlog.Log) models.ErrorLog {
	if item == nil {
		return models.ErrorLog{}
	}
	return models.ErrorLog{
		ID:         item.ID,
		RequestID:  item.RequestID,
		TraceID:    item.TraceID,
		UserID:     item.UserID,
		Method:     item.Method,
		Route:      item.Route,
		Path:       item.Path,
		StatusCode: item.StatusCode,
		ErrorCode:  item.ErrorCode,
		Cause:      item.Cause,
		Detail:     item.Detail,
		LatencyMS:  item.LatencyMS,
		ClientIP:   item.ClientIP,
		UserAgent:  item.UserAgent,
		CreatedAt:  item.CreatedAt,
	}
}

func toDomainErrorLog(item models.ErrorLog) domainerrorlog.Log {
	return domainerrorlog.Log{
		ID:         item.ID,
		RequestID:  item.RequestID,
		TraceID:    item.TraceID,
		UserID:     item.UserID,
		Method:     item.Method,
		Route:      item.Route,
		Path:       item.Path,
		StatusCode: item.StatusCode,
		ErrorCode:  item.ErrorCode,
		Cause:      item.Cause,
		Detail:     item.Detail,
		LatencyMS:  item.LatencyMS,
		ClientIP:   item.ClientIP,
		UserAgent:  item.UserAgent,
		CreatedAt:  item.CreatedAt,
	}
}
