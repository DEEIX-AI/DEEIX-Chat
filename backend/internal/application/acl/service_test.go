package acl

import (
	"context"
	"testing"
	"time"

	domainacl "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/acl"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type memoryACLRepo struct {
	entries []domainacl.Entry
	seq     uint
}

func (r *memoryACLRepo) Upsert(_ context.Context, entry *domainacl.Entry) error {
	for i := range r.entries {
		if r.entries[i].ResourceType == entry.ResourceType &&
			r.entries[i].ResourcePublicID == entry.ResourcePublicID &&
			r.entries[i].GranteeUserID == entry.GranteeUserID {
			r.entries[i].Role = entry.Role
			r.entries[i].UpdatedAt = time.Now()
			*entry = r.entries[i]
			return nil
		}
	}
	r.seq++
	entry.ID = r.seq
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = entry.CreatedAt
	r.entries = append(r.entries, *entry)
	return nil
}

func (r *memoryACLRepo) Delete(_ context.Context, resourceType string, resourcePublicID string, granteeUserID uint) error {
	for i := range r.entries {
		if r.entries[i].ResourceType == resourceType &&
			r.entries[i].ResourcePublicID == resourcePublicID &&
			r.entries[i].GranteeUserID == granteeUserID {
			r.entries = append(r.entries[:i], r.entries[i+1:]...)
			return nil
		}
	}
	return repository.ErrNotFound
}

func (r *memoryACLRepo) ListByResource(_ context.Context, resourceType string, resourcePublicID string) ([]domainacl.Entry, error) {
	out := make([]domainacl.Entry, 0)
	for _, item := range r.entries {
		if item.ResourceType == resourceType && item.ResourcePublicID == resourcePublicID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (r *memoryACLRepo) Get(_ context.Context, resourceType string, resourcePublicID string, granteeUserID uint) (*domainacl.Entry, error) {
	for i := range r.entries {
		if r.entries[i].ResourceType == resourceType &&
			r.entries[i].ResourcePublicID == resourcePublicID &&
			r.entries[i].GranteeUserID == granteeUserID {
			item := r.entries[i]
			return &item, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *memoryACLRepo) ListResourcePublicIDsByGrantee(_ context.Context, resourceType string, granteeUserID uint) ([]string, error) {
	out := make([]string, 0)
	for _, item := range r.entries {
		if item.ResourceType == resourceType && item.GranteeUserID == granteeUserID {
			out = append(out, item.ResourcePublicID)
		}
	}
	return out, nil
}

type aclUserLookup struct {
	byName map[string]*domainuser.User
	byID   map[uint]*domainuser.User
}

func (a aclUserLookup) GetByUsername(_ context.Context, username string) (*domainuser.User, error) {
	user, ok := a.byName[username]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return user, nil
}

func (a aclUserLookup) GetByID(_ context.Context, userID uint) (*domainuser.User, error) {
	user, ok := a.byID[userID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return user, nil
}

type aclConfig struct{ enabled bool }

func (c aclConfig) Snapshot() config.Config {
	return config.Config{ResourceSharingEnabled: c.enabled}
}

func TestGrantAndCanAccess(t *testing.T) {
	owner := &domainuser.User{ID: 1, Username: "owner", Status: domainuser.StatusActive}
	peer := &domainuser.User{ID: 2, Username: "peer", Status: domainuser.StatusActive}
	users := aclUserLookup{
		byName: map[string]*domainuser.User{"owner": owner, "peer": peer},
		byID:   map[uint]*domainuser.User{1: owner, 2: peer},
	}
	svc := NewService(&memoryACLRepo{}, users, aclConfig{enabled: true})
	entry, err := svc.Grant(context.Background(), GrantInput{
		ActorUserID:      1,
		OwnerUserID:      1,
		ResourceType:     domainacl.ResourceTypeConversation,
		ResourcePublicID: "abc123",
		GranteeUsername:  "peer",
		Role:             domainacl.RoleEditor,
	})
	if err != nil {
		t.Fatalf("Grant() error = %v", err)
	}
	if entry.GranteeUserID != 2 || entry.Role != domainacl.RoleEditor {
		t.Fatalf("unexpected entry %#v", entry)
	}
	ok, role, err := svc.CanAccess(context.Background(), 2, 1, domainacl.ResourceTypeConversation, "abc123", domainacl.RoleEditor)
	if err != nil || !ok || role != domainacl.RoleEditor {
		t.Fatalf("CanAccess editor = (%v,%q,%v)", ok, role, err)
	}
	ok, role, err = svc.CanAccess(context.Background(), 1, 1, domainacl.ResourceTypeConversation, "abc123", domainacl.RoleOwner)
	if err != nil || !ok || role != domainacl.RoleOwner {
		t.Fatalf("CanAccess owner = (%v,%q,%v)", ok, role, err)
	}
}

func TestRoleAtLeast(t *testing.T) {
	if !domainacl.RoleAtLeast(domainacl.RoleEditor, domainacl.RoleViewer) {
		t.Fatal("editor should satisfy viewer")
	}
	if domainacl.RoleAtLeast(domainacl.RoleViewer, domainacl.RoleEditor) {
		t.Fatal("viewer should not satisfy editor")
	}
}
