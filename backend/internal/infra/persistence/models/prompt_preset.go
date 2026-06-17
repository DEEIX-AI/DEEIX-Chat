package model

// PromptPreset 记录内置和用户自定义预制提示词。
type PromptPreset struct {
	ControlPlaneModel
	Scope           string `gorm:"size:32;not null;default:'user';index:idx_prompt_presets_scope;uniqueIndex:idx_prompt_presets_scope_owner_trigger;comment:作用域(builtin/user)"`
	OwnerUserID     uint   `gorm:"not null;default:0;index:idx_prompt_presets_owner;uniqueIndex:idx_prompt_presets_scope_owner_trigger;comment:所属用户ID，内置提示词为0"`
	Title           string `gorm:"size:64;not null;default:'';comment:提示词标题"`
	Trigger         string `gorm:"size:64;not null;default:'';index:idx_prompt_presets_trigger;uniqueIndex:idx_prompt_presets_scope_owner_trigger;comment:slash触发词，不含斜杠"`
	Description     string `gorm:"size:256;not null;default:'';comment:提示词说明"`
	Content         string `gorm:"type:text;not null;default:'';comment:提示词内容"`
	Enabled         bool   `gorm:"not null;default:true;index:idx_prompt_presets_enabled;comment:是否启用"`
	SortOrder       int    `gorm:"not null;default:0;index:idx_prompt_presets_sort_order;comment:排序值"`
	CreatedByUserID uint   `gorm:"not null;default:0;index:idx_prompt_presets_created_by;comment:创建人ID"`
	UpdatedByUserID uint   `gorm:"not null;default:0;index:idx_prompt_presets_updated_by;comment:最后更新人ID"`
}

// TableName 指定表名。
func (PromptPreset) TableName() string {
	return "prompt_presets"
}
