package extraction

import (
	"context"
	"errors"
	"os"
	"testing"

	appstorage "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/objectstorage"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	extractionport "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/extraction"
)

func TestStoredExtractionRequiresObjectStoreProvider(t *testing.T) {
	service := NewServiceWithRuntime(config.NewRuntime(config.Config{}), EngineFactories{})
	if _, err := service.ExtractStoredFile(t.Context(), ExtractInput{}); !errors.Is(err, appstorage.ErrProviderNotConfigured) {
		t.Fatalf("ExtractStoredFile() error = %v, want ErrProviderNotConfigured", err)
	}
}

type documentExtractorStub struct{}

func (documentExtractorStub) ExtractText(context.Context, extractionport.DocumentRequest) (string, error) {
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

// panickingBuiltinParser 模拟第三方解析库在畸形文件上 panic。
type panickingBuiltinParser struct{}

func (panickingBuiltinParser) ExtractText([]byte) string { return "" }
func (panickingBuiltinParser) ExtractWordText(context.Context, string, []byte, string, string) extractionport.WordTextResult {
	return extractionport.WordTextResult{}
}
func (panickingBuiltinParser) ExtractExcelText([]byte, string, string) string {
	panic("index out of range [-1]")
}
func (panickingBuiltinParser) ExtractPDFText(string, int) (string, error) { return "", nil }
func (panickingBuiltinParser) ExtractPDFPages(string, int) (extractionport.PDFTextResult, error) {
	return extractionport.PDFTextResult{}, nil
}
func (panickingBuiltinParser) DetectPDFPageCount(string) int { return 0 }

func TestExtractionRecoversParserPanicAsExtractFailure(t *testing.T) {
	file, err := os.CreateTemp("", "malformed-*.xlsx")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(file.Name()) })
	if _, err := file.WriteString("PK\x03\x04 not really a workbook"); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	_ = file.Close()

	service := NewServiceWithRuntime(config.NewRuntime(config.Config{}), EngineFactories{Builtin: panickingBuiltinParser{}})
	result, err := service.ExtractTemporaryFile(t.Context(), ExtractInput{
		File: domainconversation.FileObject{
			FileName:     "malformed.xlsx",
			FileCategory: "excel",
			StoragePath:  file.Name(),
		},
	})

	if !errors.Is(err, ErrParserPanicked) {
		t.Fatalf("expected the parser panic to surface as ErrParserPanicked, got %v", err)
	}
	if ErrorCode(err) != "extract_failed" {
		t.Fatalf("expected extract_failed code, got %q", ErrorCode(err))
	}
	if result.Text != "" {
		t.Fatalf("a panicked extraction must not return partial text, got %q", result.Text)
	}
}
