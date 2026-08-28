package llm

import (
	"context"
	"fmt"
)

type transportAdapter interface {
	Name() string
	Generate(ctx context.Context, route RouteConfig, input GenerateInput) (*GenerateOutput, error)
	GenerateStream(ctx context.Context, route RouteConfig, input GenerateInput, onEvent func(GenerateStreamEvent) error) (*GenerateOutput, error)
	ListModels(ctx context.Context, route RouteConfig) ([]ModelItem, error)
}

func validateAdapter(raw string) error {
	if !IsKnownAdapter(raw) {
		return fmt.Errorf("%w: %s", ErrUnsupportedAdapter, raw)
	}
	return nil
}
