package processing

import (
	"context"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/extraction"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

func TestMarkClaimedFileProcessingEmptyRecordsTerminalEmptyState(t *testing.T) {
	repo := &processingStateRepositoryStub{}
	service := NewServiceWithRuntime(Dependencies{
		Config:           config.NewRuntime(config.Config{}),
		Repository:       repo,
		ExtractorVersion: "pipeline-v1",
	})
	file := &domainconversation.FileObject{ID: 9, UserID: 7, FileID: "file_1", DetectedMIME: "application/pdf", FileCategory: "pdf"}

	if err := service.markClaimedFileProcessingEmpty(context.Background(), file, "attempt_1", "rapidocr_ocr_empty_content"); err != nil {
		t.Fatalf("markClaimedFileProcessingEmpty() error = %v", err)
	}

	state := repo.claimedState
	if state == nil || repo.claimedAttemptID != "attempt_1" {
		t.Fatalf("claimed state was not persisted: %#v", state)
	}
	if state.ProcessingStatus != "ready" || state.ProcessingReady {
		t.Fatalf("empty file must finish the pipeline without being usable: %#v", state)
	}
	if state.ExtractStatus != domainconversation.FileSubprocessStatusEmpty {
		t.Fatalf("expected extract_status=empty, got %q", state.ExtractStatus)
	}
	if state.RAGReady || state.RAGReason != "rapidocr_ocr_empty_content" {
		t.Fatalf("expected RAG reason to carry the empty code, got %#v", state)
	}
	if state.ErrorCode != "" || state.ErrorMessage != "" {
		t.Fatalf("empty is not a failure and must not set error fields: %#v", state)
	}
}

func TestExtractionIsEmptyContentMatchesEveryEngineSuffix(t *testing.T) {
	for _, code := range []string{"tika_empty_content", "docling_empty_content", "mineru_empty_content", "rapidocr_ocr_empty_content", "llm_ocr_empty_content"} {
		if !extraction.IsEmptyContent(extraction.NewError(code, context.DeadlineExceeded)) {
			t.Fatalf("%s must classify as empty content", code)
		}
	}
	for _, code := range []string{"tika_unprocessable", "rapidocr_ocr_failed", "extract_failed", ""} {
		if extraction.IsEmptyContent(extraction.NewError(code, context.DeadlineExceeded)) {
			t.Fatalf("%s must not classify as empty content", code)
		}
	}
	if extraction.IsEmptyContent(nil) || extraction.IsEmptyContent(context.Canceled) {
		t.Fatal("nil and uncoded errors must not classify as empty content")
	}
}
