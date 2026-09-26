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

	if classifyPresentedRefreshHash(session, "current-hash", now, 15*time.Second) != refreshHashCurrent {
		t.Fatal("expected current refresh hash to be accepted")
	}
	if classifyPresentedRefreshHash(session, "previous-hash", now, 15*time.Second) != refreshHashPreviousInGrace {
		t.Fatal("expected previous refresh hash inside grace window to be accepted")
	}
	if classifyPresentedRefreshHash(session, "previous-hash", now, time.Second) != refreshHashReused {
		t.Fatal("expected previous refresh hash outside grace window to be flagged as reuse")
	}
	if classifyPresentedRefreshHash(session, "unknown-hash", now, 15*time.Second) != refreshHashUnknown {
		t.Fatal("expected unknown refresh hash to be rejected")
	}
}

// 三种结果必须区分：当前令牌 → 轮换；上一枚且在宽限期内 → 轮换（容忍丢失响应）；
// 上一枚且超出宽限期 → 令牌重用，必须吊销会话而不是当作普通无效令牌。
func TestClassifyPresentedRefreshHash(t *testing.T) {
	now := time.Now()
	rotatedAt := now.Add(-5 * time.Second)
	session := model.UserSession{
		RefreshTokenHash:         "current-hash",
		PreviousRefreshTokenHash: "previous-hash",
		RefreshRotatedAt:         &rotatedAt,
		ExpiresAt:                now.Add(time.Hour),
	}
	cases := []struct {
		name  string
		hash  string
		grace time.Duration
		want  refreshHashMatch
	}{
		{"current", "current-hash", 15 * time.Second, refreshHashCurrent},
		{"previous inside grace", "previous-hash", 15 * time.Second, refreshHashPreviousInGrace},
		{"previous outside grace is reuse", "previous-hash", time.Second, refreshHashReused},
		{"previous with grace disabled is reuse", "previous-hash", 0, refreshHashReused},
		{"unknown", "unknown-hash", 15 * time.Second, refreshHashUnknown},
		{"empty", "", 15 * time.Second, refreshHashUnknown},
	}
	for _, tc := range cases {
		if got := classifyPresentedRefreshHash(session, tc.hash, now, tc.grace); got != tc.want {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}

	// 从未轮换过的会话没有"上一枚"，任何非当前令牌都只是无效，不构成重用。
	fresh := model.UserSession{RefreshTokenHash: "current-hash", ExpiresAt: now.Add(time.Hour)}
	if got := classifyPresentedRefreshHash(fresh, "previous-hash", now, 15*time.Second); got != refreshHashUnknown {
		t.Fatalf("fresh session: got %v, want unknown", got)
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
	if classifyPresentedRefreshHash(revokedSession, "current-hash", now, 15*time.Second) != refreshHashUnknown {
		t.Fatal("expected revoked session to reject refresh hash")
	}

	expiredSession := model.UserSession{
		RefreshTokenHash: "current-hash",
		ExpiresAt:        now.Add(-time.Second),
	}
	if classifyPresentedRefreshHash(expiredSession, "current-hash", now, 15*time.Second) != refreshHashUnknown {
		t.Fatal("expected expired session to reject refresh hash")
	}
}
