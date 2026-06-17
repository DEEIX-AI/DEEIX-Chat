package skill

// Module 聚合技能 HTTP 处理器。
type Module struct {
	Handler *Handler
}

// NewModule 创建技能 HTTP 模块。
func NewModule(handler *Handler) *Module {
	return &Module{Handler: handler}
}
