package llm

import (
	"context"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

type transportAdapter interface {
	Name() string
	Generate(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput) (*portllm.GenerateOutput, error)
	GenerateStream(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput, onEvent func(portllm.GenerateStreamEvent) error) (*portllm.GenerateOutput, error)
	ListModels(ctx context.Context, route portllm.RouteConfig) ([]portllm.ModelItem, error)
}
