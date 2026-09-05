package channel

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
)

var (
	modelVendorKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	modelIconSlugPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
)

// ListModelVendors 分页查询技术厂商目录。
func (s *Service) ListModelVendors(ctx context.Context, page int, pageSize int, query string) ([]ModelVendorView, int64, error) {
	offset, limit := pagination.Offset(page, pageSize)
	items, total, err := s.presentationRepo.ListModelVendors(ctx, repository.ListModelVendorsInput{
		Offset: offset,
		Limit:  limit,
		Query:  strings.TrimSpace(query),
	})
	if err != nil {
		return nil, 0, err
	}
	views := make([]ModelVendorView, 0, len(items))
	for _, item := range items {
		views = append(views, toModelVendorView(item))
	}
	return views, total, nil
}

// CreateModelVendor 创建可供平台模型引用的技术厂商。
func (s *Service) CreateModelVendor(ctx context.Context, input CreateModelVendorInput) (*ModelVendorView, error) {
	key, err := normalizeModelVendorKey(input.Key)
	if err != nil {
		return nil, err
	}
	name, err := normalizeModelPresentationName(input.Name, ErrInvalidModelVendor)
	if err != nil {
		return nil, err
	}
	icon, err := normalizeModelPresentationIcon(input.Icon)
	if err != nil {
		return nil, err
	}
	if err = s.reserveModelIconReference(ctx, icon); err != nil {
		return nil, err
	}
	item := &domainchannel.ModelVendor{Key: key, Name: name, Icon: icon}
	if err := s.presentationRepo.CreateModelVendor(ctx, item); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, ErrModelVendorConflict
		}
		return nil, err
	}
	view := toModelVendorView(*item)
	return &view, nil
}

// UpdateModelVendor 更新技术厂商的展示名称和图标，稳定 key 保持不变。
func (s *Service) UpdateModelVendor(ctx context.Context, key string, input UpdateModelVendorInput) (*ModelVendorView, error) {
	normalizedKey, err := normalizeModelVendorKey(key)
	if err != nil {
		return nil, err
	}
	update := repository.UpdateModelVendorInput{}
	if input.Name != nil {
		name, err := normalizeModelPresentationName(*input.Name, ErrInvalidModelVendor)
		if err != nil {
			return nil, err
		}
		update.Name = &name
	}
	if input.Icon != nil {
		icon, err := normalizeModelPresentationIcon(*input.Icon)
		if err != nil {
			return nil, err
		}
		if err = s.reserveModelIconReference(ctx, icon); err != nil {
			return nil, err
		}
		update.Icon = &icon
	}
	if err := s.presentationRepo.UpdateModelVendor(ctx, normalizedKey, update); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return nil, ErrModelVendorNotFound
		default:
			return nil, err
		}
	}
	item, err := s.presentationRepo.GetModelVendorByKey(ctx, normalizedKey)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrModelVendorNotFound
		}
		return nil, err
	}
	s.InvalidateModelCatalog()
	view := toModelVendorView(*item)
	return &view, nil
}

// DeleteModelVendor 删除未被平台模型引用的自定义技术厂商。
func (s *Service) DeleteModelVendor(ctx context.Context, key string) error {
	normalizedKey, err := normalizeModelVendorKey(key)
	if err != nil {
		return err
	}
	if err = s.presentationRepo.DeleteModelVendor(ctx, normalizedKey); err != nil {
		var blocked *repository.ModelVendorDeleteBlockedError
		switch {
		case errors.As(err, &blocked):
			result := &ModelVendorDeleteBlockedError{
				Reason: blocked.Reason, ReferenceCount: blocked.ReferenceCount,
				Models: make([]ModelVendorReferenceView, 0, len(blocked.Models)),
			}
			for _, model := range blocked.Models {
				result.Models = append(result.Models, ModelVendorReferenceView{
					ID: model.ID, PlatformModelName: model.PlatformModelName,
				})
			}
			return result
		case errors.Is(err, repository.ErrModelVendorNotFound), errors.Is(err, repository.ErrNotFound):
			return ErrModelVendorNotFound
		default:
			return err
		}
	}
	s.InvalidateModelCatalog()
	return nil
}

