package knowledgebase

import (
	"context"

	appacl "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/acl"
	domainacl "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/acl"
	domainknowledgebase "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/knowledgebase"
)

type resourceACL interface {
	CanAccess(ctx context.Context, userID uint, ownerUserID uint, resourceType string, resourcePublicID string, minRole string) (bool, string, error)
	ListSharedResourcePublicIDs(ctx context.Context, resourceType string, granteeUserID uint) ([]string, error)
	Grant(ctx context.Context, input appacl.GrantInput) (*domainacl.Entry, error)
	Revoke(ctx context.Context, actorUserID uint, ownerUserID uint, resourceType string, resourcePublicID string, granteeUserID uint) error
	List(ctx context.Context, actorUserID uint, ownerUserID uint, resourceType string, resourcePublicID string) ([]domainacl.Entry, error)
}

// SetACLService 注入资源 ACL。
func (s *Service) SetACLService(acl resourceACL) {
	s.acl = acl
}

func (s *Service) isVisibleToUser(ctx context.Context, item *domainknowledgebase.KnowledgeBase, userID uint) bool {
	if item == nil || !item.Enabled {
		return false
	}
	if item.Scope == domainknowledgebase.ScopeBuiltin {
		return true
	}
	if item.Scope == domainknowledgebase.ScopeUser && item.OwnerUserID == userID {
		return true
	}
	if s.acl == nil || item.Scope != domainknowledgebase.ScopeUser {
		return false
	}
	ok, _, err := s.acl.CanAccess(ctx, userID, item.OwnerUserID, domainacl.ResourceTypeKnowledgeBase, item.PublicID, domainacl.RoleViewer)
	return err == nil && ok
}

func (s *Service) canEditKnowledgeBase(ctx context.Context, item *domainknowledgebase.KnowledgeBase, userID uint) bool {
	if item == nil || item.Scope != domainknowledgebase.ScopeUser {
		return false
	}
	if item.OwnerUserID == userID {
		return true
	}
	if s.acl == nil {
		return false
	}
	ok, _, err := s.acl.CanAccess(ctx, userID, item.OwnerUserID, domainacl.ResourceTypeKnowledgeBase, item.PublicID, domainacl.RoleEditor)
	return err == nil && ok
}

// GrantKnowledgeBaseACL 所有者授权。
func (s *Service) GrantKnowledgeBaseACL(ctx context.Context, actorUserID uint, publicID string, granteeUsername string, role string) (*domainacl.Entry, error) {
	item, err := s.get(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if item.Scope != domainknowledgebase.ScopeUser || item.OwnerUserID != actorUserID {
		return nil, ErrKnowledgeBaseNotFound
	}
	if s.acl == nil {
		return nil, appacl.ErrDisabled
	}
	return s.acl.Grant(ctx, appacl.GrantInput{
		ActorUserID:      actorUserID,
		OwnerUserID:      item.OwnerUserID,
		ResourceType:     domainacl.ResourceTypeKnowledgeBase,
		ResourcePublicID: item.PublicID,
		GranteeUsername:  granteeUsername,
		Role:             role,
	})
}

// RevokeKnowledgeBaseACL 所有者撤销。
func (s *Service) RevokeKnowledgeBaseACL(ctx context.Context, actorUserID uint, publicID string, granteeUserID uint) error {
	item, err := s.get(ctx, publicID)
	if err != nil {
		return err
	}
	if item.Scope != domainknowledgebase.ScopeUser || item.OwnerUserID != actorUserID {
		return ErrKnowledgeBaseNotFound
	}
	if s.acl == nil {
		return appacl.ErrDisabled
	}
	return s.acl.Revoke(ctx, actorUserID, item.OwnerUserID, domainacl.ResourceTypeKnowledgeBase, item.PublicID, granteeUserID)
}

// ListKnowledgeBaseACL 列出授权。
func (s *Service) ListKnowledgeBaseACL(ctx context.Context, actorUserID uint, publicID string) ([]domainacl.Entry, error) {
	item, err := s.get(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if item.Scope != domainknowledgebase.ScopeUser || item.OwnerUserID != actorUserID {
		return nil, ErrKnowledgeBaseNotFound
	}
	if s.acl == nil {
		return nil, appacl.ErrDisabled
	}
	return s.acl.List(ctx, actorUserID, item.OwnerUserID, domainacl.ResourceTypeKnowledgeBase, item.PublicID)
}

func (s *Service) sharedKnowledgeBasePublicIDs(ctx context.Context, userID uint) ([]string, error) {
	if s.acl == nil {
		return nil, nil
	}
	return s.acl.ListSharedResourcePublicIDs(ctx, domainacl.ResourceTypeKnowledgeBase, userID)
}
