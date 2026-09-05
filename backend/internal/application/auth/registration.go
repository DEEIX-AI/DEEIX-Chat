package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"math/big"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	userapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/user"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/conv"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const (
	emailRegistrationCodeTTL      = 10 * time.Minute
	emailRegistrationSendCooldown = 60 * time.Second
	emailRegistrationMaxAttempts  = 5
	emailRegistrationCodeDigits   = 6
	emailRegistrationCodeMaxValue = 1000000
	emailSMTPTimeout              = 10 * time.Second
)

type verificationEmailTemplate struct {
	Subject      string
	Title        string
	SecurityNote string
}

// EmailRegistrationStartResult reports the result of sending a registration code.
type EmailRegistrationStartResult struct {
	Sent      bool
	ExpiresAt time.Time
}

// PasswordResetStartResult 表示密码重置验证码请求结果。
type PasswordResetStartResult struct {
	Sent      bool
	ExpiresAt time.Time
}

// PasswordChangeVerificationStartResult reports the verification methods available for a password change.
type PasswordChangeVerificationStartResult struct {
	Sent             bool
	ExpiresAt        time.Time
	Method           SecurityVerificationMethod
	AvailableMethods []SecurityVerificationMethod
}

// EmailChangeVerificationStartResult reports the verification methods available for an email change.
type EmailChangeVerificationStartResult struct {
	Sent             bool
	ExpiresAt        time.Time
	Method           SecurityVerificationMethod
	AvailableMethods []SecurityVerificationMethod
}

// RegisterWithEmailInput 描述完成邮箱注册所需的凭据、验证与审计信息。
type RegisterWithEmailInput struct {
	Email          string
	Password       string
	Code           string
	TurnstileToken string
	RemoteIP       string
	RequestID      string
	AuditContext   requestmeta.SessionAuditContext
}

// ChangePasswordInput 描述修改当前用户密码所需的验证与审计信息。
type ChangePasswordInput struct {
	UserID             uint
	CurrentPassword    string
	NewPassword        string
	VerificationMethod string
	Code               string
	RequestID          string
	AuditContext       requestmeta.SessionAuditContext
}

// CompleteEmailChangeInput 描述完成邮箱变更所需的邮箱和验证信息。
type CompleteEmailChangeInput struct {
	UserID                    uint
	NewEmail                  string
	CurrentVerificationMethod string
	CurrentCode               string
	NewCode                   string
	RequestID                 string
	AuditContext              requestmeta.SessionAuditContext
}

type passwordResetAuditInput struct {
	UserID       uint
	RequestID    string
	Result       string
	Reason       string
	Email        string
	AuditContext requestmeta.SessionAuditContext
	Detail       map[string]any
}

type requestEmailVerificationCodeInput struct {
	UserID       uint
	Purpose      string
	Target       string
	EventType    string
	RequestID    string
	AuditContext requestmeta.SessionAuditContext
}

// RequestEmailRegistration sends a verification code for a new email account.
func (s *Service) RequestEmailRegistration(ctx context.Context, email string, turnstileToken string, remoteIP string, requestID string, auditCtx requestmeta.SessionAuditContext) (*EmailRegistrationStartResult, error) {
	cfg := s.cfg.Snapshot()
	if !cfg.EmailLoginEnabled || !cfg.EmailRegistrationEnabled {
		return nil, ErrEmailRegistrationDisabled
	}
	normalizedEmail, err := normalizeRegistrationEmail(email)
	if err != nil {
		return nil, err
	}
	if err = validateEmailRegistrationPolicy(cfg, normalizedEmail); err != nil {
		return nil, err
	}
	if !cfg.EmailVerificationEnabled {
		return nil, ErrEmailVerificationDisabled
	}
	if err = s.verifyRegistrationTurnstile(ctx, cfg, turnstileToken, remoteIP); err != nil {
		return nil, err
	}
	if _, err = s.repo.GetByEmail(ctx, normalizedEmail); err == nil {
		return nil, ErrEmailAlreadyExists
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	now := time.Now()
	existingVerification, err := s.repo.GetPendingContactVerification(ctx, domainuser.ContactVerificationChannelEmail, domainuser.ContactVerificationPurposeRegister, normalizedEmail, now)
	if err == nil && existingVerification.SentAt != nil && now.Sub(*existingVerification.SentAt) < emailRegistrationSendCooldown {
		return nil, ErrVerificationCodeRecent
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	expiresAt := now.Add(emailRegistrationCodeTTL)
	code, err := generateNumericCode(emailRegistrationCodeDigits)
	if err != nil {
		return nil, err
	}
	token := conv.NormalizePublicID(uuid.NewString())
	codeHash := hashRegistrationCode(cfg.JWTSecret, token, code)

	if err = s.repo.CancelPendingContactVerifications(ctx, domainuser.ContactVerificationChannelEmail, domainuser.ContactVerificationPurposeRegister, normalizedEmail); err != nil {
		return nil, err
	}
	created, err := s.repo.CreateContactVerification(ctx, &domainuser.ContactVerification{
		Channel:   domainuser.ContactVerificationChannelEmail,
		Purpose:   domainuser.ContactVerificationPurposeRegister,
		Target:    normalizedEmail,
		Token:     token,
		CodeHash:  codeHash,
		Status:    domainuser.ContactVerificationStatusPending,
		SentAt:    &now,
		ExpiresAt: &expiresAt,
	})
	if err != nil {
		return nil, err
	}

	if err := s.sendRegistrationVerificationEmail(normalizedEmail, code); err != nil {
		_ = s.repo.CancelPendingContactVerifications(ctx, domainuser.ContactVerificationChannelEmail, domainuser.ContactVerificationPurposeRegister, normalizedEmail)
		return nil, err
	}

	normalizedAuditCtx := s.resolveSessionAuditContext(ctx, auditCtx)
	s.RecordAuthEvent(
		ctx,
		repository.AuthEventInput{
			RequestID: requestID,
			EventType: "email_registration_code",
			Result:    "success",
			ClientIP:  normalizedAuditCtx.ClientIP,
			UserAgent: normalizedAuditCtx.UserAgent,
			DetailJSON: marshalAuthEventDetail(map[string]any{
				"email":           normalizedEmail,
				"verification_id": created.ID,
				"expires_at":      expiresAt,
			}),
		},
	)

	return &EmailRegistrationStartResult{
		Sent:      true,
		ExpiresAt: expiresAt,
	}, nil
}

// RegisterWithEmail creates an account after validating the registration code.
func (s *Service) RegisterWithEmail(ctx context.Context, input RegisterWithEmailInput) (*LoginResult, error) {
	cfg := s.cfg.Snapshot()
	if !cfg.EmailLoginEnabled || !cfg.EmailRegistrationEnabled {
		return nil, ErrEmailRegistrationDisabled
	}
	normalizedEmail, err := normalizeRegistrationEmail(input.Email)
	if err != nil {
		return nil, err
	}
	if err = validateEmailRegistrationPolicy(cfg, normalizedEmail); err != nil {
		return nil, err
	}
	normalizedPassword, err := userapp.NormalizePassword(input.Password)
	if err != nil {
		return nil, err
	}
	// With email verification enabled, Turnstile is checked before issuing the registration code.
	// Without that step, completion is the first registration write path and must verify it here.
	if !cfg.EmailVerificationEnabled {
		if err = s.verifyRegistrationTurnstile(ctx, cfg, input.TurnstileToken, input.RemoteIP); err != nil {
			return nil, err
		}
	}
	if _, err = s.repo.GetByEmail(ctx, normalizedEmail); err == nil {
		return nil, ErrEmailAlreadyExists
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	now := time.Now()
	var verification *domainuser.ContactVerification
	if cfg.EmailVerificationEnabled {
		verification, err = s.repo.GetPendingContactVerification(ctx, domainuser.ContactVerificationChannelEmail, domainuser.ContactVerificationPurposeRegister, normalizedEmail, now)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, ErrSecurityVerificationCodeInvalid
			}
			return nil, err
		}
		if verification.AttemptCount >= emailRegistrationMaxAttempts {
			return nil, ErrVerificationCodeAttempts
		}
		if !verifyRegistrationCode(cfg.JWTSecret, verification.Token, strings.TrimSpace(input.Code), verification.CodeHash) {
			_ = s.repo.IncrementContactVerificationAttempt(ctx, verification.ID)
			return nil, ErrSecurityVerificationCodeInvalid
		}
	}

	var verifiedAt *time.Time
	if cfg.EmailVerificationEnabled {
		verifiedAt = &now
	}
	username := registrationUsername(normalizedEmail)
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(normalizedPassword), passwordHashCost)
	if err != nil {
		return nil, err
	}
	userItem := &domainuser.User{
		PublicID:        conv.NormalizePublicID(uuid.NewString()),
		Username:        username,
		DisplayName:     userapp.NormalizeGeneratedDisplayName(registrationDisplayName(normalizedEmail)),
		Email:           normalizedEmail,
		EmailSource:     domainuser.EmailSourceLocalRegister,
		Role:            domainuser.RoleUser,
		Status:          domainuser.StatusActive,
		Timezone:        "Etc/UTC",
		Locale:          "en-US",
		EmailVerifiedAt: verifiedAt,
	}
	if err = s.createWithCredentialUsingAvailableUsername(ctx, repository.CreateWithCredentialInput{
		User: userItem,
		Credential: domainuser.Credential{
			PasswordHash:      string(passwordHash),
			PasswordAlgo:      "bcrypt",
			PasswordEnabled:   true,
			PasswordUpdatedAt: &now,
			PasswordSetAt:     &now,
			PasswordOrigin:    domainuser.PasswordOriginLocalRegister,
		},
	}); err != nil {
		return nil, err
	}
	if verification != nil {
		if err = s.repo.MarkContactVerificationVerified(ctx, verification.ID, now); err != nil {
			return nil, err
		}
	}

	normalizedAuditCtx := s.resolveSessionAuditContext(ctx, input.AuditContext)
	result, err := s.issueLoginResult(ctx, userItem, normalizedAuditCtx, now)
	if err != nil {
		return nil, err
	}
	s.RecordAuthEvent(
		ctx,
		repository.AuthEventInput{
			UserID:    result.User.ID,
			RequestID: input.RequestID,
			EventType: "email_register",
			Result:    "success",
			ClientIP:  normalizedAuditCtx.ClientIP,
			UserAgent: normalizedAuditCtx.UserAgent,
			DetailJSON: marshalAuthEventDetail(map[string]any{
				"email":      normalizedEmail,
				"session_id": result.SessionID,
			}),
		},
	)
	return result, nil
}

