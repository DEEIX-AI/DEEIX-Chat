package announcement

import (
	"context"
	"errors"
	"strings"
	"time"

	domainannouncement "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/announcement"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
)

const (
	maxAnnouncementTitleLength   = 120
	maxAnnouncementContentLength = 20000
)

// Service 封装公告业务逻辑。
type Service struct {
	repo repository.AnnouncementRepository
}

// NewService 创建公告服务。
func NewService(repo repository.AnnouncementRepository) *Service {
	return &Service{repo: repo}
}

// ListActive 查询当前用户可展示公告。
func (s *Service) ListActive(ctx context.Context, userID uint, now time.Time, includeDismissed bool) ([]domainannouncement.Announcement, error) {
	if userID == 0 {
		return nil, repository.ErrInvalidInput
	}
	return s.repo.ListActiveAnnouncements(ctx, userID, now, includeDismissed)
}

// ListAdmin 查询管理员公告列表。
func (s *Service) ListAdmin(ctx context.Context, input ListInput) ([]domainannouncement.Announcement, int64, error) {
	offset, limit := pagination.Offset(input.Page, input.PageSize)
	return s.repo.ListAdminAnnouncements(ctx, repository.AnnouncementListFilter{
		Query:  strings.TrimSpace(input.Query),
		Status: strings.TrimSpace(input.Status),
		Type:   strings.TrimSpace(input.Type),
		Pinned: input.Pinned,
	}, offset, limit)
}

// Create 创建公告。
func (s *Service) Create(ctx context.Context, actorUserID uint, input WriteInput) (*domainannouncement.Announcement, error) {
	if actorUserID == 0 {
		return nil, repository.ErrInvalidInput
	}
	item, err := normalizeWriteInput(input, true)
	if err != nil {
		return nil, err
	}
	item.CreatedByUserID = actorUserID
	return s.repo.CreateAnnouncement(ctx, item)
}

// Update 更新公告。
func (s *Service) Update(ctx context.Context, id uint, input PatchInput) (*domainannouncement.Announcement, error) {
	if id == 0 {
		return nil, repository.ErrInvalidInput
	}
	patch, err := normalizePatchInput(input)
	if err != nil {
		return nil, err
	}
	item, err := s.repo.PatchAnnouncement(ctx, id, patch)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return item, nil
}

// Delete 删除公告。
func (s *Service) Delete(ctx context.Context, id uint) error {
	if id == 0 {
		return repository.ErrInvalidInput
	}
	return mapRepositoryError(s.repo.DeleteAnnouncement(ctx, id))
}

// DismissToday 记录当前用户今天不再显示指定公告版本。
func (s *Service) DismissToday(ctx context.Context, userID uint, announcementID uint, announcementUpdatedAt time.Time, now time.Time, dismissedUntil time.Time) error {
	if userID == 0 || announcementID == 0 || announcementUpdatedAt.IsZero() || !dismissedUntil.After(now) {
		return repository.ErrInvalidInput
	}
	return mapRepositoryError(s.repo.DismissAnnouncementToday(ctx, userID, announcementID, announcementUpdatedAt, now, dismissedUntil))
}

// Close 记录当前用户关闭指定公告版本。
func (s *Service) Close(ctx context.Context, userID uint, announcementID uint, announcementUpdatedAt time.Time, now time.Time) error {
	if userID == 0 || announcementID == 0 || announcementUpdatedAt.IsZero() {
		return repository.ErrInvalidInput
	}
	return mapRepositoryError(s.repo.CloseAnnouncement(ctx, userID, announcementID, announcementUpdatedAt, now))
}

// ListInput 定义公告列表入参。
type ListInput struct {
	Query    string
	Status   string
	Type     string
	Pinned   *bool
	Page     int
	PageSize int
}

// WriteInput 定义公告创建入参。
type WriteInput struct {
	Title           string
	ContentMarkdown string
	Status          string
	Type            string
	Pinned          bool
	Priority        int
	StartsAt        *time.Time
	ExpiresAt       *time.Time
}

