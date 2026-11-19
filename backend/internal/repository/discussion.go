package repository

import (
	"context"

	domaindiscussion "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/discussion"
)

// DiscussionTemplateRepository 提供讨论模板的持久化访问。
type DiscussionTemplateRepository interface {
	ListDiscussionTemplates(ctx context.Context, status string) ([]domaindiscussion.Template, error)
	GetDiscussionTemplateByPublicID(ctx context.Context, publicID string) (*domaindiscussion.Template, error)
	CreateDiscussionTemplate(ctx context.Context, item *domaindiscussion.Template) error
	UpdateDiscussionTemplate(ctx context.Context, item *domaindiscussion.Template) error
	DeleteDiscussionTemplate(ctx context.Context, id uint) error
}
