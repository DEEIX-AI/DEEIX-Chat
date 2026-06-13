package announcement

import (
	"context"
	"strings"
	"time"

	domainannouncement "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/announcement"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repo 封装公告数据访问。
type Repo struct {
	db *gorm.DB
}

// NewRepo 创建公告仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// ListActiveAnnouncements 查询当前可展示公告。
func (r *Repo) ListActiveAnnouncements(ctx context.Context, userID uint, now time.Time) ([]domainannouncement.Announcement, error) {
	type announcementRow struct {
		model.Announcement
		UserClosedAt *time.Time `gorm:"column:user_closed_at"`
	}
	items := make([]announcementRow, 0)
	if err := r.db.WithContext(ctx).
		Model(&model.Announcement{}).
		Select("system_announcements.*, states.closed_at AS user_closed_at").
		Joins(`LEFT JOIN announcement_user_states states
			ON states.deleted_at IS NULL
				AND states.announcement_id = system_announcements.id
				AND states.user_id = ?
				AND states.announcement_updated_at = system_announcements.updated_at`, userID).
		Where("status = ?", domainannouncement.StatusActive).
		Where("(starts_at IS NULL OR starts_at <= ?)", now).
		Where("(expires_at IS NULL OR expires_at > ?)", now).
		Where("(states.dismissed_until IS NULL OR states.dismissed_until <= ?)", now).
		Order("CASE WHEN states.closed_at IS NULL THEN 0 ELSE 1 END ASC, pinned DESC, priority DESC, updated_at DESC, id DESC").
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	results := make([]domainannouncement.Announcement, 0, len(items))
	for _, item := range items {
		announcement := toDomain(item.Announcement)
		announcement.ClosedAt = item.UserClosedAt
		results = append(results, announcement)
	}
	return results, nil
}

// ListAdminAnnouncements 分页查询后台公告。
func (r *Repo) ListAdminAnnouncements(ctx context.Context, filter repository.AnnouncementListFilter, offset int, limit int) ([]domainannouncement.Announcement, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	items := make([]model.Announcement, 0, limit)
	var total int64
	query := r.db.WithContext(ctx).Model(&model.Announcement{})
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	if announcementType := strings.TrimSpace(filter.Type); announcementType != "" {
		query = query.Where("type = ?", announcementType)
	}
	if filter.Pinned != nil {
		query = query.Where("pinned = ?", *filter.Pinned)
	}
	if keyword := strings.TrimSpace(filter.Query); keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Where("LOWER(title) LIKE ? OR LOWER(content_markdown) LIKE ?", like, like)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	if err := query.
		Order("pinned DESC, priority DESC, updated_at DESC, id DESC").
		Offset(offset).
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}
	results := make([]domainannouncement.Announcement, 0, len(items))
	for _, item := range items {
		results = append(results, toDomain(item))
	}
	return results, total, nil
}

// CreateAnnouncement 创建公告。
func (r *Repo) CreateAnnouncement(ctx context.Context, item *domainannouncement.Announcement) (*domainannouncement.Announcement, error) {
	if item == nil {
		return nil, repository.ErrInvalidInput
	}
	record := model.Announcement{
		Title:           strings.TrimSpace(item.Title),
		ContentMarkdown: strings.TrimSpace(item.ContentMarkdown),
		Status:          normalizeStatus(item.Status),
		Type:            normalizeType(item.Type),
		Pinned:          item.Pinned,
		Priority:        item.Priority,
		StartsAt:        item.StartsAt,
		ExpiresAt:       item.ExpiresAt,
		CreatedByUserID: item.CreatedByUserID,
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return nil, translateError(err)
	}
	result := toDomain(record)
	return &result, nil
}

// PatchAnnouncement 更新公告字段。
func (r *Repo) PatchAnnouncement(ctx context.Context, id uint, patch repository.AnnouncementPatch) (*domainannouncement.Announcement, error) {
	if id == 0 {
		return nil, repository.ErrInvalidInput
	}
	var result domainannouncement.Announcement
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record model.Announcement
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).
			First(&record).Error; err != nil {
			return translateError(err)
		}
		updates := map[string]interface{}{}
		if patch.Title != nil {
			updates["title"] = strings.TrimSpace(*patch.Title)
		}
		if patch.ContentMarkdown != nil {
			updates["content_markdown"] = strings.TrimSpace(*patch.ContentMarkdown)
		}
		if patch.Status != nil {
			status := normalizeStatus(*patch.Status)
			if status == "" {
				return repository.ErrInvalidInput
			}
			updates["status"] = status
		}
		if patch.Type != nil {
			announcementType := normalizeType(*patch.Type)
			if announcementType == "" {
				return repository.ErrInvalidInput
			}
			updates["type"] = announcementType
		}
		if patch.Pinned != nil {
			updates["pinned"] = *patch.Pinned
		}
		if patch.Priority != nil {
			updates["priority"] = *patch.Priority
		}
		if patch.StartsAtSet {
			updates["starts_at"] = patch.StartsAt
		}
		if patch.ExpiresAtSet {
			updates["expires_at"] = patch.ExpiresAt
		}
		nextStartsAt := record.StartsAt
		if patch.StartsAtSet {
			nextStartsAt = patch.StartsAt
		}
		nextExpiresAt := record.ExpiresAt
		if patch.ExpiresAtSet {
			nextExpiresAt = patch.ExpiresAt
		}
		if nextStartsAt != nil && nextExpiresAt != nil && !nextExpiresAt.After(*nextStartsAt) {
			return repository.ErrInvalidInput
		}
		if patch.CreatedByUserIDSet {
			updates["created_by_user_id"] = patch.CreatedByUserID
		}
		if len(updates) > 0 {
			if err := tx.Model(&record).Updates(updates).Error; err != nil {
				return translateError(err)
			}
		}
		if err := tx.Where("id = ?", id).First(&record).Error; err != nil {
			return translateError(err)
		}
		result = toDomain(record)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteAnnouncement 软删除公告。
