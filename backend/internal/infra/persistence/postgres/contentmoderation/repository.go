package contentmoderation

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repo implements repository.ContentModerationRepository.
type Repo struct {
	db *gorm.DB
}

var _ repository.ContentModerationRepository = (*Repo)(nil)

// NewRepo creates a content moderation repository.
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) CreateEvent(ctx context.Context, event *domaincm.Event) error {
	if event == nil {
		return nil
	}
	row := toModelEvent(*event)
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return dberror.Translate(err)
	}
	event.ID = row.ID
	event.CreatedAt = row.CreatedAt
	event.UpdatedAt = row.UpdatedAt
	return nil
}

func (r *Repo) GetEventByPublicID(ctx context.Context, publicID string) (*domaincm.Event, error) {
	var row model.ContentModerationEvent
	if err := r.db.WithContext(ctx).Where("public_id = ?", strings.TrimSpace(publicID)).First(&row).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	item := toDomainEvent(row)
	return &item, nil
}

func (r *Repo) GetLatestHitEventByRunID(ctx context.Context, runID string) (*domaincm.Event, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, nil
	}
	var row model.ContentModerationEvent
	err := r.db.WithContext(ctx).
		Where("run_id = ? AND result = ?", runID, domaincm.ResultHit).
		Order("id DESC").
		First(&row).Error
	if err != nil {
		if dberror.IsRecordNotFound(err) || err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, dberror.Translate(err)
	}
	item := toDomainEvent(row)
	return &item, nil
}

func (r *Repo) ListEvents(ctx context.Context, filter domaincm.EventListFilter) ([]domaincm.Event, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.ContentModerationEvent{})
	if v := strings.TrimSpace(filter.Query); v != "" {
		terms := moderationEventExactSearchTerms(v)
		conditions := []string{
			"content_moderation_events.public_id IN ?",
			"content_moderation_events.run_id IN ?",
			"content_moderation_events.message_public_id IN ?",
			"content_moderation_events.result IN ?",
			"content_moderation_events.direction IN ?",
			"content_moderation_events.modality IN ?",
			"content_moderation_events.model IN ?",
			"content_moderation_events.error_code IN ?",
			"content_moderation_events.content_summary IN ?",
			`content_moderation_events.user_id IN (
				SELECT users.id
				FROM identity_users AS users
				WHERE users.deleted_at IS NULL
					AND (
						LOWER(users.username) = ?
						OR LOWER(users.display_name) = ?
						OR LOWER(users.public_id) = ?
					)
			)`,
		}
		args := []any{terms, terms, terms, terms, terms, terms, terms, terms, terms}
		normalized := strings.ToLower(v)
		args = append(args, normalized, normalized, normalized)
		if userID, err := strconv.ParseUint(v, 10, 64); err == nil && userID > 0 {
			conditions = append(conditions, "content_moderation_events.user_id = ?")
			args = append(args, userID)
		}
		q = q.Where("("+strings.Join(conditions, " OR ")+")", args...)
	}
	if v := strings.TrimSpace(filter.Direction); v != "" {
		q = q.Where("direction = ?", v)
	}
	if v := strings.TrimSpace(filter.Modality); v != "" {
		q = q.Where("modality = ?", v)
	}
	if v := strings.TrimSpace(filter.Result); v != "" {
		q = q.Where("result = ?", v)
	}
	if v := strings.TrimSpace(filter.Category); v != "" {
		q = q.Where("categories_json LIKE ?", "%\""+v+"\"%")
	}
	if filter.UserID > 0 {
		q = q.Where("user_id = ?", filter.UserID)
	}
	if v := strings.TrimSpace(filter.RunID); v != "" {
		q = q.Where("run_id = ?", v)
	}
	if filter.From != nil {
		q = q.Where("created_at >= ?", *filter.From)
	}
	if filter.To != nil {
		q = q.Where("created_at <= ?", *filter.To)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, dberror.Translate(err)
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	var rows []model.ContentModerationEvent
	if err := q.Order("id desc").Offset(filter.Offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, 0, dberror.Translate(err)
	}
	items := make([]domaincm.Event, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainEvent(row))
	}
	return items, total, nil
}

