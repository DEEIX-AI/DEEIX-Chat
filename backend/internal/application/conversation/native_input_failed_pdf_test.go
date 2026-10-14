package conversation

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	appprocessing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/processing"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/objectstorage"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// failedFileProcessingRepository 让 WaitUntilReady 读到一个处理失败的文件。
type failedFileProcessingRepository struct {
	repository.FileProcessingStatusRepository
	file model.FileObject
}

func (r failedFileProcessingRepository) GetActiveFileObjectByID(context.Context, uint, string) (*model.FileObject, error) {
	file := r.file
	return &file, nil
}

// scannedPDF 是文本提取失败的本轮 PDF（扫描件、图片型 PDF）。
func scannedPDF() AttachmentInput {
	return AttachmentInput{
		FileID: "scan", FileName: "scan.pdf", Kind: "file", FileCategory: fileCategoryPDF, MimeType: "application/pdf",
		StoragePath: "files/scan", FileSize: int64(len("%PDF-1.7 scan")), Current: true, MessageRole: "user",
	}
}

func failedPDFService(store *objectstorage.LocalStore) *Service {
	return &Service{
		storeProvider: &conversationTestStoreProvider{store: store},
		processingSvc: appprocessing.NewServiceWithRuntime(appprocessing.Dependencies{
			Repository: failedFileProcessingRepository{file: model.FileObject{
				FileID: "scan", FileCategory: fileCategoryPDF, ProcessingStatus: "failed",
				ProcessingErrorCode: "extract_empty", ProcessingErrorMessage: "PDF 未提取到可读文本。",
			}},
		}),
	}
}

// 模型能直接阅读 PDF 时，本轮 PDF 提取失败不再拒绝发送；否则仍按「文件未准备完成」拒绝。
func TestHydrateAttachmentsForSendKeepsFailedPDFForNativeReaders(t *testing.T) {
	service := failedPDFService(objectstorage.NewLocal(t.TempDir()))

	items, err := service.hydrateAttachmentsForSend(t.Context(), 1, []AttachmentInput{scannedPDF()}, nativeInputPolicy{PDF: true}, nil)
	if err != nil {
		t.Fatalf("native PDF readers must accept a failed PDF: %v", err)
	}
	if len(items) != 1 || items[0].ProcessingStatus != "failed" || items[0].ProcessingErrorCode != "extract_empty" {
		t.Fatalf("expected the failure state to be kept for planning, got %#v", items)
	}

	if _, err = service.hydrateAttachmentsForSend(t.Context(), 1, []AttachmentInput{scannedPDF()}, nativeInputPolicy{}, nil); !errors.Is(err, ErrFileProcessingNotReady) {
		t.Fatalf("models without native PDF input must still reject the file, got %v", err)
	}

	oversized := scannedPDF()
	oversized.FileSize = maxNativePDFTotalBytes + 1
	if _, err = service.hydrateAttachmentsForSend(t.Context(), 1, []AttachmentInput{oversized}, nativeInputPolicy{PDF: true}, nil); !errors.Is(err, ErrFileProcessingNotReady) {
		t.Fatalf("PDFs over the native budget must still be rejected, got %v", err)
	}
}

// 提取失败的 PDF 在支持 PDF 的路由上以原生文档发送；故障转移到不支持的路由时留「无法读取」说明。
func TestBuildMessageRoutePromptSendsFailedPDFNatively(t *testing.T) {
	store := objectstorage.NewLocal(t.TempDir())
	if _, err := store.Put(t.Context(), "files/scan", bytes.NewReader([]byte("%PDF-1.7 scan")), objectstorage.PutOptions{ContentType: "application/pdf"}); err != nil {
		t.Fatalf("put pdf: %v", err)
	}
	service := failedPDFService(store)
	hydrated, err := service.hydrateAttachmentsForSend(t.Context(), 1, []AttachmentInput{scannedPDF()}, nativeInputPolicy{PDF: true}, nil)
	if err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	planned := buildConversationFileContextPlan(hydrated, "", config.Config{}, "", "", false).Attachments
	input := messageRoutePromptInput{
		DomainMessages:    []model.Message{{Role: "user", Content: "这是什么"}},
		ConversationFiles: planned,
	}

	native, err := service.buildMessageRoutePrompt(t.Context(), &channel.ResolvedRoute{
		Protocol: llm.AdapterAnthropicMessages, CatalogInputModalities: []string{"text", "image", "pdf"},
	}, input)
	if err != nil {
		t.Fatalf("build native prompt: %v", err)
	}
	parts := native.Messages[len(native.Messages)-1].Parts
	if len(parts) != 3 || parts[1].Kind != llm.ContentPartDocument || string(parts[1].Data) != "%PDF-1.7 scan" {
		t.Fatalf("expected the failed PDF as a native document, got %#v", parts)
	}

	fallback, err := service.buildMessageRoutePrompt(t.Context(), &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIChatCompletions, BaseURL: "https://relay.example.com/v1", CatalogInputModalities: []string{"text", "image", "pdf"},
	}, input)
	if err != nil {
		t.Fatalf("build fallback prompt: %v", err)
	}
	documents, _ := turnParts(t, fallback.Messages[len(fallback.Messages)-1])
	if !strings.Contains(documents, `<document source="scan.pdf" access="unavailable">`) {
		t.Fatalf("routes without native PDF must explain the file is unreadable, got %q", documents)
	}
}
