package billing

// Module 聚合计费 HTTP 处理器。
type Module struct {
	Handler *Handler
}

// NewModule 创建计费 HTTP 模块。
func NewModule(handler *Handler) *Module {
	return &Module{Handler: handler}
}
