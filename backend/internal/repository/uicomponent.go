package repository

import (
	"context"

	domainuicomponent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/uicomponent"
)

// UIComponentRepository 定义交互式组件持久化能力。
type UIComponentRepository interface {
	ListUIComponents(ctx context.Context, filter UIComponentListFilter, offset int, limit int) ([]domainuicomponent.Component, int64, error)
	GetUIComponent(ctx context.Context, id uint) (*domainuicomponent.Component, error)
	CreateUIComponent(ctx context.Context, item *domainuicomponent.Component) (*domainuicomponent.Component, error)
	PatchUIComponent(ctx context.Context, id uint, patch UIComponentPatch) (*domainuicomponent.Component, error)
	DeleteUIComponent(ctx context.Context, id uint) error
}

// UIComponentListFilter 描述组件列表筛选条件。
type UIComponentListFilter struct {
	IDs           []uint
	Query         string
	Scope         string
	OwnerUserID   *uint
	Enabled       *bool
	VisibleUserID *uint
}

// UIComponentPatch 描述可更新的组件字段。
type UIComponentPatch struct {
	Name               *string
	Version            *int
	Description        *string
	PropsSummary       *string
	PropsSchema        *string
	RendererSource     *string
	Enabled            *bool
	SortOrder          *int
	UpdatedByUserIDSet bool
	UpdatedByUserID    uint
}