func (r *Repo) DeleteAnnouncement(ctx context.Context, id uint) error {
	if id == 0 {
		return repository.ErrInvalidInput
	}
	result := r.db.WithContext(ctx).Delete(&model.Announcement{}, id)
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// DismissAnnouncementToday 记录用户今天不再显示指定公告版本。
func (r *Repo) DismissAnnouncementToday(ctx context.Context, userID uint, announcementID uint, announcementUpdatedAt time.Time, now time.Time, dismissedUntil time.Time) error {
	if userID == 0 || announcementID == 0 || announcementUpdatedAt.IsZero() || !dismissedUntil.After(now) {
		return repository.ErrInvalidInput
	}
	return r.saveUserState(ctx, userID, announcementID, announcementUpdatedAt, now, &dismissedUntil, nil)
}

// CloseAnnouncement 记录用户关闭指定公告版本。
func (r *Repo) CloseAnnouncement(ctx context.Context, userID uint, announcementID uint, announcementUpdatedAt time.Time, now time.Time) error {
	if userID == 0 || announcementID == 0 || announcementUpdatedAt.IsZero() {
		return repository.ErrInvalidInput
	}
	return r.saveUserState(ctx, userID, announcementID, announcementUpdatedAt, now, nil, &now)
}

func (r *Repo) saveUserState(ctx context.Context, userID uint, announcementID uint, announcementUpdatedAt time.Time, now time.Time, dismissedUntil *time.Time, closedAt *time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var announcement model.Announcement
		if err := tx.
			Where("id = ?", announcementID).
			Where("updated_at = ?", announcementUpdatedAt).
			Where("status = ?", domainannouncement.StatusActive).
			Where("(starts_at IS NULL OR starts_at <= ?)", now).
			Where("(expires_at IS NULL OR expires_at > ?)", now).
			First(&announcement).Error; err != nil {
			return translateError(err)
		}

		state := model.AnnouncementUserState{
			AnnouncementID:        announcement.ID,
			UserID:                userID,
			AnnouncementUpdatedAt: announcement.UpdatedAt,
			DismissedUntil:        dismissedUntil,
			ClosedAt:              closedAt,
		}
		assignments := []string{"updated_at", "deleted_at"}
		if dismissedUntil != nil {
			assignments = append(assignments, "dismissed_until")
		}
		if closedAt != nil {
			assignments = append(assignments, "closed_at")
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "announcement_id"},
				{Name: "user_id"},
				{Name: "announcement_updated_at"},
			},
			DoUpdates: clause.AssignmentColumns(assignments),
		}).Create(&state).Error; err != nil {
			return translateError(err)
		}
		return nil
	})
}

func toDomain(item model.Announcement) domainannouncement.Announcement {
	return domainannouncement.Announcement{
		ID:              item.ID,
		Title:           item.Title,
		ContentMarkdown: item.ContentMarkdown,
		Status:          item.Status,
		Type:            item.Type,
		Pinned:          item.Pinned,
		Priority:        item.Priority,
		StartsAt:        item.StartsAt,
		ExpiresAt:       item.ExpiresAt,
		CreatedByUserID: item.CreatedByUserID,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

func normalizeStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "", domainannouncement.StatusActive:
		return domainannouncement.StatusActive
	case domainannouncement.StatusInactive:
		return domainannouncement.StatusInactive
	default:
		return ""
	}
}

func normalizeType(announcementType string) string {
	switch strings.TrimSpace(announcementType) {
	case "", domainannouncement.TypeGeneral:
		return domainannouncement.TypeGeneral
	case domainannouncement.TypeCritical:
		return domainannouncement.TypeCritical
	case domainannouncement.TypeWarning:
		return domainannouncement.TypeWarning
	case domainannouncement.TypeInfo:
		return domainannouncement.TypeInfo
	case domainannouncement.TypeNormal:
		return domainannouncement.TypeNormal
	default:
		return ""
	}
}

func translateError(err error) error {
	if err == nil {
		return nil
	}
	if dberror.IsRecordNotFound(err) {
		return repository.ErrNotFound
	}
	return err
}
