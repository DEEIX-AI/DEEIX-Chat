package errorlog

import (
	"context"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	domainerrorlog "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/errorlog"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
	"go.uber.org/zap"
)

const (
	// maxInflightWrites 限制同时进行的写入数：数据库故障时 5xx 会集中爆发，
	// 超出部分直接丢弃（访问日志里仍有完整记录），避免写错误日志反过来压垮数据库。
	maxInflightWrites = 32
	writeTimeout      = 5 * time.Second
	maxCauseBytes     = 4096
	maxDetailBytes    = 16 << 10
	maxPathBytes      = 512
	maxRouteBytes     = 255
	maxUserAgentBytes = 255
)

// RecordInput 描述一次失败请求。Cause 与 Path 由调用方脱敏后传入。
type RecordInput struct {
	RequestID  string
	TraceID    string
	UserID     uint
	Method     string
	Route      string
	Path       string
	StatusCode int
	ErrorCode  string
	Cause      string
	// Detail 是错误链携带的原始诊断内容（如上游响应体），调用方遮蔽凭证后传入。
	Detail     string
	LatencyMS  int64
	ClientIP   string
	UserAgent  string
	OccurredAt time.Time
}

// ListFilter 描述管理员错误日志列表的筛选条件。
type ListFilter struct {
	RequestID   string
	Query       string
	ErrorCode   string
	StatusClass string
	UserID      uint
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	Sort        string
}

// Service 封装错误日志的异步写入与查询。
type Service struct {
	repo     repository.ErrorLogRepository
	logger   *zap.Logger
	inflight chan struct{}
	dropped  atomic.Int64
}

// NewService 创建服务。
func NewService(repo repository.ErrorLogRepository, logger *zap.Logger) *Service {
	return &Service{repo: repo, logger: logger, inflight: make(chan struct{}, maxInflightWrites)}
}

// Record 异步持久化一次失败请求，不阻塞响应；写入失败或被限流丢弃时只记日志。
func (s *Service) Record(ctx context.Context, input RecordInput) {
	item := toDomainLog(input)
	if item.RequestID == "" {
		return
	}
	select {
	case s.inflight <- struct{}{}:
	default:
		if dropped := s.dropped.Add(1); dropped == 1 || dropped%100 == 0 {
			s.logger.Warn("error_log_write_dropped", zap.Int64("dropped_total", dropped), zap.String("request_id", item.RequestID))
		}
		return
	}
	// 请求可能已结束或被取消：脱离请求取消，但保留上下文值并限定写入时长。
	writeCtx, cancel := background.WithTimeout(ctx, writeTimeout)
	background.Go(s.logger, "error_log_write", func() {
		defer func() { <-s.inflight }()
		defer cancel()
		if err := s.repo.Create(writeCtx, &item); err != nil {
			s.logger.Warn("error_log_write_failed", zap.String("request_id", item.RequestID), zap.Error(err))
		}
	})
}

// List 分页查询错误日志。
func (s *Service) List(ctx context.Context, page int, pageSize int, filter ListFilter) ([]domainerrorlog.Log, int64, error) {
	offset, limit := pagination.Offset(page, pageSize)
	return s.repo.List(ctx, offset, limit, repository.ErrorLogListFilter{
		RequestID:   strings.TrimSpace(filter.RequestID),
		Query:       strings.TrimSpace(filter.Query),
		ErrorCode:   strings.TrimSpace(filter.ErrorCode),
		StatusClass: strings.TrimSpace(filter.StatusClass),
		UserID:      filter.UserID,
		CreatedFrom: filter.CreatedFrom,
		CreatedTo:   filter.CreatedTo,
		Sort:        strings.TrimSpace(filter.Sort),
	})
}

func toDomainLog(input RecordInput) domainerrorlog.Log {
	occurredAt := input.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	return domainerrorlog.Log{
		RequestID:  truncateUTF8(strings.TrimSpace(input.RequestID), 64),
		TraceID:    truncateUTF8(strings.TrimSpace(input.TraceID), 64),
		UserID:     input.UserID,
		Method:     truncateUTF8(strings.TrimSpace(input.Method), 16),
		Route:      truncateUTF8(strings.TrimSpace(input.Route), maxRouteBytes),
		Path:       truncateUTF8(strings.TrimSpace(input.Path), maxPathBytes),
		StatusCode: input.StatusCode,
		ErrorCode:  truncateUTF8(strings.TrimSpace(input.ErrorCode), 128),
		Cause:      truncateUTF8(strings.TrimSpace(input.Cause), maxCauseBytes),
		Detail:     truncateUTF8(strings.TrimSpace(input.Detail), maxDetailBytes),
		LatencyMS:  input.LatencyMS,
		ClientIP:   truncateUTF8(strings.TrimSpace(input.ClientIP), 64),
		UserAgent:  truncateUTF8(strings.TrimSpace(input.UserAgent), maxUserAgentBytes),
		CreatedAt:  occurredAt,
	}
}

func truncateUTF8(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut]
}
