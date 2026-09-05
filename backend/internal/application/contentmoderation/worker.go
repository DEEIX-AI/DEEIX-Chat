package contentmoderation

import (
	"context"
	"errors"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"strings"
	"time"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

type moderationTask struct {
	Coord       *RunCoordinator
	Direction   string
	Modality    string
	Text        string
	FileIDs     []string
	Selected    []string
	Location    domaincm.ContentLocation
	RawImages   []OutputImageSource
	IsolateOnly bool // input images: isolate copy only, do not revoke user files
}

type taskResult struct {
	Hit        bool
	Categories []string
	Scores     map[string]float64
	EventID    string
	LatencyMS  int64
	Err        error
	ErrorCode  string
}

func (s *Service) workerLoop(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case task, ok := <-s.taskQueue:
			if !ok || task == nil {
				return
			}

			// Physical token (fixed channel) + logical activeWorkers gate.
			select {
			case <-ctx.Done():
				s.markTaskDequeued()
				if task.Coord != nil {
					task.Coord.onTaskResult(task, taskResult{Err: ErrModerationTimeout, ErrorCode: domaincm.ErrorCodeTimeout})
				}
				return
			case <-s.stopCh:
				s.markTaskDequeued()
				if task.Coord != nil {
					task.Coord.onTaskResult(task, taskResult{Err: ErrWorkerLost, ErrorCode: domaincm.ErrorCodeWorkerLost})
				}
				return
			case s.workerSem <- struct{}{}:
			}
			if !s.waitLogicalSlot(ctx) {
				s.markTaskDequeued()
				if task.Coord != nil {
					task.Coord.onTaskResult(task, taskResult{Err: ErrWorkerLost, ErrorCode: domaincm.ErrorCodeWorkerLost})
				}
				<-s.workerSem
				continue
			}
			s.markTaskDequeued()
			s.executeTask(ctx, task)
			s.releaseLogicalSlot()
			<-s.workerSem
		}
	}
}

func (s *Service) markTaskDequeued() {
	s.workerMu.Lock()
	if s.queuedCount > 0 {
		s.queuedCount--
	}
	s.workerMu.Unlock()
}

// waitLogicalSlot blocks until activeWorkers < maxConcurrency.
// Returns false if the worker is shutting down before a slot is acquired.
func (s *Service) waitLogicalSlot(ctx context.Context) bool {
	for {
		s.workerMu.Lock()
		limit := s.maxConcurrency
		if limit < 1 {
			limit = defaultMaxConcurrency
		}
		if s.activeWorkers < limit {
			s.activeWorkers++
			s.workerMu.Unlock()
			return true
		}
		wake := s.workerWake
		s.workerMu.Unlock()
		select {
		case <-ctx.Done():
			return false
		case <-s.stopCh:
			return false
		case <-wake:
		}
	}
}

