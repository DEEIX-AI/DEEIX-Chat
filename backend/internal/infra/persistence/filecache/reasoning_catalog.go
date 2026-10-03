package filecache

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/modelcatalog/modelsdev"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const (
	maxReasoningCatalogCacheBytes = 16 << 20
	reasoningCatalogCacheRelPath  = "admin/models-dev-reasoning-catalog.json"
)

// ReasoningCatalogCache 以明文 JSON 持久化 models.dev 推理目录（与 OpenRouter 定价缓存同在 storage/admin），
// 重启后无需联网即可恢复最近一次同步结果。
type ReasoningCatalogCache struct {
	path string
}

// NewReasoningCatalogCache 创建位于 storage 根目录下的推理目录缓存仓储。
func NewReasoningCatalogCache(storageRoot string) repository.ReasoningCatalogCacheRepository {
	return &ReasoningCatalogCache{
		path: filepath.Join(filepath.Clean(storageRoot), filepath.FromSlash(reasoningCatalogCacheRelPath)),
	}
}

// Load 读取并解析目录快照；文件不存在时 found=false。
func (c *ReasoningCatalogCache) Load(ctx context.Context) (domainchannel.ReasoningCatalogSnapshot, bool, error) {
	data, found, err := c.load(ctx)
	if err != nil || !found {
		return domainchannel.ReasoningCatalogSnapshot{}, false, err
	}
	snapshot, err := modelsdev.DecodeReasoningSnapshot(data)
	if err != nil {
		return domainchannel.ReasoningCatalogSnapshot{}, false, fmt.Errorf("decode reasoning catalog cache: %w", err)
	}
	return snapshot, true, nil
}

// Store 把目录快照序列化为明文 JSON 并原子写入。
func (c *ReasoningCatalogCache) Store(ctx context.Context, snapshot domainchannel.ReasoningCatalogSnapshot) error {
	data, err := modelsdev.MarshalReasoningSnapshot(snapshot)
	if err != nil {
		return fmt.Errorf("encode reasoning catalog cache: %w", err)
	}
	return c.store(ctx, data)
}

func (c *ReasoningCatalogCache) load(ctx context.Context) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if c == nil || c.path == "" || c.path == "." {
		return nil, false, fmt.Errorf("reasoning catalog cache path is not configured")
	}
	file, err := os.Open(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("open reasoning catalog cache: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxReasoningCatalogCacheBytes+1))
	if err != nil {
		return nil, false, fmt.Errorf("read reasoning catalog cache: %w", err)
	}
	if len(data) > maxReasoningCatalogCacheBytes {
		return nil, false, fmt.Errorf("reasoning catalog cache exceeds %d bytes", maxReasoningCatalogCacheBytes)
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// store 通过同目录临时文件和原子重命名写入快照字节。
func (c *ReasoningCatalogCache) store(ctx context.Context, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil || c.path == "" || c.path == "." {
		return fmt.Errorf("reasoning catalog cache path is not configured")
	}
	if len(data) == 0 || len(data) > maxReasoningCatalogCacheBytes {
		return fmt.Errorf("reasoning catalog cache payload must be between 1 and %d bytes", maxReasoningCatalogCacheBytes)
	}

	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create reasoning catalog cache directory: %w", err)
	}
	temporary, err := os.CreateTemp(dir, filepath.Base(c.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create reasoning catalog cache temporary file: %w", err)
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
		return fmt.Errorf("set reasoning catalog cache permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write reasoning catalog cache: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close reasoning catalog cache: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, c.path); err != nil {
		return fmt.Errorf("replace reasoning catalog cache: %w", err)
	}
	committed = true
	return nil
}
