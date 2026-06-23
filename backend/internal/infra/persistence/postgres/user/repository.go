package user

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var userListSearchEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// translateError 将 gorm 底层错误统一映射为仓储语义错误。
func translateError(err error) error {
	if err == nil {
		return nil
	}
	if dberror.IsRecordNotFound(err) {
		return repository.ErrNotFound
	}
	if dberror.IsUniqueConstraint(err) {
		return translateUniqueConstraint(err)
	}
	return err
}

func translateUniqueConstraint(err error) error {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "idx_identity_users_username"):
		return repository.ErrDuplicateUsername
	case strings.Contains(msg, "uk_identity_user_links_provider_subject"):
		return repository.ErrDuplicateUserIdentity
	default:
		return repository.ErrDuplicate
	}
}

const defaultFreePlanCode = "free"

// Repo 封装用户数据访问。
type Repo struct {
	db *gorm.DB
}

// NewRepo 创建仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// GetByUsername 按用户名查询用户。
func (r *Repo) GetByUsername(ctx context.Context, username string) (*domainuser.User, error) {
	var item model.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomainUser(item), nil
}

// GetByEmail 按邮箱查询用户。
func (r *Repo) GetByEmail(ctx context.Context, email string) (*domainuser.User, error) {
	var item model.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomainUser(item), nil
}

// GetByPublicID 按公开 ID 查询用户。
func (r *Repo) GetByPublicID(ctx context.Context, publicID string) (*domainuser.User, error) {
	var item model.User
	if err := r.db.WithContext(ctx).Where("public_id = ?", publicID).First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomainUser(item), nil
}

// ListUsersByLowerEmails 按小写邮箱批量查询用户。
func (r *Repo) ListUsersByLowerEmails(ctx context.Context, emails []string) (map[string]domainuser.User, error) {
	results := make(map[string]domainuser.User)
	normalized := make([]string, 0, len(emails))
	seen := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		value := strings.ToLower(strings.TrimSpace(email))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	if len(normalized) == 0 {
		return results, nil
	}

	items := make([]model.User, 0)
	if err := r.db.WithContext(ctx).
		Where("LOWER(email) IN ?", normalized).
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	for _, item := range items {
		results[strings.ToLower(strings.TrimSpace(item.Email))] = *toDomainUser(item)
	}
	return results, nil
}

// ListAllUsernames 查询当前全部用户名，用于导入时规避唯一约束冲突。
func (r *Repo) ListAllUsernames(ctx context.Context) ([]string, error) {
	var usernames []string
	if err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Pluck("username", &usernames).Error; err != nil {
		return nil, translateError(err)
	}
	return usernames, nil
}

// GetByID 按 ID 查询用户。
func (r *Repo) GetByID(ctx context.Context, userID uint) (*domainuser.User, error) {
	var item model.User
	if err := r.db.WithContext(ctx).Where("id = ?", userID).First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomainUser(item), nil
}

// UpdateProfile 更新用户资料并返回最新结果。
func (r *Repo) UpdateProfile(ctx context.Context, userID uint, input repository.UpdateUserFieldsInput) (*domainuser.User, error) {
	return r.updateUserFields(ctx, userID, input)
}

// UpdateUsernameOnce 修改用户登录名，仅允许用户自主修改一次。
func (r *Repo) UpdateUsernameOnce(ctx context.Context, userID uint, username string, changedAt time.Time) (*domainuser.User, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userID).First(&current).Error; err != nil {
			return translateError(err)
		}
		if current.UsernameChangedAt != nil {
			return repository.ErrConflict
		}
		if current.Username == username {
			return translateError(tx.Model(&model.User{}).
				Where("id = ?", userID).
				Update("username_changed_at", changedAt).
				Error)
		}

		var existing model.User
		err := tx.Where("LOWER(username) = ? AND id <> ?", username, userID).First(&existing).Error
		if err == nil {
			return repository.ErrDuplicateUsername
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return translateError(err)
		}

		return translateError(tx.Model(&model.User{}).
			Where("id = ?", userID).
			Updates(map[string]interface{}{
				"username":            username,
				"username_changed_at": changedAt,
			}).
			Error)
	})
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, userID)
}

// UpdateFields 更新用户字段并返回最新结果。
func (r *Repo) UpdateFields(ctx context.Context, userID uint, input repository.UpdateUserFieldsInput) (*domainuser.User, error) {
	return r.updateUserFields(ctx, userID, input)
}

func (r *Repo) updateUserFields(ctx context.Context, userID uint, input repository.UpdateUserFieldsInput) (*domainuser.User, error) {
	updates := userFieldUpdates(input)
	if len(updates) == 0 {
		return r.GetByID(ctx, userID)
	}

	if input.Role != nil && *input.Role != model.RoleSuperAdmin {
		return r.updateUserFieldsInTransaction(ctx, userID, updates, true)
	}

	return r.updateUserFieldsInTransaction(ctx, userID, updates, false)
}

func (r *Repo) updateUserFieldsInTransaction(
	ctx context.Context,
	userID uint,
	updates map[string]interface{},
	withSuperAdminGuard bool,
) (*domainuser.User, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if withSuperAdminGuard {
			var superAdmins []model.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Select("id").
				Where("role = ?", model.RoleSuperAdmin).
				Order("id ASC").
				Find(&superAdmins).Error; err != nil {
				return translateError(err)
			}

			targetIsSuperAdmin := false
			for _, item := range superAdmins {
				if item.ID == userID {
					targetIsSuperAdmin = true
					break
				}
			}
			if targetIsSuperAdmin && len(superAdmins) <= 1 {
				return repository.ErrLastSuperAdminRoleChange
			}
		}

		if rawAvatarURL, ok := updates["avatar_url"].(string); ok {
			if fileID, isFileAvatar := domainuser.ParseFileAvatarURL(rawAvatarURL); isFileAvatar {
				var fileObject model.FileObject
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
					Select("id").
					Where("user_id = ? AND file_id = ? AND status = ?", userID, fileID, "active").
					First(&fileObject).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return repository.ErrNotFound
					}
					return translateError(err)
				}
			}
		}

		result := tx.Model(&model.User{}).
			Where("id = ?", userID).
			Updates(updates)
		if result.Error != nil {
			return translateError(result.Error)
		}
		if result.RowsAffected == 0 {
			return repository.ErrNotFound
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, userID)
}

func userFieldUpdates(input repository.UpdateUserFieldsInput) map[string]interface{} {
	updates := make(map[string]interface{})
	if input.AvatarURL != nil {
		updates["avatar_url"] = *input.AvatarURL
	}
	if input.DisplayName != nil {
		updates["display_name"] = *input.DisplayName
	}
	if input.Email != nil {
		updates["email"] = *input.Email
	}
	if input.EmailVerifiedAt != nil {
		updates["email_verified_at"] = *input.EmailVerifiedAt
	}
	if input.EmailSource != nil {
		updates["email_source"] = *input.EmailSource
	}
	if input.EmailBootstrapUsedAt != nil {
		updates["email_bootstrap_used_at"] = *input.EmailBootstrapUsedAt
	}
	if input.Phone != nil {
		updates["phone"] = *input.Phone
	}
	if input.PhoneVerifiedAt != nil {
		updates["phone_verified_at"] = *input.PhoneVerifiedAt
	}
	if input.Role != nil {
		updates["role"] = *input.Role
	}
	if input.Timezone != nil {
		updates["timezone"] = *input.Timezone
	}
	if input.Locale != nil {
		updates["locale"] = *input.Locale
	}
	if input.ProfilePreferences != nil {
		updates["profile_preferences"] = *input.ProfilePreferences
	}
	if input.AppearancePreferences != nil {
		updates["appearance_preferences"] = *input.AppearancePreferences
	}
	if input.OnboardingCompletedAt != nil {
		updates["onboarding_completed_at"] = *input.OnboardingCompletedAt
	}
	return updates
}

