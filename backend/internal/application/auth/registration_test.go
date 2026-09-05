package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
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

func TestBuildVerificationEmailMessageEscapesDynamicHTMLContent(t *testing.T) {
	template := verificationEmailTemplate{
		Subject:      "验证码",
		Title:        `<script>alert("title")</script>`,
		SecurityNote: `<img src=x onerror=alert("note")>`,
	}
	message := buildVerificationEmailMessage(
		"DEEIX Chat <no-reply@example.com>",
		"user@example.com",
		"123456",
		template,
		`https://deeix.example/logo.svg?a="quoted"`,
	)
	htmlBody := buildVerificationHTML("123456", template, `https://deeix.example/logo.svg?a="quoted"`)

	if strings.Contains(htmlBody, `<script>alert("title")</script>`) || strings.Contains(htmlBody, `<img src=x onerror=alert("note")>`) {
		t.Fatalf("expected dynamic HTML content to be escaped, got:\n%s", htmlBody)
	}
	if !strings.Contains(message, `&lt;script&gt;alert(&#34;title&#34;)&lt;/script&gt;`) {
		t.Fatalf("expected escaped title in HTML body, got:\n%s", message)
	}
	if !strings.Contains(message, `&lt;img src=x onerror=alert(&#34;note&#34;)&gt;`) {
		t.Fatalf("expected escaped security note in HTML body, got:\n%s", message)
	}
	if !strings.Contains(message, `a=&#34;quoted&#34;`) {
		t.Fatalf("expected escaped logo URL attribute, got:\n%s", message)
	}
}

func TestNormalizeRegistrationEmailRejectsHeaderInjection(t *testing.T) {
	for _, raw := range []string{
		"attacker@example.com\r\nBcc: victim@example.com",
		"attacker@example.com\nCc: victim@example.com",
	} {
		if _, err := normalizeRegistrationEmail(raw); err == nil {
			t.Fatalf("expected header injection payload %q to be rejected", raw)
		}
	}
}