// ListModelDisplayGroups 分页查询管理员创建的模型展示分组。
func (s *Service) ListModelDisplayGroups(ctx context.Context, page int, pageSize int, query string) ([]ModelDisplayGroupView, int64, error) {
	offset, limit := pagination.Offset(page, pageSize)
	items, total, err := s.presentationRepo.ListModelDisplayGroups(ctx, repository.ListModelDisplayGroupsInput{
		Offset: offset,
		Limit:  limit,
		Query:  strings.TrimSpace(query),
	})
	if err != nil {
		return nil, 0, err
	}
	views := make([]ModelDisplayGroupView, 0, len(items))
	for _, item := range items {
		views = append(views, toModelDisplayGroupView(item))
	}
	return views, total, nil
}

// CreateModelDisplayGroup 创建可选展示分组；未绑定分组的模型仍按技术厂商展示。
func (s *Service) CreateModelDisplayGroup(ctx context.Context, input CreateModelDisplayGroupInput) (*ModelDisplayGroupView, error) {
	name, err := normalizeModelPresentationName(input.Name, ErrInvalidModelDisplayGroup)
	if err != nil {
		return nil, err
	}
	icon, err := normalizeModelPresentationIcon(input.Icon)
	if err != nil {
		return nil, err
	}
	modelIDs, err := normalizeModelDisplayGroupMembers(input.ModelIDs)
	if err != nil {
		return nil, err
	}
	if err = s.reserveModelIconReference(ctx, icon); err != nil {
		return nil, err
	}
	item := &domainchannel.ModelDisplayGroup{Name: name, Icon: icon}
	if err := s.presentationRepo.CreateModelDisplayGroup(ctx, item, modelIDs); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, ErrModelDisplayGroupConflict
		}
		if errors.Is(err, repository.ErrInvalidInput) {
			return nil, ErrInvalidModelDisplayGroup
		}
		return nil, err
	}
	s.InvalidateModelCatalog()
	view := toModelDisplayGroupView(*item)
	return &view, nil
}

// UpdateModelDisplayGroup 更新自定义模型展示分组。
func (s *Service) UpdateModelDisplayGroup(ctx context.Context, groupID uint, input UpdateModelDisplayGroupInput) (*ModelDisplayGroupView, error) {
	if groupID == 0 {
		return nil, ErrInvalidModelDisplayGroup
	}
	update := repository.UpdateModelDisplayGroupInput{}
	if input.Name != nil {
		name, err := normalizeModelPresentationName(*input.Name, ErrInvalidModelDisplayGroup)
		if err != nil {
			return nil, err
		}
		update.Name = &name
	}
	if input.Icon != nil {
		icon, err := normalizeModelPresentationIcon(*input.Icon)
		if err != nil {
			return nil, err
		}
		update.Icon = &icon
	}
	if input.ModelIDs != nil {
		modelIDs, err := normalizeModelDisplayGroupMembers(*input.ModelIDs)
		if err != nil {
			return nil, err
		}
		update.ModelIDs = &modelIDs
	}
	if update.Icon != nil {
		if err := s.reserveModelIconReference(ctx, *update.Icon); err != nil {
			return nil, err
		}
	}
	if err := s.presentationRepo.UpdateModelDisplayGroup(ctx, groupID, update); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return nil, ErrModelDisplayGroupNotFound
		case errors.Is(err, repository.ErrDuplicate):
			return nil, ErrModelDisplayGroupConflict
		case errors.Is(err, repository.ErrInvalidInput):
			return nil, ErrInvalidModelDisplayGroup
		default:
			return nil, err
		}
	}
	item, err := s.presentationRepo.GetModelDisplayGroupByID(ctx, groupID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrModelDisplayGroupNotFound
		}
		return nil, err
	}
	s.InvalidateModelCatalog()
	view := toModelDisplayGroupView(*item)
	return &view, nil
}

func normalizeModelDisplayGroupMembers(modelIDs []uint) ([]uint, error) {
	seen := make(map[uint]struct{}, len(modelIDs))
	normalized := make([]uint, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		if modelID == 0 {
			return nil, ErrInvalidModelDisplayGroup
		}
		if _, exists := seen[modelID]; exists {
			continue
		}
		seen[modelID] = struct{}{}
		normalized = append(normalized, modelID)
	}
	return normalized, nil
}

