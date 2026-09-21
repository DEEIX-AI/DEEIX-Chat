package uicomponent

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

var (
	// ErrComponentNotFound 表示组件不存在或当前用户无权访问。
	ErrComponentNotFound = apperr.New("ui_component.not_found", "ui component not found")
	// ErrInvalidComponent 表示组件参数不合法。
	ErrInvalidComponent = apperr.New("request.invalid_ui_component", "invalid ui component")
	// ErrComponentConflict 表示组件名在当前作用域内已存在。
	ErrComponentConflict = apperr.New("ui_component.already_exists", "ui component name already exists")
	// ErrBuiltinProtected 表示内置组件不允许删除或改名。
	ErrBuiltinProtected = apperr.New("ui_component.builtin_protected", "builtin ui component is protected")
)