// ListUsers 分页查询用户。
func (r *Repo) ListUsers(ctx context.Context, offset int, limit int, filter repository.UserListFilter) ([]domainuser.User, int64, error) {
	items := make([]model.User, 0)
	var total int64

	query := r.db.WithContext(ctx).Model(&model.User{})
	if keyword := strings.TrimSpace(filter.Query); keyword != "" {
		like := "%" + userListSearchEscaper.Replace(strings.ToLower(keyword)) + "%"
		query = query.Where(
			"LOWER(username) LIKE ? ESCAPE '\\' OR LOWER(display_name) LIKE ? ESCAPE '\\' OR LOWER(email) LIKE ? ESCAPE '\\' OR LOWER(public_id) LIKE ? ESCAPE '\\'",
			like,
			like,
			like,
			like,
		)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	if err := query.
		Order("id DESC").
		Offset(offset).
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}
	return toDomainUsers(items), total, nil
}

// CountSuperAdmins 统计超级管理员数量。
func (r *Repo) CountSuperAdmins(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("role = ?", model.RoleSuperAdmin).
		Count(&count).Error; err != nil {
		return 0, translateError(err)
	}
	return count, nil
}

// GetActivePlanByCode 按编码查询启用套餐。
func (r *Repo) GetActivePlanByCode(ctx context.Context, code string) (*domainbilling.Plan, error) {
	var item model.BillingPlan
	if err := r.db.WithContext(ctx).
		Where("code = ? AND is_active = ?", code, true).
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomainPlan(item), nil
}

// GetActiveDefaultPriceByPlanID 查询套餐默认启用价格。
func (r *Repo) GetActiveDefaultPriceByPlanID(ctx context.Context, planID uint) (*domainbilling.Price, error) {
	var item model.BillingPrice
	if err := r.db.WithContext(ctx).
		Where("plan_id = ? AND is_active = ? AND is_default = ?", planID, true, true).
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomainPrice(item), nil
}

// CreateWithCredential 在同一事务中创建用户与凭据。
func (r *Repo) CreateWithCredential(
	ctx context.Context,
	user *domainuser.User,
	credential domainuser.Credential,
	subscriptionPlanID uint,
	subscriptionPriceID uint,
	subscriptionEndAt *time.Time,
	autoRenew bool,
) error {
	return translateError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return r.createWithCredentialTx(tx, user, credential, subscriptionPlanID, subscriptionPriceID, subscriptionEndAt, autoRenew)
	}))
}

// CreateWithCredentialAndIdentity 在同一事务中创建用户、凭据与第三方身份。
func (r *Repo) CreateWithCredentialAndIdentity(
	ctx context.Context,
	user *domainuser.User,
	credential domainuser.Credential,
	identity *domainuser.UserIdentity,
	subscriptionPlanID uint,
	subscriptionPriceID uint,
	subscriptionEndAt *time.Time,
	autoRenew bool,
) error {
	return translateError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.createWithCredentialTx(tx, user, credential, subscriptionPlanID, subscriptionPriceID, subscriptionEndAt, autoRenew); err != nil {
			return err
		}
		if identity == nil {
			return nil
		}
		identity.UserID = user.ID
		dbIdentity := toModelUserIdentity(identity)
		if err := tx.Create(dbIdentity).Error; err != nil {
			return translateError(err)
		}
		identity.ID = dbIdentity.ID
		identity.CreatedAt = dbIdentity.CreatedAt
		identity.UpdatedAt = dbIdentity.UpdatedAt
		return nil
	}))
}

// ImportUsersWithCredentialsAndBalances 在同一事务中导入用户、凭据与初始余额账户。
func (r *Repo) ImportUsersWithCredentialsAndBalances(ctx context.Context, records []repository.UserImportRecord) ([]domainuser.User, error) {
	results := make([]domainuser.User, 0, len(records))
	if len(records) == 0 {
		return results, nil
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, record := range records {
			dbUser := toModelUser(&record.User)
			if err := tx.Create(dbUser).Error; err != nil {
				return translateError(err)
			}

			now := time.Now()
			passwordAlgo := record.Credential.PasswordAlgo
			if passwordAlgo == "" {
				passwordAlgo = "bcrypt"
			}
			passwordOrigin := record.Credential.PasswordOrigin
			if passwordOrigin == "" {
				passwordOrigin = domainuser.PasswordOriginAdminCreated
			}
			passwordUpdatedAt := record.Credential.PasswordUpdatedAt
			if passwordUpdatedAt == nil {
				passwordUpdatedAt = &now
			}
			passwordSetAt := record.Credential.PasswordSetAt
			if passwordSetAt == nil {
				passwordSetAt = &now
			}

			dbCredential := &model.UserCredential{
				UserID:            dbUser.ID,
				PasswordHash:      record.Credential.PasswordHash,
				PasswordAlgo:      passwordAlgo,
				PasswordEnabled:   record.Credential.PasswordEnabled,
				PasswordUpdatedAt: passwordUpdatedAt,
				PasswordSetAt:     passwordSetAt,
				PasswordOrigin:    passwordOrigin,
				MustResetPassword: record.Credential.MustResetPassword,
				FailedLoginCount:  record.Credential.FailedLoginCount,
			}
			if err := tx.Create(dbCredential).Error; err != nil {
				return translateError(err)
			}

			balanceNanousd := record.BillingBalanceNanousd
			if balanceNanousd < 0 {
				balanceNanousd = 0
			}
			account := &model.BillingAccount{
				UserID:         dbUser.ID,
				Currency:       "USD",
				BalanceNanousd: balanceNanousd,
				Status:         "active",
			}
			if err := tx.Create(account).Error; err != nil {
				return translateError(err)
			}
			if balanceNanousd > 0 {
				transaction := &model.BalanceTransaction{
					AccountID:           account.ID,
					UserID:              dbUser.ID,
					Type:                domainbilling.BalanceTransactionTypeAdminSet,
					AmountNanousd:       balanceNanousd,
					BalanceAfterNanousd: balanceNanousd,
					RefType:             "admin_import",
					RefNo:               strings.TrimSpace(record.BillingBalanceRefNo),
					Description:         strings.TrimSpace(record.BillingBalanceDescription),
				}
				if transaction.Description == "" {
					transaction.Description = "OpenWebUI import"
				}
				if err := tx.Create(transaction).Error; err != nil {
					return translateError(err)
				}
			}

			record.User.ID = dbUser.ID
			record.User.CreatedAt = dbUser.CreatedAt
			record.User.UpdatedAt = dbUser.UpdatedAt
			results = append(results, record.User)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (r *Repo) createWithCredentialTx(
	tx *gorm.DB,
	user *domainuser.User,
	credential domainuser.Credential,
	subscriptionPlanID uint,
	subscriptionPriceID uint,
	subscriptionEndAt *time.Time,
	autoRenew bool,
) error {
	dbUser := toModelUser(user)
	if err := tx.Create(dbUser).Error; err != nil {
		return translateError(err)
	}
	user.ID = dbUser.ID
	user.CreatedAt = dbUser.CreatedAt
	user.UpdatedAt = dbUser.UpdatedAt
	passwordAlgo := credential.PasswordAlgo
	if passwordAlgo == "" {
		passwordAlgo = "bcrypt"
	}
	passwordOrigin := credential.PasswordOrigin
	if passwordOrigin == "" {
		passwordOrigin = domainuser.PasswordOriginLocalRegister
	}

	dbCredential := &model.UserCredential{
		UserID:            dbUser.ID,
		PasswordHash:      credential.PasswordHash,
		PasswordAlgo:      passwordAlgo,
		PasswordEnabled:   credential.PasswordEnabled,
		PasswordUpdatedAt: credential.PasswordUpdatedAt,
		PasswordSetAt:     credential.PasswordSetAt,
		PasswordOrigin:    passwordOrigin,
		MustResetPassword: credential.MustResetPassword,
		FailedLoginCount:  credential.FailedLoginCount,
	}
	if err := tx.Create(dbCredential).Error; err != nil {
		return translateError(err)
	}

	if user.Role == domainuser.RoleUser && subscriptionPlanID > 0 && subscriptionPriceID > 0 {
		now := time.Now()
		subscription := &model.Subscription{
			UserID:               dbUser.ID,
			PlanID:               subscriptionPlanID,
			PriceID:              subscriptionPriceID,
			Status:               "active",
			StartAt:              now,
			CurrentPeriodStartAt: now,
			CurrentPeriodEndAt:   subscriptionEndAt,
			CancelAtPeriodEnd:    false,
			CanceledAt:           nil,
			AutoRenew:            autoRenew,
		}
		if err := tx.Create(subscription).Error; err != nil {
			return translateError(err)
		}
	}

	return nil
}

// GetCredentialByUserID 查询用户凭据。
func (r *Repo) GetCredentialByUserID(ctx context.Context, userID uint) (*domainuser.Credential, error) {
	var credential model.UserCredential
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&credential).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomainCredential(credential), nil
}

func (r *Repo) GetUserTwoFactorByUserID(ctx context.Context, userID uint) (*domainuser.UserTwoFactor, error) {
	var item model.UserTwoFactor
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomainUserTwoFactor(item), nil
}

func (r *Repo) UpsertUserTwoFactor(ctx context.Context, item *domainuser.UserTwoFactor) (*domainuser.UserTwoFactor, error) {
	dbItem := toModelUserTwoFactor(item)
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"totp_enabled":              dbItem.TOTPEnabled,
				"totp_secret_encrypted":     dbItem.TOTPSecretEncrypted,
				"totp_setup_expires_at":     dbItem.TOTPSetupExpiresAt,
				"recovery_codes_hash":       dbItem.RecoveryCodesHash,
				"enforced":                  dbItem.Enforced,
				"enabled_at":                dbItem.EnabledAt,
				"last_verified_at":          dbItem.LastVerifiedAt,
				"trusted_device_expires_at": dbItem.TrustedDeviceExpiresAt,
			}),
		}).
		Create(dbItem).Error
	if err != nil {
		return nil, translateError(err)
	}
	return r.GetUserTwoFactorByUserID(ctx, item.UserID)
}

