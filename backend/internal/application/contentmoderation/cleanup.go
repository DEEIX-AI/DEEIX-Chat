package contentmoderation

import (
	"context"
	"encoding/json"
	"time"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
	"go.uber.org/zap"
)

const blockRecoveryInterval = 10 * time.Second

func (s *Service) cleanupLoop(ctx context.Context) {
	defer s.wg.Done()
	cleanupTicker := time.NewTicker(cleanupInterval)
	recoveryTicker := time.NewTicker(blockRecoveryInterval)
	defer cleanupTicker.Stop()
	defer recoveryTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-cleanupTicker.C:
			bg, cancel := context.WithTimeout(ctx, 5*time.Minute)
			s.runCleanup(bg)
			s.recoverPendingBlocks(bg)
			s.recoverStaleRuns(bg)
			cancel()
		case <-recoveryTicker.C:
			bg, cancel := context.WithTimeout(ctx, 45*time.Second)
			s.recoverPendingBlocks(bg)
			s.recoverStaleRuns(bg)
			s.retryBlockedGeneratedFileDeletes(bg, 200)
			cancel()
		}
	}
}

func (s *Service) runCleanup(ctx context.Context) {
	if s.repo == nil {
		return
	}
	now := time.Now()
	// Only clear metadata for events whose isolated objects were deleted successfully.
	// Loop until no more expired content rows remain (or a safety cap is hit).
	for pass := 0; pass < 50; pass++ {
		events, err := s.repo.ListExpiredContentEvents(ctx, now, 200)
		if err != nil {
			s.logWarn("content_moderation_list_expired_content_failed", zap.Error(err))
			break
		}
		if len(events) == 0 {
			break
		}
		clearedIDs := make([]string, 0, len(events))
		for _, event := range events {
			// Text-only hits have no isolated images; always clearable after expiry.
			// Image hits require successful object delete first.
			if s.deleteIsolatedImages(ctx, event) {
				clearedIDs = append(clearedIDs, event.PublicID)
			}
		}
		if len(clearedIDs) > 0 {
			if n, err := s.repo.ClearExpiredContentByPublicIDs(ctx, clearedIDs); err != nil {
				s.logWarn("content_moderation_clear_content_failed", zap.Error(err))
			} else if n > 0 {
				s.logWarn("content_moderation_content_cleared", zap.Int64("count", n))
			}
		}
		// If nothing could be cleared this pass (all deletes failed), stop to retry later.
		if len(clearedIDs) == 0 {
			break
		}
	}
	if n, err := s.repo.DeleteExpiredMetadata(ctx, now); err != nil {
		s.logWarn("content_moderation_delete_metadata_failed", zap.Error(err))
	} else if n > 0 {
		s.logWarn("content_moderation_metadata_deleted", zap.Int64("count", n))
	}
	cutoff := now.Add(-metadataRetention)
	if n, err := s.repo.DeleteDailyStatsBefore(ctx, cutoff); err != nil {
		s.logWarn("content_moderation_delete_stats_failed", zap.Error(err))
	} else if n > 0 {
		s.logWarn("content_moderation_stats_deleted", zap.Int64("count", n))
	}
	s.retryBlockedGeneratedFileDeletes(ctx, 200)
}

func (s *Service) retryBlockedGeneratedFileDeletes(ctx context.Context, limit int) {
	if s.fileAccess == nil {
		return
	}
	if n, err := s.fileAccess.RetryBlockedGeneratedFileDeletes(ctx, limit); err != nil {
		s.logWarn("content_moderation_blocked_file_cleanup_failed", zap.Error(err))
	} else if n > 0 {
		s.logWarn("content_moderation_blocked_files_deleted", zap.Int("count", n))
	}
}

// deleteIsolatedImages removes encrypted image copies. Returns true only when all deletes succeed
// (or there were no images), so callers can safely clear metadata paths.
func (s *Service) deleteIsolatedImages(ctx context.Context, event domaincm.Event) bool {
	if event.ImageMetaJSON == "" || event.ImageMetaJSON == "[]" {
		return true
	}
	images := unmarshalIsolatedImageMetadata(event.ImageMetaJSON)
	if images == nil {
		return false
	}
	if len(images) == 0 {
		return true
	}
	if s.objectStore == nil {
		return false
	}
	for _, img := range images {
		if img.StoragePath == "" {
			continue
		}
		if err := s.objectStore.Delete(ctx, img.StoragePath); err != nil {
			s.logWarn("content_moderation_delete_isolated_image_failed",
				zap.String("event_id", event.PublicID),
				zap.String("path", img.StoragePath),
				zap.Error(err),
			)
			return false
		}
	}
	return true
}