func moderationEventExactSearchTerms(query string) []string {
	raw := strings.TrimSpace(query)
	normalized := strings.ToLower(raw)
	if raw == normalized {
		return []string{raw}
	}
	return []string{raw, normalized}
}

func (r *Repo) ClearExpiredContentByPublicIDs(ctx context.Context, publicIDs []string) (int64, error) {
	if len(publicIDs) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).Model(&model.ContentModerationEvent{}).
		Where("public_id IN ?", publicIDs).
		Updates(map[string]any{
			"encrypted_text":  "",
			"image_count":     0,
			"image_meta_json": "[]",
			"content_summary": "",
		})
	return res.RowsAffected, dberror.Translate(res.Error)
}

func (r *Repo) ListExpiredContentEvents(ctx context.Context, before time.Time, limit int) ([]domaincm.Event, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []model.ContentModerationEvent
	// Include text ciphertext and image isolation metadata so pure-text hits expire too.
	if err := r.db.WithContext(ctx).
		Where("content_expires_at <= ? AND (encrypted_text <> '' OR image_count > 0 OR (image_meta_json <> '' AND image_meta_json <> '[]'))", before).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	items := make([]domaincm.Event, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainEvent(row))
	}
	return items, nil
}

func (r *Repo) DeleteExpiredMetadata(ctx context.Context, before time.Time) (int64, error) {
	// Physical delete: retention policy requires rows to disappear, not soft-delete.
	res := r.db.WithContext(ctx).
		Unscoped().
		Where(
			"metadata_expires_at <= ? AND encrypted_text = '' AND image_count = 0 AND (image_meta_json = '' OR image_meta_json = '[]')",
			before,
		).
		Delete(&model.ContentModerationEvent{})
	return res.RowsAffected, dberror.Translate(res.Error)
}

func (r *Repo) IncrementDailyStat(ctx context.Context, input repository.DailyStatIncrement) error {
	day := input.StatDate.UTC().Truncate(24 * time.Hour)
	row := model.ContentModerationDailyStat{
		StatDate:     day,
		Direction:    input.Direction,
		Modality:     input.Modality,
		Result:       input.Result,
		Category:     input.Category,
		CheckCount:   input.CheckCount,
		ContentItems: input.ContentItems,
		HitCount:     input.HitCount,
		FailureCount: input.FailureCount,
		LatencySumMS: input.LatencyMS,
		LatencyCount: 0,
	}
	if input.LatencyMS > 0 {
		row.LatencyCount = 1
	}
	return dberror.Translate(r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "stat_date"},
			{Name: "direction"},
			{Name: "modality"},
			{Name: "result"},
			{Name: "category"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"check_count":    gorm.Expr("check_count + ?", input.CheckCount),
			"content_items":  gorm.Expr("content_items + ?", input.ContentItems),
			"hit_count":      gorm.Expr("hit_count + ?", input.HitCount),
			"failure_count":  gorm.Expr("failure_count + ?", input.FailureCount),
			"latency_sum_ms": gorm.Expr("latency_sum_ms + ?", input.LatencyMS),
			"latency_count":  gorm.Expr("latency_count + ?", row.LatencyCount),
			"updated_at":     time.Now(),
		}),
	}).Create(&row).Error)
}

func (r *Repo) ListDailyStats(ctx context.Context, from, to time.Time) ([]domaincm.DailyStat, error) {
	var rows []model.ContentModerationDailyStat
	if err := r.db.WithContext(ctx).
		Where("stat_date >= ? AND stat_date <= ?", from.UTC().Truncate(24*time.Hour), to.UTC().Truncate(24*time.Hour)).
		Order("stat_date asc, direction, modality, result, category").
		Find(&rows).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	items := make([]domaincm.DailyStat, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainStat(row))
	}
	return items, nil
}

func (r *Repo) DeleteDailyStatsBefore(ctx context.Context, before time.Time) (int64, error) {
	res := r.db.WithContext(ctx).
		Unscoped().
		Where("stat_date < ?", before.UTC().Truncate(24*time.Hour)).
		Delete(&model.ContentModerationDailyStat{})
	return res.RowsAffected, dberror.Translate(res.Error)
}

