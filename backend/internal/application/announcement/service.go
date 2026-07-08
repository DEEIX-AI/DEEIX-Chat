package announcement

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	domainannouncement "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/announcement"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const (
	maxAnnouncementTitleLength   = 120
	maxAnnouncementContentLength = 20000
)

// Service 封装公告业务逻辑。
type Service struct {
	repo          repository.AnnouncementRepository
	cfg           *config.Runtime
	routeResolver announcementAIRouteResolver
	llmClient     *llm.Client
}

// NewService 创建公告服务。
func NewService(repo repository.AnnouncementRepository) *Service {
	return &Service{repo: repo}
}

// SetAIGenerator configures the model route used for AI announcement drafts.
func (s *Service) SetAIGenerator(cfg *config.Runtime, routeResolver announcementAIRouteResolver, llmClient *llm.Client) {
	s.cfg = cfg
	s.routeResolver = routeResolver
	s.llmClient = llmClient
}

type announcementAIRouteResolver interface {
	ResolveDefaultRoute(ctx context.Context, input channel.ResolveRouteInput) (*channel.ResolvedRoute, error)
	MarkRouteFailure(ctx context.Context, route *channel.ResolvedRoute, cause error)
	MarkRouteSuccess(ctx context.Context, route *channel.ResolvedRoute)
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
	page, pageSize := normalizePage(input.Page, input.PageSize)
	return s.repo.ListAdminAnnouncements(ctx, repository.AnnouncementListFilter{
		Query:  strings.TrimSpace(input.Query),
		Status: strings.TrimSpace(input.Status),
		Type:   strings.TrimSpace(input.Type),
		Pinned: input.Pinned,
	}, (page-1)*pageSize, pageSize)
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
	status := normalizeStatus(input.Status)
	announcementType := normalizeType(input.Type)
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
		status := normalizeStatus(*input.Status)
		if status == "" {
			return repository.AnnouncementPatch{}, ErrInvalidAnnouncement
		}
		patch.Status = &status
	}
	if input.Type != nil {
		announcementType := normalizeType(*input.Type)
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

func normalizeStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "", domainannouncement.StatusActive:
		return domainannouncement.StatusActive
	case domainannouncement.StatusInactive:
		return domainannouncement.StatusInactive
	default:
		return ""
	}
}

func normalizeType(announcementType string) string {
	switch strings.TrimSpace(announcementType) {
	case "", domainannouncement.TypeGeneral:
		return domainannouncement.TypeGeneral
	case domainannouncement.TypeCritical:
		return domainannouncement.TypeCritical
	case domainannouncement.TypeWarning:
		return domainannouncement.TypeWarning
	case domainannouncement.TypeInfo:
		return domainannouncement.TypeInfo
	case domainannouncement.TypeNormal:
		return domainannouncement.TypeNormal
	default:
		return ""
	}
}

func validWindow(startsAt *time.Time, expiresAt *time.Time) bool {
	if startsAt == nil || expiresAt == nil {
		return true
	}
	return expiresAt.After(*startsAt)
}

func normalizePage(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	const maxPageSize = 1000
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
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
