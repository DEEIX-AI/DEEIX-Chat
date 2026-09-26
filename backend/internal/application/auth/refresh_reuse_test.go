package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/token"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
)

// refreshReuseRepo 模拟仓储对刷新令牌轮换与重用检测的行为，记录服务层的调用结果。
type refreshReuseRepo struct {
	repository.AuthRepository
	user    *domainuser.User
	session *domainuser.Session
	// rotateResult 由测试指定：仓储层对本次轮换的裁决。
	rotateResult error
	revoked      bool
	events       []repository.AuthEventInput
}

func (r *refreshReuseRepo) GetSessionByUserAndSessionID(_ context.Context, userID uint, sessionID string) (*domainuser.Session, error) {
	if r.session == nil || r.session.UserID != userID || r.session.SessionID != sessionID {
		return nil, repository.ErrNotFound
	}
	return r.session, nil
}

func (r *refreshReuseRepo) GetByID(_ context.Context, id uint) (*domainuser.User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, repository.ErrNotFound
	}
	return r.user, nil
}

func (r *refreshReuseRepo) RotateSessionTokens(_ context.Context, _ repository.RotateSessionTokensInput) error {
	if errors.Is(r.rotateResult, repository.ErrRefreshTokenReuse) {
		// 与真实仓储一致：重用检测在同一事务里吊销会话。
		r.revoked = true
	}
	return r.rotateResult
}

func (r *refreshReuseRepo) TouchSessionActivity(_ context.Context, _ uint, _ string, _ repository.UpdateSessionActivityInput) error {
	return nil
}

func (r *refreshReuseRepo) RecordAuthEvent(_ context.Context, input repository.AuthEventInput) error {
	r.events = append(r.events, input)
	return nil
}

func (r *refreshReuseRepo) lastEvent() repository.AuthEventInput {
	if len(r.events) == 0 {
		return repository.AuthEventInput{}
	}
	return r.events[len(r.events)-1]
}

func newRefreshReuseFixture(t *testing.T, rotateResult error) (*Service, *refreshReuseRepo, string) {
	t.Helper()
	const secret = "test-secret"
	now := time.Now()
	user := &domainuser.User{ID: 7, Username: "alice", Role: "user", Status: domainuser.StatusActive}
	session := &domainuser.Session{
		UserID:    user.ID,
		SessionID: "sess-1",
		ExpiresAt: now.Add(time.Hour),
	}
	refreshToken, err := token.GenerateWithClaims(token.GenerateClaimsInput{
		Secret: secret, UserID: user.ID, Username: user.Username, Role: user.Role,
		SessionID: session.SessionID, TokenID: "jti-old", TokenType: "refresh", TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}
	repo := &refreshReuseRepo{user: user, session: session, rotateResult: rotateResult}
	service := newTestService(config.Config{
		JWTSecret: secret, TokenTTLHours: 1, RefreshTokenTTLHours: 24,
	}, repo, nil)
	return service, repo, refreshToken
}

// 宽限期外重放已轮换的令牌：会话必须被吊销、返回终态错误，并留下可追溯的审计事件。
func TestRefreshRevokesSessionOnRefreshTokenReuse(t *testing.T) {
	service, repo, refreshToken := newRefreshReuseFixture(t, repository.ErrRefreshTokenReuse)

	_, err := service.Refresh(context.Background(), refreshToken, "req-1", requestmeta.SessionAuditContext{ClientIP: "203.0.113.9"})

	if !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("err = %v, want ErrSessionRevoked (a terminating code the clients clear their session on)", err)
	}
	if !repo.revoked {
		t.Fatal("expected the repository to have revoked the whole session")
	}
	event := repo.lastEvent()
	if event.EventType != "token_refresh" || event.Result != "failure" || event.Reason != "refresh_token_reuse_detected" {
		t.Fatalf("audit event = %+v, want token_refresh/failure/refresh_token_reuse_detected", event)
	}
	if event.UserID != 7 || event.ClientIP != "203.0.113.9" {
		t.Fatalf("audit event must carry user and client IP for incident triage: %+v", event)
	}
}

// 普通的无效令牌（从未属于该会话）不是重用：不吊销，保持原有的 401 invalid_refresh_token。
func TestRefreshUnknownTokenDoesNotRevokeSession(t *testing.T) {
	service, repo, refreshToken := newRefreshReuseFixture(t, repository.ErrInvalidInput)

	_, err := service.Refresh(context.Background(), refreshToken, "req-2", requestmeta.SessionAuditContext{})

	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("err = %v, want ErrInvalidRefreshToken", err)
	}
	if repo.revoked {
		t.Fatal("an unknown token must not revoke the session")
	}
	if repo.lastEvent().Reason != "refresh_token_hash_mismatch" {
		t.Fatalf("audit reason = %q", repo.lastEvent().Reason)
	}
}
