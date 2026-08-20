package knowledgebase

// Module 封装知识库 HTTP 模块。
type Module struct {
	Handler *Handler
}

// NewModule 创建知识库 HTTP 模块。
func NewModule(handler *Handler) *Module {
	return &Module{Handler: handler}
}
