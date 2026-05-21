package auth

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type twoFactorLookupRepo struct {
	repository.AuthRepository

	item *domainuser.UserTwoFactor
	err  error
}

func (r *twoFactorLookupRepo) GetUserTwoFactorByUserID(ctx context.Context, userID uint) (*domainuser.UserTwoFactor, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.item, nil
}

type twoFactorConfirmRepo struct {
	repository.AuthRepository

	item               *domainuser.UserTwoFactor
	ignoreEnabledWrite bool
}

func (r *twoFactorConfirmRepo) GetUserTwoFactorByUserID(ctx context.Context, userID uint) (*domainuser.UserTwoFactor, error) {
	if r.item == nil {
		return nil, repository.ErrNotFound
	}
	copyItem := *r.item
	return &copyItem, nil
}

func (r *twoFactorConfirmRepo) UpdateUserTwoFactor(ctx context.Context, userID uint, input repository.UpdateUserTwoFactorInput) (*domainuser.UserTwoFactor, error) {
	if r.item == nil {
		return nil, repository.ErrNotFound
	}
	if input.TOTPEnabled != nil && !r.ignoreEnabledWrite {
		r.item.TOTPEnabled = *input.TOTPEnabled
	}
	if input.TOTPSetupExpiresAt != nil {
		r.item.TOTPSetupExpiresAt = *input.TOTPSetupExpiresAt
	}
	if input.ExpectedRecoveryHash != nil && r.item.RecoveryCodesHash != *input.ExpectedRecoveryHash {
		return nil, repository.ErrNotFound
	}
	if input.RecoveryCodesHash != nil {
		r.item.RecoveryCodesHash = *input.RecoveryCodesHash
	}
	if input.EnabledAt != nil {
		r.item.EnabledAt = *input.EnabledAt
	}
	if input.LastVerifiedAt != nil {
		r.item.LastVerifiedAt = *input.LastVerifiedAt
	}
	return r.GetUserTwoFactorByUserID(ctx, userID)
}

func TestConfirmCurrentTwoFactorSetupReturnsNotStartedWhenMissing(t *testing.T) {
	service := NewService(config.Config{}, &twoFactorLookupRepo{err: repository.ErrNotFound}, nil)

	_, err := service.ConfirmCurrentTwoFactorSetup(context.Background(), 42, "123456")
	if !errors.Is(err, ErrTwoFactorSetupNotStarted) {
		t.Fatalf("expected setup not started, got %v", err)
	}
}

func TestConfirmCurrentTwoFactorSetupIsIdempotentWhenAlreadyEnabled(t *testing.T) {
	now := time.Now()
	service := NewService(config.Config{}, &twoFactorLookupRepo{item: &domainuser.UserTwoFactor{
		UserID:      42,
		TOTPEnabled: true,
		EnabledAt:   &now,
	}}, nil)

	result, err := service.ConfirmCurrentTwoFactorSetup(context.Background(), 42, "000000")
	if err != nil {
		t.Fatalf("expected already-enabled setup confirm to be idempotent, got %v", err)
	}
	if result == nil || !result.Status.TOTPEnabled || result.Status.EnabledAt == nil {
		t.Fatalf("expected enabled status, got %#v", result)
	}
	if len(result.RecoveryCodes) != 0 {
		t.Fatalf("expected no recovery codes on idempotent confirm, got %d", len(result.RecoveryCodes))
	}
}

func TestConfirmCurrentTwoFactorSetupFailsWhenEnabledStateDoesNotPersist(t *testing.T) {
	cfg := config.Config{
		JWTSecret:         "test-jwt-secret",
		DataEncryptionKey: "test-data-encryption-key",
	}
	secret, err := generateTOTPSecret()
	if err != nil {
		t.Fatalf("generate secret: %v", err)
	}
	encrypted, err := secretbox.EncryptString(cfg.DataEncryptionKey, secret)
	if err != nil {
		t.Fatalf("encrypt secret: %v", err)
	}
	now := time.Now()
	expiresAt := now.Add(time.Minute)
	code, err := generateTOTPCode(secret, now.Unix()/totpStepSeconds)
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	service := NewService(cfg, &twoFactorConfirmRepo{
		item: &domainuser.UserTwoFactor{
			UserID:              42,
			TOTPEnabled:         false,
			TOTPSecretEncrypted: encrypted,
			TOTPSetupExpiresAt:  &expiresAt,
		},
		ignoreEnabledWrite: true,
	}, nil)

	_, err = service.ConfirmCurrentTwoFactorSetup(context.Background(), 42, code)
	if !errors.Is(err, ErrTwoFactorSetupNotPersisted) {
		t.Fatalf("expected setup persistence error, got %v", err)
	}
}