func (r *Repo) UpdateUserTwoFactor(ctx context.Context, userID uint, input repository.UpdateUserTwoFactorInput) (*domainuser.UserTwoFactor, error) {
	updates := userTwoFactorUpdates(input)
	if len(updates) == 0 {
		return r.GetUserTwoFactorByUserID(ctx, userID)
	}
	result := r.db.WithContext(ctx).
		Model(&model.UserTwoFactor{}).
		Where("user_id = ?", userID)
	if input.ExpectedRecoveryHash != nil {
		result = result.Where("recovery_codes_hash = ?", *input.ExpectedRecoveryHash)
	}
	result = result.Updates(updates)
	if result.Error != nil {
		return nil, translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, repository.ErrNotFound
	}
	return r.GetUserTwoFactorByUserID(ctx, userID)
}

func userTwoFactorUpdates(input repository.UpdateUserTwoFactorInput) map[string]interface{} {
	updates := make(map[string]interface{})
	if input.TOTPEnabled != nil {
		updates["totp_enabled"] = *input.TOTPEnabled
	}
	if input.TOTPSetupExpiresAt != nil {
		updates["totp_setup_expires_at"] = *input.TOTPSetupExpiresAt
	}
	if input.RecoveryCodesHash != nil {
		updates["recovery_codes_hash"] = *input.RecoveryCodesHash
	}
	if input.Enforced != nil {
		updates["enforced"] = *input.Enforced
	}
	if input.EnabledAt != nil {
		updates["enabled_at"] = *input.EnabledAt
	}
	if input.LastVerifiedAt != nil {
		updates["last_verified_at"] = *input.LastVerifiedAt
	}
	if input.TrustedDeviceExpiresAt != nil {
		updates["trusted_device_expires_at"] = *input.TrustedDeviceExpiresAt
	}
	return updates
}

func (r *Repo) DeleteUserTwoFactor(ctx context.Context, userID uint) error {
	result := r.db.WithContext(ctx).
		Unscoped().
		Where("user_id = ?", userID).
		Delete(&model.UserTwoFactor{})
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// MarkLoginFailure 标记一次登录失败并按阈值写入锁定时间。
func (r *Repo) MarkLoginFailure(
	ctx context.Context,
	userID uint,
	lockThreshold int,
	lockUntil time.Time,
) (*domainuser.Credential, error) {
	var updated model.UserCredential

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var credential model.UserCredential
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).
			First(&credential).Error; err != nil {
			return err
		}

		nextFailedCount := credential.FailedLoginCount + 1
		updates := map[string]interface{}{
			"failed_login_count": nextFailedCount,
		}
		if lockThreshold > 0 && nextFailedCount >= lockThreshold {
			updates["locked_until"] = lockUntil
		}

		if err := tx.Model(&model.UserCredential{}).
			Where("id = ?", credential.ID).
			Updates(updates).Error; err != nil {
			return err
		}

		if err := tx.Where("id = ?", credential.ID).First(&updated).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, translateError(err)
	}

	return toDomainCredential(updated), nil
}

// ResetLoginFailure 清零登录失败计数并解除锁定。
func (r *Repo) ResetLoginFailure(ctx context.Context, userID uint) error {
	return translateError(r.db.WithContext(ctx).
		Model(&model.UserCredential{}).
		Where("user_id = ?", userID).
		Updates(map[string]interface{}{
			"failed_login_count": 0,
			"locked_until":       nil,
		}).
		Error)
}

// UpdateUserStatus 更新用户状态。
func (r *Repo) UpdateUserStatus(ctx context.Context, userID uint, status string) error {
	return translateError(r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Update("status", status).
		Error)
}

// ResetPasswordByAdmin 重置用户密码并更新凭据元信息。
func (r *Repo) ResetPasswordByAdmin(ctx context.Context, userID uint, passwordHash string, mustResetPassword bool) error {
	return r.UpdatePassword(ctx, userID, passwordHash, domainuser.PasswordOriginAdminReset, mustResetPassword)
}

func (r *Repo) MarkBootstrapSuperAdminPasswordResetRequired(ctx context.Context, username string) error {
	normalizedUsername := strings.TrimSpace(username)
	if normalizedUsername == "" {
		return nil
	}
	result := r.db.WithContext(ctx).
		Model(&model.UserCredential{}).
		Where("user_id IN (?)",
			r.db.Model(&model.User{}).
				Select("id").
				Where("username = ? AND role = ? AND username_changed_at IS NULL", normalizedUsername, domainuser.RoleSuperAdmin),
		).
		Where("password_origin = ?", domainuser.PasswordOriginAdminCreated).
		Where("must_reset_password = ?", false).
		Update("must_reset_password", true)
	if result.Error != nil {
		return translateError(result.Error)
	}
	return nil
}

