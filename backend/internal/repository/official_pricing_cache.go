package repository

import "context"

// OpenRouterPricingCacheRepository 定义 OpenRouter 官方定价快照的持久化边界。
//
// data 是 application 层拥有的序列化快照；仓储实现只负责原子读写，
// 不解释业务字段。缓存不存在时返回 found=false、err=nil。
type OpenRouterPricingCacheRepository interface {
	Load(ctx context.Context) (data []byte, found bool, err error)
	Store(ctx context.Context, data []byte) error
}
