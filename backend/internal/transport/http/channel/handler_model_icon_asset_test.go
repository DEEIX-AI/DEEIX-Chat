package channel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/objectstore"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/gin-gonic/gin"
)

func TestGetModelIconAssetValidatesObjectBeforeConditionalResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	data := []byte("validated image content")
	digest := sha256.Sum256(data)
	hash := hex.EncodeToString(digest[:])
	readyAt := time.Now()
	item := domainchannel.ModelIconAsset{
		ID: 1, PublicID: "ico_00000000000000000000000000000001", SHA256: hash,
		StoragePath: "model-icons/" + hash[:2] + "/" + hash + ".png", ContentType: "image/png", SizeBytes: int64(len(data)),
		ReadyAt: &readyAt, LeaseExpiresAt: time.Now().Add(time.Hour),
	}
	store := objectstore.NewLocal(t.TempDir())
	if _, err := store.Put(t.Context(), item.StoragePath, bytes.NewReader(data), objectstore.PutOptions{
		SizeBytes: int64(len(data)), ContentType: item.ContentType,
	}); err != nil {
		t.Fatalf("seed icon object: %v", err)
	}
	provider := &countingModelIconStoreProvider{store: store}
	service := appchannel.NewServiceWithRuntime(config.NewRuntime(config.Config{}), nil, nil, nil, nil)
	service.SetModelIconAssetRepository(handlerModelIconAssetRepo{item: item})
	service.SetObjectStoreProvider(provider)
	handler := NewHandler(service)
	router := gin.New()
	router.GET("/api/v1/llm/icon-assets/:public_id", handler.GetModelIconAsset)

	etag := `"` + hash + `"`
	conditional := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/llm/icon-assets/"+item.PublicID, nil)
	request.Header.Set("If-None-Match", etag)
	router.ServeHTTP(conditional, request)
	if conditional.Code != http.StatusNotModified {
		t.Fatalf("conditional status = %d, want 304", conditional.Code)
	}
	if provider.opens != 1 {
		t.Fatalf("conditional request opened object store %d times, want 1", provider.opens)
	}
	if conditional.Header().Get("ETag") != etag || conditional.Header().Get("Cache-Control") == "" {
		t.Fatalf("conditional cache headers = %#v", conditional.Header())
	}

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/llm/icon-assets/"+item.PublicID, nil))
	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), data) {
		t.Fatalf("content response status=%d body=%q", response.Code, response.Body.Bytes())
	}
	if provider.opens != 2 {
		t.Fatalf("content request opened object store %d times, want 2", provider.opens)
	}
	if response.Header().Get("ETag") != etag ||
		response.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" ||
		response.Header().Get("X-Content-Type-Options") != "nosniff" ||
		response.Header().Get("Cross-Origin-Resource-Policy") != "cross-origin" {
		t.Fatalf("content cache/security headers = %#v", response.Header())
	}
}

func TestGetModelIconAssetDoesNotReturnNotModifiedWhenObjectIsMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	data := []byte("missing image content")
	digest := sha256.Sum256(data)
	hash := hex.EncodeToString(digest[:])
	readyAt := time.Now()
	item := domainchannel.ModelIconAsset{
		ID: 2, PublicID: "ico_00000000000000000000000000000002", SHA256: hash,
		StoragePath: "model-icons/" + hash[:2] + "/" + hash + ".png", ContentType: "image/png", SizeBytes: int64(len(data)),
		ReadyAt: &readyAt, LeaseExpiresAt: time.Now().Add(time.Hour),
	}
	provider := &countingModelIconStoreProvider{store: objectstore.NewLocal(t.TempDir())}
	service := appchannel.NewServiceWithRuntime(config.NewRuntime(config.Config{}), nil, nil, nil, nil)
	service.SetModelIconAssetRepository(handlerModelIconAssetRepo{item: item})
	service.SetObjectStoreProvider(provider)
	handler := NewHandler(service)
	router := gin.New()
	router.GET("/api/v1/llm/icon-assets/:public_id", handler.GetModelIconAsset)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/llm/icon-assets/"+item.PublicID, nil)
	request.Header.Set("If-None-Match", `"`+hash+`"`)
	responseRecorder := httptest.NewRecorder()
	router.ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusNotFound {
		t.Fatalf("missing object status = %d, want 404; body=%s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if provider.opens != 1 {
		t.Fatalf("missing object request opened object store %d times, want 1", provider.opens)
	}
}

func TestUploadModelIconAssetRejectsOversizedMultipartBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := appchannel.NewServiceWithRuntime(config.NewRuntime(config.Config{}), nil, nil, nil, nil)
	handler := NewHandler(service)
	router := gin.New()
	router.POST("/api/v1/admin/llm/icon-assets", handler.UploadModelIconAsset)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "oversized.png")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err = part.Write(make([]byte, maxModelIconUploadRequestBytes+1)); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/llm/icon-assets", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized upload status = %d, want 413; body=%s", response.Code, response.Body.String())
	}
}

