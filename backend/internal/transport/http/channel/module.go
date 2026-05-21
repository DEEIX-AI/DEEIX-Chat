package channel

// Module 聚合模型与上游 HTTP 处理器。
type Module struct {
	Handler *Handler
}

// NewModule 创建模型与上游 HTTP 模块。
func NewModule(handler *Handler) *Module {
	return &Module{Handler: handler}
}
