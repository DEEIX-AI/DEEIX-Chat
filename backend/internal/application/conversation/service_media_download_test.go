package conversation

import (
	"context"
	"errors"
	"fmt"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type generatedMediaDownloaderStub struct {
	downloadImage func(context.Context, string, string, int64) ([]byte, string, error)
	downloadVideo func(context.Context, string, string, string, int64) ([]byte, string, error)
}

func (s generatedMediaDownloaderStub) DownloadImage(ctx context.Context, sourceURL string, trustedProviderEndpoint string, maxBytes int64) ([]byte, string, error) {
	return s.downloadImage(ctx, sourceURL, trustedProviderEndpoint, maxBytes)
}

func (s generatedMediaDownloaderStub) DownloadVideo(ctx context.Context, sourceURL string, trustedProviderEndpoint string, apiKey string, maxBytes int64) ([]byte, string, error) {
	return s.downloadVideo(ctx, sourceURL, trustedProviderEndpoint, apiKey, maxBytes)
}

type generatedMediaTooLargeError struct{}

type generatedMediaStateRepository struct {
	repository.ConversationRepository
	status       string
	errorCode    string
	contextError error
}

func (r *generatedMediaStateRepository) UpdateMessageState(ctx context.Context, _ uint, status string, errorCode string, _ string) error {
	r.status = status
	r.errorCode = errorCode
	r.contextError = ctx.Err()
	return nil
}

func (generatedMediaTooLargeError) Error() string {
	return "too large"
}

func (generatedMediaTooLargeError) MediaArtifactResponseTooLarge() {}

func TestReadGeneratedImageDelegatesURLDownloadAndValidatesBytes(t *testing.T) {
	pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	service := &Service{
		cfg: config.NewRuntime(config.Config{MaxUploadFileBytes: 1024}),
		mediaDownloader: generatedMediaDownloaderStub{
			downloadImage: func(_ context.Context, sourceURL string, trustedProviderEndpoint string, maxBytes int64) ([]byte, string, error) {
				if sourceURL != "https://cdn.example.test/image" || trustedProviderEndpoint != "http://model.internal:8080/v1" || maxBytes != 1024 {
					t.Fatalf("unexpected download input: URL=%q trustedEndpoint=%q maxBytes=%d", sourceURL, trustedProviderEndpoint, maxBytes)
				}
				return pngHeader, "image/png", nil
			},
		},
	}

	data, mimeType, err := service.readGeneratedImage(t.Context(), llm.GeneratedImage{
		URL:      "https://cdn.example.test/image",
		MIMEType: "application/octet-stream",
	}, "http://model.internal:8080/v1")
	if err != nil {
		t.Fatalf("read generated image: %v", err)
	}
	if string(data) != string(pngHeader) || mimeType != "image/png" {
		t.Fatalf("unexpected generated image: data=%q MIME=%q", data, mimeType)
	}
}

func TestReadGeneratedVideoMapsAdapterSizeLimit(t *testing.T) {
	service := &Service{
		cfg: config.NewRuntime(config.Config{MaxUploadFileBytes: 1024}),
		mediaDownloader: generatedMediaDownloaderStub{
			downloadVideo: func(context.Context, string, string, string, int64) ([]byte, string, error) {
				return nil, "", generatedMediaTooLargeError{}
			},
		},
	}

	_, _, err := service.readGeneratedVideo(t.Context(), llm.GeneratedVideo{
		URL: "https://cdn.example.test/video",
	}, "", "")
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("expected application file size error, got %v", err)
	}
}

