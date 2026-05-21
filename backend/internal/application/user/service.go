package user

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const passwordHashCost = 12

// Service 封装用户业务能力。
type Service struct {
	repo repository.UserRepository
}

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// NewService 创建服务。
func NewService(repo repository.UserRepository) *Service {
	return &Service{repo: repo}
}

// GetByID 查询用户详情。
func (s *Service) GetByID(ctx context.Context, userID uint) (*domainuser.User, error) {
	item, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return item, nil
}

// ListUsers 分页查询用户列表。
func (s *Service) ListUsers(ctx context.Context, page int, pageSize int) ([]domainuser.User, int64, error) {
	offset, limit := normalizePage(page, pageSize)
	return s.repo.ListUsers(ctx, offset, limit)
}

func normalizePage(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	return offset, pageSize
}

// CountSuperAdmins 统计超级管理员数量。
func (s *Service) CountSuperAdmins(ctx context.Context) (int64, error) {
	return s.repo.CountSuperAdmins(ctx)
}

// CreateUser 创建普通用户账号。
func (s *Service) CreateUser(
	ctx context.Context,
	username string,
	password string,
	avatarURL string,
	displayName string,
	email string,
	phone string,
	timezone string,
	locale string,
	billingMode string,
	subscriptionTier string,
	subscriptionExpiresAt *time.Time,
) (*domainuser.User, error) {
	normalizedUsername, err := NormalizeUsername(username)
	if err != nil {
		return nil, err
	}
	_, err = s.repo.GetByUsername(ctx, normalizedUsername)
	if err == nil {
		return nil, ErrUsernameTaken
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	normalizedPassword, err := NormalizePassword(password)
	if err != nil {
		return nil, err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(normalizedPassword), passwordHashCost)
	if err != nil {
		return nil, err
	}
	now := time.Now()

	normalizedAvatarURL := strings.TrimSpace(avatarURL)
	if err = validateAvatarURL(normalizedAvatarURL); err != nil {
		return nil, err
	}

	normalizedDisplayName := strings.TrimSpace(displayName)
	if normalizedDisplayName == "" {
		normalizedDisplayName = normalizedUsername
	}
	normalizedDisplayName, err = NormalizeDisplayName(normalizedDisplayName)
	if err != nil {
		return nil, err
	}

	normalizedEmail, err := NormalizeEmail(email)
	if err != nil {
		return nil, err
	}

	normalizedPhone, err := NormalizePhone(phone)
	if err != nil {
		return nil, err
	}

	normalizedTimezone := strings.TrimSpace(timezone)
	if normalizedTimezone == "" {
		normalizedTimezone = "Etc/UTC"
	}
	if _, err = time.LoadLocation(normalizedTimezone); err != nil {
		return nil, ErrInvalidTimeZone
	}

	normalizedLocale, err := normalizeLocale(locale)
	if err != nil {
		return nil, err
	}

	normalizedBillingMode := strings.ToLower(strings.TrimSpace(billingMode))
	var subscriptionPlanID uint
	var subscriptionPriceID uint
	var normalizedSubscriptionEndAt *time.Time
	autoRenew := false
	if normalizedBillingMode == "period" {
		normalizedSubscriptionTier := strings.ToLower(strings.TrimSpace(subscriptionTier))
		if normalizedSubscriptionTier == "" {
			normalizedSubscriptionTier = defaultFreePlanCode
		}

		plan, planErr := s.repo.GetActivePlanByCode(ctx, normalizedSubscriptionTier)
		if planErr != nil {
			if errors.Is(planErr, repository.ErrNotFound) {
				return nil, ErrInvalidSubscriptionTier
			}
			return nil, planErr
		}

		price, priceErr := s.repo.GetActiveDefaultPriceByPlanID(ctx, plan.ID)
		if priceErr != nil {
			if errors.Is(priceErr, repository.ErrNotFound) {
				return nil, ErrInvalidSubscriptionTier
			}
			return nil, priceErr
		}

		subscriptionPlanID = plan.ID
		subscriptionPriceID = price.ID
		if plan.Code != defaultFreePlanCode {
			if subscriptionExpiresAt == nil {
				return nil, ErrSubscriptionExpiryRequired
			}
			expiresAt := subscriptionExpiresAt.UTC()
			if !expiresAt.After(time.Now().UTC()) {
				return nil, ErrInvalidSubscriptionExpiry
			}
			normalizedSubscriptionEndAt = &expiresAt
		} else if price.BillingInterval != domainbilling.IntervalLifetime {
			autoRenew = true
		}
	}

	item := &domainuser.User{
		PublicID:    normalizePublicID(uuid.NewString()),
		Username:    normalizedUsername,
		DisplayName: normalizedDisplayName,
		AvatarURL:   normalizedAvatarURL,
		Email:       normalizedEmail,
		EmailSource: domainuser.EmailSourceAdminSet,
		Phone:       normalizedPhone,
		Role:        domainuser.RoleUser,
		Status:      domainuser.StatusActive,
		Timezone:    normalizedTimezone,
		Locale:      normalizedLocale,
	}

	if err = s.repo.CreateWithCredential(
		ctx,
		item,
		domainuser.Credential{
			PasswordHash:      string(passwordHash),
			PasswordAlgo:      "bcrypt",
			PasswordEnabled:   true,
			PasswordUpdatedAt: &now,
			PasswordSetAt:     &now,
			PasswordOrigin:    domainuser.PasswordOriginAdminCreated,
		},
		subscriptionPlanID,
		subscriptionPriceID,
		normalizedSubscriptionEndAt,
		autoRenew,
	); err != nil {
		return nil, err
	}
	return item, nil
}

// RevokeAllSessions 吊销指定用户的全部会话。
func (s *Service) RevokeAllSessions(ctx context.Context, userID uint, reason string) error {
	return s.repo.RevokeAllSessions(ctx, userID, reason)
}

// UpdateUserStatus 更新用户状态。
func (s *Service) UpdateUserStatus(ctx context.Context, userID uint, status string) error {
	return s.repo.UpdateUserStatus(ctx, userID, status)
}

// UpdateFields 更新用户字段。
func (s *Service) UpdateFields(ctx context.Context, userID uint, input repository.UpdateUserFieldsInput) (*domainuser.User, error) {
	item, err := s.repo.UpdateFields(ctx, userID, input)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return item, nil
}

// ResetLoginFailure 重置用户登录失败计数和锁定信息。
func (s *Service) ResetLoginFailure(ctx context.Context, userID uint) error {
	return s.repo.ResetLoginFailure(ctx, userID)
}

// ResetPasswordByAdmin 管理员重置用户密码。
func (s *Service) ResetPasswordByAdmin(ctx context.Context, userID uint, newPassword string, mustResetPassword bool) error {
	normalizedPassword, err := NormalizePassword(newPassword)
	if err != nil {
		return err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(normalizedPassword), passwordHashCost)
	if err != nil {
		return err
	}

	if err = s.repo.ResetPasswordByAdmin(ctx, userID, string(passwordHash), mustResetPassword); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	return nil
}

// DeleteAccountHard 删除用户主记录及主要用户域数据。
func (s *Service) DeleteAccountHard(ctx context.Context, userID uint) error {
	if err := s.repo.DeleteAccountHard(ctx, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	return nil
}

// ListAuthEvents 查询认证事件列表。
func (s *Service) ListAuthEvents(
	ctx context.Context,
	userID uint,
	eventType string,
	result string,
	page int,
	pageSize int,
) ([]domainuser.AuthEvent, int64, error) {
	offset, limit := normalizePage(page, pageSize)

	return s.repo.ListAuthEvents(
		ctx,
		userID,
		strings.TrimSpace(eventType),
		strings.TrimSpace(result),
		offset,
		limit,
	)
}

// RecordAuthEvent 写入认证事件。
func (s *Service) RecordAuthEvent(
	ctx context.Context,
	userID uint,
	requestID string,
	eventType string,
	result string,
	reason string,
	clientIP string,
	userAgent string,
	detailJSON string,
) error {
	return s.repo.RecordAuthEvent(
		ctx,
		userID,
		requestID,
		eventType,
		result,
		reason,
		clientIP,
		userAgent,
		detailJSON,
	)
}

func normalizePublicID(raw string) string {
	return strings.ReplaceAll(raw, "-", "")
}

func validateAvatarURL(raw string) error {
	if raw == "" || strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "generated:github:") {
		return nil
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return ErrInvalidAvatarURL
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ErrInvalidAvatarURL
	}
	return nil
}

func normalizeLocale(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "en-US", nil
	}

	normalized := strings.ReplaceAll(trimmed, "_", "-")
	parts := strings.Split(normalized, "-")
	if len(parts) == 0 || len(parts) > 2 {
		return "", ErrInvalidLocale
	}

	languagePart := strings.ToLower(parts[0])
	if len(languagePart) < 2 || len(languagePart) > 3 || !isAlpha(languagePart) {
		return "", ErrInvalidLocale
	}

	if len(parts) == 1 {
		return languagePart, nil
	}

	regionPart := strings.ToUpper(parts[1])
	if len(regionPart) != 2 || !isAlpha(regionPart) {
		return "", ErrInvalidLocale
	}

	return languagePart + "-" + regionPart, nil
}

func isAlpha(value string) bool {
	for _, r := range value {
		if !unicode.IsLetter(r) || r > unicode.MaxASCII {
			return false
		}
	}
	return true
}
