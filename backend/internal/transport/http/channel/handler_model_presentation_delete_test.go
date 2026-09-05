package channel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/gin-gonic/gin"
)

func TestDeleteModelVendorReturnsReferencedModelDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	presentationRepo := handlerModelPresentationRepo{deleteVendorErr: &repository.ModelVendorDeleteBlockedError{
		Reason:         repository.ModelVendorDeleteReasonReferencedModels,
		ReferenceCount: 2,
		Models:         []repository.ModelVendorReference{{ID: 7, PlatformModelName: "acme-chat"}},
	}}
	service := appchannel.NewServiceWithRuntime(config.NewRuntime(config.Config{}), nil, presentationRepo, nil, nil)
	handler := NewHandler(service)
	router := gin.New()
	router.DELETE("/api/v1/admin/llm/model-vendors/:key", handler.DeleteModelVendor)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/v1/admin/llm/model-vendors/acme", nil))
	if response.Code != http.StatusConflict {
		t.Fatalf("delete vendor status = %d, want 409; body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		ErrorCode string                           `json:"errorCode"`
		Details   ModelVendorDeleteConflictDetails `json:"details"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode delete vendor response: %v", err)
	}
	if payload.ErrorCode != "llm.model_vendor_in_use" || payload.Details.ReferenceCount != 2 ||
		len(payload.Details.Models) != 1 || payload.Details.Models[0].PlatformModelName != "acme-chat" {
		t.Fatalf("unexpected delete vendor response: %#v", payload)
	}
}

type handlerModelPresentationRepo struct {
	deleteVendorErr error
}

func (handlerModelPresentationRepo) CreateModelVendor(context.Context, *domainchannel.ModelVendor) error {
	return nil
}

func (handlerModelPresentationRepo) UpdateModelVendor(context.Context, string, repository.UpdateModelVendorInput) error {
	return nil
}

func (r handlerModelPresentationRepo) DeleteModelVendor(context.Context, string) error {
	return r.deleteVendorErr
}

func (handlerModelPresentationRepo) GetModelVendorByKey(context.Context, string) (*domainchannel.ModelVendor, error) {
	return nil, repository.ErrNotFound
}

func (handlerModelPresentationRepo) ListModelVendors(context.Context, repository.ListModelVendorsInput) ([]domainchannel.ModelVendor, int64, error) {
	return nil, 0, nil
}

func (handlerModelPresentationRepo) CreateModelDisplayGroup(context.Context, *domainchannel.ModelDisplayGroup, []uint) error {
	return nil
}

func (handlerModelPresentationRepo) UpdateModelDisplayGroup(context.Context, uint, repository.UpdateModelDisplayGroupInput) error {
	return nil
}

func (handlerModelPresentationRepo) SetModelsDisplayGroup(context.Context, []uint, uint) error {
	return nil
}

func (handlerModelPresentationRepo) GetModelDisplayGroupByID(context.Context, uint) (*domainchannel.ModelDisplayGroup, error) {
	return nil, repository.ErrNotFound
}

func (handlerModelPresentationRepo) ListModelDisplayGroups(context.Context, repository.ListModelDisplayGroupsInput) ([]domainchannel.ModelDisplayGroup, int64, error) {
	return nil, 0, nil
}

func (handlerModelPresentationRepo) DeleteModelDisplayGroup(context.Context, uint) error {
	return nil
}

var _ repository.ModelPresentationRepository = handlerModelPresentationRepo{}
