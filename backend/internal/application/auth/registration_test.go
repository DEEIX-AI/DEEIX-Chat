package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
)

func TestBuildVerificationEmailMessageEncodesChineseSubject(t *testing.T) {
	message := buildVerificationEmailMessage("DEEIX Chat <no-reply@example.com>", "user@example.com", "123456", verificationEmailTemplate{
		Subject:      "DEEIX Chat 验证码",
		Title:        "完成邮箱注册",
		SecurityNote: "如果不是您本人操作，请忽略这封邮件。",
	}, "https://deeix.example/logo.svg")

	if !strings.Contains(message, "Subject: =?utf-8?") {
		t.Fatalf("expected encoded utf-8 subject, got:\n%s", message)
	}
	if strings.Contains(message, "Subject: DEEIX Chat 验证码") {
		t.Fatalf("expected subject to be MIME encoded, got:\n%s", message)
	}
	if !strings.Contains(message, "Content-Type: multipart/alternative; boundary=") {
		t.Fatalf("expected multipart alternative content type, got:\n%s", message)
	}
	if !strings.Contains(message, "Content-Type: text/plain; charset=UTF-8") {
		t.Fatalf("expected text/plain part, got:\n%s", message)
	}
	if !strings.Contains(message, "Content-Type: text/html; charset=UTF-8") {
		t.Fatalf("expected text/html part, got:\n%s", message)
	}
	if !strings.Contains(message, "验证码：123456") {
		t.Fatalf("expected plain verification code, got:\n%s", message)
	}
	if !strings.Contains(message, "<!doctype html>") || !strings.Contains(message, ">123456<") {
		t.Fatalf("expected html verification body, got:\n%s", message)
	}
	if !strings.Contains(message, `src="https://deeix.example/logo.svg"`) {
		t.Fatalf("expected html logo, got:\n%s", message)
	}
}

func TestSendRegistrationVerificationEmailRejectsInvalidFrom(t *testing.T) {
	service := NewService(config.Config{
		Env:          "production",
		SMTPHost:     "smtp.example.com",
		SMTPPort:     587,
		SMTPUsername: "smtp-user",
		SMTPPassword: "smtp-password",
		SMTPFrom:     "不是合法发件人",
	}, nil, nil)

	err := service.sendRegistrationVerificationEmail("user@example.com", "123456")
	if err == nil || err.Error() != "smtp from is invalid" {
		t.Fatalf("expected invalid from error, got %v", err)
	}
}

func TestValidateEmailRegistrationPolicy(t *testing.T) {
	cfg := config.Config{
		EmailRegistrationDomains: "example.com, @deeix-chat.ai\ncorp.cn",
		EmailRegistrationNoAlias: true,
	}

	if err := validateEmailRegistrationPolicy(cfg, "user@deeix-chat.ai"); err != nil {
		t.Fatalf("expected deeix-chat.ai to pass, got %v", err)
	}
	if err := validateEmailRegistrationPolicy(cfg, "user+alias@deeix-chat.ai"); err == nil || err.Error() != "email aliases are not allowed" {
		t.Fatalf("expected alias rejection, got %v", err)
	}
	if err := validateEmailRegistrationPolicy(cfg, "user@blocked.com"); err == nil || err.Error() != "email domain is not allowed" {
		t.Fatalf("expected domain rejection, got %v", err)
	}
}

