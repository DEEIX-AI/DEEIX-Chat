package settings

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

func TestSettingSpecsAreUniqueAndWellFormed(t *testing.T) {
	seen := make(map[string]struct{}, len(settingSpecs))
	for _, spec := range settingSpecs {
		key := spec.fullKey()
		if _, dup := seen[key]; dup {
			t.Fatalf("duplicate setting spec %s", key)
		}
		seen[key] = struct{}{}
		if strings.TrimSpace(spec.Namespace) == "" || strings.TrimSpace(spec.Key) == "" {
			t.Fatalf("setting spec %q has empty namespace or key", key)
		}
		if strings.TrimSpace(spec.Description) == "" {
			t.Fatalf("setting spec %s has no description", key)
		}
		switch spec.ValueType {
		case "int", "bool", "string", "json":
		default:
			t.Fatalf("setting spec %s has unknown value type %q", key, spec.ValueType)
		}
		if spec.Sensitive && spec.ValueType != "string" {
			t.Fatalf("sensitive setting %s must be a string, got %q", key, spec.ValueType)
		}
	}
}

func TestSettingSpecDefaultsPassTheirOwnValidation(t *testing.T) {
	for _, spec := range settingSpecs {
		if err := validateSettingValue(spec.Namespace, spec.Key, spec.Default); err != nil {
			t.Fatalf("default value of %s rejected by its validator: %v", spec.fullKey(), err)
		}
	}
}

func TestTypedSettingSpecsRejectMalformedValues(t *testing.T) {
	for _, spec := range settingSpecs {
		switch spec.ValueType {
		case "bool":
			if spec.Validate == nil {
				t.Fatalf("bool setting %s has no validator", spec.fullKey())
			}
			if err := spec.Validate("enabled", spec.fullKey()); err == nil {
				t.Fatalf("bool setting %s accepted non-boolean value", spec.fullKey())
			}
			for _, value := range []string{"true", "false"} {
				if err := spec.Validate(value, spec.fullKey()); err != nil {
					t.Fatalf("bool setting %s rejected %q: %v", spec.fullKey(), value, err)
				}
			}
		case "int":
			if spec.Validate == nil {
				t.Fatalf("int setting %s has no validator", spec.fullKey())
			}
			if err := spec.Validate("abc", spec.fullKey()); err == nil {
				t.Fatalf("int setting %s accepted non-integer value", spec.fullKey())
			}
		}
	}
}

// 每个带 Apply 的配置项必须恰好写入一个运行时字段，且不同配置项不能写同一个字段。
func TestSettingSpecApplyTargetsDistinctRuntimeFields(t *testing.T) {
	var runtimeSettings RuntimeSettings
	owners := map[string]string{}
	for _, spec := range settingSpecs {
		if spec.Apply == nil {
			continue
		}
		var cfg config.Config
		item := spec.seedSetting()
		item.Value = nonZeroSampleValue(spec)
		runtimeSettings.applyItem(&cfg, item)

		changed := changedConfigFields(cfg)
		if len(changed) != 1 {
			t.Fatalf("applying %s changed fields %v, want exactly one", spec.fullKey(), changed)
		}
		if owner, taken := owners[changed[0]]; taken {
			t.Fatalf("%s and %s both write config.%s", owner, spec.fullKey(), changed[0])
		}
		owners[changed[0]] = spec.fullKey()
	}
}

// nonZeroSampleValue 优先使用注册表默认值（它一定能被该项的解析器接受），零值默认则退回按类型构造的样例。
func nonZeroSampleValue(spec settingSpec) string {
	switch strings.TrimSpace(spec.Default) {
	case "", "0", "false":
	default:
		return spec.Default
	}
	switch spec.ValueType {
	case "int":
		return "7"
	case "bool":
		return "true"
	case "json":
		return `{"sample":true}`
	default:
		return "sample"
	}
}

func changedConfigFields(cfg config.Config) []string {
	zero := reflect.ValueOf(config.Config{})
	value := reflect.ValueOf(cfg)
	var changed []string
	for i := 0; i < value.NumField(); i++ {
		if !reflect.DeepEqual(value.Field(i).Interface(), zero.Field(i).Interface()) {
			changed = append(changed, value.Type().Field(i).Name)
		}
	}
	return changed
}