// UpdatePassword 更新用户本地密码状态。
func (r *Repo) UpdatePassword(ctx context.Context, userID uint, passwordHash string, passwordOrigin string, mustResetPassword bool) error {
	now := time.Now()
	if passwordOrigin == "" {
		passwordOrigin = domainuser.PasswordOriginUserSet
	}
	result := r.db.WithContext(ctx).
		Model(&model.UserCredential{}).
		Where("user_id = ?", userID).
		Updates(map[string]interface{}{
			"password_hash":       passwordHash,
			"password_algo":       "bcrypt",
			"password_enabled":    true,
			"password_updated_at": now,
			"password_set_at":     now,
			"password_origin":     passwordOrigin,
			"must_reset_password": mustResetPassword,
			"failed_login_count":  0,
			"locked_until":        nil,
		})
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// UpdateLastLogin 更新用户最后登录时间。
func (r *Repo) UpdateLastLogin(ctx context.Context, userID uint) error {
	return translateError(r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Update("last_login_at", time.Now()).
		Error)
}

// ListLatestSessionActivityByUserIDs 批量查询用户最近会话活跃时间。
func (r *Repo) ListLatestSessionActivityByUserIDs(ctx context.Context, userIDs []uint) (map[uint]time.Time, error) {
	results := make(map[uint]time.Time, len(userIDs))
	if len(userIDs) == 0 {
		return results, nil
	}

	sessions := make([]model.UserSession, 0, len(userIDs))
	latestSessionQuery := r.db.WithContext(ctx).
		Model(&model.UserSession{}).
		Select("identity_sessions.*, ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY COALESCE(last_seen_at, issued_at) DESC, id DESC) AS row_number").
		Where("user_id IN ?", userIDs)

	if err := r.db.WithContext(ctx).
		Table("(?) AS latest_sessions", latestSessionQuery).
		Where("row_number = ?", 1).
		Find(&sessions).Error; err != nil {
		return nil, translateError(err)
	}

	for _, session := range sessions {
		if session.LastSeenAt != nil {
			results[session.UserID] = *session.LastSeenAt
			continue
		}
		results[session.UserID] = session.IssuedAt
	}
	return results, nil
}

// DeleteAccountHard 删除用户主记录及主要用户域数据。
func (r *Repo) DeleteAccountHard(ctx context.Context, userID uint) error {
	return translateError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		conversationSubQuery := tx.Unscoped().Model(&model.Conversation{}).Select("id").Where("user_id = ?", userID)
		runSubQuery := tx.Unscoped().Model(&model.ConversationRun{}).Select("run_id").Where("user_id = ?", userID)

		steps := []struct {
			label string
			run   func(*gorm.DB) error
		}{
			{
				label: "identity_credentials",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.UserCredential{}).Error
				},
			},
			{
				label: "identity_sessions",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.UserSession{}).Error
				},
			},
			{
				label: "identity_auth_events",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.UserAuthEvent{}).Error
				},
			},
			{
				label: "identity_contact_verifications",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.UserContactVerification{}).Error
				},
			},
			{
				label: "identity_user_links",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.UserIdentity{}).Error
				},
			},
			{
				label: "identity_mfa_settings",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.UserTwoFactor{}).Error
				},
			},
			{
				label: "identity_trusted_devices",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.TrustedDevice{}).Error
				},
			},
			{
				label: "chat_context_records",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ? OR conversation_id IN (?)", userID, conversationSubQuery).Delete(&model.ChatContextRecord{}).Error
				},
			},
			{
				label: "chat_run_events",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ? OR run_id IN (?)", userID, runSubQuery).Delete(&model.ChatRunEvent{}).Error
				},
			},
			{
				label: "chat_runs",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.ConversationRun{}).Error
				},
			},
			{
				label: "chat_attachments",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.Attachment{}).Error
				},
			},
			{
				label: "chat_message_chunks",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.MessageChunk{}).Error
				},
			},
			{
				label: "chat_feedback",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.ConversationMessageFeedback{}).Error
				},
			},
			{
				label: "chat_messages",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.Message{}).Error
				},
			},
			{
				label: "chat_conversations",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.Conversation{}).Error
				},
			},
			{
				label: "file_chunks",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.FileChunk{}).Error
				},
			},
			{
				label: "file_objects",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.FileObject{}).Error
				},
			},
			{
				label: "file_storage_quotas",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.UserStorageQuota{}).Error
				},
			},
			{
				label: "user_memories",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.UserMemory{}).Error
				},
			},
			{
				label: "user_settings",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.UserSetting{}).Error
				},
			},
			// 财务审计事实不在账号硬删除中清理：
			// billing_usage_ledgers、billing_balance_transactions、billing_payment_orders
			// 保留调用、余额和支付追溯快照。
			{
				label: "billing_subscriptions",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.Subscription{}).Error
				},
			},
			{
				label: "billing_accounts",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("user_id = ?", userID).Delete(&model.BillingAccount{}).Error
				},
			},
			{
				label: "user",
				run: func(db *gorm.DB) error {
					return db.Unscoped().Where("id = ?", userID).Delete(&model.User{}).Error
				},
			},
		}

		for _, step := range steps {
			if err := step.run(tx); err != nil {
				return fmt.Errorf("delete %s: %w", step.label, err)
			}
		}

		return nil
	}))
}

// ListDistinctFileStoragePathsByUserID 查询用户文件去重后的存储路径。
func (r *Repo) ListDistinctFileStoragePathsByUserID(ctx context.Context, userID uint) ([]string, error) {
	paths := make([]string, 0)
	if err := r.db.WithContext(ctx).
		Model(&model.FileObject{}).
		Distinct("storage_path").
		Where("user_id = ? AND storage_path <> ''", userID).
		Pluck("storage_path", &paths).Error; err != nil {
		return nil, translateError(err)
	}
	return paths, nil
}

// RecordAuthEvent 写入认证事件。
func (r *Repo) RecordAuthEvent(
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
	item := &model.UserAuthEvent{
		RequestID:  requestID,
		UserID:     userID,
		EventType:  eventType,
		Result:     result,
		Reason:     reason,
		ClientIP:   clientIP,
		UserAgent:  userAgent,
		DetailJSON: detailJSON,
		OccurredAt: time.Now(),
	}
	return translateError(r.db.WithContext(ctx).Create(item).Error)
}

// CreateSession 创建用户登录会话。
func (r *Repo) CreateSession(ctx context.Context, item *domainuser.Session) error {
	dbSession := toModelSession(item)
	if err := r.db.WithContext(ctx).Create(dbSession).Error; err != nil {
		return translateError(err)
	}
	item.ID = dbSession.ID
	item.CreatedAt = dbSession.CreatedAt
	item.UpdatedAt = dbSession.UpdatedAt
	return nil
}

// GetSessionByUserAndSessionID 查询用户会话。
func (r *Repo) GetSessionByUserAndSessionID(ctx context.Context, userID uint, sessionID string) (*domainuser.Session, error) {
	var item model.UserSession
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND session_id = ?", userID, sessionID).
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomainSession(item), nil
}

// RotateSessionTokens 以会话行锁原子校验并轮换令牌信息。
func (r *Repo) RotateSessionTokens(ctx context.Context, input repository.RotateSessionTokensInput) error {
	return translateError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.UserSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND session_id = ?", input.UserID, input.SessionID).
			First(&item).Error; err != nil {
			return translateError(err)
		}

		if !sessionAcceptsPresentedRefreshHash(item, input.PresentedRefreshHash, input.Now, input.PreviousTokenGrace) {
			return repository.ErrInvalidInput
		}

		updates := map[string]interface{}{
			"previous_refresh_token_hash": item.RefreshTokenHash,
			"refresh_token_hash":          input.NextRefreshHash,
			"refresh_rotated_at":          input.Now,
			"access_jti":                  input.NextAccessJTI,
			"issued_at":                   input.IssuedAt,
			"expires_at":                  input.ExpiresAt,
			"revoked_at":                  nil,
			"revoke_reason":               "",
		}

		return translateError(tx.Model(&model.UserSession{}).
			Where("id = ?", item.ID).
			Updates(updates).
			Error)
	}))
}

func sessionAcceptsPresentedRefreshHash(
	item model.UserSession,
	presentedHash string,
	now time.Time,
	previousTokenGrace time.Duration,
) bool {
	normalizedPresentedHash := strings.TrimSpace(presentedHash)
	if normalizedPresentedHash == "" {
		return false
	}
	if item.RevokedAt != nil || !item.ExpiresAt.After(now) {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(item.RefreshTokenHash)), []byte(normalizedPresentedHash)) == 1 {
		return true
	}
	if previousTokenGrace <= 0 || item.RefreshRotatedAt == nil || now.Sub(*item.RefreshRotatedAt) > previousTokenGrace {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(strings.TrimSpace(item.PreviousRefreshTokenHash)), []byte(normalizedPresentedHash)) == 1
}

// TouchSessionActivity 更新会话最近活跃时间及审计元数据。
func (r *Repo) TouchSessionActivity(ctx context.Context, userID uint, sessionID string, input repository.UpdateSessionActivityInput) error {
	updates := sessionActivityUpdates(input)
	if len(updates) == 0 {
		return nil
	}

	return translateError(r.db.WithContext(ctx).
		Model(&model.UserSession{}).
		Where("user_id = ? AND session_id = ? AND revoked_at IS NULL", userID, sessionID).
		Updates(updates).
		Error)
}

