package userapikey

import (
	"context"
	"strings"
	"testing"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	domainuserapikey "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/userapikey"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type memoryAPIKeyRepo struct {
	items []domainuserapikey.UserAPIKey
	seq   uint
}

func (r *memoryAPIKeyRepo) Create(_ context.Context, item *domainuserapikey.UserAPIKey) error {
	r.seq++
	item.ID = r.seq
	item.CreatedAt = time.Now()
	item.UpdatedAt = item.CreatedAt
	clone := *item
	r.items = append(r.items, clone)
	return nil
}

func (r *memoryAPIKeyRepo) ListByUserID(_ context.Context, userID uint) ([]domainuserapikey.UserAPIKey, error) {
	out := make([]domainuserapikey.UserAPIKey, 0)
	for _, item := range r.items {
		if item.UserID == userID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (r *memoryAPIKeyRepo) GetByPublicID(_ context.Context, userID uint, publicID string) (*domainuserapikey.UserAPIKey, error) {
	for i := range r.items {
		if r.items[i].UserID == userID && r.items[i].PublicID == publicID {
			item := r.items[i]
			return &item, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *memoryAPIKeyRepo) GetActiveByHash(_ context.Context, keyHash string) (*domainuserapikey.UserAPIKey, error) {
	now := time.Now()
	for i := range r.items {
		item := r.items[i]
		if item.KeyHash == keyHash && item.RevokedAt == nil && (item.ExpiresAt == nil || item.ExpiresAt.After(now)) {
			return &item, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *memoryAPIKeyRepo) Revoke(_ context.Context, userID uint, publicID string, revokedAt time.Time) error {
	for i := range r.items {
		if r.items[i].UserID == userID && r.items[i].PublicID == publicID && r.items[i].RevokedAt == nil {
			r.items[i].RevokedAt = &revokedAt
			return nil
		}
	}
	return repository.ErrNotFound
}

func (r *memoryAPIKeyRepo) TouchLastUsed(_ context.Context, id uint, usedAt time.Time) error {
	for i := range r.items {
		if r.items[i].ID == id {
			r.items[i].LastUsedAt = &usedAt
			return nil
		}
	}
	return nil
}

type staticUserLookup struct {
	user *domainuser.User
}

func (s staticUserLookup) GetByID(_ context.Context, userID uint) (*domainuser.User, error) {
	if s.user == nil || s.user.ID != userID {
		return nil, repository.ErrNotFound
	}
	return s.user, nil
}

type staticConfig struct {
	enabled bool
}

func (s staticConfig) Snapshot() config.Config {
	return config.Config{UserAPIKeysEnabled: s.enabled}
}

func TestCreateAndAuthenticateAPIKey(t *testing.T) {
	repo := &memoryAPIKeyRepo{}
	svc := NewService(repo, staticUserLookup{user: &domainuser.User{ID: 7, Status: domainuser.StatusActive}}, staticConfig{enabled: true})
	created, err := svc.Create(context.Background(), 7, "cursor")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !strings.HasPrefix(created.Plaintext, "sk-") {
		t.Fatalf("plaintext = %q, want sk- prefix", created.Plaintext)
	}
	auth, err := svc.Authenticate(context.Background(), created.Plaintext)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if auth.UserID != 7 {
		t.Fatalf("Authenticate user = %d, want 7", auth.UserID)
	}
	if err := svc.Revoke(context.Background(), 7, created.Key.PublicID); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if _, err := svc.Authenticate(context.Background(), created.Plaintext); err != ErrInvalidKey {
		t.Fatalf("Authenticate after revoke = %v, want ErrInvalidKey", err)
	}
}

func TestAPIKeyDisabled(t *testing.T) {
	svc := NewService(&memoryAPIKeyRepo{}, nil, staticConfig{enabled: false})
	if _, err := svc.Create(context.Background(), 1, "x"); err != ErrDisabled {
		t.Fatalf("Create disabled = %v, want ErrDisabled", err)
	}
}