func TestNormalizeEditableUsername(t *testing.T) {
	got, err := normalizeEditableUsername(" Alice_01 ")
	if err != nil {
		t.Fatalf("expected valid username, got %v", err)
	}
	if got != "alice_01" {
		t.Fatalf("expected lowercase username, got %q", got)
	}

	for _, raw := range []string{"ab", "admin", "user@example.com", "-alice", "alice.", "alice_"} {
		if _, err := normalizeEditableUsername(raw); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
}

func TestGeneratedUsernameWithSuffix(t *testing.T) {
	if got := generatedUsernameWithSuffix("alice", 0); got != "alice" {
		t.Fatalf("expected base username, got %q", got)
	}
	if got := generatedUsernameWithSuffix("alice", 1); got != "alice-2" {
		t.Fatalf("expected suffixed username, got %q", got)
	}

	long := strings.Repeat("a", 70)
	got := generatedUsernameWithSuffix(long, 9)
	if len(got) > 16 {
		t.Fatalf("expected username length <= 16, got %d", len(got))
	}
	if !strings.HasSuffix(got, "-10") {
		t.Fatalf("expected numeric suffix to be preserved, got %q", got)
	}
}

func TestCanBootstrapEmailOnlyAllowsMissingOrUnverifiedProviderEmail(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		user *domainuser.User
		want bool
	}{
		{
			name: "missing email",
			user: &domainuser.User{},
			want: true,
		},
		{
			name: "unverified provider email",
			user: &domainuser.User{Email: "provider@example.com", EmailSource: domainuser.EmailSourceProviderUnverified},
			want: true,
		},
		{
			name: "local registration email",
			user: &domainuser.User{Email: "user@example.com", EmailSource: domainuser.EmailSourceLocalRegister},
			want: false,
		},
		{
			name: "already verified",
			user: &domainuser.User{Email: "provider@example.com", EmailSource: domainuser.EmailSourceProviderUnverified, EmailVerifiedAt: &now},
			want: false,
		},
		{
			name: "bootstrap already used",
			user: &domainuser.User{Email: "provider@example.com", EmailSource: domainuser.EmailSourceProviderUnverified, EmailBootstrapUsedAt: &now},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canBootstrapEmail(tc.user); got != tc.want {
				t.Fatalf("canBootstrapEmail() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRequestEmailRegistrationUsesSendCooldown(t *testing.T) {
	sentAt := time.Now().Add(-10 * time.Second)
	repo := &emailRegistrationRepo{
		pending: &domainuser.ContactVerification{
			ID:     1,
			SentAt: &sentAt,
		},
	}
	service := NewService(config.Config{
		Env:                      "dev",
		JWTSecret:                "test-secret",
		EmailLoginEnabled:        true,
		EmailRegistrationEnabled: true,
		EmailVerificationEnabled: true,
	}, repo, nil)

	_, err := service.RequestEmailRegistration(context.Background(), "user@example.com", "", requestmeta.SessionAuditContext{})
	if err == nil || err.Error() != "verification code was sent recently" {
		t.Fatalf("expected cooldown error, got %v", err)
	}
	if repo.cancelCount != 0 || repo.createCount != 0 {
		t.Fatalf("expected no verification rewrite during cooldown, cancel=%d create=%d", repo.cancelCount, repo.createCount)
	}
}

func TestResolveSecurityVerificationMethod(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name              string
		emailVerification bool
		user              *domainuser.User
		twoFactor         *domainuser.UserTwoFactor
		want              SecurityVerificationMethod
	}{
		{
			name:              "enabled two factor wins",
			emailVerification: true,
			user:              &domainuser.User{ID: 1, Email: "user@example.com", EmailVerifiedAt: &now},
			twoFactor:         &domainuser.UserTwoFactor{UserID: 1, TOTPEnabled: true, TOTPSecretEncrypted: "secret"},
			want:              SecurityVerificationMethodTwoFactor,
		},
		{
			name:              "verified email when no two factor",
			emailVerification: true,
			user:              &domainuser.User{ID: 1, Email: "user@example.com", EmailVerifiedAt: &now},
			want:              SecurityVerificationMethodEmail,
		},
		{
			name:              "unverified email is not a verification method",
			emailVerification: true,
			user:              &domainuser.User{ID: 1, Email: "user@example.com"},
			want:              SecurityVerificationMethodNone,
		},
		{
			name:              "email verification disabled",
			emailVerification: false,
			user:              &domainuser.User{ID: 1, Email: "user@example.com", EmailVerifiedAt: &now},
			want:              SecurityVerificationMethodNone,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := NewService(config.Config{EmailVerificationEnabled: tc.emailVerification}, &securityVerificationRepo{user: tc.user, twoFactor: tc.twoFactor}, nil)
			got, err := service.resolveSecurityVerificationMethod(context.Background(), tc.user)
			if err != nil {
				t.Fatalf("resolveSecurityVerificationMethod() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("resolveSecurityVerificationMethod() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRequestPasswordChangeVerificationDoesNotUseUnverifiedEmail(t *testing.T) {
	service := NewService(config.Config{EmailVerificationEnabled: true}, &securityVerificationRepo{
		user: &domainuser.User{ID: 1, Email: "user@example.com"},
	}, nil)

	result, err := service.RequestPasswordChangeVerification(context.Background(), 1, "", "", requestmeta.SessionAuditContext{})
	if err != nil {
		t.Fatalf("RequestPasswordChangeVerification() error = %v", err)
	}
	if result.Sent || result.Method != SecurityVerificationMethodNone {
		t.Fatalf("expected no verification email for unverified address, got sent=%v method=%q", result.Sent, result.Method)
	}
}

func TestCompleteEmailChangeDoesNotVerifyEmailWhenEmailVerificationDisabled(t *testing.T) {
	repo := &securityVerificationRepo{
		user: &domainuser.User{ID: 1, Email: "old@example.com", EmailSource: domainuser.EmailSourceUserSet},
	}
	service := NewService(config.Config{EmailVerificationEnabled: false}, repo, nil)

	updated, err := service.CompleteEmailChange(context.Background(), 1, "new@example.com", "", "", "", "", requestmeta.SessionAuditContext{})
	if err != nil {
		t.Fatalf("CompleteEmailChange() error = %v", err)
	}
	if updated.Email != "new@example.com" {
		t.Fatalf("expected updated email, got %q", updated.Email)
	}
	if updated.EmailVerifiedAt != nil {
		t.Fatalf("expected email_verified_at to stay nil when verification is disabled")
	}
}

func TestVerifyEmailCodeUsesUserScopedPendingVerification(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(time.Minute)
	userToken := "user-token"
	otherToken := "other-token"
	code := "123456"
	repo := &securityVerificationRepo{
		pendingVerifications: []domainuser.ContactVerification{
			{
				ID:        1,
				UserID:    1,
				Channel:   domainuser.ContactVerificationChannelEmail,
				Purpose:   domainuser.ContactVerificationPurposeEmailChangeNew,
				Target:    "new@example.com",
				Token:     userToken,
				CodeHash:  hashRegistrationCode("test-secret", userToken, code),
				Status:    domainuser.ContactVerificationStatusPending,
				ExpiresAt: &expiresAt,
			},
			{
				ID:        2,
				UserID:    2,
				Channel:   domainuser.ContactVerificationChannelEmail,
				Purpose:   domainuser.ContactVerificationPurposeEmailChangeNew,
				Target:    "new@example.com",
				Token:     otherToken,
				CodeHash:  hashRegistrationCode("test-secret", otherToken, "654321"),
				Status:    domainuser.ContactVerificationStatusPending,
				ExpiresAt: &expiresAt,
			},
		},
	}
	service := NewService(config.Config{JWTSecret: "test-secret"}, repo, nil)

	err := service.verifyEmailCode(context.Background(), 1, domainuser.ContactVerificationPurposeEmailChangeNew, "new@example.com", code, now)
	if err != nil {
		t.Fatalf("verifyEmailCode() error = %v", err)
	}
	if repo.pendingVerifications[0].Status != domainuser.ContactVerificationStatusVerified {
		t.Fatalf("expected verification for user 1 to be marked verified")
	}
	if repo.pendingVerifications[1].Status != domainuser.ContactVerificationStatusPending {
		t.Fatalf("expected verification for user 2 to remain pending")
	}
}

type emailRegistrationRepo struct {
	repository.AuthRepository

	pending     *domainuser.ContactVerification
	cancelCount int
	createCount int
}

func (r *emailRegistrationRepo) GetByEmail(ctx context.Context, email string) (*domainuser.User, error) {
	return nil, repository.ErrNotFound
}

func (r *emailRegistrationRepo) RecordAuthEvent(ctx context.Context, userID uint, requestID string, eventType string, result string, reason string, clientIP string, userAgent string, detailJSON string) error {
	return nil
}

func (r *emailRegistrationRepo) CancelPendingContactVerifications(ctx context.Context, channel string, purpose string, target string) error {
	r.cancelCount++
	return nil
}

func (r *emailRegistrationRepo) CreateContactVerification(ctx context.Context, item *domainuser.ContactVerification) (*domainuser.ContactVerification, error) {
	r.createCount++
	item.ID = uint(r.createCount)
	return item, nil
}

func (r *emailRegistrationRepo) GetPendingContactVerification(ctx context.Context, channel string, purpose string, target string, now time.Time) (*domainuser.ContactVerification, error) {
	if r.pending == nil {
		return nil, repository.ErrNotFound
	}
	if r.pending.ExpiresAt != nil && r.pending.ExpiresAt.Before(now) {
		return nil, repository.ErrNotFound
	}
	return r.pending, nil
}

type securityVerificationRepo struct {
	repository.AuthRepository

	user                 *domainuser.User
	twoFactor            *domainuser.UserTwoFactor
	pendingVerifications []domainuser.ContactVerification
}

func (r *securityVerificationRepo) GetByID(ctx context.Context, userID uint) (*domainuser.User, error) {
	if r.user == nil || r.user.ID != userID {
		return nil, repository.ErrNotFound
	}
	copyItem := *r.user
	return &copyItem, nil
}

func (r *securityVerificationRepo) GetByEmail(ctx context.Context, email string) (*domainuser.User, error) {
	return nil, repository.ErrNotFound
}

func (r *securityVerificationRepo) GetUserTwoFactorByUserID(ctx context.Context, userID uint) (*domainuser.UserTwoFactor, error) {
	if r.twoFactor == nil || r.twoFactor.UserID != userID {
		return nil, repository.ErrNotFound
	}
	copyItem := *r.twoFactor
	return &copyItem, nil
}

func (r *securityVerificationRepo) UpdateProfile(ctx context.Context, userID uint, input repository.UpdateUserFieldsInput) (*domainuser.User, error) {
	if r.user == nil || r.user.ID != userID {
		return nil, repository.ErrNotFound
	}
	if input.Email != nil {
		r.user.Email = *input.Email
	}
	if input.EmailSource != nil {
		r.user.EmailSource = *input.EmailSource
	}
	if input.EmailVerifiedAt != nil {
		r.user.EmailVerifiedAt = *input.EmailVerifiedAt
	}
	copyItem := *r.user
	return &copyItem, nil
}

func (r *securityVerificationRepo) GetPendingContactVerificationForUser(ctx context.Context, userID uint, channel string, purpose string, target string, now time.Time) (*domainuser.ContactVerification, error) {
	for i := len(r.pendingVerifications) - 1; i >= 0; i-- {
		item := r.pendingVerifications[i]
		if item.UserID != userID || item.Channel != channel || item.Purpose != purpose || item.Target != target || item.Status != domainuser.ContactVerificationStatusPending {
			continue
		}
		if item.ExpiresAt != nil && !item.ExpiresAt.After(now) {
			continue
		}
		copyItem := item
		return &copyItem, nil
	}
	return nil, repository.ErrNotFound
}

func (r *securityVerificationRepo) IncrementContactVerificationAttempt(ctx context.Context, verificationID uint) error {
	for i := range r.pendingVerifications {
		if r.pendingVerifications[i].ID == verificationID {
			r.pendingVerifications[i].AttemptCount++
			return nil
		}
	}
	return repository.ErrNotFound
}

func (r *securityVerificationRepo) MarkContactVerificationVerified(ctx context.Context, verificationID uint, now time.Time) error {
	for i := range r.pendingVerifications {
		if r.pendingVerifications[i].ID == verificationID && r.pendingVerifications[i].Status == domainuser.ContactVerificationStatusPending {
			r.pendingVerifications[i].Status = domainuser.ContactVerificationStatusVerified
			r.pendingVerifications[i].VerifiedAt = &now
			r.pendingVerifications[i].ConsumedAt = &now
			return nil
		}
	}
	return repository.ErrNotFound
}

func (r *securityVerificationRepo) RecordAuthEvent(ctx context.Context, userID uint, requestID string, eventType string, result string, reason string, clientIP string, userAgent string, detailJSON string) error {
	return nil
}
