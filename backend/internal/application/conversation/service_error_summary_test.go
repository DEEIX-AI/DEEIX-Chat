package conversation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	apprag "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/rag"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

func TestMessageErrorSummaryUsesPublicApplicationMessage(t *testing.T) {
	err := fmt.Errorf("resolve model route: %w", ErrModelAccessDenied)

	if got := messageErrorSummary(err); got != ErrModelAccessDenied.Message() {
		t.Fatalf("messageErrorSummary() = %q, want %q", got, ErrModelAccessDenied.Message())
	}
	if strings.Contains(messageErrorSummary(err), ErrModelAccessDenied.Error()) {
		t.Fatalf("messageErrorSummary() leaked internal error text: %q", messageErrorSummary(err))
	}
}

func TestMessageErrorSummaryRedactsUnknownPlainError(t *testing.T) {
	err := errors.New("dial postgres with password=secret")

	if got := messageErrorSummary(err); got != "internal server error" {
		t.Fatalf("messageErrorSummary() = %q, want generic internal summary", got)
	}
}

func TestMessageErrorSummaryMapsKnownPlainSentinels(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "upstream", err: ErrUpstreamRequestFailed, want: "upstream service unavailable"},
		{name: "quota", err: ErrStorageQuotaExceeded, want: "quota exceeded"},
		{name: "tool answer", err: ErrToolRunFinalAnswerMissing, want: "tool run ended without a final answer"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := messageErrorSummary(fmt.Errorf("operation failed: %w", test.err)); got != test.want {
				t.Fatalf("messageErrorSummary() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRAGFallbackErrorMessageIsStableAndSafe(t *testing.T) {
	secret := errors.New("embedding provider response contains api-key=secret")
	tests := []struct {
		name   string
		status apprag.RetrieveStatus
		err    error
		want   string
	}{
		{name: "canceled", status: apprag.RetrieveStatusError, err: fmt.Errorf("retrieve: %w", context.Canceled), want: ragFallbackErrorCanceled},
		{name: "timeout status", status: apprag.RetrieveStatusTimeout, err: secret, want: ragFallbackErrorTimeout},
		{name: "deadline", status: apprag.RetrieveStatusError, err: fmt.Errorf("retrieve: %w", context.DeadlineExceeded), want: ragFallbackErrorTimeout},
		{name: "unavailable", status: apprag.RetrieveStatusUnavailable, err: secret, want: ragFallbackErrorUnavailable},
		{name: "failure", status: apprag.RetrieveStatusError, err: secret, want: ragFallbackErrorFailed},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ragFallbackErrorMessage(test.status, test.err)
			if got != test.want {
				t.Fatalf("ragFallbackErrorMessage() = %q, want %q", got, test.want)
			}
			if strings.Contains(got, secret.Error()) {
				t.Fatalf("ragFallbackErrorMessage() leaked retrieval error: %q", got)
			}
		})
	}
}

func TestBuildRAGFallbackProcessTracePayloadRedactsRetrievalError(t *testing.T) {
	secret := errors.New("redis: connection refused with token=secret")
	payload := buildRAGFallbackProcessTracePayload(
		"query",
		[]model.FileObject{{FileID: "file-1", FileName: "notes.md"}},
		apprag.RetrieveResult{Status: apprag.RetrieveStatusError},
		"rag_error",
		false,
		secret,
	)

	if payload.Error != ragFallbackErrorFailed {
		t.Fatalf("payload error = %#v, want %q", payload.Error, ragFallbackErrorFailed)
	}
	if strings.Contains(fmt.Sprint(payload), secret.Error()) {
		t.Fatalf("payload leaked retrieval error: %#v", payload)
	}
}
