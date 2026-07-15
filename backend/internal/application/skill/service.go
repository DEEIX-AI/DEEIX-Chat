package skill

import (
	"context"
	"errors"
	"strings"

	domainskill "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/skill"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const (
	maxSkillTitleLength       = 64
	maxSkillTriggerLength     = 64
	maxSkillDescriptionLength = 256
	maxSkillMarkdownLength    = 10000
)

// Service 封装技能业务逻辑。
type Service struct {
	repo        repository.SkillRepository
	auditWriter auditWriter
}

type auditWriter interface {
	Write(ctx context.Context, requestID string, actorUserID uint, action string, resource string, resourceID string, ip string, userAgent string, detail interface{})
}

// NewService 创建技能服务。
func NewService(repo repository.SkillRepository) *Service {
	return &Service{repo: repo}
}

// SetAuditWriter 注入审计写入器。
func (s *Service) SetAuditWriter(writer auditWriter) {
	s.auditWriter = writer
}

// AuditInput 描述技能审计写入。
type AuditInput struct {
	UserID     uint
	RequestID  string
	Action     string
	ResourceID string
	ClientIP   string
	UserAgent  string
	Detail     interface{}
}

// RecordAudit 记录技能审计日志。
func (s *Service) RecordAudit(ctx context.Context, input AuditInput) {
	if s.auditWriter == nil {
		return
	}
	s.auditWriter.Write(
		ctx,
		strings.TrimSpace(input.RequestID),
		input.UserID,
		strings.TrimSpace(input.Action),
		"skills",
		strings.TrimSpace(input.ResourceID),
		strings.TrimSpace(input.ClientIP),
		strings.TrimSpace(input.UserAgent),
		input.Detail,
	)
}

// ListVisible 查询当前用户可使用的技能。
func (s *Service) ListVisible(ctx context.Context, userID uint, input ListInput) ([]domainskill.Skill, int64, error) {
	if userID == 0 {
		return nil, 0, repository.ErrInvalidInput
	}
	page, pageSize := normalizePage(input.Page, input.PageSize)
	return s.repo.ListSkills(ctx, repository.SkillListFilter{
		IDs:           input.IDs,
		Query:         strings.TrimSpace(input.Query),
		VisibleUserID: &userID,
	}, (page-1)*pageSize, pageSize)
}

// ListMine 查询当前用户自定义技能。
func (s *Service) ListMine(ctx context.Context, userID uint, input ListInput) ([]domainskill.Skill, int64, error) {
	if userID == 0 {
		return nil, 0, repository.ErrInvalidInput
	}
	page, pageSize := normalizePage(input.Page, input.PageSize)
	return s.repo.ListSkills(ctx, repository.SkillListFilter{
		Query:          strings.TrimSpace(input.Query),
		SearchMarkdown: true,
		Scope:          domainskill.ScopeUser,
		OwnerUserID:    &userID,
		Enabled:        input.Enabled,
	}, (page-1)*pageSize, pageSize)
}

// ListAdminBuiltin 查询管理员内置技能列表。
func (s *Service) ListAdminBuiltin(ctx context.Context, input ListInput) ([]domainskill.Skill, int64, error) {
	page, pageSize := normalizePage(input.Page, input.PageSize)
	return s.repo.ListSkills(ctx, repository.SkillListFilter{
		Query:          strings.TrimSpace(input.Query),
		SearchMarkdown: true,
		Scope:          domainskill.ScopeBuiltin,
		Enabled:        input.Enabled,
	}, (page-1)*pageSize, pageSize)
}

// ResolveAvailable 查询当前用户可使用的技能。
func (s *Service) ResolveAvailable(ctx context.Context, userID uint, id uint) (*domainskill.Skill, error) {
	if userID == 0 || id == 0 {
		return nil, repository.ErrInvalidInput
	}
	item, err := s.repo.GetSkill(ctx, id)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	if !item.Enabled {
		return nil, ErrSkillNotFound
	}
	if item.Scope == domainskill.ScopeBuiltin {
		return item, nil
	}
	if item.Scope == domainskill.ScopeUser && item.OwnerUserID == userID {
		return item, nil
	}
	return nil, ErrSkillNotFound
}

// CreateUser 创建用户自定义技能。
func (s *Service) CreateUser(ctx context.Context, userID uint, input WriteInput) (*domainskill.Skill, error) {
	if userID == 0 {
		return nil, repository.ErrInvalidInput
	}
	item, err := normalizeWriteInput(input, domainskill.ScopeUser, userID, userID)
	if err != nil {
		return nil, err
	}
	return s.create(ctx, item)
}

// CreateBuiltin 创建管理员内置技能。
func (s *Service) CreateBuiltin(ctx context.Context, actorUserID uint, input WriteInput) (*domainskill.Skill, error) {
	if actorUserID == 0 {
		return nil, repository.ErrInvalidInput
	}
	item, err := normalizeWriteInput(input, domainskill.ScopeBuiltin, 0, actorUserID)
	if err != nil {
		return nil, err
	}
	return s.create(ctx, item)
}

// UpdateUser 更新当前用户自定义技能。
func (s *Service) UpdateUser(ctx context.Context, userID uint, id uint, input PatchInput) (*domainskill.Skill, error) {
	if userID == 0 || id == 0 {
		return nil, repository.ErrInvalidInput
	}
	item, err := s.repo.GetSkill(ctx, id)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	if item.Scope != domainskill.ScopeUser || item.OwnerUserID != userID {
		return nil, ErrSkillNotFound
	}
	return s.update(ctx, id, userID, input)
}

// UpdateBuiltin 更新管理员内置技能。
func (s *Service) UpdateBuiltin(ctx context.Context, actorUserID uint, id uint, input PatchInput) (*domainskill.Skill, error) {
	if actorUserID == 0 || id == 0 {
		return nil, repository.ErrInvalidInput
	}
	item, err := s.repo.GetSkill(ctx, id)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	if item.Scope != domainskill.ScopeBuiltin {
		return nil, ErrSkillNotFound
	}
	return s.update(ctx, id, actorUserID, input)
}

// DeleteUser 删除当前用户自定义技能。
func (s *Service) DeleteUser(ctx context.Context, userID uint, id uint) error {
	if userID == 0 || id == 0 {
		return repository.ErrInvalidInput
	}
	item, err := s.repo.GetSkill(ctx, id)
	if err != nil {
		return mapRepositoryError(err)
	}
	if item.Scope != domainskill.ScopeUser || item.OwnerUserID != userID {
		return ErrSkillNotFound
	}
	return mapRepositoryError(s.repo.DeleteSkill(ctx, id))
}

// DeleteBuiltin 删除管理员内置技能。
func (s *Service) DeleteBuiltin(ctx context.Context, actorUserID uint, id uint) error {
	if actorUserID == 0 || id == 0 {
		return repository.ErrInvalidInput
	}
	item, err := s.repo.GetSkill(ctx, id)
	if err != nil {
		return mapRepositoryError(err)
	}
	if item.Scope != domainskill.ScopeBuiltin {
		return ErrSkillNotFound
	}
	return mapRepositoryError(s.repo.DeleteSkill(ctx, id))
}

// ListInput 定义技能列表入参。
type ListInput struct {
	IDs      []uint
	Query    string
	Enabled  *bool
	Page     int
	PageSize int
}

// WriteInput 定义技能创建入参。
type WriteInput struct {
	Title       string
	Trigger     string
	Description string
	Markdown    string
	Enabled     bool
	SortOrder   int
}

// PatchInput 定义技能更新入参。
type PatchInput struct {
	Title       *string
	Trigger     *string
	Description *string
	Markdown    *string
	Enabled     *bool
	SortOrder   *int
}

func (s *Service) create(ctx context.Context, item *domainskill.Skill) (*domainskill.Skill, error) {
	result, err := s.repo.CreateSkill(ctx, item)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return result, nil
}

func (s *Service) update(ctx context.Context, id uint, actorUserID uint, input PatchInput) (*domainskill.Skill, error) {
	patch, err := normalizePatchInput(input, actorUserID)
	if err != nil {
		return nil, err
	}
	item, err := s.repo.PatchSkill(ctx, id, patch)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return item, nil
}

func normalizeWriteInput(input WriteInput, scope string, ownerUserID uint, actorUserID uint) (*domainskill.Skill, error) {
	title, trigger, description, markdown, err := normalizeFields(input)
	if err != nil {
		return nil, err
	}
	if scope != domainskill.ScopeBuiltin && scope != domainskill.ScopeUser {
		return nil, ErrInvalidSkill
	}
	return &domainskill.Skill{
		Scope:           scope,
		OwnerUserID:     ownerUserID,
		Title:           title,
		Trigger:         trigger,
		Description:     description,
		Markdown:        markdown,
		Enabled:         input.Enabled,
		SortOrder:       input.SortOrder,
		CreatedByUserID: actorUserID,
		UpdatedByUserID: actorUserID,
	}, nil
}

func normalizePatchInput(input PatchInput, actorUserID uint) (repository.SkillPatch, error) {
	patch := repository.SkillPatch{
		UpdatedByUserIDSet: true,
		UpdatedByUserID:    actorUserID,
	}
	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		if title == "" || runeCount(title) > maxSkillTitleLength {
			return repository.SkillPatch{}, ErrInvalidSkill
		}
		patch.Title = &title
	}
	if input.Trigger != nil {
		trigger := normalizeTrigger(*input.Trigger)
		if trigger == "" || runeCount(trigger) > maxSkillTriggerLength {
			return repository.SkillPatch{}, ErrInvalidSkill
		}
		patch.Trigger = &trigger
	}
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		if runeCount(description) > maxSkillDescriptionLength {
			return repository.SkillPatch{}, ErrInvalidSkill
		}
		patch.Description = &description
	}
	if input.Markdown != nil {
		markdown := strings.TrimSpace(*input.Markdown)
		if markdown == "" || runeCount(markdown) > maxSkillMarkdownLength {
			return repository.SkillPatch{}, ErrInvalidSkill
		}
		patch.Markdown = &markdown
	}
	if input.Enabled != nil {
		patch.Enabled = input.Enabled
	}
	if input.SortOrder != nil {
		patch.SortOrder = input.SortOrder
	}
	return patch, nil
}

