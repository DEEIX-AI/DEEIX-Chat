package embedding

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/extraction"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/filetype"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/imageutil"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	portembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/embeddingutil"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/tokenestimate"
	"go.uber.org/zap"
)

const (
	embeddingErrorLimit           = 255
	embeddingFailureMessage       = "向量化失败，请稍后重试。"
	embeddingUnavailableMessage   = "向量化服务暂时不可用，请稍后重试。"
	embeddingNotConfiguredMessage = "向量化服务尚未配置。"
	embeddingTimeoutMessage       = "向量化超时，请稍后重试。"
	embeddingCanceledMessage      = "向量化已取消。"
	embeddingNoTextMessage        = "无法读取文件提取文本。"
	embeddingEmptyChunksMessage   = "文件没有可用于向量化的内容。"
	embeddingConfigurationChanged = "向量化配置已变更，请重新提交任务。"
	embeddingImageTooLargeMessage = "图片过大，缩放后仍超过向量化上限。"
	embeddingImageFormatMessage   = "图片格式不受当前向量化服务支持。"
	embeddingModalityMessage      = "当前向量化协议不支持该输入类型。"
	embeddingStalledMessage       = "向量化任务超时或被中断，请重试。"
)

// ErrorSummary 返回长度有限、用户可见的描述，不暴露
// 提供方响应、URL、凭据或内部存储细节。
func ErrorSummary(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return embeddingCanceledMessage
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return embeddingTimeoutMessage
	}
	if errors.Is(err, ErrEmbeddingServiceNotConfigured) {
		return embeddingNotConfiguredMessage
	}
	if errors.Is(err, ErrEmbeddingServiceUnavailable) {
		return embeddingUnavailableMessage
	}
	if errors.Is(err, errNoExtractableText) {
		return embeddingNoTextMessage
	}
	if errors.Is(err, errEmptyChunks) {
		return embeddingEmptyChunksMessage
	}
	if errors.Is(err, errEmbeddingConfigurationChanged) {
		return embeddingConfigurationChanged
	}
	if errors.Is(err, errImageTooLarge) {
		return embeddingImageTooLargeMessage
	}
	if errors.Is(err, errImageFormatUnsupported) {
		return embeddingImageFormatMessage
	}
	if errors.Is(err, portembedding.ErrModalityUnsupported) {
		return embeddingModalityMessage
	}
	return embeddingFailureMessage
}

const (
	WorkerConcurrency = 4
	MaxTargetedFiles  = 100
	// JobTimeout 是单个文件一次向量化执行的时长上限。所有执行入口都必须遵守，
	// 停滞判定依赖它：超过上限仍停在 processing 的任务只可能是执行进程已退出。
	JobTimeout = 5 * time.Minute
)

const (
	// processingStallTimeout 留出一分钟余量，避免把刚好接近上限的执行误判为停滞。
	processingStallTimeout = JobTimeout + time.Minute
	// queueStallTimeout 是任务排队后允许等待的时长；超过后视为队列消息丢失（如进程重启、内存队列），允许重新提交。
	// 误判只会产生一次重复投递，执行结果幂等。
	queueStallTimeout = 30 * time.Minute
	// stallSweepInterval 是后台回收停滞任务的巡检间隔。
	stallSweepInterval = time.Minute
)

const (
	SkipReasonNotFound     = "not_found"
	SkipReasonNotReady     = "not_ready"
	SkipReasonUnsupported  = "unsupported"
	SkipReasonAlreadyReady = "already_ready"
	SkipReasonProcessing   = "processing"
	SkipReasonQueueBusy    = "queue_busy"
	SkipReasonSubmitFailed = "submit_failed"
	ReasonOutdatedIndex    = "outdated_index"
)

type TargetedFileSkip struct {
	FileID string
	Reason string
}

type TargetedSubmissionResult struct {
	SubmittedFileIDs []string
	Skipped          []TargetedFileSkip
}

type TargetedJob struct {
	FileID             string
	UserID             uint
	EmbeddingSignature string
	EmbeddingHost      string
	// Reprocess 表示文件还没有可用文本（处理失败、无文本或排队停滞），必须先重新走处理流水线提取，
	// 提取结果保存后再向量化；否则直接按已保存的文本向量化。
	Reprocess bool
	// Reclaimed 表示消息在原消费者停止续租后被重新领取，只有这种情况才能接手停在 processing 的文件。
	Reclaimed bool
}

type TargetedSubmissionPlan struct {
	Jobs    []TargetedJob
	Skipped []TargetedFileSkip
}

type FileVectorizationCapability struct {
	CanVectorize bool
	Reason       string
}

// Service 封装文件 embedding 执行与状态管理能力。
type Service struct {
	cfg         *config.Runtime
	repo        repository.EmbeddingRepository
	extractSvc  *extraction.Service
	embedClient EmbeddingClient
	logger      *zap.Logger
	workSlots   chan struct{}
	reindexJobs chan reindexJob
	reindexMu   sync.Mutex
	reindexing  bool

	vectorStoreMu        sync.Mutex
	vectorStoreChecked   bool
	vectorStoreAvailable bool
}

// EmbeddingClient 调用外部服务将文本批量转换为向量。
type EmbeddingClient interface {
	CallAPI(ctx context.Context, input portembedding.Request) ([][]float32, error)
}

// NewServiceWithRuntime 创建使用运行时配置容器的 embedding 服务。
func NewServiceWithRuntime(cfg *config.Runtime, repo repository.EmbeddingRepository, extractSvc *extraction.Service, embedClient EmbeddingClient, logger *zap.Logger) *Service {
	return &Service{
		cfg:         cfg,
		repo:        repo,
		extractSvc:  extractSvc,
		embedClient: embedClient,
		logger:      logger,
		workSlots:   make(chan struct{}, WorkerConcurrency),
		reindexJobs: make(chan reindexJob, 1),
	}
}

// StartBackgroundWorkers 启动后台重建任务的常驻执行协程与停滞任务巡检；ctx 取消后不再领取新任务。
func (s *Service) StartBackgroundWorkers(ctx context.Context) {
	if s == nil || ctx == nil {
		return
	}
	background.Go(s.logger, "embedding_reindex_dispatch", func() {
		for {
			select {
			case <-ctx.Done():
				return
			case job := <-s.reindexJobs:
				s.runReindex(ctx, job)
			}
		}
	})
	background.Go(s.logger, "embedding_stall_sweep", func() {
		ticker := time.NewTicker(stallSweepInterval)
		defer ticker.Stop()
		// 启动时先巡检一次，回收上次进程退出时遗留在 processing 的任务。
		s.sweepStalledTasks(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.sweepStalledTasks(ctx)
			}
		}
	})
}

