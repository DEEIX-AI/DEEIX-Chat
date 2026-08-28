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
		{name: "invalid reference", err: ErrInvalidKnowledgeBaseReference, want: MessageErrorCodeKnowledgeBaseInvalidReference},
		{name: "unavailable", err: ErrKnowledgeBaseUnavailable, want: MessageErrorCodeKnowledgeBaseUnavailable},
		{name: "not ready", err: ErrKnowledgeBaseNotReady, want: MessageErrorCodeKnowledgeBaseNotReady},
		{name: "wrapped unavailable", err: fmt.Errorf("retrieve: %w", ErrKnowledgeBaseUnavailable), want: MessageErrorCodeKnowledgeBaseUnavailable},
		{name: "upstream rate limited", err: wrapUpstreamRequestError(&llm.UpstreamError{StatusCode: http.StatusTooManyRequests}), want: MessageErrorCodeUpstreamRateLimited},
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
