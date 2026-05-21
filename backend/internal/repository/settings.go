package repository

import (
	"context"

	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
)

// SettingsRepository 定义系统配置读写能力。
type SettingsRepository interface {
	ListAll(ctx context.Context) ([]domainsettings.SystemSetting, error)
	ListByNamespace(ctx context.Context, namespace string) ([]domainsettings.SystemSetting, error)
	Upsert(ctx context.Context, items []domainsettings.SystemSetting) error
	UpsertWithDescription(ctx context.Context, items []domainsettings.SystemSetting) error
	Delete(ctx context.Context, namespace, key string) error
}

// SettingsCacheRepository 封装配置项的缓存能力，屏蔽 Redis 细节。
type SettingsCacheRepository interface {
	// Set 将指定 namespace/key 的配置值写入缓存。
	Set(ctx context.Context, namespace, key, value string) error
	// Del 删除指定 namespace/key 的缓存项。
	Del(ctx context.Context, namespace, key string) error
}