// Available 返回当前对话 RAG 检索能力是否可用及原因。
func (s *Service) Available(ctx context.Context) (bool, string) {
	cfg := s.snapshot()
	if !cfg.RAGEnabled {
		return false, "rag_disabled"
	}
	available, reason, _ := s.indexingAvailable(ctx, cfg)
	return available, reason
}

// IndexingAvailable 返回文件向量索引维护能力是否可用及原因。
func (s *Service) IndexingAvailable(ctx context.Context) (bool, string) {
	available, reason, _ := s.indexingAvailable(ctx, s.snapshot())
	return available, reason
}

func (s *Service) indexingAvailable(ctx context.Context, cfg config.Config) (bool, string, error) {
	if !cfg.EmbeddingEnabled {
		return false, "embedding_disabled", nil
	}
	if strings.TrimSpace(cfg.RAGModel) == "" {
		return false, "embedding_model_missing", nil
	}
	if strings.TrimSpace(cfg.EmbeddingHost) == "" {
		return false, "embedding_host_missing", nil
	}
	if s.embedClient == nil {
		return false, "embedding_client_missing", nil
	}
	if s.repo == nil {
		return false, "vector_store_unavailable", nil
	}
	available, err := s.cachedVectorStoreAvailable(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("embedding vector store availability check failed", zap.Error(err))
		}
		return false, "vector_store_error", err
	}
	if !available {
		return false, "vector_store_unavailable", nil
	}
	return true, "available", nil
}

// cachedVectorStoreAvailable 缓存随进程启动确定的向量存储结构状态。
// 配置项仍由 indexingAvailable 每次读取运行时快照，只有昂贵且在运行期间不应变化的
// 扩展、字段和索引结构检查会被缓存；失败结果不会缓存，避免瞬时数据库错误污染后续请求。
func (s *Service) cachedVectorStoreAvailable(ctx context.Context) (bool, error) {
	s.vectorStoreMu.Lock()
	defer s.vectorStoreMu.Unlock()
	if s.vectorStoreChecked {
		return s.vectorStoreAvailable, nil
	}
	available, err := s.repo.VectorStoreAvailable(ctx)
	if err != nil {
		return false, err
	}
	s.vectorStoreAvailable = available
	s.vectorStoreChecked = true
	return available, nil
}

// ShouldTrigger 判断当前文件是否应触发 embedding。
func (s *Service) ShouldTrigger(fileObj domainconversation.FileObject) bool {
	cfg := s.snapshot()
	if !cfg.EmbeddingEnabled || !cfg.EmbedTriggerOnUpload || strings.TrimSpace(cfg.RAGModel) == "" || strings.TrimSpace(cfg.EmbeddingHost) == "" {
		return false
	}
	return canEmbedFile(cfg, fileObj)
}

func canEmbedFile(cfg config.Config, fileObj domainconversation.FileObject) bool {
	if strings.TrimSpace(fileObj.StoragePath) == "" || strings.ToLower(strings.TrimSpace(fileObj.Status)) != "active" {
		return false
	}
	return supportsEmbeddingSource(fileObj, cfg)
}

// MaybeTrigger 在满足条件时异步触发 embedding。
func (s *Service) MaybeTrigger(ctx context.Context, fileObj domainconversation.FileObject) {
	if !s.ShouldTrigger(fileObj) {
		return
	}
	background.Go(s.logger, "embedding_process_file", func() {
		ctx, cancel := background.WithTimeout(ctx, JobTimeout)
		defer cancel()
		if available, _, _ := s.indexingAvailable(ctx, s.snapshot()); !available {
			return
		}
		if err := s.ProcessFile(ctx, fileObj); err != nil && s.logger != nil {
			s.logger.Warn("embedding_failed",
				zap.String("file_id", fileObj.FileID),
				zap.Error(err),
			)
		}
	})
}

// PlanFiles 校验当前用户指定文件并生成向量化任务计划。
// 任务认领与投递由 processing 应用服务逐项完成，避免批量预认领后因中途失败遗留 processing 状态。
func (s *Service) PlanFiles(ctx context.Context, userID uint, fileIDs []string) (TargetedSubmissionPlan, error) {
	return s.planFiles(ctx, fileIDs, false, func(ids []string) ([]domainconversation.FileObject, error) {
		return s.repo.GetActiveFileObjectsByIDs(ctx, userID, ids)
	})
}

// PlanAdminFiles 为管理员跨用户重试指定文件生成向量化任务计划；任务仍归属文件所有者。
// 没有可用文本的文件会规划为重新处理任务，先重新提取再向量化。
func (s *Service) PlanAdminFiles(ctx context.Context, fileIDs []string) (TargetedSubmissionPlan, error) {
	return s.planFiles(ctx, fileIDs, true, func(ids []string) ([]domainconversation.FileObject, error) {
		return s.repo.GetActiveFileObjectsByFileIDs(ctx, ids)
	})
}

func (s *Service) planFiles(
	ctx context.Context,
	fileIDs []string,
	allowReprocess bool,
	loadFiles func(ids []string) ([]domainconversation.FileObject, error),
) (TargetedSubmissionPlan, error) {
	plan := TargetedSubmissionPlan{
		Jobs:    []TargetedJob{},
		Skipped: []TargetedFileSkip{},
	}
	normalizedIDs := normalizeTargetedFileIDs(fileIDs)
	if len(normalizedIDs) > MaxTargetedFiles {
		return plan, ErrTooManyTargetedFiles
	}
	if len(normalizedIDs) == 0 {
		return plan, nil
	}

	cfg := s.snapshot()
	available, reason, err := s.indexingAvailable(ctx, cfg)
	if !available {
		return plan, embeddingAvailabilityError(reason, err)
	}

	files, err := loadFiles(normalizedIDs)
	if err != nil {
		return plan, err
	}
	filesByID := make(map[string]domainconversation.FileObject, len(files))
	for i := range files {
		filesByID[files[i].FileID] = files[i]
	}

	embeddingSignature := configuredModelSignature(cfg)
	embeddingHost := strings.TrimRight(strings.TrimSpace(cfg.EmbeddingHost), "/")
	stalledBefore := queueStalledBefore(time.Now())
	for _, fileID := range normalizedIDs {
		fileObj, found := filesByID[fileID]
		if !found {
			plan.Skipped = append(plan.Skipped, TargetedFileSkip{FileID: fileID, Reason: SkipReasonNotFound})
			continue
		}
		reason, reprocess := fileVectorizationPlan(cfg, fileObj, embeddingSignature, stalledBefore, allowReprocess)
		if reason != "" {
			plan.Skipped = append(plan.Skipped, TargetedFileSkip{FileID: fileID, Reason: reason})
			continue
		}

		plan.Jobs = append(plan.Jobs, TargetedJob{
			FileID:             fileID,
			UserID:             fileObj.UserID,
			EmbeddingSignature: embeddingSignature,
			EmbeddingHost:      embeddingHost,
			Reprocess:          reprocess,
		})
	}
	return plan, nil
}

