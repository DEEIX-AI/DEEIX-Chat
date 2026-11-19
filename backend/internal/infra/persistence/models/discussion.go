package models

// DiscussionTemplate 存储后台定义的讨论模型模板。
type DiscussionTemplate struct {
	BaseModel
	PublicID    string `gorm:"size:32;not null;default:'';uniqueIndex:idx_chat_discussion_templates_public_id;comment:公开模板ID"`
	Name        string `gorm:"size:128;not null;default:'';comment:模板名称"`
	Description string `gorm:"size:500;not null;default:'';comment:模板描述"`
	Icon        string `gorm:"size:128;not null;default:'';comment:模板图标"`
	Status      string `gorm:"size:32;not null;default:'active';index:idx_chat_discussion_templates_status;comment:状态(active/inactive)"`
	MaxRounds   int    `gorm:"not null;default:1;comment:最大轮数"`
	ConfigJSON  string `gorm:"type:text;not null;default:'{}';comment:编排配置JSON"`
	SortOrder   int    `gorm:"not null;default:0;index:idx_chat_discussion_templates_sort_order;comment:排序"`
}

// TableName 指定表名。
func (DiscussionTemplate) TableName() string {
	return "chat_discussion_templates"
}
