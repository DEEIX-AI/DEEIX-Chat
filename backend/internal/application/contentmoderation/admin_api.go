package contentmoderation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
)

// StatsFilter 限定管理员统计查询范围。
type StatsFilter struct {
	From *time.Time
	To   *time.Time
}

// EventListInput 是超级管理员事件查询参数。
type EventListInput struct {
	Query     string
	Direction string
	Modality  string
	Result    string
	Category  string
	UserID    uint
	RunID     string
	From      *time.Time
	To        *time.Time
	Page      int
	PageSize  int
}

// EventDetail 是超级管理员详情载荷（可能包含解密后的文本）。
type EventDetail struct {
	Event           domaincm.Event
	CategoryScores  map[string]float64
	DecryptedText   string
	TextAvailable   bool
	ImagesAvailable bool
	Images          []domaincm.IsolatedImageMeta
}

// GetStats 返回最近 90 天的匿名聚合数据（admin 及以上）。
func (s *Service) GetStats(ctx context.Context, actorRole string, filter StatsFilter) ([]domaincm.DailyStat, error) {
	if !isAdminRole(actorRole) {
		return nil, ErrAdminRequired
	}
	now := time.Now().UTC()
	to := now
	from := now.Add(-metadataRetention)
	if filter.To != nil && !filter.To.IsZero() {
		to = filter.To.UTC()
	}
	if filter.From != nil && !filter.From.IsZero() {
		from = filter.From.UTC()
	}
	minFrom := now.Add(-metadataRetention)
	if from.Before(minFrom) {
		from = minFrom
	}
	if to.Before(from) {
		return nil, ErrInvalidEventFilter
	}
	return s.repo.ListDailyStats(ctx, from, to)
}

// ListEvents 为超级管理员列出保留的审核决策元数据。
func (s *Service) ListEvents(ctx context.Context, actorRole string, input EventListInput) ([]domaincm.Event, int64, error) {
	if !isSuperAdmin(actorRole) {
		return nil, 0, ErrSuperAdminRequired
	}
	direction := strings.TrimSpace(input.Direction)
	if direction != "" && direction != domaincm.DirectionInput && direction != domaincm.DirectionOutput {
		return nil, 0, ErrInvalidEventFilter
	}
	modality := strings.TrimSpace(input.Modality)
	if modality != "" && modality != domaincm.ModalityText && modality != domaincm.ModalityImage {
		return nil, 0, ErrInvalidEventFilter
	}
	result := strings.TrimSpace(input.Result)
	if result != "" && result != domaincm.ResultHit && result != domaincm.ResultFailedOpen && result != domaincm.ResultPassed {
		return nil, 0, ErrInvalidEventFilter
	}
	category := strings.TrimSpace(input.Category)
	if category != "" && !IsKnownCategory(category) {
		return nil, 0, ErrInvalidEventFilter
	}
	if input.From != nil && input.To != nil && input.To.Before(*input.From) {
		return nil, 0, ErrInvalidEventFilter
	}
	query := strings.TrimSpace(input.Query)
	if len(query) > 200 {
		return nil, 0, ErrInvalidEventFilter
	}
	offset, limit := pagination.Offset(input.Page, input.PageSize)
	return s.repo.ListEvents(ctx, domaincm.EventListFilter{
		Query:     query,
		Direction: direction,
		Modality:  modality,
		Result:    result,
		Category:  category,
		UserID:    input.UserID,
		RunID:     strings.TrimSpace(input.RunID),
		From:      input.From,
		To:        input.To,
		Offset:    offset,
		Limit:     limit,
	})
}

// GetEventDetail 在文本仍被保留时返回解密后的文本。
func (s *Service) GetEventDetail(
	ctx context.Context,
	actorRole string,
	eventID string,
) (*EventDetail, error) {
	if !isSuperAdmin(actorRole) {
		return nil, ErrSuperAdminRequired
	}
	event, err := s.repo.GetEventByPublicID(ctx, strings.TrimSpace(eventID))
	if errors.Is(err, repository.ErrNotFound) || (err == nil && event == nil) {
		return nil, ErrEventNotFound
	}
	if err != nil {
		return nil, err
	}
	detail := &EventDetail{Event: *event}
	_ = json.Unmarshal([]byte(event.CategoryScoresJSON), &detail.CategoryScores)
	detail.Images = unmarshalIsolatedImageMetadata(event.ImageMetaJSON)

	if event.Result == domaincm.ResultHit &&
		event.Modality == domaincm.ModalityText &&
		strings.TrimSpace(event.EncryptedText) != "" &&
		time.Now().Before(event.ContentExpiresAt) {
		if plain, decErr := s.decryptText(event.EncryptedText); decErr == nil {
			detail.DecryptedText = plain
			detail.TextAvailable = true
		}
	}
	if event.Modality == domaincm.ModalityImage && len(detail.Images) > 0 && time.Now().Before(event.ContentExpiresAt) {
		detail.ImagesAvailable = true
	}
	return detail, nil
}

// OpenEventImage 解密隔离图片，供超级管理员流式读取。
func (s *Service) OpenEventImage(
	ctx context.Context,
	actorRole string,
	eventID string,
	index int,
) (data []byte, mimeType string, err error) {
	if !isSuperAdmin(actorRole) {
		return nil, "", ErrSuperAdminRequired
	}
	event, err := s.repo.GetEventByPublicID(ctx, strings.TrimSpace(eventID))
	if errors.Is(err, repository.ErrNotFound) || (err == nil && event == nil) {
		return nil, "", ErrEventNotFound
	}
	if err != nil {
		return nil, "", err
	}
	if time.Now().After(event.ContentExpiresAt) {
		return nil, "", ErrEventNotFound
	}
	images := unmarshalIsolatedImageMetadata(event.ImageMetaJSON)
	var meta *domaincm.IsolatedImageMeta
	for i := range images {
		if images[i].Index == index {
			meta = &images[i]
			break
		}
	}
	if meta == nil || s.objectStore == nil {
		return nil, "", ErrEventNotFound
	}
	raw, err := s.objectStore.Open(ctx, meta.StoragePath)
	if err != nil {
		return nil, "", err
	}
	// 隔离图片以 encryptBytes 载荷（v1: base64）存储，而非 UTF-8 文本。
	plain, err := s.decryptBytes(string(raw))
	if err != nil {
		return nil, "", err
	}
	return plain, textutil.FirstNonEmpty(meta.MimeType, "image/png"), nil
}

// CategoryCatalog 返回管理界面使用的类别列表。
func CategoryCatalog() map[string][]string {
	return map[string][]string{
		"text":  AllTextCategories(),
		"image": ImageCategories(),
	}
}
