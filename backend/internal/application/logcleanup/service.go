package logcleanup

import (
	"context"
	"strings"
	"time"

	appaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const (
	TypeAudit        = repository.LogCleanupTypeAudit
	TypeAuth         = repository.LogCleanupTypeAuth
	TypeUsage        = repository.LogCleanupTypeUsage
	TypeOrders       = repository.LogCleanupTypeOrders
	TypeConversation = repository.LogCleanupTypeConversation
	TypeModeration   = repository.LogCleanupTypeModeration
	TypeErrors       = repository.LogCleanupTypeErrors
)

type auditWriter interface {
	Write(ctx context.Context, input appaudit.WriteInput)
}

// Input 描述一次管理员日志清理请求。
type Input struct {
	Type        string
	Before      time.Time
	RequestID   string
	ActorUserID uint
	IP          string
	UserAgent   string
}

// Result 描述一次管理员日志清理结果。
type Result struct {
	Type         string
	Before       time.Time
	DeletedCount int64
}

// Service 封装日志清理和清理审计。
type Service struct {
	repo        repository.LogCleanupRepository
	auditWriter auditWriter
}

// NewService 创建日志清理服务。
func NewService(repo repository.LogCleanupRepository, auditWriter auditWriter) *Service {
	return &Service{repo: repo, auditWriter: auditWriter}
}

// Cleanup 物理删除指定时间点之前的一类日志，并记录管理员操作审计。
func (s *Service) Cleanup(ctx context.Context, input Input) (*Result, error) {
	logType := strings.ToLower(strings.TrimSpace(input.Type))
	if !validType(logType) {
		return nil, ErrInvalidType
	}
	if input.Before.IsZero() {
		return nil, ErrInvalidBefore
	}
	if input.Before.After(time.Now()) {
		return nil, ErrFutureBefore
	}

	deletedCount, err := s.repo.DeleteBefore(ctx, logType, input.Before)
	if err != nil {
		return nil, err
	}

	if s.auditWriter != nil {
		s.auditWriter.Write(ctx, appaudit.WriteInput{
			RequestID:   input.RequestID,
			ActorUserID: input.ActorUserID,
			Action:      "admin_cleanup_logs",
			Resource:    "logs",
			ResourceID:  logType,
			IP:          input.IP,
			UserAgent:   input.UserAgent,
			Detail: map[string]any{
				"type":          logType,
				"before":        input.Before.Format(time.RFC3339),
				"deleted_count": deletedCount,
			},
		})
	}

	return &Result{
		Type:         logType,
		Before:       input.Before,
		DeletedCount: deletedCount,
	}, nil
}

func validType(value string) bool {
	switch value {
	case TypeAudit, TypeAuth, TypeUsage, TypeOrders, TypeConversation, TypeModeration, TypeErrors:
		return true
	default:
		return false
	}
}
