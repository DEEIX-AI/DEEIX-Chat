// Package system 暴露与部署形态相关、不属于任何业务模块的公开只读端点。
package system

// Module 聚合 system HTTP 处理器。
type Module struct {
	Handler *Handler
}

// NewModule 创建 system HTTP 模块。
func NewModule(handler *Handler) *Module {
	return &Module{Handler: handler}
}
