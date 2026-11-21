package llm

import (
	"reflect"
	"testing"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func TestServerToolVendorForBaseURL(t *testing.T) {
	cases := map[string]serverToolVendor{
		"https://dashscope.aliyuncs.com/compatible-mode/v1":           serverToolVendorBailian,
		"https://dashscope-us.aliyuncs.com/compatible-mode/v1":        serverToolVendorBailian,
		"https://ws1.cn-beijing.maas.aliyuncs.com/compatible-mode/v1": serverToolVendorBailian,
		"https://dashscope-intl.aliyuncs.com/compatible-mode/v1":      serverToolVendorBailianIntl,
		"https://ws1.ap-southeast-1.maas.aliyuncs.com/apps/anthropic": serverToolVendorBailianIntl,
		"https://ark.cn-beijing.volces.com/api/v3":                    serverToolVendorArk,
		"https://api.openai.com/v1":                                   serverToolVendorUnknown,
		"https://ark.cn-beijing.volces.com.evil.example/api/v3":       serverToolVendorUnknown,
		"https://evil-dashscope.aliyuncs.com.example/v1":              serverToolVendorUnknown,
	}
	for baseURL, want := range cases {
		if got := serverToolVendorForBaseURL(baseURL); got != want {
			t.Errorf("serverToolVendorForBaseURL(%q) = %v, want %v", baseURL, got, want)
		}
	}
}

func TestParseServerSideToolUsageReadsBailianShapes(t *testing.T) {
	qwen4 := map[string]any{
		"usage":      map[string]any{"input_tokens": 10},
		"tool_usage": map[string]any{"web_search": map[string]any{"num_requests": 2}, "web_extractor": map[string]any{"num_requests": 1}},
	}
	if got := parseServerSideToolUsage(qwen4); got["web_search"] != 2 || got["web_extractor"] != 1 {
		t.Fatalf("qwen4 top-level tool_usage = %#v", got)
	}
	qwen38 := map[string]any{"usage": map[string]any{"x_tools": map[string]any{"web_search": map[string]any{"count": 3}}}}
	if got := parseServerSideToolUsage(qwen38); got["web_search"] != 3 {
		t.Fatalf("qwen3.8 usage.x_tools = %#v", got)
	}
}

func TestAttributeVendorServerToolUsage(t *testing.T) {
	cases := []struct {
		name     string
		route    portllm.RouteConfig
		usage    map[string]int64
		rawUsage string
		want     map[string]int64
	}{
		{
			name:  "unknown host keeps usage",
			route: portllm.RouteConfig{Protocol: portllm.AdapterOpenAIResponses, BaseURL: "https://api.openai.com/v1"},
			usage: map[string]int64{"web_search": 2},
			want:  map[string]int64{"web_search": 2},
		},
		{
			name:  "bailian responses",
			route: portllm.RouteConfig{Protocol: portllm.AdapterOpenAIResponses, BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1"},
			usage: map[string]int64{"web_search": 2, "web_extractor": 1},
			want:  map[string]int64{usageKeyBailianWebSearch: 2, "web_extractor": 1},
		},
		{
			name:  "bailian singapore anthropic",
			route: portllm.RouteConfig{Protocol: portllm.AdapterAnthropicMessages, BaseURL: "https://dashscope-intl.aliyuncs.com/apps/anthropic"},
			usage: map[string]int64{"web_search": 1},
			want:  map[string]int64{usageKeyBailianWebSearchIntl: 1},
		},
		{
			name:     "bailian chat completions x_tools",
			route:    portllm.RouteConfig{Protocol: portllm.AdapterOpenAIChatCompletions, BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1"},
			rawUsage: `{"prompt_tokens":10,"x_tools":{"search":{"count":1}}}`,
			want:     map[string]int64{usageKeyBailianWebSearch: 1},
		},
		{
			name:     "ark responses splits sources",
			route:    portllm.RouteConfig{Protocol: portllm.AdapterOpenAIResponses, BaseURL: "https://ark.cn-beijing.volces.com/api/v3"},
			usage:    map[string]int64{"web_search": 4},
			rawUsage: `{"tool_usage":{"web_search":4},"tool_usage_details":{"web_search":{"search_engine":2,"toutiao":1,"douyin":1}}}`,
			want:     map[string]int64{usageKeyArkWebSearch: 2, usageKeyArkContentSource: 2},
		},
		{
			name:     "ark responses doubao",
			route:    portllm.RouteConfig{Protocol: portllm.AdapterOpenAIResponses, BaseURL: "https://ark.cn-beijing.volces.com/api/v3"},
			usage:    map[string]int64{"web_search": 1},
			rawUsage: `{"tool_usage":{"web_search":1},"tool_usage_details":{"web_search":{"doubao":1}}}`,
			want:     map[string]int64{usageKeyArkDoubaoSearch: 1},
		},
		{
			name:  "ark responses without details",
			route: portllm.RouteConfig{Protocol: portllm.AdapterOpenAIResponses, BaseURL: "https://ark.cn-beijing.volces.com/api/v3"},
			usage: map[string]int64{"web_search": 3},
			want:  map[string]int64{usageKeyArkWebSearch: 3},
		},
		{
			name:     "ark chat completions web_search_options",
			route:    portllm.RouteConfig{Protocol: portllm.AdapterOpenAIChatCompletions, BaseURL: "https://ark.cn-beijing.volces.com/api/v3"},
			rawUsage: `{"prompt_tokens":10,"tool_usage":{"web_search":2}}`,
			want:     map[string]int64{usageKeyArkDoubaoSearch: 2},
		},
		{
			name:  "ark anthropic messages",
			route: portllm.RouteConfig{Protocol: portllm.AdapterAnthropicMessages, BaseURL: "https://ark.cn-beijing.volces.com/api/compatible"},
			usage: map[string]int64{"web_search": 1},
			want:  map[string]int64{usageKeyArkDoubaoSearch: 1},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output := &portllm.GenerateOutput{ServerSideToolUsage: tc.usage, Usage: portllm.Usage{RawUsageJSON: tc.rawUsage}}
			attributeVendorServerToolUsage(tc.route, output)
			if !reflect.DeepEqual(output.ServerSideToolUsage, tc.want) {
				t.Fatalf("usage = %#v, want %#v", output.ServerSideToolUsage, tc.want)
			}
		})
	}
	attributeVendorServerToolUsage(portllm.RouteConfig{BaseURL: "https://ark.cn-beijing.volces.com/api/v3"}, nil)
}