// RequestPasswordChangeVerification sends the selected verification code for a password change.
func (s *Service) RequestPasswordChangeVerification(ctx context.Context, userID uint, requestedMethod string, requestID string, auditCtx requestmeta.SessionAuditContext) (*PasswordChangeVerificationStartResult, error) {
	item, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	methods, err := s.resolveSecurityVerificationMethods(ctx, item)
	if err != nil {
		return nil, err
	}
	method := methods[0]
	if normalizedMethod := normalizeSecurityVerificationMethod(requestedMethod); normalizedMethod != "" {
		method = normalizedMethod
	}
	if !containsSecurityVerificationMethod(methods, method) {
		return nil, ErrSecurityVerificationMethodUnavailable
	}
	if method != SecurityVerificationMethodEmail {
		return &PasswordChangeVerificationStartResult{Sent: false, Method: method, AvailableMethods: methods}, nil
	}
	normalizedEmail, err := normalizeRegistrationEmail(item.Email)
	if err != nil {
		return nil, ErrSecurityVerificationEmailInvalid
	}

	cfg := s.cfg.Snapshot()
	now := time.Now()
	existingVerification, err := s.repo.GetPendingContactVerificationForUser(ctx, userID, domainuser.ContactVerificationChannelEmail, domainuser.ContactVerificationPurposePasswordChange, normalizedEmail, now)
	if err == nil && existingVerification.SentAt != nil && now.Sub(*existingVerification.SentAt) < emailRegistrationSendCooldown {
		return nil, ErrVerificationCodeRecent
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	expiresAt := now.Add(emailRegistrationCodeTTL)
	code, err := generateNumericCode(emailRegistrationCodeDigits)
	if err != nil {
		return nil, err
	}
	token := conv.NormalizePublicID(uuid.NewString())
	codeHash := hashRegistrationCode(cfg.JWTSecret, token, code)

	if err = s.repo.CancelPendingContactVerificationsForUser(ctx, userID, domainuser.ContactVerificationChannelEmail, domainuser.ContactVerificationPurposePasswordChange, normalizedEmail); err != nil {
		return nil, err
	}
	created, err := s.repo.CreateContactVerification(ctx, &domainuser.ContactVerification{
		UserID:    userID,
		Channel:   domainuser.ContactVerificationChannelEmail,
		Purpose:   domainuser.ContactVerificationPurposePasswordChange,
		Target:    normalizedEmail,
		Token:     token,
		CodeHash:  codeHash,
		Status:    domainuser.ContactVerificationStatusPending,
		SentAt:    &now,
		ExpiresAt: &expiresAt,
	})
	if err != nil {
		return nil, err
	}

	if err := s.sendPasswordChangeVerificationEmail(normalizedEmail, code); err != nil {
		_ = s.repo.CancelPendingContactVerificationsForUser(ctx, userID, domainuser.ContactVerificationChannelEmail, domainuser.ContactVerificationPurposePasswordChange, normalizedEmail)
		return nil, err
	}

	normalizedAuditCtx := s.resolveSessionAuditContext(ctx, auditCtx)
	s.RecordAuthEvent(
		ctx,
		repository.AuthEventInput{
			UserID:    userID,
			RequestID: requestID,
			EventType: "password_change_code",
			Result:    "success",
			ClientIP:  normalizedAuditCtx.ClientIP,
			UserAgent: normalizedAuditCtx.UserAgent,
			DetailJSON: marshalAuthEventDetail(map[string]any{
				"email":           normalizedEmail,
				"verification_id": created.ID,
				"expires_at":      expiresAt,
			}),
		},
	)

	return &PasswordChangeVerificationStartResult{
		Sent:             true,
		ExpiresAt:        expiresAt,
		Method:           method,
		AvailableMethods: methods,
	}, nil
}

// ChangePassword validates the current security requirements and changes the password.
func (s *Service) ChangePassword(ctx context.Context, input ChangePasswordInput) error {
	normalizedPassword, err := userapp.NormalizePassword(input.NewPassword)
	if err != nil {
		return err
	}
	item, err := s.repo.GetByID(ctx, input.UserID)
	if err != nil {
		return err
	}
	credential, err := s.repo.GetCredentialByUserID(ctx, input.UserID)
	if err != nil {
		return err
	}
	initialReset := credential.MustResetPassword || isBootstrapSuperAdminAdminCreatedPassword(*item, credential)
	if initialReset && isBootstrapSuperAdminAdminCreatedPassword(*item, credential) && passwordMatchesCredential(normalizedPassword, credential) {
		return ErrPasswordReuse
	}
	if credential.PasswordEnabled && !initialReset {
		if err = bcrypt.CompareHashAndPassword([]byte(credential.PasswordHash), []byte(input.CurrentPassword)); err != nil {
			return ErrInvalidCredentials
		}
	}

	now := time.Now()
	normalizedEmail := ""
	if !initialReset {
		methods, methodErr := s.resolveSecurityVerificationMethods(ctx, item)
		if methodErr != nil {
			return methodErr
		}
		method := methods[0]
		if normalizedMethod := normalizeSecurityVerificationMethod(input.VerificationMethod); normalizedMethod != "" {
			method = normalizedMethod
		}
		if !containsSecurityVerificationMethod(methods, method) {
			return ErrSecurityVerificationMethodUnavailable
		}
		if method == SecurityVerificationMethodEmail {
			normalizedEmail, err = normalizeRegistrationEmail(item.Email)
			if err != nil {
				return ErrSecurityVerificationEmailInvalid
			}
		}
		if method != SecurityVerificationMethodNone {
			if err = s.verifySecurityCodeWithMethod(ctx, verifySecurityCodeInput{
				User:       item,
				Method:     method,
				Purpose:    domainuser.ContactVerificationPurposePasswordChange,
				Target:     normalizedEmail,
				Code:       input.Code,
				VerifiedAt: now,
			}); err != nil {
				return ErrSecurityVerificationCodeInvalid
			}
		}
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(normalizedPassword), passwordHashCost)
	if err != nil {
		return err
	}
	if err = s.repo.UpdatePassword(ctx, input.UserID, string(passwordHash), domainuser.PasswordOriginUserSet, false); err != nil {
		return err
	}
	if err = s.repo.RevokeAllSessions(ctx, input.UserID, "password_change"); err != nil {
		return err
	}

	normalizedAuditCtx := s.resolveSessionAuditContext(ctx, input.AuditContext)
	s.RecordAuthEvent(
		ctx,
		repository.AuthEventInput{
			UserID:    input.UserID,
			RequestID: input.RequestID,
			EventType: "password_change",
			Result:    "success",
			ClientIP:  normalizedAuditCtx.ClientIP,
			UserAgent: normalizedAuditCtx.UserAgent,
			DetailJSON: marshalAuthEventDetail(map[string]any{
				"email": normalizedEmail,
			}),
		},
	)
	return nil
}

// RequestPasswordReset sends a password reset code without revealing account existence.
func (s *Service) RequestPasswordReset(ctx context.Context, email string, requestID string, auditCtx requestmeta.SessionAuditContext) (*PasswordResetStartResult, error) {
	cfg := s.cfg.Snapshot()
	normalizedEmail, err := normalizeRegistrationEmail(email)
	if err != nil {
		return nil, err
	}
	item, _, ok, err := s.resolvePasswordResetTarget(ctx, normalizedEmail, cfg)
	if err != nil {
		return nil, err
	}
	if !ok {
		s.recordPasswordResetEvent(ctx, passwordResetAuditInput{
			RequestID:    requestID,
			Result:       "failure",
			Reason:       "unavailable",
			Email:        normalizedEmail,
			AuditContext: auditCtx,
		})
		return inactivePasswordResetStartResult(), nil
	}

	now := time.Now()
	existingVerification, err := s.repo.GetPendingContactVerificationForUser(ctx, item.ID, domainuser.ContactVerificationChannelEmail, domainuser.ContactVerificationPurposePasswordReset, normalizedEmail, now)
	if err == nil && existingVerification.SentAt != nil && now.Sub(*existingVerification.SentAt) < emailRegistrationSendCooldown {
		s.recordPasswordResetEvent(ctx, passwordResetAuditInput{
			UserID:       item.ID,
			RequestID:    requestID,
			Result:       "failure",
			Reason:       "sent_recently",
			Email:        normalizedEmail,
			AuditContext: auditCtx,
		})
		if existingVerification.ExpiresAt == nil {
			return inactivePasswordResetStartResult(), nil
		}
		return &PasswordResetStartResult{
			Sent:      true,
			ExpiresAt: *existingVerification.ExpiresAt,
		}, nil
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	expiresAt := now.Add(emailRegistrationCodeTTL)
	resetCode, err := generateNumericCode(emailRegistrationCodeDigits)
	if err != nil {
		return nil, err
	}
	token := conv.NormalizePublicID(uuid.NewString())
	codeHash := hashRegistrationCode(cfg.JWTSecret, token, resetCode)

	if err = s.repo.CancelPendingContactVerificationsForUser(ctx, item.ID, domainuser.ContactVerificationChannelEmail, domainuser.ContactVerificationPurposePasswordReset, normalizedEmail); err != nil {
		return nil, err
	}
	created, err := s.repo.CreateContactVerification(ctx, &domainuser.ContactVerification{
		UserID:    item.ID,
		Channel:   domainuser.ContactVerificationChannelEmail,
		Purpose:   domainuser.ContactVerificationPurposePasswordReset,
		Target:    normalizedEmail,
		Token:     token,
		CodeHash:  codeHash,
		Status:    domainuser.ContactVerificationStatusPending,
		SentAt:    &now,
		ExpiresAt: &expiresAt,
	})
	if err != nil {
		return nil, err
	}

	if err = s.sendPasswordResetVerificationEmail(normalizedEmail, resetCode); err != nil {
		_ = s.repo.CancelPendingContactVerificationsForUser(ctx, item.ID, domainuser.ContactVerificationChannelEmail, domainuser.ContactVerificationPurposePasswordReset, normalizedEmail)
		s.recordPasswordResetEvent(ctx, passwordResetAuditInput{
			UserID:       item.ID,
			RequestID:    requestID,
			Result:       "failure",
			Reason:       "send_failed",
			Email:        normalizedEmail,
			AuditContext: auditCtx,
		})
		return nil, err
	}
	s.recordPasswordResetEvent(ctx, passwordResetAuditInput{
		UserID:       item.ID,
		RequestID:    requestID,
		Result:       "success",
		Email:        normalizedEmail,
		AuditContext: auditCtx,
		Detail:       map[string]any{"verification_id": created.ID, "expires_at": expiresAt},
	})
	return &PasswordResetStartResult{
		Sent:      true,
		ExpiresAt: expiresAt,
	}, nil
}

// CompletePasswordReset verifies a reset code and sets the new password.
func (s *Service) CompletePasswordReset(ctx context.Context, email string, code string, newPassword string, requestID string, auditCtx requestmeta.SessionAuditContext) error {
	cfg := s.cfg.Snapshot()
	normalizedEmail, err := normalizeRegistrationEmail(email)
	if err != nil {
		return err
	}
	normalizedPassword, err := userapp.NormalizePassword(newPassword)
	if err != nil {
		return err
	}
	item, _, ok, err := s.resolvePasswordResetTarget(ctx, normalizedEmail, cfg)
	if err != nil {
		return err
	}
	if !ok {
		s.recordPasswordResetEvent(ctx, passwordResetAuditInput{
			RequestID:    requestID,
			Result:       "failure",
			Reason:       "unavailable",
			Email:        normalizedEmail,
			AuditContext: auditCtx,
		})
		return ErrPasswordResetFailed
	}

	now := time.Now()
	if err = s.verifyEmailCode(ctx, item.ID, domainuser.ContactVerificationPurposePasswordReset, normalizedEmail, strings.TrimSpace(code), now); err != nil {
		s.recordPasswordResetEvent(ctx, passwordResetAuditInput{
			UserID:       item.ID,
			RequestID:    requestID,
			Result:       "failure",
			Reason:       "invalid_code",
			Email:        normalizedEmail,
			AuditContext: auditCtx,
		})
		return ErrPasswordResetFailed
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(normalizedPassword), passwordHashCost)
	if err != nil {
		return err
	}
	if err = s.repo.UpdatePassword(ctx, item.ID, string(passwordHash), domainuser.PasswordOriginUserSet, false); err != nil {
		return err
	}
	if item.Status == domainuser.StatusLocked {
		if err = s.repo.UpdateUserStatus(ctx, item.ID, domainuser.StatusActive); err != nil {
			return err
		}
	}
	if err = s.repo.RevokeAllSessions(ctx, item.ID, "password_reset"); err != nil {
		return err
	}
	s.recordPasswordResetEvent(ctx, passwordResetAuditInput{
		UserID:       item.ID,
		RequestID:    requestID,
		Result:       "success",
		Email:        normalizedEmail,
		AuditContext: auditCtx,
	})
	return nil
}

func (s *Service) resolvePasswordResetTarget(ctx context.Context, email string, cfg config.Config) (*domainuser.User, *domainuser.Credential, bool, error) {
	if !passwordResetEnabled(cfg) {
		return nil, nil, false, nil
	}
	item, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, false, nil
		}
		return nil, nil, false, err
	}
	if !hasVerifiedEmail(item) || item.Email != email || (item.Status != domainuser.StatusActive && item.Status != domainuser.StatusLocked) {
		return item, nil, false, nil
	}
	credential, err := s.repo.GetCredentialByUserID(ctx, item.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return item, nil, false, nil
		}
		return item, nil, false, err
	}
	if !credential.PasswordEnabled {
		return item, credential, false, nil
	}
	return item, credential, true, nil
}