// QueueTargetedJob 原子登记单个已规划任务，防止并发提交产生重复队列消息；停滞的排队任务允许重新登记。
// 真正的 processing 状态由 worker 领取消息后再设置。
func (s *Service) QueueTargetedJob(ctx context.Context, job TargetedJob) (bool, error) {
	if s == nil || s.repo == nil || strings.TrimSpace(job.FileID) == "" || strings.TrimSpace(job.EmbeddingSignature) == "" {
		return false, nil
	}
	return s.repo.QueueFileEmbedding(ctx, job.UserID, job.FileID, job.EmbeddingSignature, queueStalledBefore(time.Now()))
}

// ResetForReprocessing 把没有可用文本的文件原子重置为待处理，并登记任务对应向量空间的向量化请求。
// 返回 false 表示文件已不满足条件（例如已被其他请求重置或正在提取），调用方应跳过而不是重复投递。
func (s *Service) ResetForReprocessing(ctx context.Context, job TargetedJob) (bool, error) {
	if s == nil || s.repo == nil || strings.TrimSpace(job.FileID) == "" || strings.TrimSpace(job.EmbeddingSignature) == "" {
		return false, nil
	}
	return s.repo.ResetFileForReprocessing(ctx, job.UserID, job.FileID, job.EmbeddingSignature, queueStalledBefore(time.Now()))
}

// PlanReprocessing 扫描全平台没有可用文本的文件（处理失败、排队停滞；includeEmpty 时含无文本文件），
// 为当前向量空间生成重新处理任务。每批最多 limit 个，afterID 用于分页。
func (s *Service) PlanReprocessing(ctx context.Context, afterID uint, limit int, includeEmpty bool) ([]TargetedJob, uint, error) {
	if s == nil || s.repo == nil {
		return nil, afterID, nil
	}
	cfg := s.snapshot()
	files, err := s.repo.ListFilesForReprocessing(ctx, repository.ListFilesForReprocessingInput{
		Limit:         limit,
		AfterID:       afterID,
		IncludeEmpty:  includeEmpty,
		StalledBefore: queueStalledBefore(time.Now()),
	})
	if err != nil || len(files) == 0 {
		return nil, afterID, err
	}
	signature := configuredModelSignature(cfg)
	host := strings.TrimRight(strings.TrimSpace(cfg.EmbeddingHost), "/")
	jobs := make([]TargetedJob, 0, len(files))
	for i := range files {
		// 当前配置无法向量化的文件（如未启用 OCR 的图片）重新提取也建不了索引，不为它们花提取成本。
		if !canEmbedFile(cfg, files[i]) {
			continue
		}
		jobs = append(jobs, TargetedJob{
			FileID:             files[i].FileID,
			UserID:             files[i].UserID,
			EmbeddingSignature: signature,
			EmbeddingHost:      host,
			Reprocess:          true,
		})
	}
	return jobs, files[len(files)-1].ID, nil
}

// EmbedAfterReprocessing 在处理流水线完成提取后，为显式重新处理请求登记的向量空间建立索引。
// 登记的向量空间与当前配置不一致时（期间改过配置）标记为失效，交给常规补建处理。
func (s *Service) EmbedAfterReprocessing(ctx context.Context, fileObj domainconversation.FileObject) error {
	if s == nil || s.repo == nil {
		return nil
	}
	cfg := s.snapshot()
	signature := configuredModelSignature(cfg)
	if strings.TrimSpace(fileObj.EmbedSignature) != signature {
		return s.updateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, fileObj.EmbedSignature, "stale", errEmbeddingConfigurationChanged)
	}
	available, reason, err := s.indexingAvailable(ctx, cfg)
	if !available {
		availabilityErr := embeddingAvailabilityError(reason, err)
		_ = s.updateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, signature, "failed", availabilityErr)
		return availabilityErr
	}
	releaseSlot, err := s.acquireWorkSlot(ctx)
	if err != nil {
		return err
	}
	defer releaseSlot()
	claimed, err := s.repo.ClaimFileEmbedding(ctx, fileObj.UserID, fileObj.FileID, signature)
	if err != nil || !claimed {
		return err
	}
	return s.processClaimedFile(ctx, fileObj, cfg, signature)
}

// ResolveFileVectorizationCapabilities 返回前端展示所需的后端事实状态。
func (s *Service) ResolveFileVectorizationCapabilities(
	ctx context.Context,
	files []domainconversation.FileObject,
) map[string]FileVectorizationCapability {
	capabilities := make(map[string]FileVectorizationCapability, len(files))
	cfg := s.snapshot()
	signature := configuredModelSignature(cfg)
	available, reason, _ := s.indexingAvailable(ctx, cfg)
	if !available {
		for i := range files {
			capabilityReason := reason
			if fileVectorIndexOutdated(files[i], signature) {
				capabilityReason = ReasonOutdatedIndex
			}
			capabilities[files[i].FileID] = FileVectorizationCapability{Reason: capabilityReason}
		}
		return capabilities
	}
	stalledBefore := queueStalledBefore(time.Now())
	for i := range files {
		skipReason, _ := fileVectorizationPlan(cfg, files[i], signature, stalledBefore, false)
		reason := skipReason
		if reason == "" && fileVectorIndexOutdated(files[i], signature) {
			reason = ReasonOutdatedIndex
		}
		capabilities[files[i].FileID] = FileVectorizationCapability{
			CanVectorize: skipReason == "",
			Reason:       reason,
		}
	}
	return capabilities
}