func (s *Service) recoverStaleRuns(ctx context.Context) {
	if s.repo == nil {
		return
	}
	olderThan := time.Now().Add(-2 * time.Minute)
	runIDs, err := s.repo.ListStaleModeratingRuns(ctx, olderThan, 100)
	if err != nil {
		return
	}
	for _, runID := range runIDs {
		if s.HasActiveCoordinator(runID) {
			continue
		}
		if s.recoverKnownHit(ctx, runID) {
			continue
		}
		s.recordFailedOpen(ctx, RunMeta{RunID: runID}, domaincm.DirectionOutput, domaincm.ModalityText, domaincm.ErrorCodeWorkerLost, 0)
		if err := s.repo.UpdateRunModeration(ctx, runID, domaincm.ModerationStateFailedOpen, "", "[]"); err != nil {
			s.logWarn("content_moderation_recover_mark_failed_open_failed", zap.String("run_id", runID), zap.Error(err))
		}
	}
}

func (s *Service) recoverKnownHit(ctx context.Context, runID string) bool {
	alreadyNotified := s.hasPendingBlock(runID)
	event, err := s.repo.GetLatestHitEventByRunID(ctx, runID)
	if err != nil {
		// A repository read failure must not convert a potentially known hit into failed-open.
		s.logWarn("content_moderation_recover_hit_lookup_failed", zap.String("run_id", runID), zap.Error(err))
		return true
	}
	if event == nil {
		return false
	}
	var categories []string
	_ = json.Unmarshal([]byte(event.CategoriesJSON), &categories)
	info := BlockInfo{EventID: event.PublicID, Direction: event.Direction, Categories: categories}
	fileIDs, err := s.repo.ApplyRunBlock(ctx, runID, event.Direction == domaincm.DirectionInput, event.PublicID, event.CategoriesJSON)
	if err != nil {
		s.registerPendingBlock(RunMeta{RunID: runID}, info)
		s.logWarn("content_moderation_recover_hit_apply_failed", zap.String("run_id", runID), zap.Error(err))
		return true
	}
	s.removePendingBlock(runID)
	s.deleteBlockedOutputFiles(ctx, fileIDs)
	if !alreadyNotified {
		s.notifyBlockedRecovery(ctx, runID, info)
	}
	return true
}

func (s *Service) recoverPendingBlocks(ctx context.Context) {
	for _, item := range s.pendingBlockSnapshot() {
		fileIDs, err := s.repo.ApplyRunBlock(
			ctx,
			item.meta.RunID,
			item.info.Direction == domaincm.DirectionInput,
			item.info.EventID,
			mustJSON(item.info.Categories),
		)
		if err != nil {
			s.logWarn("content_moderation_pending_block_retry_failed", zap.String("run_id", item.meta.RunID), zap.Error(err))
			continue
		}
		s.removePendingBlock(item.meta.RunID)
		s.deleteBlockedOutputFiles(ctx, fileIDs)
	}
}

func (s *Service) handleLateBlock(parent context.Context, meta RunMeta, info BlockInfo) {
	if s == nil || s.repo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	fileIDs, err := s.repo.ApplyRunBlock(
		ctx,
		meta.RunID,
		info.Direction == domaincm.DirectionInput,
		info.EventID,
		mustJSON(info.Categories),
	)
	if err != nil {
		s.registerPendingBlock(meta, info)
		if stateErr := s.repo.UpdateRunModeration(ctx, meta.RunID, domaincm.ModerationStateModerating, info.EventID, mustJSON(info.Categories)); stateErr != nil {
			s.logWarn("content_moderation_late_block_mark_pending_failed", zap.String("run_id", meta.RunID), zap.Error(stateErr))
		}
		s.logWarn("content_moderation_late_block_apply_failed", zap.String("run_id", meta.RunID), zap.Error(err))
	} else {
		s.removePendingBlock(meta.RunID)
		s.deleteBlockedOutputFiles(ctx, fileIDs)
	}
	cancel()
	s.notifyBlockedRecovery(parent, meta.RunID, info)
}

func (s *Service) notifyBlockedRecovery(ctx context.Context, runID string, info BlockInfo) {
	if s.onBlocked != nil {
		s.onBlocked(ctx, runID, info)
	}
	if s.emitEvent != nil {
		s.emitEvent(ctx, runID, "moderation_blocked", map[string]any{
			"type":       "moderation_blocked",
			"eventID":    info.EventID,
			"direction":  info.Direction,
			"categories": info.Categories,
		})
	}
}

func (s *Service) deleteBlockedOutputFiles(parent context.Context, fileIDs []string) {
	if s == nil || s.fileAccess == nil || len(fileIDs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	for _, fileID := range fileIDs {
		if err := s.fileAccess.DeleteGeneratedFileArtifacts(ctx, fileID); err != nil {
			s.logWarn("content_moderation_delete_blocked_output_failed", zap.String("file_id", fileID), zap.Error(err))
		}
	}
}
