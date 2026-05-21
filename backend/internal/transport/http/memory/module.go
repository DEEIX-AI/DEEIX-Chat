package memory

// Module 聚合记忆 HTTP 处理器。
type Module struct {
	Handler *Handler
}

// NewModule 创建记忆 HTTP 模块。
func NewModule(handler *Handler) *Module {
	return &Module{Handler: handler}
}