// ProcessTargetedJob 执行从可恢复队列中领取的显式向量化任务。
func (s *Service) ProcessTargetedJob(ctx context.Context, job TargetedJob) error {
	if s == nil || s.repo == nil || strings.TrimSpace(job.FileID) == "" {
		return nil
	}
	releaseSlot, err := s.acquireWorkSlot(ctx)
	if err != nil {
		return err
	}
	defer releaseSlot()
	// 超时从领取执行槽后开始计算：排队等待不会把文件置为 processing，不影响停滞判定。
	ctx, cancel := context.WithTimeout(ctx, JobTimeout)
	defer cancel()

	cfg := s.snapshot()
	if configuredModelSignature(cfg) != strings.TrimSpace(job.EmbeddingSignature) ||
		strings.TrimRight(strings.TrimSpace(cfg.EmbeddingHost), "/") != strings.TrimRight(strings.TrimSpace(job.EmbeddingHost), "/") {
		_ = s.updateFileObjectEmbedStatus(ctx, job.UserID, job.FileID, job.EmbeddingSignature, "stale", errEmbeddingConfigurationChanged)
		return nil
	}
	available, reason, err := s.indexingAvailable(ctx, cfg)
	if !available {
		switch reason {
		case "embedding_disabled", "embedding_model_missing", "embedding_host_missing":
			_ = s.updateFileObjectEmbedStatus(ctx, job.UserID, job.FileID, job.EmbeddingSignature, "stale", errEmbeddingConfigurationChanged)
			return nil
		default:
			return embeddingAvailabilityError(reason, err)
		}
	}
	fileObj, err := s.repo.GetActiveFileObjectByID(ctx, job.UserID, job.FileID)
	if err != nil || fileObj == nil {
		return err
	}
	if fileObj.EmbedSignature != job.EmbeddingSignature || strings.ToLower(strings.TrimSpace(fileObj.EmbedStatus)) != "processing" {
		claimed, claimErr := s.repo.ClaimFileEmbedding(ctx, job.UserID, job.FileID, job.EmbeddingSignature)
		if claimErr != nil || !claimed {
			return claimErr
		}
	} else {
		// 首次投递却发现文件已在执行，说明另一条消息或补建任务正在处理，重复执行只会浪费一次向量化调用。
		if !job.Reclaimed {
			return nil
		}
		// 租约过期后重新领取的消息接手上次中断的执行；刷新更新时间，避免停滞巡检把本次执行判为中断。
		if _, refreshErr := s.repo.UpdateFileObjectEmbedStatus(ctx, job.UserID, job.FileID, job.EmbeddingSignature, "processing", ""); refreshErr != nil {
			return refreshErr
		}
	}
	return s.processClaimedFile(ctx, *fileObj, cfg, job.EmbeddingSignature)
}

// FailTargetedJob 将投递失败的已领取任务释放为可重试状态。
func (s *Service) FailTargetedJob(ctx context.Context, job TargetedJob, cause error) error {
	return s.updateFileObjectEmbedStatus(ctx, job.UserID, job.FileID, job.EmbeddingSignature, "failed", cause)
}

// RequeueTargetedJob 将等待重试的任务恢复为排队状态，避免重试退避期间误显示为执行中或失败。
func (s *Service) RequeueTargetedJob(ctx context.Context, job TargetedJob, cause error) error {
	return s.updateFileObjectEmbedStatus(ctx, job.UserID, job.FileID, job.EmbeddingSignature, "queued", cause)
}

// fileVectorizationPlan 判断文件当前能否提交向量化以及走哪条路径。skipReason 非空时不能提交；
// reprocess 为 true 时文件还没有可用文本，必须先由处理流水线重新提取。
// allowReprocess 只对管理员开放：重新提取可能产生付费 OCR 调用，普通用户保持只能向量化已有文本的行为。
func fileVectorizationPlan(
	cfg config.Config,
	fileObj domainconversation.FileObject,
	embeddingSignature string,
	stalledBefore time.Time,
	allowReprocess bool,
) (skipReason string, reprocess bool) {
	if fileObj.EmbedSignature == embeddingSignature {
		switch strings.ToLower(strings.TrimSpace(fileObj.EmbedStatus)) {
		case "ready":
			return SkipReasonAlreadyReady, false
		case "queued", "processing":
			if !fileEmbeddingStalled(fileObj, stalledBefore) {
				return SkipReasonProcessing, false
			}
		}
	}
	if !canEmbedFile(cfg, fileObj) {
		return SkipReasonUnsupported, false
	}
	switch {
	case allowReprocess && fileNeedsReprocessing(fileObj, stalledBefore):
		return "", true
	case !fileObj.ProcessingReady:
		return SkipReasonNotReady, false
	default:
		return "", false
	}
}

// fileNeedsReprocessing 判断文件是否没有可用文本、需要重新走处理流水线：处理失败、提取完成但无文本，
// 或排队早于停滞时刻仍未开始（队列消息丢失）。条件与仓储的 ResetFileForReprocessing 保持一致。
func fileNeedsReprocessing(fileObj domainconversation.FileObject, stalledBefore time.Time) bool {
	switch strings.ToLower(strings.TrimSpace(fileObj.ProcessingStatus)) {
	case "failed":
		return true
	case "ready":
		return fileObj.ExtractStatus == domainconversation.FileSubprocessStatusEmpty
	default:
		return fileProcessingStalled(fileObj, stalledBefore)
	}
}

func fileVectorIndexOutdated(fileObj domainconversation.FileObject, embeddingSignature string) bool {
	status := strings.ToLower(strings.TrimSpace(fileObj.EmbedStatus))
	return status == "stale" || (status == "ready" && strings.TrimSpace(embeddingSignature) != "" && fileObj.EmbedSignature != embeddingSignature)
}

func normalizeTargetedFileIDs(fileIDs []string) []string {
	normalized := make([]string, 0, len(fileIDs))
	seen := make(map[string]struct{}, len(fileIDs))
	for _, value := range fileIDs {
		fileID := strings.TrimSpace(value)
		if fileID == "" {
			continue
		}
		if _, exists := seen[fileID]; exists {
			continue
		}
		seen[fileID] = struct{}{}
		normalized = append(normalized, fileID)
	}
	return normalized
}

func embeddingAvailabilityError(reason string, cause error) error {
	if cause != nil {
		return fmt.Errorf("%w: %w", ErrEmbeddingServiceUnavailable, cause)
	}
	if reason == "embedding_disabled" || reason == "embedding_model_missing" || reason == "embedding_host_missing" {
		return ErrEmbeddingServiceNotConfigured
	}
	return ErrEmbeddingServiceUnavailable
}

// ProcessFile 执行 embedding 完整流程。
func (s *Service) ProcessFile(ctx context.Context, fileObj domainconversation.FileObject) error {
	cfg := s.snapshot()
	embeddingSignature := configuredModelSignature(cfg)
	if !cfg.EmbeddingEnabled || strings.TrimSpace(cfg.RAGModel) == "" || strings.TrimSpace(cfg.EmbeddingHost) == "" {
		return nil
	}
	if s.repo == nil {
		return nil
	}
	if !canEmbedFile(cfg, fileObj) {
		return nil
	}
	releaseSlot, err := s.acquireWorkSlot(ctx)
	if err != nil {
		return err
	}
	defer releaseSlot()

	claimed, err := s.repo.ClaimFileEmbedding(ctx, fileObj.UserID, fileObj.FileID, embeddingSignature)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	return s.processClaimedFile(ctx, fileObj, cfg, embeddingSignature)
}