func passwordResetEnabled(cfg config.Config) bool {
	if !cfg.PasswordResetEnabled || !cfg.EmailVerificationEnabled {
		return false
	}
	if !cfg.UsernameLoginEnabled && !cfg.EmailLoginEnabled {
		return false
	}
	return strings.TrimSpace(cfg.SMTPHost) != "" &&
		cfg.SMTPPort > 0 &&
		cfg.SMTPPort <= 65535 &&
		strings.TrimSpace(cfg.SMTPUsername) != "" &&
		strings.TrimSpace(cfg.SMTPPassword) != ""
}

func inactivePasswordResetStartResult() *PasswordResetStartResult {
	return &PasswordResetStartResult{
		Sent:      true,
		ExpiresAt: time.Now().Add(emailRegistrationCodeTTL),
	}
}

func (s *Service) recordPasswordResetEvent(ctx context.Context, input passwordResetAuditInput) {
	detail := map[string]any{"email_hash": hashAuditEmail(input.Email)}
	for key, value := range input.Detail {
		detail[key] = value
	}
	normalizedAuditCtx := s.resolveSessionAuditContext(ctx, input.AuditContext)
	s.RecordAuthEvent(
		ctx,
		repository.AuthEventInput{
			UserID:     input.UserID,
			RequestID:  input.RequestID,
			EventType:  "password_reset",
			Result:     input.Result,
			Reason:     input.Reason,
			ClientIP:   normalizedAuditCtx.ClientIP,
			UserAgent:  normalizedAuditCtx.UserAgent,
			DetailJSON: marshalAuthEventDetail(detail),
		},
	)
}

