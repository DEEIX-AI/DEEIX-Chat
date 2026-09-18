package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"

	"golang.org/x/crypto/bcrypt"
)

type loginLockRepo struct {
	repository.AuthRepository

	userItem   *user.User
	credential *user.Credential

	markCalls     int
	markThreshold int
	statusUpdates []string
}

func (r *loginLockRepo) GetByUsername(context.Context, string) (*user.User, error) {
	if r.userItem == nil {
		return nil, repository.ErrNotFound
	}
	return r.userItem, nil
}

func (r *loginLockRepo) GetCredentialByUserID(context.Context, uint) (*user.Credential, error) {
	if r.credential == nil {
		return nil, repository.ErrNotFound
	}
	return r.credential, nil
}

func (r *loginLockRepo) MarkLoginFailure(_ context.Context, _ uint, lockThreshold int, lockUntil time.Time) (*user.Credential, error) {
	r.markCalls++
	r.markThreshold = lockThreshold

	updated := *r.credential
	updated.FailedLoginCount++
	if lockThreshold > 0 && updated.FailedLoginCount >= lockThreshold {
		locked := lockUntil
		updated.LockedUntil = &locked
	}
	r.credential = &updated
	return &updated, nil
}

func (r *loginLockRepo) ResetLoginFailure(context.Context, uint) error {
	updated := *r.credential
	updated.FailedLoginCount = 0
	updated.LockedUntil = nil
	r.credential = &updated
	return nil
}

func (r *loginLockRepo) UpdateUserStatus(_ context.Context, _ uint, status string) error {
	r.statusUpdates = append(r.statusUpdates, status)
	if r.userItem != nil {
		r.userItem.Status = status
	}
	return nil
}

func (r *loginLockRepo) RecordAuthEvent(context.Context, repository.AuthEventInput) error { return nil }

func newLoginLockService(t *testing.T, cfg config.Config, repo *loginLockRepo) *Service {
	t.Helper()
	return newTestService(cfg, repo, nil)
}

func lockedCredential(t *testing.T, password string, lockedUntil time.Time, status string) (*loginLockRepo, config.Config) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	locked := lockedUntil
	return &loginLockRepo{
			userItem: &user.User{ID: 7, Username: "locked-user", Status: status},
			credential: &user.Credential{
				ID:               11,
				UserID:           7,
				PasswordHash:     string(hash),
				PasswordEnabled:  true,
				FailedLoginCount: 5,
				LockedUntil:      &locked,
			},
		}, config.Config{
			UsernameLoginEnabled: true,
			EmailLoginEnabled:    true,
			LoginMaxFailures:     5,
			LoginLockMinutes:     15,
			JWTSecret:            "test-secret",
		}
}

func TestLoginLockPolicyHonorsDisabledConfiguration(t *testing.T) {
	cases := []struct {
		name       string
		failures   int
		lockMinute int
		wantLimit  int
		wantLocked time.Duration
	}{
		{name: "default", failures: 5, lockMinute: 15, wantLimit: 5, wantLocked: 15 * time.Minute},
		{name: "zero failures disables", failures: 0, lockMinute: 15},
		{name: "zero minutes disables", failures: 5, lockMinute: 0},
		{name: "negative disables", failures: -1, lockMinute: -1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newLoginLockService(t, config.Config{
				LoginMaxFailures: testCase.failures,
				LoginLockMinutes: testCase.lockMinute,
			}, &loginLockRepo{})

			limit, locked := service.loginLockPolicy()
			if limit != testCase.wantLimit || locked != testCase.wantLocked {
				t.Fatalf("loginLockPolicy() = (%d, %v), want (%d, %v)", limit, locked, testCase.wantLimit, testCase.wantLocked)
			}
		})
	}
}

func TestLoginReportsLockedAccountOnlyForCorrectPassword(t *testing.T) {
	repo, cfg := lockedCredential(t, "correct-password", time.Now().Add(10*time.Minute), user.StatusLocked)
	service := newLoginLockService(t, cfg, repo)

	_, err := service.Login(context.Background(), "locked-user", "correct-password", "req-1", requestmeta.SessionAuditContext{})
	if !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("correct password on locked account = %v, want ErrAccountLocked", err)
	}

	var lockedErr *AccountLockedError
	if !errors.As(err, &lockedErr) {
		t.Fatalf("expected AccountLockedError, got %T", err)
	}
	if lockedErr.RetryAfter <= 0 || lockedErr.RetryAfter > 10*time.Minute {
		t.Fatalf("RetryAfter = %v, want within (0, 10m]", lockedErr.RetryAfter)
	}
	if repo.markCalls != 0 {
		t.Fatalf("locked attempt must not record another failure, got %d calls", repo.markCalls)
	}
}

func TestLoginKeepsGenericErrorForWrongPasswordOnLockedAccount(t *testing.T) {
	repo, cfg := lockedCredential(t, "correct-password", time.Now().Add(10*time.Minute), user.StatusLocked)
	service := newLoginLockService(t, cfg, repo)

	_, err := service.Login(context.Background(), "locked-user", "wrong-password", "req-1", requestmeta.SessionAuditContext{})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password on locked account = %v, want ErrInvalidCredentials", err)
	}
	if errors.Is(err, ErrAccountLocked) {
		t.Fatal("wrong password must not reveal the locked state")
	}
}

func TestLoginResetsExpiredLockBeforeCountingNewFailure(t *testing.T) {
	repo, cfg := lockedCredential(t, "correct-password", time.Now().Add(-time.Minute), user.StatusLocked)
	service := newLoginLockService(t, cfg, repo)

	_, err := service.Login(context.Background(), "locked-user", "wrong-password", "req-1", requestmeta.SessionAuditContext{})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expired lock with wrong password = %v, want ErrInvalidCredentials", err)
	}
	if repo.credential.FailedLoginCount != 1 {
		t.Fatalf("failure count = %d, want 1 after the expired lock was reset", repo.credential.FailedLoginCount)
	}
	if repo.credential.LockedUntil != nil {
		t.Fatalf("single failure must not lock the account, got %v", repo.credential.LockedUntil)
	}
	if len(repo.statusUpdates) == 0 || repo.statusUpdates[0] != user.StatusActive {
		t.Fatalf("expected account to be reactivated, got %#v", repo.statusUpdates)
	}
}

func TestLoginFailureWithoutLockPolicyStaysUnlocked(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	repo := &loginLockRepo{
		userItem:   &user.User{ID: 7, Username: "plain-user", Status: user.StatusActive},
		credential: &user.Credential{ID: 11, UserID: 7, PasswordHash: string(hash), PasswordEnabled: true},
	}
	service := newLoginLockService(t, config.Config{
		UsernameLoginEnabled: true,
		LoginMaxFailures:     0,
		LoginLockMinutes:     0,
		JWTSecret:            "test-secret",
	}, repo)

	_, err = service.Login(context.Background(), "plain-user", "wrong-password", "req-1", requestmeta.SessionAuditContext{})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("disabled lock policy = %v, want ErrInvalidCredentials", err)
	}
	if repo.markThreshold != 0 {
		t.Fatalf("mark threshold = %d, want 0 when locking is disabled", repo.markThreshold)
	}
	if repo.credential.LockedUntil != nil {
		t.Fatalf("locking disabled must not lock the account, got %#v", repo.credential.LockedUntil)
	}
}
