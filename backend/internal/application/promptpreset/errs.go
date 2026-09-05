package promptpreset

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

var (
	// ErrPromptPresetNotFound 表示预制提示词不存在或当前用户无权访问。
	ErrPromptPresetNotFound = apperr.New("prompt_preset.not_found", "prompt preset not found")
	// ErrInvalidPromptPreset 表示预制提示词参数不合法。
	ErrInvalidPromptPreset = apperr.New("request.invalid_prompt_preset", "invalid prompt preset")
	// ErrPromptPresetConflict 表示触发词在当前作用域内已存在。
	ErrPromptPresetConflict = apperr.New("prompt_preset_trigger.already_exists", "prompt preset trigger already exists")
)