// SetModelsDisplayGroup 批量设置模型的自定义展示分组；groupID 为 0 时恢复按技术厂商展示。
func (s *Service) SetModelsDisplayGroup(ctx context.Context, modelIDs []uint, groupID uint) error {
	normalizedIDs, err := normalizeModelDisplayGroupMembers(modelIDs)
	if err != nil || len(normalizedIDs) == 0 {
		return ErrInvalidModelDisplayGroup
	}
	if err = s.presentationRepo.SetModelsDisplayGroup(ctx, normalizedIDs, groupID); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return ErrModelDisplayGroupNotFound
		case errors.Is(err, repository.ErrInvalidInput):
			return ErrInvalidModelDisplayGroup
		default:
			return err
		}
	}
	s.InvalidateModelCatalog()
	return nil
}

// DeleteModelDisplayGroup 删除自定义展示分组，关联模型恢复按技术厂商展示。
func (s *Service) DeleteModelDisplayGroup(ctx context.Context, groupID uint) error {
	if groupID == 0 {
		return ErrInvalidModelDisplayGroup
	}
	if err := s.presentationRepo.DeleteModelDisplayGroup(ctx, groupID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrModelDisplayGroupNotFound
		}
		return err
	}
	s.InvalidateModelCatalog()
	return nil
}

func normalizeModelVendorKey(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if canonical := canonicalVendorKey(value); canonical != "" {
		return canonical, nil
	}
	if !modelVendorKeyPattern.MatchString(value) {
		return "", ErrInvalidModelVendor
	}
	return value, nil
}

func normalizeModelPresentationName(raw string, invalidErr error) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || len([]rune(value)) > 64 {
		return "", invalidErr
	}
	return value, nil
}

func normalizeModelPresentationIcon(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	if len([]rune(value)) > 2048 || strings.ContainsFunc(value, unicode.IsControl) {
		return "", ErrInvalidModelIconReference
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, ModelIconAssetRefPrefix) {
		publicID, ok := modelIconAssetPublicID(lower)
		if !ok {
			return "", ErrInvalidModelIconReference
		}
		return ModelIconAssetRefPrefix + publicID, nil
	}
	if strings.HasPrefix(lower, "data:") {
		return "", ErrInvalidModelIconReference
	}
	if modelIconSlugPattern.MatchString(lower) {
		return lower, nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil {
		return "", ErrInvalidModelIconReference
	}
	scheme := strings.ToLower(parsed.Scheme)
	if (scheme == "http" || scheme == "https") && parsed.Host != "" {
		return value, nil
	}
	if parsed.Scheme == "" && parsed.Host == "" && strings.HasPrefix(parsed.Path, "/") &&
		!strings.HasPrefix(value, "//") && !strings.Contains(value, `\`) {
		return value, nil
	}
	return "", ErrInvalidModelIconReference
}

func (s *Service) resolvePlatformModelVendor(ctx context.Context, explicit string, candidates ...string) (string, error) {
	var key string
	var err error
	if strings.TrimSpace(explicit) == "" {
		key = normalizeModelVendor("", candidates...)
	} else {
		key, err = normalizeModelVendorKey(explicit)
		if err != nil {
			return "", err
		}
	}
	if _, err := s.presentationRepo.GetModelVendorByKey(ctx, key); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", ErrModelVendorNotFound
		}
		return "", err
	}
	return key, nil
}

func (s *Service) validateModelDisplayGroup(ctx context.Context, groupID uint) error {
	if groupID == 0 {
		return nil
	}
	if _, err := s.presentationRepo.GetModelDisplayGroupByID(ctx, groupID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrModelDisplayGroupNotFound
		}
		return err
	}
	return nil
}

func toModelVendorView(item domainchannel.ModelVendor) ModelVendorView {
	return ModelVendorView{
		ID: item.ID, Key: item.Key, Name: item.Name, Icon: item.Icon,
		BuiltIn: item.BuiltIn, SortOrder: item.SortOrder,
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
		UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
	}
}

func toModelDisplayGroupView(item domainchannel.ModelDisplayGroup) ModelDisplayGroupView {
	return ModelDisplayGroupView{
		ID: item.ID, Name: item.Name, Icon: item.Icon, SortOrder: item.SortOrder,
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
		UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
	}
}