func sessionActivityUpdates(input repository.UpdateSessionActivityInput) map[string]interface{} {
	updates := make(map[string]interface{})
	if input.LastSeenAt != nil {
		updates["last_seen_at"] = *input.LastSeenAt
	}
	if input.ClientIP != nil {
		updates["client_ip"] = *input.ClientIP
	}
	if input.UserAgent != nil {
		updates["user_agent"] = *input.UserAgent
	}
	if input.DeviceName != nil {
		updates["device_name"] = *input.DeviceName
	}
	if input.BrowserName != nil {
		updates["browser_name"] = *input.BrowserName
	}
	if input.OSName != nil {
		updates["os_name"] = *input.OSName
	}
	if input.DeviceType != nil {
		updates["device_type"] = *input.DeviceType
	}
	if input.GeoSource != nil {
		updates["geo_source"] = *input.GeoSource
	}
	if input.GeoAccuracy != nil {
		updates["geo_accuracy"] = *input.GeoAccuracy
	}
	if input.CountryCode != nil {
		updates["country_code"] = *input.CountryCode
	}
	if input.RegionName != nil {
		updates["region_name"] = *input.RegionName
	}
	if input.CityName != nil {
		updates["city_name"] = *input.CityName
	}
	if input.TimezoneName != nil {
		updates["timezone_name"] = *input.TimezoneName
	}
	if input.IPLatitude != nil {
		updates["ip_latitude"] = *input.IPLatitude
	}
	if input.IPLongitude != nil {
		updates["ip_longitude"] = *input.IPLongitude
	}
	if input.PreciseLatitude != nil {
		updates["precise_latitude"] = *input.PreciseLatitude
	}
	if input.PreciseLongitude != nil {
		updates["precise_longitude"] = *input.PreciseLongitude
	}
	if input.PreciseAccuracyM != nil {
		updates["precise_accuracy_m"] = *input.PreciseAccuracyM
	}
	if input.PreciseLocatedAt != nil {
		updates["precise_located_at"] = *input.PreciseLocatedAt
	}
	return updates
}

// RevokeSession 吊销单个会话。
func (r *Repo) RevokeSession(ctx context.Context, userID uint, sessionID string, reason string) error {
	now := time.Now()
	return translateError(r.db.WithContext(ctx).
		Model(&model.UserSession{}).
		Where("user_id = ? AND session_id = ? AND revoked_at IS NULL", userID, sessionID).
		Updates(map[string]interface{}{
			"revoked_at":    now,
			"revoke_reason": reason,
		}).
		Error)
}

// RevokeAllSessions 吊销用户全部活跃会话。
func (r *Repo) RevokeAllSessions(ctx context.Context, userID uint, reason string) error {
	now := time.Now()
	return translateError(r.db.WithContext(ctx).
		Model(&model.UserSession{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Updates(map[string]interface{}{
			"revoked_at":    now,
			"revoke_reason": reason,
		}).
		Error)
}

// ListActiveSessionsByUserID 查询用户当前活跃会话。
func (r *Repo) ListActiveSessionsByUserID(ctx context.Context, userID uint, now time.Time) ([]domainuser.Session, error) {
	items := make([]model.UserSession, 0)
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND revoked_at IS NULL AND expires_at > ?", userID, now).
		Order("COALESCE(last_seen_at, issued_at) DESC").
		Order("id DESC").
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomainSessions(items), nil
}

// ListAuthEvents 查询用户认证事件。
func (r *Repo) ListAuthEvents(
	ctx context.Context,
	userID uint,
	eventType string,
	result string,
	offset int,
	limit int,
) ([]domainuser.AuthEvent, int64, error) {
	items := make([]model.UserAuthEvent, 0)
	var total int64

	query := r.db.WithContext(ctx).Model(&model.UserAuthEvent{})
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}
	if result != "" {
		query = query.Where("result = ?", result)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	if err := query.
		Order("occurred_at DESC").
		Order("id DESC").
		Offset(offset).
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}
	return toDomainAuthEvents(items), total, nil
}

func toDomainUser(item model.User) *domainuser.User {
	return &domainuser.User{
		ID:                    item.ID,
		PublicID:              item.PublicID,
		Username:              item.Username,
		DisplayName:           item.DisplayName,
		AvatarURL:             item.AvatarURL,
		Email:                 item.Email,
		Phone:                 item.Phone,
		Role:                  item.Role,
		Status:                item.Status,
		Timezone:              item.Timezone,
		Locale:                item.Locale,
		ProfilePreferences:    item.ProfilePreferences,
		AppearancePreferences: item.AppearancePreferences,
		OnboardingCompletedAt: item.OnboardingCompletedAt,
		EmailVerifiedAt:       item.EmailVerifiedAt,
		EmailSource:           item.EmailSource,
		EmailBootstrapUsedAt:  item.EmailBootstrapUsedAt,
		PhoneVerifiedAt:       item.PhoneVerifiedAt,
		UsernameChangedAt:     item.UsernameChangedAt,
		LastLoginAt:           item.LastLoginAt,
		CreatedAt:             item.CreatedAt,
		UpdatedAt:             item.UpdatedAt,
	}
}

func (r *Repo) ListIdentityProviders(ctx context.Context, includeDisabled bool) ([]domainuser.IdentityProvider, error) {
	items := make([]model.AuthIdentityProvider, 0)
	query := r.db.WithContext(ctx).Order("sort_order ASC, id ASC")
	if !includeDisabled {
		query = query.Where("login_enabled = ? OR registration_enabled = ?", true, true)
	}
	if err := query.Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomainIdentityProviders(items), nil
}

func (r *Repo) HasActiveSuperAdminIdentity(ctx context.Context) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.UserIdentity{}).
		Joins("JOIN identity_users ON identity_users.id = identity_user_links.user_id").
		Where("identity_users.role = ? AND identity_users.status = ?", model.RoleSuperAdmin, model.UserStatusActive).
		Count(&count).Error
	if err != nil {
		return false, translateError(err)
	}
	return count > 0, nil
}

func (r *Repo) GetIdentityProviderByPublicID(ctx context.Context, publicID string) (*domainuser.IdentityProvider, error) {
	var item model.AuthIdentityProvider
	if err := r.db.WithContext(ctx).Where("public_id = ?", publicID).First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomainIdentityProvider(item), nil
}

func (r *Repo) GetIdentityProviderBySlug(ctx context.Context, slug string) (*domainuser.IdentityProvider, error) {
	var item model.AuthIdentityProvider
	if err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomainIdentityProvider(item), nil
}

func (r *Repo) CreateIdentityProvider(ctx context.Context, provider *domainuser.IdentityProvider) (*domainuser.IdentityProvider, error) {
	item := toModelIdentityProvider(provider)
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.deleteStaleIdentityProvidersBySlug(ctx, tx, item.Slug); err != nil {
			return err
		}
		if err := tx.Create(item).Error; err != nil {
			return translateError(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return toDomainIdentityProvider(*item), nil
}

func (r *Repo) UpdateIdentityProvider(ctx context.Context, publicID string, input repository.UpdateIdentityProviderInput) (*domainuser.IdentityProvider, error) {
	updates := identityProviderUpdates(input)
	if len(updates) == 0 {
		return r.GetIdentityProviderByPublicID(ctx, publicID)
	}
	result := r.db.WithContext(ctx).
		Model(&model.AuthIdentityProvider{}).
		Where("public_id = ?", publicID).
		Updates(updates)
	if result.Error != nil {
		return nil, translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, repository.ErrNotFound
	}
	return r.GetIdentityProviderByPublicID(ctx, publicID)
}

func identityProviderUpdates(input repository.UpdateIdentityProviderInput) map[string]interface{} {
	updates := make(map[string]interface{})
	if input.Type != nil {
		updates["type"] = *input.Type
	}
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.Slug != nil {
		updates["slug"] = *input.Slug
	}
	if input.LogoURL != nil {
		updates["logo_url"] = *input.LogoURL
	}
	if input.LoginEnabled != nil {
		updates["login_enabled"] = *input.LoginEnabled
	}
	if input.RegistrationEnabled != nil {
		updates["registration_enabled"] = *input.RegistrationEnabled
	}
	if input.ClientID != nil {
		updates["client_id"] = *input.ClientID
	}
	if input.ClientSecret != nil {
		updates["client_secret_encrypted"] = *input.ClientSecret
	}
	if input.IssuerURL != nil {
		updates["issuer_url"] = *input.IssuerURL
	}
	if input.DiscoveryURL != nil {
		updates["discovery_url"] = *input.DiscoveryURL
	}
	if input.AuthURL != nil {
		updates["auth_url"] = *input.AuthURL
	}
	if input.TokenURL != nil {
		updates["token_url"] = *input.TokenURL
	}
	if input.UserInfoURL != nil {
		updates["user_info_url"] = *input.UserInfoURL
	}
	if input.JWKSURL != nil {
		updates["jwks_url"] = *input.JWKSURL
	}
	if input.Scopes != nil {
		updates["scopes"] = *input.Scopes
	}
	if input.PKCEEnabled != nil {
		updates["pkce_enabled"] = *input.PKCEEnabled
	}
	if input.DefaultRole != nil {
		updates["default_role"] = *input.DefaultRole
	}
	if input.SubjectField != nil {
		updates["subject_field"] = *input.SubjectField
	}
	if input.EmailField != nil {
		updates["email_field"] = *input.EmailField
	}
	if input.EmailVerifiedField != nil {
		updates["email_verified_field"] = *input.EmailVerifiedField
	}
	if input.NameField != nil {
		updates["name_field"] = *input.NameField
	}
	if input.AvatarField != nil {
		updates["avatar_field"] = *input.AvatarField
	}
	return updates
}

func (r *Repo) UpdateIdentityProviderSortOrders(ctx context.Context, publicIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for index, publicID := range publicIDs {
			result := tx.Model(&model.AuthIdentityProvider{}).
				Where("public_id = ?", publicID).
				Update("sort_order", (index+1)*100)
			if result.Error != nil {
				return translateError(result.Error)
			}
			if result.RowsAffected == 0 {
				return repository.ErrNotFound
			}
		}
		return nil
	})
}

