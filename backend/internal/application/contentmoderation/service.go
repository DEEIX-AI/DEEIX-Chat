package contentmoderation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	contentRetention  = 30 * 24 * time.Hour
	metadataRetention = 90 * 24 * time.Hour
	cleanupInterval   = 6 * time.Hour
)

// EventEmitter publishes recovery-stream events for a run (optional).
type EventEmitter func(ctx context.Context, runID string, eventType string, payload map[string]any)

// CancelRun cancels in-flight upstream generation for a run.
type CancelRun func(ctx context.Context, runID string)

// OnBlocked is invoked after a run is marked blocked (e.g. sanitize recovery stream).
type OnBlocked func(ctx context.Context, runID string, info BlockInfo)

// PreparedImage is a resized moderation-ready image.
type PreparedImage struct {
	Data   []byte
	SHA256 string
	Mime   string
	Size   int64
	FileID string
}

// ImageLoader loads and prepares an image for moderation.
type ImageLoader func(ctx context.Context, userID uint, fileID string) (PreparedImage, error)

// OutputImageSource provides request-scoped image bytes and optional source metadata for moderation.
type OutputImageSource struct {
	FileID   string
	Data     []byte
	MimeType string
	SHA256   string
}

// ObjectStore abstracts isolated image storage.
type ObjectStore interface {
	Put(ctx context.Context, path string, data []byte, contentType string) error
	Open(ctx context.Context, path string) ([]byte, error)
	Delete(ctx context.Context, path string) error
}

// FileAccessController marks ordinary generated files inaccessible after a hit.
type FileAccessController interface {
	RevokeGeneratedFile(ctx context.Context, fileID string) error
	DeleteGeneratedFileArtifacts(ctx context.Context, fileID string) error
	RetryBlockedGeneratedFileDeletes(ctx context.Context, limit int) (int, error)
}

type pendingBlock struct {
	meta RunMeta
	info BlockInfo
}

// Service orchestrates config, workers, events, and run coordinators.
type Service struct {
	settingsRepo      repository.SettingsRepository
	repo              repository.ContentModerationRepository
	dataEncryptionKey string
	logger            *zap.Logger
	objectStore       ObjectStore
	fileAccess        FileAccessController
	imageLoader       ImageLoader
	emitEvent         EventEmitter
	cancelRun         CancelRun
	onBlocked         OnBlocked
	provider          Provider
	auditWriter       auditWriter

	configMu     sync.RWMutex
	cachedConfig *runtimeConfig
	cachedAt     time.Time

	workerMu       sync.Mutex
	taskQueue      chan *moderationTask
	workerSem      chan struct{} // fixed capacity maxPhysicalConcurrency; never replaced
	workerWake     chan struct{} // wakes workers waiting for a logical concurrency slot
	maxConcurrency int
	queueCapacity  int
	queuedCount    int // logical admission counter (paired with queueCapacity)
	activeWorkers  int // logical concurrency counter (paired with maxConcurrency)
	stopCh         chan struct{}
	wg             sync.WaitGroup

	coordMu      sync.Mutex
	coordinators map[string]*RunCoordinator

	pendingBlockMu sync.Mutex
	pendingBlocks  map[string]pendingBlock
}

// NewService creates the content moderation service.
func NewService(
	settingsRepo repository.SettingsRepository,
	repo repository.ContentModerationRepository,
	dataEncryptionKey string,
	logger *zap.Logger,
) *Service {
	s := &Service{
		settingsRepo:      settingsRepo,
		repo:              repo,
		dataEncryptionKey: dataEncryptionKey,
		logger:            logger,
		coordinators:      make(map[string]*RunCoordinator),
		pendingBlocks:     make(map[string]pendingBlock),
		stopCh:            make(chan struct{}),
		maxConcurrency:    defaultMaxConcurrency,
		queueCapacity:     defaultQueueCapacity,
	}
	s.taskQueue = make(chan *moderationTask, maxPhysicalQueueCapacity)
	// Fixed physical concurrency ceiling; logical maxConcurrency is enforced via activeWorkers.
	s.workerSem = make(chan struct{}, maxPhysicalConcurrency)
	s.workerWake = make(chan struct{}, maxPhysicalConcurrency)
	return s
}

func (s *Service) SetObjectStore(store ObjectStore)               { s.objectStore = store }
func (s *Service) SetFileAccessController(c FileAccessController) { s.fileAccess = c }
func (s *Service) SetImageLoader(loader ImageLoader)              { s.imageLoader = loader }
func (s *Service) SetEventEmitter(emit EventEmitter)              { s.emitEvent = emit }
func (s *Service) SetCancelRun(cancel CancelRun)                  { s.cancelRun = cancel }
func (s *Service) SetOnBlocked(fn OnBlocked)                      { s.onBlocked = fn }