func (s *Service) processClaimedFile(ctx context.Context, fileObj domainconversation.FileObject, cfg config.Config, embeddingSignature string) error {
	fileChunks, embeddings, err := s.buildFileChunks(ctx, fileObj, cfg, embeddingSignature)
	if err != nil {
		if errors.Is(err, errNoExtractableText) || extraction.IsEmptyContent(err) {
			return s.markFileEmpty(ctx, fileObj, embeddingSignature)
		}
		_ = s.updateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, embeddingSignature, "failed", err)
		return err
	}
	published, err := s.repo.ReplaceFileChunks(ctx, fileObj.ID, embeddingSignature, fileChunks, embeddings)
	if err != nil {
		_ = s.updateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, embeddingSignature, "failed", err)
		return err
	}
	if !published {
		return nil
	}

	if current, countErr := s.repo.UpdateFileObjectChunkCount(ctx, fileObj.ID, embeddingSignature, len(fileChunks)); countErr != nil {
		return countErr
	} else if !current {
		return nil
	}
	return s.completeFileEmbedding(ctx, fileObj, embeddingSignature, cfg.EmbeddingHost)
}

// maxImageEmbeddingBytes 是一张图片送入嵌入服务前的字节上限；超过时先缩放再发送。
const (
	maxImageEmbeddingBytes    = 4 << 20
	maxImageEmbeddingReadSize = 32 << 20
)

var imageEmbeddingResizeEdges = []int{1536, 1024, 768}

// buildFileChunks 把文件变成分片与对应向量。
// 图片在协议支持时以原图算向量（单分片，OCR 文本作为 Content 保留给全文检索），否则回到提取文本路径。
func (s *Service) buildFileChunks(
	ctx context.Context,
	fileObj domainconversation.FileObject,
	cfg config.Config,
	embeddingSignature string,
) ([]domainconversation.FileChunk, [][]float32, error) {
	now := time.Now()
	if canEmbedImagePixels(cfg, fileObj) {
		data, mimeType, err := s.loadImageForEmbedding(ctx, fileObj)
		if err != nil {
			return nil, nil, err
		}
		embeddings, err := s.embedInputsWithConfig(ctx, []portembedding.Input{{Kind: portembedding.InputImage, MimeType: mimeType, Data: data}}, portembedding.PurposeDocument, cfg)
		if err != nil {
			return nil, nil, err
		}
		// OCR 文本可能不存在，不影响图片入库。
		text, _ := s.loadSourceText(ctx, fileObj)
		chunk := domainconversation.FileChunk{
			FileObjID:          fileObj.ID,
			UserID:             fileObj.UserID,
			ChunkIndex:         0,
			Modality:           domainconversation.FileChunkModalityImage,
			Content:            strings.TrimSpace(text),
			TokenCount:         int(tokenestimate.Estimate(text)),
			EmbeddingSignature: embeddingSignature,
			CreatedAt:          now,
		}
		return []domainconversation.FileChunk{chunk}, embeddings, nil
	}

	text, err := s.loadSourceText(ctx, fileObj)
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(text) == "" {
		return nil, nil, errNoExtractableText
	}
	chunks := embeddingutil.ChunkText(text, cfg.EmbedChunkSizeTokens, cfg.EmbedChunkOverlapTokens)
	if len(chunks) == 0 {
		return nil, nil, errEmptyChunks
	}
	embeddings, err := s.embedTextsWithConfig(ctx, chunks, cfg)
	if err != nil {
		return nil, nil, err
	}
	fileChunks := make([]domainconversation.FileChunk, 0, len(chunks))
	for i, chunk := range chunks {
		fileChunks = append(fileChunks, domainconversation.FileChunk{
			FileObjID:          fileObj.ID,
			UserID:             fileObj.UserID,
			ChunkIndex:         i,
			Modality:           domainconversation.FileChunkModalityText,
			Content:            chunk,
			TokenCount:         int(tokenestimate.Estimate(chunk)),
			EmbeddingSignature: embeddingSignature,
			CreatedAt:          now,
		})
	}
	return fileChunks, embeddings, nil
}

// loadImageForEmbedding 读取图片原图，过大时缩放到 maxImageEmbeddingBytes 以内。
func (s *Service) loadImageForEmbedding(ctx context.Context, fileObj domainconversation.FileObject) ([]byte, string, error) {
	if s.extractSvc == nil {
		return nil, "", fmt.Errorf("extract service not configured")
	}
	declaredMIME := textutil.FirstNonEmpty(fileObj.DetectedMIME, fileObj.MimeType)
	if !imageutil.IsSupportedMimeType(declaredMIME) {
		return nil, "", fmt.Errorf("%w: %s", errImageFormatUnsupported, declaredMIME)
	}
	data, err := s.extractSvc.ReadStoredFile(ctx, fileObj.StoragePath, maxImageEmbeddingReadSize)
	if err != nil {
		return nil, "", err
	}
	mimeType := imageutil.ResolveMimeType(declaredMIME)
	// 逐级缩小长边直到体积达标；ResizeIfNeeded 只在尺寸超过长边时才重新编码。
	for _, maxEdge := range imageEmbeddingResizeEdges {
		if len(data) <= maxImageEmbeddingBytes {
			break
		}
		data, mimeType = imageutil.ResizeIfNeeded(data, mimeType, maxEdge)
	}
	if len(data) > maxImageEmbeddingBytes {
		return nil, "", errImageTooLarge
	}
	return data, mimeType, nil
}

// protocolSupportsImage 读取当前协议声明的图片能力。
func protocolSupportsImage(cfg config.Config) bool {
	return portembedding.ProtocolCapabilities(portembedding.Protocol(cfg.EmbeddingProtocol)).Image
}

// canEmbedImagePixels 判断文件能否直接以原图算向量：协议支持图片，且格式是多模态接口接受的格式。
func canEmbedImagePixels(cfg config.Config, fileObj domainconversation.FileObject) bool {
	if !strings.EqualFold(strings.TrimSpace(fileObj.FileCategory), "image") || !protocolSupportsImage(cfg) {
		return false
	}
	return imageutil.IsSupportedMimeType(textutil.FirstNonEmpty(fileObj.DetectedMIME, fileObj.MimeType))
}

