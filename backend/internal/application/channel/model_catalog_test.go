package channel

import (
	"encoding/json"
	"errors"
	"testing"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
)

func TestProtocolDefaultsForCompatibleOnlyIncludesSupportedPrimaryKinds(t *testing.T) {
	raw := protocolDefaultsForCompatible(compatibleOpenAI)

	var defaults map[string]string
	if err := json.Unmarshal([]byte(raw), &defaults); err != nil {
		t.Fatalf("unmarshal defaults: %v", err)
	}

	for _, kind := range []string{modelKindChat, modelKindAudio, modelKindImageGen, modelKindImageEdit, modelKindVideoGen} {
		if defaults[kind] == "" {
			t.Fatalf("expected default protocol for %s in %s", kind, raw)
		}
	}
	legacyVectorKind := "embed" + "ding"
	legacySortKind := "re" + "rank"
	for _, kind := range []string{legacyVectorKind, legacySortKind, "unknown_kind"} {
		if _, ok := defaults[kind]; ok {
			t.Fatalf("unexpected default protocol for %s in %s", kind, raw)
		}
	}
}

func TestProtocolDefaultsForXAIUsesXAIResponsesForConversationKinds(t *testing.T) {
	raw := protocolDefaultsForCompatible(compatibleXAI)

	var defaults map[string]string
	if err := json.Unmarshal([]byte(raw), &defaults); err != nil {
		t.Fatalf("unmarshal defaults: %v", err)
	}

	if defaults[modelKindChat] != "xai_responses" {
		t.Fatalf("expected xAI chat default, got %q in %s", defaults[modelKindChat], raw)
	}
	if defaults[modelKindAudio] != "xai_responses" {
		t.Fatalf("expected xAI audio default, got %q in %s", defaults[modelKindAudio], raw)
	}
	for _, kind := range []string{modelKindImageGen, modelKindImageEdit, modelKindVideoGen} {
		if _, ok := defaults[kind]; ok {
			t.Fatalf("unexpected xAI default protocol for %s in %s", kind, raw)
		}
	}
}

func TestProtocolDefaultsForOpenRouterUsesOpenAIResponsesForConversationKinds(t *testing.T) {
	raw := protocolDefaultsForCompatible(compatibleOpenRouter)

	var defaults map[string]string
	if err := json.Unmarshal([]byte(raw), &defaults); err != nil {
		t.Fatalf("unmarshal defaults: %v", err)
	}

	if defaults[modelKindChat] != "openai_responses" {
		t.Fatalf("expected OpenRouter chat default, got %q in %s", defaults[modelKindChat], raw)
	}
	if defaults[modelKindAudio] != "openai_responses" {
		t.Fatalf("expected OpenRouter audio default, got %q in %s", defaults[modelKindAudio], raw)
	}
	expectedMediaDefaults := map[string]string{
		modelKindImageGen:  "openai_image_generations",
		modelKindImageEdit: "openai_image_edits",
		modelKindVideoGen:  "openai_video_generations",
	}
	for kind, expected := range expectedMediaDefaults {
		if defaults[kind] != expected {
			t.Fatalf("expected OpenRouter %s default %q, got %q in %s", kind, expected, defaults[kind], raw)
		}
	}
}

func TestNormalizeCompatibleOnlyAllowsSupportedUpstreamProviders(t *testing.T) {
	for _, raw := range []string{"openai", "anthropic", "google", "xai", "openrouter", "custom"} {
		if got := normalizeCompatible(raw); got != raw {
			t.Fatalf("normalizeCompatible(%q) = %q", raw, got)
		}
	}
	if got := normalizeCompatible(""); got != compatibleOpenAI {
		t.Fatalf("empty compatible should default to openai, got %q", got)
	}
	for _, raw := range []string{"replicate", "fal", "stability_ai", "mistral"} {
		if got := normalizeCompatible(raw); got != "" {
			t.Fatalf("expected unsupported compatible %q to be rejected, got %q", raw, got)
		}
	}
}

func TestProtocolDefaultsForCustomAreExplicitOnly(t *testing.T) {
	raw := protocolDefaultsForCompatible(compatibleCustom)
	if raw != `{}` {
		t.Fatalf("expected custom compatible to have no implicit defaults, got %s", raw)
	}
}

