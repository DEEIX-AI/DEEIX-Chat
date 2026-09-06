package userapikey

// Module 聚合用户 API Key HTTP 处理器。
type Module struct {
	Handler *Handler
}

// NewModule 创建模块。
func NewModule(handler *Handler) *Module {
	return &Module{Handler: handler}
}
