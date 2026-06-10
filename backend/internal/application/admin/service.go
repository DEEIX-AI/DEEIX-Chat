package admin

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"

	auditapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
	authapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/auth"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	systemeventapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/systemevent"
	userapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/userview"
	domainaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/audit"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainsystemevent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/systemevent"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type userService interface {
	ListUsers(ctx context.Context, page int, pageSize int) ([]domainuser.User, int64, error)
	CountSuperAdmins(ctx context.Context) (int64, error)
	CreateUser(
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
	) (*domainuser.User, error)
	GetByID(ctx context.Context, userID uint) (*domainuser.User, error)
	RevokeAllSessions(ctx context.Context, userID uint, reason string) error
	UpdateUserStatus(ctx context.Context, userID uint, status string) error
	UpdateFields(ctx context.Context, userID uint, input repository.UpdateUserFieldsInput) (*domainuser.User, error)
	ResetLoginFailure(ctx context.Context, userID uint) error
	ResetPasswordByAdmin(ctx context.Context, userID uint, newPassword string, mustResetPassword bool) error
	DeleteAccountHard(ctx context.Context, userID uint) error
	RecordAuthEvent(
		ctx context.Context,
		userID uint,
		requestID string,
		eventType string,
		result string,
		reason string,
		clientIP string,
		userAgent string,
		detailJSON string,
	) error
	ListAuthEvents(
		ctx context.Context,
		userID uint,
		eventType string,
		result string,
		page int,
		pageSize int,
	) ([]domainuser.AuthEvent, int64, error)
}

type auditService interface {
	Write(
		ctx context.Context,
		requestID string,
		actorUserID uint,
		action string,
		resource string,
		resourceID string,
		ip string,
		userAgent string,
		detail interface{},
	)
	List(ctx context.Context, page int, pageSize int, filter auditapp.ListFilter) ([]domainaudit.Log, int64, error)
}

type systemEventService interface {
	List(ctx context.Context, page int, pageSize int, filter systemeventapp.ListFilter) ([]domainsystemevent.Event, int64, error)
}

type usageLogService interface {
	ListUsageLogs(ctx context.Context, page int, pageSize int, filter billing.UsageLogListFilter) ([]domainbilling.UsageLedger, int64, error)
}

type orderLogService interface {
	ListPaymentOrders(ctx context.Context, page int, pageSize int, filter billing.PaymentOrderListFilter) ([]domainbilling.PaymentOrder, int64, error)
}

type conversationEventService interface {
	ListConversationEventLogs(ctx context.Context, page int, pageSize int, filter appconversation.EventLogListFilter) ([]domainconversation.EventLog, int64, error)
}

type authSecurityService interface {
	GetCurrentTwoFactorStatus(ctx context.Context, userID uint) (*authapp.TwoFactorStatusResult, error)
	ResetUserTwoFactorByAdmin(ctx context.Context, userID uint) error
}

// Service 聚合后台域服务依赖。
type Service struct {
	userService          userService
	auditService         auditService
	systemEventService   systemEventService
	usageLogService      usageLogService
	orderLogService      orderLogService
	conversationEventSvc conversationEventService
	authSecurityService  authSecurityService
	subscriptionResolver subscriptionResolver
}

type subscriptionResolver interface {
	ListCurrentSubscriptionSnapshots(
		ctx context.Context,
		userIDs []uint,
		now time.Time,
	) (map[uint]billing.UserSubscriptionSnapshot, error)
	GetCurrentSubscriptionSnapshot(
		ctx context.Context,
		userID uint,
		now time.Time,
	) (*billing.UserSubscriptionSnapshot, error)
	GetBillingMode(ctx context.Context) (string, error)
	ListBillingAccountSnapshots(ctx context.Context, userIDs []uint) (map[uint]billing.UserBillingAccountSnapshot, error)
	SetUserSubscriptionByPlanCode(
		ctx context.Context,
		userID uint,
		planCode string,
		expiresAt *time.Time,
	) (*billing.UserSubscriptionSnapshot, error)
}

// UserLabel 是后台日志里展示用户身份的轻量信息。
type UserLabel struct {
	ID          uint
	Username    string
	DisplayName string
	Label       string
}

