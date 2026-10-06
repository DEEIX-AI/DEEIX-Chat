package channel

import (
	"context"
	"errors"
	"reflect"
	"testing"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// anthropicGatewayUpstream 模拟以 Anthropic 兼容类型接入的多协议网关：它的模型目录里混有图像、视频模型，
// 而 Anthropic 的系统兜底只覆盖对话与音频（#830）。
func anthropicGatewayUpstream() *domainchannel.Upstream {
	return &domainchannel.Upstream{ID: 9, Name: "gateway", Compatible: "anthropic", BaseURL: "https://gateway.example.com/anthropic/v1"}
}

func anthropicGatewayCatalog() []llm.ModelItem {
	return []llm.ModelItem{
		{ID: "claude-sonnet-4-5", OwnedBy: "anthropic"},
		{ID: "gpt-image-2", OwnedBy: "openai"},
		{ID: "nano-banana-pro", OwnedBy: "google"},
		{ID: "veo-3", OwnedBy: "google"},
	}
}

func TestReconcileRemoteModelSnapshotStoresModelsWithoutResolvableProtocol(t *testing.T) {
	repo := &modelUpdateRepo{upstreamModels: map[string]domainchannel.UpstreamModel{}}
	service := newTestService(config.Config{}, repo, repo, nil, nil)

	result, err := service.reconcileRemoteModelSnapshot(t.Context(), anthropicGatewayUpstream(), anthropicGatewayCatalog(), false)
	if err != nil {
		t.Fatalf("sync with unresolvable media models: %v", err)
	}
	if !repo.transactionCommitted || result.CreatedUpstreamModels != 4 {
		t.Fatalf("expected every remote model to be stored, got %+v", result)
	}
	if got := repo.upstreamModels["claude-sonnet-4-5"].SuggestedProtocol; got != "anthropic_messages" {
		t.Fatalf("chat model protocol = %q, want anthropic_messages", got)
	}
	for _, name := range []string{"gpt-image-2", "nano-banana-pro", "veo-3"} {
		stored, ok := repo.upstreamModels[name]
		if !ok || stored.Status != "active" || stored.SuggestedProtocol != "" {
			t.Fatalf("expected %s stored active without a suggested protocol, got %+v", name, stored)
		}
	}
	if want := []string{"gpt-image-2", "nano-banana-pro", "veo-3"}; !reflect.DeepEqual(result.UnresolvedProtocolModels, want) {
		t.Fatalf("unresolved protocol models = %v, want %v", result.UnresolvedProtocolModels, want)
	}
}

func TestReconcileRemoteModelSnapshotBackfillsProtocolOnceDefaultsAreSet(t *testing.T) {
	repo := &modelUpdateRepo{upstreamModels: map[string]domainchannel.UpstreamModel{}}
	service := newTestService(config.Config{}, repo, repo, nil, nil)
	upstream := anthropicGatewayUpstream()
	if _, err := service.reconcileRemoteModelSnapshot(t.Context(), upstream, anthropicGatewayCatalog(), false); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	upstream.ProtocolDefaultsJSON = `{"image_gen":"openai_image_generations","image_edit":"openai_image_edits"}`
	result, err := service.reconcileRemoteModelSnapshot(t.Context(), upstream, anthropicGatewayCatalog(), false)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if got := repo.upstreamModels["gpt-image-2"].SuggestedProtocol; got != "openai_image_generations" {
		t.Fatalf("image model protocol after defaults = %q, want openai_image_generations", got)
	}
	if result.UpdatedUpstreamModels != 2 {
		t.Fatalf("expected both image models to count as updated, got %+v", result)
	}
	if want := []string{"veo-3"}; !reflect.DeepEqual(result.UnresolvedProtocolModels, want) {
		t.Fatalf("unresolved protocol models = %v, want %v", result.UnresolvedProtocolModels, want)
	}
}

func TestReconcileRemoteModelSnapshotStillRollsBackOnStorageErrors(t *testing.T) {
	storageErr := errors.New("storage unavailable")
	repo := &modelUpdateRepo{upstreamModels: map[string]domainchannel.UpstreamModel{}, catalogApplyErr: storageErr}
	service := newTestService(config.Config{}, repo, repo, nil, nil)

	if _, err := service.reconcileRemoteModelSnapshot(t.Context(), anthropicGatewayUpstream(), anthropicGatewayCatalog(), false); !errors.Is(err, storageErr) {
		t.Fatalf("expected the storage error to abort the sync, got %v", err)
	}
	if repo.transactionCommitted || len(repo.upstreamModels) != 0 {
		t.Fatalf("expected nothing committed, got %+v", repo.upstreamModels)
	}
}

func TestBuildUpstreamModelSyncPlanToleratesManagedModelsWithoutResolvableProtocol(t *testing.T) {
	upstream := anthropicGatewayUpstream()
	managed := domainchannel.UpstreamModel{
		ID: 1, UpstreamID: 9, BindingCode: "image-code", UpstreamModelName: "gpt-image-2",
		Vendor: "openai", Icon: normalizeModelIcon("", "openai", "gpt-image-2"),
		KindsJSON: inferKindsJSON("gpt-image-2"), Status: "active", Source: "sync",
	}

	plan, err := buildUpstreamModelSyncPlan(upstream, anthropicGatewayCatalog(), []domainchannel.UpstreamModel{managed}, map[string]repositoryUpstreamModelSnapshot{})
	if err != nil {
		t.Fatalf("build sync plan: %v", err)
	}
	if !reflect.DeepEqual(plan.UnchangedModels, []string{"gpt-image-2"}) {
		t.Fatalf("expected the managed image model unchanged, got %+v", plan)
	}
	if want := []string{"gpt-image-2", "nano-banana-pro", "veo-3"}; !reflect.DeepEqual(plan.UnresolvedProtocolModels, want) {
		t.Fatalf("unresolved protocol models = %v, want %v", plan.UnresolvedProtocolModels, want)
	}
}

func TestListRemoteModelsReportsUnresolvedProtocolModels(t *testing.T) {
	const encryptionKey = "test-data-encryption-key-32-bytes"
	apiKeysEnc, err := encryptAPIKeys(encryptionKey, `{"strategy":"failover","keys":[{"key":"sk-test","status":"active"}]}`)
	if err != nil {
		t.Fatalf("encrypt api keys: %v", err)
	}
	upstream := anthropicGatewayUpstream()
	upstream.APIKeysEnc = apiKeysEnc
	// gpt-image-2 已被同步过：预览的同步计划也要为它推断协议，过去这一步同样会失败。
	repo := &modelUpdateRepo{upstream: *upstream, upstreamModels: map[string]domainchannel.UpstreamModel{
		"gpt-image-2": {
			ID: 1, UpstreamID: 9, BindingCode: "image-code", UpstreamModelName: "gpt-image-2",
			KindsJSON: inferKindsJSON("gpt-image-2"), Status: "active", Source: "sync",
		},
	}}
	service := newTestService(config.Config{DataEncryptionKey: encryptionKey}, repo, repo, nil, staticModelsGateway{items: anthropicGatewayCatalog()})

	data, err := service.ListRemoteModels(t.Context(), upstream.ID)
	if err != nil {
		t.Fatalf("list remote models: %v", err)
	}
	for _, item := range data.Items {
		if item.UpstreamModelName == "gpt-image-2" && (item.SuggestedProtocol != "" || len(item.SuggestedProtocols) != 0) {
			t.Fatalf("expected no suggested protocol for gpt-image-2, got %+v", item)
		}
	}
	if want := []string{"gpt-image-2", "nano-banana-pro", "veo-3"}; !reflect.DeepEqual(data.SyncPlan.UnresolvedProtocolModels, want) {
		t.Fatalf("unresolved protocol models = %v, want %v", data.SyncPlan.UnresolvedProtocolModels, want)
	}
}

// staticModelsGateway 返回固定的远端模型目录，用于不发网络请求的同步测试。
type staticModelsGateway struct {
	items []llm.ModelItem
}

func (g staticModelsGateway) Generate(context.Context, llm.RouteConfig, llm.GenerateInput) (*llm.GenerateOutput, error) {
	return nil, errors.New("not implemented")
}

func (g staticModelsGateway) ListModels(context.Context, llm.RouteConfig) ([]llm.ModelItem, error) {
	return g.items, nil
}

func TestBindingStillRequiresProtocolForModelsSyncedWithoutOne(t *testing.T) {
	upstream := anthropicGatewayUpstream()
	kindsJSON := inferKindsJSON("gpt-image-2")
	// 绑定路由时用同步记录的空建议协议推断，仍须得到 protocol_required：容错只发生在同步目录这一步。
	if _, err := resolveRouteProtocol("", upstream.Compatible, upstream.ProtocolDefaultsJSON, kindsJSON); !errors.Is(err, ErrProtocolRequired) {
		t.Fatalf("binding without a protocol: got %v, want ErrProtocolRequired", err)
	}
	if _, err := resolveRouteProtocols(nil, upstream.Compatible, upstream.ProtocolDefaultsJSON, kindsJSON); !errors.Is(err, ErrProtocolRequired) {
		t.Fatalf("importing without a protocol: got %v, want ErrProtocolRequired", err)
	}
	// 管理员手动指定协议后即可绑定。
	if protocol, err := resolveRouteProtocol("openai_image_generations", upstream.Compatible, upstream.ProtocolDefaultsJSON, kindsJSON); err != nil || protocol != "openai_image_generations" {
		t.Fatalf("binding with an explicit protocol: got %q, %v", protocol, err)
	}
}
