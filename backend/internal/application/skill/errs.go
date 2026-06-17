package skill

import "errors"

var (
	// ErrSkillNotFound 表示技能不存在或当前用户无权访问。
	ErrSkillNotFound = errors.New("skill not found")
	// ErrInvalidSkill 表示技能参数不合法。
	ErrInvalidSkill = errors.New("invalid skill")
	// ErrSkillConflict 表示触发词在当前作用域内已存在。
	ErrSkillConflict = errors.New("skill trigger already exists")
)