func normalizeFields(input WriteInput) (string, string, string, string, error) {
	title := strings.TrimSpace(input.Title)
	trigger := normalizeTrigger(input.Trigger)
	description := strings.TrimSpace(input.Description)
	markdown := strings.TrimSpace(input.Markdown)
	if title == "" || runeCount(title) > maxSkillTitleLength {
		return "", "", "", "", ErrInvalidSkill
	}
	if trigger == "" || runeCount(trigger) > maxSkillTriggerLength {
		return "", "", "", "", ErrInvalidSkill
	}
	if runeCount(description) > maxSkillDescriptionLength {
		return "", "", "", "", ErrInvalidSkill
	}
	if markdown == "" || runeCount(markdown) > maxSkillMarkdownLength {
		return "", "", "", "", ErrInvalidSkill
	}
	return title, trigger, description, markdown, nil
}

func normalizeTrigger(value string) string {
	return strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(value), "/"))
}

func runeCount(value string) int {
	return len([]rune(value))
}

func normalizePage(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	const maxPageSize = 100
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
		return ErrSkillNotFound
	}
	if errors.Is(err, repository.ErrDuplicate) {
		return ErrSkillConflict
	}
	if errors.Is(err, repository.ErrInvalidInput) {
		return ErrInvalidSkill
	}
	return err
}
