package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/token"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

const (
	twoFactorChallengeTokenType = "2fa_challenge"
	twoFactorChallengeTTL       = 5 * time.Minute
	twoFactorSetupTTL           = 30 * time.Minute
	totpStepSeconds             = int64(30)
	totpDigits                  = 6
	totpValidationWindow        = int64(1)
)

type recoveryCodeRecord struct {
	Hash string
}

func (s *Service) buildTwoFactorChallenge(ctx context.Context, item *user.User) (*LoginResult, error) {
	methods, err := s.resolveSecurityVerificationMethods(ctx, item)
	if err != nil {
		return nil, err
	}
	challengeToken, err := token.GenerateWithClaims(
		s.cfg.Snapshot().JWTSecret,
		item.ID,
		item.Username,
		item.Role,
		"",
		"",
		twoFactorChallengeTokenType,
		twoFactorChallengeTTL,
	)
	if err != nil {
		return nil, err
	}
	userView, err := s.buildUserView(ctx, *item)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		User:                    userView,
		TwoFactorRequired:       true,
		TwoFactorChallengeToken: challengeToken,
		VerificationMethods:     methods,
	}, nil
}

func (s *Service) shouldRequireTwoFactor(ctx context.Context, item *user.User) (bool, error) {
	if item == nil {
		return false, nil
	}
	twoFactor, err := s.repo.GetUserTwoFactorByUserID(ctx, item.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return twoFactor.TOTPEnabled && strings.TrimSpace(twoFactor.TOTPSecretEncrypted) != "", nil
}

func (s *Service) VerifyLoginTwoFactor(
	ctx context.Context,
	challengeToken string,
	verificationMethod string,
	code string,
	requestID string,
	auditCtx requestmeta.SessionAuditContext,
) (*LoginResult, error) {
	normalizedAuditCtx := s.resolveSessionAuditContext(ctx, auditCtx)
	claims, err := token.Parse(s.cfg.Snapshot().JWTSecret, strings.TrimSpace(challengeToken))
	if err != nil {
		reason := "invalid_challenge"
		if errors.Is(err, jwt.ErrTokenExpired) {
			reason = "expired_challenge"
		}
		s.RecordAuthEvent(ctx, 0, requestID, "two_factor_verify", "failure", reason, normalizedAuditCtx.ClientIP, normalizedAuditCtx.UserAgent, "")
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTwoFactorChallengeExpired
		}
		return nil, ErrInvalidCredentials
	}
	if claims.TokenType != twoFactorChallengeTokenType || claims.UserID == 0 {
		s.RecordAuthEvent(ctx, 0, requestID, "two_factor_verify", "failure", "invalid_challenge", normalizedAuditCtx.ClientIP, normalizedAuditCtx.UserAgent, "")
		return nil, ErrInvalidCredentials
	}
	item, err := s.repo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	if item.Status != user.StatusActive {
		return nil, ErrInvalidCredentials
	}
	method := normalizeSecurityVerificationMethod(verificationMethod)
	if method == "" {
		method = SecurityVerificationMethodTwoFactor
	}
	methods, err := s.resolveSecurityVerificationMethods(ctx, item)
	if err != nil {
		return nil, err
	}
	if method == SecurityVerificationMethodNone || !containsSecurityVerificationMethod(methods, method) {
		s.RecordAuthEvent(ctx, item.ID, requestID, "two_factor_verify", "failure", "unavailable_method", normalizedAuditCtx.ClientIP, normalizedAuditCtx.UserAgent, "")
		if err = s.markTwoFactorLoginFailure(ctx, item); err != nil {
			return nil, err
		}
		return nil, ErrInvalidCredentials
	}
	switch method {
	case SecurityVerificationMethodTwoFactor:
		err = s.verifyCurrentTwoFactorCode(ctx, item.ID, code)
	case SecurityVerificationMethodEmail:
		normalizedEmail, emailErr := normalizeRegistrationEmail(item.Email)
		if emailErr != nil {
			err = emailErr
		} else {
			err = s.verifyEmailCode(ctx, item.ID, user.ContactVerificationPurposeLogin, normalizedEmail, strings.TrimSpace(code), time.Now())
		}
	}
	if err != nil {
		s.RecordAuthEvent(ctx, item.ID, requestID, "two_factor_verify", "failure", "invalid_code", normalizedAuditCtx.ClientIP, normalizedAuditCtx.UserAgent, "")
		if lockErr := s.markTwoFactorLoginFailure(ctx, item); lockErr != nil {
			return nil, lockErr
		}
		return nil, ErrInvalidCredentials
	}
	if err = s.repo.ResetLoginFailure(ctx, item.ID); err != nil {
		return nil, err
	}
	result, err := s.issueLoginResult(ctx, item, normalizedAuditCtx, time.Now())
	if err != nil {
		return nil, err
	}
	s.RecordAuthEvent(
		ctx,
		item.ID,
		requestID,
		"two_factor_verify",
		"success",
		"",
		normalizedAuditCtx.ClientIP,
		normalizedAuditCtx.UserAgent,
		marshalAuthEventDetail(map[string]interface{}{"session_id": result.SessionID}),
	)
	return result, nil
}

