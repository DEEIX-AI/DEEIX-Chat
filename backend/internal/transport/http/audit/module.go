package audit

// Module 聚合审计 HTTP 处理器。
type Module struct {
	Handler *Handler
}

// NewModule 创建审计 HTTP 模块。
func NewModule(handler *Handler) *Module {
	return &Module{Handler: handler}
}