func (r *Repo) UpdateRunModeration(ctx context.Context, runID string, state string, eventPublicID string, categoriesJSON string) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}
	updates := map[string]any{
		"moderation_state": strings.TrimSpace(state),
	}
	if eventPublicID != "" {
		updates["moderation_event_id"] = eventPublicID
	}
	if categoriesJSON != "" {
		updates["moderation_categories_json"] = categoriesJSON
	}
	if state == domaincm.ModerationStateBlocked {
		updates["status"] = domaincm.StatusBlocked
	}
	return dberror.Translate(r.db.WithContext(ctx).Model(&model.ConversationRun{}).
		Where("run_id = ?", runID).
		Updates(updates).Error)
}

// ApplyRunBlock writes blocked message state, revokes assistant attachments, clears
// assistant text/process traces, and marks the run blocked in a single transaction.
func (r *Repo) ApplyRunBlock(ctx context.Context, runID string, includeUser bool, eventPublicID string, categoriesJSON string) ([]string, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, nil
	}
	fileIDs := make([]string, 0)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var assistantMessageIDs []uint
		if err := tx.Model(&model.Message{}).
			Where("run_id = ? AND role = ?", runID, "assistant").
			Pluck("id", &assistantMessageIDs).Error; err != nil {
			return err
		}
		if len(assistantMessageIDs) > 0 {
			var attachmentFileIDs []string
			if err := tx.Model(&model.Attachment{}).
				Where("message_id IN ? AND status <> ? AND file_id <> ''", assistantMessageIDs, "deleted").
				Pluck("file_id", &attachmentFileIDs).Error; err != nil {
				return err
			}
			seen := make(map[string]struct{}, len(attachmentFileIDs))
			for _, fileID := range attachmentFileIDs {
				fileID = strings.TrimSpace(fileID)
				if fileID == "" {
					continue
				}
				if _, ok := seen[fileID]; ok {
					continue
				}
				seen[fileID] = struct{}{}
				fileIDs = append(fileIDs, fileID)
			}
			if err := tx.Model(&model.Attachment{}).
				Where("message_id IN ? AND status <> ?", assistantMessageIDs, "deleted").
				Update("status", "deleted").Error; err != nil {
				return err
			}
			if len(fileIDs) > 0 {
				if err := tx.Model(&model.FileObject{}).
					Where("file_id IN ?", fileIDs).
					Updates(map[string]any{
						"status":  "moderation_blocked",
						"user_id": 0,
					}).Error; err != nil {
					return err
				}
			}
		}

		msgUpdates := map[string]any{
			"status":                     domaincm.StatusBlocked,
			"moderation_event_id":        eventPublicID,
			"moderation_categories_json": categoriesJSON,
			"error_code":                 "content_moderation.blocked",
			"error_message":              "content blocked by moderation",
		}
		msgQ := tx.Model(&model.Message{}).Where("run_id = ?", runID)
		if !includeUser {
			msgQ = msgQ.Where("role = ?", "assistant")
		}
		if err := msgQ.Updates(msgUpdates).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Message{}).
			Where("run_id = ? AND role = ?", runID, "assistant").
			Updates(map[string]any{
				"content":           "",
				"reasoning_content": "",
				"content_type":      "text",
			}).Error; err != nil {
			return err
		}
		// Drop user-visible process traces / upstream-think so history cannot rehydrate withdrawn content.
		if err := tx.Where("run_id = ? AND event_scope IN ?", runID, []string{"trace_block", "trace_event"}).
			Delete(&model.ChatRunEvent{}).Error; err != nil {
			return err
		}
		runUpdates := map[string]any{
			"moderation_state":           domaincm.ModerationStateBlocked,
			"moderation_event_id":        eventPublicID,
			"moderation_categories_json": categoriesJSON,
			"status":                     domaincm.StatusBlocked,
		}
		res := tx.Model(&model.ConversationRun{}).
			Where("run_id = ?", runID).
			Updates(runUpdates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("content moderation apply block: run %s not found", runID)
		}
		return nil
	})
	if err != nil {
		return nil, dberror.Translate(err)
	}
	return fileIDs, nil
}

