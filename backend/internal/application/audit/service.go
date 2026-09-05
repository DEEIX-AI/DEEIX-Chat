package audit

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	domainaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/audit"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/traceid"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
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

// WriteInput 描述一条待写入的审计日志。
type WriteInput struct {
	RequestID   string
	ActorUserID uint
	Action      string
	Resource    string
	ResourceID  string
	IP          string
	UserAgent   string
	Detail      any
}

func normalizeWriteInput(input WriteInput) WriteInput {
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.Action = strings.TrimSpace(input.Action)
	input.Resource = strings.TrimSpace(input.Resource)
	input.ResourceID = strings.TrimSpace(input.ResourceID)
	input.IP = strings.TrimSpace(input.IP)
	input.UserAgent = strings.TrimSpace(input.UserAgent)
	return input
}

// Service 封装审计业务能力。
type Service struct {
	repo   repository.AuditRepository
	logger *zap.Logger
}

// NewService 创建服务。
func NewService(repo repository.AuditRepository, logger *zap.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

// Write 写入审计日志（DB 持久化 + 结构化日志输出）。
func (s *Service) Write(ctx context.Context, input WriteInput) {
	input = normalizeWriteInput(input)

	detailJSON := "{}"
	if input.Detail != nil {
		if raw, err := json.Marshal(input.Detail); err == nil {
			detailJSON = string(raw)
		}
	}

	traceID := traceid.FromContext(ctx)

	// 结构化日志输出（供日志平台采集）
	s.logger.Info("audit",
		zap.String("trace_id", traceID),
		zap.String("request_id", input.RequestID),
		zap.Uint("user_id", input.ActorUserID),
		zap.String("action", input.Action),
		zap.String("resource", input.Resource),
		zap.String("resource_id", input.ResourceID),
		zap.String("ip", input.IP),
		zap.String("user_agent", input.UserAgent),
		zap.String("detail", detailJSON),
	)

	// DB 持久化
	if err := s.repo.Create(ctx, &domainaudit.Log{
		RequestID:   input.RequestID,
		ActorUserID: input.ActorUserID,
		Action:      input.Action,
		Resource:    input.Resource,
		ResourceID:  input.ResourceID,
		IP:          input.IP,
		UserAgent:   input.UserAgent,
		DetailJSON:  detailJSON,
	}); err != nil {
		s.logger.Error("audit_persist_failed",
			zap.String("trace_id", traceID),
			zap.String("request_id", input.RequestID),
			zap.String("action", input.Action),
			zap.Error(err),
		)
	}
}

// List 分页查询审计日志。
func (s *Service) List(ctx context.Context, page int, pageSize int, filter ListFilter) ([]domainaudit.Log, int64, error) {
	offset, limit := pagination.Offset(page, pageSize)
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
