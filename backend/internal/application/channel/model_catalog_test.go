package channel

import (
	"encoding/json"
	"errors"
	"testing"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
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
	if defaults[modelKindImageGen] != "xai_image" {
		t.Fatalf("expected xAI image default, got %q in %s", defaults[modelKindImageGen], raw)
	}
	if defaults[modelKindImageEdit] != "xai_image_edits" {
		t.Fatalf("expected xAI image edit default, got %q in %s", defaults[modelKindImageEdit], raw)
	}
	for _, kind := range []string{modelKindVideoGen} {
		if _, ok := defaults[kind]; ok {
			t.Fatalf("unexpected xAI default protocol for %s in %s", kind, raw)
		}
	}
}

func TestProtocolDefaultsForOpenRouterUsesOpenRouterResponsesForConversationKinds(t *testing.T) {
	raw := protocolDefaultsForCompatible(compatibleOpenRouter)

	var defaults map[string]string
	if err := json.Unmarshal([]byte(raw), &defaults); err != nil {
		t.Fatalf("unmarshal defaults: %v", err)
	}

	if defaults[modelKindChat] != "openrouter_responses" {
		t.Fatalf("expected OpenRouter chat default, got %q in %s", defaults[modelKindChat], raw)
	}
	if defaults[modelKindAudio] != "openrouter_responses" {
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

func TestProtocolDefaultsForGoogleUsesGoogleImageGeneration(t *testing.T) {
	raw := protocolDefaultsForCompatible(compatibleGoogle)

	var defaults map[string]string
	if err := json.Unmarshal([]byte(raw), &defaults); err != nil {
		t.Fatalf("unmarshal defaults: %v", err)
	}

	if defaults[modelKindChat] != "google_generate_content" {
		t.Fatalf("expected Google chat default, got %q in %s", defaults[modelKindChat], raw)
	}
	if defaults[modelKindImageGen] != "google_image_generation" {
		t.Fatalf("expected Google image generation default, got %q in %s", defaults[modelKindImageGen], raw)
	}
	if defaults[modelKindImageEdit] != "google_image_generation" {
		t.Fatalf("expected Google image edit default, got %q in %s", defaults[modelKindImageEdit], raw)
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

func TestProtocolDefaultsForCustomUsesOpenAICompatibleDefaults(t *testing.T) {
	raw := protocolDefaultsForCompatible(compatibleCustom)

	var defaults map[string]string
	if err := json.Unmarshal([]byte(raw), &defaults); err != nil {
		t.Fatalf("unmarshal defaults: %v", err)
	}
	if defaults[modelKindChat] != "openai_chat_completions" {
		t.Fatalf("expected custom chat default, got %q in %s", defaults[modelKindChat], raw)
	}
	if defaults[modelKindAudio] != "openai_chat_completions" {
		t.Fatalf("expected custom audio default, got %q in %s", defaults[modelKindAudio], raw)
	}
	if defaults[modelKindImageGen] != "openai_image_generations" {
		t.Fatalf("expected custom image generation default, got %q in %s", defaults[modelKindImageGen], raw)
	}
	if defaults[modelKindImageEdit] != "openai_image_edits" {
		t.Fatalf("expected custom image edit default, got %q in %s", defaults[modelKindImageEdit], raw)
	}
	for _, kind := range []string{modelKindVideoGen} {
		if _, ok := defaults[kind]; ok {
			t.Fatalf("unexpected custom default protocol for %s in %s", kind, raw)
		}
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
		"nano-banana-pro":                      "google",
		"gemini-3-pro-image":                   "google",
		"gemini-3-pro-image-preview":           "google",
		"openrouter/unknown/model":             "openrouter",
	}

	for platformModelName, expected := range tests {
		if got := detectModelVendor(platformModelName); got != expected {
			t.Fatalf("detectModelVendor(%q) = %q, want %q", platformModelName, got, expected)
		}
	}
}

func TestReasoningContentPassbackRequiredForDeepSeekChatCompletions(t *testing.T) {
	if !reasoningContentPassbackRequired(llm.AdapterOpenAIChatCompletions, "deepseek", "deepseek-v4-flash-free") {
		t.Fatal("expected DeepSeek Chat Completions route to require reasoning_content passback")
	}
	if reasoningContentPassbackRequired(llm.AdapterOpenAIChatCompletions, "openai", "gpt-5.4") {
		t.Fatal("expected OpenAI Chat Completions route to skip reasoning_content passback")
	}
	if reasoningContentPassbackRequired(llm.AdapterOpenAIResponses, "deepseek", "deepseek-v4-flash-free") {
		t.Fatal("expected non Chat Completions route to skip reasoning_content passback")
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
		"google nano":       {vendor: "google", model: "nano-banana-pro", expected: "nanobanana"},
		"google image":      {vendor: "google", model: "gemini-3-pro-image", expected: "nanobanana"},
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

func TestInferKindsJSONRecognizesGeminiImageModels(t *testing.T) {
	for _, modelName := range []string{
		"nano-banana",
		"nano-banana-2",
		"nano-banana-pro",
		"gemini-2.5-flash-image",
		"gemini-3.1-flash-image",
		"gemini-3-pro-image",
		"gemini-3.1-flash-image-preview",
		"gemini-3-pro-image-preview",
	} {
		if got := inferKindsJSON(modelName); got != `["image_gen","image_edit"]` {
			t.Fatalf("expected %s to infer image generation and edit kinds, got %s", modelName, got)
		}
	}
}

func TestInferKindsJSONRecognizesXAIImageModels(t *testing.T) {
	for _, modelName := range []string{
		"grok-imagine-image",
		"grok-imagine-image-quality",
		"grok-imagine-image-pro",
		"grok-imagine-image-preview",
	} {
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

func TestResolveRouteProtocolAcceptsExplicitProtocolForAnyDeclaredKind(t *testing.T) {
	resolved, err := resolveRouteProtocol("openai_image_edits", compatibleOpenAI, "", `["image_gen","image_edit"]`)
	if err != nil {
		t.Fatalf("expected explicit image edit protocol to be accepted for dual-kind image model: %v", err)
	}
	if resolved != "openai_image_edits" {
		t.Fatalf("expected openai_image_edits, got %q", resolved)
	}
}

func TestSupportedRouteProtocolCombinationOnlyAllowsSameProviderImagePair(t *testing.T) {
	tests := []struct {
		name      string
		protocols []string
		want      bool
	}{
		{name: "single chat", protocols: []string{"openai_responses"}, want: true},
		{name: "single image generation", protocols: []string{"openai_image_generations"}, want: true},
		{name: "openai image generation and edit", protocols: []string{"openai_image_generations", "openai_image_edits"}, want: true},
		{name: "xai image generation and edit", protocols: []string{"xai_image", "xai_image_edits"}, want: true},
		{name: "duplicate protocol", protocols: []string{"openai_responses", "openai_responses"}, want: true},
		{name: "two chat protocols", protocols: []string{"openai_responses", "openai_chat_completions"}, want: false},
		{name: "image generation with chat", protocols: []string{"openai_image_generations", "openai_responses"}, want: false},
		{name: "mixed provider image pair", protocols: []string{"openai_image_generations", "xai_image_edits"}, want: false},
		{name: "three protocols", protocols: []string{"openai_image_generations", "openai_image_edits", "openai_responses"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSupportedRouteProtocolCombination(tt.protocols); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestResolveRouteProtocolsExpandsOpenAIDualImageKinds(t *testing.T) {
	protocols, err := resolveRouteProtocols(nil, compatibleOpenAI, "", `["image_gen","image_edit"]`)
	if err != nil {
		t.Fatalf("resolve route protocols: %v", err)
	}
	expected := []string{"openai_image_generations", "openai_image_edits"}
	if len(protocols) != len(expected) {
		t.Fatalf("expected %d protocols, got %#v", len(expected), protocols)
	}
	for i, expectedProtocol := range expected {
		if protocols[i] != expectedProtocol {
			t.Fatalf("expected protocol %d to be %q, got %#v", i, expectedProtocol, protocols)
		}
	}
}

func TestResolveRouteProtocolsKeepsSingleGoogleProtocolForDualImageKinds(t *testing.T) {
	protocols, err := resolveRouteProtocols(nil, compatibleGoogle, "", `["image_gen","image_edit"]`)
	if err != nil {
		t.Fatalf("resolve route protocols: %v", err)
	}
	if len(protocols) != 1 || protocols[0] != "google_image_generation" {
		t.Fatalf("expected single Google image protocol, got %#v", protocols)
	}
}

func TestResolveRouteProtocolsExpandsXAIDualImageKinds(t *testing.T) {
	protocols, err := resolveRouteProtocols(nil, compatibleXAI, "", `["image_gen","image_edit"]`)
	if err != nil {
		t.Fatalf("resolve route protocols: %v", err)
	}
	expected := []string{"xai_image", "xai_image_edits"}
	if len(protocols) != len(expected) {
		t.Fatalf("expected %d protocols, got %#v", len(expected), protocols)
	}
	for i, expectedProtocol := range expected {
		if protocols[i] != expectedProtocol {
			t.Fatalf("expected protocol %d to be %q, got %#v", i, expectedProtocol, protocols)
		}
	}
}

func TestResolveRouteProtocolsKeepsSingleProtocolForGenerationOnlyModels(t *testing.T) {
	tests := []struct {
		name       string
		compatible string
		kindsJSON  string
		expected   string
	}{
		{name: "openai generation", compatible: compatibleOpenAI, kindsJSON: `["image_gen"]`, expected: "openai_image_generations"},
		{name: "google generation", compatible: compatibleGoogle, kindsJSON: `["image_gen"]`, expected: "google_image_generation"},
		{name: "xai generation", compatible: compatibleXAI, kindsJSON: `["image_gen"]`, expected: "xai_image"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			protocols, err := resolveRouteProtocols(nil, tt.compatible, "", tt.kindsJSON)
			if err != nil {
				t.Fatalf("resolve route protocols: %v", err)
			}
			if len(protocols) != 1 || protocols[0] != tt.expected {
				t.Fatalf("expected [%s], got %#v", tt.expected, protocols)
			}
		})
	}
}

func TestResolveRouteProtocolsRejectsUnsupportedExplicitMultiProtocol(t *testing.T) {
	_, err := resolveRouteProtocols([]string{"openai_responses", "openai_chat_completions"}, compatibleOpenAI, "", `["chat"]`)
	if !errors.Is(err, ErrInvalidRouteProtocolCombination) {
		t.Fatalf("expected ErrInvalidRouteProtocolCombination, got %v", err)
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
	if !IsRouteAllowedForTask(TaskTypeImageGeneration, `["image_gen"]`, "google_image_generation") {
		t.Fatalf("expected image generation task to allow Google image generation protocol")
	}
	if !IsRouteAllowedForTask(TaskTypeImageGeneration, `["image_gen"]`, "xai_image") {
		t.Fatalf("expected image generation task to allow xAI image protocol")
	}
	if IsRouteAllowedForTask(TaskTypeImageGeneration, `["chat"]`, "openai_responses") {
		t.Fatalf("expected image generation task to reject chat protocol")
	}
	if !IsRouteAllowedForTask(TaskTypeImageEdit, `["image_edit"]`, "openai_image_edits") {
		t.Fatalf("expected image edit task to allow image edit protocol")
	}
	if !IsRouteAllowedForTask(TaskTypeImageEdit, `["image_edit"]`, "google_image_generation") {
		t.Fatalf("expected image edit task to allow Google image protocol")
	}
	if !IsRouteAllowedForTask(TaskTypeImageEdit, `["image_edit"]`, "xai_image_edits") {
		t.Fatalf("expected image edit task to allow xAI image edits protocol")
	}
	if IsRouteAllowedForTask(TaskTypeImageEdit, `["image_edit"]`, "xai_image") {
		t.Fatalf("expected image edit task to reject xAI image generation protocol")
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

func TestNormalizePlatformModelNameAllowsDisplaySpaces(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "plain", raw: "claude-sonnet", want: "claude-sonnet"},
		{name: "display spaces", raw: "GPT Image 1", want: "GPT Image 1"},
		{name: "trim outer spaces", raw: "  GPT Image 1  ", want: "GPT Image 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePlatformModelName(tt.raw)
			if err != nil {
				t.Fatalf("normalize platform model name: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestNormalizePlatformModelNameRejectsUnsafeWhitespace(t *testing.T) {
	tests := []string{
		"",
		"   ",
		"claude\tsonnet",
		"claude\nsonnet",
		"claude\u00a0sonnet",
	}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if _, err := normalizePlatformModelName(raw); err == nil {
				t.Fatal("expected unsafe platform model name to be rejected")
			}
		})
	}
}