func (r *Repo) DeleteIdentityProvider(ctx context.Context, publicID string, force bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var provider model.AuthIdentityProvider
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("public_id = ?", publicID).
			First(&provider).Error; err != nil {
			return translateError(err)
		}
		if !force {
			if err := r.ensureIdentityProviderDeleteAllowed(ctx, tx, provider.ID); err != nil {
				return err
			}
		}
		return r.deleteIdentityProviderByIDHard(ctx, tx, provider.ID)
	})
}

func (r *Repo) ensureIdentityProviderDeleteAllowed(ctx context.Context, tx *gorm.DB, providerID uint) error {
	providerIdentities := make([]model.UserIdentity, 0)
	if err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("provider_id = ?", providerID).
		Find(&providerIdentities).Error; err != nil {
		return translateError(err)
	}

	checkedUserIDs := make(map[uint]struct{}, len(providerIdentities))
	dependentUsers := 0
	for _, providerIdentity := range providerIdentities {
		if _, checked := checkedUserIDs[providerIdentity.UserID]; checked {
			continue
		}
		checkedUserIDs[providerIdentity.UserID] = struct{}{}

		var credential model.UserCredential
		if err := tx.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", providerIdentity.UserID).
			First(&credential).Error; err != nil {
			return translateError(err)
		}
		if credential.PasswordEnabled {
			continue
		}

		var identityCount int64
		if err := tx.WithContext(ctx).
			Model(&model.UserIdentity{}).
			Where("user_id = ?", providerIdentity.UserID).
			Count(&identityCount).Error; err != nil {
			return translateError(err)
		}
		if identityCount <= 1 {
			dependentUsers++
		}
	}
	if dependentUsers > 0 {
		return &repository.IdentityProviderDeleteConflictError{DependentUsers: dependentUsers}
	}
	return nil
}

func (r *Repo) deleteStaleIdentityProvidersBySlug(ctx context.Context, tx *gorm.DB, slug string) error {
	staleProviders := make([]model.AuthIdentityProvider, 0)
	if err := tx.WithContext(ctx).
		Unscoped().
		Where("slug = ? AND deleted_at IS NOT NULL", slug).
		Find(&staleProviders).Error; err != nil {
		return translateError(err)
	}
	for _, provider := range staleProviders {
		if err := r.deleteIdentityProviderByIDHard(ctx, tx, provider.ID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) deleteIdentityProviderByIDHard(ctx context.Context, tx *gorm.DB, providerID uint) error {
	if err := tx.WithContext(ctx).
		Unscoped().
		Where("provider_id = ?", providerID).
		Delete(&model.UserIdentity{}).Error; err != nil {
		return translateError(err)
	}
	result := tx.WithContext(ctx).
		Unscoped().
		Where("id = ?", providerID).
		Delete(&model.AuthIdentityProvider{})
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *Repo) GetUserIdentityByProviderSubject(ctx context.Context, providerID uint, subject string) (*domainuser.UserIdentity, error) {
	var item model.UserIdentity
	if err := r.db.WithContext(ctx).
		Where("provider_id = ? AND provider_subject = ?", providerID, subject).
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomainUserIdentity(item), nil
}

func (r *Repo) ListUserIdentitiesByUserID(ctx context.Context, userID uint) ([]domainuser.UserIdentity, error) {
	items := make([]model.UserIdentity, 0)
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	results := make([]domainuser.UserIdentity, 0, len(items))
	for _, item := range items {
		results = append(results, *toDomainUserIdentity(item))
	}
	return results, nil
}

func (r *Repo) CreateUserIdentity(ctx context.Context, identity *domainuser.UserIdentity) (*domainuser.UserIdentity, error) {
	item := toModelUserIdentity(identity)
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.deleteStaleUserIdentitiesByProviderSubject(ctx, tx, item.ProviderID, item.ProviderSubject); err != nil {
			return err
		}
		if err := tx.Create(item).Error; err != nil {
			return translateError(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return toDomainUserIdentity(*item), nil
}

func (r *Repo) deleteStaleUserIdentitiesByProviderSubject(ctx context.Context, tx *gorm.DB, providerID uint, subject string) error {
	if err := tx.WithContext(ctx).
		Unscoped().
		Where("provider_id = ? AND provider_subject = ? AND deleted_at IS NOT NULL", providerID, subject).
		Delete(&model.UserIdentity{}).Error; err != nil {
		return translateError(err)
	}
	return nil
}

func (r *Repo) DeleteUserIdentity(ctx context.Context, userID uint, identityID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var credential model.UserCredential
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).
			First(&credential).Error; err != nil {
			return translateError(err)
		}

		identities := make([]model.UserIdentity, 0)
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).
			Order("id ASC").
			Find(&identities).Error; err != nil {
			return translateError(err)
		}

		found := false
		for _, identity := range identities {
			if identity.ID == identityID {
				found = true
				break
			}
		}
		if !found {
			return repository.ErrNotFound
		}
		if !credential.PasswordEnabled && len(identities) <= 1 {
			return repository.ErrConflict
		}

		result := tx.
			Unscoped().
			Where("id = ? AND user_id = ?", identityID, userID).
			Delete(&model.UserIdentity{})
		if result.Error != nil {
			return translateError(result.Error)
		}
		if result.RowsAffected == 0 {
			return repository.ErrNotFound
		}
		return nil
	})
}

func (r *Repo) UpdateUserIdentityLogin(ctx context.Context, identityID uint, profileJSON string, providerDisplayName string, email string, emailVerified bool) error {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&model.UserIdentity{}).
		Where("id = ?", identityID).
		Updates(map[string]interface{}{
			"profile_json":          profileJSON,
			"provider_display_name": providerDisplayName,
			"email":                 email,
			"email_verified":        emailVerified,
			"last_login_at":         now,
		})
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *Repo) CancelPendingContactVerifications(ctx context.Context, channel string, purpose string, target string) error {
	now := time.Now()
	return translateError(r.db.WithContext(ctx).
		Model(&model.UserContactVerification{}).
		Where("channel = ? AND purpose = ? AND target = ? AND status = ?", channel, purpose, target, model.ContactVerificationStatusPending).
		Updates(map[string]interface{}{
			"status":      model.ContactVerificationStatusCanceled,
			"consumed_at": now,
		}).Error)
}

func (r *Repo) CancelPendingContactVerificationsForUser(ctx context.Context, userID uint, channel string, purpose string, target string) error {
	now := time.Now()
	return translateError(r.db.WithContext(ctx).
		Model(&model.UserContactVerification{}).
		Where("user_id = ? AND channel = ? AND purpose = ? AND target = ? AND status = ?", userID, channel, purpose, target, model.ContactVerificationStatusPending).
		Updates(map[string]interface{}{
			"status":      model.ContactVerificationStatusCanceled,
			"consumed_at": now,
		}).Error)
}

func (r *Repo) CreateContactVerification(ctx context.Context, item *domainuser.ContactVerification) (*domainuser.ContactVerification, error) {
	dbItem := toModelContactVerification(item)
	if err := r.db.WithContext(ctx).Create(dbItem).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomainContactVerification(*dbItem), nil
}

