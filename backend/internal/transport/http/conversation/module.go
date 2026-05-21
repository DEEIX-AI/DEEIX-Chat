package conversation

// Module 聚合会话 HTTP 处理器。
type Module struct {
	Handler *Handler
}

// NewModule 创建会话 HTTP 模块。
func NewModule(handler *Handler) *Module {
	return &Module{Handler: handler}
}
