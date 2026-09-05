package processing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/extraction"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

func TestExtractTextForProcessingRequiresExtractionService(t *testing.T) {
	service := NewServiceWithRuntime(Dependencies{
		Config:           config.NewRuntime(config.Config{}),
		ExtractorVersion: "pipeline-v1",
	})
	if service.extractSvc != nil {
		t.Fatal("expected nil extraction dependency to remain explicit")
	}
	if _, err := service.extractTextForProcessing(t.Context(), domainconversation.FileObject{}); !errors.Is(err, errExtractionServiceNotConfigured) {
		t.Fatalf("extractTextForProcessing() error = %v, want errExtractionServiceNotConfigured", err)
	}
}

type processingStateRepositoryStub struct {
	file             domainconversation.FileObject
	claimRecovery    bool
	claimedState     *domainconversation.FileObjectProcessing
	claimedAttemptID string
}

func (r *processingStateRepositoryStub) GetActiveFileObjectByID(context.Context, uint, string) (*domainconversation.FileObject, error) {
	file := r.file
	return &file, nil
}

func (*processingStateRepositoryStub) UpdateFileObjectProcessingState(context.Context, *domainconversation.FileObjectProcessing) error {
	return nil
}

func (r *processingStateRepositoryStub) UpdateClaimedFileObjectProcessingState(
	_ context.Context,
	state *domainconversation.FileObjectProcessing,
	attemptID string,
) (bool, error) {
	r.claimedState = state
	r.claimedAttemptID = attemptID
	return true, nil
}

func (*processingStateRepositoryStub) GetFileObjectProcessingByObjectID(context.Context, uint) (*domainconversation.FileObjectProcessing, error) {
	return nil, nil
}

func (*processingStateRepositoryStub) CloneFileObjectProcessingState(context.Context, uint, uint, uint) error {
	return nil
}

func (r *processingStateRepositoryStub) TryClaimFileObjectProcessing(
	_ context.Context,
	_ uint,
	_ string,
	allowRecovery bool,
	_ string,
	_ string,
) (bool, error) {
	r.claimRecovery = allowRecovery
	return true, nil
}

func (*processingStateRepositoryStub) ResetFileObjectProcessingForRetry(context.Context, uint, string, string) (bool, error) {
	return true, nil
}

func (*processingStateRepositoryStub) GetActiveFileProcessingStatusesByIDs(context.Context, uint, []string) ([]domainconversation.FileObject, error) {
	return nil, nil
}

func TestResolveProcessingExtractTimeoutUsesMinerUConfig(t *testing.T) {
	cfg := config.Config{
		ExtractEngine:               extraction.EngineMinerU,
		ExtractMinerUTimeoutSeconds: 180,
	}

	got := resolveProcessingExtractTimeout(cfg, "pdf")
	if got != 180*time.Second {
		t.Fatalf("expected MinerU timeout to be 180s, got %s", got)
	}
}

func TestResolveProcessingExtractTimeoutFallsBackToDefault(t *testing.T) {
	cfg := config.Config{
		ExtractEngine:               extraction.EngineMinerU,
		ExtractMinerUTimeoutSeconds: 0,
	}

	got := resolveProcessingExtractTimeout(cfg, "word")
	if got != defaultExtractTimeout {
		t.Fatalf("expected default timeout %s, got %s", defaultExtractTimeout, got)
	}
}

func TestResolveProcessingExtractTimeoutUsesImageOCRConfig(t *testing.T) {
	cfg := config.Config{
		ExtractEngine:                     extraction.EngineBuiltin,
		ExtractImageOCREnabled:            true,
		ExtractOCREngine:                  extraction.OCREngineRapidOCR,
		ExtractRapidOCRTimeoutSeconds:     90,
		ExtractTesseractOCRTimeoutSeconds: 120,
	}

	got := resolveProcessingExtractTimeout(cfg, "image")
	if got != 90*time.Second {
		t.Fatalf("expected image OCR timeout to be 90s, got %s", got)
	}
}