// markTwoFactorLoginFailure 复用登录失败锁定策略，避免 2FA 校验成为独立暴力尝试入口。
func (s *Service) markTwoFactorLoginFailure(ctx context.Context, item *user.User) error {
	if item == nil || item.ID == 0 {
		return nil
	}
	now := time.Now()
	lockUntil := now.Add(s.loginLockDuration())
	updatedCredential, err := s.repo.MarkLoginFailure(ctx, item.ID, s.loginLockThreshold(), lockUntil)
	if err != nil {
		return err
	}
	if updatedCredential.LockedUntil == nil || !now.Before(*updatedCredential.LockedUntil) {
		return nil
	}
	if lockErr := s.repo.UpdateUserStatus(ctx, item.ID, user.StatusLocked); lockErr != nil {
		s.warn("lock_account_failed", zap.Uint("user_id", item.ID), zap.Error(lockErr))
	}
	return ErrAccountLocked
}

func (s *Service) RequestLoginEmailVerification(
	ctx context.Context,
	challengeToken string,
	requestID string,
	auditCtx requestmeta.SessionAuditContext,
) (*EmailChangeVerificationStartResult, error) {
	normalizedAuditCtx := s.resolveSessionAuditContext(ctx, auditCtx)
	claims, err := token.Parse(s.cfg.Snapshot().JWTSecret, strings.TrimSpace(challengeToken))
	if err != nil || claims.TokenType != twoFactorChallengeTokenType || claims.UserID == 0 {
		s.RecordAuthEvent(ctx, 0, requestID, "login_email_code", "failure", "invalid_challenge", normalizedAuditCtx.ClientIP, normalizedAuditCtx.UserAgent, "")
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTwoFactorChallengeExpired
		}
		return nil, ErrInvalidCredentials
	}
	item, err := s.repo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	if item.Status != user.StatusActive {
		return nil, ErrInvalidCredentials
	}
	methods, err := s.resolveSecurityVerificationMethods(ctx, item)
	if err != nil {
		return nil, err
	}
	if !containsSecurityVerificationMethod(methods, SecurityVerificationMethodEmail) {
		return nil, fmt.Errorf("verification method is unavailable")
	}
	normalizedEmail, err := normalizeRegistrationEmail(item.Email)
	if err != nil {
		return nil, fmt.Errorf("user email is invalid")
	}
	return s.requestEmailVerificationCode(ctx, item.ID, user.ContactVerificationPurposeLogin, normalizedEmail, "login_email_code", requestID, auditCtx)
}

func (s *Service) GetCurrentTwoFactorStatus(ctx context.Context, userID uint) (*TwoFactorStatusResult, error) {
	twoFactor, err := s.repo.GetUserTwoFactorByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return &TwoFactorStatusResult{Available: true, Required: false}, nil
		}
		return nil, err
	}
	return &TwoFactorStatusResult{
		Available:     true,
		TOTPEnabled:   twoFactor.TOTPEnabled,
		Required:      false,
		RecoveryCount: len(parseRecoveryCodeRecords(twoFactor.RecoveryCodesHash)),
		EnabledAt:     twoFactor.EnabledAt,
	}, nil
}

