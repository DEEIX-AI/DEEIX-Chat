package repository

import (
	"context"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
)

// PermissionGroupRepository 定义权限组（模型访问控制）依赖的仓储能力。
type PermissionGroupRepository interface {
	ListPermissionGroups(ctx context.Context) ([]domainchannel.PermissionGroup, error)
	GetPermissionGroup(ctx context.Context, id uint) (*domainchannel.PermissionGroup, error)
	CreatePermissionGroup(ctx context.Context, item *domainchannel.PermissionGroup) error
	UpdatePermissionGroup(ctx context.Context, id uint, name string, description string, rateMultiplierPercent int) (*domainchannel.PermissionGroup, error)
	DeletePermissionGroup(ctx context.Context, id uint) error

	ListGroupModelIDs(ctx context.Context, groupID uint) ([]uint, error)
	SetGroupModels(ctx context.Context, groupID uint, modelIDs []uint) error

	ListGroupUserIDs(ctx context.Context, groupID uint) ([]uint, error)
	SetGroupUsers(ctx context.Context, groupID uint, userIDs []uint) error

	ListUserGroupIDs(ctx context.Context, userID uint) ([]uint, error)
	ListModelGroupIDs(ctx context.Context, platformModelID uint) ([]uint, error)
	ListModelsWithGroupAccess(ctx context.Context) (map[uint][]uint, error)
	ListDefaultGroupIDs(ctx context.Context) ([]uint, error)
	GetUserGroupRateMultiplierPercent(ctx context.Context, userID uint, extraGroupIDs []uint) (int, error)
	CountBillingPlansWithGroupID(ctx context.Context, groupID uint) (int64, error)
}
