package user

// Module 聚合用户 HTTP 处理器。
type Module struct {
	Handler *Handler
}

// NewModule 创建用户 HTTP 模块。
func NewModule(handler *Handler) *Module {
	return &Module{Handler: handler}
}
