package repository

import (
	"context"

	domainacl "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/acl"
)

// ACLRepository 定义资源 ACL 持久化能力。
type ACLRepository interface {
	Upsert(ctx context.Context, entry *domainacl.Entry) error
	Delete(ctx context.Context, resourceType string, resourcePublicID string, granteeUserID uint) error
	ListByResource(ctx context.Context, resourceType string, resourcePublicID string) ([]domainacl.Entry, error)
	Get(ctx context.Context, resourceType string, resourcePublicID string, granteeUserID uint) (*domainacl.Entry, error)
	ListResourcePublicIDsByGrantee(ctx context.Context, resourceType string, granteeUserID uint) ([]string, error)
}