func (s *Service) releaseLogicalSlot() {
	s.workerMu.Lock()
	if s.activeWorkers > 0 {
		s.activeWorkers--
	}
	wake := s.workerWake
	s.workerMu.Unlock()
	if wake != nil {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
}

func (s *Service) enqueue(task *moderationTask) error {
	if task == nil {
		return nil
	}
	s.workerMu.Lock()
	limit := s.queueCapacity
	if limit < 1 {
		limit = defaultQueueCapacity
	}
	if s.queuedCount >= limit {
		s.workerMu.Unlock()
		if task.Coord != nil {
			bg, cancel := context.WithTimeout(task.Coord.ctx, 5*time.Second)
			s.recordFailedOpen(bg, task.Coord.meta, task.Direction, task.Modality, domaincm.ErrorCodeQueueFull, 0)
			s.bumpDailyStat(bg, repository.DailyStatIncrement{
				Direction:    task.Direction,
				Modality:     task.Modality,
				Result:       domaincm.ResultFailedOpen,
				CheckCount:   1,
				ContentItems: contentItemCount(task),
				FailureCount: 1,
			})
			cancel()
		}
		return ErrQueueFull
	}
	queue := s.taskQueue
	select {
	case queue <- task:
		s.queuedCount++
		s.workerMu.Unlock()
		return nil
	default:
		s.workerMu.Unlock()
		if task.Coord != nil {
			bg, cancel := context.WithTimeout(task.Coord.ctx, 5*time.Second)
			s.recordFailedOpen(bg, task.Coord.meta, task.Direction, task.Modality, domaincm.ErrorCodeQueueFull, 0)
			s.bumpDailyStat(bg, repository.DailyStatIncrement{
				Direction:    task.Direction,
				Modality:     task.Modality,
				Result:       domaincm.ResultFailedOpen,
				CheckCount:   1,
				ContentItems: contentItemCount(task),
				FailureCount: 1,
			})
			cancel()
		}
		return ErrQueueFull
	}
}

func (s *Service) executeTask(parent context.Context, task *moderationTask) {
	started := time.Now()
	cfg := task.Coord.cfg
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	runCtx, cancelRun := context.WithCancel(task.Coord.ctx)
	stopWorkerCancellation := context.AfterFunc(parent, cancelRun)
	defer func() {
		stopWorkerCancellation()
		cancelRun()
	}()
	providerCtx, cancelProvider := context.WithTimeout(runCtx, timeout)

	selected := task.Selected
	if len(selected) == 0 {
		selected = cfg.Policy.CategoriesFor(task.Direction, task.Modality)
	}

	var (
		resp *Response
		err  error
	)
	if s.provider == nil {
		err = ErrModerationService
	} else {
		providerConfig := providerConfigFromRuntime(cfg)
		switch task.Modality {
		case domaincm.ModalityImage:
			images := make([]ProviderImage, 0, len(task.RawImages))
			for _, image := range task.RawImages {
				if len(image.Data) == 0 {
					continue
				}
				images = append(images, ProviderImage{Data: image.Data, MimeType: image.MimeType})
			}
			resp, err = s.provider.ModerateImages(providerCtx, providerConfig, images, selected, task.Modality)
		default:
			resp, err = s.provider.ModerateText(providerCtx, providerConfig, task.Text, selected, task.Modality)
		}
	}
	cancelProvider()
	latency := time.Since(started).Milliseconds()
	// Start a fresh persistence budget only after the upstream request finishes.
	// Slow moderation requests must not consume the time reserved for recording
	// their result and statistics.
	persistCtx, persistCancel := context.WithTimeout(runCtx, 10*time.Second)
	defer persistCancel()

	if err != nil {
		code := classifyErrorCode(err)
		s.logWarn("content_moderation_provider_failed",
			zap.String("run_id", task.Coord.meta.RunID),
			zap.String("direction", task.Direction),
			zap.String("modality", task.Modality),
			zap.String("error_code", code),
			zap.Error(err),
		)
		s.recordFailedOpen(persistCtx, task.Coord.meta, task.Direction, task.Modality, code, latency)
		s.bumpDailyStat(persistCtx, repository.DailyStatIncrement{
			Direction:    task.Direction,
			Modality:     task.Modality,
			Result:       domaincm.ResultFailedOpen,
			CheckCount:   1,
			ContentItems: contentItemCount(task),
			FailureCount: 1,
			LatencyMS:    latency,
		})
		task.Coord.onTaskResult(task, taskResult{
			Err:       err,
			ErrorCode: code,
			LatencyMS: latency,
		})
		return
	}

	eval := EvaluateHit(resp, selected, task.Modality)
	if !eval.Hit {
		if _, recordErr := s.recordPass(persistCtx, task, latency, resp); recordErr != nil {
			s.logWarn("content_moderation_record_pass_failed", zap.Error(recordErr))
		}
		s.bumpDailyStat(persistCtx, repository.DailyStatIncrement{
			Direction:    task.Direction,
			Modality:     task.Modality,
			Result:       domaincm.ResultPassed,
			CheckCount:   1,
			ContentItems: contentItemCount(task),
			LatencyMS:    latency,
		})
		task.Coord.onTaskResult(task, taskResult{LatencyMS: latency})
		return
	}

	eventID, recordErr := s.recordHit(persistCtx, task, eval, latency, resp)
	if recordErr != nil {
		s.logWarn("content_moderation_record_hit_failed", zap.Error(recordErr))
	}
	s.bumpDailyStat(persistCtx, repository.DailyStatIncrement{
		Direction:    task.Direction,
		Modality:     task.Modality,
		Result:       domaincm.ResultHit,
		CheckCount:   1,
		ContentItems: contentItemCount(task),
		HitCount:     1,
		LatencyMS:    latency,
	})
	for _, cat := range eval.Categories {
		s.bumpDailyStat(persistCtx, repository.DailyStatIncrement{
			Direction: task.Direction,
			Modality:  task.Modality,
			Result:    domaincm.ResultHit,
			Category:  cat,
			HitCount:  1,
		})
	}
	lateBlock := task.Coord.onTaskResult(task, taskResult{
		Hit:        true,
		Categories: eval.Categories,
		Scores:     eval.Scores,
		EventID:    eventID,
		LatencyMS:  latency,
	})
	if lateBlock != nil {
		s.handleLateBlock(runCtx, task.Coord.meta, *lateBlock)
	}
}

func contentItemCount(task *moderationTask) int64 {
	if task == nil {
		return 0
	}
	if task.Modality == domaincm.ModalityImage {
		n := len(task.RawImages)
		if n == 0 {
			return 1
		}
		return int64(n)
	}
	return 1
}

func classifyErrorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrQueueFull):
		return domaincm.ErrorCodeQueueFull
	case errors.Is(err, ErrModerationTimeout):
		return domaincm.ErrorCodeTimeout
	case errors.Is(err, ErrModerationRateLimited):
		return domaincm.ErrorCodeRateLimited
	case errors.Is(err, ErrModerationInvalidResp):
		return domaincm.ErrorCodeInvalidResp
	case errors.Is(err, ErrModerationNetwork):
		return domaincm.ErrorCodeNetworkError
	case errors.Is(err, ErrWorkerLost):
		return domaincm.ErrorCodeWorkerLost
	default:
		return domaincm.ErrorCodeServiceError
	}
}

