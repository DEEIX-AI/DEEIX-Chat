package processing

import (
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

func TestFileProcessingStatusDoesNotInventFailureMessages(t *testing.T) {
	for _, category := range []string{"pdf", "document", "image"} {
		for _, status := range []string{"uploaded", "queued", "extracting", "embedding", "ready"} {
			t.Run(category+"/"+status, func(t *testing.T) {
				file := &model.FileObject{
					FileCategory: category, ProcessingStatus: status,
					ProcessingReady: status == "ready", ExtractStatus: "ready", EmbedStatus: "none",
					ExtractEngine: "mineru", ExtractChars: 638, ExtractPages: 1,
				}
				got := fileProcessingStatusFromFileObject(file)
				if got.ErrorCode != "" || got.ErrorMessage != "" {
					t.Fatalf("file without an error acquired a failure: code=%q message=%q", got.ErrorCode, got.ErrorMessage)
				}
				if file.ProcessingErrorMessage != "" {
					t.Fatal("mapping must not modify the stored file")
				}
			})
		}
	}
}

func TestFileProcessingStatusEmptyAndEmbeddingFailureStaySeparate(t *testing.T) {
	for _, file := range []model.FileObject{
		{FileCategory: "pdf", ProcessingStatus: "ready", ExtractStatus: "empty", EmbedStatus: "empty"},
		{FileCategory: "pdf", ProcessingStatus: "ready", ProcessingReady: true, ExtractStatus: "ready", EmbedStatus: "failed", EmbedError: "embedding unavailable"},
		{FileCategory: "pdf", ProcessingStatus: "ready", ProcessingReady: true, ProcessingErrorCode: " \t", ProcessingErrorMessage: " \n"},
	} {
		got := fileProcessingStatusFromFileObject(&file)
		if got.ErrorMessage != "" {
			t.Fatalf("non-processing failure acquired a processing error: %q", got.ErrorMessage)
		}
		if got.EmbedError != file.EmbedError {
			t.Fatalf("embedding error changed: %q", got.EmbedError)
		}
	}
}

func TestFileProcessingStatusKeepsSafeFailureMessages(t *testing.T) {
	for _, tc := range []struct {
		name, status, code, message, want string
	}{
		{"coded failure", "failed", "extract_timeout", "", "文件提取超时，请稍后重试，或缩小文件后重试。"},
		{"legacy failure", "failed", "", "", "PDF 未提取到可读文本。该文件可能是扫描件、图片型 PDF、加密 PDF，或文档内容本身不可复制。"},
		{"message only", "ready", "", "private upstream diagnostic", "PDF 未提取到可读文本。该文件可能是扫描件、图片型 PDF、加密 PDF，或文档内容本身不可复制。"},
		{"unknown code", "failed", "unknown_error", "private upstream diagnostic", "文件处理失败，请稍后重试。"},
		{"code without failed status", "ready", "extract_timeout", "", "文件提取超时，请稍后重试，或缩小文件后重试。"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := fileProcessingStatusFromFileObject(&model.FileObject{
				FileCategory: "pdf", ProcessingStatus: tc.status,
				ProcessingErrorCode: tc.code, ProcessingErrorMessage: tc.message,
			})
			if got.ErrorMessage != tc.want {
				t.Fatalf("message = %q, want %q", got.ErrorMessage, tc.want)
			}
			if got.ErrorCode != tc.code {
				t.Fatalf("error code changed: %q", got.ErrorCode)
			}
		})
	}
}
