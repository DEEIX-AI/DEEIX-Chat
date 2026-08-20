package conversation

import (
	"errors"
	"fmt"
	"testing"
)

func TestClassifyRunErrorCodeKnowledgeBaseErrors(t *testing.T) {
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
