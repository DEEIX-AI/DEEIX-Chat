package systemevent

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	domainsystemevent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/systemevent"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/traceid"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// ListFilter 描述系统事件筛选条件。
type ListFilter struct {
	Query       string
	Level       string
	Source      string
	Event       string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	Sort        string
}

// WriteInput 描述写入系统事件的输入。
type WriteInput struct {
	RequestID  string
	TraceID    string
	Level      string
	Source     string
	Event      string
	Resource   string
	ResourceID string
	Message    string
	Detail     interface{}
}

// Service 封装系统事件能力。
type Service struct {
	repo repository.SystemEventRepository
}

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// NewService 创建系统事件服务。
func NewService(repo repository.SystemEventRepository) *Service {
	return &Service{repo: repo}
}

// Write 写入系统事件。
func (s *Service) Write(ctx context.Context, input WriteInput) {
	if s == nil || s.repo == nil {
		return
	}
	detailJSON := "{}"
	if input.Detail != nil {
		if raw, err := json.Marshal(input.Detail); err == nil {
			detailJSON = string(raw)
		}
	}
	level := normalizeLevel(input.Level)
	traceIDValue := strings.TrimSpace(input.TraceID)
	if traceIDValue == "" {
		traceIDValue = traceid.FromContext(ctx)
	}
	_ = s.repo.Create(ctx, &domainsystemevent.Event{
		RequestID:  strings.TrimSpace(input.RequestID),
		TraceID:    traceIDValue,
		Level:      level,
		Source:     strings.TrimSpace(input.Source),
		Event:      strings.TrimSpace(input.Event),
		Resource:   strings.TrimSpace(input.Resource),
		ResourceID: strings.TrimSpace(input.ResourceID),
		Message:    strings.TrimSpace(input.Message),
		DetailJSON: detailJSON,
	})
}

// List 分页查询系统事件。
func (s *Service) List(ctx context.Context, page int, pageSize int, filter ListFilter) ([]domainsystemevent.Event, int64, error) {
	offset, limit := normalizePage(page, pageSize)
	return s.repo.List(ctx, offset, limit, repository.SystemEventListFilter{
		Query:       filter.Query,
		Level:       filter.Level,
		Source:      filter.Source,
		Event:       filter.Event,
		CreatedFrom: filter.CreatedFrom,
		CreatedTo:   filter.CreatedTo,
		Sort:        filter.Sort,
	})
}

func normalizeLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "warn", "warning":
		return "warn"
	case "error":
		return "error"
	default:
		return "info"
	}
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
