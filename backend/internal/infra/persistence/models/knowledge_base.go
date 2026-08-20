package model

import "time"

// KnowledgeBase 记录平台内置和用户个人知识库。
type KnowledgeBase struct {
	ControlPlaneModel
	PublicID        string `gorm:"size:32;not null;default:'';uniqueIndex:idx_knowledge_bases_public_id;comment:知识库公开ID"`
	Scope           string `gorm:"size:32;not null;default:'user';index:idx_knowledge_bases_scope;uniqueIndex:idx_knowledge_bases_scope_owner_name,priority:1;comment:作用域(builtin/user)"`
	OwnerUserID     uint   `gorm:"not null;default:0;index:idx_knowledge_bases_owner;uniqueIndex:idx_knowledge_bases_scope_owner_name,priority:2;comment:所属用户ID，内置知识库为0"`
	Name            string `gorm:"size:80;not null;default:'';uniqueIndex:idx_knowledge_bases_scope_owner_name,priority:3;comment:知识库名称"`
	Description     string `gorm:"size:255;not null;default:'';comment:知识库描述"`
	Enabled         bool   `gorm:"not null;default:true;index:idx_knowledge_bases_enabled;comment:是否可用"`
	SortOrder       int    `gorm:"not null;default:0;index:idx_knowledge_bases_sort_order;comment:排序值"`
	Revision        uint64 `gorm:"not null;default:1;comment:内容修订版本"`
	CreatedByUserID uint   `gorm:"not null;default:0;index:idx_knowledge_bases_created_by;comment:创建人ID"`
	UpdatedByUserID uint   `gorm:"not null;default:0;index:idx_knowledge_bases_updated_by;comment:最后更新人ID"`
}

// TableName 指定表名。
func (KnowledgeBase) TableName() string {
	return "knowledge_bases"
}

// KnowledgeBaseFile 记录知识库与文件对象的多对多关联。
type KnowledgeBaseFile struct {
	KnowledgeBaseID uint      `gorm:"primaryKey;index:idx_knowledge_base_files_base_order,priority:1;comment:知识库ID"`
	FileObjectID    uint      `gorm:"primaryKey;index:idx_knowledge_base_files_file;comment:文件对象ID"`
	SortOrder       int       `gorm:"not null;default:0;index:idx_knowledge_base_files_base_order,priority:2;comment:知识库内排序"`
	AddedByUserID   uint      `gorm:"not null;default:0;comment:添加人ID"`
	CreatedAt       time.Time `gorm:"comment:添加时间"`
}

// TableName 指定表名。
func (KnowledgeBaseFile) TableName() string {
	return "knowledge_base_files"
}
