package admin

// Module 聚合后台管理 HTTP 处理器。
type Module struct {
	Handler *Handler
}

// NewModule 创建后台管理 HTTP 模块。
func NewModule(handler *Handler) *Module {
	return &Module{Handler: handler}
}
