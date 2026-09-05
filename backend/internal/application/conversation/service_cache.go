package conversation

import (
	"context"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainmemory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
	"go.uber.org/zap"
)

const (
	// snapshotCacheTTL：Snapshot 仅在压缩后变化，缓存 2 分钟可大幅减少 DB 查询。
	snapshotCacheTTL = 2 * time.Minute
	// userMemCacheTTL：用户记忆在会话期间极少变化，缓存 3 分钟。
	userMemCacheTTL = 3 * time.Minute
	// userSettingCacheTTL：用户设置在会话期间几乎不变，缓存 10 分钟。
	userSettingCacheTTL = 10 * time.Minute
	// inMemoryCacheSweepInterval：主动清理过期内存缓存，避免冷 key 长期驻留。
	inMemoryCacheSweepInterval = time.Minute
)

type cachedSnapshot struct {
	snapshot  *model.ContextSnapshot
	expiresAt time.Time
}

type cachedUserMemories struct {
	memories  []domainmemory.UserMemory
	expiresAt time.Time
}

// getCachedSnapshot 从内存缓存读取最新 Snapshot，未命中时回退到 DB 查询。
func (s *Service) getCachedSnapshot(ctx context.Context, conversationID uint) (*model.ContextSnapshot, error) {
	if v, ok := s.snapshotCache.Load(conversationID); ok {
		entry := v.(*cachedSnapshot)
		if time.Now().Before(entry.expiresAt) {
			return entry.snapshot, nil
		}
		s.snapshotCache.Delete(conversationID)
	}
	snap, err := s.compactSvc.GetLatestSnapshot(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	s.snapshotCache.Store(conversationID, &cachedSnapshot{
		snapshot:  snap,
		expiresAt: time.Now().Add(snapshotCacheTTL),
	})
	return snap, nil
}

// invalidateSnapshotCache 压缩完成后主动清除缓存，确保下次请求拿到最新 Snapshot。
func (s *Service) invalidateSnapshotCache(conversationID uint) {
	s.snapshotCache.Delete(conversationID)
}

// getUserSettingCached 从共享缓存读取用户设置，未命中或缓存不可用时回退到 DB。
func (s *Service) getUserSettingCached(ctx context.Context, userID uint, key string) (string, error) {
	if s.cache == nil {
		return s.repo.GetUserSettingValue(ctx, userID, key)
	}
	version, versionErr := s.cache.GetUserSettingCacheVersion(ctx, userID, key, userSettingCacheTTL)
	if versionErr == nil {
		if value, ok, err := s.cache.GetUserSettingCache(ctx, userID, key, version); err == nil && ok {
			return value, nil
		} else if err != nil && s.logger != nil {
			s.logger.Warn("user_setting_cache_read_failed", zap.Uint("user_id", userID), zap.String("key", key), zap.Error(err))
		}
	} else if s.logger != nil {
		s.logger.Warn("user_setting_cache_version_read_failed", zap.Uint("user_id", userID), zap.String("key", key), zap.Error(versionErr))
	}
	value, err := s.repo.GetUserSettingValue(ctx, userID, key)
	if err != nil {
		return "", err
	}
	if versionErr == nil {
		if err := s.cache.SetUserSettingCache(ctx, userID, key, version, value, userSettingCacheTTL); err != nil && s.logger != nil {
			s.logger.Warn("user_setting_cache_write_failed", zap.Uint("user_id", userID), zap.String("key", key), zap.Error(err))
		}
	}
	return value, nil
}

// RefreshUserSettingCache 推进指定设置的共享缓存版本，并用数据库中已提交的当前值回填新版本。
// 版本化数据键保证了在途旧读取与并发更新都无法覆盖当前值。
func (s *Service) RefreshUserSettingCache(ctx context.Context, userID uint, keys []string) {
	if s.cache == nil {
		return
	}
	versions := make(map[string]string, len(keys))
	refreshKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		version, err := s.cache.AdvanceUserSettingCacheVersion(ctx, userID, key, userSettingCacheTTL)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("user_setting_cache_version_advance_failed", zap.Uint("user_id", userID), zap.String("key", key), zap.Error(err))
			}
			continue
		}
		versions[key] = version
		refreshKeys = append(refreshKeys, key)
	}
	values, err := s.repo.GetUserSettingValues(ctx, userID, refreshKeys)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("user_setting_cache_refresh_load_failed", zap.Uint("user_id", userID), zap.Error(err))
		}
		return
	}
	for _, key := range refreshKeys {
		if err := s.cache.SetUserSettingCache(ctx, userID, key, versions[key], values[key], userSettingCacheTTL); err != nil && s.logger != nil {
			s.logger.Warn("user_setting_cache_refresh_write_failed", zap.Uint("user_id", userID), zap.String("key", key), zap.Error(err))
		}
	}
}

// getCachedUserMemories 从内存缓存读取用户长期记忆，未命中时回退到 DB 查询。
func (s *Service) getCachedUserMemories(ctx context.Context, userID uint) ([]domainmemory.UserMemory, error) {
	if v, ok := s.userMemCache.Load(userID); ok {
		entry := v.(*cachedUserMemories)
		if time.Now().Before(entry.expiresAt) {
			return entry.memories, nil
		}
		s.userMemCache.Delete(userID)
	}
	mems, err := s.memoryRecorder.ListUserMemories(ctx, userID)
	if err != nil {
		return nil, err
	}
	s.userMemCache.Store(userID, &cachedUserMemories{
		memories:  mems,
		expiresAt: time.Now().Add(userMemCacheTTL),
	})
	return mems, nil
}

func (s *Service) startInMemoryCacheCleanupWorker(ctx context.Context) {
	if s == nil {
		return
	}
	background.Go(s.logger, "in_memory_cache_cleanup", func() {
		ticker := time.NewTicker(inMemoryCacheSweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				s.cleanupExpiredInMemoryCaches(now)
			}
		}
	})
}

func (s *Service) cleanupExpiredInMemoryCaches(now time.Time) {
	s.snapshotCache.Range(func(key, value any) bool {
		entry, ok := value.(*cachedSnapshot)
		if !ok || !now.Before(entry.expiresAt) {
			s.snapshotCache.Delete(key)
		}
		return true
	})
	s.userMemCache.Range(func(key, value any) bool {
		entry, ok := value.(*cachedUserMemories)
		if !ok || !now.Before(entry.expiresAt) {
			s.userMemCache.Delete(key)
		}
		return true
	})
}