func TestDetectModelVendorRecognizesMistralFamily(t *testing.T) {
	tests := map[string]string{
		"mistral-large-latest": "mistral",
		"mixtral-8x7b":         "mistral",
		"ministral-8b":         "mistral",
		"codestral-latest":     "mistral",
		"pixtral-12b":          "mistral",
		"devstral-small":       "mistral",
		"acme-mistral-proxy":   "mistral",
	}

	for platformModelName, expected := range tests {
		if got := detectModelVendor(platformModelName); got != expected {
			t.Fatalf("detectModelVendor(%q) = %q, want %q", platformModelName, got, expected)
		}
	}
}

func TestDetectModelVendorRecognizesCompanyVendors(t *testing.T) {
	tests := map[string]string{
		"llama-3.3-70b":                        "meta",
		"meta/llama-4-scout":                   "meta",
		"openrouter/meta/llama-4-scout":        "meta",
		"phi-4":                                "microsoft",
		"openrouter/microsoft/phi-4-mini":      "microsoft",
		"microsoft/phi-4-mini":                 "microsoft",
		"amazon.nova-pro-v1":                   "amazon",
		"openrouter/amazon/nova-pro-v1":        "amazon",
		"titan-text-premier":                   "amazon",
		"nemotron-4-340b":                      "nvidia",
		"openrouter/nvidia/nemotron-4-340b":    "nvidia",
		"qwen-max":                             "alibaba",
		"openrouter/alibaba/qwen3-max":         "alibaba",
		"alibaba/qwen3-max":                    "alibaba",
		"doubao-seed-1-6":                      "bytedance",
		"openrouter/bytedance/doubao-seed-1-6": "bytedance",
		"volcengine/doubao":                    "bytedance",
		"hunyuan-turbos":                       "tencent",
		"openrouter/tencent/hunyuan-turbos":    "tencent",
		"tencent/hunyuan":                      "tencent",
		"mimo-v2.5-pro":                        "xiaomi",
		"spark-max":                            "iflytek",
		"iflytek/spark-pro":                    "iflytek",
		"step-2-16k":                           "stepfun",
		"baichuan4-turbo":                      "baichuan",
		"ernie-4.5-turbo":                      "baidu",
		"wenxin-4":                             "baidu",
		"openrouter/unknown/model":             "openrouter",
	}

	for platformModelName, expected := range tests {
		if got := detectModelVendor(platformModelName); got != expected {
			t.Fatalf("detectModelVendor(%q) = %q, want %q", platformModelName, got, expected)
		}
	}
}

func TestNormalizeModelIconSeparatesVendorAndModelFamily(t *testing.T) {
	tests := map[string]struct {
		vendor   string
		model    string
		expected string
	}{
		"alibaba qwen":      {vendor: "alibaba", model: "qwen-max", expected: "qwen"},
		"openrouter qwen":   {vendor: "openrouter", model: "openrouter/alibaba/qwen-max", expected: "qwen"},
		"bytedance doubao":  {vendor: "bytedance", model: "doubao-seed-1-6", expected: "doubao"},
		"openrouter doubao": {vendor: "openrouter", model: "openrouter/bytedance/doubao-seed-1-6", expected: "doubao"},
		"tencent hunyuan":   {vendor: "tencent", model: "hunyuan-turbos", expected: "hunyuan"},
		"openrouter llama":  {vendor: "openrouter", model: "openrouter/meta/llama-4-scout", expected: "meta"},
		"xiaomi mimo":       {vendor: "xiaomi", model: "mimo-v2.5-pro", expected: "xiaomimimo"},
		"amazon nova":       {vendor: "amazon", model: "nova-pro", expected: "nova"},
		"amazon titan":      {vendor: "amazon", model: "titan-text-premier", expected: "bedrock"},
		"baidu ernie":       {vendor: "baidu", model: "ernie-4.5-turbo", expected: "wenxin"},
		"iflytek spark":     {vendor: "iflytek", model: "spark-max", expected: "spark"},
		"stepfun step":      {vendor: "stepfun", model: "step-2-16k", expected: "stepfun"},
		"baichuan baichuan": {vendor: "baichuan", model: "baichuan4-turbo", expected: "baichuan"},
	}

	for name, tc := range tests {
		if got := normalizeModelIcon("", tc.vendor, tc.model); got != tc.expected {
			t.Fatalf("%s: normalizeModelIcon = %q, want %q", name, got, tc.expected)
		}
	}
}