func hashAuditEmail(email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

// RequestEmailBootstrapVerification sends a code for setting the initial account email.
func (s *Service) RequestEmailBootstrapVerification(ctx context.Context, userID uint, newEmail string, requestID string, auditCtx requestmeta.SessionAuditContext) (*EmailChangeVerificationStartResult, error) {
	cfg := s.cfg.Snapshot()
	if !cfg.EmailVerificationEnabled {
		return &EmailChangeVerificationStartResult{Sent: false, Method: SecurityVerificationMethodNone, AvailableMethods: []SecurityVerificationMethod{SecurityVerificationMethodNone}}, nil
	}
	item, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !canBootstrapEmail(item) {
		return nil, ErrEmailBootstrapNotAllowed
	}
	normalizedEmail, err := normalizeRegistrationEmail(newEmail)
	if err != nil {
		return nil, err
	}
	if err = validateEmailRegistrationPolicy(cfg, normalizedEmail); err != nil {
		return nil, err
	}
	if existingUser, findErr := s.repo.GetByEmail(ctx, normalizedEmail); findErr == nil && existingUser.ID != userID {
		return nil, ErrEmailAlreadyExists
	} else if findErr != nil && !errors.Is(findErr, repository.ErrNotFound) {
		return nil, findErr
	}
	return s.requestEmailVerificationCode(ctx, requestEmailVerificationCodeInput{
		UserID:       userID,
		Purpose:      domainuser.ContactVerificationPurposeEmailBootstrapNew,
		Target:       normalizedEmail,
		EventType:    "email_bootstrap_code",
		RequestID:    requestID,
		AuditContext: auditCtx,
	})
}

// CompleteEmailBootstrap verifies and stores the initial account email.
func (s *Service) CompleteEmailBootstrap(ctx context.Context, userID uint, newEmail string, code string, requestID string, auditCtx requestmeta.SessionAuditContext) (*domainuser.User, error) {
	cfg := s.cfg.Snapshot()
	item, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !canBootstrapEmail(item) {
		return nil, ErrEmailBootstrapNotAllowed
	}
	normalizedEmail, err := normalizeRegistrationEmail(newEmail)
	if err != nil {
		return nil, err
	}
	if err = validateEmailRegistrationPolicy(cfg, normalizedEmail); err != nil {
		return nil, err
	}
	if existingUser, findErr := s.repo.GetByEmail(ctx, normalizedEmail); findErr == nil && existingUser.ID != userID {
		return nil, ErrEmailAlreadyExists
	} else if findErr != nil && !errors.Is(findErr, repository.ErrNotFound) {
		return nil, findErr
	}
	now := time.Now()
	if cfg.EmailVerificationEnabled {
		if err = s.verifyEmailCode(ctx, userID, domainuser.ContactVerificationPurposeEmailBootstrapNew, normalizedEmail, strings.TrimSpace(code), now); err != nil {
			return nil, err
		}
	}
	emailVerifiedAt := optionalEmailVerifiedAt(cfg.EmailVerificationEnabled, now)
	emailSource := domainuser.EmailSourceUserSet
	emailBootstrapUsedAt := &now
	updated, err := s.repo.UpdateProfile(ctx, userID, repository.UpdateUserFieldsInput{
		Email:                &normalizedEmail,
		EmailVerifiedAt:      &emailVerifiedAt,
		EmailSource:          &emailSource,
		EmailBootstrapUsedAt: &emailBootstrapUsedAt,
	})
	if err != nil {
		return nil, err
	}
	s.recordEmailSecurityEvent(ctx, userID, requestID, "email_bootstrap", normalizedEmail, auditCtx)
	return updated, nil
}

// RequestCurrentEmailVerification sends a verification code for the current email.
func (s *Service) RequestCurrentEmailVerification(ctx context.Context, userID uint, requestID string, auditCtx requestmeta.SessionAuditContext) (*EmailChangeVerificationStartResult, error) {
	cfg := s.cfg.Snapshot()
	if !cfg.EmailVerificationEnabled {
		return &EmailChangeVerificationStartResult{Sent: false, Method: SecurityVerificationMethodNone, AvailableMethods: []SecurityVerificationMethod{SecurityVerificationMethodNone}}, nil
	}
	item, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !hasEmailCandidate(item) {
		return nil, ErrCurrentEmailNotVerified
	}
	normalizedEmail, err := normalizeRegistrationEmail(item.Email)
	if err != nil {
		return nil, ErrSecurityVerificationEmailInvalid
	}
	return s.requestEmailVerificationCode(ctx, requestEmailVerificationCodeInput{
		UserID:       userID,
		Purpose:      domainuser.ContactVerificationPurposeEmailVerifyCurrent,
		Target:       normalizedEmail,
		EventType:    "email_verify_current_code",
		RequestID:    requestID,
		AuditContext: auditCtx,
	})
}

// CompleteCurrentEmailVerification verifies the current account email.
func (s *Service) CompleteCurrentEmailVerification(ctx context.Context, userID uint, code string, requestID string, auditCtx requestmeta.SessionAuditContext) (*domainuser.User, error) {
	cfg := s.cfg.Snapshot()
	if !cfg.EmailVerificationEnabled {
		return nil, ErrEmailVerificationDisabled
	}
	item, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !hasEmailCandidate(item) {
		return nil, ErrCurrentEmailNotVerified
	}
	normalizedEmail, err := normalizeRegistrationEmail(item.Email)
	if err != nil {
		return nil, ErrSecurityVerificationEmailInvalid
	}
	now := time.Now()
	if err = s.verifyEmailCode(ctx, userID, domainuser.ContactVerificationPurposeEmailVerifyCurrent, normalizedEmail, strings.TrimSpace(code), now); err != nil {
		return nil, err
	}
	emailVerifiedAt := &now
	updated, err := s.repo.UpdateProfile(ctx, userID, repository.UpdateUserFieldsInput{
		EmailVerifiedAt: &emailVerifiedAt,
	})
	if err != nil {
		return nil, err
	}
	s.recordEmailSecurityEvent(ctx, userID, requestID, "email_verify_current", normalizedEmail, auditCtx)
	return updated, nil
}

// RequestCurrentEmailChangeVerification starts the security verification for an email change.
func (s *Service) RequestCurrentEmailChangeVerification(ctx context.Context, userID uint, requestedMethod string, requestID string, auditCtx requestmeta.SessionAuditContext) (*EmailChangeVerificationStartResult, error) {
	item, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	methods, err := s.resolveSecurityVerificationMethods(ctx, item)
	if err != nil {
		return nil, err
	}
	method := methods[0]
	if normalizedMethod := normalizeSecurityVerificationMethod(requestedMethod); normalizedMethod != "" {
		method = normalizedMethod
	}
	if !containsSecurityVerificationMethod(methods, method) {
		return nil, ErrSecurityVerificationMethodUnavailable
	}
	if method != SecurityVerificationMethodEmail {
		return &EmailChangeVerificationStartResult{Sent: false, Method: method, AvailableMethods: methods}, nil
	}
	normalizedEmail, err := normalizeRegistrationEmail(item.Email)
	if err != nil {
		return nil, ErrSecurityVerificationEmailInvalid
	}
	return s.requestEmailVerificationCode(ctx, requestEmailVerificationCodeInput{
		UserID:       userID,
		Purpose:      domainuser.ContactVerificationPurposeEmailChangeCurrent,
		Target:       normalizedEmail,
		EventType:    "email_change_current_code",
		RequestID:    requestID,
		AuditContext: auditCtx,
	})
}

// RequestNewEmailChangeVerification sends a code to the proposed new email.
func (s *Service) RequestNewEmailChangeVerification(ctx context.Context, userID uint, newEmail string, requestID string, auditCtx requestmeta.SessionAuditContext) (*EmailChangeVerificationStartResult, error) {
	cfg := s.cfg.Snapshot()
	if !cfg.EmailVerificationEnabled {
		return &EmailChangeVerificationStartResult{Sent: false, Method: SecurityVerificationMethodNone, AvailableMethods: []SecurityVerificationMethod{SecurityVerificationMethodNone}}, nil
	}
	normalizedEmail, err := normalizeRegistrationEmail(newEmail)
	if err != nil {
		return nil, err
	}
	if err = validateEmailRegistrationPolicy(cfg, normalizedEmail); err != nil {
		return nil, err
	}
	if _, err = s.repo.GetByEmail(ctx, normalizedEmail); err == nil {
		return nil, ErrEmailAlreadyExists
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	return s.requestEmailVerificationCode(ctx, requestEmailVerificationCodeInput{
		UserID:       userID,
		Purpose:      domainuser.ContactVerificationPurposeEmailChangeNew,
		Target:       normalizedEmail,
		EventType:    "email_change_new_code",
		RequestID:    requestID,
		AuditContext: auditCtx,
	})
}

// CompleteEmailChange verifies both sides of an email change and updates the account.
func (s *Service) CompleteEmailChange(ctx context.Context, input CompleteEmailChangeInput) (*domainuser.User, error) {
	cfg := s.cfg.Snapshot()
	item, err := s.repo.GetByID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	methods, err := s.resolveSecurityVerificationMethods(ctx, item)
	if err != nil {
		return nil, err
	}
	method := methods[0]
	if normalizedMethod := normalizeSecurityVerificationMethod(input.CurrentVerificationMethod); normalizedMethod != "" {
		method = normalizedMethod
	}
	if !containsSecurityVerificationMethod(methods, method) {
		return nil, ErrSecurityVerificationMethodUnavailable
	}
	currentEmail := ""
	if strings.TrimSpace(item.Email) != "" {
		currentEmail, err = normalizeRegistrationEmail(item.Email)
		if err != nil {
			return nil, ErrSecurityVerificationEmailInvalid
		}
	}
	normalizedEmail, err := normalizeRegistrationEmail(input.NewEmail)
	if err != nil {
		return nil, err
	}
	if currentEmail != "" && normalizedEmail == currentEmail {
		return nil, ErrEmailUnchanged
	}
	if err = validateEmailRegistrationPolicy(cfg, normalizedEmail); err != nil {
		return nil, err
	}
	if _, err = s.repo.GetByEmail(ctx, normalizedEmail); err == nil {
		return nil, ErrEmailAlreadyExists
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	now := time.Now()
	if method != SecurityVerificationMethodNone {
		if method == SecurityVerificationMethodEmail && currentEmail == "" {
			return nil, ErrSecurityVerificationEmailInvalid
		}
		if err = s.verifySecurityCodeWithMethod(ctx, verifySecurityCodeInput{
			User:       item,
			Method:     method,
			Purpose:    domainuser.ContactVerificationPurposeEmailChangeCurrent,
			Target:     currentEmail,
			Code:       input.CurrentCode,
			VerifiedAt: now,
		}); err != nil {
			return nil, err
		}
	}
	if cfg.EmailVerificationEnabled {
		if err = s.verifyEmailCode(ctx, input.UserID, domainuser.ContactVerificationPurposeEmailChangeNew, normalizedEmail, strings.TrimSpace(input.NewCode), now); err != nil {
			return nil, err
		}
	}
	emailVerifiedAt := optionalEmailVerifiedAt(cfg.EmailVerificationEnabled, now)
	emailSource := domainuser.EmailSourceUserSet
	updated, err := s.repo.UpdateProfile(ctx, input.UserID, repository.UpdateUserFieldsInput{
		Email:           &normalizedEmail,
		EmailVerifiedAt: &emailVerifiedAt,
		EmailSource:     &emailSource,
	})
	if err != nil {
		return nil, err
	}
	s.recordEmailSecurityEvent(ctx, input.UserID, input.RequestID, "email_change", normalizedEmail, input.AuditContext)
	return updated, nil
}

func normalizeRegistrationEmail(raw string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" || len(normalized) > 128 || containsEmailControlCharacter(normalized) {
		return "", ErrInvalidEmail
	}
	parsed, err := mail.ParseAddress(normalized)
	if err != nil || strings.ToLower(parsed.Address) != normalized {
		return "", ErrInvalidEmail
	}
	return normalized, nil
}

func containsEmailControlCharacter(value string) bool {
	return strings.ContainsFunc(value, func(r rune) bool {
		return r == 0 || r == '\r' || r == '\n'
	})
}

func validateEmailRegistrationPolicy(cfg config.Config, email string) error {
	local, domain, ok := strings.Cut(email, "@")
	if !ok || local == "" || domain == "" {
		return ErrInvalidEmail
	}
	if cfg.EmailRegistrationNoAlias && strings.Contains(local, "+") {
		return ErrEmailAliasNotAllowed
	}
	allowedDomains := splitRegistrationDomains(cfg.EmailRegistrationDomains)
	if len(allowedDomains) == 0 {
		return nil
	}
	normalizedDomain := strings.ToLower(strings.TrimSpace(domain))
	for _, allowed := range allowedDomains {
		if normalizedDomain == allowed {
			return nil
		}
	}
	return ErrEmailDomainNotAllowed
}

func splitRegistrationDomains(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	domains := make([]string, 0, len(parts))
	for _, part := range parts {
		domain := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(part)), "@")
		if domain != "" {
			domains = append(domains, domain)
		}
	}
	return domains
}