// NewService 创建服务。
func NewService(userService userService, auditService auditService) *Service {
	return &Service{
		userService:  userService,
		auditService: auditService,
	}
}

// SetAuthSecurityService 注入认证安全校验能力。
func (s *Service) SetAuthSecurityService(service authSecurityService) {
	s.authSecurityService = service
}

// SetSystemEventService 注入系统事件查询能力。
func (s *Service) SetSystemEventService(service systemEventService) {
	s.systemEventService = service
}

// SetUsageLogService 注入调用日志查询能力。
func (s *Service) SetUsageLogService(service usageLogService) {
	s.usageLogService = service
}

// SetOrderLogService 注入支付订单日志查询能力。
func (s *Service) SetOrderLogService(service orderLogService) {
	s.orderLogService = service
}

// SetConversationEventService 注入对话事件查询能力。
func (s *Service) SetConversationEventService(service conversationEventService) {
	s.conversationEventSvc = service
}

// SetSubscriptionResolver 注入订阅派生解析能力。
func (s *Service) SetSubscriptionResolver(resolver subscriptionResolver) {
	s.subscriptionResolver = resolver
}

// ListUsers 查询用户分页列表。
func (s *Service) ListUsers(ctx context.Context, page int, pageSize int) ([]userview.UserView, int64, error) {
	items, total, err := s.userService.ListUsers(ctx, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	results, err := s.BuildUserViews(ctx, items)
	if err != nil {
		return nil, 0, err
	}
	return results, total, nil
}

// BuildUserView 构建单个用户的前端展示视图。
func (s *Service) BuildUserView(ctx context.Context, item domainuser.User) (userview.UserView, error) {
	if s.subscriptionResolver == nil {
		return s.applyTwoFactorView(ctx, userview.FromUser(item, nil))
	}

	mode, err := s.subscriptionResolver.GetBillingMode(ctx)
	if err != nil {
		return userview.UserView{}, err
	}
	if mode == "usage" {
		accounts, accountErr := s.subscriptionResolver.ListBillingAccountSnapshots(ctx, []uint{item.ID})
		if accountErr != nil {
			return userview.UserView{}, accountErr
		}
		account, ok := accounts[item.ID]
		view := userview.FromUser(item, nil)
		if ok {
			view = userview.WithBillingAccount(view, &userview.BillingAccountState{
				Currency:       account.Currency,
				BalanceNanousd: account.BalanceNanousd,
				Status:         account.Status,
			})
		}
		return s.applyTwoFactorView(ctx, view)
	}

	subscription, err := s.subscriptionResolver.GetCurrentSubscriptionSnapshot(ctx, item.ID, time.Now())
	if err != nil {
		return userview.UserView{}, err
	}
	if subscription == nil {
		return s.applyTwoFactorView(ctx, userview.FromUser(item, nil))
	}

	return s.applyTwoFactorView(ctx, userview.FromUser(item, &userview.SubscriptionState{
		PlanID:    subscription.PlanID,
		PlanName:  subscription.PlanName,
		Tier:      subscription.Tier,
		Status:    subscription.Status,
		ExpiresAt: subscription.ExpiresAt,
	}))
}

// BuildUserViews 批量构建用户展示视图。
func (s *Service) BuildUserViews(ctx context.Context, items []domainuser.User) ([]userview.UserView, error) {
	results := make([]userview.UserView, 0, len(items))
	if len(items) == 0 {
		return results, nil
	}

	if s.subscriptionResolver == nil {
		for _, item := range items {
			view, err := s.applyTwoFactorView(ctx, userview.FromUser(item, nil))
			if err != nil {
				return nil, err
			}
			results = append(results, view)
		}
		return results, nil
	}

	userIDs := make([]uint, 0, len(items))
	for _, item := range items {
		userIDs = append(userIDs, item.ID)
	}

	mode, err := s.subscriptionResolver.GetBillingMode(ctx)
	if err != nil {
		return nil, err
	}
	if mode == "usage" {
		accounts, accountErr := s.subscriptionResolver.ListBillingAccountSnapshots(ctx, userIDs)
		if accountErr != nil {
			return nil, accountErr
		}
		for _, item := range items {
			account := accounts[item.ID]
			view, viewErr := s.applyTwoFactorView(ctx, userview.WithBillingAccount(userview.FromUser(item, nil), &userview.BillingAccountState{
				Currency:       account.Currency,
				BalanceNanousd: account.BalanceNanousd,
				Status:         account.Status,
			}))
			if viewErr != nil {
				return nil, viewErr
			}
			results = append(results, view)
		}
		return results, nil
	}

	subscriptions, err := s.subscriptionResolver.ListCurrentSubscriptionSnapshots(ctx, userIDs, time.Now())
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		subscription, ok := subscriptions[item.ID]
		if !ok {
			view, viewErr := s.applyTwoFactorView(ctx, userview.FromUser(item, nil))
			if viewErr != nil {
				return nil, viewErr
			}
			results = append(results, view)
			continue
		}

		view, viewErr := s.applyTwoFactorView(ctx, userview.FromUser(item, &userview.SubscriptionState{
			PlanID:    subscription.PlanID,
			PlanName:  subscription.PlanName,
			Tier:      subscription.Tier,
			Status:    subscription.Status,
			ExpiresAt: subscription.ExpiresAt,
		}))
		if viewErr != nil {
			return nil, viewErr
		}
		results = append(results, view)
	}

	return results, nil
}

