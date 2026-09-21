package uicomponent

// Module 聚合交互式组件 HTTP 处理器。
type Module struct {
	Handler *Handler
}

// NewModule 创建组件 HTTP 模块。
func NewModule(handler *Handler) *Module {
	return &Module{Handler: handler}
}
