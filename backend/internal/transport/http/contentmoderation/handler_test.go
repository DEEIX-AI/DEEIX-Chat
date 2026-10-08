package contentmoderation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appcm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/contentmoderation"
	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
	cmport "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/contentmoderation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

func TestParseOptionalUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("missing userId returns zero without error", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/events", nil)

		userID, ok := parseOptionalUserID(c)
		if !ok {
			t.Fatal("expected ok=true for missing userId")
		}
		if userID != 0 {
			t.Fatalf("expected UserID 0, got %d", userID)
		}
	})

	t.Run("blank userId returns zero without error", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/events?user_id=%20", nil)

		userID, ok := parseOptionalUserID(c)
		if !ok {
			t.Fatal("expected ok=true for blank userId")
		}
		if userID != 0 {
			t.Fatalf("expected UserID 0, got %d", userID)
		}
	})

	t.Run("valid userId is parsed", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/events?user_id=42", nil)

		userID, ok := parseOptionalUserID(c)
		if !ok {
			t.Fatal("expected ok=true for valid userId")
		}
		if userID != 42 {
			t.Fatalf("expected UserID 42, got %d", userID)
		}
	})

	invalidCases := []struct {
		name  string
		query string
	}{
		{name: "non-numeric", query: "user_id=abc"},
		{name: "negative", query: "user_id=-1"},
		{name: "zero", query: "user_id=0"},
		// 大于任何平台的 uint（超出 ParseUint 位宽限制）。
		{name: "overflow bit width", query: "user_id=18446744073709551616"},
	}
	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/events?"+tc.query, nil)

			userID, ok := parseOptionalUserID(c)
			if ok {
				t.Fatalf("expected ok=false for %q, got userID=%d", tc.query, userID)
			}
			if userID != 0 {
				t.Fatalf("expected UserID 0 on error, got %d", userID)
			}
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d", recorder.Code)
			}
		})
	}
}

func TestParseOptionalRFC3339(t *testing.T) {
	gin.SetMode(gin.TestMode)
	valid := "2026-08-10T01:02:03Z"
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/stats?from="+valid, nil)
	parsed, ok := parseOptionalRFC3339(c, "from")
	if !ok || parsed == nil || !parsed.Equal(time.Date(2026, 8, 10, 1, 2, 3, 0, time.UTC)) {
		t.Fatalf("unexpected parsed time: %v, ok=%v", parsed, ok)
	}

	recorder := httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/stats?from=not-a-time", nil)
	if parsed, ok = parseOptionalRFC3339(c, "from"); ok || parsed != nil {
		t.Fatalf("expected invalid time to be rejected: %v, ok=%v", parsed, ok)
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

type errorEnvelope struct {
	ErrorMsg  string `json:"errorMsg"`
	ErrorCode string `json:"errorCode"`
}

func TestWriteErrorMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const (
		invalidConfigCode = "content_moderation.invalid_config"
		invalidConfigMsg  = "invalid content moderation config"
	)
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{name: "superadmin required", err: appcm.ErrSuperAdminRequired, wantStatus: http.StatusForbidden, wantCode: "auth.superadmin_required", wantMsg: "superadmin permission required"},
		{name: "admin required", err: appcm.ErrAdminRequired, wantStatus: http.StatusForbidden, wantCode: "auth.admin_required", wantMsg: "admin permission required"},
		{name: "event not found", err: appcm.ErrEventNotFound, wantStatus: http.StatusNotFound, wantCode: "content_moderation.event_not_found", wantMsg: "content moderation event not found"},
		{name: "service config required", err: appcm.ErrServiceConfigRequired, wantStatus: http.StatusBadRequest, wantCode: "content_moderation.config_required", wantMsg: "content moderation service config and policy are required when enabled"},
		{name: "port invalid base url", err: cmport.ErrInvalidBaseURL, wantStatus: http.StatusBadRequest, wantCode: invalidConfigCode, wantMsg: invalidConfigMsg},
		{name: "wrapped port invalid base url", err: fmt.Errorf("x: %w", cmport.ErrInvalidBaseURL), wantStatus: http.StatusBadRequest, wantCode: invalidConfigCode, wantMsg: invalidConfigMsg},
		{name: "invalid model", err: appcm.ErrInvalidModel, wantStatus: http.StatusBadRequest, wantCode: invalidConfigCode, wantMsg: invalidConfigMsg},
		{name: "wrapped invalid model", err: fmt.Errorf("x: %w", appcm.ErrInvalidModel), wantStatus: http.StatusBadRequest, wantCode: invalidConfigCode, wantMsg: invalidConfigMsg},
		{name: "invalid timeout", err: appcm.ErrInvalidTimeout, wantStatus: http.StatusBadRequest, wantCode: invalidConfigCode, wantMsg: invalidConfigMsg},
		{name: "invalid concurrency", err: appcm.ErrInvalidConcurrency, wantStatus: http.StatusBadRequest, wantCode: invalidConfigCode, wantMsg: invalidConfigMsg},
		{name: "invalid queue capacity", err: appcm.ErrInvalidQueueCapacity, wantStatus: http.StatusBadRequest, wantCode: invalidConfigCode, wantMsg: invalidConfigMsg},
		{name: "invalid categories", err: appcm.ErrInvalidCategories, wantStatus: http.StatusBadRequest, wantCode: invalidConfigCode, wantMsg: invalidConfigMsg},
		{name: "image text only category", err: appcm.ErrImageTextOnlyCategory, wantStatus: http.StatusBadRequest, wantCode: invalidConfigCode, wantMsg: invalidConfigMsg},
		{name: "invalid config", err: appcm.ErrInvalidConfig, wantStatus: http.StatusBadRequest, wantCode: invalidConfigCode, wantMsg: invalidConfigMsg},
		{name: "probe failed", err: appcm.ErrProbeFailed, wantStatus: http.StatusBadRequest, wantCode: "content_moderation.probe_failed", wantMsg: "content moderation probe failed"},
		{name: "invalid event filter", err: appcm.ErrInvalidEventFilter, wantStatus: http.StatusBadRequest, wantCode: "request.invalid_query", wantMsg: "invalid query parameter"},
		{name: "unknown error", err: errors.New("database exploded"), wantStatus: http.StatusInternalServerError, wantCode: "internal.error", wantMsg: "internal server error"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

			writeError(c, tc.err)

			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.wantStatus)
			}
			var body errorEnvelope
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body %q: %v", recorder.Body.String(), err)
			}
			if body.ErrorCode != tc.wantCode || body.ErrorMsg != tc.wantMsg {
				t.Fatalf("body = (%q, %q), want (%q, %q)", body.ErrorCode, body.ErrorMsg, tc.wantCode, tc.wantMsg)
			}
		})
	}
}