// PatchInput 定义公告更新入参。
type PatchInput struct {
	Title              *string
	ContentMarkdown    *string
	Status             *string
	Type               *string
	Pinned             *bool
	Priority           *int
	StartsAtSet        bool
	StartsAt           *time.Time
	ExpiresAtSet       bool
	ExpiresAt          *time.Time
	CreatedByUserIDSet bool
	CreatedByUserID    uint
}

func normalizeWriteInput(input WriteInput, requireContent bool) (*domainannouncement.Announcement, error) {
	title := strings.TrimSpace(input.Title)
	content := strings.TrimSpace(input.ContentMarkdown)
	status := domainannouncement.NormalizeStatus(input.Status)
	announcementType := domainannouncement.NormalizeType(input.Type)
	if title == "" || len(title) > maxAnnouncementTitleLength {
		return nil, ErrInvalidAnnouncement
	}
	if requireContent && content == "" {
		return nil, ErrInvalidAnnouncement
	}
	if len(content) > maxAnnouncementContentLength {
		return nil, ErrInvalidAnnouncement
	}
	if status == "" {
		return nil, ErrInvalidAnnouncement
	}
	if announcementType == "" {
		return nil, ErrInvalidAnnouncement
	}
	if !validWindow(input.StartsAt, input.ExpiresAt) {
		return nil, ErrInvalidAnnouncement
	}
	return &domainannouncement.Announcement{
		Title:           title,
		ContentMarkdown: content,
		Status:          status,
		Type:            announcementType,
		Pinned:          input.Pinned,
		Priority:        input.Priority,
		StartsAt:        input.StartsAt,
		ExpiresAt:       input.ExpiresAt,
	}, nil
}

func normalizePatchInput(input PatchInput) (repository.AnnouncementPatch, error) {
	patch := repository.AnnouncementPatch{
		StartsAtSet:        input.StartsAtSet,
		StartsAt:           input.StartsAt,
		ExpiresAtSet:       input.ExpiresAtSet,
		ExpiresAt:          input.ExpiresAt,
		CreatedByUserIDSet: input.CreatedByUserIDSet,
		CreatedByUserID:    input.CreatedByUserID,
	}
	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		if title == "" || len(title) > maxAnnouncementTitleLength {
			return repository.AnnouncementPatch{}, ErrInvalidAnnouncement
		}
		patch.Title = &title
	}
	if input.ContentMarkdown != nil {
		content := strings.TrimSpace(*input.ContentMarkdown)
		if content == "" || len(content) > maxAnnouncementContentLength {
			return repository.AnnouncementPatch{}, ErrInvalidAnnouncement
		}
		patch.ContentMarkdown = &content
	}
	if input.Status != nil {
		status := domainannouncement.NormalizeStatus(*input.Status)
		if status == "" {
			return repository.AnnouncementPatch{}, ErrInvalidAnnouncement
		}
		patch.Status = &status
	}
	if input.Type != nil {
		announcementType := domainannouncement.NormalizeType(*input.Type)
		if announcementType == "" {
			return repository.AnnouncementPatch{}, ErrInvalidAnnouncement
		}
		patch.Type = &announcementType
	}
	if input.Pinned != nil {
		patch.Pinned = input.Pinned
	}
	if input.Priority != nil {
		patch.Priority = input.Priority
	}
	if input.StartsAtSet && input.ExpiresAtSet && !validWindow(input.StartsAt, input.ExpiresAt) {
		return repository.AnnouncementPatch{}, ErrInvalidAnnouncement
	}
	return patch, nil
}

func validWindow(startsAt *time.Time, expiresAt *time.Time) bool {
	if startsAt == nil || expiresAt == nil {
		return true
	}
	return expiresAt.After(*startsAt)
}

func mapRepositoryError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrNotFound) {
		return ErrAnnouncementNotFound
	}
	return err
}
