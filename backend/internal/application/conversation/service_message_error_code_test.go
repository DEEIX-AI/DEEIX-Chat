package conversation

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	appchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func TestClassifyRunErrorCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "conversation not found", err: ErrConversationNotFound, want: "conversation.not_found"},
		{name: "invalid file reference", err: ErrInvalidFileReference, want: "file.invalid_reference"},
		{name: "file not found", err: ErrFileNotFound, want: "file.not_found"},
		{name: "storage quota", err: ErrStorageQuotaExceeded, want: MessageErrorCodeQuotaExceeded},
		{name: "file too large", err: ErrFileTooLarge, want: "file.too_large"},
		{name: "invalid reference", err: ErrInvalidKnowledgeBaseReference, want: MessageErrorCodeKnowledgeBaseInvalidReference},
		{name: "unavailable", err: ErrKnowledgeBaseUnavailable, want: MessageErrorCodeKnowledgeBaseUnavailable},
		{name: "not ready", err: ErrKnowledgeBaseNotReady, want: MessageErrorCodeKnowledgeBaseNotReady},
		{name: "wrapped unavailable", err: fmt.Errorf("retrieve: %w", ErrKnowledgeBaseUnavailable), want: MessageErrorCodeKnowledgeBaseUnavailable},
		{name: "route not configured", err: ErrModelRouteNotConfigured, want: "llm.model_route_not_configured"},
		{name: "empty response", err: ErrUpstreamEmptyResponse, want: MessageErrorCodeUpstreamEmptyResponse},
		{name: "wrapped empty response", err: fmt.Errorf("generation: %w", ErrUpstreamEmptyResponse), want: MessageErrorCodeUpstreamEmptyResponse},
		{name: "joined empty response", err: errors.Join(ErrUpstreamRequestFailed, ErrUpstreamEmptyResponse), want: MessageErrorCodeUpstreamEmptyResponse},
		{name: "tool final answer missing", err: ErrToolRunFinalAnswerMissing, want: MessageErrorCodeToolRunFinalAnswerMissing},
		{name: "generation canceled", err: ErrMessageGenerationCanceled, want: "conversation_run.canceled"},
		{name: "generation interrupted", err: ErrMessageGenerationInterrupted, want: "conversation_run.stream_interrupted"},
		{name: "image prompt required", err: ErrMediaImagePromptRequired, want: "media.image_prompt_required"},
		{name: "image generation rejects inputs", err: ErrMediaImageGenerationRejectsInputs, want: "media.image_generation_rejects_inputs"},
		{name: "image edit input required", err: ErrMediaImageEditInputRequired, want: "media.image_edit_input_required"},
		{name: "image edit too many inputs", err: ErrMediaImageEditTooManyInputs, want: "media.image_edit_too_many_inputs"},
		{name: "image edit input invalid", err: ErrMediaImageEditInputInvalid, want: "media.image_edit_input_invalid"},
		{name: "video prompt required", err: ErrMediaVideoPromptRequired, want: "media.video_prompt_required"},
		{name: "video input invalid", err: ErrMediaVideoInputInvalid, want: "media.video_input_invalid"},
		{name: "video too many inputs", err: ErrMediaVideoTooManyInputs, want: "media.video_too_many_inputs"},
		{name: "media route mismatch", err: ErrMediaRouteProtocolMismatch, want: "media.route_protocol_mismatch"},
		{name: "upstream rate limited", err: wrapUpstreamRequestError(&llm.UpstreamError{StatusCode: http.StatusTooManyRequests}), want: MessageErrorCodeUpstreamRateLimited},
		{name: "upstream unavailable", err: ErrUpstreamRequestFailed, want: MessageErrorCodeUpstreamUnavailable},
		{name: "wrapped routes unavailable", err: wrapUpstreamRequestError(appchannel.ErrAllRoutesUnavailable), want: MessageErrorCodeUpstreamUnavailable},
		{name: "internal", err: errors.New("unexpected"), want: messageErrorCodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := classifyRunErrorCode(tt.err); got != tt.want {
				t.Fatalf("classifyRunErrorCode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMessageErrorSummaryUsesUpstreamBoundaryForWrappedRouteFailure(t *testing.T) {
	t.Parallel()

	err := wrapUpstreamRequestError(appchannel.ErrAllRoutesUnavailable)
	if got := messageErrorSummary(err); got != "upstream service unavailable" {
		t.Fatalf("messageErrorSummary() = %q, want %q", got, "upstream service unavailable")
	}
}

func TestMessageErrorCodePreservesEmptyResponseThroughUpstreamWrapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{name: "wrapped", err: fmt.Errorf("generation: %w", ErrUpstreamEmptyResponse)},
		{name: "joined", err: errors.Join(ErrUpstreamRequestFailed, ErrUpstreamEmptyResponse)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := MessageErrorCode(tt.err); got != MessageErrorCodeUpstreamEmptyResponse {
				t.Fatalf("MessageErrorCode() = %q, want %q", got, MessageErrorCodeUpstreamEmptyResponse)
			}
		})
	}
}

func TestMapRouteResolutionErrorPreservesFailureSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  error
		wantIs []error
	}{
		{name: "access denied", input: appchannel.ErrModelAccessDenied, wantIs: []error{ErrModelAccessDenied}},
		{name: "route missing", input: appchannel.ErrRouteNotFound, wantIs: []error{ErrModelRouteNotConfigured}},
		{
			name:   "routes unavailable",
			input:  appchannel.ErrAllRoutesUnavailable,
			wantIs: []error{ErrUpstreamRequestFailed, appchannel.ErrAllRoutesUnavailable},
		},
		{
			name:   "routes rate limited",
			input:  &appchannel.RoutesRateLimitedError{},
			wantIs: []error{ErrUpstreamRequestFailed, appchannel.ErrAllRoutesRateLimited},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mapRouteResolutionError(tt.input)
			for _, target := range tt.wantIs {
				if !errors.Is(got, target) {
					t.Fatalf("mapRouteResolutionError() = %v, want errors.Is(_, %v)", got, target)
				}
			}
		})
	}
}
