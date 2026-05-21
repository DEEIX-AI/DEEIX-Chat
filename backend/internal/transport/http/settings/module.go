package settings

// Module 聚合 settings HTTP 处理器。
type Module struct {
	Handler *Handler
}

// NewModule 创建 settings HTTP 模块。
func NewModule(handler *Handler) *Module {
	return &Module{Handler: handler}
}
