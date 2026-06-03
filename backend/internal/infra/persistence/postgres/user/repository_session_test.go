package user

import (
	"testing"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
)

func TestSessionAcceptsPresentedRefreshHash(t *testing.T) {
	now := time.Now()
	rotatedAt := now.Add(-5 * time.Second)
	session := model.UserSession{
		RefreshTokenHash:         "current-hash",
		PreviousRefreshTokenHash: "previous-hash",
		RefreshRotatedAt:         &rotatedAt,
		ExpiresAt:                now.Add(time.Hour),
	}

	if !sessionAcceptsPresentedRefreshHash(session, "current-hash", now, 15*time.Second) {
		t.Fatal("expected current refresh hash to be accepted")
	}
	if !sessionAcceptsPresentedRefreshHash(session, "previous-hash", now, 15*time.Second) {
		t.Fatal("expected previous refresh hash inside grace window to be accepted")
	}
	if sessionAcceptsPresentedRefreshHash(session, "previous-hash", now, time.Second) {
		t.Fatal("expected previous refresh hash outside grace window to be rejected")
	}
	if sessionAcceptsPresentedRefreshHash(session, "unknown-hash", now, 15*time.Second) {
		t.Fatal("expected unknown refresh hash to be rejected")
	}
}

func TestSessionRejectsPresentedRefreshHashForInactiveSession(t *testing.T) {
	now := time.Now()
	revokedAt := now
	revokedSession := model.UserSession{
		RefreshTokenHash: "current-hash",
		ExpiresAt:        now.Add(time.Hour),
		RevokedAt:        &revokedAt,
	}
	if sessionAcceptsPresentedRefreshHash(revokedSession, "current-hash", now, 15*time.Second) {
		t.Fatal("expected revoked session to reject refresh hash")
	}

	expiredSession := model.UserSession{
		RefreshTokenHash: "current-hash",
		ExpiresAt:        now.Add(-time.Second),
	}
	if sessionAcceptsPresentedRefreshHash(expiredSession, "current-hash", now, 15*time.Second) {
		t.Fatal("expected expired session to reject refresh hash")
	}
}