func (s *Service) ResetUserTwoFactorByAdmin(ctx context.Context, userID uint) error {
	if err := s.repo.DeleteUserTwoFactor(ctx, userID); err != nil && !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	return nil
}

func (s *Service) StartCurrentTwoFactorSetup(ctx context.Context, userID uint) (*TwoFactorSetupStartResult, error) {
	item, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	current, err := s.repo.GetUserTwoFactorByUserID(ctx, userID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if current != nil && current.TOTPEnabled {
		return nil, fmt.Errorf("two factor authentication is already enabled")
	}
	if current != nil && strings.TrimSpace(current.TOTPSecretEncrypted) != "" &&
		current.TOTPSetupExpiresAt != nil && time.Now().Before(*current.TOTPSetupExpiresAt) {
		secret, decryptErr := secretbox.DecryptString(s.cfg.Snapshot().DataEncryptionKey, current.TOTPSecretEncrypted)
		if decryptErr != nil {
			return nil, decryptErr
		}
		return &TwoFactorSetupStartResult{
			Secret:     secret,
			OTPAuthURL: buildOTPAuthURL("DEEIX Chat", firstNonEmpty(item.Email, item.Username), secret),
			ExpiresAt:  *current.TOTPSetupExpiresAt,
		}, nil
	}
	expiresAt := time.Now().Add(twoFactorSetupTTL)
	secret, err := generateTOTPSecret()
	if err != nil {
		return nil, err
	}
	encrypted, err := secretbox.EncryptString(s.cfg.Snapshot().DataEncryptionKey, secret)
	if err != nil {
		return nil, err
	}
	if _, err = s.repo.UpsertUserTwoFactor(ctx, &user.UserTwoFactor{
		UserID:              userID,
		TOTPEnabled:         false,
		TOTPSecretEncrypted: encrypted,
		TOTPSetupExpiresAt:  &expiresAt,
		RecoveryCodesHash:   "",
	}); err != nil {
		return nil, err
	}
	return &TwoFactorSetupStartResult{
		Secret:     secret,
		OTPAuthURL: buildOTPAuthURL("DEEIX Chat", firstNonEmpty(item.Email, item.Username), secret),
		ExpiresAt:  expiresAt,
	}, nil
}

func (s *Service) ConfirmCurrentTwoFactorSetup(ctx context.Context, userID uint, code string) (*TwoFactorSetupConfirmResult, error) {
	twoFactor, err := s.repo.GetUserTwoFactorByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrTwoFactorSetupNotStarted
		}
		return nil, err
	}
	if twoFactor.TOTPEnabled {
		status, statusErr := s.GetCurrentTwoFactorStatus(ctx, userID)
		if statusErr != nil {
			return nil, statusErr
		}
		return &TwoFactorSetupConfirmResult{Status: *status}, nil
	}
	if strings.TrimSpace(twoFactor.TOTPSecretEncrypted) == "" {
		return nil, ErrTwoFactorSetupNotStarted
	}
	now := time.Now()
	if twoFactor.TOTPSetupExpiresAt == nil || now.After(*twoFactor.TOTPSetupExpiresAt) {
		_ = s.repo.DeleteUserTwoFactor(ctx, userID)
		return nil, ErrTwoFactorSetupExpired
	}
	secret, err := secretbox.DecryptString(s.cfg.Snapshot().DataEncryptionKey, twoFactor.TOTPSecretEncrypted)
	if err != nil {
		return nil, err
	}
	if !verifyTOTP(secret, code, now) {
		s.warn("two_factor_setup_invalid_code",
			zap.Uint("user_id", userID),
			zap.Int("code_length", len(normalizeTwoFactorCode(code))),
			zap.Int64("server_step", now.Unix()/totpStepSeconds),
			zap.String("secret_fingerprint", totpSecretFingerprint(secret)),
		)
		return nil, ErrInvalidCredentials
	}
	recoveryCodes, recoveryHash, err := generateRecoveryCodes(s.cfg.Snapshot().JWTSecret)
	if err != nil {
		return nil, err
	}
	totpEnabled := true
	var setupExpiresAt *time.Time
	enabledAt := &now
	lastVerifiedAt := &now
	if _, err = s.repo.UpdateUserTwoFactor(ctx, userID, repository.UpdateUserTwoFactorInput{
		TOTPEnabled:        &totpEnabled,
		TOTPSetupExpiresAt: &setupExpiresAt,
		RecoveryCodesHash:  &recoveryHash,
		EnabledAt:          &enabledAt,
		LastVerifiedAt:     &lastVerifiedAt,
	}); err != nil {
		return nil, err
	}
	status, err := s.GetCurrentTwoFactorStatus(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !status.TOTPEnabled {
		s.warn("two_factor_setup_not_persisted",
			zap.Uint("user_id", userID),
			zap.Int("recovery_count", status.RecoveryCount),
			zap.Bool("enabled_at_set", status.EnabledAt != nil),
		)
		return nil, ErrTwoFactorSetupNotPersisted
	}
	return &TwoFactorSetupConfirmResult{RecoveryCodes: recoveryCodes, Status: *status}, nil
}

