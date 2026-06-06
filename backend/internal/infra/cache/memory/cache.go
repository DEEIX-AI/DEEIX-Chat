package memory

import (
	"sync"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// Cache implements the repository cache interfaces for single-process deployments.
type Cache struct {
	mu  sync.Mutex
	ops uint64

	settings map[string]expiringString

	fileSeq      int64
	fileQueue    []repository.FileProcessingMessage
	fileInflight map[string]repository.FileProcessingMessage
	fileDLQ      []repository.FileProcessingMessage
	fileNotify   chan struct{}

	rag map[string]expiringRAG

	streams map[string]*generationStream

	upstreamCB   map[uint]*circuitState
	modelCB      map[string]*circuitState
	upstreamMeta map[uint]upstreamMetadata
	rateLimits   map[uint]rateLimitState
	keyCounters  map[uint]int64

	slidingHTTP map[string][]time.Time
	fixedHTTP   map[string]fixedWindowCounter
}

type expiringString struct {
	value     string
	expiresAt time.Time
}

type expiringRAG struct {
	chunks    []domainconversation.RAGChunk
	expiresAt time.Time
}

// New creates an in-memory cache backend.
func New() *Cache {
	return &Cache{
		settings:     map[string]expiringString{},
		fileInflight: map[string]repository.FileProcessingMessage{},
		fileNotify:   make(chan struct{}),
		rag:          map[string]expiringRAG{},
		streams:      map[string]*generationStream{},
		upstreamCB:   map[uint]*circuitState{},
		modelCB:      map[string]*circuitState{},
		upstreamMeta: map[uint]upstreamMetadata{},
		rateLimits:   map[uint]rateLimitState{},
		keyCounters:  map[uint]int64{},
		slidingHTTP:  map[string][]time.Time{},
		fixedHTTP:    map[string]fixedWindowCounter{},
	}
}

// NewSettingsCache returns the settings cache interface.
func NewSettingsCache(cache *Cache) repository.SettingsCacheRepository {
	return cache
}

// NewConversationCache returns the conversation cache interface.
func NewConversationCache(cache *Cache) repository.ConversationCacheRepository {
	return cache
}

// NewChannelCache returns the channel cache interface.
func NewChannelCache(cache *Cache) repository.ChannelCacheRepository {
	return cache
}

// NewRateLimiter returns a single-process HTTP rate limiter.
func NewRateLimiter(cache *Cache) *Cache {
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
