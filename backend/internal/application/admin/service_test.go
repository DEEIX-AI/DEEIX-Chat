package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	auditapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	domainaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/audit"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestBuildUserViewsIncludesBillingBalanceOutsideSelfMode(t *testing.T) {
	users := newAdminUserServiceFake(map[uint]domainuser.User{
		7: {ID: 7, Username: "alice", Role: domainuser.RoleUser},
	})
	planID := uint(42)
	expiresAt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	service := NewService(users, auditServiceFake{})
	service.SetSubscriptionResolver(subscriptionResolverFake{
		billingMode: "period",
		subscriptions: map[uint]billing.UserSubscriptionSnapshot{
			7: {
				UserID:    7,
				PlanID:    &planID,
				PlanName:  "Pro Monthly",
				Tier:      "pro",
				Status:    "active",
				ExpiresAt: &expiresAt,
			},
		},
		accounts: map[uint]billing.UserBillingAccountSnapshot{
			7: {
				UserID:         7,
				Currency:       "USD",
				BalanceNanousd: 2_500_000_000,
				Status:         "active",
			},
		},
	})

	views, err := service.BuildUserViews(context.Background(), []domainuser.User{
		{ID: 7, Username: "alice", Role: domainuser.RoleUser},
	})
	if err != nil {
		t.Fatalf("expected build user views to succeed, got %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}
	if views[0].SubscriptionTier != "pro" {
		t.Fatalf("expected subscription tier to be preserved, got %q", views[0].SubscriptionTier)
	}
	if views[0].SubscriptionPlanName != "Pro Monthly" {
		t.Fatalf("expected subscription plan name to be preserved, got %q", views[0].SubscriptionPlanName)
	}
	if views[0].SubscriptionStatus != "active" {
		t.Fatalf("expected subscription status to be preserved, got %q", views[0].SubscriptionStatus)
	}
	if views[0].SubscriptionPlanID == nil || *views[0].SubscriptionPlanID != planID {
		t.Fatalf("expected subscription plan id %d, got %+v", planID, views[0].SubscriptionPlanID)
	}
	if views[0].SubscriptionExpiresAt == nil || !views[0].SubscriptionExpiresAt.Equal(expiresAt) {
		t.Fatalf("expected subscription expiration %v, got %+v", expiresAt, views[0].SubscriptionExpiresAt)
	}
	if views[0].BillingBalanceNanousd != 2_500_000_000 {
		t.Fatalf("expected billing balance nanousd to be preserved, got %d", views[0].BillingBalanceNanousd)
	}
	if views[0].BillingAccountCurrency != "USD" {
		t.Fatalf("expected billing currency to be preserved, got %q", views[0].BillingAccountCurrency)
	}
	if views[0].BillingAccountStatus != "active" {
		t.Fatalf("expected billing status to be preserved, got %q", views[0].BillingAccountStatus)
	}
}

func TestBuildUserViewsUsageModeKeepsAccountOnlyView(t *testing.T) {
	users := newAdminUserServiceFake(map[uint]domainuser.User{
		7: {ID: 7, Username: "alice", Role: domainuser.RoleUser},
	})
	service := NewService(users, auditServiceFake{})
	service.SetSubscriptionResolver(subscriptionResolverFake{
		billingMode: "usage",
		subscriptions: map[uint]billing.UserSubscriptionSnapshot{
			7: {
				UserID:   7,
				PlanName: "Should Not Be Used",
				Tier:     "pro",
				Status:   "active",
			},
		},
		accounts: map[uint]billing.UserBillingAccountSnapshot{
			7: {
				UserID:         7,
				Currency:       "USD",
				BalanceNanousd: 900_000_000,
				Status:         "active",
			},
		},
	})

	views, err := service.BuildUserViews(context.Background(), []domainuser.User{
		{ID: 7, Username: "alice", Role: domainuser.RoleUser},
	})
	if err != nil {
		t.Fatalf("expected build user views to succeed, got %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}
	if views[0].SubscriptionTier != "free" {
		t.Fatalf("expected usage mode to keep account-only subscription tier, got %q", views[0].SubscriptionTier)
	}
	if views[0].SubscriptionPlanName != "free" {
		t.Fatalf("expected usage mode to keep account-only plan name, got %q", views[0].SubscriptionPlanName)
	}
	if views[0].BillingBalanceNanousd != 900_000_000 {
		t.Fatalf("expected billing balance nanousd to be preserved, got %d", views[0].BillingBalanceNanousd)
	}
}

func TestPatchUserByAdminAllowsAdditionalSuperAdmin(t *testing.T) {
	users := newAdminUserServiceFake(map[uint]domainuser.User{
		1: {ID: 1, Role: domainuser.RoleSuperAdmin},
		2: {ID: 2, Role: domainuser.RoleUser},
	})
	service := NewService(users, auditServiceFake{})

	nextRole := domainuser.RoleSuperAdmin
	updated, err := service.PatchUserByAdmin(
		context.Background(),
		"req_1",
		1,
		2,
		PatchUserInput{Role: &nextRole},
		"127.0.0.1",
		"test",
	)
	if err != nil {
		t.Fatalf("expected second superadmin promotion to succeed, got %v", err)
	}
	if updated.Role != domainuser.RoleSuperAdmin {
		t.Fatalf("expected promoted role %q, got %q", domainuser.RoleSuperAdmin, updated.Role)
	}
}

func TestPatchUserByAdminKeepsLastSuperAdminProtected(t *testing.T) {
	count := int64(1)
	users := newAdminUserServiceFake(map[uint]domainuser.User{
		1: {ID: 1, Role: domainuser.RoleSuperAdmin},
		2: {ID: 2, Role: domainuser.RoleSuperAdmin},
	})
	users.superAdminCount = &count
	service := NewService(users, auditServiceFake{})

	nextRole := domainuser.RoleUser
	_, err := service.PatchUserByAdmin(
		context.Background(),
		"req_1",
		2,
		1,
		PatchUserInput{Role: &nextRole},
		"127.0.0.1",
		"test",
	)
	if !errors.Is(err, ErrLastSuperAdminRoleChangeNotAllowed) {
		t.Fatalf("expected last superadmin protection, got %v", err)
	}
}

func TestPatchUserByAdminAllowsAdminRole(t *testing.T) {
	users := newAdminUserServiceFake(map[uint]domainuser.User{
		1: {ID: 1, Role: domainuser.RoleAdmin},
		2: {ID: 2, Role: domainuser.RoleUser},
	})
	service := NewService(users, auditServiceFake{})

	nextRole := domainuser.RoleAdmin
	updated, err := service.PatchUserByAdmin(
		context.Background(),
		"req_1",
		1,
		2,
		PatchUserInput{Role: &nextRole},
		"127.0.0.1",
		"test",
	)
	if err != nil {
		t.Fatalf("expected admin role promotion to succeed, got %v", err)
	}
	if updated.Role != domainuser.RoleAdmin {
		t.Fatalf("expected promoted role %q, got %q", domainuser.RoleAdmin, updated.Role)
	}
}

func TestPatchUserByAdminRequiresAdminActor(t *testing.T) {
	users := newAdminUserServiceFake(map[uint]domainuser.User{
		1: {ID: 1, Role: domainuser.RoleUser},
		2: {ID: 2, Role: domainuser.RoleUser},
	})
	service := NewService(users, auditServiceFake{})

	nextRole := domainuser.RoleAdmin
	_, err := service.PatchUserByAdmin(
		context.Background(),
		"req_1",
		1,
		2,
		PatchUserInput{Role: &nextRole},
		"127.0.0.1",
		"test",
	)
	if !errors.Is(err, ErrAdminPermissionRequired) {
		t.Fatalf("expected admin permission protection, got %v", err)
	}
}

func TestPatchUserByAdminCannotPromoteSuperAdmin(t *testing.T) {
	users := newAdminUserServiceFake(map[uint]domainuser.User{
		1: {ID: 1, Role: domainuser.RoleAdmin},
		2: {ID: 2, Role: domainuser.RoleUser},
	})
	service := NewService(users, auditServiceFake{})

	nextRole := domainuser.RoleSuperAdmin
	_, err := service.PatchUserByAdmin(
		context.Background(),
		"req_1",
		1,
		2,
		PatchUserInput{Role: &nextRole},
		"127.0.0.1",
		"test",
	)
	if !errors.Is(err, ErrSuperAdminManagementNotAllowed) {
		t.Fatalf("expected admin superadmin promotion protection, got %v", err)
	}
}

func TestPatchUserByAdminCannotManageSuperAdmin(t *testing.T) {
	users := newAdminUserServiceFake(map[uint]domainuser.User{
		1: {ID: 1, Role: domainuser.RoleAdmin},
		2: {ID: 2, Role: domainuser.RoleSuperAdmin},
	})
	service := NewService(users, auditServiceFake{})

	displayName := "Root"
	_, err := service.PatchUserByAdmin(
		context.Background(),
		"req_1",
		1,
		2,
		PatchUserInput{DisplayName: &displayName},
		"127.0.0.1",
		"test",
	)
	if !errors.Is(err, ErrSuperAdminManagementNotAllowed) {
		t.Fatalf("expected admin superadmin management protection, got %v", err)
	}
}

func TestPatchUserByAdminMapsRepositoryLastSuperAdminGuard(t *testing.T) {
	users := newAdminUserServiceFake(map[uint]domainuser.User{
		1: {ID: 1, Role: domainuser.RoleSuperAdmin},
		2: {ID: 2, Role: domainuser.RoleSuperAdmin},
	})
	users.updateFieldsErr = repository.ErrLastSuperAdminRoleChange
	service := NewService(users, auditServiceFake{})

	nextRole := domainuser.RoleUser
	_, err := service.PatchUserByAdmin(
		context.Background(),
		"req_1",
		2,
		1,
		PatchUserInput{Role: &nextRole},
		"127.0.0.1",
		"test",
	)
	if !errors.Is(err, ErrLastSuperAdminRoleChangeNotAllowed) {
		t.Fatalf("expected repository guard to map to admin error, got %v", err)
	}
}

type adminUserServiceFake struct {
	users           map[uint]domainuser.User
	updateFieldsErr error
	superAdminCount *int64
}

func newAdminUserServiceFake(users map[uint]domainuser.User) *adminUserServiceFake {
	copied := make(map[uint]domainuser.User, len(users))
	for id, item := range users {
		copied[id] = item
	}
	return &adminUserServiceFake{users: copied}
}

func (s *adminUserServiceFake) ListUsers(context.Context, int, int, repository.UserListFilter) ([]domainuser.User, int64, error) {
	return nil, 0, nil
}

func (s *adminUserServiceFake) ListLatestSessionActivityByUserIDs(context.Context, []uint) (map[uint]time.Time, error) {
	return map[uint]time.Time{}, nil
}

func (s *adminUserServiceFake) CountSuperAdmins(context.Context) (int64, error) {
	if s.superAdminCount != nil {
		return *s.superAdminCount, nil
	}
	var count int64
	for _, item := range s.users {
		if item.Role == domainuser.RoleSuperAdmin {
			count++
		}
	}
	return count, nil
}

func (s *adminUserServiceFake) CreateUser(
	context.Context,
	string,
	string,
	string,
	string,
	string,
	string,
	string,
	string,
	string,
	string,
	*time.Time,
) (*domainuser.User, error) {
	return nil, nil
}

func (s *adminUserServiceFake) GetByID(_ context.Context, userID uint) (*domainuser.User, error) {
	item, ok := s.users[userID]
	if !ok {
		return nil, errors.New("user not found")
	}
	return &item, nil
}

func (s *adminUserServiceFake) RevokeAllSessions(context.Context, uint, string) error {
	return nil
}

func (s *adminUserServiceFake) UpdateUserStatus(_ context.Context, userID uint, status string) error {
	item, ok := s.users[userID]
	if !ok {
		return errors.New("user not found")
	}
	item.Status = status
	s.users[userID] = item
	return nil
}

func (s *adminUserServiceFake) UpdateFields(_ context.Context, userID uint, input repository.UpdateUserFieldsInput) (*domainuser.User, error) {
	if s.updateFieldsErr != nil {
		return nil, s.updateFieldsErr
	}
	item, ok := s.users[userID]
	if !ok {
		return nil, errors.New("user not found")
	}
	if input.Role != nil {
		item.Role = *input.Role
	}
	if input.Timezone != nil {
		item.Timezone = *input.Timezone
	}
	if input.Locale != nil {
		item.Locale = *input.Locale
	}
	s.users[userID] = item
	return &item, nil
}

func (s *adminUserServiceFake) ResetLoginFailure(context.Context, uint) error {
	return nil
}

func (s *adminUserServiceFake) ResetPasswordByAdmin(context.Context, uint, string, bool) error {
	return nil
}

func (s *adminUserServiceFake) DeleteAccountHard(context.Context, uint) error {
	return nil
}

func (s *adminUserServiceFake) RecordAuthEvent(context.Context, uint, string, string, string, string, string, string, string) error {
	return nil
}

func (s *adminUserServiceFake) ListAuthEvents(context.Context, uint, string, string, int, int) ([]domainuser.AuthEvent, int64, error) {
	return nil, 0, nil
}

type auditServiceFake struct{}

func (auditServiceFake) Write(
	context.Context,
	string,
	uint,
	string,
	string,
	string,
	string,
	string,
	interface{},
) {
}

func (auditServiceFake) List(context.Context, int, int, auditapp.ListFilter) ([]domainaudit.Log, int64, error) {
	return nil, 0, nil
}

type subscriptionResolverFake struct {
	billingMode   string
	subscriptions map[uint]billing.UserSubscriptionSnapshot
	accounts      map[uint]billing.UserBillingAccountSnapshot
}

func (s subscriptionResolverFake) ListCurrentSubscriptionSnapshots(context.Context, []uint, time.Time) (map[uint]billing.UserSubscriptionSnapshot, error) {
	if s.subscriptions == nil {
		return map[uint]billing.UserSubscriptionSnapshot{}, nil
	}
	return s.subscriptions, nil
}

func (s subscriptionResolverFake) GetCurrentSubscriptionSnapshot(_ context.Context, userID uint, _ time.Time) (*billing.UserSubscriptionSnapshot, error) {
	if s.subscriptions == nil {
		return nil, nil
	}
	subscription, ok := s.subscriptions[userID]
	if !ok {
		return nil, nil
	}
	return &subscription, nil
}

func (s subscriptionResolverFake) GetBillingMode(context.Context) (string, error) {
	return s.billingMode, nil
}

func (s subscriptionResolverFake) ListBillingAccountSnapshots(context.Context, []uint) (map[uint]billing.UserBillingAccountSnapshot, error) {
	if s.accounts == nil {
		return map[uint]billing.UserBillingAccountSnapshot{}, nil
	}
	return s.accounts, nil
}

func (s subscriptionResolverFake) SetUserSubscriptionByPlanCode(context.Context, uint, string, *time.Time) (*billing.UserSubscriptionSnapshot, error) {
	return nil, nil
}

var _ userService = (*adminUserServiceFake)(nil)
var _ auditService = auditServiceFake{}
var _ subscriptionResolver = subscriptionResolverFake{}
