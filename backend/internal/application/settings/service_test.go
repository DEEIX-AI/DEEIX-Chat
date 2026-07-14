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

type testVectorStore struct {
	available bool
	err       error
}

func (s testVectorStore) VectorStoreAvailable(context.Context) (bool, error) {
	return s.available, s.err
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

func TestValidateEmbeddingDependentSettingsRejectsEmbeddingWithoutVectorStore(t *testing.T) {
	repo := &testSettingsRepo{byNamespace: map[string][]domainsettings.SystemSetting{
		"chat": {
			{Namespace: "chat", Key: "rag_enabled", Value: "false"},
			{Namespace: "chat", Key: "message_embedding_enabled", Value: "false"},
			{Namespace: "chat", Key: "semantic_context_enabled", Value: "false"},
		},
		"file": {
			{Namespace: "file", Key: "embedding_enabled", Value: "false"},
			{Namespace: "file", Key: "embedding_host", Value: "https://embedding.example.com"},
			{Namespace: "file", Key: "rag_model", Value: "embed-model"},
		},
	}}
	service := NewService(repo, "test-data-encryption-key")
	service.SetVectorStoreAvailabilityService(testVectorStore{available: false})

	err := service.validateEmbeddingDependentSettings(context.Background(), []PatchItem{
		{Namespace: "file", Key: "embedding_enabled", Value: "true"},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateEmbeddingDependentSettingsAllowsEmbeddingWithVectorStore(t *testing.T) {
	repo := &testSettingsRepo{byNamespace: map[string][]domainsettings.SystemSetting{
		"chat": {
			{Namespace: "chat", Key: "rag_enabled", Value: "false"},
			{Namespace: "chat", Key: "message_embedding_enabled", Value: "false"},
			{Namespace: "chat", Key: "semantic_context_enabled", Value: "false"},
		},
		"file": {
			{Namespace: "file", Key: "embedding_enabled", Value: "false"},
			{Namespace: "file", Key: "embedding_host", Value: "https://embedding.example.com"},
			{Namespace: "file", Key: "rag_model", Value: "embed-model"},
		},
	}}
	service := NewService(repo, "test-data-encryption-key")
	service.SetVectorStoreAvailabilityService(testVectorStore{available: true})

	err := service.validateEmbeddingDependentSettings(context.Background(), []PatchItem{
		{Namespace: "file", Key: "embedding_enabled", Value: "true"},
	})
	if err != nil {
		t.Fatalf("expected validation to pass, got %v", err)
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

func TestValidateTurnstileRegistrationSettings(t *testing.T) {
	repo := &testSettingsRepo{byNamespace: map[string][]domainsettings.SystemSetting{
		"auth": {
			{Namespace: "auth", Key: "email_login_enabled", Value: "true"},
			{Namespace: "auth", Key: "email_registration_enabled", Value: "true"},
			{Namespace: "auth", Key: "turnstile_registration_enabled", Value: "false"},
			{Namespace: "auth", Key: "turnstile_site_key", Value: ""},
			{Namespace: "auth", Key: "turnstile_secret_key", Value: ""},
		},
	}}
	service := NewService(repo, "test-data-encryption-key")

	if _, err := service.applyAuthSettingDependencies(context.Background(), []PatchItem{
		{Namespace: "auth", Key: "turnstile_registration_enabled", Value: "true"},
	}); err == nil {
		t.Fatal("expected missing turnstile keys to fail")
	}

	if _, err := service.applyAuthSettingDependencies(context.Background(), []PatchItem{
		{Namespace: "auth", Key: "turnstile_registration_enabled", Value: "true"},
		{Namespace: "auth", Key: "turnstile_site_key", Value: "site-key"},
		{Namespace: "auth", Key: "turnstile_secret_key", Value: "secret-key"},
	}); err != nil {
		t.Fatalf("expected complete turnstile settings to pass, got %v", err)
	}
}

func TestValidateTurnstileRegistrationEnabledRequiresBool(t *testing.T) {
	if err := validatePatchItem(PatchItem{Namespace: "auth", Key: "turnstile_registration_enabled", Value: "enabled"}); err == nil {
		t.Fatal("expected turnstile registration switch to reject non-bool value")
	}
}

func TestValidatePasswordResetRequiresEmailVerification(t *testing.T) {
	repo := &testSettingsRepo{byNamespace: map[string][]domainsettings.SystemSetting{
		"auth": {
			{Namespace: "auth", Key: "username_login_enabled", Value: "true"},
			{Namespace: "auth", Key: "email_login_enabled", Value: "true"},
			{Namespace: "auth", Key: "third_party_login_enabled", Value: "true"},
			{Namespace: "auth", Key: "email_verification_enabled", Value: "false"},
			{Namespace: "auth", Key: "password_reset_enabled", Value: "false"},
		},
	}}
	service := NewService(repo, "test-data-encryption-key")

	if _, err := service.applyAuthSettingDependencies(context.Background(), []PatchItem{
		{Namespace: "auth", Key: "password_reset_enabled", Value: "true"},
	}); err == nil {
		t.Fatal("expected password reset to require email verification")
	}
}

func TestRuntimeSettingsNormalizeConfigDisablesPasswordReset(t *testing.T) {
	runtimeSettings := NewRuntimeSettings(nil, nil, "test-data-encryption-key")
	cfg := config.Config{
		UsernameLoginEnabled:     true,
		EmailLoginEnabled:        true,
		EmailVerificationEnabled: false,
		PasswordResetEnabled:     true,
	}

	runtimeSettings.normalizeConfig(&cfg)

	if cfg.PasswordResetEnabled {
		t.Fatal("expected password reset disabled when email verification is disabled")
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
	if err := validatePatchItem(PatchItem{Namespace: "billing", Key: "native_tool_pricing_json", Value: `{"xai.web_search":{"priceNanousd":1000000,"unit":"call","priceLabel":"","billable":true}}`}); err != nil {
		t.Fatalf("expected native tool pricing JSON to pass, got %v", err)
	}
	if err := validatePatchItem(PatchItem{Namespace: "billing", Key: "native_tool_pricing_json", Value: `{"unknownTool":{"priceNanousd":1000000,"unit":"call","priceLabel":"","billable":true}}`}); err == nil {
		t.Fatal("expected unsupported native tool pricing key to fail")
	}
}

func TestValidateBillingDisplayCurrencySetting(t *testing.T) {
	for _, currency := range []string{"USD", "CNY"} {
		if err := validatePatchItem(PatchItem{Namespace: "billing", Key: "display_currency", Value: currency}); err != nil {
			t.Fatalf("expected %s to pass, got %v", currency, err)
		}
	}
	if err := validatePatchItem(PatchItem{Namespace: "billing", Key: "display_currency", Value: "EUR"}); err == nil {
		t.Fatal("expected unsupported display currency to fail")
	}
}

func TestValidateMCPSelectedToolsSetting(t *testing.T) {
	if err := validatePatchItem(PatchItem{Namespace: "mcp", Key: "mcp_max_selected_tools_per_message", Value: "32"}); err != nil {
		t.Fatalf("expected selected tool limit to pass, got %v", err)
	}
	if err := validatePatchItem(PatchItem{Namespace: "mcp", Key: "mcp_max_selected_tools_per_message", Value: "0"}); err == nil {
		t.Fatal("expected zero selected tool limit to fail")
	}
	if err := validatePatchItem(PatchItem{Namespace: "mcp", Key: "mcp_max_selected_tools_per_message", Value: "129"}); err == nil {
		t.Fatal("expected selected tool limit above safe maximum to fail")
	}
}

func TestValidateCustomPromptSettings(t *testing.T) {
	for _, item := range []PatchItem{
		{Namespace: "mcp", Key: "mcp_tool_prompt", Value: "Use MCP tools carefully."},
		{Namespace: "chat", Key: "skills_prompt", Value: "Use selected skills only when relevant."},
	} {
		if err := validatePatchItem(item); err != nil {
			t.Fatalf("expected %s:%s to pass, got %v", item.Namespace, item.Key, err)
		}
	}
}

func TestRuntimeSettingsAppliesCustomPromptSettings(t *testing.T) {
	runtimeSettings := NewRuntimeSettings(nil, nil, "test-data-encryption-key")
	cfg := config.Config{}

	runtimeSettings.applyItem(&cfg, domainsettings.SystemSetting{Namespace: "mcp", Key: "mcp_tool_prompt", Value: "Use MCP tools carefully."})
	runtimeSettings.applyItem(&cfg, domainsettings.SystemSetting{Namespace: "chat", Key: "skills_prompt", Value: "Use selected skills only when relevant."})

	if cfg.MCPToolPrompt != "Use MCP tools carefully." {
		t.Fatalf("expected MCP tool prompt to be applied, got %q", cfg.MCPToolPrompt)
	}
	if cfg.SkillsPrompt != "Use selected skills only when relevant." {
		t.Fatalf("expected skills prompt to be applied, got %q", cfg.SkillsPrompt)
	}
}

func TestRuntimeSettingsAppliesConversationDefaultModel(t *testing.T) {
	runtimeSettings := NewRuntimeSettings(nil, nil, "test-data-encryption-key")
	cfg := config.Config{}

	runtimeSettings.applyItem(&cfg, domainsettings.SystemSetting{Namespace: "chat", Key: "conversation_default_model", Value: " gpt-5-mini "})

	if cfg.ConversationDefaultModel != "gpt-5-mini" {
		t.Fatalf("expected conversation default model to be applied, got %q", cfg.ConversationDefaultModel)
	}
}

func TestValidateMinerUFileTypesSetting(t *testing.T) {
	if err := validatePatchItem(PatchItem{Namespace: "extract", Key: "mineru_file_types", Value: "pdf,word,presentation,excel"}); err != nil {
		t.Fatalf("expected mineru file types to pass, got %v", err)
	}
	if err := validatePatchItem(PatchItem{Namespace: "extract", Key: "mineru_file_types", Value: "pdf,slides"}); err == nil {
		t.Fatal("expected unsupported mineru file type to fail")
	}
}

func TestRuntimeSettingsAppliesMinerUFileTypes(t *testing.T) {
	runtimeSettings := NewRuntimeSettings(nil, nil, "test-data-encryption-key")
	cfg := config.Config{ExtractMinerUFileTypes: "pdf,word,presentation"}

	runtimeSettings.applyItem(&cfg, domainsettings.SystemSetting{Namespace: "extract", Key: "mineru_file_types", Value: "pdf,excel"})

	if cfg.ExtractMinerUFileTypes != "pdf,excel" {
		t.Fatalf("expected mineru file types to apply, got %q", cfg.ExtractMinerUFileTypes)
	}
}

func TestValidateFullContextLimitsAllowUnlimitedValues(t *testing.T) {
	cases := []PatchItem{
		{Namespace: "file", Key: "full_context_limit_enabled", Value: "true"},
		{Namespace: "file", Key: "full_context_limit_enabled", Value: "false"},
		{Namespace: "file", Key: "file_full_context_max_bytes", Value: ""},
		{Namespace: "file", Key: "file_full_context_max_bytes", Value: "0"},
		{Namespace: "file", Key: "full_context_max_tokens", Value: ""},
		{Namespace: "file", Key: "full_context_max_tokens", Value: "0"},
		{Namespace: "file", Key: "full_context_pdf_max_pages", Value: ""},
		{Namespace: "file", Key: "full_context_pdf_max_pages", Value: "0"},
	}

	for _, item := range cases {
		if err := validatePatchItem(item); err != nil {
			t.Fatalf("expected %s:%s=%q to pass, got %v", item.Namespace, item.Key, item.Value, err)
		}
	}
}

func TestValidateFullContextLimitsEnforcesConfiguredRanges(t *testing.T) {
	cases := []struct {
		name string
		item PatchItem
		want bool
	}{
		{
			name: "full context limit mode must be boolean",
			item: PatchItem{Namespace: "file", Key: "full_context_limit_enabled", Value: "disabled"},
			want: false,
		},
		{
			name: "1M token limit passes",
			item: PatchItem{Namespace: "file", Key: "full_context_max_tokens", Value: "1000000"},
			want: true,
		},
		{
			name: "token limit below minimum fails",
			item: PatchItem{Namespace: "file", Key: "full_context_max_tokens", Value: "127"},
			want: false,
		},
		{
			name: "token limit above maximum fails",
			item: PatchItem{Namespace: "file", Key: "full_context_max_tokens", Value: "1000001"},
			want: false,
		},
		{
			name: "negative byte limit fails",
			item: PatchItem{Namespace: "file", Key: "file_full_context_max_bytes", Value: "-1"},
			want: false,
		},
		{
			name: "pdf page limit above maximum fails",
			item: PatchItem{Namespace: "file", Key: "full_context_pdf_max_pages", Value: "501"},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePatchItem(tc.item)
			if tc.want && err != nil {
				t.Fatalf("expected validation to pass, got %v", err)
			}
			if !tc.want && err == nil {
				t.Fatal("expected validation to fail")
			}
		})
	}
}

func TestRuntimeSettingsDisablesFullContextLimits(t *testing.T) {
	runtimeSettings := NewRuntimeSettings(nil, nil, "test-data-encryption-key")
	cfg := config.Config{
		FileFullContextLimitEnabled: true,
		FileFullContextMaxBytes:     65536,
		FileFullContextMaxTokens:    65536,
		FileFullContextPDFMaxPages:  20,
	}

	runtimeSettings.applyItem(&cfg, domainsettings.SystemSetting{Namespace: "file", Key: "full_context_limit_enabled", Value: "false"})
	runtimeSettings.normalizeConfig(&cfg)

	if cfg.FileFullContextLimitEnabled {
		t.Fatal("expected full context limit switch to be disabled")
	}
	if cfg.FileFullContextMaxBytes != 0 || cfg.FileFullContextMaxTokens != 0 || cfg.FileFullContextPDFMaxPages != 0 {
		t.Fatalf(
			"expected disabled full context limits to be unlimited, got bytes=%d tokens=%d pdfPages=%d",
			cfg.FileFullContextMaxBytes,
			cfg.FileFullContextMaxTokens,
			cfg.FileFullContextPDFMaxPages,
		)
	}
}

func TestRuntimeSettingsTreatsEmptyFullContextLimitsAsUnlimited(t *testing.T) {
	runtimeSettings := NewRuntimeSettings(nil, nil, "test-data-encryption-key")
	cfg := config.Config{
		FileFullContextLimitEnabled: true,
		FileFullContextMaxBytes:     65536,
		FileFullContextMaxTokens:    65536,
		FileFullContextPDFMaxPages:  20,
	}

	runtimeSettings.applyItem(&cfg, domainsettings.SystemSetting{Namespace: "file", Key: "file_full_context_max_bytes", Value: ""})
	runtimeSettings.applyItem(&cfg, domainsettings.SystemSetting{Namespace: "file", Key: "full_context_max_tokens", Value: ""})
	runtimeSettings.applyItem(&cfg, domainsettings.SystemSetting{Namespace: "file", Key: "full_context_pdf_max_pages", Value: ""})

	if cfg.FileFullContextMaxBytes != 0 || cfg.FileFullContextMaxTokens != 0 || cfg.FileFullContextPDFMaxPages != 0 {
		t.Fatalf(
			"expected empty full context limits to be unlimited, got bytes=%d tokens=%d pdfPages=%d",
			cfg.FileFullContextMaxBytes,
			cfg.FileFullContextMaxTokens,
			cfg.FileFullContextPDFMaxPages,
		)
	}
}
