package llm

import "testing"

func TestBuildOpenAICompatibleURLsRespectVersionedBasePath(t *testing.T) {
	cases := []struct {
		name     string
		baseURL  string
		endpoint string
		want     string
	}{
		{
			name:     "plain base uses v1 chat completions",
			baseURL:  "https://api.example.com",
			endpoint: EndpointChatCompletions,
			want:     "https://api.example.com/v1/chat/completions",
		},
		{
			name:     "openai v1 base is not duplicated",
			baseURL:  "https://api.openai.com/v1",
			endpoint: EndpointResponses,
			want:     "https://api.openai.com/v1/responses",
		},
		{
			name:     "openai image generations endpoint",
			baseURL:  "https://api.openai.com/v1",
			endpoint: EndpointImageGenerations,
			want:     "https://api.openai.com/v1/images/generations",
		},
		{
			name:     "openai image edits endpoint",
			baseURL:  "https://api.openai.com/v1",
			endpoint: EndpointImageEdits,
			want:     "https://api.openai.com/v1/images/edits",
		},
		{
			name:     "xai v1 base is not duplicated",
			baseURL:  "https://api.x.ai/v1",
			endpoint: EndpointResponses,
			want:     "https://api.x.ai/v1/responses",
		},
		{
			name:     "xai image generations endpoint",
			baseURL:  "https://api.x.ai/v1",
			endpoint: EndpointImageGenerations,
			want:     "https://api.x.ai/v1/images/generations",
		},
		{
			name:     "xai proxy plain base gets v1 image endpoint",
			baseURL:  "https://proxy.example.com",
			endpoint: EndpointImageGenerations,
			want:     "https://proxy.example.com/v1/images/generations",
		},
		{
			name:     "xai proxy v1 base is not duplicated",
			baseURL:  "https://proxy.example.com/v1",
			endpoint: EndpointImageGenerations,
			want:     "https://proxy.example.com/v1/images/generations",
		},
		{
			name:     "xai proxy nested v1 base is not duplicated",
			baseURL:  "https://proxy.example.com/xai/v1",
			endpoint: EndpointImageGenerations,
			want:     "https://proxy.example.com/xai/v1/images/generations",
		},
		{
			name:     "xai proxy v4 base is respected",
			baseURL:  "https://proxy.example.com/v4",
			endpoint: EndpointImageGenerations,
			want:     "https://proxy.example.com/v4/images/generations",
		},
		{
			name:     "bigmodel v4 base is respected",
			baseURL:  "https://open.bigmodel.cn/api/paas/v4",
			endpoint: EndpointChatCompletions,
			want:     "https://open.bigmodel.cn/api/paas/v4/chat/completions",
		},
		{
			name:     "trailing slash on versioned base is trimmed",
			baseURL:  "https://open.bigmodel.cn/api/paas/v4/",
			endpoint: EndpointResponses,
			want:     "https://open.bigmodel.cn/api/paas/v4/responses",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildOpenAIRequestURL(tt.baseURL, tt.endpoint); got != tt.want {
				t.Fatalf("unexpected request url: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildOpenAIModelsURLRespectsVersionedBasePath(t *testing.T) {
	cases := map[string]string{
		"https://api.example.com":              "https://api.example.com/v1/models",
		"https://api.example.com/v1":           "https://api.example.com/v1/models",
		"https://open.bigmodel.cn/api/paas/v4": "https://open.bigmodel.cn/api/paas/v4/models",
	}
	for baseURL, want := range cases {
		if got := buildOpenAIModelsURL(baseURL); got != want {
			t.Fatalf("unexpected models url for %q: got %q, want %q", baseURL, got, want)
		}
	}
}

func TestBuildAnthropicURLsRespectVersionedBasePath(t *testing.T) {
	messageCases := map[string]string{
		"https://api.anthropic.com":              "https://api.anthropic.com/v1/messages",
		"https://api.anthropic.com/v1":           "https://api.anthropic.com/v1/messages",
		"https://proxy.example.com/anthropic/v4": "https://proxy.example.com/anthropic/v4/messages",
	}
	for baseURL, want := range messageCases {
		if got := buildAnthropicMessagesURL(baseURL); got != want {
			t.Fatalf("unexpected anthropic messages url for %q: got %q, want %q", baseURL, got, want)
		}
	}
	if got, want := buildAnthropicModelsURL("https://proxy.example.com/anthropic/v4"), "https://proxy.example.com/anthropic/v4/models"; got != want {
		t.Fatalf("unexpected anthropic models url: got %q, want %q", got, want)
	}
}

func TestBuildGeminiURLsRespectVersionedBasePath(t *testing.T) {
	cases := map[string]string{
		"https://generativelanguage.googleapis.com":        "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent",
		"https://generativelanguage.googleapis.com/v1beta": "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent",
		"https://proxy.example.com/google/v1":              "https://proxy.example.com/google/v1beta/models/gemini-2.0-flash:generateContent",
	}
	for baseURL, want := range cases {
		if got := buildGeminiGenerateURL(baseURL, "gemini-2.0-flash"); got != want {
			t.Fatalf("unexpected gemini generate url for %q: got %q, want %q", baseURL, got, want)
		}
	}
	if got, want := buildGeminiModelsURL("https://generativelanguage.googleapis.com/v1beta"), "https://generativelanguage.googleapis.com/v1beta/models"; got != want {
		t.Fatalf("unexpected gemini models url: got %q, want %q", got, want)
	}
	if got, want := buildGeminiModelsURL("https://proxy.example.com/v1"), "https://proxy.example.com/v1beta/models"; got != want {
		t.Fatalf("unexpected gemini models url for v1 base: got %q, want %q", got, want)
	}
	if got, want := buildGeminiStreamURL("https://proxy.example.com/v1", "gemini-3-pro-image-preview"), "https://proxy.example.com/v1beta/models/gemini-3-pro-image-preview:streamGenerateContent?alt=sse"; got != want {
		t.Fatalf("unexpected gemini stream url for v1 base: got %q, want %q", got, want)
	}
}

func TestBuildVersionedEndpointURLKeepsDefaultForNonVersionedBasePath(t *testing.T) {
	cases := map[string]string{
		"https://proxy.example.com/openai":      "https://proxy.example.com/openai/v1/models",
		"https://proxy.example.com/openai-api":  "https://proxy.example.com/openai-api/v1/models",
		"https://proxy.example.com/api/version": "https://proxy.example.com/api/version/v1/models",
	}
	for baseURL, want := range cases {
		if got := buildVersionedEndpointURL(baseURL, "v1", "/models"); got != want {
			t.Fatalf("unexpected endpoint url for %q: got %q, want %q", baseURL, got, want)
		}
	}
}
