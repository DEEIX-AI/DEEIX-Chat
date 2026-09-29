package memory

import (
	"sync"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// Cache 为单进程部署实现仓储层缓存接口。
type Cache struct {
	mu  sync.Mutex
	ops uint64

	settings            map[string]expiringString
	userSettings        map[string]expiringString
	userSettingVersions map[string]expiringString

	fileSeq             int64
	fileProcessingQueue fileQueueState
	fileEmbeddingQueue  fileQueueState

	rag map[string]expiringRAG

	streams      map[string]*generationStream
	streamNotify chan struct{}

	upstreamCB   map[uint]*circuitState
	modelCB      map[string]*circuitState
	upstreamMeta map[uint]upstreamMetadata
	rateLimits   map[routeRateLimitKey]rateLimitState
	keyCounters  map[uint]apiKeyCounter

	slidingHTTP map[string][]time.Time
	fixedHTTP   map[string]fixedWindowCounter

	providerAuthTransactions map[string]expiringProviderAuthTransaction
	providerAuthGrants       map[string]expiringProviderAuthGrant
}

type expiringString struct {
	value     string
	expiresAt time.Time
}

type expiringRAG struct {
	chunks    []domainconversation.RAGChunk
	expiresAt time.Time
}

// New 创建内存缓存后端。
func New() *Cache {
	return &Cache{
		settings:                 map[string]expiringString{},
		userSettings:             map[string]expiringString{},
		userSettingVersions:      map[string]expiringString{},
		fileProcessingQueue:      newFileQueueState(),
		fileEmbeddingQueue:       newFileQueueState(),
		rag:                      map[string]expiringRAG{},
		streams:                  map[string]*generationStream{},
		streamNotify:             make(chan struct{}),
		upstreamCB:               map[uint]*circuitState{},
		modelCB:                  map[string]*circuitState{},
		upstreamMeta:             map[uint]upstreamMetadata{},
		rateLimits:               map[routeRateLimitKey]rateLimitState{},
		keyCounters:              map[uint]apiKeyCounter{},
		slidingHTTP:              map[string][]time.Time{},
		fixedHTTP:                map[string]fixedWindowCounter{},
		providerAuthTransactions: map[string]expiringProviderAuthTransaction{},
		providerAuthGrants:       map[string]expiringProviderAuthGrant{},
	}
}

// NewSettingsCache 返回设置缓存接口。
func NewSettingsCache(cache *Cache) repository.SettingsCacheRepository {
	return cache
}

// NewConversationCache 返回会话缓存接口。
func NewConversationCache(cache *Cache) repository.ConversationCacheRepository {
	return cache
}

// NewChannelCache 返回渠道缓存接口。
func NewChannelCache(cache *Cache) repository.ChannelCacheRepository {
	return cache
}

// NewRateLimiter 返回单进程 HTTP 限流器。
func NewRateLimiter(cache *Cache) *Cache {
	return cache
}

// NewProviderAuthBridge 返回单进程的 provider auth bridge 存储。
func NewProviderAuthBridge(cache *Cache) repository.ProviderAuthBridgeRepository {
	return cache
}

func ttlFromNow(ttl time.Duration) time.Time {
	if ttl <= 0 {
		return time.Now().Add(time.Minute)
	}
	return time.Now().Add(ttl)
}

func expired(t time.Time) bool {
	return !t.IsZero() && time.Now().After(t)
}