func (s *Service) recordFailedOpen(
	ctx context.Context,
	meta RunMeta,
	direction, modality, errorCode string,
	latencyMS int64,
) {
	if s.repo == nil {
		return
	}
	now := time.Now()
	event := &domaincm.Event{
		PublicID:            newPublicEventID(),
		UserID:              meta.UserID,
		ConversationID:      meta.ConversationID,
		RunID:               meta.RunID,
		MessageID:           meta.MessageID,
		MessagePublicID:     meta.MessagePublicID,
		Direction:           direction,
		Modality:            modality,
		Result:              domaincm.ResultFailedOpen,
		CategoriesJSON:      "[]",
		CategoryScoresJSON:  "{}",
		LatencyMS:           latencyMS,
		ErrorCode:           errorCode,
		ErrorMessage:        truncate(moderationFailureMessage(errorCode), 255),
		ContentLocationJSON: "{}",
		ContentSummary:      "",
		ImageMetaJSON:       "[]",
		ContentExpiresAt:    now.Add(contentRetention),
		MetadataExpiresAt:   now.Add(metadataRetention),
	}
	if cfg, err := s.loadRuntimeConfig(ctx); err == nil {
		event.Model = cfg.Model
		event.PolicyVersion = cfg.Policy.Version
	}
	if err := s.repo.CreateEvent(ctx, event); err != nil {
		s.logWarn("content_moderation_failed_open_event_failed", zap.Error(err))
	}
	s.logWarn("content_moderation_failed_open",
		zap.String("run_id", meta.RunID),
		zap.String("direction", direction),
		zap.String("modality", modality),
		zap.String("error_code", errorCode),
	)
}