func generateNumericCode(digits int) (string, error) {
	max := big.NewInt(emailRegistrationCodeMaxValue)
	value, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", digits, value.Int64()), nil
}

func hashRegistrationCode(secret string, token string, code string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret) + ":" + strings.TrimSpace(token) + ":" + strings.TrimSpace(code)))
	return hex.EncodeToString(sum[:])
}

func verifyRegistrationCode(secret string, token string, code string, expectedHash string) bool {
	actual := hashRegistrationCode(secret, token, code)
	return subtle.ConstantTimeCompare([]byte(actual), []byte(strings.TrimSpace(expectedHash))) == 1
}

func registrationUsername(email string) string {
	local := email
	if at := strings.Index(email, "@"); at > 0 {
		local = email[:at]
	}
	prefix := normalizeProviderSlug(local)
	if prefix == "" {
		prefix = "user"
	}
	sum := sha256.Sum256([]byte(email))
	suffix := hex.EncodeToString(sum[:])[:8]
	maxPrefixLength := userapp.UsernameMaxLength - 9
	if len(prefix) > maxPrefixLength {
		prefix = strings.Trim(prefix[:maxPrefixLength], "-_")
	}
	return prefix + "-" + suffix
}

func generatedUsernameWithSuffix(base string, attempt int) string {
	normalized := strings.Trim(strings.TrimSpace(base), "-_")
	if normalized == "" {
		normalized = "user"
	}
	if attempt <= 0 {
		if len(normalized) > userapp.UsernameMaxLength {
			return strings.Trim(normalized[:userapp.UsernameMaxLength], "-_")
		}
		return normalized
	}
	suffix := fmt.Sprintf("-%d", attempt+1)
	limit := userapp.UsernameMaxLength - len(suffix)
	if limit < 1 {
		limit = 1
	}
	if len(normalized) > limit {
		normalized = strings.Trim(normalized[:limit], "-_")
	}
	if normalized == "" {
		normalized = "user"
	}
	return normalized + suffix
}

