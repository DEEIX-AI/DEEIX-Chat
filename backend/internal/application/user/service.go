package user

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/url"
	"path/filepath"
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
	repo                repository.UserRepository
	avatarContentOpener avatarContentOpener
	avatarFileValidator avatarFileValidator
}

type avatarContentOpener interface {
	OpenAvatarFileContent(ctx context.Context, userID uint, fileID string) (*AvatarFileContent, error)
}

type avatarFileValidator interface {
	ValidateImageFile(ctx context.Context, userID uint, fileID string) error
}

const (
	defaultPageSize = 20
	maxPageSize     = 1000
)

// NewService 创建服务。
func NewService(repo repository.UserRepository) *Service {
	return &Service{repo: repo}
}

// SetAvatarContentOpener 注入头像文件内容读取能力。
func (s *Service) SetAvatarContentOpener(opener avatarContentOpener) {
	s.avatarContentOpener = opener
}

// SetAvatarFileValidator 注入头像文件校验能力。
func (s *Service) SetAvatarFileValidator(validator avatarFileValidator) {
	s.avatarFileValidator = validator
}

// AvatarFileContent 描述用户域读取到的头像源文件内容。
type AvatarFileContent struct {
	Reader      io.ReadCloser
	ContentType string
	SizeBytes   int64
	ModTime     time.Time
	FileName    string
}

// AvatarContentResult 描述当前头像内容读取结果。
type AvatarContentResult struct {
	Reader      io.ReadCloser
	ContentType string
	SizeBytes   int64
	ModTime     time.Time
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

// GetByPublicID 按公开 ID 查询用户详情。
func (s *Service) GetByPublicID(ctx context.Context, publicID string) (*domainuser.User, error) {
	normalizedPublicID := strings.TrimSpace(publicID)
	if normalizedPublicID == "" {
		return nil, ErrUserNotFound
	}
	item, err := s.repo.GetByPublicID(ctx, normalizedPublicID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return item, nil
}

// OpenAvatarContent 打开用户当前上传头像内容。
func (s *Service) OpenAvatarContent(ctx context.Context, publicID string) (*AvatarContentResult, error) {
	item, err := s.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	fileID, ok := domainuser.ParseFileAvatarURL(item.AvatarURL)
	if !ok || s.avatarContentOpener == nil {
		return nil, ErrAvatarNotFound
	}

	content, err := s.avatarContentOpener.OpenAvatarFileContent(ctx, item.ID, fileID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrAvatarNotFound
		}
		return nil, err
	}
	contentType := strings.TrimSpace(content.ContentType)
	if contentType == "" {
		contentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(content.FileName)))
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		_ = content.Reader.Close()
		return nil, ErrAvatarNotFound
	}
	return &AvatarContentResult{
		Reader:      content.Reader,
		ContentType: contentType,
		SizeBytes:   content.SizeBytes,
		ModTime:     content.ModTime,
	}, nil
}

// ListUsers 分页查询用户列表。
func (s *Service) ListUsers(ctx context.Context, page int, pageSize int, filter repository.UserListFilter) ([]domainuser.User, int64, error) {
	offset, limit := normalizePage(page, pageSize)
	return s.repo.ListUsers(ctx, offset, limit, filter)
}

// ListLatestSessionActivityByUserIDs 批量查询用户最近会话活跃时间。
func (s *Service) ListLatestSessionActivityByUserIDs(ctx context.Context, userIDs []uint) (map[uint]time.Time, error) {
	if len(userIDs) == 0 {
		return map[uint]time.Time{}, nil
	}
	return s.repo.ListLatestSessionActivityByUserIDs(ctx, userIDs)
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

// ListUsersByLowerEmails 按小写邮箱批量查询用户。
func (s *Service) ListUsersByLowerEmails(ctx context.Context, emails []string) (map[string]domainuser.User, error) {
	return s.repo.ListUsersByLowerEmails(ctx, emails)
}

// ListAllUsernames 查询当前全部用户名。
func (s *Service) ListAllUsernames(ctx context.Context) ([]string, error) {
	return s.repo.ListAllUsernames(ctx)
}

// ImportUsersWithCredentialsAndBalances 批量导入用户、凭据与初始余额。
func (s *Service) ImportUsersWithCredentialsAndBalances(ctx context.Context, records []repository.UserImportRecord) ([]domainuser.User, error) {
	return s.repo.ImportUsersWithCredentialsAndBalances(ctx, records)
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
	if _, ok := domainuser.ParseFileAvatarURL(normalizedAvatarURL); ok {
		return nil, ErrInvalidAvatarURL
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
	avatarFileReferenceRequested := false
	if input.AvatarURL != nil {
		normalizedAvatarURL := strings.TrimSpace(*input.AvatarURL)
		if err := validateAvatarURL(normalizedAvatarURL); err != nil {
			return nil, err
		}
		if fileID, ok := domainuser.ParseFileAvatarURL(normalizedAvatarURL); ok {
			avatarFileReferenceRequested = true
			if s.avatarFileValidator == nil {
				return nil, ErrInvalidAvatarURL
			}
			if err := s.avatarFileValidator.ValidateImageFile(ctx, userID, fileID); err != nil {
				return nil, ErrInvalidAvatarURL
			}
		}
		input.AvatarURL = &normalizedAvatarURL
	}
	item, err := s.repo.UpdateFields(ctx, userID, input)
	if err != nil {
		if avatarFileReferenceRequested && errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidAvatarURL
		}
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
	if strings.HasPrefix(raw, "file:") {
		if _, ok := domainuser.ParseFileAvatarURL(raw); !ok {
			return ErrInvalidAvatarURL
		}
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
