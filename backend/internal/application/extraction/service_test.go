package extraction

import (
	"context"
	"errors"
	"testing"

	appstorage "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/objectstorage"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	extractport "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/extract"
)

func TestStoredExtractionRequiresObjectStoreProvider(t *testing.T) {
	service := NewServiceWithRuntime(config.NewRuntime(config.Config{}), EngineFactories{})
	if _, err := service.ExtractStoredFile(t.Context(), ExtractInput{}); !errors.Is(err, appstorage.ErrProviderNotConfigured) {
		t.Fatalf("ExtractStoredFile() error = %v, want ErrProviderNotConfigured", err)
	}
}

type documentExtractorStub struct{}

func (documentExtractorStub) ExtractText(context.Context, extractport.DocumentRequest) (string, error) {
	return "", nil
}

func TestEngineFactoriesAreScopedToService(t *testing.T) {
	runtime := config.NewRuntime(config.Config{ExtractEngine: EngineTika})
	configured := NewServiceWithRuntime(runtime, EngineFactories{
		NewTika: func(config.Config) DocumentExtractor { return documentExtractorStub{} },
	})
	unconfigured := NewServiceWithRuntime(runtime, EngineFactories{})

	if configured.resolvePrimaryEngine() == nil {
		t.Fatal("configured service did not resolve its engine")
	}
	if unconfigured.resolvePrimaryEngine() != nil {
		t.Fatal("engine factory leaked across service instances")
	}
}
