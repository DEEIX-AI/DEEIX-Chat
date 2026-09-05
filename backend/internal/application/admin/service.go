package admin

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	auditapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
	authapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/auth"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	applogcleanup "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/logcleanup"
	systemeventapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/systemevent"
	userapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/userview"
	domainaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/audit"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainsystemevent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/systemevent"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type userService interface {
	ListUsers(ctx context.Context, page int, pageSize int, filter repository.UserListFilter) ([]domainuser.User, int64, error)
	ListIdentityProviders(ctx context.Context, includeDisabled bool) ([]domainuser.IdentityProvider, error)
	ListUserIdentitiesByUserIDs(ctx context.Context, userIDs []uint) (map[uint][]domainuser.UserIdentity, error)
	ListLatestSessionActivityByUserIDs(ctx context.Context, userIDs []uint) (map[uint]time.Time, error)
	CountSuperAdmins(ctx context.Context) (int64, error)
	CreateUser(ctx context.Context, input userapp.CreateUserInput) (*domainuser.User, error)
	GetByID(ctx context.Context, userID uint) (*domainuser.User, error)
	RevokeAllSessions(ctx context.Context, userID uint, reason string) error
	UpdateUserStatus(ctx context.Context, userID uint, status string) error
	UpdateFields(ctx context.Context, userID uint, input repository.UpdateUserFieldsInput) (*domainuser.User, error)
	ResetLoginFailure(ctx context.Context, userID uint) error
	ResetPasswordByAdmin(ctx context.Context, userID uint, newPassword string, mustResetPassword bool) error
	DeleteAccountHard(ctx context.Context, userID uint) error
	RecordAuthEvent(ctx context.Context, input repository.AuthEventInput) error
	ListAuthEvents(ctx context.Context, input userapp.AuthEventListInput) ([]domainuser.AuthEvent, int64, error)
}

type auditService interface {
	Write(ctx context.Context, input auditapp.WriteInput)
	List(ctx context.Context, page int, pageSize int, filter auditapp.ListFilter) ([]domainaudit.Log, int64, error)
}

type systemEventService interface {
	List(ctx context.Context, page int, pageSize int, filter systemeventapp.ListFilter) ([]domainsystemevent.Event, int64, error)
}

type usageLogService interface {
	ListUsageLogs(ctx context.Context, page int, pageSize int, filter billing.UsageLogListFilter) ([]domainbilling.UsageLedger, int64, error)
}

type usageStatisticsService interface {
	GetUsageStatistics(ctx context.Context, filter billing.UsageStatisticsFilter) (domainbilling.UsageStatistics, error)
}

type orderLogService interface {
	ListPaymentOrders(ctx context.Context, page int, pageSize int, filter billing.PaymentOrderListFilter) ([]domainbilling.PaymentOrder, int64, error)
	ListRedemptions(ctx context.Context, page int, pageSize int, filter billing.RedemptionListFilter) ([]billing.RedemptionRecordView, int64, error)
}

type conversationEventService interface {
	ListConversationEventLogs(ctx context.Context, page int, pageSize int, filter appconversation.EventLogListFilter) ([]domainconversation.EventLog, int64, error)
	GetConversationEventLog(ctx context.Context, eventID uint) (*domainconversation.EventLog, error)
}

type logCleanupService interface {
	Cleanup(ctx context.Context, input applogcleanup.Input) (*applogcleanup.Result, error)
	CleanupConversationRuns(ctx context.Context, input applogcleanup.ConversationRunInput) (*applogcleanup.ConversationRunResult, error)
}

type authSecurityService interface {
	GetCurrentTwoFactorStatus(ctx context.Context, userID uint) (*authapp.TwoFactorStatusResult, error)
	ResetUserTwoFactorByAdmin(ctx context.Context, userID uint) error
}