func TestListAndDeleteModelIconAssetsUseAdminContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	readyAt := time.Now()
	item := domainchannel.ModelIconAsset{
		ID: 3, PublicID: "ico_00000000000000000000000000000003", ContentType: "image/png",
		SizeBytes: 128, Width: 24, Height: 24, ReadyAt: &readyAt, CreatedAt: readyAt,
	}
	service := appchannel.NewServiceWithRuntime(config.NewRuntime(config.Config{}), nil, nil, nil, nil)
	service.SetModelIconAssetRepository(handlerModelIconAssetRepo{item: item})
	handler := NewHandler(service)
	router := gin.New()
	router.GET("/api/v1/admin/llm/icon-assets", handler.ListModelIconAssets)
	router.DELETE("/api/v1/admin/llm/icon-assets/:public_id", handler.DeleteModelIconAsset)

	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/api/v1/admin/llm/icon-assets", nil))
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listResponse.Code, listResponse.Body.String())
	}
	var listEnvelope struct {
		Data struct {
			Total   int64                            `json:"total"`
			Results []ModelIconAssetListItemResponse `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listEnvelope); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if listEnvelope.Data.Total != 1 || len(listEnvelope.Data.Results) != 1 || listEnvelope.Data.Results[0].PublicID != item.PublicID {
		t.Fatalf("unexpected list response: %#v", listEnvelope)
	}

	deleteResponse := httptest.NewRecorder()
	router.ServeHTTP(deleteResponse, httptest.NewRequest(http.MethodDelete, "/api/v1/admin/llm/icon-assets/"+item.PublicID, nil))
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}
}

func TestDeleteModelIconAssetReturnsReferenceSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	readyAt := time.Now()
	item := domainchannel.ModelIconAsset{ID: 4, PublicID: "ico_00000000000000000000000000000004", ReadyAt: &readyAt}
	service := appchannel.NewServiceWithRuntime(config.NewRuntime(config.Config{}), nil, nil, nil, nil)
	service.SetModelIconAssetRepository(handlerModelIconAssetRepo{
		item:       item,
		references: repository.ModelIconAssetReferenceSummary{Models: 2, Vendors: 1},
	})
	handler := NewHandler(service)
	router := gin.New()
	router.DELETE("/api/v1/admin/llm/icon-assets/:public_id", handler.DeleteModelIconAsset)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/v1/admin/llm/icon-assets/"+item.PublicID, nil))
	if response.Code != http.StatusConflict {
		t.Fatalf("delete conflict status=%d body=%s", response.Code, response.Body.String())
	}
	var envelope struct {
		ErrorCode string                              `json:"errorCode"`
		Details   ModelIconAssetDeleteConflictDetails `json:"details"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode conflict response: %v", err)
	}
	if envelope.ErrorCode != "llm.model_icon_asset_in_use" || envelope.Details.ReferenceCount != 3 {
		t.Fatalf("unexpected conflict response: %#v", envelope)
	}
}

type countingModelIconStoreProvider struct {
	store objectstore.Store
	opens int
}

func (p *countingModelIconStoreProvider) Open(context.Context) (objectstore.Store, error) {
	p.opens++
	return p.store, nil
}

type handlerModelIconAssetRepo struct {
	item       domainchannel.ModelIconAsset
	references repository.ModelIconAssetReferenceSummary
}

func (r handlerModelIconAssetRepo) CreateModelIconAsset(context.Context, *domainchannel.ModelIconAsset) error {
	return repository.ErrInvalidInput
}

func (r handlerModelIconAssetRepo) GetModelIconAssetByPublicID(_ context.Context, publicID string) (*domainchannel.ModelIconAsset, error) {
	if publicID != r.item.PublicID {
		return nil, repository.ErrNotFound
	}
	item := r.item
	return &item, nil
}

func (r handlerModelIconAssetRepo) GetModelIconAssetBySHA256(context.Context, string) (*domainchannel.ModelIconAsset, error) {
	return nil, repository.ErrNotFound
}

func (r handlerModelIconAssetRepo) ListModelIconAssets(context.Context, int, int) ([]domainchannel.ModelIconAsset, int64, error) {
	if r.item.ReadyAt == nil || r.item.DeleteRequestedAt != nil || r.item.DeletingAt != nil {
		return []domainchannel.ModelIconAsset{}, 0, nil
	}
	return []domainchannel.ModelIconAsset{r.item}, 1, nil
}

func (r handlerModelIconAssetRepo) RefreshModelIconAssetUploadLease(context.Context, string, time.Time, time.Time) error {
	return repository.ErrNotFound
}

func (r handlerModelIconAssetRepo) MarkModelIconAssetReady(context.Context, string, time.Time) error {
	return repository.ErrNotFound
}

func (r handlerModelIconAssetRepo) ReserveModelIconAssetReference(context.Context, string, time.Time) error {
	return repository.ErrNotFound
}

func (r handlerModelIconAssetRepo) ListExpiredModelIconAssets(context.Context, time.Time, int) ([]domainchannel.ModelIconAsset, error) {
	return []domainchannel.ModelIconAsset{}, nil
}

func (r handlerModelIconAssetRepo) HasModelIconAssetReference(context.Context, string) (bool, error) {
	return false, nil
}

func (r handlerModelIconAssetRepo) GetModelIconAssetReferenceSummary(context.Context, string) (repository.ModelIconAssetReferenceSummary, error) {
	return r.references, nil
}

func (r handlerModelIconAssetRepo) MarkModelIconAssetUnreferenced(context.Context, uint, time.Time, time.Time, time.Time) (bool, error) {
	return false, nil
}

func (r handlerModelIconAssetRepo) RequestModelIconAssetDeletion(context.Context, uint, time.Time, time.Time) error {
	return nil
}

func (r handlerModelIconAssetRepo) ClaimModelIconAssetDeletion(context.Context, uint, time.Time, time.Time) (bool, error) {
	return false, nil
}

func (r handlerModelIconAssetRepo) DeleteClaimedModelIconAsset(context.Context, uint) error {
	return nil
}

var _ repository.ModelIconAssetRepository = handlerModelIconAssetRepo{}
