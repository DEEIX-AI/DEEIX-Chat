package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	domaindiscussion "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/discussion"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func encodeDiscussionSnapshot(snapshot *domaindiscussion.Snapshot) (string, error) {
	if snapshot == nil {
		return "", nil
	}
	raw, err := json.Marshal(snapshot)
	return string(raw), err
}

func decodeDiscussionSnapshot(raw string) (*domaindiscussion.Snapshot, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var snapshot domaindiscussion.Snapshot
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func toDiscussionSnapshotDomain(entity models.ConversationDiscussionSnapshot) (*domaindiscussion.ConversationSnapshot, error) {
	active, err := decodeDiscussionSnapshot(entity.ActiveSnapshotJSON)
	if err != nil {
		return nil, err
	}
	pending, err := decodeDiscussionSnapshot(entity.PendingSnapshotJSON)
	if err != nil {
		return nil, err
	}
	return &domaindiscussion.ConversationSnapshot{
		ConversationID:   entity.ConversationID,
		TemplatePublicID: entity.TemplatePublicID,
		Active:           active,
		ActiveVersion:    entity.ActiveVersion,
		Pending:          pending,
		PendingVersion:   entity.PendingVersion,
		PendingCreatedAt: entity.PendingCreatedAt,
		UpdatedAt:        entity.UpdatedAt,
	}, nil
}

func findDiscussionSnapshot(tx *gorm.DB, conversationID uint, templatePublicID string, lock bool) (*models.ConversationDiscussionSnapshot, error) {
	var entity models.ConversationDiscussionSnapshot
	query := tx.Where("conversation_id = ? AND template_public_id = ?", conversationID, templatePublicID)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&entity).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

func nextDiscussionSnapshotVersion(entity models.ConversationDiscussionSnapshot) int64 {
	if entity.PendingVersion > entity.ActiveVersion {
		return entity.PendingVersion + 1
	}
	return entity.ActiveVersion + 1
}

func validatePendingVersion(entity models.ConversationDiscussionSnapshot, expected int64) error {
	if entity.PendingSnapshotJSON == "" {
		if expected == 0 {
			return nil
		}
		return domaindiscussion.ErrSnapshotConflict
	}
	if expected != entity.PendingVersion {
		return domaindiscussion.ErrSnapshotConflict
	}
	return nil
}

func translateDiscussionSnapshotCreateError(err error) error {
	translated := translateError(err)
	if errors.Is(translated, repository.ErrDuplicate) {
		return domaindiscussion.ErrSnapshotConflict
	}
	return translated
}

func (r *Repo) GetDiscussionSnapshot(ctx context.Context, conversationID uint, templatePublicID string) (*domaindiscussion.ConversationSnapshot, error) {
	entity, err := findDiscussionSnapshot(r.db.WithContext(ctx), conversationID, templatePublicID, false)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, translateError(err)
	}
	return toDiscussionSnapshotDomain(*entity)
}

func (r *Repo) RefreshDiscussionSnapshot(ctx context.Context, conversationID uint, templatePublicID string, pending domaindiscussion.Snapshot, expectedPendingVersion int64) (*domaindiscussion.ConversationSnapshot, error) {
	var result *domaindiscussion.ConversationSnapshot
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entity, err := findDiscussionSnapshot(tx, conversationID, templatePublicID, true)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if expectedPendingVersion != 0 {
				return domaindiscussion.ErrSnapshotConflict
			}
			now := time.Now()
			raw, encodeErr := encodeDiscussionSnapshot(&pending)
			if encodeErr != nil {
				return encodeErr
			}
			entity = &models.ConversationDiscussionSnapshot{
				ConversationID:      conversationID,
				TemplatePublicID:    templatePublicID,
				PendingSnapshotJSON: raw,
				PendingVersion:      1,
				PendingCreatedAt:    &now,
			}
			if err = tx.Create(entity).Error; err != nil {
				return translateDiscussionSnapshotCreateError(err)
			}
		} else if err != nil {
			return translateError(err)
		} else {
			if err = validatePendingVersion(*entity, expectedPendingVersion); err != nil {
				return err
			}
			raw, encodeErr := encodeDiscussionSnapshot(&pending)
			if encodeErr != nil {
				return encodeErr
			}
			now := time.Now()
			entity.PendingSnapshotJSON = raw
			entity.PendingVersion = nextDiscussionSnapshotVersion(*entity)
			entity.PendingCreatedAt = &now
			if err = tx.Save(entity).Error; err != nil {
				return translateError(err)
			}
		}
		result, err = toDiscussionSnapshotDomain(*entity)
		return err
	})
	return result, err
}