// SetProvider injects the infrastructure adapter used for moderation calls.
func (s *Service) SetProvider(provider Provider) {
	if s == nil {
		return
	}
	s.provider = provider
}

// SetAuditWriter injects the operation-audit sink used for privileged review reads.
func (s *Service) SetAuditWriter(writer auditWriter) {
	if s != nil {
		s.auditWriter = writer
	}
}

// StartBackgroundWorkers starts the worker pool and cleanup loop.
// Worker 循环一次性按物理上限启动，有效并发由逻辑额度（maxConcurrency）控制。
func (s *Service) StartBackgroundWorkers(ctx context.Context) {
	if cfg, err := s.readRuntimeConfig(ctx); err == nil {
		s.resizeWorker(cfg.MaxConcurrency, cfg.QueueCapacity)
	}
	for range maxPhysicalConcurrency {
		s.wg.Add(1)
		go s.workerLoop(ctx)
	}
	s.wg.Add(1)
	go s.cleanupLoop(ctx)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// 从生命周期 ctx 派生，进程关停时可即时取消，避免 Stop 阻塞等待恢复任务。
		bg, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		s.runCleanup(bg)
		s.recoverPendingBlocks(bg)
		s.recoverStaleRuns(bg)
	}()
}

// Stop stops workers.
func (s *Service) Stop() {
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
	s.wg.Wait()
}

// maxPhysicalQueueCapacity is the fixed channel buffer. Configured queueCapacity
// is enforced as a logical limit so resize never swaps channels under workers.
const maxPhysicalQueueCapacity = 4096

// maxPhysicalConcurrency is the fixed workerSem capacity. Logical maxConcurrency
// is enforced with activeWorkers so resize never replaces the semaphore channel.
const maxPhysicalConcurrency = 64

// resizeWorker 调整逻辑并发额度与队列额度，并唤醒等待逻辑槽位的 worker。
func (s *Service) resizeWorker(maxConcurrency, queueCapacity int) {
	s.workerMu.Lock()
	defer s.workerMu.Unlock()
	if maxConcurrency < 1 {
		maxConcurrency = defaultMaxConcurrency
	}
	if maxConcurrency > maxPhysicalConcurrency {
		maxConcurrency = maxPhysicalConcurrency
	}
	if queueCapacity < 1 {
		queueCapacity = defaultQueueCapacity
	}
	if queueCapacity > maxPhysicalQueueCapacity {
		queueCapacity = maxPhysicalQueueCapacity
	}

	// Logical limits only - never replace taskQueue or workerSem.
	previousConcurrency := s.maxConcurrency
	s.queueCapacity = queueCapacity
	s.maxConcurrency = maxConcurrency

	if maxConcurrency > previousConcurrency {
		for i := previousConcurrency; i < maxConcurrency; i++ {
			select {
			case s.workerWake <- struct{}{}:
			default:
			}
		}
	}
}

// BeginRun registers a per-run coordinator. Persistent callers must claim the run before entry.
func (s *Service) BeginRun(ctx context.Context, meta RunMeta) *RunCoordinator {
	cfg, err := s.loadRuntimeConfig(ctx)
	if err != nil {
		// Configuration/storage failures are fail-open, but must remain observable.
		// Return a coordinator so the conversation run is durably settled as
		// failed_open instead of becoming indistinguishable from an intentionally
		// disabled policy.
		coord := newRunCoordinator(ctx, s, meta, runtimeConfig{Timeout: defaultTimeoutSeconds * time.Second})
		coord.failedOpen = true
		s.coordMu.Lock()
		s.coordinators[meta.RunID] = coord
		s.coordMu.Unlock()
		s.recordFailedOpen(
			ctx,
			meta,
			domaincm.DirectionInput,
			domaincm.ModalityText,
			domaincm.ErrorCodeConfigMissing,
			0,
		)
		s.bumpDailyStat(ctx, repository.DailyStatIncrement{
			Direction:    domaincm.DirectionInput,
			Modality:     domaincm.ModalityText,
			Result:       domaincm.ResultFailedOpen,
			CheckCount:   1,
			ContentItems: 1,
			FailureCount: 1,
		})
		s.logWarn("content_moderation_config_load_failed", zap.String("run_id", meta.RunID), zap.Error(err))
		return coord
	}
	if !cfg.Enabled || !cfg.Policy.Enabled() {
		return nil
	}
	coord := newRunCoordinator(ctx, s, meta, cfg)
	s.coordMu.Lock()
	s.coordinators[meta.RunID] = coord
	s.coordMu.Unlock()
	if !meta.Ephemeral {
		if err := s.repo.UpdateRunModeration(ctx, meta.RunID, domaincm.ModerationStatePending, "", "[]"); err != nil {
			s.logWarn("content_moderation_mark_pending_failed", zap.String("run_id", meta.RunID), zap.Error(err))
		}
	}
	return coord
}

