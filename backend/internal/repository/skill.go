package repository

import (
	"context"

	domainskill "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/skill"
)

// SkillRepository 定义技能持久化能力。
type SkillRepository interface {
	ListSkills(ctx context.Context, filter SkillListFilter, offset int, limit int) ([]domainskill.Skill, int64, error)
	GetSkill(ctx context.Context, id uint) (*domainskill.Skill, error)
	CreateSkill(ctx context.Context, item *domainskill.Skill) (*domainskill.Skill, error)
	PatchSkill(ctx context.Context, id uint, patch SkillPatch) (*domainskill.Skill, error)
	DeleteSkill(ctx context.Context, id uint) error
}

// SkillListFilter 描述技能列表筛选条件。
type SkillListFilter struct {
	Query          string
	SearchMarkdown bool
	Scope          string
	OwnerUserID    *uint
	Enabled        *bool
	VisibleUserID  *uint
}

// SkillPatch 描述可更新的技能字段。
type SkillPatch struct {
	Title              *string
	Trigger            *string
	Description        *string
	Markdown           *string
	Enabled            *bool
	SortOrder          *int
	UpdatedByUserIDSet bool
	UpdatedByUserID    uint
}