// listEventsRepoStub 只实现 ListEvents，其余方法由嵌入的 nil 接口承接（被调用即 panic）。
type listEventsRepoStub struct {
	repository.ContentModerationRepository
	filters []domaincm.EventListFilter
}

func (s *listEventsRepoStub) ListEvents(_ context.Context, filter domaincm.EventListFilter) ([]domaincm.Event, int64, error) {
	s.filters = append(s.filters, filter)
	return []domaincm.Event{}, 0, nil
}

func TestListEventsReadsSnakeCasePagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name         string
		query        string
		wantPage     int
		wantPageSize int
		wantOffset   int
	}{
		{name: "defaults", query: "", wantPage: pagination.DefaultPage, wantPageSize: pagination.DefaultPageSize, wantOffset: 0},
		{name: "page and page_size", query: "page=3&page_size=50", wantPage: 3, wantPageSize: 50, wantOffset: 100},
		{name: "maximum page size", query: "page=2&page_size=1000", wantPage: 2, wantPageSize: 1000, wantOffset: 1000},
		{name: "page size above maximum is clamped", query: "page=1&page_size=5000", wantPage: 1, wantPageSize: pagination.MaxPageSize, wantOffset: 0},
		{name: "camelCase pageSize is ignored", query: "page=2&pageSize=50", wantPage: 2, wantPageSize: pagination.DefaultPageSize, wantOffset: pagination.DefaultPageSize},
		{name: "invalid values fall back to defaults", query: "page=abc&page_size=-5", wantPage: pagination.DefaultPage, wantPageSize: pagination.DefaultPageSize, wantOffset: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &listEventsRepoStub{}
			handler := NewHandler(appcm.NewService(nil, repo, nil, nil))
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/admin/content-moderation/events?"+tc.query, nil)
			c.Set(middleware.ContextKeyUserRole, "superadmin")

			handler.ListEvents(c)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if len(repo.filters) != 1 {
				t.Fatalf("ListEvents called %d times, want 1", len(repo.filters))
			}
			if got := repo.filters[0]; got.Offset != tc.wantOffset || got.Limit != tc.wantPageSize {
				t.Fatalf("filter offset/limit = (%d, %d), want (%d, %d)", got.Offset, got.Limit, tc.wantOffset, tc.wantPageSize)
			}
			var body struct {
				Data struct {
					Page     int `json:"page"`
					PageSize int `json:"pageSize"`
				} `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.Data.Page != tc.wantPage || body.Data.PageSize != tc.wantPageSize {
				t.Fatalf("response page/pageSize = (%d, %d), want (%d, %d)", body.Data.Page, body.Data.PageSize, tc.wantPage, tc.wantPageSize)
			}
		})
	}
}