func TestVerifyTOTPToleratesConfiguredClockSkew(t *testing.T) {
	secret, err := generateTOTPSecret()
	if err != nil {
		t.Fatalf("generate secret: %v", err)
	}
	now := time.Unix(1_700_000_000, 0)
	code, err := generateTOTPCode(secret, now.Unix()/totpStepSeconds+totpValidationWindow)
	if err != nil {
		t.Fatalf("generate totp code: %v", err)
	}
	if !verifyTOTP(secret, code, now) {
		t.Fatalf("expected TOTP code within validation window to pass")
	}
}

func TestVerifyTOTPRejectsOutsideClockSkewWindow(t *testing.T) {
	secret, err := generateTOTPSecret()
	if err != nil {
		t.Fatalf("generate secret: %v", err)
	}
	now := time.Unix(1_700_000_000, 0)
	code, err := generateTOTPCode(secret, now.Unix()/totpStepSeconds+totpValidationWindow+1)
	if err != nil {
		t.Fatalf("generate totp code: %v", err)
	}
	if verifyTOTP(secret, code, now) {
		t.Fatalf("expected TOTP code outside validation window to fail")
	}
}

func TestNormalizeTwoFactorCodeKeepsRecoveryCodeCharacters(t *testing.T) {
	got := normalizeTwoFactorCode(" abcd-efgh ijkl ")
	if got != "ABCDEFGHIJKL" {
		t.Fatalf("expected normalized recovery code, got %q", got)
	}
}

func TestConsumeRecoveryCodeUsesCompareAndSwap(t *testing.T) {
	cfg := config.Config{JWTSecret: "test-jwt-secret"}
	codes, recoveryHash, err := generateRecoveryCodes(cfg.JWTSecret)
	if err != nil {
		t.Fatalf("generate recovery codes: %v", err)
	}
	repo := &twoFactorConfirmRepo{item: &domainuser.UserTwoFactor{
		UserID:            42,
		TOTPEnabled:       true,
		RecoveryCodesHash: recoveryHash,
	}}
	service := NewService(cfg, repo, nil)

	if !service.consumeRecoveryCode(context.Background(), 42, recoveryHash, codes[0]) {
		t.Fatalf("expected first recovery code consumption to succeed")
	}
	if service.consumeRecoveryCode(context.Background(), 42, recoveryHash, codes[0]) {
		t.Fatalf("expected stale recovery hash consumption to fail")
	}
}

func TestGenerateTOTPCodeMatchesRFC6238SHA1ModuloSixDigits(t *testing.T) {
	code, err := generateTOTPCode("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", 59/totpStepSeconds)
	if err != nil {
		t.Fatalf("generate TOTP code: %v", err)
	}
	if code != "287082" {
		t.Fatalf("expected RFC 6238 SHA1 test vector modulo six digits, got %q", code)
	}
}

func TestBuildOTPAuthURLPreservesIssuerAccountSeparator(t *testing.T) {
	got := buildOTPAuthURL("DEEIX Chat", "user@example.com", "ABCDEF")
	if strings.Contains(got, "DEEIX%20Chat%3A") {
		t.Fatalf("issuer/account separator should stay as ':' in otpauth label: %s", got)
	}
	if !strings.HasPrefix(got, "otpauth://totp/DEEIX%20Chat:") {
		t.Fatalf("expected otpauth label to start with issuer and raw separator, got %s", got)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse otpauth URL: %v", err)
	}
	query := parsed.Query()
	if query.Get("secret") != "ABCDEF" || query.Get("issuer") != "DEEIX Chat" || query.Get("digits") != "6" || query.Get("period") != "30" {
		t.Fatalf("unexpected otpauth query: %s", parsed.RawQuery)
	}
}