// GetCoordinator returns an active coordinator if present.
func (s *Service) GetCoordinator(runID string) *RunCoordinator {
	s.coordMu.Lock()
	defer s.coordMu.Unlock()
	return s.coordinators[strings.TrimSpace(runID)]
}

func (s *Service) releaseCoordinator(runID string) {
	s.coordMu.Lock()
	delete(s.coordinators, strings.TrimSpace(runID))
	s.coordMu.Unlock()
}

func (s *Service) registerPendingBlock(meta RunMeta, info BlockInfo) {
	if s == nil || strings.TrimSpace(meta.RunID) == "" {
		return
	}
	s.pendingBlockMu.Lock()
	s.pendingBlocks[strings.TrimSpace(meta.RunID)] = pendingBlock{meta: meta, info: info}
	s.pendingBlockMu.Unlock()
}

func (s *Service) removePendingBlock(runID string) {
	if s == nil {
		return
	}
	s.pendingBlockMu.Lock()
	delete(s.pendingBlocks, strings.TrimSpace(runID))
	s.pendingBlockMu.Unlock()
}

func (s *Service) hasPendingBlock(runID string) bool {
	if s == nil {
		return false
	}
	s.pendingBlockMu.Lock()
	defer s.pendingBlockMu.Unlock()
	_, ok := s.pendingBlocks[strings.TrimSpace(runID)]
	return ok
}

func (s *Service) pendingBlockSnapshot() []pendingBlock {
	if s == nil {
		return nil
	}
	s.pendingBlockMu.Lock()
	defer s.pendingBlockMu.Unlock()
	items := make([]pendingBlock, 0, len(s.pendingBlocks))
	for _, item := range s.pendingBlocks {
		items = append(items, item)
	}
	return items
}

// HasActiveCoordinator reports whether a run still has an in-memory coordinator.
func (s *Service) HasActiveCoordinator(runID string) bool {
	return s.GetCoordinator(runID) != nil
}

// RecoverRunIfStale fail-opens a moderating run with no live coordinator.
func (s *Service) RecoverRunIfStale(ctx context.Context, runID string) {
	state, err := s.repo.GetRunModerationState(ctx, runID)
	if err != nil {
		return
	}
	if state != domaincm.ModerationStateModerating && state != domaincm.ModerationStatePending {
		return
	}
	if s.HasActiveCoordinator(runID) {
		return
	}
	if s.recoverKnownHit(ctx, runID) {
		return
	}
	s.recordFailedOpen(ctx, RunMeta{RunID: runID}, domaincm.DirectionOutput, domaincm.ModalityText, domaincm.ErrorCodeWorkerLost, 0)
	if err := s.repo.UpdateRunModeration(ctx, runID, domaincm.ModerationStateFailedOpen, "", "[]"); err != nil {
		s.logWarn("content_moderation_recover_run_mark_failed_open_failed", zap.String("run_id", runID), zap.Error(err))
	}
}

func providerConfigFromRuntime(cfg runtimeConfig) ProviderConfig {
	return ProviderConfig{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
		Timeout: cfg.Timeout,
	}
}

func (s *Service) encryptText(plaintext string) (string, error) {
	return secretbox.EncryptString(s.dataEncryptionKey, plaintext)
}

func (s *Service) decryptText(ciphertext string) (string, error) {
	return secretbox.DecryptString(s.dataEncryptionKey, ciphertext)
}

// encryptBytes encrypts arbitrary binary (isolated images) as a v1: base64 payload string.
func (s *Service) encryptBytes(plaintext []byte) (string, error) {
	return secretbox.Encrypt(s.dataEncryptionKey, plaintext)
}

// decryptBytes decrypts a payload produced by encryptBytes.
func (s *Service) decryptBytes(ciphertext string) ([]byte, error) {
	return secretbox.Decrypt(s.dataEncryptionKey, ciphertext)
}

func newPublicEventID() string {
	return "cme_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:24]
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func (s *Service) logWarn(msg string, fields ...zap.Field) {
	if s.logger != nil {
		s.logger.Warn(msg, fields...)
	}
}

func isolatedImagePath(eventPublicID string, index int, sha string) string {
	return fmt.Sprintf("moderation-isolated/%s/%d_%s.bin", eventPublicID, index, sha[:min(16, len(sha))])
}