func TestSettingRegistryDerivedViews(t *testing.T) {
	for _, namespace := range []string{"auth", "billing", "chat", "storage", "file", "extract", "mcp", "circuit", "knowledgebase"} {
		if !IsValidNamespace(namespace) {
			t.Fatalf("expected namespace %q to be valid", namespace)
		}
	}
	if IsValidNamespace("content_moderation") {
		t.Fatalf("content_moderation is managed by its own module and must not be a settings namespace")
	}

	if !IsSensitiveSetting("auth", "smtp_password") || !IsSensitiveSetting(" file ", " embedding_key ") {
		t.Fatalf("expected credential settings to be sensitive")
	}
	if IsSensitiveSetting("auth", "turnstile_site_key") || IsSensitiveSetting("billing", "stripe_publishable_key") {
		t.Fatalf("public keys must not be classified as sensitive")
	}
	if IsSensitiveSetting("auth", "unknown_secret") {
		t.Fatalf("unregistered keys must not be classified as sensitive")
	}

	seeded := defaultSettings()
	if len(seeded) != len(settingSpecs) {
		t.Fatalf("expected %d seeded settings, got %d", len(settingSpecs), len(seeded))
	}
	for i, item := range seeded {
		spec := settingSpecs[i]
		want := domainsettings.SystemSetting{Namespace: spec.Namespace, Key: spec.Key, Value: spec.Default, ValueType: spec.ValueType, Description: spec.Description}
		if item != want {
			t.Fatalf("seed item %d mismatch: got %+v want %+v", i, item, want)
		}
	}
}

func TestValidatePatchItemUsesRegistry(t *testing.T) {
	cases := []struct {
		name     string
		item     PatchItem
		wantCode string
		wantRule string
	}{
		{name: "unknown key", item: PatchItem{Namespace: "auth", Key: "nope", Value: "1"}, wantCode: settingCodeInvalidKey, wantRule: "invalid_key"},
		{name: "clear non-sensitive", item: PatchItem{Namespace: "auth", Key: "smtp_host", Clear: true}, wantCode: settingCodeSMTPInvalid, wantRule: "clear_not_allowed"},
		{name: "clear sensitive", item: PatchItem{Namespace: "auth", Key: "smtp_password", Clear: true}},
		{name: "int out of range", item: PatchItem{Namespace: "auth", Key: "token_ttl_hours", Value: "0"}, wantCode: settingCodeInvalidValue, wantRule: "integer_range"},
		{name: "int trimmed", item: PatchItem{Namespace: "auth", Key: "token_ttl_hours", Value: " 24 "}},
		{name: "bool garbage", item: PatchItem{Namespace: "chat", Key: "context_compact_enabled", Value: "yes"}, wantCode: settingCodeInvalidValue, wantRule: "bool"},
		{name: "enum", item: PatchItem{Namespace: "billing", Key: "mode", Value: "free"}, wantCode: settingCodeInvalidValue, wantRule: "enum"},
		{name: "optional empty int64", item: PatchItem{Namespace: "file", Key: "image_max_bytes", Value: ""}},
		{name: "optional int64 below min", item: PatchItem{Namespace: "file", Key: "image_max_bytes", Value: "0"}, wantCode: settingCodeInvalidValue, wantRule: "optional_integer_min"},
		{name: "epay gateway", item: PatchItem{Namespace: "billing", Key: "epay_gateway_url", Value: "ftp://pay.example.com"}, wantCode: settingCodeBillingPaymentInvalid, wantRule: "epay_url"},
		{name: "login path", item: PatchItem{Namespace: "auth", Key: "login_default_next_path", Value: "//evil"}, wantCode: settingCodeInvalidValue, wantRule: "local_path"},
		{name: "unvalidated text", item: PatchItem{Namespace: "chat", Key: "compact_system_prompt", Value: strings.Repeat("x", 50000)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePatchItem(tc.item)
			if tc.wantCode == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected validation error")
			}
			var validationErr *SettingValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("expected SettingValidationError, got %T", err)
			}
			if validationErr.Code() != tc.wantCode || validationErr.Details().Rule != tc.wantRule {
				t.Fatalf("unexpected validation error: code=%q details=%+v", validationErr.Code(), validationErr.Details())
			}
		})
	}
}