func (s *Service) createWithCredentialUsingAvailableUsername(ctx context.Context, input repository.CreateWithCredentialInput) error {
	baseUsername := input.User.Username
	for attempt := 0; attempt < 20; attempt++ {
		input.User.ID = 0
		input.User.Username = generatedUsernameWithSuffix(baseUsername, attempt)
		err := s.repo.CreateWithCredential(ctx, input)
		if errors.Is(err, repository.ErrDuplicateUsername) {
			continue
		}
		return err
	}
	return ErrUsernameTaken
}

func registrationDisplayName(email string) string {
	if at := strings.Index(email, "@"); at > 0 {
		return email[:at]
	}
	return email
}

func optionalEmailVerifiedAt(enabled bool, now time.Time) *time.Time {
	if !enabled {
		return nil
	}
	return &now
}

func canBootstrapEmail(item *domainuser.User) bool {
	if item == nil || item.EmailVerifiedAt != nil || item.EmailBootstrapUsedAt != nil {
		return false
	}
	if strings.TrimSpace(item.Email) == "" {
		return true
	}
	return item.EmailSource == domainuser.EmailSourceProviderUnverified
}

func (s *Service) sendRegistrationVerificationEmail(to string, code string) error {
	return s.sendEmailVerificationCode(to, code, verificationEmailTemplate{
		Subject:      "DEEIX Chat 验证码",
		Title:        "完成邮箱注册",
		SecurityNote: "如果不是您本人操作，请忽略这封邮件。",
	}, "email registration")
}