func (s *Service) acquireWorkSlot(ctx context.Context) (func(), error) {
	if s == nil || s.workSlots == nil {
		return func() {}, nil
	}
	select {
	case s.workSlots <- struct{}{}:
		return func() { <-s.workSlots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Service) completeFileEmbedding(ctx context.Context, fileObj domainconversation.FileObject, expectedSignature string, expectedHost string) error {
	const configurationChanged = "embedding configuration changed during processing"
	if !s.embeddingConfigurationCurrent(expectedSignature, expectedHost) {
		_, err := s.repo.UpdateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, expectedSignature, "stale", configurationChanged)
		return err
	}
	current, err := s.repo.UpdateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, expectedSignature, "ready", "")
	if err != nil || !current {
		return err
	}
	// 第二次检查用于封闭首次检查与发布就绪状态之间
	// 配置发生变化的窗口。之后的变更会观察到
	// 已就绪文件，并由常规的全局失效路径处理。
	if !s.embeddingConfigurationCurrent(expectedSignature, expectedHost) {
		_, err = s.repo.UpdateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, expectedSignature, "stale", configurationChanged)
		return err
	}
	return nil
}

func (s *Service) embeddingConfigurationCurrent(expectedSignature string, expectedHost string) bool {
	cfg := s.snapshot()
	return configuredModelSignature(cfg) == expectedSignature &&
		strings.TrimRight(strings.TrimSpace(cfg.EmbeddingHost), "/") == strings.TrimRight(strings.TrimSpace(expectedHost), "/")
}

// markFileEmpty 将无文本文件记为终态 empty。这不是失败，不向调用方返回错误。
func (s *Service) markFileEmpty(ctx context.Context, fileObj domainconversation.FileObject, embeddingSignature string) error {
	return s.updateFileObjectEmbedStatus(ctx, fileObj.UserID, fileObj.FileID, embeddingSignature, domainconversation.FileSubprocessStatusEmpty, errNoExtractableText)
}

func (s *Service) updateFileObjectEmbedStatus(ctx context.Context, userID uint, fileID string, embeddingSignature string, status string, embedErr error) error {
	if s == nil || s.repo == nil {
		return nil
	}
	writeCtx := ctx
	if writeCtx == nil || writeCtx.Err() != nil {
		var cancel context.CancelFunc
		writeCtx, cancel = background.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	_, err := s.repo.UpdateFileObjectEmbedStatus(writeCtx, userID, fileID, embeddingSignature, status, ErrorSummary(embedErr))
	return err
}

// WaitReady 轮询等待文件 embedding 就绪。
func (s *Service) WaitReady(ctx context.Context, userID uint, fileID string, timeout time.Duration) bool {
	if s == nil || s.repo == nil {
		return false
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		fo, err := s.repo.GetActiveFileObjectByID(ctx, userID, fileID)
		if err != nil || fo == nil {
			return false
		}
		if fo.EmbedStatus == "ready" {
			return true
		}
		if fo.EmbedStatus == "failed" || fo.EmbedStatus == domainconversation.FileSubprocessStatusEmpty {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(500 * time.Millisecond):
		}
	}
	return false
}

// loadSourceText 返回文件文本。处理流水线已判定为空的文件直接返回 errNoExtractableText，
// 不再重新提取或 OCR；重新提取由处理流水线负责，结果会保存下来供后续向量化复用。
func (s *Service) loadSourceText(ctx context.Context, fileObj domainconversation.FileObject) (string, error) {
	if s != nil && s.repo != nil {
		if result, err := s.repo.GetFileObjectProcessingByObjectID(ctx, fileObj.ID); err == nil && result != nil {
			if result.ExtractStatus == domainconversation.FileSubprocessStatusEmpty {
				return "", errNoExtractableText
			}
			if path := strings.TrimSpace(result.ExtractStoragePath); path != "" && s.extractSvc != nil {
				text, readErr := s.extractSvc.ReadExtractedText(ctx, path)
				if readErr == nil && strings.TrimSpace(text) != "" {
					return text, nil
				}
			}
		}
	}

	cfg := s.snapshot()
	if s.extractSvc == nil {
		return "", fmt.Errorf("extract service not configured")
	}
	result, err := s.extractSvc.ExtractStoredFile(ctx, extraction.ExtractInput{
		File:                  fileObj,
		PDFMaxPages:           cfg.FileFullContextPDFMaxPages,
		OCREngine:             cfg.ExtractOCREngine,
		ImageOCREnabled:       cfg.ExtractImageOCREnabled,
		PDFOCRFallbackEnabled: cfg.ExtractPDFOCRFallbackEnabled,
	})
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

// EmbedTexts 对外暴露向量化能力，供消息历史 embedding 等场景复用。
// 参数与返回值与内部 embedTexts 相同，失败时返回 error 而非 panic。
func (s *Service) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings, _, err := s.EmbedTextsWithSignature(ctx, texts)
	return embeddings, err
}

// EmbedTextsWithSignature 为待入库文本生成向量和签名；同一份配置快照避免配置切换期间错标向量空间。
func (s *Service) EmbedTextsWithSignature(ctx context.Context, texts []string) ([][]float32, string, error) {
	return s.embedWithSignature(ctx, texts, portembedding.PurposeDocument)
}

// EmbedQueriesWithSignature 为检索查询生成向量；区分文档与查询的模型据此选择对应的任务类型。
func (s *Service) EmbedQueriesWithSignature(ctx context.Context, texts []string) ([][]float32, string, error) {
	return s.embedWithSignature(ctx, texts, portembedding.PurposeQuery)
}

func (s *Service) embedWithSignature(ctx context.Context, texts []string, purpose portembedding.Purpose) ([][]float32, string, error) {
	cfg := s.snapshot()
	embeddings, err := s.embedInputsWithConfig(ctx, portembedding.TextInputs(texts), purpose, cfg)
	if err != nil {
		return nil, "", err
	}
	return embeddings, configuredModelSignature(cfg), nil
}

func (s *Service) embedTextsWithConfig(ctx context.Context, texts []string, cfg config.Config) ([][]float32, error) {
	return s.embedInputsWithConfig(ctx, portembedding.TextInputs(texts), portembedding.PurposeDocument, cfg)
}

// embedInputsWithConfig 按 EmbedBatchSize 分批调用嵌入服务；图片按字节体积计入批次上限，避免单次请求过大。
func (s *Service) embedInputsWithConfig(ctx context.Context, inputs []portembedding.Input, purpose portembedding.Purpose, cfg config.Config) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	model := strings.TrimSpace(cfg.RAGModel)
	host := strings.TrimSpace(cfg.EmbeddingHost)
	if !cfg.EmbeddingEnabled {
		return nil, fmt.Errorf("embedding disabled")
	}
	if model == "" || host == "" {
		return nil, fmt.Errorf("embedding model or host missing")
	}
	if s.embedClient == nil {
		return nil, fmt.Errorf("embedding client not configured")
	}

	apiBase := strings.TrimRight(host, "/")
	apiKey := strings.TrimSpace(cfg.EmbeddingKey)
	batchSize := cfg.EmbedBatchSize
	if batchSize <= 0 {
		batchSize = 20
	}

	var allEmbeddings [][]float32
	for _, batch := range splitEmbeddingBatches(inputs, batchSize) {
		batchEmbeddings, batchErr := s.embedClient.CallAPI(ctx, portembedding.Request{
			Protocol:       portembedding.Protocol(cfg.EmbeddingProtocol),
			APIBase:        apiBase,
			APIKey:         apiKey,
			Model:          model,
			Inputs:         batch,
			Purpose:        purpose,
			Dimensions:     cfg.EmbeddingOutputDimensions,
			OmitDimensions: cfg.EmbeddingDimensionsPolicy == config.EmbeddingDimensionsPolicyOmit,
			TimeoutSeconds: cfg.EmbeddingTimeoutSeconds,
		})
		if batchErr != nil {
			return nil, batchErr
		}
		if len(batchEmbeddings) != len(batch) {
			return nil, fmt.Errorf("embedding batch returned %d vectors for %d inputs", len(batchEmbeddings), len(batch))
		}
		allEmbeddings = append(allEmbeddings, batchEmbeddings...)
	}
	if !cfg.EmbeddingNormalize {
		return allEmbeddings, nil
	}
	for index := range allEmbeddings {
		allEmbeddings[index] = l2Normalize(allEmbeddings[index])
	}
	return allEmbeddings, nil
}

