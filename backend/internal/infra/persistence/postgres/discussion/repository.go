package discussion

import (
	"context"
	"errors"
	"strings"

	domaindiscussion "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/discussion"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
)

// Repo 是讨论模板的 GORM 仓储。
type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) ListDiscussionTemplates(ctx context.Context, status string) ([]domaindiscussion.Template, error) {
	query := r.db.WithContext(ctx).Model(&models.DiscussionTemplate{})
	if value := strings.TrimSpace(status); value != "" {
		query = query.Where("status = ?", value)
	}
	var rows []models.DiscussionTemplate
	if err := query.Order("sort_order asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]domaindiscussion.Template, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainTemplate(row))
	}
	return items, nil
}

func (r *Repo) GetDiscussionTemplateByPublicID(ctx context.Context, publicID string) (*domaindiscussion.Template, error) {
	var row models.DiscussionTemplate
	err := r.db.WithContext(ctx).Where("public_id = ?", strings.TrimSpace(publicID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domaindiscussion.ErrTemplateNotFound
	}
	if err != nil {
		return nil, err
	}
	item := toDomainTemplate(row)
	return &item, nil
}

func (r *Repo) CreateDiscussionTemplate(ctx context.Context, item *domaindiscussion.Template) error {
	if item == nil {
		return domaindiscussion.ErrTemplateInvalid
	}
	row := &models.DiscussionTemplate{
		PublicID:    item.PublicID,
		Name:        item.Name,
		Description: item.Description,
		Icon:        item.Icon,
		Status:      item.Status,
		MaxRounds:   item.MaxRounds,
		ConfigJSON:  item.ConfigJSON,
		SortOrder:   item.SortOrder,
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return err
	}
	item.ID = row.ID
	item.CreatedAt = row.CreatedAt
	item.UpdatedAt = row.UpdatedAt
	return nil
}

func (r *Repo) UpdateDiscussionTemplate(ctx context.Context, item *domaindiscussion.Template) error {
	if item == nil {
		return domaindiscussion.ErrTemplateInvalid
	}
	return r.db.WithContext(ctx).Model(&models.DiscussionTemplate{}).Where("id = ?", item.ID).Updates(map[string]any{
		"name":        item.Name,
		"description": item.Description,
		"icon":        item.Icon,
		"status":      item.Status,
		"max_rounds":  item.MaxRounds,
		"config_json": item.ConfigJSON,
		"sort_order":  item.SortOrder,
	}).Error
}

func (r *Repo) DeleteDiscussionTemplate(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.DiscussionTemplate{}, id).Error
}

func toDomainTemplate(row models.DiscussionTemplate) domaindiscussion.Template {
	return domaindiscussion.Template{
		ID:          row.ID,
		PublicID:    row.PublicID,
		Name:        row.Name,
		Description: row.Description,
		Icon:        row.Icon,
		Status:      row.Status,
		MaxRounds:   row.MaxRounds,
		ConfigJSON:  row.ConfigJSON,
		SortOrder:   row.SortOrder,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

var _ repository.DiscussionTemplateRepository = (*Repo)(nil)
