package promptpreset

import (
	"context"
	"errors"
	"strings"

	domainpromptpreset "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/promptpreset"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const (
	maxPromptPresetNameLength        = 64
	maxPromptPresetDescriptionLength = 256
	maxPromptPresetContentLength     = 10000
)

// Service 封装预制提示词业务逻辑。
type Service struct {
	repo        repository.PromptPresetRepository
	auditWriter auditWriter
}

type auditWriter interface {
	Write(ctx context.Context, requestID string, actorUserID uint, action string, resource string, resourceID string, ip string, userAgent string, detail interface{})
}

// NewService 创建预制提示词服务。
func NewService(repo repository.PromptPresetRepository) *Service {
	return &Service{repo: repo}
}

// SetAuditWriter 注入审计写入器。
func (s *Service) SetAuditWriter(writer auditWriter) {
	s.auditWriter = writer
}

// AuditInput 描述预制提示词审计写入。
type AuditInput struct {
	UserID     uint
	RequestID  string
	Action     string
	ResourceID string
	ClientIP   string
	UserAgent  string
	Detail     interface{}
}

// RecordAudit 记录预制提示词审计日志。
func (s *Service) RecordAudit(ctx context.Context, input AuditInput) {
	if s.auditWriter == nil {
		return
	}
	s.auditWriter.Write(
		ctx,
		strings.TrimSpace(input.RequestID),
		input.UserID,
		strings.TrimSpace(input.Action),
		"prompt_presets",
		strings.TrimSpace(input.ResourceID),
		strings.TrimSpace(input.ClientIP),
		strings.TrimSpace(input.UserAgent),
		input.Detail,
	)
}

// ListVisible 查询当前用户可在 slash 选择器中使用的提示词。
func (s *Service) ListVisible(ctx context.Context, userID uint, input ListInput) ([]domainpromptpreset.PromptPreset, int64, error) {
	if userID == 0 {
		return nil, 0, repository.ErrInvalidInput
	}
	page, pageSize := normalizePage(input.Page, input.PageSize)
	return s.repo.ListPromptPresets(ctx, repository.PromptPresetListFilter{
		Query:         strings.TrimSpace(input.Query),
		VisibleUserID: &userID,
	}, (page-1)*pageSize, pageSize)
}

// ListMine 查询当前用户自定义提示词。
func (s *Service) ListMine(ctx context.Context, userID uint, input ListInput) ([]domainpromptpreset.PromptPreset, int64, error) {
	if userID == 0 {
		return nil, 0, repository.ErrInvalidInput
	}
	page, pageSize := normalizePage(input.Page, input.PageSize)
	return s.repo.ListPromptPresets(ctx, repository.PromptPresetListFilter{
		Query:       strings.TrimSpace(input.Query),
		Scope:       domainpromptpreset.ScopeUser,
		OwnerUserID: &userID,
		Enabled:     input.Enabled,
	}, (page-1)*pageSize, pageSize)
}

// ListAdminBuiltin 查询管理员内置提示词列表。
func (s *Service) ListAdminBuiltin(ctx context.Context, input ListInput) ([]domainpromptpreset.PromptPreset, int64, error) {
	page, pageSize := normalizePage(input.Page, input.PageSize)
	return s.repo.ListPromptPresets(ctx, repository.PromptPresetListFilter{
		Query:   strings.TrimSpace(input.Query),
		Scope:   domainpromptpreset.ScopeBuiltin,
		Enabled: input.Enabled,
	}, (page-1)*pageSize, pageSize)
}

// CreateUser 创建用户自定义提示词。
func (s *Service) CreateUser(ctx context.Context, userID uint, input WriteInput) (*domainpromptpreset.PromptPreset, error) {
	if userID == 0 {
		return nil, repository.ErrInvalidInput
	}
	item, err := normalizeWriteInput(input, domainpromptpreset.ScopeUser, userID, userID)
	if err != nil {
		return nil, err
	}
	return s.create(ctx, item)
}

// CreateBuiltin 创建管理员内置提示词。
func (s *Service) CreateBuiltin(ctx context.Context, actorUserID uint, input WriteInput) (*domainpromptpreset.PromptPreset, error) {
	if actorUserID == 0 {
		return nil, repository.ErrInvalidInput
	}
	item, err := normalizeWriteInput(input, domainpromptpreset.ScopeBuiltin, 0, actorUserID)
	if err != nil {
		return nil, err
	}
	return s.create(ctx, item)
}

// UpdateUser 更新当前用户自定义提示词。
func (s *Service) UpdateUser(ctx context.Context, userID uint, id uint, input PatchInput) (*domainpromptpreset.PromptPreset, error) {
	if userID == 0 || id == 0 {
		return nil, repository.ErrInvalidInput
	}
	item, err := s.repo.GetPromptPreset(ctx, id)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	if item.Scope != domainpromptpreset.ScopeUser || item.OwnerUserID != userID {
		return nil, ErrPromptPresetNotFound
	}
	return s.update(ctx, id, userID, input)
}

// UpdateBuiltin 更新管理员内置提示词。
func (s *Service) UpdateBuiltin(ctx context.Context, actorUserID uint, id uint, input PatchInput) (*domainpromptpreset.PromptPreset, error) {
	if actorUserID == 0 || id == 0 {
		return nil, repository.ErrInvalidInput
	}
	item, err := s.repo.GetPromptPreset(ctx, id)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	if item.Scope != domainpromptpreset.ScopeBuiltin {
		return nil, ErrPromptPresetNotFound
	}
	return s.update(ctx, id, actorUserID, input)
}

// DeleteUser 删除当前用户自定义提示词。
func (s *Service) DeleteUser(ctx context.Context, userID uint, id uint) error {
	if userID == 0 || id == 0 {
		return repository.ErrInvalidInput
	}
	item, err := s.repo.GetPromptPreset(ctx, id)
	if err != nil {
		return mapRepositoryError(err)
	}
	if item.Scope != domainpromptpreset.ScopeUser || item.OwnerUserID != userID {
		return ErrPromptPresetNotFound
	}
	return mapRepositoryError(s.repo.DeletePromptPreset(ctx, id))
}

// DeleteBuiltin 删除管理员内置提示词。
func (s *Service) DeleteBuiltin(ctx context.Context, actorUserID uint, id uint) error {
	if actorUserID == 0 || id == 0 {
		return repository.ErrInvalidInput
	}
	item, err := s.repo.GetPromptPreset(ctx, id)
	if err != nil {
		return mapRepositoryError(err)
	}
	if item.Scope != domainpromptpreset.ScopeBuiltin {
		return ErrPromptPresetNotFound
	}
	return mapRepositoryError(s.repo.DeletePromptPreset(ctx, id))
}

// ListInput 定义预制提示词列表入参。
type ListInput struct {
	Query    string
	Enabled  *bool
	Page     int
	PageSize int
}

// WriteInput 定义预制提示词创建入参。
type WriteInput struct {
	Title       string
	Trigger     string
	Description string
	Content     string
	Enabled     bool
	SortOrder   int
}

// PatchInput 定义预制提示词更新入参。
type PatchInput struct {
	Title       *string
	Trigger     *string
	Description *string
	Content     *string
	Enabled     *bool
	SortOrder   *int
}

func (s *Service) create(ctx context.Context, item *domainpromptpreset.PromptPreset) (*domainpromptpreset.PromptPreset, error) {
	result, err := s.repo.CreatePromptPreset(ctx, item)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return result, nil
}

func (s *Service) update(ctx context.Context, id uint, actorUserID uint, input PatchInput) (*domainpromptpreset.PromptPreset, error) {
	patch, err := normalizePatchInput(input, actorUserID)
	if err != nil {
		return nil, err
	}
	item, err := s.repo.PatchPromptPreset(ctx, id, patch)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return item, nil
}

func normalizeWriteInput(input WriteInput, scope string, ownerUserID uint, actorUserID uint) (*domainpromptpreset.PromptPreset, error) {
	title, trigger, description, content, err := normalizeFields(
		input.Title,
		input.Trigger,
		input.Description,
		input.Content,
	)
	if err != nil {
		return nil, err
	}
	if scope != domainpromptpreset.ScopeBuiltin && scope != domainpromptpreset.ScopeUser {
		return nil, ErrInvalidPromptPreset
	}
	return &domainpromptpreset.PromptPreset{
		Scope:           scope,
		OwnerUserID:     ownerUserID,
		Title:           title,
		Trigger:         trigger,
		Description:     description,
		Content:         content,
		Enabled:         input.Enabled,
		SortOrder:       input.SortOrder,
		CreatedByUserID: actorUserID,
		UpdatedByUserID: actorUserID,
	}, nil
}

func normalizePatchInput(input PatchInput, actorUserID uint) (repository.PromptPresetPatch, error) {
	patch := repository.PromptPresetPatch{
		UpdatedByUserIDSet: true,
		UpdatedByUserID:    actorUserID,
	}
	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		if title == "" || runeCount(title) > maxPromptPresetNameLength {
			return repository.PromptPresetPatch{}, ErrInvalidPromptPreset
		}
		patch.Title = &title
	}
	if input.Trigger != nil {
		trigger := normalizeTrigger(*input.Trigger)
		if trigger == "" || runeCount(trigger) > maxPromptPresetNameLength {
			return repository.PromptPresetPatch{}, ErrInvalidPromptPreset
		}
		patch.Trigger = &trigger
	}
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		if runeCount(description) > maxPromptPresetDescriptionLength {
			return repository.PromptPresetPatch{}, ErrInvalidPromptPreset
		}
		patch.Description = &description
	}
	if input.Content != nil {
		content := strings.TrimSpace(*input.Content)
		if content == "" || runeCount(content) > maxPromptPresetContentLength {
			return repository.PromptPresetPatch{}, ErrInvalidPromptPreset
		}
		patch.Content = &content
	}
	if input.Enabled != nil {
		patch.Enabled = input.Enabled
	}
	if input.SortOrder != nil {
		patch.SortOrder = input.SortOrder
	}
	return patch, nil
}

func normalizeFields(titleValue string, triggerValue string, descriptionValue string, contentValue string) (string, string, string, string, error) {
	title := strings.TrimSpace(titleValue)
	trigger := normalizeTrigger(triggerValue)
	description := strings.TrimSpace(descriptionValue)
	content := strings.TrimSpace(contentValue)
	if title == "" || runeCount(title) > maxPromptPresetNameLength {
		return "", "", "", "", ErrInvalidPromptPreset
	}
	if trigger == "" || runeCount(trigger) > maxPromptPresetNameLength {
		return "", "", "", "", ErrInvalidPromptPreset
	}
	if runeCount(description) > maxPromptPresetDescriptionLength {
		return "", "", "", "", ErrInvalidPromptPreset
	}
	if content == "" || runeCount(content) > maxPromptPresetContentLength {
		return "", "", "", "", ErrInvalidPromptPreset
	}
	return title, trigger, description, content, nil
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
		return ErrPromptPresetNotFound
	}
	if errors.Is(err, repository.ErrDuplicate) {
		return ErrPromptPresetConflict
	}
	if errors.Is(err, repository.ErrInvalidInput) {
		return ErrInvalidPromptPreset
	}
	return err
}
