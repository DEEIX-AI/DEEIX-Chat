package repository

import (
	"context"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
)

// ReasoningCatalogCacheRepository 定义 models.dev 推理目录快照的持久化边界，用于重启后无需联网即可恢复
// 最近一次同步结果。缓存不存在时返回 found=false、err=nil；内容损坏时返回错误，由调用方回退到其他来源。
type ReasoningCatalogCacheRepository interface {
	Load(ctx context.Context) (snapshot domainchannel.ReasoningCatalogSnapshot, found bool, err error)
	Store(ctx context.Context, snapshot domainchannel.ReasoningCatalogSnapshot) error
}
