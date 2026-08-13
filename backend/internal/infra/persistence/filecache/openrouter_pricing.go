package filecache

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const (
	maxOpenRouterPricingCacheBytes = 16 << 20
	openRouterPricingCacheRelPath  = "admin/openrouter-model-pricing.json"
)

// OpenRouterPricingCache 使用本地文件持久化 OpenRouter 官方定价快照。
type OpenRouterPricingCache struct {
	path string
}

// NewOpenRouterPricingCache 创建位于 storage 根目录下的定价缓存仓储。
func NewOpenRouterPricingCache(storageRoot string) repository.OpenRouterPricingCacheRepository {
	return &OpenRouterPricingCache{
		path: filepath.Join(filepath.Clean(storageRoot), filepath.FromSlash(openRouterPricingCacheRelPath)),
	}
}

// Load 读取受大小限制的定价快照。
func (c *OpenRouterPricingCache) Load(ctx context.Context) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if c == nil || c.path == "" || c.path == "." {
		return nil, false, fmt.Errorf("openrouter pricing cache path is not configured")
	}
	file, err := os.Open(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("open openrouter pricing cache: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxOpenRouterPricingCacheBytes+1))
	if err != nil {
		return nil, false, fmt.Errorf("read openrouter pricing cache: %w", err)
	}
	if len(data) > maxOpenRouterPricingCacheBytes {
		return nil, false, fmt.Errorf("openrouter pricing cache exceeds %d bytes", maxOpenRouterPricingCacheBytes)
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// Store 通过同目录临时文件和原子重命名写入定价快照。
func (c *OpenRouterPricingCache) Store(ctx context.Context, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil || c.path == "" || c.path == "." {
		return fmt.Errorf("openrouter pricing cache path is not configured")
	}
	if len(data) == 0 || len(data) > maxOpenRouterPricingCacheBytes {
		return fmt.Errorf("openrouter pricing cache payload must be between 1 and %d bytes", maxOpenRouterPricingCacheBytes)
	}

	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create openrouter pricing cache directory: %w", err)
	}
	temporary, err := os.CreateTemp(dir, filepath.Base(c.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create openrouter pricing cache temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := temporary.Chmod(0o644); err != nil {
		return fmt.Errorf("set openrouter pricing cache permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write openrouter pricing cache: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close openrouter pricing cache: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, c.path); err != nil {
		return fmt.Errorf("replace openrouter pricing cache: %w", err)
	}
	committed = true
	return nil
}
