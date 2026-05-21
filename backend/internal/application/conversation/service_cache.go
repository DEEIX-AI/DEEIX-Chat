package conversation

import (
	"context"
	"fmt"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainmemory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/memory"
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

type cachedUserSetting struct {
	value     string
	valid     bool
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

// getUserSettingCached 从内存缓存读取用户设置，未命中时回退到 DB 查询。
func (s *Service) getUserSettingCached(ctx context.Context, userID uint, key string) (string, error) {
	cacheKey := fmt.Sprintf("%d:%s", userID, key)
	if v, ok := s.userSettingCache.Load(cacheKey); ok {
		entry := v.(*cachedUserSetting)
		if time.Now().Before(entry.expiresAt) {
			if !entry.valid {
				return "", fmt.Errorf("not found")
			}
			return entry.value, nil
		}
		s.userSettingCache.Delete(cacheKey)
	}
	val, err := s.repo.GetUserSettingValue(ctx, userID, key)
	if err != nil {
		s.userSettingCache.Store(cacheKey, &cachedUserSetting{valid: false, expiresAt: time.Now().Add(userSettingCacheTTL)})
		return "", err
	}
	s.userSettingCache.Store(cacheKey, &cachedUserSetting{value: val, valid: true, expiresAt: time.Now().Add(userSettingCacheTTL)})
	return val, nil
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
	go func() {
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
	}()
}

func (s *Service) cleanupExpiredInMemoryCaches(now time.Time) {
	s.snapshotCache.Range(func(key, value interface{}) bool {
		entry, ok := value.(*cachedSnapshot)
		if !ok || !now.Before(entry.expiresAt) {
			s.snapshotCache.Delete(key)
		}
		return true
	})
	s.userMemCache.Range(func(key, value interface{}) bool {
		entry, ok := value.(*cachedUserMemories)
		if !ok || !now.Before(entry.expiresAt) {
			s.userMemCache.Delete(key)
		}
		return true
	})
	s.userSettingCache.Range(func(key, value interface{}) bool {
		entry, ok := value.(*cachedUserSetting)
		if !ok || !now.Before(entry.expiresAt) {
			s.userSettingCache.Delete(key)
		}
		return true
	})
}