func (r *Repo) GetRunModerationState(ctx context.Context, runID string) (string, error) {
	var state string
	err := r.db.WithContext(ctx).Model(&model.ConversationRun{}).
		Select("moderation_state").
		Where("run_id = ?", strings.TrimSpace(runID)).
		Limit(1).
		Scan(&state).Error
	return state, dberror.Translate(err)
}

func (r *Repo) ListStaleModeratingRuns(ctx context.Context, olderThan time.Time, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 100
	}
	var runIDs []string
	err := r.db.WithContext(ctx).Model(&model.ConversationRun{}).
		Select("run_id").
		Where("moderation_state IN ? AND updated_at < ?", []string{
			domaincm.ModerationStateModerating,
			domaincm.ModerationStatePending,
		}, olderThan).
		Limit(limit).
		Pluck("run_id", &runIDs).Error
	return runIDs, dberror.Translate(err)
}

func toModelEvent(item domaincm.Event) model.ContentModerationEvent {
	return model.ContentModerationEvent{
		BaseModel:           model.BaseModel{ID: item.ID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt},
		PublicID:            item.PublicID,
		UserID:              item.UserID,
		ConversationID:      item.ConversationID,
		RunID:               item.RunID,
		MessageID:           item.MessageID,
		MessagePublicID:     item.MessagePublicID,
		Direction:           item.Direction,
		Modality:            item.Modality,
		Model:               item.Model,
		PolicyVersion:       item.PolicyVersion,
		Result:              item.Result,
		CategoriesJSON:      item.CategoriesJSON,
		CategoryScoresJSON:  item.CategoryScoresJSON,
		LatencyMS:           item.LatencyMS,
		ErrorCode:           item.ErrorCode,
		ErrorMessage:        item.ErrorMessage,
		ContentLocationJSON: item.ContentLocationJSON,
		ContentSummary:      item.ContentSummary,
		EncryptedText:       item.EncryptedText,
		ImageCount:          item.ImageCount,
		ImageMetaJSON:       item.ImageMetaJSON,
		ContentExpiresAt:    item.ContentExpiresAt,
		MetadataExpiresAt:   item.MetadataExpiresAt,
	}
}

func toDomainEvent(row model.ContentModerationEvent) domaincm.Event {
	return domaincm.Event{
		ID:                  row.ID,
		PublicID:            row.PublicID,
		UserID:              row.UserID,
		ConversationID:      row.ConversationID,
		RunID:               row.RunID,
		MessageID:           row.MessageID,
		MessagePublicID:     row.MessagePublicID,
		Direction:           row.Direction,
		Modality:            row.Modality,
		Model:               row.Model,
		PolicyVersion:       row.PolicyVersion,
		Result:              row.Result,
		CategoriesJSON:      row.CategoriesJSON,
		CategoryScoresJSON:  row.CategoryScoresJSON,
		LatencyMS:           row.LatencyMS,
		ErrorCode:           row.ErrorCode,
		ErrorMessage:        row.ErrorMessage,
		ContentLocationJSON: row.ContentLocationJSON,
		ContentSummary:      row.ContentSummary,
		EncryptedText:       row.EncryptedText,
		ImageCount:          row.ImageCount,
		ImageMetaJSON:       row.ImageMetaJSON,
		ContentExpiresAt:    row.ContentExpiresAt,
		MetadataExpiresAt:   row.MetadataExpiresAt,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
	}
}

func toDomainStat(row model.ContentModerationDailyStat) domaincm.DailyStat {
	return domaincm.DailyStat{
		ID:           row.ID,
		StatDate:     row.StatDate,
		Direction:    row.Direction,
		Modality:     row.Modality,
		Result:       row.Result,
		Category:     row.Category,
		CheckCount:   row.CheckCount,
		ContentItems: row.ContentItems,
		HitCount:     row.HitCount,
		FailureCount: row.FailureCount,
		LatencySumMS: row.LatencySumMS,
		LatencyCount: row.LatencyCount,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}
