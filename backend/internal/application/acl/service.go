package acl

import (
	"context"
	"errors"
	"strings"

	domainacl "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/acl"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/conv"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
)

var (
	ErrDisabled        = apperr.New("collaboration.resource_sharing_disabled", "resource sharing is disabled")
	ErrInvalidResource = apperr.New("acl.invalid_resource", "invalid resource")
	ErrInvalidRole     = apperr.New("acl.invalid_role", "invalid role")
	ErrInvalidUsername = apperr.New("acl.invalid_username", "invalid username")
	ErrUserNotFound    = apperr.New("acl.user_not_found", "user not found")
	ErrCannotShareSelf = apperr.New("acl.cannot_share_self", "cannot share with yourself")
	ErrEntryNotFound   = apperr.New("acl.entry_not_found", "ACL entry not found")
	ErrForbidden       = apperr.New("acl.forbidden", "forbidden")
)

type runtimeConfig interface {
	Snapshot() config.Config
}

type userLookup interface {
	GetByUsername(ctx context.Context, username string) (*domainuser.User, error)
	GetByID(ctx context.Context, userID uint) (*domainuser.User, error)
}

// Service 封装资源 ACL 业务。
type Service struct {
	repo  repository.ACLRepository
	users userLookup
	cfg   runtimeConfig
}

// NewService 创建服务。
func NewService(repo repository.ACLRepository, users userLookup, cfg runtimeConfig) *Service {
	return &Service{repo: repo, users: users, cfg: cfg}
}

// GrantInput 描述授权请求。
type GrantInput struct {
	ActorUserID      uint
	OwnerUserID      uint
	ResourceType     string
	ResourcePublicID string
	GranteeUsername  string
	Role             string
}

// Grant 由资源所有者授予或更新权限。
func (s *Service) Grant(ctx context.Context, input GrantInput) (*domainacl.Entry, error) {
	if err := s.requireEnabled(); err != nil {
		return nil, err
	}
	if input.ActorUserID == 0 || input.ActorUserID != input.OwnerUserID {
		return nil, ErrForbidden
	}
	resourceType, resourcePublicID, err := normalizeResource(input.ResourceType, input.ResourcePublicID)
	if err != nil {
		return nil, err
	}
	role, err := normalizeRole(input.Role)
	if err != nil {
		return nil, err
	}
	username := strings.TrimSpace(input.GranteeUsername)
	if username == "" {
		return nil, ErrInvalidUsername
	}
	grantee, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if grantee == nil || grantee.Status != domainuser.StatusActive {
		return nil, ErrUserNotFound
	}
	if grantee.ID == input.OwnerUserID {
		return nil, ErrCannotShareSelf
	}
	entry := &domainacl.Entry{
		ResourceType:     resourceType,
		ResourcePublicID: resourcePublicID,
		GranteeUserID:    grantee.ID,
		GranteeUsername:  grantee.Username,
		Role:             role,
	}
	if err := s.repo.Upsert(ctx, entry); err != nil {
		return nil, err
	}
	entry.GranteeUsername = grantee.Username
	return entry, nil
}

// Revoke 由资源所有者撤销授权。
func (s *Service) Revoke(ctx context.Context, actorUserID uint, ownerUserID uint, resourceType string, resourcePublicID string, granteeUserID uint) error {
	if err := s.requireEnabled(); err != nil {
		return err
	}
	if actorUserID == 0 || actorUserID != ownerUserID {
		return ErrForbidden
	}
	resourceType, resourcePublicID, err := normalizeResource(resourceType, resourcePublicID)
	if err != nil {
		return err
	}
	if granteeUserID == 0 {
		return ErrEntryNotFound
	}
	if err := s.repo.Delete(ctx, resourceType, resourcePublicID, granteeUserID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrEntryNotFound
		}
		return err
	}
	return nil
}

// List 列出资源授权（仅所有者）。
func (s *Service) List(ctx context.Context, actorUserID uint, ownerUserID uint, resourceType string, resourcePublicID string) ([]domainacl.Entry, error) {
	if err := s.requireEnabled(); err != nil {
		return nil, err
	}
	if actorUserID == 0 || actorUserID != ownerUserID {
		return nil, ErrForbidden
	}
	resourceType, resourcePublicID, err := normalizeResource(resourceType, resourcePublicID)
	if err != nil {
		return nil, err
	}
	items, err := s.repo.ListByResource(ctx, resourceType, resourcePublicID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if user, userErr := s.users.GetByID(ctx, items[i].GranteeUserID); userErr == nil && user != nil {
			items[i].GranteeUsername = user.Username
		}
	}
	return items, nil
}

// ResolveRole 返回用户对资源的有效角色（含 owner）。
func (s *Service) ResolveRole(ctx context.Context, userID uint, ownerUserID uint, resourceType string, resourcePublicID string) (string, error) {
	if userID == 0 {
		return "", nil
	}
	if userID == ownerUserID {
		return domainacl.RoleOwner, nil
	}
	if s == nil || s.cfg == nil || !s.cfg.Snapshot().ResourceSharingEnabled {
		return "", nil
	}
	resourceType, resourcePublicID, err := normalizeResource(resourceType, resourcePublicID)
	if err != nil {
		return "", err
	}
	entry, err := s.repo.Get(ctx, resourceType, resourcePublicID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	return entry.Role, nil
}

// CanAccess 判断用户是否至少拥有 minRole。
func (s *Service) CanAccess(ctx context.Context, userID uint, ownerUserID uint, resourceType string, resourcePublicID string, minRole string) (bool, string, error) {
	role, err := s.ResolveRole(ctx, userID, ownerUserID, resourceType, resourcePublicID)
	if err != nil {
		return false, "", err
	}
	return domainacl.RoleAtLeast(role, minRole), role, nil
}

// ListSharedResourcePublicIDs 返回共享给用户的资源公开 ID。
func (s *Service) ListSharedResourcePublicIDs(ctx context.Context, resourceType string, granteeUserID uint) ([]string, error) {
	if s == nil || s.cfg == nil || !s.cfg.Snapshot().ResourceSharingEnabled {
		return nil, nil
	}
	resourceType = strings.TrimSpace(resourceType)
	if resourceType == "" || granteeUserID == 0 {
		return nil, nil
	}
	return s.repo.ListResourcePublicIDsByGrantee(ctx, resourceType, granteeUserID)
}

func (s *Service) requireEnabled() error {
	if s == nil || s.cfg == nil || !s.cfg.Snapshot().ResourceSharingEnabled {
		return ErrDisabled
	}
	return nil
}

func normalizeResource(resourceType string, resourcePublicID string) (string, string, error) {
	normalizedType := strings.TrimSpace(resourceType)
	switch normalizedType {
	case domainacl.ResourceTypeConversation, domainacl.ResourceTypeKnowledgeBase:
	default:
		return "", "", ErrInvalidResource
	}
	normalizedID := conv.NormalizePublicID(resourcePublicID)
	if normalizedID == "" {
		return "", "", ErrInvalidResource
	}
	return normalizedType, normalizedID, nil
}

func normalizeRole(role string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(role)) {
	case domainacl.RoleViewer:
		return domainacl.RoleViewer, nil
	case domainacl.RoleEditor:
		return domainacl.RoleEditor, nil
	default:
		return "", ErrInvalidRole
	}
}
