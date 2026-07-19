package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appadmin "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/admin"
	auditapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	domainaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/audit"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

func TestPatchUserReturnsForbiddenWhenAdminManagesSuperAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	users := &handlerUserServiceFake{users: map[uint]domainuser.User{
		1: {ID: 1, Role: domainuser.RoleAdmin},
		2: {ID: 2, Role: domainuser.RoleSuperAdmin},
	}}
	handler := NewHandler(appadmin.NewService(users, handlerAuditServiceFake{}))

	router := gin.New()
	router.PATCH("/admin/users/:id", func(c *gin.Context) {
		c.Set(middleware.ContextKeyUserID, uint(1))
		c.Set(middleware.ContextKeyRequestID, "req_test")
		handler.PatchUser(c)
	})

	request := httptest.NewRequest(http.MethodPatch, "/admin/users/2", strings.NewReader(`{"displayName":"Root"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "user.superadmin_management_protected") {
		t.Fatalf("expected superadmin management error code, got body=%s", recorder.Body.String())
	}
}

func TestGetUsageStatisticsRejectsConflictingSubjectFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := appadmin.NewService(&handlerUserServiceFake{}, handlerAuditServiceFake{})
	service.SetUsageStatisticsService(handlerUsageStatisticsServiceFake{})
	handler := NewHandler(service)
	router := gin.New()
	router.GET("/admin/usage-statistics", handler.GetUsageStatistics)

	request := httptest.NewRequest(http.MethodGet, "/admin/usage-statistics?user_id=1&permission_group_id=2", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "usage_statistics.subject_conflict") {
		t.Fatalf("expected stable subject conflict error code, got body=%s", recorder.Body.String())
	}
}

func TestGetUsageStatisticsResolvesRankingMetrics(t *testing.T) {
	tests := []struct {
		name            string
		query           string
		wantSection     string
		wantModelRankBy string
		wantUserRankBy  string
	}{
		{
			name:            "default ranking metrics",
			wantSection:     "all",
			wantModelRankBy: "cost",
			wantUserRankBy:  "cost",
		},
		{
			name:            "independent ranking metrics",
			query:           "?section=models&model_rank_by=tokens&user_rank_by=calls",
			wantSection:     "models",
			wantModelRankBy: "tokens",
			wantUserRankBy:  "calls",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			capturedFilter := appbilling.UsageStatisticsFilter{}
			service := appadmin.NewService(&handlerUserServiceFake{}, handlerAuditServiceFake{})
			service.SetUsageStatisticsService(handlerUsageStatisticsCaptureFake{filter: &capturedFilter})
			handler := NewHandler(service)
			router := gin.New()
			router.GET("/admin/usage-statistics", handler.GetUsageStatistics)

			request := httptest.NewRequest(http.MethodGet, "/admin/usage-statistics"+tt.query, nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("expected success, got status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if capturedFilter.Section != tt.wantSection ||
				capturedFilter.ModelRankBy != tt.wantModelRankBy ||
				capturedFilter.UserRankBy != tt.wantUserRankBy {
				t.Fatalf("unexpected ranking metrics: %+v", capturedFilter)
			}
			if !strings.Contains(recorder.Body.String(), `"section":"`+tt.wantSection+`"`) {
				t.Fatalf("expected response section %q, got body=%s", tt.wantSection, recorder.Body.String())
			}
		})
	}
}

type handlerUserServiceFake struct {
	users map[uint]domainuser.User
}

func (s *handlerUserServiceFake) ListUsers(context.Context, int, int, repository.UserListFilter) ([]domainuser.User, int64, error) {
	return nil, 0, nil
}

func (s *handlerUserServiceFake) ListIdentityProviders(context.Context, bool) ([]domainuser.IdentityProvider, error) {
	return []domainuser.IdentityProvider{}, nil
}

func (s *handlerUserServiceFake) ListUserIdentitiesByUserIDs(context.Context, []uint) (map[uint][]domainuser.UserIdentity, error) {
	return map[uint][]domainuser.UserIdentity{}, nil
}

func (s *handlerUserServiceFake) ListLatestSessionActivityByUserIDs(context.Context, []uint) (map[uint]time.Time, error) {
	return map[uint]time.Time{}, nil
}

func (s *handlerUserServiceFake) CountSuperAdmins(context.Context) (int64, error) {
	var count int64
	for _, item := range s.users {
		if item.Role == domainuser.RoleSuperAdmin {
			count++
		}
	}
	return count, nil
}

func (s *handlerUserServiceFake) CreateUser(
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

func (s *handlerUserServiceFake) GetByID(_ context.Context, userID uint) (*domainuser.User, error) {
	item, ok := s.users[userID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &item, nil
}

func (s *handlerUserServiceFake) RevokeAllSessions(context.Context, uint, string) error {
	return nil
}

func (s *handlerUserServiceFake) UpdateUserStatus(context.Context, uint, string) error {
	return nil
}

func (s *handlerUserServiceFake) UpdateFields(context.Context, uint, repository.UpdateUserFieldsInput) (*domainuser.User, error) {
	return nil, nil
}

func (s *handlerUserServiceFake) ResetLoginFailure(context.Context, uint) error {
	return nil
}

func (s *handlerUserServiceFake) ResetPasswordByAdmin(context.Context, uint, string, bool) error {
	return nil
}

func (s *handlerUserServiceFake) DeleteAccountHard(context.Context, uint) error {
	return nil
}

func (s *handlerUserServiceFake) RecordAuthEvent(context.Context, uint, string, string, string, string, string, string, string) error {
	return nil
}

func (s *handlerUserServiceFake) ListAuthEvents(context.Context, uint, string, string, int, int) ([]domainuser.AuthEvent, int64, error) {
	return nil, 0, nil
}

type handlerAuditServiceFake struct{}

type handlerUsageStatisticsServiceFake struct{}

func (handlerUsageStatisticsServiceFake) GetUsageStatistics(context.Context, appbilling.UsageStatisticsFilter) (domainbilling.UsageStatistics, error) {
	return domainbilling.UsageStatistics{}, nil
}

type handlerUsageStatisticsCaptureFake struct {
	filter *appbilling.UsageStatisticsFilter
}

func (f handlerUsageStatisticsCaptureFake) GetUsageStatistics(_ context.Context, filter appbilling.UsageStatisticsFilter) (domainbilling.UsageStatistics, error) {
	*f.filter = filter
	return domainbilling.UsageStatistics{}, nil
}

func (handlerAuditServiceFake) Write(context.Context, string, uint, string, string, string, string, string, interface{}) {
}

func (handlerAuditServiceFake) List(context.Context, int, int, auditapp.ListFilter) ([]domainaudit.Log, int64, error) {
	return nil, 0, nil
}
