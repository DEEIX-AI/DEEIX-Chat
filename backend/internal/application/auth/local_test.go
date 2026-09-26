package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
)

// localRepo 只实现本地模式登录路径需要的仓储方法。
type localRepo struct {
	repository.AuthRepository
	users      map[string]*domainuser.User
	credential *domainuser.Credential
	created    int
	sessions   int
	events     []repository.AuthEventInput
}

func (r *localRepo) GetByUsername(_ context.Context, username string) (*domainuser.User, error) {
	if u, ok := r.users[username]; ok {
		return u, nil
	}
	return nil, repository.ErrNotFound
}

func (r *localRepo) CreateWithCredential(_ context.Context, input repository.CreateWithCredentialInput) error {
	r.created++
	input.User.ID = uint(len(r.users) + 1)
	r.users[input.User.Username] = input.User
	cred := input.Credential
	r.credential = &cred
	return nil
}

func (r *localRepo) GetCredentialByUserID(_ context.Context, _ uint) (*domainuser.Credential, error) {
	if r.credential == nil {
		return nil, repository.ErrNotFound
	}
	return r.credential, nil
}

func (r *localRepo) ListUserIdentitiesByUserID(_ context.Context, _ uint) ([]domainuser.UserIdentity, error) {
	return nil, nil
}

func (r *localRepo) CreateSession(_ context.Context, _ *domainuser.Session) error {
	r.sessions++
	return nil
}

func (r *localRepo) UpdateLastLogin(_ context.Context, _ uint) error { return nil }

func (r *localRepo) GetUserTwoFactorByUserID(_ context.Context, _ uint) (*domainuser.UserTwoFactor, error) {
	return nil, repository.ErrNotFound
}

func (r *localRepo) RecordAuthEvent(_ context.Context, input repository.AuthEventInput) error {
	r.events = append(r.events, input)
	return nil
}

type noSubscription struct{}

func (noSubscription) GetCurrentSubscriptionSnapshot(_ context.Context, _ uint, _ time.Time) (*billing.UserSubscriptionSnapshot, error) {
	return nil, nil
}

func newLocalFixture(t *testing.T) (*Service, *localRepo) {
	t.Helper()
	repo := &localRepo{users: map[string]*domainuser.User{}}
	cfg := config.Config{JWTSecret: "local-test-secret", TokenTTLHours: 1, RefreshTokenTTLHours: 24}
	if err := cfg.ApplyLocalMode(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	service := newTestService(cfg, repo, nil)
	service.SetSubscriptionResolver(noSubscription{})
	return service, repo
}

func TestEnsureLocalOwnerIsIdempotentAndPasswordless(t *testing.T) {
	service, repo := newLocalFixture(t)

	first, err := service.EnsureLocalOwner(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.EnsureLocalOwner(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if repo.created != 1 || first.ID != second.ID {
		t.Fatalf("owner must be created exactly once, created=%d", repo.created)
	}
	if first.Role != domainuser.RoleSuperAdmin || first.Status != domainuser.StatusActive {
		t.Fatalf("owner must be an active superadmin: %+v", first)
	}
	if first.OnboardingCompletedAt == nil || first.UsernameChangedAt == nil {
		t.Fatal("owner must not be sent through first-login onboarding")
	}
	if repo.credential == nil || repo.credential.PasswordEnabled || repo.credential.MustResetPassword {
		t.Fatalf("owner must have no usable password: %+v", repo.credential)
	}
}

func TestLocalGrantIsSingleUseAndBoundToCurrentIssue(t *testing.T) {
	service, repo := newLocalFixture(t)
	audit := requestmeta.SessionAuditContext{ClientIP: "127.0.0.1"}

	grant, err := service.IssueLocalGrant()
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.ExchangeLocalGrant(context.Background(), grant, "req-1", audit)
	if err != nil {
		t.Fatalf("first exchange must succeed: %v", err)
	}
	if result.AccessToken == "" || result.RefreshToken == "" || result.User.Username != "owner" {
		t.Fatalf("exchange must issue a normal session for the owner: %+v", result.User)
	}
	if repo.sessions != 1 {
		t.Fatalf("expected one persisted session, got %d", repo.sessions)
	}

	if _, err := service.ExchangeLocalGrant(context.Background(), grant, "req-2", audit); !errors.Is(err, ErrLocalGrantInvalid) {
		t.Fatalf("replaying a consumed grant must fail with ErrLocalGrantInvalid, got %v", err)
	}
	if _, err := service.ExchangeLocalGrant(context.Background(), "bogus", "req-3", audit); !errors.Is(err, ErrLocalGrantInvalid) {
		t.Fatalf("unknown grant must fail, got %v", err)
	}

	// 重新签发后，旧 grant 立即作废（即便它从未被使用）。
	stale, _ := service.IssueLocalGrant()
	fresh, _ := service.IssueLocalGrant()
	if _, err := service.ExchangeLocalGrant(context.Background(), stale, "req-4", audit); !errors.Is(err, ErrLocalGrantInvalid) {
		t.Fatalf("superseded grant must fail, got %v", err)
	}
	if _, err := service.ExchangeLocalGrant(context.Background(), fresh, "req-5", audit); err != nil {
		t.Fatalf("current grant must succeed: %v", err)
	}

	failures := 0
	for _, e := range repo.events {
		if e.EventType == "local_grant_exchange" && e.Result == "failure" {
			failures++
		}
	}
	if failures != 3 {
		t.Fatalf("every rejected exchange must leave an audit event, got %d", failures)
	}
}

func TestLocalGrantExpires(t *testing.T) {
	service, _ := newLocalFixture(t)
	grant, _ := service.IssueLocalGrant()

	localGrant.mu.Lock()
	localGrant.expiresAt = time.Now().Add(-time.Second)
	localGrant.mu.Unlock()

	if _, err := service.ExchangeLocalGrant(context.Background(), grant, "req", requestmeta.SessionAuditContext{}); !errors.Is(err, ErrLocalGrantInvalid) {
		t.Fatalf("expired grant must fail, got %v", err)
	}
}
