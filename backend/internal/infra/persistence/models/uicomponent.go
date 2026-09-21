package model

// UIComponent 对应 ui_components 表，存放可供模型输出、前端渲染的交互式组件。
type UIComponent struct {
	ControlPlaneModel
	Scope           string `gorm:"size:32;not null;default:'user';index:idx_ui_components_scope;uniqueIndex:idx_ui_components_scope_owner_name;comment:作用域(builtin/platform/user)"`
	OwnerUserID     uint   `gorm:"not null;default:0;index:idx_ui_components_owner;uniqueIndex:idx_ui_components_scope_owner_name;comment:所属用户ID，内置与平台组件为0"`
	Name            string `gorm:"size:64;not null;default:'';uniqueIndex:idx_ui_components_scope_owner_name;comment:渲染标记名"`
	Version         int    `gorm:"not null;comment:入参契约版本"`
	Description     string `gorm:"size:256;not null;default:'';comment:给模型看的组件说明"`
	PropsSummary    string `gorm:"size:1024;not null;default:'';comment:给模型看的入参摘要"`
	PropsSchema     string `gorm:"type:text;not null;default:'';comment:JSON Schema，前端校验自定义组件入参"`
	RendererKind    string `gorm:"size:16;not null;default:'sandbox';comment:渲染方式(builtin/sandbox)"`
	RendererSource  string `gorm:"type:text;not null;default:'';comment:sandbox渲染源HTML"`
	Enabled         bool   `gorm:"not null;index:idx_ui_components_enabled;comment:是否启用"`
	SortOrder       int    `gorm:"not null;default:0;index:idx_ui_components_sort_order;comment:排序值"`
	CreatedByUserID uint   `gorm:"not null;default:0;comment:创建人ID"`
	UpdatedByUserID uint   `gorm:"not null;default:0;comment:最后更新人ID"`
}

// TableName 指定表名。
func (UIComponent) TableName() string {
	return "ui_components"
}
