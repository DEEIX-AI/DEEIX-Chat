package settings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestTriggerReindexReturnsBadRequestWhenEmbeddingNotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	runtime := config.NewRuntime(config.Config{})
	handler := &Handler{
		embeddingSvc: appembedding.NewServiceWithRuntime(runtime, testEmbeddingRepo{}, nil, nil, zap.NewNop()),
	}

	router := gin.New()
	router.POST("/reindex", handler.TriggerReindex)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/reindex", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var body response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ErrorCode != "embedding.service_not_configured" {
		t.Fatalf("expected embedding.service_not_configured, got %q", body.ErrorCode)
	}
}

type testEmbeddingRepo struct{}

func (testEmbeddingRepo) VectorStoreAvailable(context.Context) (bool, error) {
	return true, nil
}

func (testEmbeddingRepo) GetActiveFileObjectByID(context.Context, uint, string) (*domainconversation.FileObject, error) {
	return nil, nil
}

func (testEmbeddingRepo) GetFileObjectProcessingByObjectID(context.Context, uint) (*domainconversation.FileObjectProcessing, error) {
	return nil, nil
}

func (testEmbeddingRepo) UpdateFileObjectEmbedStatus(context.Context, uint, string, string, string) error {
	return nil
}

func (testEmbeddingRepo) UpdateFileObjectChunkCount(context.Context, uint, int) error {
	return nil
}

func (testEmbeddingRepo) ReplaceFileChunks(context.Context, uint, []domainconversation.FileChunk, [][]float32) error {
	return nil
}

func (testEmbeddingRepo) MarkAllEmbeddedFilesStale(context.Context) (int64, error) {
	return 0, nil
}

func (testEmbeddingRepo) CountFilesByEmbedStatus(context.Context, string) (int64, error) {
	return 0, nil
}

func (testEmbeddingRepo) ListFilesForReindex(context.Context, int, uint) ([]domainconversation.FileObject, error) {
	return nil, nil
}