func (r *Repo) GetPendingContactVerification(ctx context.Context, channel string, purpose string, target string, now time.Time) (*domainuser.ContactVerification, error) {
	var item model.UserContactVerification
	if err := r.db.WithContext(ctx).
		Where("channel = ? AND purpose = ? AND target = ? AND status = ?", channel, purpose, target, model.ContactVerificationStatusPending).
		Where("expires_at IS NULL OR expires_at > ?", now).
		Order("id DESC").
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomainContactVerification(item), nil
}

func (r *Repo) GetPendingContactVerificationForUser(ctx context.Context, userID uint, channel string, purpose string, target string, now time.Time) (*domainuser.ContactVerification, error) {
	var item model.UserContactVerification
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND channel = ? AND purpose = ? AND target = ? AND status = ?", userID, channel, purpose, target, model.ContactVerificationStatusPending).
		Where("expires_at IS NULL OR expires_at > ?", now).
		Order("id DESC").
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomainContactVerification(item), nil
}

func (r *Repo) IncrementContactVerificationAttempt(ctx context.Context, verificationID uint) error {
	result := r.db.WithContext(ctx).
		Model(&model.UserContactVerification{}).
		Where("id = ?", verificationID).
		UpdateColumn("attempt_count", gorm.Expr("attempt_count + ?", 1))
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *Repo) MarkContactVerificationVerified(ctx context.Context, verificationID uint, now time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&model.UserContactVerification{}).
		Where("id = ? AND status = ?", verificationID, model.ContactVerificationStatusPending).
		Updates(map[string]interface{}{
			"status":      model.ContactVerificationStatusVerified,
			"verified_at": now,
			"consumed_at": now,
		})
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func toDomainUsers(items []model.User) []domainuser.User {
	results := make([]domainuser.User, 0, len(items))
	for _, item := range items {
		results = append(results, *toDomainUser(item))
	}
	return results
}

func toModelUser(item *domainuser.User) *model.User {
	if item == nil {
		return &model.User{}
	}
	return &model.User{
		BaseModel: model.BaseModel{
			ID:        item.ID,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		},
		PublicID:              item.PublicID,
		Username:              item.Username,
		DisplayName:           item.DisplayName,
		AvatarURL:             item.AvatarURL,
		Email:                 item.Email,
		Phone:                 item.Phone,
		Role:                  item.Role,
		Status:                item.Status,
		Timezone:              item.Timezone,
		Locale:                item.Locale,
		ProfilePreferences:    item.ProfilePreferences,
		AppearancePreferences: item.AppearancePreferences,
		OnboardingCompletedAt: item.OnboardingCompletedAt,
		EmailVerifiedAt:       item.EmailVerifiedAt,
		EmailSource:           item.EmailSource,
		EmailBootstrapUsedAt:  item.EmailBootstrapUsedAt,
		PhoneVerifiedAt:       item.PhoneVerifiedAt,
		UsernameChangedAt:     item.UsernameChangedAt,
		LastLoginAt:           item.LastLoginAt,
	}
}

func toDomainPlan(item model.BillingPlan) *domainbilling.Plan {
	return &domainbilling.Plan{
		ID:                  item.ID,
		Code:                item.Code,
		Name:                item.Name,
		Description:         item.Description,
		FeatureJSON:         item.FeatureJSON,
		PeriodCreditNanousd: item.PeriodCreditNanousd,
		DiscountPercent:     item.DiscountPercent,
		SortOrder:           item.SortOrder,
		IsActive:            item.IsActive,
		CreatedAt:           item.CreatedAt,
		UpdatedAt:           item.UpdatedAt,
	}
}