// maxImageBatchBytes 限制一个批次里图片字节总量；Gemini 单请求上限 20MB，留出 base64 膨胀余量。
const maxImageBatchBytes = 12 << 20

// splitEmbeddingBatches 按条数切批，并保证图片批次的字节总量不超过 maxImageBatchBytes。
func splitEmbeddingBatches(inputs []portembedding.Input, batchSize int) [][]portembedding.Input {
	var batches [][]portembedding.Input
	var current []portembedding.Input
	currentBytes := 0
	flush := func() {
		if len(current) > 0 {
			batches = append(batches, current)
			current = nil
			currentBytes = 0
		}
	}
	for _, item := range inputs {
		size := len(item.Data)
		if len(current) >= batchSize || (size > 0 && currentBytes+size > maxImageBatchBytes && len(current) > 0) {
			flush()
		}
		current = append(current, item)
		currentBytes += size
	}
	flush()
	return batches
}

func (s *Service) snapshot() config.Config {
	if s == nil || s.cfg == nil {
		return config.Config{}
	}
	return s.cfg.Snapshot()
}

// EmbeddingIndexStatus 表示向量索引的当前健康状态。
type EmbeddingIndexStatus struct {
	ModelSignature string
	ReadyCount     int64
	StaleCount     int64
	// PendingCount 是尚未索引的可向量化文件与排队、执行中的任务之和。
	PendingCount int64
	// StalledCount 是 PendingCount 中排队或执行超过停滞阈值的任务数。
	StalledCount int64
	// ActiveCount 是 PendingCount 中仍在正常排队或执行的任务数，管理端据此判断是否需要继续刷新进度。
	ActiveCount int64
	FailedCount int64
	EmptyCount  int64
	// UnsupportedCount 是当前配置下类型无法向量化、从未进入索引流程的文件数。
	UnsupportedCount int64
	NeedsReindex     bool
	// ReindexRunning 表示本实例的补建任务仍在执行。
	ReindexRunning bool
}

// ComputeModelSignature 根据模型名和输出维度计算模型签名（格式: hex8@dims）。
// 相同模型/维度组合始终产生相同签名，用于检测配置变更。
func ComputeModelSignature(model string, outputDimensions int) string {
	return embeddingutil.ModelSignature(model, outputDimensions)
}

// ComputeSpaceSignature 在管理员修改模型、输出维度或提供方端点时，
// 派生新的不透明向量空间标识。
func ComputeSpaceSignature(model string, outputDimensions int, endpoint string) string {
	return embeddingutil.SpaceSignature(model, outputDimensions, endpoint)
}

func configuredModelSignature(cfg config.Config) string {
	if signature := strings.TrimSpace(cfg.EmbeddingModelSignature); signature != "" {
		return signature
	}
	if strings.TrimSpace(cfg.RAGModel) == "" {
		return ""
	}
	return ComputeModelSignature(cfg.RAGModel, cfg.EmbeddingOutputDimensions)
}

// GetIndexStatus 返回向量索引的健康状态快照。
func (s *Service) GetIndexStatus(ctx context.Context) (EmbeddingIndexStatus, error) {
	cfg := s.snapshot()
	signature := configuredModelSignature(cfg)
	status := EmbeddingIndexStatus{
		ModelSignature: signature,
	}
	if s.repo == nil {
		return status, nil
	}
	counts, err := s.repo.CountFileEmbeddingStates(ctx, embeddableFileScope(cfg), queueStalledBefore(time.Now()))
	if err != nil {
		return status, err
	}
	for _, item := range counts {
		switch item.Status {
		case "ready":
			status.ReadyCount += item.Count
		case "stale":
			status.StaleCount += item.Count
		case "failed":
			status.FailedCount += item.Count
		case domainconversation.FileSubprocessStatusEmpty:
			status.EmptyCount += item.Count
		case "none":
			// none 只表示从未进入索引流程：当前配置下不支持的文件单独计数；
			// 处理流水线失败的文件没有文本可用，属于失败而不是待处理。
			switch {
			case !item.Embeddable:
				status.UnsupportedCount += item.Count
			case item.ProcessingFailed:
				status.FailedCount += item.Count
			default:
				status.PendingCount += item.Count
			}
		case domainconversation.FileSubprocessStatusQueued, domainconversation.FileSubprocessStatusProcessing:
			status.PendingCount += item.Count
			if item.Stalled {
				status.StalledCount += item.Count
			} else {
				status.ActiveCount += item.Count
			}
		}
	}
	status.NeedsReindex = status.StaleCount > 0
	s.reindexMu.Lock()
	status.ReindexRunning = s.reindexing
	s.reindexMu.Unlock()
	return status, nil
}

// MarkFilesStale 将不属于目标向量空间的文件标记为失效。
func (s *Service) MarkFilesStale(ctx context.Context, activeSignature string) (int64, error) {
	if s.repo == nil {
		return 0, nil
	}
	signature := strings.TrimSpace(activeSignature)
	if signature == "" {
		return 0, nil
	}
	return s.repo.MarkEmbeddedFilesStale(ctx, signature)
}

// ReconcileIndex 对账当前运行时配置与文件索引状态，用于启动恢复和失败补偿。
func (s *Service) ReconcileIndex(ctx context.Context) (int64, error) {
	return s.MarkFilesStale(ctx, configuredModelSignature(s.snapshot()))
}

