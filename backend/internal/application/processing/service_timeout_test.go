package processing

import (
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/extraction"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

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