func TestResolveProcessingExtractTimeoutUsesMistralOCRConfig(t *testing.T) {
	cfg := config.Config{
		ExtractEngine:                   extraction.EngineBuiltin,
		ExtractImageOCREnabled:          true,
		ExtractOCREngine:                extraction.OCREngineMistral,
		ExtractMistralOCRTimeoutSeconds: 75,
	}

	got := resolveProcessingExtractTimeout(cfg, "image")
	if got != 75*time.Second {
		t.Fatalf("expected Mistral OCR timeout to be 75s, got %s", got)
	}
}

func TestProcessingSupportsPresentationExtractionAndRAG(t *testing.T) {
	if !supportsExtraction("presentation") {
		t.Fatal("presentation should support extraction")
	}
	if !supportsRAG("presentation") {
		t.Fatal("presentation should support RAG")
	}
}

func TestResolveProcessingExtractTimeoutAddsPDFOCRFallbackWindow(t *testing.T) {
	cfg := config.Config{
		ExtractEngine:                     extraction.EngineTika,
		ExtractTikaTimeoutSeconds:         80,
		ExtractPDFOCRFallbackEnabled:      true,
		ExtractOCREngine:                  extraction.OCREngineTesseract,
		ExtractTesseractOCRTimeoutSeconds: 90,
	}

	got := resolveProcessingExtractTimeout(cfg, "pdf")
	if got != 170*time.Second {
		t.Fatalf("expected PDF extraction plus OCR fallback timeout to be 170s, got %s", got)
	}
}

func TestResolveProcessingExtractTimeoutIgnoresOCRForPDFWhenFallbackDisabled(t *testing.T) {
	cfg := config.Config{
		ExtractEngine:                extraction.EngineDocling,
		ExtractDoclingTimeoutSeconds: 75,
		ExtractOCREngine:             extraction.OCREngineLLM,
		ExtractLLMOCRTimeoutSeconds:  180,
	}

	got := resolveProcessingExtractTimeout(cfg, "pdf")
	if got != 75*time.Second {
		t.Fatalf("expected PDF timeout to use primary engine only, got %s", got)
	}
}

func TestProcessFileFinalizesImageWhenOCRIsDisabled(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		status        string
		allowRecovery bool
	}{
		{name: "queued", status: "queued"},
		{name: "reclaimed", status: "extracting", allowRecovery: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &processingStateRepositoryStub{
				file: domainconversation.FileObject{
					ID:               9,
					FileID:           "file_1",
					UserID:           7,
					DetectedMIME:     "image/png",
					FileCategory:     "image",
					ProcessingStatus: testCase.status,
				},
			}
			service := NewServiceWithRuntime(Dependencies{
				Config:           config.NewRuntime(config.Config{}),
				Repository:       repo,
				ExtractorVersion: "pipeline-v1",
			})

			claimed, err := service.processFile(context.Background(), 7, "file_1", testCase.allowRecovery, "attempt_1")
			if err != nil {
				t.Fatalf("process image: %v", err)
			}
			if !claimed || repo.claimRecovery != testCase.allowRecovery {
				t.Fatalf("claim mismatch: claimed=%v recovery=%v", claimed, repo.claimRecovery)
			}
			if repo.claimedAttemptID != "attempt_1" || repo.claimedState == nil {
				t.Fatalf("claimed state was not persisted: attempt=%q state=%#v", repo.claimedAttemptID, repo.claimedState)
			}
			if repo.claimedState.ProcessingStatus != "ready" || !repo.claimedState.ProcessingReady {
				t.Fatalf("image did not reach ready: %#v", repo.claimedState)
			}
			if repo.claimedState.ExtractStatus != "none" || repo.claimedState.RAGReason != "image_not_applicable" {
				t.Fatalf("unexpected disabled OCR state: %#v", repo.claimedState)
			}
		})
	}
}
