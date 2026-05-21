package mcp

type Module struct {
	Handler *Handler
}

func NewModule(handler *Handler) *Module {
	return &Module{Handler: handler}
}
