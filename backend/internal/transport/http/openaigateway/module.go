package openaigateway

// Module 聚合 OpenAI 兼容网关。
type Module struct {
	Handler *Handler
}

// NewModule 创建模块。
func NewModule(handler *Handler) *Module {
	return &Module{Handler: handler}
}
