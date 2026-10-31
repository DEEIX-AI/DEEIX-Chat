package conversation

import (
	"testing"

	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	appprocessing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/processing"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

func TestFileResponsesDoNotInventFailureMessages(t *testing.T) {
	for _, tc := range []struct {
		name, status, extract, embed, code, message, want string
	}{
		{"successful MinerU PDF", "ready", "ready", "none", "", "", ""},
		{"queued PDF", "queued", "none", "none", "", "", ""},
		{"extracting PDF", "extracting", "none", "none", "", "", ""},
		{"empty PDF", "ready", "empty", "empty", "", "", ""},
		{"stalled embedding marked failed", "ready", "ready", "failed", "embed_failed", "embedding stalled", "文件已完成文本提取，但向量化失败。"},
		{"whitespace error fields", "ready", "ready", "none", " \t", " \n", ""},
		{"stale message without code", "ready", "ready", "none", "", "old diagnostic", ""},
		{"actual failure", "failed", "failed", "none", "extract_timeout", "", "文件提取超时，请稍后重试，或缩小文件后重试。"},
		{"legacy failure without fields", "failed", "failed", "none", "", "", "PDF 未提取到可读文本。该文件可能是扫描件、图片型 PDF、加密 PDF，或文档内容本身不可复制。"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := &model.FileObject{
				FileID: "synthetic-pdf", FileCategory: "pdf", MimeType: "application/pdf",
				ProcessingStatus: tc.status, ProcessingReady: tc.extract == "ready",
				ExtractStatus: tc.extract, ExtractEngine: "mineru", ExtractChars: 638, ExtractPages: 1,
				EmbedStatus: tc.embed, ProcessingErrorCode: tc.code, ProcessingErrorMessage: tc.message,
			}
			object := toFileObjectResponse(file, appembedding.FileVectorizationCapability{})
			if object.ProcessingErrorMessage != tc.want {
				t.Errorf("file response message = %q, want %q", object.ProcessingErrorMessage, tc.want)
			}
			status := toFileProcessingStatusResponse(&appprocessing.FileProcessingStatusDTO{
				FileID: file.FileID, FileCategory: file.FileCategory, ProcessingStatus: tc.status,
				ProcessingReady: file.ProcessingReady, ExtractStatus: tc.extract, EmbedStatus: tc.embed,
				ErrorCode: tc.code, ErrorMessage: object.ProcessingErrorMessage, ExtractChars: 638, ExtractPages: 1,
			})
			if status.ErrorMessage != tc.want {
				t.Errorf("processing response message = %q, want %q", status.ErrorMessage, tc.want)
			}
			if object.ProcessingStatus != tc.status || status.ProcessingStatus != tc.status {
				t.Fatal("mapping changed processing status")
			}
		})
	}
}