// ReindexStaleFiles 提交一次去重的后台重建任务，返回本次纳入重建的文件数。
// 只处理已有可用文本且尚未索引、失效、失败或停滞排队的文件，已就绪的文件不会重建；
// 没有可用文本的文件由处理流水线重新提取（见 processing.Service.ReprocessFilesWithoutText）。
// 后台任务通过固定 worker 数执行，不会按文件数量无限创建 goroutine。
func (s *Service) ReindexStaleFiles(ctx context.Context) (int, error) {
	if s.repo == nil {
		return 0, nil
	}
	cfg := s.snapshot()
	available, _, err := s.indexingAvailable(ctx, cfg)
	if err != nil {
		return 0, err
	}
	if !available {
		return 0, ErrEmbeddingServiceNotConfigured
	}

	s.reindexMu.Lock()
	if s.reindexing {
		s.reindexMu.Unlock()
		return 0, ErrReindexInProgress
	}
	s.reindexing = true
	s.reindexMu.Unlock()
	started := false
	defer func() {
		if started {
			return
		}
		s.reindexMu.Lock()
		s.reindexing = false
		s.reindexMu.Unlock()
	}()

	const pageSize = 100
	stalledBefore := queueStalledBefore(time.Now())
	submitted := 0
	var afterID uint
	for {
		files, err := s.repo.ListFilesForReindex(ctx, repository.ListFilesForReindexInput{
			Limit:         pageSize,
			AfterID:       afterID,
			StalledBefore: stalledBefore,
		})
		if err != nil {
			return submitted, err
		}
		if len(files) == 0 {
			break
		}
		for _, f := range files {
			if canEmbedFile(cfg, f) {
				submitted++
			}
		}
		if len(files) < pageSize {
			break
		}
		afterID = files[len(files)-1].ID
	}
	if submitted == 0 {
		return 0, nil
	}

	started = true
	// reindexing 标记保证同一时刻至多一个待执行任务，缓冲为 1 的通道不会阻塞。
	s.reindexJobs <- reindexJob{
		signature:     configuredModelSignature(cfg),
		stalledBefore: stalledBefore,
	}
	return submitted, nil
}

type reindexJob struct {
	signature     string
	stalledBefore time.Time
}

func (s *Service) runReindex(ctx context.Context, job reindexJob) {
	expectedSignature := job.signature
	defer func() {
		s.reindexMu.Lock()
		s.reindexing = false
		s.reindexMu.Unlock()
	}()

	jobs := make(chan domainconversation.FileObject, WorkerConcurrency)
	var workers sync.WaitGroup
	for range WorkerConcurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for fileObj := range jobs {
				if ctx.Err() != nil || configuredModelSignature(s.snapshot()) != expectedSignature {
					continue
				}
				jobCtx, cancel := context.WithTimeout(ctx, JobTimeout)
				err := s.ProcessFile(jobCtx, fileObj)
				cancel()
				if err != nil && !errors.Is(err, context.Canceled) && s.logger != nil {
					s.logger.Warn("embedding_reindex_failed", zap.String("file_id", fileObj.FileID), zap.Error(err))
				}
			}
		}()
	}

	cfg := s.snapshot()
	const pageSize = 100
	var afterID uint
scan:
	for ctx.Err() == nil && configuredModelSignature(s.snapshot()) == expectedSignature {
		files, err := s.repo.ListFilesForReindex(ctx, repository.ListFilesForReindexInput{
			Limit:         pageSize,
			AfterID:       afterID,
			StalledBefore: job.stalledBefore,
		})
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("embedding_reindex_list_failed", zap.Error(err))
			}
			break
		}
		if len(files) == 0 {
			break
		}
		for _, fileObj := range files {
			if !canEmbedFile(cfg, fileObj) {
				continue
			}
			select {
			case jobs <- fileObj:
			case <-ctx.Done():
				break scan
			}
		}
		if len(files) < pageSize {
			break
		}
		afterID = files[len(files)-1].ID
	}
	close(jobs)
	workers.Wait()
}

func supportsEmbeddingSource(fileObj domainconversation.FileObject, cfg config.Config) bool {
	switch strings.ToLower(strings.TrimSpace(fileObj.FileCategory)) {
	case "video", "audio":
		return false
	case "image":
		return cfg.ExtractImageOCREnabled || canEmbedImagePixels(cfg, fileObj)
	}
	mime := strings.ToLower(strings.TrimSpace(fileObj.MimeType))
	name := strings.TrimSpace(fileObj.FileName)
	return filetype.IsText(mime, name) || isPDFMIME(mime, name) || isWordMIME(mime, name) || isPresentationMIME(mime, name) || isExcelMIME(mime, name)
}

// l2Normalize 对向量做 L2 归一化（除以欧氏模长），返回单位向量。
// 零向量（模为 0）保持不变，避免除零。
func l2Normalize(vector []float32) []float32 {
	var sumSq float64
	for _, v := range vector {
		sumSq += float64(v) * float64(v)
	}
	if sumSq == 0 {
		return vector
	}
	norm := float32(1.0 / math.Sqrt(sumSq))
	result := make([]float32, len(vector))
	for i, v := range vector {
		result[i] = v * norm
	}
	return result
}

func isPDFMIME(mimeType, fileName string) bool {
	m := strings.ToLower(strings.TrimSpace(mimeType))
	if m == "application/pdf" {
		return true
	}
	if idx := strings.LastIndex(fileName, "."); idx >= 0 {
		return strings.ToLower(fileName[idx+1:]) == "pdf"
	}
	return false
}

func isWordMIME(mimeType, fileName string) bool {
	m := strings.ToLower(strings.TrimSpace(mimeType))
	ext := ""
	if idx := strings.LastIndex(fileName, "."); idx >= 0 {
		ext = strings.ToLower(fileName[idx+1:])
	}
	return strings.Contains(m, "wordprocessingml") || strings.Contains(m, "msword") ||
		ext == "docx" || ext == "doc"
}

func isPresentationMIME(mimeType, fileName string) bool {
	m := strings.ToLower(strings.TrimSpace(mimeType))
	ext := ""
	if idx := strings.LastIndex(fileName, "."); idx >= 0 {
		ext = strings.ToLower(fileName[idx+1:])
	}
	return strings.Contains(m, "presentationml") || strings.Contains(m, "ms-powerpoint") ||
		ext == "pptx" || ext == "ppt"
}

func isExcelMIME(mimeType, fileName string) bool {
	m := strings.ToLower(strings.TrimSpace(mimeType))
	ext := ""
	if idx := strings.LastIndex(fileName, "."); idx >= 0 {
		ext = strings.ToLower(fileName[idx+1:])
	}
	return strings.Contains(m, "spreadsheetml") || strings.Contains(m, "ms-excel") ||
		m == "text/csv" || ext == "xlsx" || ext == "xls" || ext == "csv"
}