func (s *Service) CancelCurrentTwoFactorSetup(ctx context.Context, userID uint) error {
	twoFactor, err := s.repo.GetUserTwoFactorByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		return err
	}
	if twoFactor.TOTPEnabled {
		return nil
	}
	return s.repo.DeleteUserTwoFactor(ctx, userID)
}

func (s *Service) DisableCurrentTwoFactor(ctx context.Context, userID uint, code string) error {
	if err := s.verifyCurrentTwoFactorCode(ctx, userID, code); err != nil {
		return err
	}
	if err := s.repo.DeleteUserTwoFactor(ctx, userID); err != nil && !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	return nil
}

func (s *Service) RegenerateCurrentTwoFactorRecoveryCodes(ctx context.Context, userID uint, code string) (*TwoFactorSetupConfirmResult, error) {
	if err := s.verifyCurrentTwoFactorCode(ctx, userID, code); err != nil {
		return nil, err
	}
	recoveryCodes, recoveryHash, err := generateRecoveryCodes(s.cfg.Snapshot().JWTSecret)
	if err != nil {
		return nil, err
	}
	if _, err = s.repo.UpdateUserTwoFactor(ctx, userID, repository.UpdateUserTwoFactorInput{
		RecoveryCodesHash: &recoveryHash,
	}); err != nil {
		return nil, err
	}
	status, err := s.GetCurrentTwoFactorStatus(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &TwoFactorSetupConfirmResult{RecoveryCodes: recoveryCodes, Status: *status}, nil
}

func (s *Service) verifyCurrentTwoFactorCode(ctx context.Context, userID uint, code string) error {
	twoFactor, err := s.repo.GetUserTwoFactorByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if !twoFactor.TOTPEnabled {
		return ErrInvalidCredentials
	}
	normalizedCode := normalizeTwoFactorCode(code)
	secret, err := secretbox.DecryptString(s.cfg.Snapshot().DataEncryptionKey, twoFactor.TOTPSecretEncrypted)
	if err != nil {
		return err
	}
	now := time.Now()
	if verifyTOTP(secret, normalizedCode, now) {
		lastVerifiedAt := &now
		_, err = s.repo.UpdateUserTwoFactor(ctx, userID, repository.UpdateUserTwoFactorInput{LastVerifiedAt: &lastVerifiedAt})
		return err
	}
	if s.consumeRecoveryCode(ctx, userID, twoFactor.RecoveryCodesHash, normalizedCode) {
		lastVerifiedAt := &now
		_, err = s.repo.UpdateUserTwoFactor(ctx, userID, repository.UpdateUserTwoFactorInput{LastVerifiedAt: &lastVerifiedAt})
		return err
	}
	return ErrInvalidCredentials
}

func (s *Service) consumeRecoveryCode(ctx context.Context, userID uint, rawRecords string, code string) bool {
	records := parseRecoveryCodeRecords(rawRecords)
	if len(records) == 0 || strings.TrimSpace(code) == "" {
		return false
	}
	hash := hashRecoveryCode(s.cfg.Snapshot().JWTSecret, code)
	next := make([]recoveryCodeRecord, 0, len(records))
	matched := false
	for _, record := range records {
		if subtle.ConstantTimeCompare([]byte(record.Hash), []byte(hash)) == 1 {
			matched = true
			continue
		}
		next = append(next, record)
	}
	if !matched {
		return false
	}
	payload, _ := json.Marshal(next)
	recoveryCodesHash := string(payload)
	expectedRecoveryHash := rawRecords
	_, err := s.repo.UpdateUserTwoFactor(ctx, userID, repository.UpdateUserTwoFactorInput{
		RecoveryCodesHash:    &recoveryCodesHash,
		ExpectedRecoveryHash: &expectedRecoveryHash,
	})
	return err == nil
}

func generateTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(raw), "="), nil
}

