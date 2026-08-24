package settings

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

type settingsSeedRepo struct {
	items map[string]domainsettings.SystemSetting
}

func newSettingsSeedRepo(items ...domainsettings.SystemSetting) *settingsSeedRepo {
	repo := &settingsSeedRepo{items: map[string]domainsettings.SystemSetting{}}
	for _, item := range items {
		repo.items[item.Namespace+":"+item.Key] = item
	}
	return repo
}

func (r *settingsSeedRepo) ListAll(context.Context) ([]domainsettings.SystemSetting, error) {
	items := make([]domainsettings.SystemSetting, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, item)
	}
	return items, nil
}

func (r *settingsSeedRepo) ListByNamespace(_ context.Context, namespace string) ([]domainsettings.SystemSetting, error) {
	items := make([]domainsettings.SystemSetting, 0)
	for _, item := range r.items {
		if item.Namespace == namespace {
			items = append(items, item)
		}
	}
	return items, nil
}

func (r *settingsSeedRepo) Upsert(_ context.Context, items []domainsettings.SystemSetting) error {
	for _, item := range items {
		r.items[item.Namespace+":"+item.Key] = item
	}
	return nil
}

func (r *settingsSeedRepo) UpsertWithDescription(_ context.Context, items []domainsettings.SystemSetting) error {
	for _, item := range items {
		key := item.Namespace + ":" + item.Key
		if _, ok := r.items[key]; !ok {
			r.items[key] = item
		}
	}
	return nil
}

func (r *settingsSeedRepo) Delete(_ context.Context, namespace string, key string) error {
	delete(r.items, namespace+":"+key)
	return nil
}

