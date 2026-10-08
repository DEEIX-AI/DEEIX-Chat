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

// EventEmitter 为运行发布恢复流事件（可选）。
type EventEmitter func(ctx context.Context, runID string, eventType string, payload map[string]any)

// CancelRun 取消某次运行尚未完成的上游生成。
type CancelRun func(ctx context.Context, runID string)

// OnBlocked 在运行被标记为拦截后调用（例如清理恢复流）。
type OnBlocked func(ctx context.Context, runID string, info BlockInfo)

// PreparedImage 是已缩放、可用于审核的图片。
type PreparedImage struct {
	Data   []byte
	SHA256 string
	Mime   string
	Size   int64
	FileID string
}

// ImageLoader 加载并准备用于审核的图片。
type ImageLoader func(ctx context.Context, userID uint, fileID string) (PreparedImage, error)

// OutputImageSource 提供请求作用域内的图片字节及可选的来源元数据，用于审核。
type OutputImageSource struct {
	FileID   string
	Data     []byte
	MimeType string
	SHA256   string
}

// ObjectStore 抽象隔离图片存储。
type ObjectStore interface {
	Put(ctx context.Context, path string, data []byte, contentType string) error
	Open(ctx context.Context, path string) ([]byte, error)
	Delete(ctx context.Context, path string) error
}

// FileAccessController 在命中后将普通生成文件标记为不可访问。
type FileAccessController interface {
	RevokeGeneratedFile(ctx context.Context, fileID string) error
	DeleteGeneratedFileArtifacts(ctx context.Context, fileID string) error
	RetryBlockedGeneratedFileDeletes(ctx context.Context, limit int) (int, error)
}

type pendingBlock struct {
	meta RunMeta
	info BlockInfo
}

// Service 编排配置、worker、事件与运行协调器。
type Service struct {
	settingsRepo repository.SettingsRepository
	repo         repository.ContentModerationRepository
	keyring      *secretbox.Keyring
	logger       *zap.Logger
	objectStore  ObjectStore
	fileAccess   FileAccessController
	imageLoader  ImageLoader
	emitEvent    EventEmitter
	cancelRun    CancelRun
	onBlocked    OnBlocked
	provider     Provider
	auditWriter  auditWriter

	configMu     sync.RWMutex
	cachedConfig *runtimeConfig
	cachedAt     time.Time

	workerMu       sync.Mutex
	taskQueue      chan *moderationTask
	workerSem      chan struct{} // 固定容量 maxPhysicalConcurrency；永不替换
	workerWake     chan struct{} // 唤醒等待逻辑并发槽位的 worker
	maxConcurrency int
	queueCapacity  int
	queuedCount    int // 逻辑准入计数器（与 queueCapacity 配合）
	activeWorkers  int // 逻辑并发计数器（与 maxConcurrency 配合）
	stopCh         chan struct{}
	wg             sync.WaitGroup

	coordMu      sync.Mutex
	coordinators map[string]*RunCoordinator

	pendingBlockMu sync.Mutex
	pendingBlocks  map[string]pendingBlock
}

// NewService 创建内容审核服务。
func NewService(
	settingsRepo repository.SettingsRepository,
	repo repository.ContentModerationRepository,
	keyring *secretbox.Keyring,
	logger *zap.Logger,
) *Service {
	s := &Service{
		settingsRepo:   settingsRepo,
		repo:           repo,
		keyring:        keyring,
		logger:         logger,
		coordinators:   make(map[string]*RunCoordinator),
		pendingBlocks:  make(map[string]pendingBlock),
		stopCh:         make(chan struct{}),
		maxConcurrency: defaultMaxConcurrency,
		queueCapacity:  defaultQueueCapacity,
	}
	s.taskQueue = make(chan *moderationTask, maxPhysicalQueueCapacity)
	// 固定的物理并发上限；逻辑 maxConcurrency 通过 activeWorkers 约束。
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

// SetProvider 注入用于审核调用的基础设施适配器。
func (s *Service) SetProvider(provider Provider) {
	if s == nil {
		return
	}
	s.provider = provider
}

// SetAuditWriter 注入用于特权审核读取的操作审计输出端。
func (s *Service) SetAuditWriter(writer auditWriter) {
	if s != nil {
		s.auditWriter = writer
	}
}

// StartBackgroundWorkers 启动 worker 池与清理循环。
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

// Stop 停止 worker。
func (s *Service) Stop() {
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
	s.wg.Wait()
}

// maxPhysicalQueueCapacity 是固定的 channel 缓冲大小。配置的 queueCapacity
// 作为逻辑上限约束，因此调整大小时绝不会在 worker 运行中替换 channel。
const maxPhysicalQueueCapacity = 4096

// maxPhysicalConcurrency 是固定的 workerSem 容量。逻辑 maxConcurrency
// 通过 activeWorkers 约束，因此调整大小时绝不会替换信号量 channel。
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

	// 仅调整逻辑上限——绝不替换 taskQueue 或 workerSem。
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

// BeginRun 注册单次运行的协调器。持久化调用方必须在进入前认领该运行。
func (s *Service) BeginRun(ctx context.Context, meta RunMeta) *RunCoordinator {
	cfg, err := s.loadRuntimeConfig(ctx)
	if err != nil {
		// 配置/存储失败按 fail-open 处理，但必须保持可观测。
		// 仍返回协调器，使会话运行被持久化为 failed_open，避免与有意关闭审核策略的情况混淆。
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

// GetCoordinator 返回 runID 对应的活动协调器；不存在时返回 nil。
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

// HasActiveCoordinator 报告运行是否仍有内存中的协调器。
func (s *Service) HasActiveCoordinator(runID string) bool {
	return s.GetCoordinator(runID) != nil
}

// RecoverRunIfStale 对没有存活协调器的审核中运行执行 fail-open。
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
	return s.keyring.EncryptString(plaintext)
}

func (s *Service) decryptText(ciphertext string) (string, error) {
	return s.keyring.DecryptString(ciphertext)
}

// encryptBytes 将任意二进制数据（隔离图片）加密为 v1: base64 载荷字符串。
func (s *Service) encryptBytes(plaintext []byte) (string, error) {
	return s.keyring.Encrypt(plaintext)
}

// decryptBytes 解密由 encryptBytes 生成的载荷。
func (s *Service) decryptBytes(ciphertext string) ([]byte, error) {
	return s.keyring.Decrypt(ciphertext)
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