func (s *Service) applyTwoFactorView(ctx context.Context, view userview.UserView) (userview.UserView, error) {
	if s.authSecurityService == nil {
		return view, nil
	}
	status, err := s.authSecurityService.GetCurrentTwoFactorStatus(ctx, view.ID)
	if err != nil {
		return userview.UserView{}, err
	}
	view.TwoFactorAvailable = status.Available
	view.TwoFactorEnabled = status.TOTPEnabled
	view.TwoFactorRequired = status.Required
	view.TwoFactorRecoveryCount = status.RecoveryCount
	return view, nil
}

// CreateUser 创建普通用户。
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
	subscriptionTier string,
	subscriptionExpiresAt *time.Time,
) (*domainuser.User, error) {
	billingMode := "self"
	if s.subscriptionResolver != nil {
		mode, err := s.subscriptionResolver.GetBillingMode(ctx)
		if err != nil {
			return nil, err
		}
		billingMode = mode
	}
	return s.userService.CreateUser(
		ctx,
		username,
		password,
		avatarURL,
		displayName,
		email,
		phone,
		timezone,
		locale,
		billingMode,
		subscriptionTier,
		subscriptionExpiresAt,
	)
}

// ResolveUserLabels 批量解析日志展示用的用户名称。
func (s *Service) ResolveUserLabels(ctx context.Context, userIDs []uint) map[uint]UserLabel {
	labels := make(map[uint]UserLabel)
	seen := make(map[uint]struct{})
	for _, userID := range userIDs {
		if userID == 0 {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		item, err := s.userService.GetByID(ctx, userID)
		if err != nil || item == nil {
			continue
		}
		label := strings.TrimSpace(item.DisplayName)
		if label == "" {
			label = strings.TrimSpace(item.Username)
		}
		if label == "" {
			label = strconv.FormatUint(uint64(userID), 10)
		}
		labels[userID] = UserLabel{
			ID:          userID,
			Username:    item.Username,
			DisplayName: item.DisplayName,
			Label:       label,
		}
	}
	return labels
}

// WriteAdminCreateUserAudit 记录管理员创建用户审计日志。
func (s *Service) WriteAdminCreateUserAudit(
	ctx context.Context,
	requestID string,
	actorUserID uint,
	createdUserID uint,
	username string,
	ip string,
	userAgent string,
) {
	s.auditService.Write(
		ctx,
		requestID,
		actorUserID,
		"admin_create_user",
		"user",
		strconv.FormatUint(uint64(createdUserID), 10),
		ip,
		userAgent,
		map[string]string{"username": username},
	)
}

// RevokeUserSessionsByAdmin 吊销指定用户全部会话。
func (s *Service) RevokeUserSessionsByAdmin(
	ctx context.Context,
	requestID string,
	actorUserID uint,
	targetUserID uint,
	ip string,
	userAgent string,
) error {
	actorUser, err := s.getActorUser(ctx, actorUserID)
	if err != nil {
		return err
	}
	targetUser, err := s.userService.GetByID(ctx, targetUserID)
	if err != nil {
		return err
	}
	if err = ensureActorCanManageTarget(actorUser, targetUser); err != nil {
		return err
	}

	if err := s.userService.RevokeAllSessions(ctx, targetUserID, "admin_revoke_all_sessions"); err != nil {
		return err
	}

	s.auditService.Write(
		ctx,
		requestID,
		actorUserID,
		"admin_revoke_user_sessions",
		"user",
		strconv.FormatUint(uint64(targetUserID), 10),
		ip,
		userAgent,
		map[string]string{"target_user_id": strconv.FormatUint(uint64(targetUserID), 10)},
	)

	return nil
}

// UpdateUserStatusByAdmin 修改普通用户状态。
func (s *Service) UpdateUserStatusByAdmin(
	ctx context.Context,
	requestID string,
	actorUserID uint,
	targetUserID uint,
	status string,
	reason string,
	ip string,
	userAgent string,
) (*domainuser.User, error) {
	nextStatus := strings.TrimSpace(status)
	if !isManageableStatus(nextStatus) {
		return nil, ErrInvalidUserStatus
	}

	targetUser, err := s.userService.GetByID(ctx, targetUserID)
	if err != nil {
		return nil, err
	}
	actorUser, err := s.getActorUser(ctx, actorUserID)
	if err != nil {
		return nil, err
	}
	if err = ensureActorCanManageTarget(actorUser, targetUser); err != nil {
		return nil, err
	}
	if targetUser.Role == domainuser.RoleSuperAdmin {
		return nil, ErrSuperAdminStatusChangeNotAllowed
	}

	if err = s.userService.UpdateUserStatus(ctx, targetUserID, nextStatus); err != nil {
		return nil, err
	}

	if nextStatus == domainuser.StatusActive {
		if err = s.userService.ResetLoginFailure(ctx, targetUserID); err != nil {
			return nil, err
		}
	} else {
		if err = s.userService.RevokeAllSessions(ctx, targetUserID, "admin_set_status_"+nextStatus); err != nil {
			return nil, err
		}
	}

	updatedUser, err := s.userService.GetByID(ctx, targetUserID)
	if err != nil {
		return nil, err
	}

	s.auditService.Write(
		ctx,
		requestID,
		actorUserID,
		"admin_update_user_status",
		"user",
		strconv.FormatUint(uint64(targetUserID), 10),
		ip,
		userAgent,
		map[string]string{
			"from_status": targetUser.Status,
			"to_status":   nextStatus,
			"reason":      strings.TrimSpace(reason),
		},
	)

	return updatedUser, nil
}

func isManageableStatus(status string) bool {
	switch status {
	case domainuser.StatusActive, domainuser.StatusLocked, domainuser.StatusSuspended, domainuser.StatusDeactivated:
		return true
	default:
		return false
	}
}

func isManageableRole(role string) bool {
	switch role {
	case domainuser.RoleUser, domainuser.RoleAdmin, domainuser.RoleSuperAdmin:
		return true
	default:
		return false
	}
}

func (s *Service) getActorUser(ctx context.Context, actorUserID uint) (*domainuser.User, error) {
	actorUser, err := s.userService.GetByID(ctx, actorUserID)
	if err != nil {
		return nil, err
	}
	if !domainuser.IsAdminRole(actorUser.Role) {
		return nil, ErrAdminPermissionRequired
	}
	return actorUser, nil
}

func ensureActorCanManageTarget(actorUser *domainuser.User, targetUser *domainuser.User) error {
	if actorUser.Role != domainuser.RoleSuperAdmin && targetUser.Role == domainuser.RoleSuperAdmin {
		return ErrSuperAdminManagementNotAllowed
	}
	return nil
}

// PatchUserByAdmin 统一维护头像、角色、状态和时区等可编辑字段。
func (s *Service) PatchUserByAdmin(
	ctx context.Context,
	requestID string,
	actorUserID uint,
	targetUserID uint,
	req PatchUserInput,
	ip string,
	userAgent string,
) (*domainuser.User, error) {
	targetUser, err := s.userService.GetByID(ctx, targetUserID)
	if err != nil {
		return nil, err
	}
	actorUser, err := s.getActorUser(ctx, actorUserID)
	if err != nil {
		return nil, err
	}
	if err = ensureActorCanManageTarget(actorUser, targetUser); err != nil {
		return nil, err
	}

	updateInput := repository.UpdateUserFieldsInput{}
	auditDetail := make(map[string]string)
	roleChanged := false

	if req.AvatarURL != nil {
		nextAvatarURL := strings.TrimSpace(*req.AvatarURL)
		if nextAvatarURL != targetUser.AvatarURL {
			updateInput.AvatarURL = &nextAvatarURL
			auditDetail["from_avatar_url"] = targetUser.AvatarURL
			auditDetail["to_avatar_url"] = nextAvatarURL
			targetUser.AvatarURL = nextAvatarURL
		}
	}

	if req.DisplayName != nil {
		nextDisplayName, normalizeErr := userapp.NormalizeDisplayName(*req.DisplayName)
		if normalizeErr != nil {
			return nil, normalizeErr
		}
		if nextDisplayName != targetUser.DisplayName {
			updateInput.DisplayName = &nextDisplayName
			auditDetail["from_display_name"] = targetUser.DisplayName
			auditDetail["to_display_name"] = nextDisplayName
			targetUser.DisplayName = nextDisplayName
		}
	}

	if req.Email != nil {
		nextEmail, normalizeErr := userapp.NormalizeEmail(*req.Email)
		if normalizeErr != nil {
			return nil, ErrInvalidUserEmail
		}
		if nextEmail != targetUser.Email {
			var emailVerifiedAt *time.Time
			emailSource := domainuser.EmailSourceAdminSet
			updateInput.Email = &nextEmail
			updateInput.EmailVerifiedAt = &emailVerifiedAt
			updateInput.EmailSource = &emailSource
			auditDetail["from_email"] = targetUser.Email
			auditDetail["to_email"] = nextEmail
			if targetUser.EmailVerifiedAt != nil {
				auditDetail["email_verification_reset"] = "true"
			}
			targetUser.Email = nextEmail
			targetUser.EmailVerifiedAt = nil
		}
	}

	if req.Phone != nil {
		nextPhone, normalizeErr := userapp.NormalizePhone(*req.Phone)
		if normalizeErr != nil {
			return nil, ErrInvalidUserPhone
		}
		if nextPhone != targetUser.Phone {
			var phoneVerifiedAt *time.Time
			updateInput.Phone = &nextPhone
			updateInput.PhoneVerifiedAt = &phoneVerifiedAt
			auditDetail["from_phone"] = targetUser.Phone
			auditDetail["to_phone"] = nextPhone
			if targetUser.PhoneVerifiedAt != nil {
				auditDetail["phone_verification_reset"] = "true"
			}
			targetUser.Phone = nextPhone
			targetUser.PhoneVerifiedAt = nil
		}
	}

	if req.Role != nil {
		nextRole := strings.TrimSpace(*req.Role)
		if !isManageableRole(nextRole) {
			return nil, ErrInvalidUserRole
		}
		if nextRole != targetUser.Role {
			if actorUserID == targetUserID {
				return nil, ErrSelfRoleChangeNotAllowed
			}
			if nextRole == domainuser.RoleSuperAdmin && actorUser.Role != domainuser.RoleSuperAdmin {
				return nil, ErrSuperAdminManagementNotAllowed
			}

			superAdminCount, countErr := s.userService.CountSuperAdmins(ctx)
			if countErr != nil {
				return nil, countErr
			}
			if targetUser.Role == domainuser.RoleSuperAdmin && nextRole != domainuser.RoleSuperAdmin && superAdminCount <= 1 {
				return nil, ErrLastSuperAdminRoleChangeNotAllowed
			}
			updateInput.Role = &nextRole
			auditDetail["from_role"] = targetUser.Role
			auditDetail["to_role"] = nextRole
			targetUser.Role = nextRole
			roleChanged = true
		}
	}

	if req.Timezone != nil {
		nextTimezone := strings.TrimSpace(*req.Timezone)
		if nextTimezone == "" {
			nextTimezone = "Etc/UTC"
		}
		if _, tzErr := time.LoadLocation(nextTimezone); tzErr != nil {
			return nil, ErrInvalidUserTimeZone
		}
		if nextTimezone != targetUser.Timezone {
			updateInput.Timezone = &nextTimezone
			auditDetail["from_timezone"] = targetUser.Timezone
			auditDetail["to_timezone"] = nextTimezone
			targetUser.Timezone = nextTimezone
		}
	}

	if req.Locale != nil {
		nextLocale, normalizeErr := normalizeAdminLocale(*req.Locale)
		if normalizeErr != nil {
			return nil, ErrInvalidUserLocale
		}
		if nextLocale != targetUser.Locale {
			updateInput.Locale = &nextLocale
			auditDetail["from_locale"] = targetUser.Locale
			auditDetail["to_locale"] = nextLocale
			targetUser.Locale = nextLocale
		}
	}

	if req.ProfilePreferences != nil {
		nextProfilePreferences := strings.TrimSpace(*req.ProfilePreferences)
		if nextProfilePreferences != targetUser.ProfilePreferences {
			updateInput.ProfilePreferences = &nextProfilePreferences
			auditDetail["from_profile_preferences"] = targetUser.ProfilePreferences
			auditDetail["to_profile_preferences"] = nextProfilePreferences
			targetUser.ProfilePreferences = nextProfilePreferences
		}
	}

	if req.SubscriptionTier != nil || req.SubscriptionExpiresAt != nil {
		if s.subscriptionResolver == nil {
			return nil, billing.ErrPaymentRequired
		}
		billingMode, modeErr := s.subscriptionResolver.GetBillingMode(ctx)
		if modeErr != nil {
			return nil, modeErr
		}
		if billingMode != "period" {
			return nil, billing.ErrPaymentRequired
		}

		now := time.Now()
		currentSubscription, snapshotErr := s.subscriptionResolver.GetCurrentSubscriptionSnapshot(ctx, targetUserID, now)
		if snapshotErr != nil {
			return nil, snapshotErr
		}

		nextTier := ""
		if currentSubscription != nil {
			nextTier = currentSubscription.Tier
		}
		if req.SubscriptionTier != nil {
			nextTier = strings.ToLower(strings.TrimSpace(*req.SubscriptionTier))
		}
		if nextTier == "" {
			nextTier = "free"
		}

		nextExpiresAt := req.SubscriptionExpiresAt
		if req.SubscriptionExpiresAt == nil && currentSubscription != nil {
			nextExpiresAt = currentSubscription.ExpiresAt
		}

		fromTier := "free"
		var fromExpiresAt string
		if currentSubscription != nil {
			fromTier = strings.TrimSpace(currentSubscription.Tier)
			if fromTier == "" {
				fromTier = "free"
			}
			if currentSubscription.ExpiresAt != nil {
				fromExpiresAt = currentSubscription.ExpiresAt.UTC().Format(time.RFC3339Nano)
			}
		}
		toExpiresAt := ""
		if nextExpiresAt != nil {
			toExpiresAt = nextExpiresAt.UTC().Format(time.RFC3339Nano)
		}

		if fromTier != nextTier || fromExpiresAt != toExpiresAt {
			updatedSubscription, updateErr := s.subscriptionResolver.SetUserSubscriptionByPlanCode(ctx, targetUserID, nextTier, nextExpiresAt)
			if updateErr != nil {
				return nil, updateErr
			}

			auditDetail["from_subscription_tier"] = fromTier
			auditDetail["to_subscription_tier"] = nextTier
			auditDetail["from_subscription_expires_at"] = fromExpiresAt
			if updatedSubscription != nil && updatedSubscription.ExpiresAt != nil {
				auditDetail["to_subscription_expires_at"] = updatedSubscription.ExpiresAt.UTC().Format(time.RFC3339Nano)
			} else {
				auditDetail["to_subscription_expires_at"] = ""
			}
		}
	}

	if req.Status != nil {
		nextStatus := strings.TrimSpace(*req.Status)
		if !isManageableStatus(nextStatus) {
			return nil, ErrInvalidUserStatus
		}
		if nextStatus != targetUser.Status {
			if actorUserID == targetUserID {
				return nil, ErrSelfStatusChangeNotAllowed
			}
			if targetUser.Role == domainuser.RoleSuperAdmin {
				return nil, ErrSuperAdminStatusChangeNotAllowed
			}

			if err = s.userService.UpdateUserStatus(ctx, targetUserID, nextStatus); err != nil {
				return nil, err
			}
			if nextStatus == domainuser.StatusActive {
				if err = s.userService.ResetLoginFailure(ctx, targetUserID); err != nil {
					return nil, err
				}
			} else {
				if err = s.userService.RevokeAllSessions(ctx, targetUserID, "admin_set_status_"+nextStatus); err != nil {
					return nil, err
				}
			}

			auditDetail["from_status"] = targetUser.Status
			auditDetail["to_status"] = nextStatus
			targetUser.Status = nextStatus
		}
	}

	if !updateInput.IsZero() {
		targetUser, err = s.userService.UpdateFields(ctx, targetUserID, updateInput)
		if err != nil {
			if errors.Is(err, repository.ErrLastSuperAdminRoleChange) {
				return nil, ErrLastSuperAdminRoleChangeNotAllowed
			}
			return nil, err
		}
		if roleChanged {
			if err = s.userService.RevokeAllSessions(ctx, targetUserID, "admin_set_role_"+targetUser.Role); err != nil {
				return nil, err
			}
			auditDetail["sessions_revoked"] = "true"
		}
	}

	if len(auditDetail) == 0 {
		return nil, ErrEmptyAdminUserPatch
	}

	if reason := strings.TrimSpace(req.Reason); reason != "" {
		auditDetail["reason"] = reason
	}
	s.auditService.Write(
		ctx,
		requestID,
		actorUserID,
		"admin_patch_user",
		"user",
		strconv.FormatUint(uint64(targetUserID), 10),
		ip,
		userAgent,
		auditDetail,
	)

	return s.userService.GetByID(ctx, targetUserID)
}

func normalizeAdminLocale(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "en-US", nil
	}

	normalized := strings.ReplaceAll(trimmed, "_", "-")
	parts := strings.Split(normalized, "-")
	if len(parts) == 0 || len(parts) > 2 {
		return "", ErrInvalidUserLocale
	}

	languagePart := strings.ToLower(parts[0])
	if len(languagePart) < 2 || len(languagePart) > 3 || !isASCIIAlpha(languagePart) {
		return "", ErrInvalidUserLocale
	}

	if len(parts) == 1 {
		return languagePart, nil
	}

	regionPart := strings.ToUpper(parts[1])
	if len(regionPart) != 2 || !isASCIIAlpha(regionPart) {
		return "", ErrInvalidUserLocale
	}

	return languagePart + "-" + regionPart, nil
}

