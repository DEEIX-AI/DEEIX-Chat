package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appauth "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/auth"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

func TestWriteAccountLockedResponseReportsRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	writeAccountLockedResponse(context, &appauth.AccountLockedError{RetryAfter: 90 * time.Second})

	if recorder.Code != http.StatusLocked {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusLocked)
	}
	if header := recorder.Header().Get("Retry-After"); header != "90" {
		t.Fatalf("Retry-After = %q, want %q", header, "90")
	}

	var payload response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ErrorCode != "auth.account_locked" {
		t.Fatalf("errorCode = %q, want %q", payload.ErrorCode, "auth.account_locked")
	}
	if payload.ErrorMsg != "account is temporarily locked, try again later" {
		t.Fatalf("errorMsg = %q", payload.ErrorMsg)
	}
}

func TestWriteAccountLockedResponseOmitsUnknownRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	writeAccountLockedResponse(context, appauth.ErrAccountLocked)

	if recorder.Code != http.StatusLocked {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusLocked)
	}
	if header := recorder.Header().Get("Retry-After"); header != "" {
		t.Fatalf("Retry-After = %q, want empty when the unlock time is unknown", header)
	}

	var payload response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ErrorCode != "auth.account_locked" {
		t.Fatalf("errorCode = %q, want %q", payload.ErrorCode, "auth.account_locked")
	}
}