func moderationFailureMessage(errorCode string) string {
	switch strings.TrimSpace(errorCode) {
	case domaincm.ErrorCodeTimeout:
		return ErrModerationTimeout.Error()
	case domaincm.ErrorCodeRateLimited:
		return ErrModerationRateLimited.Error()
	case domaincm.ErrorCodeQueueFull:
		return ErrQueueFull.Error()
	case domaincm.ErrorCodeInvalidResp:
		return ErrModerationInvalidResp.Error()
	case domaincm.ErrorCodeNetworkError:
		return ErrModerationNetwork.Error()
	case domaincm.ErrorCodeWorkerLost:
		return ErrWorkerLost.Error()
	case domaincm.ErrorCodeConfigMissing:
		return "content moderation configuration unavailable"
	case domaincm.ErrorCodeServiceError:
		return ErrModerationService.Error()
	default:
		return "content moderation check failed"
	}
}

func (s *Service) recordPass(ctx context.Context, task *moderationTask, latencyMS int64, resp *Response) (string, error) {
	if s.repo == nil || task == nil || task.Coord == nil {
		return "", nil
	}
	now := time.Now()
	publicID := newPublicEventID()
	modelName := task.Coord.cfg.Model
	policyVersion := task.Coord.cfg.Policy.Version
	if resp != nil && strings.TrimSpace(resp.Model) != "" {
		modelName = resp.Model
	}
	summary := "image_pass"
	if task.Modality == domaincm.ModalityText {
		if task.Direction == domaincm.DirectionInput {
			summary = "input_text_pass"
		} else {
			summary = "output_text_pass"
		}
	}
	// Pass events keep metadata only — do not retain encrypted content payloads.
	event := &domaincm.Event{
		PublicID:            publicID,
		UserID:              task.Coord.meta.UserID,
		ConversationID:      task.Coord.meta.ConversationID,
		RunID:               task.Coord.meta.RunID,
		MessageID:           task.Coord.meta.MessageID,
		MessagePublicID:     task.Coord.meta.MessagePublicID,
		Direction:           task.Direction,
		Modality:            task.Modality,
		Model:               modelName,
		PolicyVersion:       policyVersion,
		Result:              domaincm.ResultPassed,
		CategoriesJSON:      "[]",
		CategoryScoresJSON:  "{}",
		LatencyMS:           latencyMS,
		ContentLocationJSON: marshalContentLocation(task.Location),
		ContentSummary:      summary,
		ImageMetaJSON:       "[]",
		ContentExpiresAt:    now,
		MetadataExpiresAt:   now.Add(metadataRetention),
	}
	if err := s.repo.CreateEvent(ctx, event); err != nil {
		return publicID, err
	}
	return publicID, nil
}

func (s *Service) deleteUntrackedIsolatedImages(ctx context.Context, eventID string, images []domaincm.IsolatedImageMeta) {
	if s == nil || s.objectStore == nil {
		return
	}
	for _, image := range images {
		path := strings.TrimSpace(image.StoragePath)
		if path == "" {
			continue
		}
		if err := s.objectStore.Delete(ctx, path); err != nil {
			s.logWarn(
				"content_moderation_rollback_isolated_image_failed",
				zap.String("event_id", eventID),
				zap.String("path", path),
				zap.Error(err),
			)
		}
	}
}

