package embedding

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestErrorSummaryClassifiesNoExtractableTextWithFileID(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "typed sentinel", err: fmt.Errorf("%w file-1", errNoExtractableText)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ErrorSummary(test.err); got != embeddingNoTextMessage {
				t.Fatalf("ErrorSummary() = %q, want %q", got, embeddingNoTextMessage)
			}
		})
	}
}

func TestErrorSummaryDoesNotExposeEmbeddingProviderDetails(t *testing.T) {
	secret := "https://embedding.example/v1?api_key=secret-token"
	for _, err := range []error{
		errors.New("provider returned 502: " + secret),
		fmt.Errorf("call embedding provider: %w", context.DeadlineExceeded),
	} {
		summary := ErrorSummary(err)
		if strings.Contains(summary, secret) || strings.Contains(summary, "secret-token") {
			t.Fatalf("ErrorSummary() leaked provider details: %q", summary)
		}
		if len([]rune(summary)) > embeddingErrorLimit {
			t.Fatalf("ErrorSummary() length = %d, want <= %d", len([]rune(summary)), embeddingErrorLimit)
		}
	}
}

func TestErrorSummaryUsesGenericMessageForUnknownErrors(t *testing.T) {
	if got := ErrorSummary(errors.New(strings.Repeat("x", embeddingErrorLimit+100))); got != embeddingFailureMessage {
		t.Fatalf("ErrorSummary(unknown) = %q, want generic failure", got)
	}
}