func verifyTOTP(secret string, rawCode string, now time.Time) bool {
	code := normalizeTwoFactorCode(rawCode)
	if len(code) != totpDigits {
		return false
	}
	counter := now.Unix() / totpStepSeconds
	for offset := -totpValidationWindow; offset <= totpValidationWindow; offset++ {
		expected, err := generateTOTPCode(secret, counter+offset)
		if err != nil {
			return false
		}
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

func generateTOTPCode(secret string, counter int64) (string, error) {
	normalizedSecret := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""))
	padding := len(normalizedSecret) % 8
	if padding > 0 {
		normalizedSecret += strings.Repeat("=", 8-padding)
	}
	key, err := base32.StdEncoding.DecodeString(normalizedSecret)
	if err != nil {
		return "", err
	}
	msg := make([]byte, 8)
	binary.BigEndian.PutUint64(msg, uint64(counter))
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(msg)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	binaryCode := (uint32(sum[offset])&0x7f)<<24 |
		(uint32(sum[offset+1])&0xff)<<16 |
		(uint32(sum[offset+2])&0xff)<<8 |
		(uint32(sum[offset+3]) & 0xff)
	mod := uint32(math.Pow10(totpDigits))
	return fmt.Sprintf("%0"+strconv.Itoa(totpDigits)+"d", binaryCode%mod), nil
}

func normalizeTwoFactorCode(value string) string {
	var builder strings.Builder
	for _, char := range strings.TrimSpace(value) {
		switch {
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char - ('a' - 'A'))
		case char >= 'A' && char <= 'Z':
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func generateRecoveryCodes(secret string) ([]string, string, error) {
	codes := make([]string, 10)
	records := make([]recoveryCodeRecord, 10)
	for i := range codes {
		raw := make([]byte, 9)
		if _, err := rand.Read(raw); err != nil {
			return nil, "", err
		}
		encoded := strings.ToUpper(base64.RawStdEncoding.EncodeToString(raw))
		code := fmt.Sprintf("%s-%s-%s", encoded[:4], encoded[4:8], encoded[8:12])
		codes[i] = code
		records[i] = recoveryCodeRecord{Hash: hashRecoveryCode(secret, code)}
	}
	payload, err := json.Marshal(records)
	if err != nil {
		return nil, "", err
	}
	return codes, string(payload), nil
}

func parseRecoveryCodeRecords(raw string) []recoveryCodeRecord {
	records := make([]recoveryCodeRecord, 0)
	if strings.TrimSpace(raw) == "" {
		return records
	}
	if err := json.Unmarshal([]byte(raw), &records); err != nil {
		return nil
	}
	return records
}

func hashRecoveryCode(secret string, code string) string {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	_, _ = mac.Write([]byte(normalizeTwoFactorCode(code)))
	return hex.EncodeToString(mac.Sum(nil))
}

func totpSecretFingerprint(secret string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(sum[:8])
}

func buildOTPAuthURL(issuer string, account string, secret string) string {
	issuer = strings.TrimSpace(issuer)
	account = strings.TrimSpace(account)
	label := url.PathEscape(issuer)
	if account != "" {
		label += ":" + url.PathEscape(account)
	}
	query := url.Values{}
	query.Set("secret", strings.TrimSpace(secret))
	query.Set("issuer", issuer)
	query.Set("algorithm", "SHA1")
	query.Set("digits", strconv.Itoa(totpDigits))
	query.Set("period", strconv.FormatInt(totpStepSeconds, 10))
	return "otpauth://totp/" + label + "?" + query.Encode()
}
