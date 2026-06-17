package skill

import "time"

const (
	// ScopeBuiltin 表示管理员维护的全局内置技能。
	ScopeBuiltin = "builtin"
	// ScopeUser 表示用户维护的个人自定义技能。
	ScopeUser = "user"
)

// Skill 表示可在会话中按需加载的 SKILL.md 能力包。
type Skill struct {
	ID              uint
	Scope           string
	OwnerUserID     uint
	Title           string
	Trigger         string
	Description     string
	Markdown        string
	Enabled         bool
	SortOrder       int
	CreatedByUserID uint
	UpdatedByUserID uint
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
