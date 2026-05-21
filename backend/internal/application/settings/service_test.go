package settings

import (
	"context"
	"testing"

	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

type testSettingsRepo struct {
	byNamespace map[string][]domainsettings.SystemSetting
}

func (r *testSettingsRepo) ListAll(ctx context.Context) ([]domainsettings.SystemSetting, error) {
	var result []domainsettings.SystemSetting
	for _, items := range r.byNamespace {
		result = append(result, items...)
	}
	return result, nil
}

func (r *testSettingsRepo) ListByNamespace(ctx context.Context, namespace string) ([]domainsettings.SystemSetting, error) {
	return r.byNamespace[namespace], nil
}

func (r *testSettingsRepo) Upsert(ctx context.Context, items []domainsettings.SystemSetting) error {
	return nil
}

func (r *testSettingsRepo) UpsertWithDescription(ctx context.Context, items []domainsettings.SystemSetting) error {
	return nil
}

func (r *testSettingsRepo) Delete(ctx context.Context, namespace, key string) error {
	return nil
}

func TestApplyEmbeddingDependentCascadesDisablesRAGAndSemanticFeatures(t *testing.T) {
	repo := &testSettingsRepo{byNamespace: map[string][]domainsettings.SystemSetting{
		"chat": {
			{Key: "rag_enabled", Value: "true"},
			{Key: "message_embedding_enabled", Value: "true"},
			{Key: "semantic_context_enabled", Value: "true"},
		},
		"file": {
			{Key: "embedding_enabled", Value: "true"},
			{Key: "embedding_host", Value: "http://127.0.0.1:8001/v1"},
			{Key: "rag_model", Value: "embed-model"},
		},
	}}
	service := NewService(repo, "test-data-encryption-key")

	patches, err := service.applyEmbeddingDependentCascades(context.Background(), []PatchItem{
		{Namespace: "file", Key: "embedding_host", Value: ""},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := map[string]string{
		"chat:rag_enabled":               "false",
		"chat:message_embedding_enabled": "false",
		"chat:semantic_context_enabled":  "false",
	}
	got := make(map[string]string)
	for _, item := range patches {
		got[item.Namespace+":"+item.Key] = item.Value
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("expected %s=%s, got %q in %#v", key, value, got[key], patches)
		}
	}
}

func TestValidateEmbeddingDependentSettingsRejectsRAGWithoutEmbedding(t *testing.T) {
	repo := &testSettingsRepo{byNamespace: map[string][]domainsettings.SystemSetting{
		"chat": {
			{Key: "message_embedding_enabled", Value: "false"},
			{Key: "semantic_context_enabled", Value: "false"},
		},
		"file": {
			{Key: "embedding_enabled", Value: "false"},
			{Key: "embedding_host", Value: ""},
			{Key: "rag_model", Value: "embed-model"},
		},
	}}
	service := NewService(repo, "test-data-encryption-key")

	err := service.validateEmbeddingDependentSettings(context.Background(), []PatchItem{
		{Namespace: "chat", Key: "rag_enabled", Value: "true"},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestRuntimeSettingsNormalizeConfigDisablesEmbeddingDependentFeatures(t *testing.T) {
	runtimeSettings := NewRuntimeSettings(nil, nil, "test-data-encryption-key")
	cfg := config.Config{
		EmbeddingEnabled:        false,
		EmbeddingHost:           "",
		RAGModel:                "embed-model",
		RAGEnabled:              true,
		MessageEmbeddingEnabled: true,
		SemanticContextEnabled:  true,
	}

	runtimeSettings.normalizeConfig(&cfg)

	if cfg.RAGEnabled || cfg.MessageEmbeddingEnabled || cfg.SemanticContextEnabled {
		t.Fatalf("expected embedding dependent features disabled, got rag=%v message=%v semantic=%v", cfg.RAGEnabled, cfg.MessageEmbeddingEnabled, cfg.SemanticContextEnabled)
	}
}

func TestValidateModelOptionPolicySettings(t *testing.T) {
	if err := validatePatchItem(PatchItem{Namespace: "chat", Key: "model_option_policy_mode", Value: "allowlist"}); err != nil {
		t.Fatalf("expected allowlist mode to pass, got %v", err)
	}
	if err := validatePatchItem(PatchItem{Namespace: "chat", Key: "model_option_allowed_paths", Value: config.DefaultModelOptionAllowedPathsJSON()}); err != nil {
		t.Fatalf("expected default allow paths to pass, got %v", err)
	}
	if err := validatePatchItem(PatchItem{Namespace: "chat", Key: "model_option_allowed_paths", Value: `{"unknown":["temperature"]}`}); err == nil {
		t.Fatal("expected unsupported protocol key to fail")
	}
	if err := validatePatchItem(PatchItem{Namespace: "chat", Key: "model_option_allowed_paths", Value: `{"default":["bad path"]}`}); err == nil {
		t.Fatal("expected whitespace path to fail")
	}
}

func TestValidateFullContextMaxTokensAllowsLargeContextWindows(t *testing.T) {
	if err := validatePatchItem(PatchItem{Namespace: "file", Key: "full_context_max_tokens", Value: "1000000"}); err != nil {
		t.Fatalf("expected 1M full context token limit to pass, got %v", err)
	}
	if err := validatePatchItem(PatchItem{Namespace: "file", Key: "full_context_max_tokens", Value: "1000001"}); err == nil {
		t.Fatal("expected full context token limit above 1M to fail")
	}
}