func (r *Repo) UpdateDiscussionSnapshotPending(ctx context.Context, conversationID uint, templatePublicID string, pending domaindiscussion.Snapshot, expectedPendingVersion int64) (*domaindiscussion.ConversationSnapshot, error) {
	var result *domaindiscussion.ConversationSnapshot
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entity, err := findDiscussionSnapshot(tx, conversationID, templatePublicID, true)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domaindiscussion.ErrPendingNotFound
		}
		if err != nil {
			return translateError(err)
		}
		if err = validatePendingVersion(*entity, expectedPendingVersion); err != nil {
			return err
		}
		if entity.PendingSnapshotJSON == "" {
			return domaindiscussion.ErrPendingNotFound
		}
		raw, err := encodeDiscussionSnapshot(&pending)
		if err != nil {
			return err
		}
		entity.PendingSnapshotJSON = raw
		if err = tx.Model(entity).Update("pending_snapshot_json", raw).Error; err != nil {
			return translateError(err)
		}
		result, err = toDiscussionSnapshotDomain(*entity)
		return err
	})
	return result, err
}

func (r *Repo) RollbackDiscussionSnapshot(ctx context.Context, conversationID uint, templatePublicID string, expectedPendingVersion int64) (*domaindiscussion.ConversationSnapshot, error) {
	var result *domaindiscussion.ConversationSnapshot
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entity, err := findDiscussionSnapshot(tx, conversationID, templatePublicID, true)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if expectedPendingVersion == 0 {
				return nil
			}
			return domaindiscussion.ErrSnapshotConflict
		}
		if err != nil {
			return translateError(err)
		}
		if err = validatePendingVersion(*entity, expectedPendingVersion); err != nil {
			return err
		}
		entity.PendingSnapshotJSON = ""
		entity.PendingVersion = 0
		entity.PendingCreatedAt = nil
		if err = tx.Save(entity).Error; err != nil {
			return translateError(err)
		}
		result, err = toDiscussionSnapshotDomain(*entity)
		return err
	})
	return result, err
}

func (r *Repo) CommitDiscussionSnapshot(ctx context.Context, conversationID uint, templatePublicID string, snapshot domaindiscussion.Snapshot, usePending bool, expectedPendingVersion int64) (*domaindiscussion.ConversationSnapshot, bool, error) {
	var result *domaindiscussion.ConversationSnapshot
	committed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entity, err := findDiscussionSnapshot(tx, conversationID, templatePublicID, true)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if usePending {
				return domaindiscussion.ErrPendingNotFound
			}
			raw, encodeErr := encodeDiscussionSnapshot(&snapshot)
			if encodeErr != nil {
				return encodeErr
			}
			entity = &models.ConversationDiscussionSnapshot{
				ConversationID:     conversationID,
				TemplatePublicID:   templatePublicID,
				ActiveSnapshotJSON: raw,
				ActiveVersion:      1,
			}
			if err = tx.Create(entity).Error; err != nil {
				return translateDiscussionSnapshotCreateError(err)
			}
			committed = true
		} else if err != nil {
			return translateError(err)
		} else if usePending {
			if entity.PendingSnapshotJSON == "" {
				return domaindiscussion.ErrPendingNotFound
			}
			if err = validatePendingVersion(*entity, expectedPendingVersion); err != nil {
				return err
			}
			entity.ActiveSnapshotJSON = entity.PendingSnapshotJSON
			entity.ActiveVersion = nextDiscussionSnapshotVersion(*entity)
			entity.PendingSnapshotJSON = ""
			entity.PendingVersion = 0
			entity.PendingCreatedAt = nil
			if err = tx.Save(entity).Error; err != nil {
				return translateError(err)
			}
			committed = true
		} else if entity.ActiveSnapshotJSON == "" {
			raw, encodeErr := encodeDiscussionSnapshot(&snapshot)
			if encodeErr != nil {
				return encodeErr
			}
			entity.ActiveSnapshotJSON = raw
			entity.ActiveVersion = nextDiscussionSnapshotVersion(*entity)
			if err = tx.Save(entity).Error; err != nil {
				return translateError(err)
			}
			committed = true
		}
		result, err = toDiscussionSnapshotDomain(*entity)
		return err
	})
	return result, committed, err
}
