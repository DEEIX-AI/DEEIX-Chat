package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

type runtimeTestProber struct {
	message string
}

func (p runtimeTestProber) ProbeTika(context.Context, string, string) (bool, string) {
	return false, p.message
}

func (runtimeTestProber) ResolveManagedTikaBaseURL(context.Context) string {
	return "http://deeix-chat-tika:9998"
}

func (p runtimeTestProber) ProbeDocling(context.Context, string, string) (bool, string) {
	return false, p.message
}

func (p runtimeTestProber) ProbeMinerU(context.Context, string, string) (bool, string) {
	return false, p.message
}

func (p runtimeTestProber) ProbeOCR(context.Context, string, string) (bool, string) {
	return false, p.message
}

func (p runtimeTestProber) ProbeRapidOCR(context.Context, string, string) (bool, string) {
	return false, p.message
}

func (runtimeTestProber) ResolveManagedRapidOCRBaseURL(context.Context) string {
	return "http://deeix-chat-rapidocr:8002/ocr"
}

type runtimeTestDockerRunner struct {
	err error
}

func (r runtimeTestDockerRunner) Available() bool {
	return true
}

func (r runtimeTestDockerRunner) RunWithTimeout(context.Context, time.Duration, ...string) (string, error) {
	return "", r.err
}

func TestGetTikaStatusDoesNotExposeDockerError(t *testing.T) {
	service := NewService(
		config.NewRuntime(config.Config{ExtractTikaSource: "managed"}),
		runtimeTestProber{},
	)
	service.SetDockerRunner(runtimeTestDockerRunner{err: errors.New("docker: permission denied; secret=/var/run/docker.sock")})

	view := service.GetTikaStatus(context.Background())
	if view.Status != "failed" {
		t.Fatalf("status = %q, want failed", view.Status)
	}
	if view.Message != tikaContainerStatusFailedMessage {
		t.Fatalf("message = %q, want %q", view.Message, tikaContainerStatusFailedMessage)
	}
}

func TestGetRapidOCRStatusDoesNotExposeDockerError(t *testing.T) {
	service := NewService(
		config.NewRuntime(config.Config{ExtractRapidOCRSource: "managed"}),
		runtimeTestProber{},
	)
	service.SetDockerRunner(runtimeTestDockerRunner{err: errors.New("docker: permission denied; secret=/var/run/docker.sock")})

	view := service.GetRapidOCRStatus(context.Background())
	if view.Status != "failed" {
		t.Fatalf("status = %q, want failed", view.Status)
	}
	if view.Message != rapidOCRContainerStatusFailedMessage {
		t.Fatalf("message = %q, want %q", view.Message, rapidOCRContainerStatusFailedMessage)
	}
}

func TestExternalRuntimeStatusDoesNotExposeProbeError(t *testing.T) {
	const probeError = "dial tcp 10.0.0.4:8005: connection refused; token=secret"
	service := NewService(
		config.NewRuntime(config.Config{
			ExtractTikaSource:          "external",
			ExtractTikaBaseURL:         "https://tika.example.test",
			ExtractDoclingBaseURL:      "https://docling.example.test",
			ExtractTesseractOCRBaseURL: "https://tesseract.example.test",
			ExtractRapidOCRSource:      "external",
			ExtractRapidOCRBaseURL:     "https://rapidocr.example.test/ocr",
			ExtractMinerUBaseURL:       "https://mineru.example.test",
		}),
		runtimeTestProber{message: probeError},
	)

	tests := []struct {
		name string
		get  func() ServiceRuntimeView
		want string
	}{
		{name: "tika", get: func() ServiceRuntimeView { return service.GetTikaStatus(context.Background()) }, want: tikaProbeFailedMessage},
		{name: "docling", get: func() ServiceRuntimeView { return service.GetDoclingStatus(context.Background()) }, want: doclingProbeFailedMessage},
		{name: "tesseract", get: func() ServiceRuntimeView { return service.GetTesseractStatus(context.Background()) }, want: tesseractProbeFailedMessage},
		{name: "rapidocr", get: func() ServiceRuntimeView { return service.GetRapidOCRStatus(context.Background()) }, want: rapidOCRProbeFailedMessage},
		{name: "mineru", get: func() ServiceRuntimeView { return service.GetMinerUStatus(context.Background()) }, want: minerUProbeFailedMessage},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			view := test.get()
			if view.Status != "unhealthy" {
				t.Fatalf("status = %q, want unhealthy", view.Status)
			}
			if view.Message != test.want {
				t.Fatalf("message = %q, want %q", view.Message, test.want)
			}
			if strings.Contains(view.Message, "secret") {
				t.Fatal("probe details must not be exposed")
			}
		})
	}
}
