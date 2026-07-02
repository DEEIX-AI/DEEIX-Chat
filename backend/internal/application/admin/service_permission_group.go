package admin

import (
	"context"
	"regexp"
	"strings"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
)

var permissionGroupCodePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type permissionGroupRepo interface {
	ListPermissionGroups(ctx context.Context) ([]domainchannel.PermissionGroup, error)
	GetPermissionGroup(ctx context.Context, id uint) (*domainchannel.PermissionGroup, error)
	CreatePermissionGroup(ctx context.Context, item *domainchannel.PermissionGroup) error
	UpdatePermissionGroup(ctx context.Context, id uint, name string, description string) (*domainchannel.PermissionGroup, error)
	DeletePermissionGroup(ctx context.Context, id uint) error
	ListGroupModelIDs(ctx context.Context, groupID uint) ([]uint, error)
	SetGroupModels(ctx context.Context, groupID uint, modelIDs []uint) error
	ListGroupUserIDs(ctx context.Context, groupID uint) ([]uint, error)
	SetGroupUsers(ctx context.Context, groupID uint, userIDs []uint) error
}

// SetPermissionGroupRepo 注入权限组仓储能力。
func (s *Service) SetPermissionGroupRepo(repo permissionGroupRepo) {
	s.permissionGroupRepo = repo
}

// ListPermissionGroups 返回全部权限组。
func (s *Service) ListPermissionGroups(ctx context.Context) ([]domainchannel.PermissionGroup, error) {
	if s.permissionGroupRepo == nil {
		return nil, ErrPermissionGroupRepoUnavailable
	}
	return s.permissionGroupRepo.ListPermissionGroups(ctx)
}

// CreatePermissionGroup 创建权限组。
func (s *Service) CreatePermissionGroup(ctx context.Context, code, name, description string) (*domainchannel.PermissionGroup, error) {
	if s.permissionGroupRepo == nil {
		return nil, ErrPermissionGroupRepoUnavailable
	}
	normalizedCode := strings.ToLower(strings.TrimSpace(code))
	if !permissionGroupCodePattern.MatchString(normalizedCode) {
		return nil, ErrInvalidPermissionGroupCode
	}
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, ErrInvalidPermissionGroupName
	}
	item := &domainchannel.PermissionGroup{
		Code:        normalizedCode,
		Name:        trimmedName,
		Description: strings.TrimSpace(description),
	}
	if err := s.permissionGroupRepo.CreatePermissionGroup(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

// UpdatePermissionGroup 更新权限组名称与说明。
func (s *Service) UpdatePermissionGroup(ctx context.Context, id uint, name, description string) (*domainchannel.PermissionGroup, error) {
	if s.permissionGroupRepo == nil {
		return nil, ErrPermissionGroupRepoUnavailable
	}
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, ErrInvalidPermissionGroupName
	}
	return s.permissionGroupRepo.UpdatePermissionGroup(ctx, id, trimmedName, strings.TrimSpace(description))
}

// DeletePermissionGroup 删除权限组，默认组不可删除。
func (s *Service) DeletePermissionGroup(ctx context.Context, id uint) error {
	if s.permissionGroupRepo == nil {
		return ErrPermissionGroupRepoUnavailable
	}
	group, err := s.permissionGroupRepo.GetPermissionGroup(ctx, id)
	if err != nil {
		return err
	}
	if group.IsDefault {
		return ErrDefaultPermissionGroupDeleteNotAllowed
	}
	return s.permissionGroupRepo.DeletePermissionGroup(ctx, id)
}

// ListGroupModels 返回权限组授权的平台模型 ID。
func (s *Service) ListGroupModels(ctx context.Context, groupID uint) ([]uint, error) {
	if s.permissionGroupRepo == nil {
		return nil, ErrPermissionGroupRepoUnavailable
	}
	return s.permissionGroupRepo.ListGroupModelIDs(ctx, groupID)
}

// SetGroupModels 全量替换权限组授权的平台模型集合。
func (s *Service) SetGroupModels(ctx context.Context, groupID uint, modelIDs []uint) error {
	if s.permissionGroupRepo == nil {
		return ErrPermissionGroupRepoUnavailable
	}
	if _, err := s.permissionGroupRepo.GetPermissionGroup(ctx, groupID); err != nil {
		return err
	}
	return s.permissionGroupRepo.SetGroupModels(ctx, groupID, modelIDs)
}

// ListGroupUsers 返回权限组内的用户 ID。
func (s *Service) ListGroupUsers(ctx context.Context, groupID uint) ([]uint, error) {
	if s.permissionGroupRepo == nil {
		return nil, ErrPermissionGroupRepoUnavailable
	}
	return s.permissionGroupRepo.ListGroupUserIDs(ctx, groupID)
}

// SetGroupUsers 全量替换权限组内的用户集合。
func (s *Service) SetGroupUsers(ctx context.Context, groupID uint, userIDs []uint) error {
	if s.permissionGroupRepo == nil {
		return ErrPermissionGroupRepoUnavailable
	}
	if _, err := s.permissionGroupRepo.GetPermissionGroup(ctx, groupID); err != nil {
		return err
	}
	return s.permissionGroupRepo.SetGroupUsers(ctx, groupID, userIDs)
}
