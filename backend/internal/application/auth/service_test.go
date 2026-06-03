package auth

import (
	"context"
	"testing"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
)

type validateAccessSessionRepo struct {
	repository.AuthRepository
	session *domainuser.Session
}

func (r *validateAccessSessionRepo) GetSessionByUserAndSessionID(_ context.Context, userID uint, sessionID string) (*domainuser.Session, error) {
	if r.session == nil || r.session.UserID != userID || r.session.SessionID != sessionID {
		return nil, repository.ErrNotFound
	}
	return r.session, nil
}

func (r *validateAccessSessionRepo) TouchSessionActivity(_ context.Context, _ uint, _ string, _ repository.UpdateSessionActivityInput) error {
	return nil
}

func TestNormalizeAppearancePreferencesAllowsFontSize(t *testing.T) {
	for _, fontSize := range []string{"small", "standard", "medium", "large"} {
		payload := `{"theme":"system","preset":"default","chatFont":"default","chatFontWeight":"regular","fontSize":"` + fontSize + `"}`

		if _, err := normalizeAppearancePreferences(payload); err != nil {
			t.Fatalf("expected fontSize %q appearance preference to be valid, got %v", fontSize, err)
		}
	}
}

func TestNormalizeAppearancePreferencesDefaultsInvalidFontSize(t *testing.T) {
	payload := `{"fontSize":"huge"}`

	normalized, err := normalizeAppearancePreferences(payload)
	if err != nil {
		t.Fatalf("expected invalid fontSize appearance preference to fall back, got %v", err)
	}
	if normalized != `{"fontSize":"standard"}` {
		t.Fatalf("expected invalid fontSize to fall back to standard, got %s", normalized)
	}
}

func TestNormalizeAppearancePreferencesRejectsUnknownKey(t *testing.T) {
	payload := `{"fontSize":"standard","unknown":"value"}`

	if _, err := normalizeAppearancePreferences(payload); err == nil {
		t.Fatal("expected unknown appearance preference key to be rejected")
	}
}

func TestValidateAccessSessionAllowsTokenIssuedBeforeLatestRefresh(t *testing.T) {
	now := time.Now()
	createdAt := now.Add(-30 * time.Minute)
	lastSeenAt := now
	service := &Service{
		repo: &validateAccessSessionRepo{
			session: &domainuser.Session{
				SessionID:  "session-id",
				UserID:     1,
				AccessJTI:  "latest-access-jti",
				CreatedAt:  createdAt,
				IssuedAt:   now,
				LastSeenAt: &lastSeenAt,
				ExpiresAt:  now.Add(24 * time.Hour),
			},
		},
	}

	err := service.ValidateAccessSession(
		context.Background(),
		1,
		"session-id",
		createdAt.Add(5*time.Minute),
		requestmeta.SessionAuditContext{},
	)
	if err != nil {
		t.Fatalf("expected access token issued before latest refresh to remain valid, got %v", err)
	}
}

func TestValidateAccessSessionRejectsTokenBeforeSessionCreation(t *testing.T) {
	now := time.Now()
	createdAt := now.Add(-30 * time.Minute)
	lastSeenAt := now
	service := &Service{
		repo: &validateAccessSessionRepo{
			session: &domainuser.Session{
				SessionID:  "session-id",
				UserID:     1,
				CreatedAt:  createdAt,
				LastSeenAt: &lastSeenAt,
				ExpiresAt:  now.Add(24 * time.Hour),
			},
		},
	}

	err := service.ValidateAccessSession(
		context.Background(),
		1,
		"session-id",
		createdAt.Add(-accessTokenSessionClockSkew-time.Second),
		requestmeta.SessionAuditContext{},
	)
	if err == nil {
		t.Fatal("expected access token issued before session creation to be rejected")
	}
}