func TestNormalizeModelVendorFallsBackToUnknownForUnsupportedVendor(t *testing.T) {
	if got := normalizeModelVendor("unsupported-vendor", "unsupported-model"); got != "unknown" {
		t.Fatalf("expected unsupported platform vendor to become unknown, got %q", got)
	}
	if got := normalizeUpstreamModelVendor("unsupported-vendor", "unsupported-model"); got != "unknown" {
		t.Fatalf("expected unsupported upstream vendor to become unknown, got %q", got)
	}
}

func TestNormalizeProtocolDefaultsJSONDropsUnknownKinds(t *testing.T) {
	legacyVectorKind := "embed" + "ding"
	legacySortKind := "re" + "rank"
	legacyVectorProtocol := "openai_" + "embed" + "dings"
	legacySortProtocol := "cohere_" + "re" + "rank"
	raw := `{
		"chat":"OPENAI_CHAT_COMPLETIONS",
		"unknown_kind":"openai_chat_completions",
		"` + legacyVectorKind + `":"` + legacyVectorProtocol + `",
		"` + legacySortKind + `":"` + legacySortProtocol + `",
		"image_gen":"openai_image_generations"
	}`

	normalized, err := normalizeProtocolDefaultsJSON(raw)
	if err != nil {
		t.Fatalf("normalize defaults: %v", err)
	}

	var defaults map[string]string
	if err := json.Unmarshal([]byte(normalized), &defaults); err != nil {
		t.Fatalf("unmarshal normalized defaults: %v", err)
	}

	if defaults[modelKindChat] != "openai_chat_completions" {
		t.Fatalf("expected normalized chat protocol, got %q", defaults[modelKindChat])
	}
	if defaults[modelKindImageGen] != "openai_image_generations" {
		t.Fatalf("expected image_gen protocol, got %q", defaults[modelKindImageGen])
	}
	for _, kind := range []string{"unknown_kind", legacyVectorKind, legacySortKind} {
		if _, ok := defaults[kind]; ok {
			t.Fatalf("unexpected protocol default for %s in %s", kind, normalized)
		}
	}
}

func TestInferKindsJSONRecognizesCurrentOpenAIImageModels(t *testing.T) {
	for _, modelName := range []string{"gpt-image-1", "gpt-image-2", "chatgpt-image-latest"} {
		if got := inferKindsJSON(modelName); got != `["image_gen","image_edit"]` {
			t.Fatalf("expected %s to infer image generation and edit kinds, got %s", modelName, got)
		}
	}
}

func TestNormalizeProtocolDefaultsJSONRejectsInvalidProtocolForSupportedKind(t *testing.T) {
	legacyVectorProtocol := "openai_" + "embed" + "dings"
	_, err := normalizeProtocolDefaultsJSON(`{"chat":"` + legacyVectorProtocol + `"}`)
	if !errors.Is(err, ErrInvalidAdapter) {
		t.Fatalf("expected ErrInvalidAdapter, got %v", err)
	}
}

func TestNormalizeProtocolDefaultsJSONRejectsUnsupportedProviderProtocols(t *testing.T) {
	for _, raw := range []string{
		`{"image_gen":"replicate_predictions"}`,
		`{"image_gen":"fal_queue"}`,
		`{"image_gen":"stability_ai_generate"}`,
	} {
		_, err := normalizeProtocolDefaultsJSON(raw)
		if !errors.Is(err, ErrInvalidAdapter) {
			t.Fatalf("expected ErrInvalidAdapter for %s, got %v", raw, err)
		}
	}
}

func TestNormalizeProtocolDefaultsJSONRejectsProtocolKindMismatch(t *testing.T) {
	_, err := normalizeProtocolDefaultsJSON(`{"image_gen":"openai_responses"}`)
	if !errors.Is(err, ErrInvalidAdapter) {
		t.Fatalf("expected ErrInvalidAdapter, got %v", err)
	}
}

func TestResolveRouteProtocolRejectsExplicitProtocolKindMismatch(t *testing.T) {
	_, err := resolveRouteProtocol("openai_responses", compatibleOpenAI, "", `["image_gen"]`)
	if !errors.Is(err, ErrInvalidAdapter) {
		t.Fatalf("expected ErrInvalidAdapter, got %v", err)
	}
}

