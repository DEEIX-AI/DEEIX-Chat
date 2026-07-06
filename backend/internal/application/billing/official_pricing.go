package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const modelsDevAPIURL = "https://models.dev/api.json"

// modelsDevCost is the cost field in the models.dev API response.
type modelsDevCost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
}

type modelsDevModel struct {
	ID   string        `json:"id"`
	Name string        `json:"name"`
	Cost modelsDevCost `json:"cost"`
}

type modelsDevProvider struct {
	ID     string                    `json:"id"`
	Models map[string]modelsDevModel `json:"models"`
}

// vendorToProviderID maps DEEIX-Chat vendor names to models.dev provider IDs.
var vendorToProviderID = map[string]string{
	"openai":    "openai",
	"anthropic": "anthropic",
	"google":    "google",
	"deepseek":  "deepseek",
	"mistral":   "mistral",
	"xai":       "xai",
	"cohere":    "cohere",
	"baidu":     "baidu",
	"alibaba":   "alibaba",
	"zhipu":     "zhipu",
	"minimax":   "minimax",
	"moonshot":  "moonshotai",
	"bytedance": "bytedance",
}

// OfficialPricingEntry holds pricing fetched from an external source.
type OfficialPricingEntry struct {
	InputUSDPerMTokens      float64
	OutputUSDPerMTokens     float64
	CacheReadUSDPerMTokens  float64
	CacheWriteUSDPerMTokens float64
}

// FetchModelsDevPricing fetches pricing from models.dev/api.json and returns
// a map keyed by the model name used in the provider's catalog (without the
// "provider/" prefix), e.g. "gpt-5.4" → entry.
func FetchModelsDevPricing(ctx context.Context) (map[string]map[string]OfficialPricingEntry, error) {
	return fetchModelsDevPricingFromURL(ctx, modelsDevAPIURL)
}

func fetchModelsDevPricingFromURL(ctx context.Context, url string) (map[string]map[string]OfficialPricingEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "DEEIX-Chat/pricing-fetch")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models.dev api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models.dev returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 20*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var providers map[string]modelsDevProvider
	if err := json.Unmarshal(body, &providers); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	// result[vendor][modelName] = pricing
	result := make(map[string]map[string]OfficialPricingEntry)

	for deeixVendor, providerID := range vendorToProviderID {
		provider, ok := providers[providerID]
		if !ok {
			continue
		}
		vendorPricing := make(map[string]OfficialPricingEntry)
		for modelKey, model := range provider.Models {
			cost := model.Cost
			if cost.Input == 0 && cost.Output == 0 {
				continue
			}
			name := stripProviderPrefix(modelKey)
			vendorPricing[name] = OfficialPricingEntry{
				InputUSDPerMTokens:      cost.Input,
				OutputUSDPerMTokens:     cost.Output,
				CacheReadUSDPerMTokens:  cost.CacheRead,
				CacheWriteUSDPerMTokens: cost.CacheWrite,
			}
		}
		if len(vendorPricing) > 0 {
			result[deeixVendor] = vendorPricing
		}
	}

	return result, nil
}

func stripProviderPrefix(key string) string {
	if idx := strings.Index(key, "/"); idx >= 0 {
		return key[idx+1:]
	}
	return key
}
