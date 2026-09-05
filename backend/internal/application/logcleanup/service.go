package logcleanup

import (
	"context"
	"strings"
	"time"

	appaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
)

const (
	TypeAudit        = repository.LogCleanupTypeAudit
	TypeAuth         = repository.LogCleanupTypeAuth
	TypeUsage        = repository.LogCleanupTypeUsage
	TypeOrders       = repository.LogCleanupTypeOrders
	TypeConversation = repository.LogCleanupTypeConversation
	TypeSystem       = repository.LogCleanupTypeSystem
)

const maxConversationRunCleanupCount = 100

var (
	ErrInvalidType   = apperr.New("request.invalid_log_cleanup_type", "invalid log cleanup type")
	ErrInvalidBefore = apperr.New("request.invalid_log_cleanup_before", "invalid log cleanup before")
	ErrFutureBefore  = apperr.New("log_cleanup.before_in_future", "log cleanup before must not be in the future")
	ErrInvalidRunIDs = apperr.New("request.invalid_conversation_run_ids", "invalid conversation run ids")
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

// ConversationRunInput 描述一次管理员按运行清理对话事件的请求。
type ConversationRunInput struct {
	RunIDs      []string
	RequestID   string
	ActorUserID uint
	IP          string
	UserAgent   string
}

// ConversationRunResult 描述按运行清理对话事件的结果。
type ConversationRunResult struct {
	RunCount     int
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

// CleanupConversationRuns 物理删除指定运行的对话事件，并记录管理员操作审计。
func (s *Service) CleanupConversationRuns(ctx context.Context, input ConversationRunInput) (*ConversationRunResult, error) {
	runIDs, err := normalizeConversationRunIDs(input.RunIDs)
	if err != nil {
		return nil, err
	}
	deletedCount, err := s.repo.DeleteConversationRuns(ctx, runIDs)
	if err != nil {
		return nil, err
	}
	if s.auditWriter != nil {
		s.auditWriter.Write(ctx, appaudit.WriteInput{
			RequestID:   input.RequestID,
			ActorUserID: input.ActorUserID,
			Action:      "admin_cleanup_conversation_runs",
			Resource:    "conversation_events",
			ResourceID:  "batch",
			IP:          input.IP,
			UserAgent:   input.UserAgent,
			Detail: map[string]any{
				"run_ids":       runIDs,
				"run_count":     len(runIDs),
				"deleted_count": deletedCount,
			},
		})
	}
	return &ConversationRunResult{RunCount: len(runIDs), DeletedCount: deletedCount}, nil
}

func normalizeConversationRunIDs(values []string) ([]string, error) {
	if len(values) == 0 || len(values) > maxConversationRunCleanupCount {
		return nil, ErrInvalidRunIDs
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		runID := strings.TrimSpace(value)
		if runID == "" || len(runID) > 64 {
			return nil, ErrInvalidRunIDs
		}
		if _, exists := seen[runID]; exists {
			continue
		}
		seen[runID] = struct{}{}
		result = append(result, runID)
	}
	if len(result) == 0 {
		return nil, ErrInvalidRunIDs
	}
	return result, nil
}

func validType(value string) bool {
	switch value {
	case TypeAudit, TypeAuth, TypeUsage, TypeOrders, TypeConversation, TypeSystem:
		return true
	default:
		return false
	}
}
