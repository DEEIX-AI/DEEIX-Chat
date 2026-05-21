package audit

import (
	"context"
	"encoding/json"
	"time"

	domainaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/audit"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/traceid"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

// ListFilter 描述审计日志列表筛选条件。
type ListFilter struct {
	Query       string
	Resource    string
	Action      string
	ActorUserID uint
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	Sort        string
}

// Service 封装审计业务能力。
type Service struct {
	repo   repository.AuditRepository
	logger *zap.Logger
}

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// NewService 创建服务。
func NewService(repo repository.AuditRepository, logger *zap.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

// Write 写入审计日志（DB 持久化 + 结构化日志输出）。
func (s *Service) Write(
	ctx context.Context,
	requestID string,
	actorUserID uint,
	action string,
	resource string,
	resourceID string,
	ip string,
	userAgent string,
	detail interface{},
) {
	detailJSON := "{}"
	if detail != nil {
		if raw, err := json.Marshal(detail); err == nil {
			detailJSON = string(raw)
		}
	}

	traceID := traceid.FromContext(ctx)

	// 结构化日志输出（供日志平台采集）
	s.logger.Info("audit",
		zap.String("trace_id", traceID),
		zap.String("request_id", requestID),
		zap.Uint("user_id", actorUserID),
		zap.String("action", action),
		zap.String("resource", resource),
		zap.String("resource_id", resourceID),
		zap.String("ip", ip),
		zap.String("user_agent", userAgent),
		zap.String("detail", detailJSON),
	)

	// DB 持久化
	if err := s.repo.Create(ctx, &domainaudit.Log{
		RequestID:   requestID,
		ActorUserID: actorUserID,
		Action:      action,
		Resource:    resource,
		ResourceID:  resourceID,
		IP:          ip,
		UserAgent:   userAgent,
		DetailJSON:  detailJSON,
	}); err != nil {
		s.logger.Error("audit_persist_failed",
			zap.String("trace_id", traceID),
			zap.String("request_id", requestID),
			zap.String("action", action),
			zap.Error(err),
		)
	}
}

// List 分页查询审计日志。
func (s *Service) List(ctx context.Context, page int, pageSize int, filter ListFilter) ([]domainaudit.Log, int64, error) {
	offset, limit := normalizePage(page, pageSize)
	return s.repo.List(ctx, offset, limit, repository.AuditLogListFilter{
		Query:       filter.Query,
		Resource:    filter.Resource,
		Action:      filter.Action,
		ActorUserID: filter.ActorUserID,
		CreatedFrom: filter.CreatedFrom,
		CreatedTo:   filter.CreatedTo,
		Sort:        filter.Sort,
	})
}

func normalizePage(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	return offset, pageSize
}
