package conversation

import (
	"context"
	"errors"

	appacl "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/acl"
	domainacl "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/acl"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
)

var ErrConversationForbidden = apperr.New("conversation.forbidden", "conversation access forbidden")

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

func (s *Service) resolveConversationAccess(
	ctx context.Context,
	userID uint,
	item *model.Conversation,
	minRole string,
) (string, error) {
	if item == nil {
		return "", ErrConversationNotFound
	}
	if item.UserID == userID {
		item.AccessRole = domainacl.RoleOwner
		return domainacl.RoleOwner, nil
	}
	if s.acl == nil {
		return "", ErrConversationNotFound
	}
	ok, role, err := s.acl.CanAccess(ctx, userID, item.UserID, domainacl.ResourceTypeConversation, item.PublicID, minRole)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrConversationNotFound
	}
	item.AccessRole = role
	return role, nil
}

// GetConversationForAccess 按公开 ID 解析会话并校验最低角色。
func (s *Service) GetConversationForAccess(ctx context.Context, userID uint, publicID string, minRole string) (*model.Conversation, error) {
	item, err := s.repo.GetConversationByPublicID(ctx, publicID, userID)
	if err == nil {
		item.AccessRole = domainacl.RoleOwner
		return item, nil
	}
	if !errors.Is(err, repository.ErrNotFound) && err != nil {
		// GetConversationByPublicID in service historically maps any error to not found;
		// repo returns ErrNotFound via dberror.
	}
	item, err = s.repo.GetConversationByPublicIDOnly(ctx, publicID)
	if err != nil {
		return nil, ErrConversationNotFound
	}
	if _, accessErr := s.resolveConversationAccess(ctx, userID, item, minRole); accessErr != nil {
		return nil, accessErr
	}
	return item, nil
}

// GrantConversationACL 所有者授权。
func (s *Service) GrantConversationACL(ctx context.Context, actorUserID uint, conversationPublicID string, granteeUsername string, role string) (*domainacl.Entry, error) {
	item, err := s.repo.GetConversationByPublicID(ctx, conversationPublicID, actorUserID)
	if err != nil {
		return nil, ErrConversationNotFound
	}
	if s.acl == nil {
		return nil, appacl.ErrDisabled
	}
	return s.acl.Grant(ctx, appacl.GrantInput{
		ActorUserID:      actorUserID,
		OwnerUserID:      item.UserID,
		ResourceType:     domainacl.ResourceTypeConversation,
		ResourcePublicID: item.PublicID,
		GranteeUsername:  granteeUsername,
		Role:             role,
	})
}

// RevokeConversationACL 所有者撤销授权。
func (s *Service) RevokeConversationACL(ctx context.Context, actorUserID uint, conversationPublicID string, granteeUserID uint) error {
	item, err := s.repo.GetConversationByPublicID(ctx, conversationPublicID, actorUserID)
	if err != nil {
		return ErrConversationNotFound
	}
	if s.acl == nil {
		return appacl.ErrDisabled
	}
	return s.acl.Revoke(ctx, actorUserID, item.UserID, domainacl.ResourceTypeConversation, item.PublicID, granteeUserID)
}

// ListConversationACL 列出授权。
func (s *Service) ListConversationACL(ctx context.Context, actorUserID uint, conversationPublicID string) ([]domainacl.Entry, error) {
	item, err := s.repo.GetConversationByPublicID(ctx, conversationPublicID, actorUserID)
	if err != nil {
		return nil, ErrConversationNotFound
	}
	if s.acl == nil {
		return nil, appacl.ErrDisabled
	}
	return s.acl.List(ctx, actorUserID, item.UserID, domainacl.ResourceTypeConversation, item.PublicID)
}