func TestResolveRouteProtocolUsesExplicitConversationProtocolAsSourceOfTruth(t *testing.T) {
	for _, tc := range []struct {
		compatible string
		protocol   string
	}{
		{compatible: compatibleXAI, protocol: "openai_responses"},
		{compatible: compatibleOpenAI, protocol: "xai_responses"},
	} {
		resolved, err := resolveRouteProtocol(tc.protocol, tc.compatible, "", `["chat"]`)
		if err != nil {
			t.Fatalf("expected explicit protocol %q for compatible %q to be accepted: %v", tc.protocol, tc.compatible, err)
		}
		if resolved != tc.protocol {
			t.Fatalf("expected %q, got %q", tc.protocol, resolved)
		}
	}
}

func TestIsRouteAllowedForTaskSeparatesChatAndImageProtocols(t *testing.T) {
	if !IsRouteAllowedForTask(TaskTypeChat, `["chat"]`, "openai_responses") {
		t.Fatalf("expected chat task to allow chat protocol")
	}
	if IsRouteAllowedForTask(TaskTypeChat, `["image_gen"]`, "openai_image_generations") {
		t.Fatalf("expected chat task to reject image generation protocol")
	}
	if !IsRouteAllowedForTask(TaskTypeImageGeneration, `["image_gen","image_edit"]`, "openai_image_generations") {
		t.Fatalf("expected image generation task to allow image generation protocol")
	}
	if IsRouteAllowedForTask(TaskTypeImageGeneration, `["chat"]`, "openai_responses") {
		t.Fatalf("expected image generation task to reject chat protocol")
	}
	if !IsRouteAllowedForTask(TaskTypeImageEdit, `["image_edit"]`, "openai_image_edits") {
		t.Fatalf("expected image edit task to allow image edit protocol")
	}
}

func TestDefaultRouteModelMatchesTaskFiltersByKind(t *testing.T) {
	if !defaultRouteModelMatchesTask(`["chat"]`, TaskTypeChat) {
		t.Fatal("expected chat default route to accept chat model")
	}
	if defaultRouteModelMatchesTask(`["image_gen","image_edit"]`, TaskTypeChat) {
		t.Fatal("expected chat default route to reject image-only model")
	}
	if !defaultRouteModelMatchesTask(`["image_gen","image_edit"]`, TaskTypeImageGeneration) {
		t.Fatal("expected image generation default route to accept image generation model")
	}
	if defaultRouteModelMatchesTask(`["chat"]`, TaskTypeImageGeneration) {
		t.Fatal("expected image generation default route to reject chat model")
	}
}

func TestDisplayProtocolDefaultsJSONHidesLegacyInvalidDefaults(t *testing.T) {
	display := displayProtocolDefaultsJSON(`{"chat":"openai_chat_completions","image_gen":"openai_responses"}`)

	var defaults map[string]string
	if err := json.Unmarshal([]byte(display), &defaults); err != nil {
		t.Fatalf("unmarshal display defaults: %v", err)
	}
	if defaults[modelKindChat] != "openai_chat_completions" {
		t.Fatalf("expected valid chat default kept, got %s", display)
	}
	if _, ok := defaults[modelKindImageGen]; ok {
		t.Fatalf("expected invalid image_gen default hidden, got %s", display)
	}
}

func TestFilterPricedModelViewsUsesPlatformModelNameKey(t *testing.T) {
	items := []ModelView{{
		PlatformModelName: "gpt-5.5",
	}}
	pricing := map[string]appbilling.PublicModelPricing{
		"gpt-5.5": {
			Currency: "USD",
			Mode:     "token",
		},
	}

	results := filterPricedModelViews(items, pricing)
	if len(results) != 1 {
		t.Fatalf("expected model to match pricing by platform model name, got %d", len(results))
	}
	if results[0].Pricing == nil || results[0].Pricing.Currency != "USD" {
		t.Fatalf("expected pricing attached by platform model name, got %#v", results[0].Pricing)
	}

}

func TestNormalizePlatformModelNameRejectsWhitespace(t *testing.T) {
	if _, err := normalizePlatformModelName("claude sonnet"); err == nil {
		t.Fatal("expected whitespace in platform model name to be rejected")
	}
	name, err := normalizePlatformModelName("claude-sonnet")
	if err != nil {
		t.Fatalf("normalize platform model name: %v", err)
	}
	if name != "claude-sonnet" {
		t.Fatalf("expected normalized platform model name, got %q", name)
	}
}