func TestReadGeneratedImageHidesAdapterSecurityDetails(t *testing.T) {
	cause := fmt.Errorf("%w: unsafe host", security.ErrUnsafeOutboundURL)
	service := &Service{
		cfg: config.NewRuntime(config.Config{MaxUploadFileBytes: 1024}),
		mediaDownloader: generatedMediaDownloaderStub{
			downloadImage: func(context.Context, string, string, int64) ([]byte, string, error) {
				return nil, "", cause
			},
		},
	}

	_, _, err := service.readGeneratedImage(t.Context(), llm.GeneratedImage{
		URL: "https://cdn.example.test/image",
	}, "")
	if !errors.Is(err, ErrGeneratedMediaArtifactUnavailable) {
		t.Fatalf("expected generated media artifact error, got %v", err)
	}
	if summary := messageErrorSummary(err); summary != ErrGeneratedMediaArtifactUnavailable.Error() {
		t.Fatalf("security detail leaked into user-facing summary: %q", summary)
	}
	if code := classifyRunErrorCode(err); code != MessageErrorCodeMediaArtifactUnavailable {
		t.Fatalf("unexpected user-facing error code: %q", code)
	}
	if code := MessageErrorCode(err); code != MessageErrorCodeMediaArtifactUnavailable {
		t.Fatalf("unexpected boundary error code: %q", code)
	}
	details := generatedMediaArtifactFailureDetails(err)
	if details.mediaType != "image" || details.stage != "download" || !errors.Is(details.cause, cause) {
		t.Fatalf("diagnostic cause was not preserved: %#v", details)
	}
}

func TestLogGeneratedMediaArtifactFailureIncludesOperationalContext(t *testing.T) {
	core, observed := observer.New(zap.WarnLevel)
	service := &Service{logger: zap.New(core)}
	cause := fmt.Errorf("download generated image failed: %w", security.ErrUnsafeOutboundURL)
	err := newGeneratedMediaArtifactError("image", "download", cause)
	run := &model.Run{
		RunID:             "run-123",
		RequestID:         "request-456",
		UserID:            7,
		ConversationID:    8,
		TaskType:          "image_generation",
		Endpoint:          "images/generations",
		UpstreamID:        9,
		UpstreamModelID:   10,
		UpstreamName:      "primary-images",
		ProviderProtocol:  "openai_images",
		PlatformModelName: "image-model",
		UpstreamModelName: "provider-image-model",
		RoutedBindingCode: "binding-11",
		ModelVendor:       "openai",
	}

	service.logGeneratedMediaArtifactFailure(t.Context(), run, 2, 3, err)

	entries := observed.FilterMessage("generated_media_artifact_failed").All()
	if len(entries) != 1 {
		t.Fatalf("expected one diagnostic log entry, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	want := map[string]any{
		"request_id":          "request-456",
		"run_id":              "run-123",
		"error_code":          MessageErrorCodeMediaArtifactUnavailable,
		"user_id":             uint64(7),
		"conversation_id":     uint64(8),
		"task_type":           "image_generation",
		"endpoint":            "images/generations",
		"media_type":          "image",
		"artifact_index":      int64(2),
		"artifact_count":      int64(3),
		"failure_stage":       "download",
		"failure_class":       "outbound_policy",
		"upstream_id":         uint64(9),
		"upstream_model_id":   uint64(10),
		"upstream_name":       "primary-images",
		"provider_protocol":   "openai_images",
		"platform_model_name": "image-model",
		"upstream_model_name": "provider-image-model",
		"routed_binding_code": "binding-11",
		"model_vendor":        "openai",
	}
	for key, expected := range want {
		if actual := fields[key]; actual != expected {
			t.Fatalf("unexpected log field %s: got %#v want %#v", key, actual, expected)
		}
	}
	if fields["error"] != cause.Error() {
		t.Fatalf("diagnostic cause missing from log: %#v", fields["error"])
	}
}

func TestFinalizeGeneratedMediaArtifactFailurePreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	repo := &generatedMediaStateRepository{}
	service := &Service{repo: repo}
	err := service.finalizeGeneratedMediaArtifactFailure(
		ctx,
		&model.Run{RunID: "run-canceled"},
		42,
		1,
		1,
		newGeneratedMediaArtifactError("video", "download", context.Canceled),
	)
	if !errors.Is(err, ErrMessageGenerationCanceled) {
		t.Fatalf("expected canceled media run, got %v", err)
	}
	if repo.contextError != nil || repo.status != "canceled" || repo.errorCode != "conversation_run.canceled" {
		t.Fatalf("unexpected persisted cancellation: status=%q code=%q contextErr=%v", repo.status, repo.errorCode, repo.contextError)
	}
}