// Service 聚合后台域服务依赖。
type Service struct {
	userService                                userService
	auditService                               auditService
	systemEventService                         systemEventService
	usageLogService                            usageLogService
	usageStatisticsService                     usageStatisticsService
	orderLogService                            orderLogService
	conversationEventSvc                       conversationEventService
	logCleanupService                          logCleanupService
	authSecurityService                        authSecurityService
	subscriptionResolver                       subscriptionResolver
	openWebUIRowLoader                         openWebUIRowLoader
	permissionGroupRepo                        permissionGroupRepo
	permissionGroupModelLookup                 permissionGroupModelLookup
	permissionGroupBillingPlanReferenceChecker permissionGroupBillingPlanReferenceChecker
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

// CreateUserInput 描述管理员创建普通用户时允许提交的账号与订阅信息。
// 计费模式由后台配置决定，不由请求方传入。
type CreateUserInput struct {
	Username              string
	Password              string
	AvatarURL             string
	DisplayName           string
	Email                 string
	Phone                 string
	Timezone              string
	Locale                string
	SubscriptionTier      string
	SubscriptionExpiresAt *time.Time
}

// CreateUserAuditInput 描述管理员创建用户审计日志所需的上下文。
type CreateUserAuditInput struct {
	RequestID     string
	ActorUserID   uint
	CreatedUserID uint
	Username      string
	IP            string
	UserAgent     string
}

// UpdateUserStatusInput 描述管理员修改用户状态时的目标与审计信息。
type UpdateUserStatusInput struct {
	RequestID    string
	ActorUserID  uint
	TargetUserID uint
	Status       string
	Reason       string
	IP           string
	UserAgent    string
}

// PatchUserByAdminInput 描述管理员局部更新用户时的目标与审计信息。
type PatchUserByAdminInput struct {
	RequestID    string
	ActorUserID  uint
	TargetUserID uint
	Patch        PatchUserInput
	IP           string
	UserAgent    string
}

// ResetUserPasswordInput 描述管理员重置用户密码时的目标与审计信息。
type ResetUserPasswordInput struct {
	RequestID         string
	ActorUserID       uint
	TargetUserID      uint
	NewPassword       string
	MustResetPassword bool
	IP                string
	UserAgent         string
}

// NewService 创建服务。
func NewService(userService userService, auditService auditService) *Service {
	return &Service{
		userService:  userService,
		auditService: auditService,
	}
}

// SetOpenWebUIRowLoader 注入 OpenWebUI 外部数据读取能力。
func (s *Service) SetOpenWebUIRowLoader(loader openWebUIRowLoader) {
	s.openWebUIRowLoader = loader
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

// SetUsageStatisticsService 注入管理员用量统计能力。
func (s *Service) SetUsageStatisticsService(service usageStatisticsService) {
	s.usageStatisticsService = service
}

// SetOrderLogService 注入支付订单日志查询能力。
func (s *Service) SetOrderLogService(service orderLogService) {
	s.orderLogService = service
}

// SetConversationEventService 注入对话事件查询能力。
func (s *Service) SetConversationEventService(service conversationEventService) {
	s.conversationEventSvc = service
}

// SetLogCleanupService 注入日志清理能力。
func (s *Service) SetLogCleanupService(service logCleanupService) {
	s.logCleanupService = service
}

// SetSubscriptionResolver 注入订阅派生解析能力。
func (s *Service) SetSubscriptionResolver(resolver subscriptionResolver) {
	s.subscriptionResolver = resolver
}

// ListUsers 查询用户分页列表。
func (s *Service) ListUsers(ctx context.Context, page int, pageSize int, filter UserListFilter) ([]userview.UserView, int64, error) {
	items, total, err := s.userService.ListUsers(ctx, page, pageSize, repository.UserListFilter{
		Query:              filter.Query,
		SubscriptionStatus: filter.SubscriptionStatus,
		IdentityProvider:   filter.IdentityProvider,
	})
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
		view, err := s.applyTwoFactorView(ctx, userview.FromUser(item, nil))
		if err != nil {
			return userview.UserView{}, err
		}
		return s.completeUserView(ctx, view)
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
		view := userViewFromMode(item, nil, accounts[item.ID], true)
		view, err = s.applyTwoFactorView(ctx, view)
		if err != nil {
			return userview.UserView{}, err
		}
		return s.completeUserView(ctx, view)
	}

	subscription, err := s.subscriptionResolver.GetCurrentSubscriptionSnapshot(ctx, item.ID, time.Now())
	if err != nil {
		return userview.UserView{}, err
	}
	account := billing.UserBillingAccountSnapshot{}
	includeAccount := false
	if mode == "period" {
		accounts, accountErr := s.subscriptionResolver.ListBillingAccountSnapshots(ctx, []uint{item.ID})
		if accountErr != nil {
			return userview.UserView{}, accountErr
		}
		account = accounts[item.ID]
		includeAccount = true
	}
	if subscription == nil {
		view, err := s.applyTwoFactorView(ctx, userViewFromMode(item, nil, account, includeAccount))
		if err != nil {
			return userview.UserView{}, err
		}
		return s.completeUserView(ctx, view)
	}

	view := userViewFromMode(item, subscription, account, includeAccount)
	view, err = s.applyTwoFactorView(ctx, view)
	if err != nil {
		return userview.UserView{}, err
	}
	return s.completeUserView(ctx, view)
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
		return s.completeUserViews(ctx, results)
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
			view, viewErr := s.applyTwoFactorView(ctx, userViewFromMode(item, nil, accounts[item.ID], true))
			if viewErr != nil {
				return nil, viewErr
			}
			results = append(results, view)
		}
		return s.completeUserViews(ctx, results)
	}

	subscriptions, err := s.subscriptionResolver.ListCurrentSubscriptionSnapshots(ctx, userIDs, time.Now())
	if err != nil {
		return nil, err
	}
	accounts := map[uint]billing.UserBillingAccountSnapshot{}
	if mode == "period" {
		accounts, err = s.subscriptionResolver.ListBillingAccountSnapshots(ctx, userIDs)
		if err != nil {
			return nil, err
		}
	}

	for _, item := range items {
		subscription, _ := subscriptions[item.ID]
		var snapshot *billing.UserSubscriptionSnapshot
		if subscription.PlanID != nil || strings.TrimSpace(subscription.PlanName) != "" || strings.TrimSpace(subscription.Tier) != "" || strings.TrimSpace(subscription.Status) != "" || subscription.ExpiresAt != nil {
			snapshot = &subscription
		}
		view, viewErr := s.applyTwoFactorView(ctx, userViewFromMode(item, snapshot, accounts[item.ID], mode == "period"))
		if viewErr != nil {
			return nil, viewErr
		}
		results = append(results, view)
	}

	return s.completeUserViews(ctx, results)
}

