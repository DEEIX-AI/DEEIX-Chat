package contentmoderation

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
		c.Request = httptest.NewRequest(http.MethodGet, "/events?userId=%20", nil)

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
		c.Request = httptest.NewRequest(http.MethodGet, "/events?userId=42", nil)

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
		{name: "non-numeric", query: "userId=abc"},
		{name: "negative", query: "userId=-1"},
		{name: "zero", query: "userId=0"},
		// Larger than any platform uint (exceeds ParseUint bit-size limit).
		{name: "overflow bit width", query: "userId=18446744073709551616"},
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

func TestParsePaginationRejectsInvalidValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, query := range []string{"page=0", "page=abc", "pageSize=0", "pageSize=101", "pageSize=abc"} {
		t.Run(query, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/events?"+query, nil)
			if _, _, ok := parsePagination(c); ok {
				t.Fatalf("expected %q to be rejected", query)
			}
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", recorder.Code)
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