func TestSeedMigratesLegacyDefaultAllowedMIMETypes(t *testing.T) {
	repo := newSettingsSeedRepo(domainsettings.SystemSetting{
		Namespace: "file",
		Key:       "allowed_mime_types",
		Value:     legacyDefaultAllowedMIMETypes,
		ValueType: "string",
	})
	service := NewService(repo, "")

	if err := service.Seed(context.Background(), config.Config{}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	got := repo.items["file:allowed_mime_types"].Value
	if got != defaultAllowedMIMETypes {
		t.Fatalf("expected legacy MIME defaults to migrate, got %q", got)
	}
}

func TestSeedKeepsCustomAllowedMIMETypes(t *testing.T) {
	custom := "image/png,text/plain"
	repo := newSettingsSeedRepo(domainsettings.SystemSetting{
		Namespace: "file",
		Key:       "allowed_mime_types",
		Value:     custom,
		ValueType: "string",
	})
	service := NewService(repo, "")

	if err := service.Seed(context.Background(), config.Config{}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	got := repo.items["file:allowed_mime_types"].Value
	if got != custom {
		t.Fatalf("expected custom MIME defaults to stay unchanged, got %q", got)
	}
}

func TestSeedUsesDefaultFullContextMaxBytesForMissingSetting(t *testing.T) {
	repo := newSettingsSeedRepo()
	service := NewService(repo, "")

	if err := service.Seed(context.Background(), config.Config{}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	want := strconv.FormatInt(config.DefaultFileFullContextMaxBytes, 10)
	if got := repo.items["file:file_full_context_max_bytes"].Value; got != want {
		t.Fatalf("expected default full-context size %q, got %q", want, got)
	}
}

func TestSeedReplacesLegacyCompactTokenThresholdWithModelAwareDefaults(t *testing.T) {
	repo := newSettingsSeedRepo(
		domainsettings.SystemSetting{
			Namespace: "chat",
			Key:       "context_compact_trigger_tokens",
			Value:     "65536",
			ValueType: "int",
		},
		domainsettings.SystemSetting{
			Namespace: "chat",
			Key:       "context_max_input_tokens",
			Value:     "32000",
			ValueType: "int",
		},
	)
	service := NewService(repo, "")

	if err := service.Seed(context.Background(), config.Config{}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	if _, exists := repo.items["chat:context_compact_trigger_tokens"]; exists {
		t.Fatal("expected obsolete fixed token threshold to be removed")
	}
	if _, exists := repo.items["chat:context_max_input_tokens"]; exists {
		t.Fatal("expected obsolete fixed input cap to be removed")
	}
	if got := repo.items["chat:context_window_fallback_tokens"].Value; got != strconv.Itoa(config.DefaultContextWindowFallbackTokens) {
		t.Fatalf("fallback window = %q, want %d", got, config.DefaultContextWindowFallbackTokens)
	}
	if got := repo.items["chat:context_compact_trigger_percent"].Value; got != strconv.Itoa(config.DefaultContextCompactTriggerPercent) {
		t.Fatalf("trigger percent = %q, want %d", got, config.DefaultContextCompactTriggerPercent)
	}
}

func TestSeedAddsMistralOCRDefaults(t *testing.T) {
	repo := newSettingsSeedRepo()
	service := NewService(repo, "test-data-encryption-key")

	if err := service.Seed(context.Background(), config.Config{}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	want := map[string]string{
		"extract:mistral_ocr_base_url":        "https://api.mistral.ai/v1/ocr",
		"extract:mistral_ocr_model":           "mistral-ocr-latest",
		"extract:mistral_ocr_timeout_seconds": "60",
	}
	for key, value := range want {
		if got := repo.items[key].Value; got != value {
			t.Fatalf("%s = %q, want %q", key, got, value)
		}
	}
	if got := repo.items["extract:mistral_ocr_auth_token"]; got.Value != "" {
		t.Fatalf("Mistral OCR auth token = %q, want empty", got.Value)
	}
}

func TestSeedKeepsExistingFullContextMaxBytes(t *testing.T) {
	const existingValue = "65536"
	repo := newSettingsSeedRepo(domainsettings.SystemSetting{
		Namespace: "file",
		Key:       "file_full_context_max_bytes",
		Value:     existingValue,
		ValueType: "int",
	})
	service := NewService(repo, "")

	if err := service.Seed(context.Background(), config.Config{}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	if got := repo.items["file:file_full_context_max_bytes"].Value; got != existingValue {
		t.Fatalf("expected existing full-context size to remain %q, got %q", existingValue, got)
	}
}

func TestSeedMigratesLegacyDefaultModelOptionAllowedPaths(t *testing.T) {
	legacy := map[string][]string{}
	if err := json.Unmarshal([]byte(config.DefaultModelOptionAllowedPathsJSON()), &legacy); err != nil {
		t.Fatalf("decode current model option defaults: %v", err)
	}
	delete(legacy, "xai_video")
	legacy["xai_responses"] = []string{"reasoning.effort"}
	legacyJSON, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("encode legacy model option defaults: %v", err)
	}
	repo := newSettingsSeedRepo(domainsettings.SystemSetting{
		Namespace: "chat",
		Key:       "model_option_allowed_paths",
		Value:     string(legacyJSON),
		ValueType: "json",
	})
	service := NewService(repo, "")

	if err := service.Seed(context.Background(), config.Config{}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	got := repo.items["chat:model_option_allowed_paths"].Value
	if got != config.DefaultModelOptionAllowedPathsJSON() {
		t.Fatalf("expected legacy model option defaults to migrate, got %q", got)
	}
}

func TestSeedAddsXAIVideoToPreviousDefaultModelOptionAllowedPaths(t *testing.T) {
	previousDefault := map[string][]string{}
	if err := json.Unmarshal([]byte(config.DefaultModelOptionAllowedPathsJSON()), &previousDefault); err != nil {
		t.Fatalf("decode current model option defaults: %v", err)
	}
	delete(previousDefault, "xai_video")
	previousJSON, err := json.Marshal(previousDefault)
	if err != nil {
		t.Fatalf("encode previous model option defaults: %v", err)
	}
	repo := newSettingsSeedRepo(domainsettings.SystemSetting{
		Namespace: "chat",
		Key:       "model_option_allowed_paths",
		Value:     string(previousJSON),
		ValueType: "json",
	})
	service := NewService(repo, "")

	if err := service.Seed(context.Background(), config.Config{}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	if got := repo.items["chat:model_option_allowed_paths"].Value; got != config.DefaultModelOptionAllowedPathsJSON() {
		t.Fatalf("expected xAI video defaults to be added, got %q", got)
	}
}

func TestSeedAddsXAIVideoExtensionsToPreviousDefaultModelOptionAllowedPaths(t *testing.T) {
	previousDefault := map[string][]string{}
	if err := json.Unmarshal([]byte(config.DefaultModelOptionAllowedPathsJSON()), &previousDefault); err != nil {
		t.Fatalf("decode current model option defaults: %v", err)
	}
	delete(previousDefault, "xai_video_extensions")
	previousJSON, err := json.Marshal(previousDefault)
	if err != nil {
		t.Fatalf("encode previous model option defaults: %v", err)
	}
	repo := newSettingsSeedRepo(domainsettings.SystemSetting{
		Namespace: "chat",
		Key:       "model_option_allowed_paths",
		Value:     string(previousJSON),
		ValueType: "json",
	})
	service := NewService(repo, "")

	if err := service.Seed(context.Background(), config.Config{}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	if got := repo.items["chat:model_option_allowed_paths"].Value; got != config.DefaultModelOptionAllowedPathsJSON() {
		t.Fatalf("expected xAI video extensions defaults to be added, got %q", got)
	}
}

func TestSeedAddsGeminiThinkingSummariesToPreviousDefaultModelOptionAllowedPaths(t *testing.T) {
	previousDefault := map[string][]string{}
	if err := json.Unmarshal([]byte(config.DefaultModelOptionAllowedPathsJSON()), &previousDefault); err != nil {
		t.Fatalf("decode current model option defaults: %v", err)
	}
	previousDefault["gemini_interactions"] = removeStringValue(
		previousDefault["gemini_interactions"],
		"generation_config.thinking_summaries",
	)
	previousJSON, err := json.Marshal(previousDefault)
	if err != nil {
		t.Fatalf("encode previous model option defaults: %v", err)
	}
	repo := newSettingsSeedRepo(domainsettings.SystemSetting{
		Namespace: "chat",
		Key:       "model_option_allowed_paths",
		Value:     string(previousJSON),
		ValueType: "json",
	})
	service := NewService(repo, "")

	if err := service.Seed(context.Background(), config.Config{}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	if got := repo.items["chat:model_option_allowed_paths"].Value; got != config.DefaultModelOptionAllowedPathsJSON() {
		t.Fatalf("expected Gemini thinking summaries default to be added, got %q", got)
	}
}

func TestSeedReplacesLegacyGeminiInteractionsOptionPaths(t *testing.T) {
	previousDefault := map[string][]string{}
	if err := json.Unmarshal([]byte(config.DefaultModelOptionAllowedPathsJSON()), &previousDefault); err != nil {
		t.Fatalf("decode current model option defaults: %v", err)
	}
	previousDefault["gemini_interactions"] = append(
		removeStringValue(previousDefault["gemini_interactions"], "response_format.schema"),
		"responseFormat.type",
		"responseFormat.aspectRatio",
		"responseFormat.imageSize",
		"responseFormat.mimeType",
		"generationConfig.videoConfig.task",
	)
	previousJSON, err := json.Marshal(previousDefault)
	if err != nil {
		t.Fatalf("encode previous model option defaults: %v", err)
	}
	repo := newSettingsSeedRepo(domainsettings.SystemSetting{
		Namespace: "chat",
		Key:       "model_option_allowed_paths",
		Value:     string(previousJSON),
		ValueType: "json",
	})
	service := NewService(repo, "")

	if err := service.Seed(context.Background(), config.Config{}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	if got := repo.items["chat:model_option_allowed_paths"].Value; got != config.DefaultModelOptionAllowedPathsJSON() {
		t.Fatalf("expected legacy Gemini Interactions paths to migrate, got %q", got)
	}
}

func TestSeedAddsGeminiGenerateContentThinkingPathsToPreviousDefaultModelOptionAllowedPaths(t *testing.T) {
	previousDefault := map[string][]string{}
	if err := json.Unmarshal([]byte(config.DefaultModelOptionAllowedPathsJSON()), &previousDefault); err != nil {
		t.Fatalf("decode current model option defaults: %v", err)
	}
	previousDefault["gemini_generate_content"] = removeStringValue(
		removeStringValue(
			previousDefault["gemini_generate_content"],
			"generationConfig.thinkingConfig.includeThoughts",
		),
		"generationConfig.thinkingConfig.thinkingLevel",
	)
	previousJSON, err := json.Marshal(previousDefault)
	if err != nil {
		t.Fatalf("encode previous model option defaults: %v", err)
	}
	repo := newSettingsSeedRepo(domainsettings.SystemSetting{
		Namespace: "chat",
		Key:       "model_option_allowed_paths",
		Value:     string(previousJSON),
		ValueType: "json",
	})
	service := NewService(repo, "")

	if err := service.Seed(context.Background(), config.Config{}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	if got := repo.items["chat:model_option_allowed_paths"].Value; got != config.DefaultModelOptionAllowedPathsJSON() {
		t.Fatalf("expected Gemini Generate Content thinking defaults to be added, got %q", got)
	}
}

func TestSeedKeepsCustomModelOptionAllowedPaths(t *testing.T) {
	custom := `{"default":["temperature"],"xai_responses":["reasoning.effort"]}`
	repo := newSettingsSeedRepo(domainsettings.SystemSetting{
		Namespace: "chat",
		Key:       "model_option_allowed_paths",
		Value:     custom,
		ValueType: "json",
	})
	service := NewService(repo, "")

	if err := service.Seed(context.Background(), config.Config{}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	got := repo.items["chat:model_option_allowed_paths"].Value
	if got != custom {
		t.Fatalf("expected custom model option defaults to stay unchanged, got %q", got)
	}
}
