package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	apperrorlog "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/errorlog"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestAccessLogSkipsHealthz(t *testing.T) {
	gin.SetMode(gin.TestMode)

	core, logs := observer.New(zap.InfoLevel)
	router := gin.New()
	router.Use(AccessLog(zap.New(core), nil))
	router.GET("/healthz", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.GET("/api/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if logs.Len() != 0 {
		t.Fatalf("expected /healthz to skip access log, got %d entries", logs.Len())
	}

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/ping", nil))
	if logs.Len() != 1 {
		t.Fatalf("expected regular route to emit access log, got %d entries", logs.Len())
	}
	entry := logs.All()[0]
	if len(entry.Context) != 0 {
		t.Fatalf("expected access log context fields to be empty, got %#v", entry.Context)
	}
	if entry.Message == "" || entry.Message == "http_request" {
		t.Fatalf("expected request details in message, got %q", entry.Message)
	}
}

func TestAccessLogRecordsErrorCodeAndCauseByRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	core, logs := observer.New(zap.InfoLevel)
	router := gin.New()
	router.Use(RequestID(), AccessLog(zap.New(core), nil))
	router.GET("/api/items/:id", func(c *gin.Context) {
		response.InternalError(c, fmt.Errorf("load item: %w", errors.New("dial tcp 10.0.0.5:5432: connection refused")))
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/items/42", nil))

	var body response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(recorder.Body.String(), "connection refused") {
		t.Fatalf("raw error must not reach the client: %s", recorder.Body.String())
	}
	if body.RequestID == "" || body.ErrorCode != response.CodeInternal {
		t.Fatalf("expected request id and internal error code, got %+v", body)
	}

	if logs.Len() != 1 {
		t.Fatalf("expected one access log entry, got %d", logs.Len())
	}
	entry := logs.All()[0]
	if entry.Level != zap.ErrorLevel {
		t.Fatalf("expected error level for 5xx, got %s", entry.Level)
	}
	for _, want := range []string{
		"RequestID: " + body.RequestID,
		"Route: /api/items/:id",
		"ErrorCode: " + response.CodeInternal,
		"Error: load item: dial tcp 10.0.0.5:5432: connection refused",
	} {
		if !strings.Contains(entry.Message, want) {
			t.Fatalf("expected access log to contain %q, got %q", want, entry.Message)
		}
	}
}

func TestAccessLogOmitsErrorFieldsForSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	core, logs := observer.New(zap.InfoLevel)
	router := gin.New()
	router.Use(AccessLog(zap.New(core), nil))
	router.GET("/api/ping", func(c *gin.Context) {
		response.Success(c, nil)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/ping", nil))
	entry := logs.All()[0]
	if entry.Level != zap.InfoLevel || strings.Contains(entry.Message, "ErrorCode:") || strings.Contains(entry.Message, "Error:") {
		t.Fatalf("successful request must log at info without error fields, got %s %q", entry.Level, entry.Message)
	}
}

func TestAccessLogWarnsOnErrorRecordedAfterSuccessfulStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	core, logs := observer.New(zap.InfoLevel)
	router := gin.New()
	router.Use(AccessLog(zap.New(core), nil))
	router.GET("/api/stream", func(c *gin.Context) {
		c.Status(http.StatusOK)
		response.RecordErrorCode(c, response.CodeUpstreamUnavailable)
		response.RecordError(c, errors.New("upstream closed stream"))
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/stream", nil))
	entry := logs.All()[0]
	if entry.Level != zap.WarnLevel || !strings.Contains(entry.Message, "Error: upstream closed stream") {
		t.Fatalf("stream terminal error must be logged as warning with cause, got %s %q", entry.Level, entry.Message)
	}
}

func TestTruncateLoggedErrorKeepsUTF8Boundary(t *testing.T) {
	text := strings.Repeat("错", maxLoggedErrorBytes)
	got := truncateLoggedError(text)
	if !utf8.ValidString(got) || !strings.Contains(got, "truncated") || len(got) > maxLoggedErrorBytes+64 {
		t.Fatalf("unexpected truncation result (len %d)", len(got))
	}
}

func TestAccessLogRedactsSecretsInCauseAndQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	core, logs := observer.New(zap.InfoLevel)
	router := gin.New()
	router.Use(AccessLog(zap.New(core), nil))
	router.GET("/api/oauth/callback", func(c *gin.Context) {
		response.InternalError(c, fmt.Errorf(
			`download artifact: Get "https://user:pw@cdn.example.com/a.png?X-Amz-Signature=abc123&size=2": EOF; auth Bearer sk-live-0123456789abcdef; api_key=AIzaSecret`,
		))
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=authcode123&state=s1&lang=zh", nil))
	message := logs.All()[0].Message
	for _, leaked := range []string{"authcode123", "s1&", ":pw@", "abc123", "sk-live-0123456789abcdef", "AIzaSecret"} {
		if strings.Contains(message, leaked) {
			t.Fatalf("access log leaked %q: %s", leaked, message)
		}
	}
	for _, kept := range []string{"lang=zh", "size=2", "cdn.example.com/a.png", "download artifact"} {
		if !strings.Contains(message, kept) {
			t.Fatalf("access log should keep non-secret context %q: %s", kept, message)
		}
	}
}

func TestRedactSecretsKeepsDiagnosticProse(t *testing.T) {
	for _, text := range []string{
		"invalid token: token is expired",
		"bearer authentication failed",
		"max_tokens=4096 exceeds model limit",
	} {
		if got := redactSecrets(text); got != text {
			t.Errorf("redactSecrets(%q) = %q; diagnostic text must stay intact", text, got)
		}
	}
}

func TestAccessLogFlagsServerErrorWithoutRecordedCause(t *testing.T) {
	gin.SetMode(gin.TestMode)

	core, logs := observer.New(zap.InfoLevel)
	router := gin.New()
	router.Use(AccessLog(zap.New(core), nil))
	router.GET("/api/broken", func(c *gin.Context) {
		response.ErrorWithCode(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/broken", nil))
	if message := logs.All()[0].Message; !strings.Contains(message, "Error: <no cause recorded>") {
		t.Fatalf("5xx without a recorded cause must be flagged, got %q", message)
	}
}

type errorRecorderStub struct {
	inputs []apperrorlog.RecordInput
}

func (stub *errorRecorderStub) Record(_ context.Context, input apperrorlog.RecordInput) {
	stub.inputs = append(stub.inputs, input)
}

func TestAccessLogPersistsServerAndStreamFailuresOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := &errorRecorderStub{}
	router := gin.New()
	router.Use(RequestID(), AccessLog(zap.NewNop(), recorder))
	router.GET("/api/internal", func(c *gin.Context) {
		response.InternalError(c, errors.New("database is locked"))
	})
	router.GET("/api/stream", func(c *gin.Context) {
		c.Status(http.StatusOK)
		response.RecordErrorCode(c, response.CodeUpstreamUnavailable)
		response.RecordError(c, errors.New("upstream closed stream"))
	})
	router.GET("/api/missing", func(c *gin.Context) {
		response.ErrorWithCode(c, http.StatusNotFound, response.CodeResourceNotFound)
	})
	router.GET("/api/ok", func(c *gin.Context) {
		response.Success(c, nil)
	})

	for _, path := range []string{"/api/internal?token=secret", "/api/stream", "/api/missing", "/api/ok"} {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	if len(recorder.inputs) != 2 {
		t.Fatalf("expected only the 5xx and the failed stream to be persisted, got %#v", recorder.inputs)
	}
	internal, stream := recorder.inputs[0], recorder.inputs[1]
	if internal.StatusCode != http.StatusInternalServerError || internal.ErrorCode != response.CodeInternal ||
		internal.Cause != "database is locked" || internal.Route != "/api/internal" || internal.RequestID == "" {
		t.Fatalf("unexpected 5xx entry: %#v", internal)
	}
	if strings.Contains(internal.Path, "secret") {
		t.Fatalf("persisted path must be redacted: %q", internal.Path)
	}
	if stream.StatusCode != http.StatusOK || stream.Cause != "upstream closed stream" {
		t.Fatalf("unexpected stream entry: %#v", stream)
	}
}

type diagnosticErrorStub struct{ body string }

func (e diagnosticErrorStub) Error() string            { return "upstream request failed: status=404" }
func (e diagnosticErrorStub) DiagnosticDetail() string { return e.body }

func TestAccessLogPersistsDiagnosticDetailFromErrorChain(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := &errorRecorderStub{}
	router := gin.New()
	router.Use(RequestID(), AccessLog(zap.NewNop(), recorder))
	router.GET("/api/stream", func(c *gin.Context) {
		c.Status(http.StatusOK)
		response.RecordErrorCode(c, response.CodeUpstreamUnavailable)
		response.RecordError(c, fmt.Errorf("generate: %w", diagnosticErrorStub{
			body: `{"error":{"message":"model not found","api_key=sk-live-123"}}`,
		}))
	})
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/stream", nil))

	if len(recorder.inputs) != 1 {
		t.Fatalf("expected one persisted entry, got %d", len(recorder.inputs))
	}
	detail := recorder.inputs[0].Detail
	if !strings.Contains(detail, "model not found") || strings.Contains(detail, "sk-live-123") {
		t.Fatalf("detail must keep the raw body but mask credentials: %q", detail)
	}
}
