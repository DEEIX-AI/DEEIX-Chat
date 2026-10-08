package repository

import (
	"context"
	"time"

	domainpersonalprovider "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/personalprovider"
)

// PersonalProviderRepository 定义用户自带 Key 模型服务的持久化能力。
// 用户侧读写一律带 OwnerUserID 条件，归属校验在仓储查询中完成，而不是事后比对。
type PersonalProviderRepository interface {
	ListByOwner(ctx context.Context, ownerUserID uint) ([]domainpersonalprovider.Provider, error)
	CountByOwner(ctx context.Context, ownerUserID uint) (int64, error)
	GetByOwner(ctx context.Context, ownerUserID uint, publicID string) (*domainpersonalprovider.Provider, error)
	Create(ctx context.Context, item *domainpersonalprovider.Provider) (*domainpersonalprovider.Provider, error)
	UpdateByOwner(ctx context.Context, ownerUserID uint, publicID string, patch PersonalProviderPatch) (*domainpersonalprovider.Provider, error)
	DeleteByOwner(ctx context.Context, ownerUserID uint, publicID string) error

	// 管理员治理：只读元数据与状态变更，不提供读取或修改 Key 的入口。
	ListForAdmin(ctx context.Context, filter PersonalProviderAdminFilter, offset int, limit int) ([]domainpersonalprovider.Provider, int64, error)
	SetStatusByPublicIDs(ctx context.Context, publicIDs []string, status string) (int64, error)
	SetStatusByHost(ctx context.Context, host string, status string) (int64, error)
	DeleteByPublicIDs(ctx context.Context, publicIDs []string) (int64, error)
}

// PersonalProviderPatch 描述可更新的字段；nil 表示不修改。
type PersonalProviderPatch struct {
	Name          *string
	Icon          *string
	APIKeyEnc     *string
	KeyHint       *string
	Models        *[]string
	Status        *string
	LastError     *string
	LastCheckedAt *time.Time
}

// PersonalProviderAdminFilter 描述管理员列表的筛选条件。
type PersonalProviderAdminFilter struct {
	Query       string
	Status      string
	Host        string
	OwnerUserID uint
}