func (s *Service) sendPasswordChangeVerificationEmail(to string, code string) error {
	return s.sendEmailVerificationCode(to, code, verificationEmailTemplate{
		Subject:      "DEEIX Chat 验证码",
		Title:        "确认修改密码",
		SecurityNote: "如果不是您本人操作，请立即检查账号安全。",
	}, "password change")
}

func (s *Service) sendPasswordResetVerificationEmail(to string, code string) error {
	return s.sendEmailVerificationCode(to, code, verificationEmailTemplate{
		Subject:      "DEEIX Chat 验证码",
		Title:        "重置密码",
		SecurityNote: "如果不是您本人操作，请立即检查账号安全。",
	}, "password reset")
}

func (s *Service) sendEmailChangeVerificationEmail(to string, code string) error {
	return s.sendEmailVerificationCode(to, code, verificationEmailTemplate{
		Subject:      "DEEIX Chat 验证码",
		Title:        "验证邮箱地址",
		SecurityNote: "如果不是您本人操作，请忽略这封邮件。",
	}, "email change")
}

func (s *Service) sendAccountDeleteVerificationEmail(to string, code string) error {
	return s.sendEmailVerificationCode(to, code, verificationEmailTemplate{
		Subject:      "DEEIX Chat 验证码",
		Title:        "确认删除账号",
		SecurityNote: "如果不是您本人操作，请立即检查账号安全。",
	}, "account deletion")
}

func (s *Service) requestEmailVerificationCode(ctx context.Context, input requestEmailVerificationCodeInput) (*EmailChangeVerificationStartResult, error) {
	if input.UserID == 0 {
		return nil, ErrUserIDRequired
	}
	cfg := s.cfg.Snapshot()
	now := time.Now()
	existingVerification, err := s.repo.GetPendingContactVerificationForUser(ctx, input.UserID, domainuser.ContactVerificationChannelEmail, input.Purpose, input.Target, now)
	if err == nil && existingVerification.SentAt != nil && now.Sub(*existingVerification.SentAt) < emailRegistrationSendCooldown {
		return nil, ErrVerificationCodeRecent
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	expiresAt := now.Add(emailRegistrationCodeTTL)
	code, err := generateNumericCode(emailRegistrationCodeDigits)
	if err != nil {
		return nil, err
	}
	token := conv.NormalizePublicID(uuid.NewString())
	codeHash := hashRegistrationCode(cfg.JWTSecret, token, code)

	if err = s.repo.CancelPendingContactVerificationsForUser(ctx, input.UserID, domainuser.ContactVerificationChannelEmail, input.Purpose, input.Target); err != nil {
		return nil, err
	}
	created, err := s.repo.CreateContactVerification(ctx, &domainuser.ContactVerification{
		UserID:    input.UserID,
		Channel:   domainuser.ContactVerificationChannelEmail,
		Purpose:   input.Purpose,
		Target:    input.Target,
		Token:     token,
		CodeHash:  codeHash,
		Status:    domainuser.ContactVerificationStatusPending,
		SentAt:    &now,
		ExpiresAt: &expiresAt,
	})
	if err != nil {
		return nil, err
	}

	if err := s.sendEmailVerificationByPurpose(input.Purpose, input.Target, code); err != nil {
		_ = s.repo.CancelPendingContactVerificationsForUser(ctx, input.UserID, domainuser.ContactVerificationChannelEmail, input.Purpose, input.Target)
		return nil, err
	}
	normalizedAuditCtx := s.resolveSessionAuditContext(ctx, input.AuditContext)
	s.RecordAuthEvent(
		ctx,
		repository.AuthEventInput{
			UserID:    input.UserID,
			RequestID: input.RequestID,
			EventType: input.EventType,
			Result:    "success",
			ClientIP:  normalizedAuditCtx.ClientIP,
			UserAgent: normalizedAuditCtx.UserAgent,
			DetailJSON: marshalAuthEventDetail(map[string]any{
				"email":           input.Target,
				"verification_id": created.ID,
				"expires_at":      expiresAt,
			}),
		},
	)
	return &EmailChangeVerificationStartResult{
		Sent:             true,
		ExpiresAt:        expiresAt,
		Method:           SecurityVerificationMethodEmail,
		AvailableMethods: []SecurityVerificationMethod{SecurityVerificationMethodEmail},
	}, nil
}

func (s *Service) sendEmailVerificationByPurpose(purpose string, target string, code string) error {
	switch purpose {
	case domainuser.ContactVerificationPurposeAccountDelete:
		return s.sendAccountDeleteVerificationEmail(target, code)
	default:
		return s.sendEmailChangeVerificationEmail(target, code)
	}
}

func (s *Service) verifyEmailCode(ctx context.Context, userID uint, purpose string, target string, code string, now time.Time) error {
	if userID == 0 {
		return ErrSecurityVerificationCodeInvalid
	}
	cfg := s.cfg.Snapshot()
	verification, err := s.repo.GetPendingContactVerificationForUser(ctx, userID, domainuser.ContactVerificationChannelEmail, purpose, target, now)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrSecurityVerificationCodeInvalid
		}
		return err
	}
	if verification.UserID != 0 && verification.UserID != userID {
		return ErrSecurityVerificationCodeInvalid
	}
	if verification.AttemptCount >= emailRegistrationMaxAttempts {
		return ErrVerificationCodeAttempts
	}
	if !verifyRegistrationCode(cfg.JWTSecret, verification.Token, strings.TrimSpace(code), verification.CodeHash) {
		_ = s.repo.IncrementContactVerificationAttempt(ctx, verification.ID)
		return ErrSecurityVerificationCodeInvalid
	}
	return s.repo.MarkContactVerificationVerified(ctx, verification.ID, now)
}

func (s *Service) recordEmailSecurityEvent(ctx context.Context, userID uint, requestID string, eventType string, email string, auditCtx requestmeta.SessionAuditContext) {
	normalizedAuditCtx := s.resolveSessionAuditContext(ctx, auditCtx)
	s.RecordAuthEvent(
		ctx,
		repository.AuthEventInput{
			UserID:    userID,
			RequestID: requestID,
			EventType: eventType,
			Result:    "success",
			ClientIP:  normalizedAuditCtx.ClientIP,
			UserAgent: normalizedAuditCtx.UserAgent,
			DetailJSON: marshalAuthEventDetail(map[string]any{
				"email": email,
			}),
		},
	)
}

func (s *Service) sendEmailVerificationCode(to string, code string, template verificationEmailTemplate, logLabel string) error {
	cfg := s.cfg.Snapshot()
	from := strings.TrimSpace(cfg.SMTPFrom)
	if from == "" {
		from = strings.TrimSpace(cfg.SMTPUsername)
	}
	if strings.TrimSpace(cfg.SMTPHost) == "" || from == "" {
		return ErrSMTPNotConfigured
	}
	parsedFrom, err := mail.ParseAddress(from)
	if err != nil {
		return ErrSMTPFromInvalid
	}
	normalizedTo, err := normalizeRegistrationEmail(to)
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(strings.TrimSpace(cfg.SMTPHost), fmt.Sprintf("%d", cfg.SMTPPort))
	var auth smtp.Auth
	if strings.TrimSpace(cfg.SMTPUsername) != "" || strings.TrimSpace(cfg.SMTPPassword) != "" {
		auth = smtp.PlainAuth("", strings.TrimSpace(cfg.SMTPUsername), strings.TrimSpace(cfg.SMTPPassword), strings.TrimSpace(cfg.SMTPHost))
	}
	message := buildVerificationEmailMessage(parsedFrom.String(), normalizedTo, code, template, publicAssetURL(cfg.PublicWebBaseURL, "logo.svg"))
	if err := sendSMTPMail(smtpConnection{
		Addr: addr,
		Host: strings.TrimSpace(cfg.SMTPHost),
		Port: cfg.SMTPPort,
		Auth: auth,
		From: parsedFrom.Address,
		To:   []string{normalizedTo},
	}, []byte(message)); err != nil {
		s.warn("email_verification_send_failed",
			zap.String("label", strings.TrimSpace(logLabel)),
			zap.String("email", normalizedTo),
			zap.Error(err),
		)
		return err
	}
	return nil
}

