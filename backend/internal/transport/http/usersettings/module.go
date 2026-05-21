package usersettings

// Module 聚合用户设置 HTTP 处理器。
type Module struct {
	Handler *Handler
}

// NewModule 创建用户设置 HTTP 模块。
func NewModule(handler *Handler) *Module {
	return &Module{Handler: handler}
}