func (s *Service) completeUserView(ctx context.Context, view userview.UserView) (userview.UserView, error) {
	views, err := s.applyIdentityProviderViews(ctx, []userview.UserView{view})
	if err != nil {
		return userview.UserView{}, err
	}
	return s.applyLastActiveView(ctx, views[0])
}

func (s *Service) completeUserViews(ctx context.Context, views []userview.UserView) ([]userview.UserView, error) {
	views, err := s.applyIdentityProviderViews(ctx, views)
	if err != nil {
		return nil, err
	}
	return s.applyLastActiveViews(ctx, views)
}

func (s *Service) applyLastActiveView(ctx context.Context, view userview.UserView) (userview.UserView, error) {
	activities, err := s.userService.ListLatestSessionActivityByUserIDs(ctx, []uint{view.ID})
	if err != nil {
		return userview.UserView{}, err
	}
	if value, ok := activities[view.ID]; ok {
		return userview.WithLastActiveAt(view, &value), nil
	}
	return view, nil
}

func userViewFromMode(
	item domainuser.User,
	subscription *billing.UserSubscriptionSnapshot,
	account billing.UserBillingAccountSnapshot,
	includeAccount bool,
) userview.UserView {
	var subscriptionState *userview.SubscriptionState
	if subscription != nil {
		subscriptionState = &userview.SubscriptionState{
			PlanID:    subscription.PlanID,
			PlanName:  subscription.PlanName,
			Tier:      subscription.Tier,
			Status:    subscription.Status,
			ExpiresAt: subscription.ExpiresAt,
		}
	}
	view := userview.FromUser(item, subscriptionState)
	if !includeAccount {
		return view
	}
	return userview.WithBillingAccount(view, &userview.BillingAccountState{
		Currency:       account.Currency,
		BalanceNanousd: account.BalanceNanousd,
		Status:         account.Status,
	})
}

