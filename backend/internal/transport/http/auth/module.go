package auth

// Module 聚合认证 HTTP 处理器。
type Module struct {
	Handler *Handler
}

// NewModule 创建认证 HTTP 模块。
func NewModule(handler *Handler) *Module {
	return &Module{Handler: handler}
}
