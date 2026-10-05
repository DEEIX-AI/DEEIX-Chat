package channel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	appchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/gin-gonic/gin"
)

// 上游的兼容类型解析不出目录接口使用的协议时，预览与同步都应返回 400 protocol_required，而不是笼统的 500。
func TestUpstreamModelSyncEndpointsMapProtocolRequiredToBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const encryptionKey = "test-data-encryption-key-32-bytes"
	apiKeysEnc, err := secretbox.EncryptString(encryptionKey, `{"strategy":"failover","keys":[{"key":"sk-test","status":"active"}]}`)
	if err != nil {
		t.Fatalf("encrypt api keys: %v", err)
	}
	repo := syncUpstreamRepo{upstream: domainchannel.Upstream{ID: 1, Name: "legacy", Compatible: "legacy-compatible", APIKeysEnc: apiKeysEnc}}
	service := appchannel.NewServiceWithRuntime(config.NewRuntime(config.Config{DataEncryptionKey: encryptionKey}), repo, nil, nil, unusedModelsGateway{})
	handler := NewHandler(service)
	router := gin.New()
	router.GET("/api/v1/admin/llm/upstreams/:id/models/remote", handler.ListRemoteModels)
	router.POST("/api/v1/admin/llm/upstreams/:id/models/sync", handler.SyncUpstreamModels)

	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/api/v1/admin/llm/upstreams/1/models/remote", nil),
		httptest.NewRequest(http.MethodPost, "/api/v1/admin/llm/upstreams/1/models/sync", nil),
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s %s status = %d, want 400; body=%s", request.Method, request.URL.Path, response.Code, response.Body.String())
		}
		var payload struct {
			ErrorCode string `json:"errorCode"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if payload.ErrorCode != "llm.protocol_required" {
			t.Fatalf("%s %s errorCode = %q, want llm.protocol_required", request.Method, request.URL.Path, payload.ErrorCode)
		}
	}
}

// syncUpstreamRepo 只实现同步入口会用到的查询；其余方法未实现，被调用即说明测试路径偏离预期。
type syncUpstreamRepo struct {
	repository.ChannelRepository
	upstream domainchannel.Upstream
}

func (r syncUpstreamRepo) GetUpstreamByID(context.Context, uint) (*domainchannel.Upstream, error) {
	item := r.upstream
	return &item, nil
}

type unusedModelsGateway struct{}

func (unusedModelsGateway) Generate(context.Context, llm.RouteConfig, llm.GenerateInput) (*llm.GenerateOutput, error) {
	return nil, errors.New("not used")
}

func (unusedModelsGateway) ListModels(context.Context, llm.RouteConfig) ([]llm.ModelItem, error) {
	return nil, errors.New("not used")
}
