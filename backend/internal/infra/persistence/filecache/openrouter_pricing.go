package filecache

import (
	"context"
	"fmt"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const (
	maxOpenRouterPricingCacheBytes = 16 << 20
	openRouterPricingCacheFile     = "openrouter-pricing.json"
	// legacyOpenRouterPricingCacheRelPath 是旧版本的存放位置，首次读取时迁移到 storage/catalogs。
	legacyOpenRouterPricingCacheRelPath = "admin/openrouter-model-pricing.json"
)

// OpenRouterPricingCache 使用本地文件持久化 OpenRouter 官方定价快照（storage/catalogs/openrouter-pricing.json）。
type OpenRouterPricingCache struct {
	file catalogFile
}

// NewOpenRouterPricingCache 创建位于 storage 根目录下的定价缓存仓储。
func NewOpenRouterPricingCache(storageRoot string) repository.OpenRouterPricingCacheRepository {
	return &OpenRouterPricingCache{
		file: newCatalogFile(storageRoot, openRouterPricingCacheFile, legacyOpenRouterPricingCacheRelPath, "openrouter pricing cache", maxOpenRouterPricingCacheBytes),
	}
}

// Load 读取受大小限制的定价快照。
func (c *OpenRouterPricingCache) Load(ctx context.Context) ([]byte, bool, error) {
	if c == nil {
		return nil, false, fmt.Errorf("openrouter pricing cache is not configured")
	}
	return c.file.load(ctx)
}

// Store 通过同目录临时文件和原子重命名写入定价快照。
func (c *OpenRouterPricingCache) Store(ctx context.Context, data []byte) error {
	if c == nil {
		return fmt.Errorf("openrouter pricing cache is not configured")
	}
	return c.file.store(ctx, data)
}