func (s *Service) applyLastActiveViews(ctx context.Context, views []userview.UserView) ([]userview.UserView, error) {
	if len(views) == 0 {
		return views, nil
	}
	userIDs := make([]uint, 0, len(views))
	for _, view := range views {
		userIDs = append(userIDs, view.ID)
	}
	activities, err := s.userService.ListLatestSessionActivityByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	for index, view := range views {
		if value, ok := activities[view.ID]; ok {
			views[index] = userview.WithLastActiveAt(view, &value)
		}
	}
	return views, nil
}

func (s *Service) applyIdentityProviderViews(ctx context.Context, views []userview.UserView) ([]userview.UserView, error) {
	if len(views) == 0 {
		return views, nil
	}
	userIDs := make([]uint, 0, len(views))
	for _, view := range views {
		userIDs = append(userIDs, view.ID)
	}
	identitiesByUserID, err := s.userService.ListUserIdentitiesByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	if len(identitiesByUserID) == 0 {
		return views, nil
	}
	providers, err := s.userService.ListIdentityProviders(ctx, true)
	if err != nil {
		return nil, err
	}
	providerByID := make(map[uint]domainuser.IdentityProvider, len(providers))
	for _, provider := range providers {
		providerByID[provider.ID] = provider
	}
	for index, view := range views {
		identities := identitiesByUserID[view.ID]
		if len(identities) == 0 {
			continue
		}
		summaries := make([]userview.IdentityProviderSummary, 0, len(identities))
		seenProviderIDs := make(map[uint]struct{}, len(identities))
		for _, identity := range identities {
			provider, ok := providerByID[identity.ProviderID]
			if !ok {
				continue
			}
			if _, seen := seenProviderIDs[provider.ID]; seen {
				continue
			}
			seenProviderIDs[provider.ID] = struct{}{}
			summaries = append(summaries, userview.IdentityProviderSummary{
				ID:      provider.ID,
				Type:    provider.Type,
				Name:    provider.Name,
				Slug:    provider.Slug,
				LogoURL: provider.LogoURL,
			})
		}
		views[index] = userview.WithIdentityProviders(view, summaries)
	}
	return views, nil
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
func (s *Service) CreateUser(ctx context.Context, input CreateUserInput) (*domainuser.User, error) {
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
		userapp.CreateUserInput{
			Username:              input.Username,
			Password:              input.Password,
			AvatarURL:             input.AvatarURL,
			DisplayName:           input.DisplayName,
			Email:                 input.Email,
			Phone:                 input.Phone,
			Timezone:              input.Timezone,
			Locale:                input.Locale,
			BillingMode:           billingMode,
			SubscriptionTier:      input.SubscriptionTier,
			SubscriptionExpiresAt: input.SubscriptionExpiresAt,
		},
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
func (s *Service) WriteAdminCreateUserAudit(ctx context.Context, input CreateUserAuditInput) {
	s.auditService.Write(ctx, auditapp.WriteInput{
		RequestID:   input.RequestID,
		ActorUserID: input.ActorUserID,
		Action:      "admin_create_user",
		Resource:    "user",
		ResourceID:  strconv.FormatUint(uint64(input.CreatedUserID), 10),
		IP:          input.IP,
		UserAgent:   input.UserAgent,
		Detail:      map[string]string{"username": input.Username},
	})
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

	s.auditService.Write(ctx, auditapp.WriteInput{
		RequestID:   requestID,
		ActorUserID: actorUserID,
		Action:      "admin_revoke_user_sessions",
		Resource:    "user",
		ResourceID:  strconv.FormatUint(uint64(targetUserID), 10),
		IP:          ip,
		UserAgent:   userAgent,
		Detail:      map[string]string{"target_user_id": strconv.FormatUint(uint64(targetUserID), 10)},
	})

	return nil
}

// UpdateUserStatusByAdmin 修改普通用户状态。
func (s *Service) UpdateUserStatusByAdmin(ctx context.Context, input UpdateUserStatusInput) (*domainuser.User, error) {
	nextStatus := strings.TrimSpace(input.Status)
	if !isManageableStatus(nextStatus) {
		return nil, ErrInvalidUserStatus
	}

	targetUser, err := s.userService.GetByID(ctx, input.TargetUserID)
	if err != nil {
		return nil, err
	}
	actorUser, err := s.getActorUser(ctx, input.ActorUserID)
	if err != nil {
		return nil, err
	}
	if err = ensureActorCanManageTarget(actorUser, targetUser); err != nil {
		return nil, err
	}
	if targetUser.Role == domainuser.RoleSuperAdmin {
		return nil, ErrSuperAdminStatusChangeNotAllowed
	}

	if err = s.userService.UpdateUserStatus(ctx, input.TargetUserID, nextStatus); err != nil {
		return nil, err
	}

	if nextStatus == domainuser.StatusActive {
		if err = s.userService.ResetLoginFailure(ctx, input.TargetUserID); err != nil {
			return nil, err
		}
	} else {
		if err = s.userService.RevokeAllSessions(ctx, input.TargetUserID, "admin_set_status_"+nextStatus); err != nil {
			return nil, err
		}
	}

	updatedUser, err := s.userService.GetByID(ctx, input.TargetUserID)
	if err != nil {
		return nil, err
	}

	s.auditService.Write(ctx, auditapp.WriteInput{
		RequestID:   input.RequestID,
		ActorUserID: input.ActorUserID,
		Action:      "admin_update_user_status",
		Resource:    "user",
		ResourceID:  strconv.FormatUint(uint64(input.TargetUserID), 10),
		IP:          input.IP,
		UserAgent:   input.UserAgent,
		Detail: map[string]string{
			"from_status": targetUser.Status,
			"to_status":   nextStatus,
			"reason":      strings.TrimSpace(input.Reason),
		},
	})

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
func (s *Service) PatchUserByAdmin(ctx context.Context, input PatchUserByAdminInput) (*domainuser.User, error) {
	targetUser, err := s.userService.GetByID(ctx, input.TargetUserID)
	if err != nil {
		return nil, err
	}
	actorUser, err := s.getActorUser(ctx, input.ActorUserID)
	if err != nil {
		return nil, err
	}
	if err = ensureActorCanManageTarget(actorUser, targetUser); err != nil {
		return nil, err
	}

	updateInput := repository.UpdateUserFieldsInput{}
	auditDetail := make(map[string]string)
	roleChanged := false

	req := input.Patch
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
			if input.ActorUserID == input.TargetUserID {
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
		currentSubscription, snapshotErr := s.subscriptionResolver.GetCurrentSubscriptionSnapshot(ctx, input.TargetUserID, now)
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
			updatedSubscription, updateErr := s.subscriptionResolver.SetUserSubscriptionByPlanCode(ctx, input.TargetUserID, nextTier, nextExpiresAt)
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
			if input.ActorUserID == input.TargetUserID {
				return nil, ErrSelfStatusChangeNotAllowed
			}
			if targetUser.Role == domainuser.RoleSuperAdmin {
				return nil, ErrSuperAdminStatusChangeNotAllowed
			}

			if err = s.userService.UpdateUserStatus(ctx, input.TargetUserID, nextStatus); err != nil {
				return nil, err
			}
			if nextStatus == domainuser.StatusActive {
				if err = s.userService.ResetLoginFailure(ctx, input.TargetUserID); err != nil {
					return nil, err
				}
			} else {
				if err = s.userService.RevokeAllSessions(ctx, input.TargetUserID, "admin_set_status_"+nextStatus); err != nil {
					return nil, err
				}
			}

			auditDetail["from_status"] = targetUser.Status
			auditDetail["to_status"] = nextStatus
			targetUser.Status = nextStatus
		}
	}

	if !updateInput.IsZero() {
		targetUser, err = s.userService.UpdateFields(ctx, input.TargetUserID, updateInput)
		if err != nil {
			if errors.Is(err, repository.ErrLastSuperAdminRoleChange) {
				return nil, ErrLastSuperAdminRoleChangeNotAllowed
			}
			return nil, err
		}
		if roleChanged {
			if err = s.userService.RevokeAllSessions(ctx, input.TargetUserID, "admin_set_role_"+targetUser.Role); err != nil {
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
	s.auditService.Write(ctx, auditapp.WriteInput{
		RequestID:   input.RequestID,
		ActorUserID: input.ActorUserID,
		Action:      "admin_patch_user",
		Resource:    "user",
		ResourceID:  strconv.FormatUint(uint64(input.TargetUserID), 10),
		IP:          input.IP,
		UserAgent:   input.UserAgent,
		Detail:      auditDetail,
	})

	return s.userService.GetByID(ctx, input.TargetUserID)
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
	if len(languagePart) < 2 || len(languagePart) > 3 || !textutil.IsASCIIAlpha(languagePart) {
		return "", ErrInvalidUserLocale
	}

	if len(parts) == 1 {
		return languagePart, nil
	}

	regionPart := strings.ToUpper(parts[1])
	if len(regionPart) != 2 || !textutil.IsASCIIAlpha(regionPart) {
		return "", ErrInvalidUserLocale
	}

	return languagePart + "-" + regionPart, nil
}

// ResetUserPasswordByAdmin 重置用户密码并吊销全部会话。
func (s *Service) ResetUserPasswordByAdmin(ctx context.Context, input ResetUserPasswordInput) error {
	targetUser, err := s.userService.GetByID(ctx, input.TargetUserID)
	if err != nil {
		return err
	}
	actorUser, err := s.getActorUser(ctx, input.ActorUserID)
	if err != nil {
		return err
	}
	if err = ensureActorCanManageTarget(actorUser, targetUser); err != nil {
		return err
	}
	if targetUser.Role == domainuser.RoleSuperAdmin {
		return ErrSuperAdminPasswordResetNotAllowed
	}

	if err = s.userService.ResetPasswordByAdmin(ctx, input.TargetUserID, input.NewPassword, input.MustResetPassword); err != nil {
		return err
	}
	if err = s.userService.RevokeAllSessions(ctx, input.TargetUserID, "admin_reset_password"); err != nil {
		return err
	}

	detailJSON := ""
	if payload, marshalErr := json.Marshal(map[string]string{
		"actor_user_id":       strconv.FormatUint(uint64(input.ActorUserID), 10),
		"must_reset_password": strconv.FormatBool(input.MustResetPassword),
	}); marshalErr == nil {
		detailJSON = string(payload)
	}

	_ = s.userService.RecordAuthEvent(
		ctx,
		repository.AuthEventInput{
			UserID:     input.TargetUserID,
			RequestID:  input.RequestID,
			EventType:  "password_reset",
			Result:     "success",
			Reason:     "admin_reset_password",
			ClientIP:   input.IP,
			UserAgent:  input.UserAgent,
			DetailJSON: detailJSON,
		},
	)

	s.auditService.Write(ctx, auditapp.WriteInput{
		RequestID:   input.RequestID,
		ActorUserID: input.ActorUserID,
		Action:      "admin_reset_user_password",
		Resource:    "user",
		ResourceID:  strconv.FormatUint(uint64(input.TargetUserID), 10),
		IP:          input.IP,
		UserAgent:   input.UserAgent,
		Detail: map[string]string{
			"must_reset_password": strconv.FormatBool(input.MustResetPassword),
		},
	})

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
	s.auditService.Write(ctx, auditapp.WriteInput{
		RequestID:   requestID,
		ActorUserID: actorUserID,
		Action:      "admin_reset_user_2fa",
		Resource:    "user",
		ResourceID:  strconv.FormatUint(uint64(targetUserID), 10),
		IP:          ip,
		UserAgent:   userAgent,
		Detail:      map[string]string{"target_user_id": strconv.FormatUint(uint64(targetUserID), 10)},
	})
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

	s.auditService.Write(ctx, auditapp.WriteInput{
		RequestID:   requestID,
		ActorUserID: actorUserID,
		Action:      "admin_delete_user",
		Resource:    "user",
		ResourceID:  strconv.FormatUint(uint64(targetUserID), 10),
		IP:          ip,
		UserAgent:   userAgent,
		Detail: map[string]string{
			"target_user_id": strconv.FormatUint(uint64(targetUserID), 10),
			"username":       targetUser.Username,
			"public_id":      targetUser.PublicID,
		},
	})

	return nil
}

// ListUserAuthEventsByAdmin 查询用户认证事件列表。
func (s *Service) ListUserAuthEventsByAdmin(ctx context.Context, input userapp.AuthEventListInput) ([]domainuser.AuthEvent, int64, error) {
	return s.userService.ListAuthEvents(ctx, input)
}

// WriteAuditLog 写通用审计日志。
func (s *Service) WriteAuditLog(ctx context.Context, input auditapp.WriteInput) {
	s.auditService.Write(ctx, input)
}
