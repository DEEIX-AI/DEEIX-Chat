package skill

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

var (
	// ErrSkillNotFound 表示技能不存在或当前用户无权访问。
	ErrSkillNotFound = apperr.New("skill.not_found", "skill not found")
	// ErrInvalidSkill 表示技能参数不合法。
	ErrInvalidSkill = apperr.New("request.invalid_skill", "invalid skill")
	// ErrSkillConflict 表示触发词在当前作用域内已存在。
	ErrSkillConflict = apperr.New("skill_trigger.already_exists", "skill trigger already exists")
)
