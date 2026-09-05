package settings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	appsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/settings"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestProspectiveEmbeddingSpaceAppliesRelevantPatchItems(t *testing.T) {
	cfg := config.Config{
		RAGModel:                  "old-model",
		EmbeddingOutputDimensions: 1536,
		EmbeddingHost:             "https://old.example/v1",
	}
	model, dimensions, host := prospectiveEmbeddingSpace(cfg, []PatchItem{
		{Namespace: "chat", Key: "rag_top_k", Value: "8"},
		{Namespace: "file", Key: "rag_model", Value: "new-model"},
		{Namespace: "file", Key: "embedding_output_dimensions", Value: "4096"},
		{Namespace: "file", Key: "embedding_host", Value: "https://new.example/v1/"},
	})
	if model != "new-model" || dimensions != 4096 || host != "https://new.example/v1/" {
		t.Fatalf("unexpected embedding space: model=%q dimensions=%d host=%q", model, dimensions, host)
	}
}

func TestUpsertSettingPatchItemReplacesInternalSignature(t *testing.T) {
	items := []appsettings.PatchItem{{Namespace: "file", Key: "embedding_model_signature", Value: "user-value"}}
	items = upsertSettingPatchItem(items, appsettings.PatchItem{
		Namespace: "file",
		Key:       "embedding_model_signature",
		Value:     "derived-value",
	})
	if len(items) != 1 || items[0].Value != "derived-value" {
		t.Fatalf("unexpected patch items: %#v", items)
	}
}

func TestTouchesEmbeddingSpaceIgnoresDerivedSignature(t *testing.T) {
	if !touchesEmbeddingSpace([]PatchItem{{Namespace: "file", Key: "embedding_host", Value: "https://example.com/v1"}}) {
		t.Fatal("expected embedding_host to touch the vector space")
	}
	if touchesEmbeddingSpace([]PatchItem{{Namespace: "file", Key: "embedding_model_signature", Value: "client-value"}}) {
		t.Fatal("derived signature must not be treated as a user-controlled vector-space setting")
	}
	if touchesEmbeddingSpace([]PatchItem{{Namespace: "chat", Key: "rag_top_k", Value: "8"}}) {
		t.Fatal("retrieval tuning must not invalidate the vector space")
	}
}

func TestContainsSettingPatchMatchesExactNamespaceAndKey(t *testing.T) {
	items := []PatchItem{{Namespace: "file", Key: "embedding_model_signature", Value: "client-value"}}
	if !containsSettingPatch(items, "file", "embedding_model_signature") {
		t.Fatal("expected exact setting patch to be found")
	}
	if containsSettingPatch(items, "chat", "embedding_model_signature") {
		t.Fatal("unexpected namespace match")
	}
}

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

func (testEmbeddingRepo) GetActiveFileObjectsByIDs(context.Context, uint, []string) ([]domainconversation.FileObject, error) {
	return nil, nil
}

func (testEmbeddingRepo) GetFileObjectProcessingByObjectID(context.Context, uint) (*domainconversation.FileObjectProcessing, error) {
	return nil, nil
}

func (testEmbeddingRepo) QueueFileEmbedding(context.Context, uint, string, string) (bool, error) {
	return true, nil
}

func (testEmbeddingRepo) ClaimFileEmbedding(context.Context, uint, string, string) (bool, error) {
	return true, nil
}

func (testEmbeddingRepo) UpdateFileObjectEmbedStatus(context.Context, uint, string, string, string, string) (bool, error) {
	return true, nil
}

func (testEmbeddingRepo) UpdateFileObjectChunkCount(context.Context, uint, string, int) (bool, error) {
	return true, nil
}

func (testEmbeddingRepo) ReplaceFileChunks(context.Context, uint, string, []domainconversation.FileChunk, [][]float32) (bool, error) {
	return true, nil
}

func (testEmbeddingRepo) MarkEmbeddedFilesStale(context.Context, string) (int64, error) {
	return 0, nil
}

func (testEmbeddingRepo) CountFilesByEmbedStatus(context.Context, string) (int64, error) {
	return 0, nil
}

func (testEmbeddingRepo) ListFilesForReindex(context.Context, int, uint) ([]domainconversation.FileObject, error) {
	return nil, nil
}
