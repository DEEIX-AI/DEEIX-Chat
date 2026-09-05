package knowledgebase

import (
	"errors"
	"fmt"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
)

func TestErrorsExposeStableContracts(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		code    string
		message string
		text    string
	}{
		{name: "not found", err: ErrKnowledgeBaseNotFound, code: "knowledge_base.not_found", message: "knowledge base not found", text: "knowledge base not found"},
		{name: "invalid", err: ErrInvalidKnowledgeBase, code: "knowledge_base.invalid", message: "invalid knowledge base request", text: "invalid knowledge base"},
		{name: "file not found", err: ErrKnowledgeBaseFileNotFound, code: "knowledge_base.not_found", message: "knowledge base not found", text: "knowledge base file not found"},
		{name: "content unavailable", err: ErrKnowledgeBaseFileContentUnavailable, code: "knowledge_base.internal", message: "knowledge base operation failed", text: "knowledge base file content unavailable"},
		{name: "cleanup unavailable", err: ErrKnowledgeBaseFileCleanupUnavailable, code: "knowledge_base.file_cleanup_unavailable", message: "platform file cleanup unavailable", text: "knowledge base file cleanup unavailable"},
		{name: "platform file in use", err: ErrPlatformFileInUse, code: "knowledge_base.platform_file_in_use", message: "platform file is in use", text: "platform file is in use"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := fmt.Errorf("operation failed: %w", tc.err)
			if !errors.Is(wrapped, tc.err) {
				t.Fatal("wrapping must preserve sentinel identity")
			}
			coded, ok := apperr.Find(wrapped)
			if !ok {
				t.Fatal("wrapped error must expose an application contract")
			}
			if coded.Code() != tc.code || coded.Message() != tc.message {
				t.Fatalf("contract = (%q, %q), want (%q, %q)", coded.Code(), coded.Message(), tc.code, tc.message)
			}
			if tc.err.Error() != tc.text {
				t.Fatalf("internal text = %q, want %q", tc.err.Error(), tc.text)
			}
		})
	}
}
