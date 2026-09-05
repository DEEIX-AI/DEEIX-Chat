package usersettings

import (
	"fmt"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
)

var (
	// ErrUnknownSetting 表示请求包含不支持的用户设置键。
	ErrUnknownSetting = apperr.New("user_settings.unknown_key", "unknown setting key")
	// ErrInvalidSettingValue 表示用户设置值不符合对应设置的约束。
	ErrInvalidSettingValue = apperr.New("user_settings.invalid_value", "invalid user setting value")
)

// settingValidationError 将具体校验原因保留在内部错误链中，对外只暴露稳定的应用错误契约。
func settingValidationError(code *apperr.Error, detail string) error {
	return fmt.Errorf("%s: %w", detail, code)
}