func isASCIIAlpha(value string) bool {
	for _, r := range value {
		if !unicode.IsLetter(r) || r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// ResetUserPasswordByAdmin 重置用户密码并吊销全部会话。
func (s *Service) ResetUserPasswordByAdmin(
	ctx context.Context,
	requestID string,
	actorUserID uint,
	targetUserID uint,
	newPassword string,
	mustResetPassword bool,
	ip string,
	userAgent string,
) error {
	targetUser, err := s.userService.GetByID(ctx, targetUserID)
	if err != nil {
		return err
	}
	actorUser, err := s.getActorUser(ctx, actorUserID)
	if err != nil {
		return err
	}
	if err = ensureActorCanManageTarget(actorUser, targetUser); err != nil {
		return err
	}
	if targetUser.Role == domainuser.RoleSuperAdmin {
		return ErrSuperAdminPasswordResetNotAllowed
	}

	if err = s.userService.ResetPasswordByAdmin(ctx, targetUserID, newPassword, mustResetPassword); err != nil {
		return err
	}
	if err = s.userService.RevokeAllSessions(ctx, targetUserID, "admin_reset_password"); err != nil {
		return err
	}

	detailJSON := ""
	if payload, marshalErr := json.Marshal(map[string]string{
		"actor_user_id":       strconv.FormatUint(uint64(actorUserID), 10),
		"must_reset_password": strconv.FormatBool(mustResetPassword),
	}); marshalErr == nil {
		detailJSON = string(payload)
	}

	_ = s.userService.RecordAuthEvent(
		ctx,
		targetUserID,
		requestID,
		"password_reset",
		"success",
		"admin_reset_password",
		ip,
		userAgent,
		detailJSON,
	)

	s.auditService.Write(
		ctx,
		requestID,
		actorUserID,
		"admin_reset_user_password",
		"user",
		strconv.FormatUint(uint64(targetUserID), 10),
		ip,
		userAgent,
		map[string]string{
			"must_reset_password": strconv.FormatBool(mustResetPassword),
		},
	)

	return nil
}

func (s *Service) ResetUserTwoFactorByAdmin(
	ctx context.Context,
	requestID string,
	actorUserID uint,
	targetUserID uint,
	ip string,
	userAgent string,
) error {
	targetUser, err := s.userService.GetByID(ctx, targetUserID)
	if err != nil {
		return err
	}
	actorUser, err := s.getActorUser(ctx, actorUserID)
	if err != nil {
		return err
	}
	if err = ensureActorCanManageTarget(actorUser, targetUser); err != nil {
		return err
	}
	if targetUser.Role == domainuser.RoleSuperAdmin {
		return ErrSuperAdminTwoFactorResetNotAllowed
	}
	if s.authSecurityService == nil {
		return nil
	}
	if err = s.authSecurityService.ResetUserTwoFactorByAdmin(ctx, targetUserID); err != nil {
		return err
	}
	if err = s.userService.RevokeAllSessions(ctx, targetUserID, "admin_reset_2fa"); err != nil {
		return err
	}
	s.auditService.Write(
		ctx,
		requestID,
		actorUserID,
		"admin_reset_user_2fa",
		"user",
		strconv.FormatUint(uint64(targetUserID), 10),
		ip,
		userAgent,
		map[string]string{"target_user_id": strconv.FormatUint(uint64(targetUserID), 10)},
	)
	return nil
}

// DeleteUserByAdmin 删除指定普通用户及其主要用户域数据。
func (s *Service) DeleteUserByAdmin(
	ctx context.Context,
	requestID string,
	actorUserID uint,
	targetUserID uint,
	ip string,
	userAgent string,
) error {
	targetUser, err := s.userService.GetByID(ctx, targetUserID)
	if err != nil {
		return err
	}
	actorUser, err := s.getActorUser(ctx, actorUserID)
	if err != nil {
		return err
	}
	if err = ensureActorCanManageTarget(actorUser, targetUser); err != nil {
		return err
	}
	if actorUserID == targetUserID {
		return ErrSelfDeleteNotAllowed
	}
	if targetUser.Role == domainuser.RoleSuperAdmin {
		return ErrSuperAdminDeleteNotAllowed
	}

	if err = s.userService.DeleteAccountHard(ctx, targetUserID); err != nil {
		return err
	}

	s.auditService.Write(
		ctx,
		requestID,
		actorUserID,
		"admin_delete_user",
		"user",
		strconv.FormatUint(uint64(targetUserID), 10),
		ip,
		userAgent,
		map[string]string{
			"target_user_id": strconv.FormatUint(uint64(targetUserID), 10),
			"username":       targetUser.Username,
			"public_id":      targetUser.PublicID,
		},
	)

	return nil
}

// ListUserAuthEventsByAdmin 查询用户认证事件列表。
func (s *Service) ListUserAuthEventsByAdmin(
	ctx context.Context,
	userID uint,
	eventType string,
	result string,
	page int,
	pageSize int,
) ([]domainuser.AuthEvent, int64, error) {
	return s.userService.ListAuthEvents(ctx, userID, eventType, result, page, pageSize)
}