func toDomainPrice(item model.BillingPrice) *domainbilling.Price {
	return &domainbilling.Price{
		ID:               item.ID,
		PlanID:           item.PlanID,
		Code:             item.Code,
		BillingInterval:  item.BillingInterval,
		Currency:         item.Currency,
		AmountCents:      item.AmountCents,
		IsActive:         item.IsActive,
		IsDefault:        item.IsDefault,
		ExternalPriceRef: item.ExternalPriceRef,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func toDomainCredential(item model.UserCredential) *domainuser.Credential {
	return &domainuser.Credential{
		ID:                item.ID,
		UserID:            item.UserID,
		PasswordHash:      item.PasswordHash,
		PasswordAlgo:      item.PasswordAlgo,
		PasswordEnabled:   item.PasswordEnabled,
		PasswordUpdatedAt: item.PasswordUpdatedAt,
		PasswordSetAt:     item.PasswordSetAt,
		PasswordOrigin:    item.PasswordOrigin,
		MustResetPassword: item.MustResetPassword,
		FailedLoginCount:  item.FailedLoginCount,
		LockedUntil:       item.LockedUntil,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
}

func toDomainUserTwoFactor(item model.UserTwoFactor) *domainuser.UserTwoFactor {
	return &domainuser.UserTwoFactor{
		ID:                     item.ID,
		UserID:                 item.UserID,
		TOTPEnabled:            item.TOTPEnabled,
		TOTPSecretEncrypted:    item.TOTPSecretEncrypted,
		TOTPSetupExpiresAt:     item.TOTPSetupExpiresAt,
		RecoveryCodesHash:      item.RecoveryCodesHash,
		Enforced:               item.Enforced,
		EnabledAt:              item.EnabledAt,
		LastVerifiedAt:         item.LastVerifiedAt,
		TrustedDeviceExpiresAt: item.TrustedDeviceExpiresAt,
		CreatedAt:              item.CreatedAt,
		UpdatedAt:              item.UpdatedAt,
	}
}

func toModelUserTwoFactor(item *domainuser.UserTwoFactor) *model.UserTwoFactor {
	if item == nil {
		return &model.UserTwoFactor{}
	}
	return &model.UserTwoFactor{
		BaseModel: model.BaseModel{
			ID:        item.ID,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		},
		UserID:                 item.UserID,
		TOTPEnabled:            item.TOTPEnabled,
		TOTPSecretEncrypted:    item.TOTPSecretEncrypted,
		TOTPSetupExpiresAt:     item.TOTPSetupExpiresAt,
		RecoveryCodesHash:      item.RecoveryCodesHash,
		Enforced:               item.Enforced,
		EnabledAt:              item.EnabledAt,
		LastVerifiedAt:         item.LastVerifiedAt,
		TrustedDeviceExpiresAt: item.TrustedDeviceExpiresAt,
	}
}

func toDomainSession(item model.UserSession) *domainuser.Session {
	return &domainuser.Session{
		ID:                       item.ID,
		SessionID:                item.SessionID,
		UserID:                   item.UserID,
		RefreshTokenHash:         item.RefreshTokenHash,
		PreviousRefreshTokenHash: item.PreviousRefreshTokenHash,
		RefreshRotatedAt:         item.RefreshRotatedAt,
		AccessJTI:                item.AccessJTI,
		ClientIP:                 item.ClientIP,
		UserAgent:                item.UserAgent,
		DeviceName:               item.DeviceName,
		BrowserName:              item.BrowserName,
		OSName:                   item.OSName,
		DeviceType:               item.DeviceType,
		GeoSource:                item.GeoSource,
		GeoAccuracy:              item.GeoAccuracy,
		CountryCode:              item.CountryCode,
		RegionName:               item.RegionName,
		CityName:                 item.CityName,
		TimezoneName:             item.TimezoneName,
		IPLatitude:               item.IPLatitude,
		IPLongitude:              item.IPLongitude,
		PreciseLatitude:          item.PreciseLatitude,
		PreciseLongitude:         item.PreciseLongitude,
		PreciseAccuracyM:         item.PreciseAccuracyM,
		PreciseLocatedAt:         item.PreciseLocatedAt,
		IssuedAt:                 item.IssuedAt,
		LastSeenAt:               item.LastSeenAt,
		ExpiresAt:                item.ExpiresAt,
		RevokedAt:                item.RevokedAt,
		RevokeReason:             item.RevokeReason,
		CreatedAt:                item.CreatedAt,
		UpdatedAt:                item.UpdatedAt,
	}
}

func toDomainSessions(items []model.UserSession) []domainuser.Session {
	results := make([]domainuser.Session, 0, len(items))
	for _, item := range items {
		results = append(results, *toDomainSession(item))
	}
	return results
}

func toModelSession(item *domainuser.Session) *model.UserSession {
	if item == nil {
		return &model.UserSession{}
	}
	return &model.UserSession{
		BaseModel: model.BaseModel{
			ID:        item.ID,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		},
		SessionID:                item.SessionID,
		UserID:                   item.UserID,
		RefreshTokenHash:         item.RefreshTokenHash,
		PreviousRefreshTokenHash: item.PreviousRefreshTokenHash,
		RefreshRotatedAt:         item.RefreshRotatedAt,
		AccessJTI:                item.AccessJTI,
		ClientIP:                 item.ClientIP,
		UserAgent:                item.UserAgent,
		DeviceName:               item.DeviceName,
		BrowserName:              item.BrowserName,
		OSName:                   item.OSName,
		DeviceType:               item.DeviceType,
		GeoSource:                item.GeoSource,
		GeoAccuracy:              item.GeoAccuracy,
		CountryCode:              item.CountryCode,
		RegionName:               item.RegionName,
		CityName:                 item.CityName,
		TimezoneName:             item.TimezoneName,
		IPLatitude:               item.IPLatitude,
		IPLongitude:              item.IPLongitude,
		PreciseLatitude:          item.PreciseLatitude,
		PreciseLongitude:         item.PreciseLongitude,
		PreciseAccuracyM:         item.PreciseAccuracyM,
		PreciseLocatedAt:         item.PreciseLocatedAt,
		IssuedAt:                 item.IssuedAt,
		LastSeenAt:               item.LastSeenAt,
		ExpiresAt:                item.ExpiresAt,
		RevokedAt:                item.RevokedAt,
		RevokeReason:             item.RevokeReason,
	}
}

func toDomainAuthEvents(items []model.UserAuthEvent) []domainuser.AuthEvent {
	results := make([]domainuser.AuthEvent, 0, len(items))
	for _, item := range items {
		results = append(results, domainuser.AuthEvent{
			ID:         item.ID,
			RequestID:  item.RequestID,
			UserID:     item.UserID,
			EventType:  item.EventType,
			Result:     item.Result,
			Reason:     item.Reason,
			ClientIP:   item.ClientIP,
			UserAgent:  item.UserAgent,
			DetailJSON: item.DetailJSON,
			OccurredAt: item.OccurredAt,
			CreatedAt:  item.CreatedAt,
			UpdatedAt:  item.UpdatedAt,
		})
	}
	return results
}

func toDomainContactVerification(item model.UserContactVerification) *domainuser.ContactVerification {
	return &domainuser.ContactVerification{
		ID:           item.ID,
		UserID:       item.UserID,
		Channel:      item.Channel,
		Purpose:      item.Purpose,
		Target:       item.Target,
		Token:        item.Token,
		CodeHash:     item.CodeHash,
		Status:       item.Status,
		SentAt:       item.SentAt,
		ExpiresAt:    item.ExpiresAt,
		VerifiedAt:   item.VerifiedAt,
		ConsumedAt:   item.ConsumedAt,
		AttemptCount: item.AttemptCount,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

func toModelContactVerification(item *domainuser.ContactVerification) *model.UserContactVerification {
	if item == nil {
		return &model.UserContactVerification{}
	}
	return &model.UserContactVerification{
		BaseModel: model.BaseModel{
			ID:        item.ID,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		},
		UserID:       item.UserID,
		Channel:      item.Channel,
		Purpose:      item.Purpose,
		Target:       item.Target,
		Token:        item.Token,
		CodeHash:     item.CodeHash,
		Status:       item.Status,
		SentAt:       item.SentAt,
		ExpiresAt:    item.ExpiresAt,
		VerifiedAt:   item.VerifiedAt,
		ConsumedAt:   item.ConsumedAt,
		AttemptCount: item.AttemptCount,
	}
}

func toDomainIdentityProviders(items []model.AuthIdentityProvider) []domainuser.IdentityProvider {
	results := make([]domainuser.IdentityProvider, 0, len(items))
	for _, item := range items {
		results = append(results, *toDomainIdentityProvider(item))
	}
	return results
}

func toDomainIdentityProvider(item model.AuthIdentityProvider) *domainuser.IdentityProvider {
	return &domainuser.IdentityProvider{
		ID:                  item.ID,
		PublicID:            item.PublicID,
		Type:                item.Type,
		Name:                item.Name,
		Slug:                item.Slug,
		LogoURL:             item.LogoURL,
		LoginEnabled:        item.LoginEnabled,
		RegistrationEnabled: item.RegistrationEnabled,
		ClientID:            item.ClientID,
		ClientSecret:        item.ClientSecretEncrypted,
		IssuerURL:           item.IssuerURL,
		DiscoveryURL:        item.DiscoveryURL,
		AuthURL:             item.AuthURL,
		TokenURL:            item.TokenURL,
		UserInfoURL:         item.UserInfoURL,
		JWKSURL:             item.JWKSURL,
		Scopes:              item.Scopes,
		PKCEEnabled:         item.PKCEEnabled,
		DefaultRole:         item.DefaultRole,
		SubjectField:        item.SubjectField,
		EmailField:          item.EmailField,
		EmailVerifiedField:  item.EmailVerifiedField,
		NameField:           item.NameField,
		AvatarField:         item.AvatarField,
		SortOrder:           item.SortOrder,
		CreatedAt:           item.CreatedAt,
		UpdatedAt:           item.UpdatedAt,
	}
}

func toModelIdentityProvider(item *domainuser.IdentityProvider) *model.AuthIdentityProvider {
	if item == nil {
		return &model.AuthIdentityProvider{}
	}
	return &model.AuthIdentityProvider{
		BaseModel: model.BaseModel{
			ID:        item.ID,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		},
		PublicID:              item.PublicID,
		Type:                  item.Type,
		Name:                  item.Name,
		Slug:                  item.Slug,
		LogoURL:               item.LogoURL,
		LoginEnabled:          item.LoginEnabled,
		RegistrationEnabled:   item.RegistrationEnabled,
		ClientID:              item.ClientID,
		ClientSecretEncrypted: item.ClientSecret,
		IssuerURL:             item.IssuerURL,
		DiscoveryURL:          item.DiscoveryURL,
		AuthURL:               item.AuthURL,
		TokenURL:              item.TokenURL,
		UserInfoURL:           item.UserInfoURL,
		JWKSURL:               item.JWKSURL,
		Scopes:                item.Scopes,
		PKCEEnabled:           item.PKCEEnabled,
		DefaultRole:           item.DefaultRole,
		SubjectField:          item.SubjectField,
		EmailField:            item.EmailField,
		EmailVerifiedField:    item.EmailVerifiedField,
		NameField:             item.NameField,
		AvatarField:           item.AvatarField,
		SortOrder:             item.SortOrder,
	}
}

func toDomainUserIdentity(item model.UserIdentity) *domainuser.UserIdentity {
	return &domainuser.UserIdentity{
		ID:                  item.ID,
		UserID:              item.UserID,
		ProviderID:          item.ProviderID,
		ProviderType:        item.ProviderType,
		ProviderSubject:     item.ProviderSubject,
		ProviderDisplayName: item.ProviderDisplayName,
		Email:               item.Email,
		EmailVerified:       item.EmailVerified,
		ProfileJSON:         item.ProfileJSON,
		LinkedAt:            item.LinkedAt,
		LastLoginAt:         item.LastLoginAt,
		CreatedAt:           item.CreatedAt,
		UpdatedAt:           item.UpdatedAt,
	}
}

func toModelUserIdentity(item *domainuser.UserIdentity) *model.UserIdentity {
	if item == nil {
		return &model.UserIdentity{}
	}
	return &model.UserIdentity{
		BaseModel: model.BaseModel{
			ID:        item.ID,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		},
		UserID:              item.UserID,
		ProviderID:          item.ProviderID,
		ProviderType:        item.ProviderType,
		ProviderSubject:     item.ProviderSubject,
		ProviderDisplayName: item.ProviderDisplayName,
		Email:               item.Email,
		EmailVerified:       item.EmailVerified,
		ProfileJSON:         item.ProfileJSON,
		LinkedAt:            item.LinkedAt,
		LastLoginAt:         item.LastLoginAt,
	}
}