func (s *Service) recordHit(ctx context.Context, task *moderationTask, eval HitEvaluation, latencyMS int64, resp *Response) (string, error) {
	now := time.Now()
	publicID := newPublicEventID()
	modelName := ""
	policyVersion := int64(0)
	if task.Coord != nil {
		modelName = task.Coord.cfg.Model
		policyVersion = task.Coord.cfg.Policy.Version
	}
	if resp != nil && strings.TrimSpace(resp.Model) != "" {
		modelName = resp.Model
	}

	encryptedText := ""
	// Opaque summary only — never store plaintext snippets in the list metadata.
	summary := "image_hit"
	if task.Modality == domaincm.ModalityText {
		if enc, err := s.encryptText(task.Text); err == nil {
			encryptedText = enc
		}
		if task.Direction == domaincm.DirectionInput {
			summary = "input_text_hit"
		} else {
			summary = "output_text_hit"
		}
	}

	imageMeta := make([]domaincm.IsolatedImageMeta, 0)
	if task.Modality == domaincm.ModalityImage && len(task.RawImages) > 0 {
		for i, img := range task.RawImages {
			data := img.Data
			if len(data) == 0 {
				// Still revoke/delete output images even when payload bytes are missing.
				if !task.IsolateOnly && s.fileAccess != nil && strings.TrimSpace(img.FileID) != "" {
					if err := s.fileAccess.RevokeGeneratedFile(ctx, img.FileID); err != nil {
						s.logWarn("content_moderation_revoke_file_failed", zap.String("file_id", img.FileID), zap.Error(err))
					}
					if err := s.fileAccess.DeleteGeneratedFileArtifacts(ctx, img.FileID); err != nil {
						s.logWarn("content_moderation_delete_file_artifacts_failed", zap.String("file_id", img.FileID), zap.Error(err))
					}
				}
				continue
			}
			sha := img.SHA256
			if sha == "" {
				sha = sha256Hex(data)
			}
			// Attempt isolation copy; output revoke/delete always runs regardless of isolation success.
			if s.objectStore != nil {
				encStr, err := s.encryptBytes(data)
				if err != nil {
					s.logWarn("content_moderation_encrypt_image_failed", zap.Error(err))
				} else {
					path := isolatedImagePath(publicID, i, sha)
					if err := s.objectStore.Put(ctx, path, []byte(encStr), "application/octet-stream"); err != nil {
						s.logWarn("content_moderation_isolate_image_failed", zap.Error(err))
					} else {
						imageMeta = append(imageMeta, domaincm.IsolatedImageMeta{
							Index:        i,
							SHA256:       sha,
							MimeType:     textutil.FirstNonEmpty(img.MimeType, "image/png"),
							SizeBytes:    int64(len(data)),
							StoragePath:  path,
							SourceFileID: img.FileID,
						})
					}
				}
			}
			if !task.IsolateOnly && s.fileAccess != nil && strings.TrimSpace(img.FileID) != "" {
				if err := s.fileAccess.RevokeGeneratedFile(ctx, img.FileID); err != nil {
					s.logWarn("content_moderation_revoke_file_failed", zap.String("file_id", img.FileID), zap.Error(err))
				}
				if err := s.fileAccess.DeleteGeneratedFileArtifacts(ctx, img.FileID); err != nil {
					s.logWarn("content_moderation_delete_file_artifacts_failed", zap.String("file_id", img.FileID), zap.Error(err))
				}
			}
		}
	}

	event := &domaincm.Event{
		PublicID:            publicID,
		UserID:              task.Coord.meta.UserID,
		ConversationID:      task.Coord.meta.ConversationID,
		RunID:               task.Coord.meta.RunID,
		MessageID:           task.Coord.meta.MessageID,
		MessagePublicID:     task.Coord.meta.MessagePublicID,
		Direction:           task.Direction,
		Modality:            task.Modality,
		Model:               modelName,
		PolicyVersion:       policyVersion,
		Result:              domaincm.ResultHit,
		CategoriesJSON:      mustJSON(eval.Categories),
		CategoryScoresJSON:  mustJSON(eval.Scores),
		LatencyMS:           latencyMS,
		ContentLocationJSON: marshalContentLocation(task.Location),
		ContentSummary:      summary,
		EncryptedText:       encryptedText,
		ImageCount:          len(imageMeta),
		ImageMetaJSON:       marshalIsolatedImageMetadata(imageMeta),
		ContentExpiresAt:    now.Add(contentRetention),
		MetadataExpiresAt:   now.Add(metadataRetention),
	}
	if err := s.repo.CreateEvent(ctx, event); err != nil {
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		s.deleteUntrackedIsolatedImages(rollbackCtx, publicID, imageMeta)
		cancel()
		return "", err
	}
	return publicID, nil
}

func (s *Service) bumpDailyStat(ctx context.Context, input repository.DailyStatIncrement) {
	if s.repo == nil {
		return
	}
	input.StatDate = time.Now().UTC().Truncate(24 * time.Hour)
	if err := s.repo.IncrementDailyStat(ctx, input); err != nil {
		s.logWarn("content_moderation_increment_daily_stat_failed", zap.Error(err))
	}
}

func truncate(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}
