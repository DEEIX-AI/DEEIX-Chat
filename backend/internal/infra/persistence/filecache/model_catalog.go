package filecache

import (
	"context"
	"errors"
	"fmt"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/modelcatalog/modelsdev"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const (
	maxModelCatalogCacheBytes = 16 << 20
	modelCatalogCacheFile     = "models-dev.json"
	// legacyModelCatalogCacheRelPath 是旧版本的存放位置，首次读取时迁移到 storage/catalogs。
	legacyModelCatalogCacheRelPath = "admin/models-dev-reasoning-catalog.json"
)

// ModelCatalogCache 以明文 JSON 持久化 models.dev 模型目录（storage/catalogs/models-dev.json），
// 重启后无需联网即可恢复最近一次同步结果。
type ModelCatalogCache struct {
	file catalogFile
}

// NewModelCatalogCache 创建位于 storage 根目录下的模型目录缓存仓储。
func NewModelCatalogCache(storageRoot string) repository.ModelCatalogCacheRepository {
	return &ModelCatalogCache{
		file: newCatalogFile(storageRoot, modelCatalogCacheFile, legacyModelCatalogCacheRelPath, "model catalog cache", maxModelCatalogCacheBytes),
	}
}

// Load 读取并解析目录快照；文件不存在或为旧格式时 found=false。
func (c *ModelCatalogCache) Load(ctx context.Context) (domainchannel.ModelCatalogSnapshot, bool, error) {
	if c == nil {
		return domainchannel.ModelCatalogSnapshot{}, false, fmt.Errorf("model catalog cache is not configured")
	}
	data, found, err := c.file.load(ctx)
	if err != nil || !found {
		return domainchannel.ModelCatalogSnapshot{}, false, err
	}
	snapshot, err := modelsdev.DecodeSnapshot(data)
	if errors.Is(err, modelsdev.ErrSnapshotOutdated) {
		// 升级后旧格式缓存视同缺失，由内置快照重新写入，不作为错误告警。
		return domainchannel.ModelCatalogSnapshot{}, false, nil
	}
	if err != nil {
		return domainchannel.ModelCatalogSnapshot{}, false, fmt.Errorf("decode model catalog cache: %w", err)
	}
	return snapshot, true, nil
}

// Store 把目录快照序列化为明文 JSON 并原子写入。
func (c *ModelCatalogCache) Store(ctx context.Context, snapshot domainchannel.ModelCatalogSnapshot) error {
	if c == nil {
		return fmt.Errorf("model catalog cache is not configured")
	}
	data, err := modelsdev.MarshalSnapshot(snapshot)
	if err != nil {
		return fmt.Errorf("encode model catalog cache: %w", err)
	}
	return c.file.store(ctx, data)
}
