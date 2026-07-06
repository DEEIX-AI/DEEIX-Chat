package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStripProviderPrefix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"openai/gpt-5.4", "gpt-5.4"},
		{"anthropic/claude-sonnet-5", "claude-sonnet-5"},
		{"gpt-5.4", "gpt-5.4"},
		{"google/gemini-3-flash-preview", "gemini-3-flash-preview"},
	}
	for _, tt := range tests {
		got := stripProviderPrefix(tt.input)
		if got != tt.want {
			t.Errorf("stripProviderPrefix(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLookupPricingExactVendorMatch(t *testing.T) {
	byVendor := map[string]map[string]OfficialPricingEntry{
		"openai": {
			"gpt-5.4": {InputUSDPerMTokens: 2.5, OutputUSDPerMTokens: 15},
		},
		"anthropic": {
			"claude-sonnet-5": {InputUSDPerMTokens: 3, OutputUSDPerMTokens: 15},
		},
	}
	entry := lookupPricing(byVendor, "gpt-5.4", "openai")
	if entry == nil {
		t.Fatal("expected match")
	}
	if entry.InputUSDPerMTokens != 2.5 {
		t.Errorf("got input %v, want 2.5", entry.InputUSDPerMTokens)
	}
}

func TestLookupPricingFallbackAcrossVendors(t *testing.T) {
	byVendor := map[string]map[string]OfficialPricingEntry{
		"openai": {
			"gpt-5.4": {InputUSDPerMTokens: 2.5, OutputUSDPerMTokens: 15},
		},
	}
	entry := lookupPricing(byVendor, "gpt-5.4", "unknown")
	if entry == nil {
		t.Fatal("expected fallback match across vendors")
	}
}

func TestLookupPricingNoMatch(t *testing.T) {
	byVendor := map[string]map[string]OfficialPricingEntry{
		"openai": {
			"gpt-5.4": {InputUSDPerMTokens: 2.5},
		},
	}
	entry := lookupPricing(byVendor, "nonexistent-model", "openai")
	if entry != nil {
		t.Errorf("expected nil, got %+v", entry)
	}
}

func TestLookupPricingEmptyVendor(t *testing.T) {
	byVendor := map[string]map[string]OfficialPricingEntry{
		"deepseek": {
			"deepseek-v4-flash": {InputUSDPerMTokens: 0.14, OutputUSDPerMTokens: 0.28},
		},
	}
	entry := lookupPricing(byVendor, "deepseek-v4-flash", "")
	if entry == nil {
		t.Fatal("expected match with empty vendor via fallback")
	}
	if entry.InputUSDPerMTokens != 0.14 {
		t.Errorf("got input %v, want 0.14", entry.InputUSDPerMTokens)
	}
}

func TestFetchModelsDevPricingParsesResponse(t *testing.T) {
	payload := map[string]interface{}{
		"openai": map[string]interface{}{
			"id": "openai",
			"models": map[string]interface{}{
				"gpt-5.4": map[string]interface{}{
					"id":   "gpt-5.4",
					"name": "GPT-5.4",
					"cost": map[string]interface{}{
						"input":       2.5,
						"output":      15,
						"cache_read":  0.25,
						"cache_write": 0,
					},
				},
				"gpt-5.4-mini": map[string]interface{}{
					"id":   "gpt-5.4-mini",
					"name": "GPT-5.4 Mini",
					"cost": map[string]interface{}{
						"input":  0.3,
						"output": 1.25,
					},
				},
			},
		},
		"anthropic": map[string]interface{}{
			"id": "anthropic",
			"models": map[string]interface{}{
				"claude-sonnet-5": map[string]interface{}{
					"id":   "claude-sonnet-5",
					"name": "Claude Sonnet 5",
					"cost": map[string]interface{}{
						"input":       3,
						"output":      15,
						"cache_read":  0.3,
						"cache_write": 3.75,
					},
				},
			},
		},
		"some-other-provider": map[string]interface{}{
			"id":     "some-other-provider",
			"models": map[string]interface{}{},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	origURL := modelsDevAPIURL
	modelsDevAPIURL_ := srv.URL
	// We can't reassign the const, so we test via a helper that accepts URL.
	result, err := fetchModelsDevPricingFromURL(context.Background(), modelsDevAPIURL_)
	_ = origURL
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	openaiModels, ok := result["openai"]
	if !ok {
		t.Fatal("expected openai in result")
	}
	if len(openaiModels) != 2 {
		t.Errorf("expected 2 openai models, got %d", len(openaiModels))
	}
	gpt54, ok := openaiModels["gpt-5.4"]
	if !ok {
		t.Fatal("expected gpt-5.4 in openai")
	}
	if gpt54.InputUSDPerMTokens != 2.5 {
		t.Errorf("gpt-5.4 input: got %v, want 2.5", gpt54.InputUSDPerMTokens)
	}
	if gpt54.OutputUSDPerMTokens != 15 {
		t.Errorf("gpt-5.4 output: got %v, want 15", gpt54.OutputUSDPerMTokens)
	}
	if gpt54.CacheReadUSDPerMTokens != 0.25 {
		t.Errorf("gpt-5.4 cache_read: got %v, want 0.25", gpt54.CacheReadUSDPerMTokens)
	}

	anthropicModels, ok := result["anthropic"]
	if !ok {
		t.Fatal("expected anthropic in result")
	}
	cs5, ok := anthropicModels["claude-sonnet-5"]
	if !ok {
		t.Fatal("expected claude-sonnet-5 in anthropic")
	}
	if cs5.CacheWriteUSDPerMTokens != 3.75 {
		t.Errorf("claude-sonnet-5 cache_write: got %v, want 3.75", cs5.CacheWriteUSDPerMTokens)
	}

	if _, ok := result["some-other-provider"]; ok {
		t.Error("unmapped provider should not appear in result")
	}
}

func TestFetchModelsDevPricingSkipsZeroCost(t *testing.T) {
	payload := map[string]interface{}{
		"openai": map[string]interface{}{
			"id": "openai",
			"models": map[string]interface{}{
				"text-embedding-3-small": map[string]interface{}{
					"id":   "text-embedding-3-small",
					"name": "Text Embedding 3 Small",
					"cost": map[string]interface{}{
						"input":  0,
						"output": 0,
					},
				},
				"gpt-5.4": map[string]interface{}{
					"id":   "gpt-5.4",
					"name": "GPT-5.4",
					"cost": map[string]interface{}{
						"input":  2.5,
						"output": 15,
					},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	result, err := fetchModelsDevPricingFromURL(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	openaiModels := result["openai"]
	if _, ok := openaiModels["text-embedding-3-small"]; ok {
		t.Error("zero-cost model should be skipped")
	}
	if _, ok := openaiModels["gpt-5.4"]; !ok {
		t.Error("non-zero cost model should be included")
	}
}

func TestFetchModelsDevPricingStripsProviderPrefix(t *testing.T) {
	payload := map[string]interface{}{
		"google": map[string]interface{}{
			"id": "google",
			"models": map[string]interface{}{
				"google/gemini-3-flash": map[string]interface{}{
					"id":   "google/gemini-3-flash",
					"name": "Gemini 3 Flash",
					"cost": map[string]interface{}{
						"input":  0.15,
						"output": 0.6,
					},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	result, err := fetchModelsDevPricingFromURL(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	googleModels := result["google"]
	if _, ok := googleModels["gemini-3-flash"]; !ok {
		t.Error("expected provider prefix to be stripped from model key")
	}
}

func TestFetchModelsDevPricingHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, err := fetchModelsDevPricingFromURL(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}