func TestSendRegistrationVerificationEmailRejectsInvalidFrom(t *testing.T) {
	service := newTestService(config.Config{
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
	service := newTestService(config.Config{
		Env:                      "dev",
		JWTSecret:                "test-secret",
		EmailLoginEnabled:        true,
		EmailRegistrationEnabled: true,
		EmailVerificationEnabled: true,
	}, repo, nil)

	_, err := service.RequestEmailRegistration(context.Background(), "user@example.com", "", "", "", requestmeta.SessionAuditContext{})
	if err == nil || err.Error() != "verification code was sent recently" {
		t.Fatalf("expected cooldown error, got %v", err)
	}
	if repo.cancelCount != 0 || repo.createCount != 0 {
		t.Fatalf("expected no verification rewrite during cooldown, cancel=%d create=%d", repo.cancelCount, repo.createCount)
	}
}

func TestRequestEmailRegistrationRequiresTurnstileWhenEnabled(t *testing.T) {
	service := newTestService(config.Config{
		EmailLoginEnabled:            true,
		EmailRegistrationEnabled:     true,
		EmailVerificationEnabled:     true,
		TurnstileRegistrationEnabled: true,
		TurnstileSiteKey:             "site-key",
		TurnstileSecretKey:           "secret-key",
	}, nil, nil)

	_, err := service.RequestEmailRegistration(context.Background(), "user@example.com", "", "127.0.0.1", "", requestmeta.SessionAuditContext{})
	if err == nil || err.Error() != "turnstile verification is required" {
		t.Fatalf("expected turnstile required error, got %v", err)
	}
}

func TestVerifyRegistrationTurnstileSkipsWhenSiteKeyEmpty(t *testing.T) {
	service := newTestService(config.Config{}, nil, nil)

	err := service.verifyRegistrationTurnstile(context.Background(), config.Config{
		TurnstileRegistrationEnabled: true,
		TurnstileSecretKey:           "secret-key",
	}, "", "")
	if err != nil {
		t.Fatalf("expected empty site key to skip turnstile, got %v", err)
	}
}

func TestVerifyRegistrationTurnstileRequiresSecretWhenSiteKeyPresent(t *testing.T) {
	service := newTestService(config.Config{}, nil, nil)

	err := service.verifyRegistrationTurnstile(context.Background(), config.Config{
		TurnstileRegistrationEnabled: true,
		TurnstileSiteKey:             "site-key",
	}, "token", "")
	if err == nil || err.Error() != "turnstile is not configured" {
		t.Fatalf("expected missing secret error, got %v", err)
	}
}

func TestVerifyRegistrationTurnstileUsesConfiguredEndpoint(t *testing.T) {
	var gotRemoteIP string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse turnstile form: %v", err)
		}
		if r.Form.Get("secret") != "secret-key" || r.Form.Get("response") != "token" {
			t.Fatalf("unexpected form: %#v", r.Form)
		}
		gotRemoteIP = r.Form.Get("remoteip")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	service := newTestService(config.Config{
		Env:                   "prod",
		SSRFProtectionEnabled: true,
	}, nil, nil)
	err := service.verifyRegistrationTurnstile(context.Background(), config.Config{
		TurnstileRegistrationEnabled: true,
		TurnstileSiteKey:             "site-key",
		TurnstileSecretKey:           "secret-key",
		TurnstileSiteverifyURL:       server.URL,
	}, "token", "203.0.113.7")
	if err != nil {
		t.Fatalf("expected configured endpoint verification to pass, got %v", err)
	}
	if gotRemoteIP != "203.0.113.7" {
		t.Fatalf("expected remote ip to be forwarded, got %q", gotRemoteIP)
	}
}

func TestRegisterWithEmailRequiresTurnstileWhenEmailVerificationDisabled(t *testing.T) {
	service := newTestService(config.Config{
		EmailLoginEnabled:            true,
		EmailRegistrationEnabled:     true,
		EmailVerificationEnabled:     false,
		TurnstileRegistrationEnabled: true,
		TurnstileSiteKey:             "site-key",
		TurnstileSecretKey:           "secret-key",
	}, nil, nil)

	_, err := service.RegisterWithEmail(context.Background(), RegisterWithEmailInput{
		Email:        "user@example.com",
		Password:     "securepass1",
		RemoteIP:     "127.0.0.1",
		AuditContext: requestmeta.SessionAuditContext{},
	})
	if err == nil || err.Error() != "turnstile verification is required" {
		t.Fatalf("expected turnstile required error, got %v", err)
	}
}

func TestRegisterWithEmailDoesNotRequireTurnstileWhenEmailVerificationEnabled(t *testing.T) {
	service := newTestService(config.Config{
		EmailLoginEnabled:            true,
		EmailRegistrationEnabled:     true,
		EmailVerificationEnabled:     true,
		TurnstileRegistrationEnabled: true,
		TurnstileSiteKey:             "site-key",
		TurnstileSecretKey:           "secret-key",
	}, &emailRegistrationRepo{}, nil)

	_, err := service.RegisterWithEmail(context.Background(), RegisterWithEmailInput{
		Email:        "user@example.com",
		Password:     "securepass1",
		RemoteIP:     "127.0.0.1",
		AuditContext: requestmeta.SessionAuditContext{},
	})
	if err == nil || err.Error() != "verification code is invalid or expired" {
		t.Fatalf("expected verification code error instead of turnstile error, got %v", err)
	}
}

func TestResolveSecurityVerificationMethods(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name              string
		emailVerification bool
		user              *domainuser.User
		twoFactor         *domainuser.UserTwoFactor
		want              []SecurityVerificationMethod
	}{
		{
			name:              "enabled two factor wins",
			emailVerification: true,
			user:              &domainuser.User{ID: 1, Email: "user@example.com", EmailVerifiedAt: &now},
			twoFactor:         &domainuser.UserTwoFactor{UserID: 1, TOTPEnabled: true, TOTPSecretEncrypted: "secret"},
			want:              []SecurityVerificationMethod{SecurityVerificationMethodTwoFactor, SecurityVerificationMethodEmail},
		},
		{
			name:              "verified email when no two factor",
			emailVerification: true,
			user:              &domainuser.User{ID: 1, Email: "user@example.com", EmailVerifiedAt: &now},
			want:              []SecurityVerificationMethod{SecurityVerificationMethodEmail},
		},
		{
			name:              "unverified email is not a verification method",
			emailVerification: true,
			user:              &domainuser.User{ID: 1, Email: "user@example.com"},
			want:              []SecurityVerificationMethod{SecurityVerificationMethodNone},
		},
		{
			name:              "email verification disabled",
			emailVerification: false,
			user:              &domainuser.User{ID: 1, Email: "user@example.com", EmailVerifiedAt: &now},
			want:              []SecurityVerificationMethod{SecurityVerificationMethodNone},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := newTestService(config.Config{EmailVerificationEnabled: tc.emailVerification}, &securityVerificationRepo{user: tc.user, twoFactor: tc.twoFactor}, nil)
			got, err := service.resolveSecurityVerificationMethods(context.Background(), tc.user)
			if err != nil {
				t.Fatalf("resolveSecurityVerificationMethods() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("resolveSecurityVerificationMethods() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRequestPasswordChangeVerificationDoesNotUseUnverifiedEmail(t *testing.T) {
	service := newTestService(config.Config{EmailVerificationEnabled: true}, &securityVerificationRepo{
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
	service := newTestService(config.Config{EmailVerificationEnabled: false}, repo, nil)

	updated, err := service.CompleteEmailChange(context.Background(), CompleteEmailChangeInput{
		UserID:       1,
		NewEmail:     "new@example.com",
		AuditContext: requestmeta.SessionAuditContext{},
	})
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
	service := newTestService(config.Config{JWTSecret: "test-secret"}, repo, nil)

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

func (r *emailRegistrationRepo) RecordAuthEvent(ctx context.Context, input repository.AuthEventInput) error {
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

func (r *securityVerificationRepo) RecordAuthEvent(ctx context.Context, input repository.AuthEventInput) error {
	return nil
}