func buildVerificationEmailMessage(from string, to string, code string, template verificationEmailTemplate, logoURL string) string {
	plainBody := buildVerificationPlainText(code, template)
	htmlBody := buildVerificationHTML(code, template, logoURL)
	var multipartBody bytes.Buffer
	writer := multipart.NewWriter(&multipartBody)
	writeEmailPart(writer, "text/plain; charset=UTF-8", plainBody)
	writeEmailPart(writer, "text/html; charset=UTF-8", htmlBody)
	_ = writer.Close()

	headers := textproto.MIMEHeader{}
	headers.Set("From", from)
	headers.Set("To", to)
	headers.Set("Subject", mime.QEncoding.Encode("utf-8", template.Subject))
	headers.Set("MIME-Version", "1.0")
	headers.Set("Content-Type", fmt.Sprintf(`multipart/alternative; boundary="%s"`, writer.Boundary()))

	lines := make([]string, 0, len(headers)+2)
	for _, key := range []string{"From", "To", "Subject", "MIME-Version", "Content-Type"} {
		lines = append(lines, key+": "+headers.Get(key))
	}
	lines = append(lines, "", multipartBody.String())
	return strings.Join(lines, "\r\n")
}

func writeEmailPart(writer *multipart.Writer, contentType string, body string) {
	headers := textproto.MIMEHeader{}
	headers.Set("Content-Type", contentType)
	headers.Set("Content-Transfer-Encoding", "8bit")
	part, _ := writer.CreatePart(headers)
	_, _ = part.Write([]byte(body))
}

func buildVerificationPlainText(code string, template verificationEmailTemplate) string {
	return fmt.Sprintf(`DEEIX Chat

%s

验证码：%s

10 分钟内有效，请不要泄露给任何人。

%s`, template.Title, strings.TrimSpace(code), template.SecurityNote)
}

func buildVerificationHTML(code string, template verificationEmailTemplate, logoURL string) string {
	escapedCode := html.EscapeString(strings.TrimSpace(code))
	escapedTitle := html.EscapeString(strings.TrimSpace(template.Title))
	escapedSecurityNote := html.EscapeString(strings.TrimSpace(template.SecurityNote))
	logoHTML := verificationEmailLogoHTML(logoURL)
	return fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>%s</title>
  </head>
  <body style="margin:0;background:#fbfaf7;color:#312f2b;font-family:ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,'Noto Sans SC','Microsoft YaHei',sans-serif;">
    <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:#fbfaf7;padding:56px 18px;">
      <tr>
        <td align="center">
          <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:360px;">
            <tr>
              <td align="center" style="padding:0 0 24px;">
                %s
              </td>
            </tr>
            <tr>
              <td align="center" style="padding:0;">
                <div style="display:inline-block;width:22px;height:3px;border-radius:999px;background:#b85f24;"></div>
                <h1 style="margin:18px 0 0;font-size:22px;line-height:1.35;font-weight:650;color:#26231f;letter-spacing:0;">%s</h1>
              </td>
            </tr>
            <tr>
              <td style="padding:28px 0 0;">
                <div style="margin:0 0 8px;font-size:13px;font-weight:600;line-height:1.4;color:#312f2b;">验证码</div>
                <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:#eee8dc;border-radius:8px;">
                  <tr>
                    <td align="center" style="padding:17px 18px 16px;">
                      <span style="font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,'Liberation Mono','Courier New',monospace;font-size:30px;line-height:1.15;font-weight:700;letter-spacing:.16em;color:#26231f;">%s</span>
                    </td>
                  </tr>
                </table>
                <div style="margin-top:8px;font-size:12px;line-height:1.65;color:#8c8378;">10 分钟内有效，请不要泄露给任何人。%s</div>
              </td>
            </tr>
            <tr>
              <td align="center" style="padding:36px 0 0;font-size:12px;line-height:1.6;color:#aaa298;">这是一封系统邮件，请勿直接回复。</td>
            </tr>
          </table>
        </td>
      </tr>
    </table>
  </body>
</html>`, escapedTitle, logoHTML, escapedTitle, escapedCode, escapedSecurityNote)
}

func verificationEmailLogoHTML(logoURL string) string {
	if trimmed := strings.TrimSpace(logoURL); trimmed != "" {
		return fmt.Sprintf(`<img src="%s" width="150" alt="DEEIX Chat" style="display:block;width:150px;height:auto;border:0;outline:none;text-decoration:none;">`, html.EscapeString(trimmed))
	}
	return `<div style="font-size:20px;line-height:1.3;font-weight:700;color:#26231f;">DEEIX Chat</div>`
}

func publicAssetURL(publicWebBaseURL string, assetPath string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(publicWebBaseURL), "/")
	if baseURL == "" {
		return ""
	}
	return baseURL + "/" + strings.TrimLeft(strings.TrimSpace(assetPath), "/")
}

// smtpConnection contains the SMTP session settings and envelope addresses.
// The message payload is passed separately so transport configuration cannot
// be confused with untrusted mail content at the SMTP DATA boundary.
type smtpConnection struct {
	Addr string
	Host string
	Port int
	Auth smtp.Auth
	From string
	To   []string
}

func sendSMTPMail(connection smtpConnection, message []byte) error {
	dialer := net.Dialer{Timeout: emailSMTPTimeout}
	var conn net.Conn
	var err error
	if connection.Port == 465 {
		conn, err = tls.DialWithDialer(&dialer, "tcp", connection.Addr, &tls.Config{
			ServerName:         connection.Host,
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: false,
		})
	} else {
		conn, err = dialer.Dial("tcp", connection.Addr)
	}
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, connection.Host)
	if err != nil {
		return err
	}
	defer client.Close()

	if connection.Port != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err = client.StartTLS(&tls.Config{ServerName: connection.Host, MinVersion: tls.VersionTLS12}); err != nil {
				return err
			}
		}
	}
	if connection.Auth != nil {
		if ok, _ := client.Extension("AUTH"); ok {
			if err = client.Auth(connection.Auth); err != nil {
				return err
			}
		} else {
			return ErrSMTPAuthUnsupported
		}
	}
	if err = client.Mail(connection.From); err != nil {
		return err
	}
	for _, recipient := range connection.To {
		if err = client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = writer.Write(message); err != nil {
		_ = writer.Close()
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}
